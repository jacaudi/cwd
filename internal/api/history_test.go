package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

func newSeededStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for i, v := range []string{"vA", "vB"} {
		body, _ := json.Marshal([]sources.Alert{{ID: v}})
		if err := s.Append(context.Background(), "nws_alerts", t0.Add(time.Duration(i)*time.Minute), v, body); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestHistory_NearestPrior(t *testing.T) {
	s := newSeededStore(t)
	h := NewHistoryHandler(s, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/history?at=2026-05-02T12:01:30Z", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	var got SnapshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	env, ok := got.Sources["nws_alerts"]
	if !ok {
		t.Fatalf("nws_alerts missing: %s", rec.Body.String())
	}
	if env.ETag != "vB" {
		t.Errorf("expected vB (nearest-prior at 12:01:30), got %q", env.ETag)
	}
}

func TestHistory_BadAt(t *testing.T) {
	s := newSeededStore(t)
	h := NewHistoryHandler(s, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/history?at=not-a-time", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHistory_BeforeAnyRow(t *testing.T) {
	s := newSeededStore(t)
	h := NewHistoryHandler(s, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/history?at=2020-01-01T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var got SnapshotResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if _, has := got.Sources["nws_alerts"]; has {
		t.Errorf("expected sources empty before first row")
	}
}
