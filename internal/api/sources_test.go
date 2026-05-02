package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/fetcher"
)

type fakeHealthProvider struct {
	healths map[string]fetcher.Health
}

func (f *fakeHealthProvider) Health() map[string]fetcher.Health { return f.healths }

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
