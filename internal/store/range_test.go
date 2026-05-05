package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}

func TestStore_Range_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		ts := t0.Add(time.Duration(i) * time.Minute)
		payload, _ := json.Marshal(map[string]int{"i": i})
		if err := s.Append(ctx, "test_source", ts, "v"+string(rune('a'+i)), payload); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	rows, err := s.Range(ctx, "test_source", t0, t0.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if got, want := len(rows), 5; got != want {
		t.Fatalf("len(rows) = %d, want %d", got, want)
	}
	for i, r := range rows {
		if got, want := r.FetchedAt.UTC(), t0.Add(time.Duration(i)*time.Minute); !got.Equal(want) {
			t.Errorf("row %d FetchedAt = %s, want %s", i, got, want)
		}
		var p map[string]int
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			t.Errorf("row %d unmarshal: %v", i, err)
		}
		if p["i"] != i {
			t.Errorf("row %d payload i = %d, want %d", i, p["i"], i)
		}
	}
}

func TestStore_Range_FiltersBySource(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "src_a", t0, "va", []byte(`{"x":1}`))
	_ = s.Append(ctx, "src_b", t0.Add(time.Minute), "vb", []byte(`{"x":2}`))

	rows, err := s.Range(ctx, "src_a", t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(rows); got != 1 {
		t.Fatalf("len(rows) = %d, want 1", got)
	}
	if rows[0].Source != "src_a" {
		t.Errorf("Source = %q, want src_a", rows[0].Source)
	}
}

func TestStore_Range_EmptyReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.Range(context.Background(), "nope", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(rows); got != 0 {
		t.Errorf("len(rows) = %d, want 0", got)
	}
}

func TestStore_RangeBuckets_HonorsN(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)

	// 24 rows, 1 hour apart over 24h.
	for i := 0; i < 24; i++ {
		ts := t0.Add(time.Duration(i) * time.Hour)
		payload, _ := json.Marshal(map[string]int{"v": i})
		_ = s.Append(ctx, "test_source", ts, time.RFC3339Nano, payload)
	}

	// Aggregator that records the count it saw.
	var seenCounts []int
	agg := func(rows []Row) ([]byte, error) {
		seenCounts = append(seenCounts, len(rows))
		return []byte(`{}`), nil
	}

	buckets, err := s.RangeBuckets(ctx, "test_source", t0, t0.Add(24*time.Hour), 6, agg)
	if err != nil {
		t.Fatalf("RangeBuckets: %v", err)
	}
	if got := len(buckets); got != 6 {
		t.Errorf("len(buckets) = %d, want 6", got)
	}
	// 24 rows / 6 buckets = 4 rows per bucket
	for i, c := range seenCounts {
		if c != 4 {
			t.Errorf("bucket %d row count = %d, want 4", i, c)
		}
	}
}

func TestStore_RangeBuckets_AggregatorErrorPropagates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "test_source", t0, "v", []byte(`{}`))

	wantErr := errAggBoom
	agg := func(rows []Row) ([]byte, error) { return nil, wantErr }

	_, err := s.RangeBuckets(ctx, "test_source", t0.Add(-time.Hour), t0.Add(time.Hour), 1, agg)
	if err == nil {
		t.Fatal("RangeBuckets: want error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want errors.Is %v", err, wantErr)
	}
}

func TestStore_RangeBuckets_BucketStartUsesFirstRow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "test_source", t0.Add(15*time.Minute), "v1", []byte(`{}`))
	_ = s.Append(ctx, "test_source", t0.Add(45*time.Minute), "v2", []byte(`{}`))
	_ = s.Append(ctx, "test_source", t0.Add(75*time.Minute), "v3", []byte(`{}`))

	agg := func(rows []Row) ([]byte, error) { return []byte(`{}`), nil }
	buckets, err := s.RangeBuckets(ctx, "test_source", t0, t0.Add(2*time.Hour), 2, agg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(buckets); got != 2 {
		t.Fatalf("len(buckets) = %d, want 2", got)
	}
	// Bucket 0 spans [t0, t0+1h); first row in it is t0+15m.
	if !buckets[0].BucketStart.Equal(t0.Add(15 * time.Minute)) {
		t.Errorf("buckets[0].BucketStart = %s, want %s", buckets[0].BucketStart, t0.Add(15*time.Minute))
	}
	// Bucket 1 spans [t0+1h, t0+2h); first row is t0+75m.
	if !buckets[1].BucketStart.Equal(t0.Add(75 * time.Minute)) {
		t.Errorf("buckets[1].BucketStart = %s, want %s", buckets[1].BucketStart, t0.Add(75*time.Minute))
	}
}

var errAggBoom = errSentinel("aggregator boom")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
