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

func TestSnapshot_FiveSourcesAllPresent(t *testing.T) {
	c := cache.New()
	now := time.Now().UTC()
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: now, Validator: "v1", Payload: []sources.Alert{{ID: "a1", Category: sources.CatTornado}}})
	c.Set(cache.Envelope{Source: "swpc_scales", FetchedAt: now, Validator: "sha256:abc", Payload: sources.SWPCForecast{Days: [3]sources.SWPCDay{{Date: "2026-05-03", R1: 5}, {}, {}}}})
	c.Set(cache.Envelope{Source: "swpc_alerts", FetchedAt: now, Validator: "sha256:def", Payload: []sources.SWPCAlert{{Code: "K08A", Issued: now}}})
	c.Set(cache.Envelope{Source: "usgs_quakes", FetchedAt: now, Validator: `"e1"`, Payload: []sources.Quake{{ID: "q1", Magnitude: 6.0}}})
	c.Set(cache.Envelope{Source: "usgs_volcanoes", FetchedAt: now, Validator: `W/"v2"`, Payload: []sources.Volcano{{ID: "vol1", Name: "Test", Alert: sources.AlertWATCH}}})

	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	src, _ := got["sources"].(map[string]any)
	for _, want := range []string{"nws_alerts", "swpc_scales", "swpc_alerts", "usgs_quakes", "usgs_volcanoes"} {
		if _, ok := src[want]; !ok {
			t.Errorf("missing %q in snapshot.sources: %+v", want, src)
		}
	}
}

// N2: REST /api/snapshot must include image.invalidate when present in cache,
// matching the SSE initial-paint snapshot. The asymmetry pre-N2 was that the
// hub's snapshot frame included image.invalidate but REST filtered it out.
func TestSnapshot_IncludesImageInvalidateWhenCached(t *testing.T) {
	c := cache.New()
	now := time.Now().UTC()
	c.Set(cache.Envelope{
		Source:    "image.invalidate",
		FetchedAt: now,
		Validator: "spc.day1otlk@2026-05-03T22:14:33Z",
		Payload: map[string]any{
			"source":    "spc",
			"name":      "day1otlk",
			"fetchedAt": now,
		},
	})
	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	src, _ := got["sources"].(map[string]any)
	if _, ok := src["image.invalidate"]; !ok {
		t.Errorf("expected image.invalidate present in /api/snapshot.sources, got: %+v", src)
	}
}

func TestSnapshot_AbsentSourceOmittedNotNull(t *testing.T) {
	c := cache.New()
	c.Set(cache.Envelope{Source: "swpc_scales", FetchedAt: time.Now(), Validator: "v", Payload: sources.SWPCForecast{}})
	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if contains(body, "nws_alerts") {
		t.Errorf("expected nws_alerts omitted (not null), body: %s", body)
	}
	if contains(body, "null") && contains(body, "usgs_quakes") {
		t.Errorf("expected usgs_quakes omitted entirely, body: %s", body)
	}
}

func TestSnapshot_RegionFilterAppliesOnlyToNWSAlerts(t *testing.T) {
	c := cache.New()
	now := time.Now()
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: now, Validator: "v", Payload: []sources.Alert{
		{ID: "in", Category: sources.CatTornado, UGCs: []string{"VAC059"}},
		{ID: "out", Category: sources.CatTornado, UGCs: []string{"CAZ505"}},
	}})
	c.Set(cache.Envelope{Source: "usgs_quakes", FetchedAt: now, Validator: "v", Payload: []sources.Quake{{ID: "q-anywhere", Magnitude: 5.0}}})

	h := NewSnapshotHandler(c, sources.NewFilter([]string{"VAC059"}, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !contains(body, `"in"`) {
		t.Errorf("kept alert missing")
	}
	if contains(body, `"out"`) {
		t.Errorf("filtered alert leaked")
	}
	// usgs_quakes is NOT region-filtered — it's global.
	if !contains(body, "q-anywhere") {
		t.Errorf("quake should pass through region filter unchanged")
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
