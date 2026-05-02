package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"
)

// Row is what Latest/At return.
type Row struct {
	Source    string
	FetchedAt time.Time
	Validator string
	Payload   []byte
}

// Append inserts (or no-ops on PK conflict) a row. Payload is gzipped on write.
func (s *Store) Append(ctx context.Context, source string, fetchedAt time.Time, validator string, payload []byte) error {
	gzipped, err := gzipBytes(payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO snapshots(source, fetched_at, etag, payload)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(source, fetched_at) DO NOTHING`,
		source, fetchedAt.UnixMilli(), nullableString(validator), gzipped,
	)
	if err != nil {
		return fmt.Errorf("store: append: %w", err)
	}
	return nil
}

// Latest returns the most recent row for source.
func (s *Store) Latest(ctx context.Context, source string) (Row, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots WHERE source = ?
		 ORDER BY fetched_at DESC LIMIT 1`, source)
	return scanRow(row)
}

// At returns the row whose fetched_at is the largest <= t.
func (s *Store) At(ctx context.Context, source string, t time.Time) (Row, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots WHERE source = ? AND fetched_at <= ?
		 ORDER BY fetched_at DESC LIMIT 1`, source, t.UnixMilli())
	return scanRow(row)
}

// Prune deletes rows older than (now - retention). Returns rows deleted.
func (s *Store) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()
	res, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE fetched_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: prune: %w", err)
	}
	return res.RowsAffected()
}

func scanRow(row *sql.Row) (Row, bool, error) {
	var r Row
	var gzipped []byte
	var ms int64
	err := row.Scan(&r.Source, &ms, &r.Validator, &gzipped)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, false, nil
	}
	if err != nil {
		return Row{}, false, fmt.Errorf("store: scan: %w", err)
	}
	r.FetchedAt = time.UnixMilli(ms).UTC()
	payload, err := gunzipBytes(gzipped)
	if err != nil {
		return Row{}, false, err
	}
	r.Payload = payload
	return r, true, nil
}

func gzipBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(in); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("store: gzip: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("store: gzip close: %w", err)
	}
	return buf.Bytes(), nil
}

func gunzipBytes(in []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, fmt.Errorf("store: gunzip: %w", err)
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
