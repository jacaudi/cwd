package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/fetcher"
	"github.com/jacaudi/cwd/internal/imageproxy"
)

type fakeHealthProvider struct {
	healths map[string]fetcher.Health
}

func (f *fakeHealthProvider) Health() map[string]fetcher.Health { return f.healths }

type stubHealthProvider struct {
	out map[string]fetcher.Health
}

func (s stubHealthProvider) Health() map[string]fetcher.Health { return s.out }

type stubImageHealth struct {
	out map[string]imageproxy.ImageHealth
}

func (s stubImageHealth) ImageHealth() map[string]imageproxy.ImageHealth { return s.out }

func TestSourcesEndpoint(t *testing.T) {
	hp := &fakeHealthProvider{
		healths: map[string]fetcher.Health{
			"nws_alerts": {
				IntervalSec: 30,
				LastAttempt: time.Now().Add(-5 * time.Second),
				LastSuccess: time.Now().Add(-5 * time.Second),
				ETag:        "sha256:abc",
				AgeSec:      5,
			},
		},
	}
	h := NewSourcesHandler(hp)
	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var got map[string]fetcher.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["nws_alerts"].ETag != "sha256:abc" {
		t.Errorf("etag: got %q", got["nws_alerts"].ETag)
	}
}

func TestSourcesHandler_IncludesImageEntriesPrefixedWithImageColon(t *testing.T) {
	now := time.Now().UTC()
	hp := stubHealthProvider{out: map[string]fetcher.Health{
		"nws_alerts": {IntervalSec: 30, LastSuccess: now, ETag: "v1"},
	}}
	ihp := stubImageHealth{out: map[string]imageproxy.ImageHealth{
		"spc.day1otlk": {IntervalSec: 120, LastSuccess: now, ETag: `"e1"`, BytesOnDisk: 1024, InHotTier: true},
		"nhc.atl_7d":   {IntervalSec: 1800, LastSuccess: time.Time{}, ConsecutiveFailures: 0},
	}}
	h := NewSourcesHandler(hp, WithImageHealth(ihp))
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	want := []string{
		`"nws_alerts"`,
		`"image:spc.day1otlk"`,
		`"image:nhc.atl_7d"`,
		`"bytesOnDisk":1024`,
		`"inHotTier":true`,
	}
	for _, w := range want {
		if !strings.Contains(string(body), w) {
			t.Errorf("body missing %s\nbody=%s", w, body)
		}
	}
}

func TestSourcesHandler_NilImageHealth_StillSurfacesSources(t *testing.T) {
	hp := stubHealthProvider{out: map[string]fetcher.Health{"nws_alerts": {IntervalSec: 30}}}
	h := NewSourcesHandler(hp) // no WithImageHealth
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"nws_alerts"`) {
		t.Errorf("body missing nws_alerts: %s", body)
	}
	if strings.Contains(string(body), `"image:`) {
		t.Errorf("body should not contain image entries when ImageHealth is nil: %s", body)
	}
}
