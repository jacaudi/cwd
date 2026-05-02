package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacaudi/cwd/internal/config"
)

func TestHealthzAlwaysOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	Healthz()(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok\n" {
		t.Errorf("body = %q, want \"ok\\n\"", rr.Body.String())
	}
}

func TestReadyzReportsReadiness(t *testing.T) {
	ready := false
	h := Readyz(func() bool { return ready })

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("not ready: status = %d, want 503", rr.Code)
	}

	ready = true
	rr = httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ready: status = %d, want 200", rr.Code)
	}
}

// TestHealthz_HEADReturns200 verifies that HEAD /healthz is accepted (200).
// Load balancers that probe with HEAD must not receive 405.
// The test routes through the full router so the method registration is exercised.
// Note: httptest.ResponseRecorder buffers body bytes that a real http.Server would
// suppress on HEAD; we assert only the status code — not body emptiness — because
// the recorder does not replicate the server's HEAD-stripping behaviour.
func TestHealthz_HEADReturns200(t *testing.T) {
	cfg := &config.Config{}
	router := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return true }})

	req := httptest.NewRequest(http.MethodHead, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD /healthz: got %d, want 200", rec.Code)
	}
}

// TestReadyz_HEADReflectsState verifies that HEAD /readyz is accepted (200 or 503).
// Load balancers that probe with HEAD must not receive 405.
// The test routes through the full router so the method registration is exercised.
func TestReadyz_HEADReflectsState(t *testing.T) {
	cfg := &config.Config{}
	router := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return false }})

	req := httptest.NewRequest(http.MethodHead, "/readyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusOK {
		t.Errorf("HEAD /readyz: got %d, want 200 or 503", rec.Code)
	}
}
