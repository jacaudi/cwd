package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sse"
)

// stallHandler holds the response open for d, then writes "late".
func stallHandler(d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(d):
			_, _ = w.Write([]byte("late"))
		case <-r.Context().Done():
			return
		}
	})
}

// TestWithJSONTimeoutInterruptsSlowHandler verifies the middleware actually
// cuts off handlers that exceed the configured duration.
func TestWithJSONTimeoutInterruptsSlowHandler(t *testing.T) {
	wrapped := WithJSONTimeout(50*time.Millisecond, stallHandler(300*time.Millisecond))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil).WithContext(context.Background())
	wrapped.ServeHTTP(rec, req)
	if rec.Body.String() == "late" {
		t.Errorf("expected timeout middleware to interrupt the slow handler; got body %q", rec.Body.String())
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 from TimeoutHandler, got %d", rec.Code)
	}
}

// TestWithJSONTimeoutAllowsFastHandler verifies the middleware passes through
// fast handlers unchanged.
func TestWithJSONTimeoutAllowsFastHandler(t *testing.T) {
	fast := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fast"))
	})
	wrapped := WithJSONTimeout(500*time.Millisecond, fast)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	wrapped.ServeHTTP(rec, req)
	if rec.Body.String() != "fast" {
		t.Errorf("expected fast body, got %q", rec.Body.String())
	}
}

// TestStreamRouteIsOutsideTimeoutGroup verifies that /api/stream is registered
// without the JSON timeout middleware. Strategy: build a router with a stream
// handler that holds open longer than a short test timeout. Then issue a
// request and confirm the response is NOT a TimeoutHandler 503.
//
// We use a sub-second budget to keep the test fast: configure the JSON
// timeout on snapshot/history/sources/etc., and rely on /api/stream being
// excluded structurally (registered outside the timeout group).
func TestStreamRouteIsOutsideTimeoutGroup(t *testing.T) {
	c := cache.New()
	hub := sse.NewHub(c, []string{"nws_alerts"}, sse.WithPingInterval(5*time.Second))
	go hub.Run(context.Background())

	router := NewRouter(RouterDeps{
		Ready:         func() bool { return true },
		StreamHandler: NewStreamHandler(hub),
		// other handlers nil-defaulted; their routes won't register.
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("GET", "/api/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	// The response should be SSE content, not the JSON timeout body.
	if strings.Contains(body, `"error":"upstream timeout"`) {
		t.Errorf("/api/stream should not be wrapped in JSON timeout middleware; got: %q", body)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("/api/stream should set text/event-stream content-type; got: %q", rec.Header().Get("Content-Type"))
	}
}
