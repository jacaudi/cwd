package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "cwd.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_OpenMigrateIdempotent(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate twice: %v", err)
	}
}

func TestStore_AppendAndLatest(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := s.Append(context.Background(), "nws_alerts", now, "vA", []byte(`["a"]`)); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, ok, err := s.Latest(context.Background(), "nws_alerts")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if !ok {
		t.Fatal("expected a row")
	}
	if got.Validator != "vA" {
		t.Errorf("validator: got %q", got.Validator)
	}
	if string(got.Payload) != `["a"]` {
		t.Errorf("payload: got %s", got.Payload)
	}
	if !got.FetchedAt.Equal(now) {
		t.Errorf("fetchedAt: got %v want %v", got.FetchedAt, now)
	}
}

func TestStore_AtReturnsNearestPrior(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for i, v := range []string{"vA", "vB", "vC"} {
		if err := s.Append(context.Background(), "nws_alerts", t0.Add(time.Duration(i)*time.Minute), v, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := s.At(context.Background(), "nws_alerts", t0.Add(90*time.Second))
	if err != nil || !ok {
		t.Fatalf("at: ok=%v err=%v", ok, err)
	}
	if got.Validator != "vB" {
		t.Errorf("got %q want vB", got.Validator)
	}

	_, ok, _ = s.At(context.Background(), "nws_alerts", t0.Add(-time.Hour))
	if ok {
		t.Errorf("expected miss before first row")
	}
}

func TestStore_Prune(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := now.Add(-31 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	if err := s.Append(context.Background(), "nws_alerts", old, "vOld", []byte("o")); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(context.Background(), "nws_alerts", recent, "vNew", []byte("n")); err != nil {
		t.Fatal(err)
	}
	n, err := s.Prune(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 pruned row, got %d", n)
	}
	got, ok, _ := s.Latest(context.Background(), "nws_alerts")
	if !ok || got.Validator != "vNew" {
		t.Errorf("latest after prune: %+v", got)
	}
}

func TestStore_GzipRoundTrip(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 64*1024)
	for i := range big {
		big[i] = byte(i % 251)
	}
	if err := s.Append(context.Background(), "nws_alerts", time.Now(), "vBig", big); err != nil {
		t.Fatalf("append big: %v", err)
	}
	got, ok, _ := s.Latest(context.Background(), "nws_alerts")
	if !ok {
		t.Fatal("no row")
	}
	if len(got.Payload) != len(big) {
		t.Errorf("payload len mismatch: %d vs %d", len(got.Payload), len(big))
	}
}
