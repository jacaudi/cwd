package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacaudi/cwd/internal/config"
)

func TestRouterServesAllEndpoints(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true}}
	h := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return true }})

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
	h := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return true }})

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
	h := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return true }})

	for _, path := range []string{
		"/api/does-not-exist",
		"/api",               // bare /api with no trailing slash must 404, not return SPA
		"/api/version/extra", // sub-paths past a real route must 404
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, rr.Code)
		}
	}
}

func TestRouterReturns404ForMissingStaticAssets(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark"}}
	h := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return true }})

	// Paths that look like static assets (have a file extension under reserved
	// prefixes) must NOT fall back to index.html — a stale <script src> after
	// a redeploy would otherwise execute as HTML and silently break the page.
	for _, path := range []string{
		"/assets/nonexistent.js",
		"/assets/nonexistent.css",
		"/assets/missing-image.png",
		"/static/foo.js",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, rr.Code)
		}
	}
}

func TestRouterFallsBackToSPAForExtensionlessRoutes(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark"}}
	h := NewRouter(RouterDeps{Config: cfg, Ready: func() bool { return true }})

	// Client-side routes (no extension, not /api/* or /assets/*) must
	// receive index.html so react-router can take over.
	for _, path := range []string{
		"/hazards",
		"/space",
		"/history/2026-01-15T12-00",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, rr.Code)
		}
	}
}
