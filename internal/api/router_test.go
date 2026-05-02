package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acaudill/cwd/internal/config"
)

func TestRouterServesAllEndpoints(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true}}
	h := NewRouter(cfg, func() bool { return true })

	cases := []struct {
		path string
		want int
	}{
		{"/healthz", 200},
		{"/readyz", 200},
		{"/api/version", 200},
		{"/api/uiconfig", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Errorf("GET %s: status = %d, want %d", tc.path, rr.Code, tc.want)
		}
	}
}

func TestRouterServesEmbeddedSPAOnUnknownPath(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark"}}
	h := NewRouter(cfg, func() bool { return true })

	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "<html") {
		end := 80
		if len(body) < end {
			end = len(body)
		}
		t.Errorf("body should contain <html, got %q", string(body)[:end])
	}
}

func TestRouterDoesNotIntercept404OnAPI(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark"}}
	h := NewRouter(cfg, func() bool { return true })

	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}
