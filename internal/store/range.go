package store

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Bucket is one downsampled point in a time series.
type Bucket struct {
	// BucketStart is the FetchedAt of the first row that fell into this
	// bucket, in UTC. Charts use this as the X-axis value.
	BucketStart time.Time
	// Count is the number of source-snapshots that fell in this bucket.
	Count int
	// Payload is the bucket-aggregated payload (shape depends on the
	// caller's Aggregator).
	Payload []byte
}

// Aggregator combines N raw payloads into a single per-bucket payload.
// Examples: max(activeCount) for NWS, last for SWPC scales, sum-by-severity
// for SWPC alerts.
type Aggregator func(rows []Row) ([]byte, error)

// Range returns every snapshot row for source whose fetched_at is in [from, to].
// Rows are returned in ascending time order, fully decompressed.
func (s *Store) Range(ctx context.Context, source string, from, to time.Time) ([]Row, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots
		 WHERE source = ? AND fetched_at BETWEEN ? AND ?
		 ORDER BY fetched_at ASC`,
		source, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("store: range query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Row, 0, 64)
	for rows.Next() {
		var r Row
		var ms int64
		var gzipped []byte
		if err := rows.Scan(&r.Source, &ms, &r.Validator, &gzipped); err != nil {
			return nil, fmt.Errorf("store: range scan: %w", err)
		}
		r.FetchedAt = time.UnixMilli(ms).UTC()
		payload, err := gunzipBytes(gzipped)
		if err != nil {
			return nil, fmt.Errorf("store: range gunzip: %w", err)
		}
		r.Payload = payload
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: range iter: %w", err)
	}
	return out, nil
}

// RangeBuckets returns at most n buckets covering [from, to], aggregating each
// bucket via agg. Buckets with zero matching rows are omitted from the result.
// Bucket size = ceil((to - from) / n) seconds, minimum 1 second.
func (s *Store) RangeBuckets(ctx context.Context, source string, from, to time.Time, n int, agg Aggregator) ([]Bucket, error) {
	if n <= 0 {
		return nil, fmt.Errorf("store: RangeBuckets: n must be > 0, got %d", n)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("store: RangeBuckets: to (%s) must be after from (%s)", to, from)
	}
	if agg == nil {
		return nil, fmt.Errorf("store: RangeBuckets: aggregator required")
	}

	// Bucket size in seconds, ceiling so the last bucket fits.
	totalSec := to.Sub(from).Seconds()
	bucketSec := int64(math.Ceil(totalSec / float64(n)))
	if bucketSec < 1 {
		bucketSec = 1
	}
	bucketMS := bucketSec * 1000

	// Single query that groups by bucket index and returns per-bucket
	// rows. We still aggregate in Go (the aggregator may need the raw
	// gzipped payloads), but the GROUP BY keeps us from over-fetching.
	rows, err := s.db.QueryContext(ctx,
		`SELECT
		   ((fetched_at - ?) / ?) AS bucket_idx,
		   source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots
		 WHERE source = ? AND fetched_at BETWEEN ? AND ?
		 ORDER BY fetched_at ASC`,
		from.UnixMilli(), bucketMS,
		source, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("store: range-buckets query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pendingBucket struct {
		idx  int64
		rows []Row
	}
	var current *pendingBucket
	out := make([]Bucket, 0, n)
	flush := func() error {
		if current == nil || len(current.rows) == 0 {
			return nil
		}
		payload, err := agg(current.rows)
		if err != nil {
			return fmt.Errorf("store: aggregator: %w", err)
		}
		out = append(out, Bucket{
			BucketStart: current.rows[0].FetchedAt,
			Count:       len(current.rows),
			Payload:     payload,
		})
		return nil
	}
	for rows.Next() {
		var idx int64
		var r Row
		var ms int64
		var gzipped []byte
		if err := rows.Scan(&idx, &r.Source, &ms, &r.Validator, &gzipped); err != nil {
			return nil, fmt.Errorf("store: range-buckets scan: %w", err)
		}
		r.FetchedAt = time.UnixMilli(ms).UTC()
		payload, err := gunzipBytes(gzipped)
		if err != nil {
			return nil, fmt.Errorf("store: range-buckets gunzip: %w", err)
		}
		r.Payload = payload
		if current == nil || idx != current.idx {
			if err := flush(); err != nil {
				return nil, err
			}
			current = &pendingBucket{idx: idx}
		}
		current.rows = append(current.rows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: range-buckets iter: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}
