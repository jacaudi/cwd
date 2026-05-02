package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
