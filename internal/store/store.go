// Package store is the SQLite ring-buffer for source envelopes.
// 30-day retention; payloads stored gzipped. Pure Go (modernc.org/sqlite).
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store wraps a SQLite connection with the snapshot schema applied.
type Store struct {
	db *sql.DB
}

// Open initializes a SQLite connection at path (created if missing). Parent
// directories are created with 0700 permissions so first-run users don't have
// to mkdir XDG_STATE_HOME themselves.
// Caller must call Migrate at least once before Append/Latest/At.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("store: mkdir %q: %w", dir, err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// Migrate runs the embedded schema; idempotent.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}
