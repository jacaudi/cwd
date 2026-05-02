package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

func TestSnapshot_EmptyCache(t *testing.T) {
	c := cache.New()
	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if _, ok := got["serverTime"]; !ok {
		t.Errorf("missing serverTime")
	}
	src, _ := got["sources"].(map[string]any)
	if _, has := src["nws_alerts"]; has {
		t.Errorf("expected nws_alerts omitted on empty cache, got: %+v", src)
	}
}

func TestSnapshot_AppliesRegionFilter(t *testing.T) {
	c := cache.New()
	c.Set(cache.Envelope{
		Source:    "nws_alerts",
		FetchedAt: time.Now(),
		Validator: "v1",
		Payload: []sources.Alert{
			{ID: "a1", Category: sources.CatTornado, UGCs: []string{"VAC059"}},
			{ID: "a2", Category: sources.CatTornado, UGCs: []string{"CAZ505"}},
		},
	})
	h := NewSnapshotHandler(c, sources.NewFilter([]string{"VAC059"}, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !contains(body, "a1") {
		t.Errorf("a1 missing: %s", body)
	}
	if contains(body, "a2") {
		t.Errorf("a2 should be filtered out: %s", body)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
