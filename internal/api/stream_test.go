package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sse"
)

func TestStream_EmitsSnapshotThenUpdate(t *testing.T) {
	c := cache.New()
	hub := sse.NewHub(c, []string{"nws_alerts"}, sse.WithPingInterval(5*time.Second))
	go hub.Run(context.Background())
	h := NewStreamHandler(hub)

	req := httptest.NewRequest("GET", "/api/stream", nil)
	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	go func() {
		time.Sleep(30 * time.Millisecond)
		c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: []string{"x"}})
	}()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "event: snapshot") {
		t.Errorf("missing snapshot frame: %s", body)
	}
	if !strings.Contains(body, "event: nws_alerts.update") {
		t.Errorf("missing update frame: %s", body)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("content-type: %q", rec.Header().Get("Content-Type"))
	}
}
