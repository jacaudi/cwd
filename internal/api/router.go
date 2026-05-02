package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"

	"github.com/acaudill/cwd/internal/config"
	"github.com/acaudill/cwd/internal/webdist"
)

// NewRouter assembles the Phase 0 HTTP surface:
//   - /healthz, /readyz             — liveness + readiness probes
//   - /api/version, /api/uiconfig   — JSON endpoints
//   - /                             — embedded SPA (with SPA-fallback for client-side routes)
//
// /api/* paths that don't match return 404 (no SPA fallback for the API namespace).
func NewRouter(cfg *config.Config, ready func() bool) http.Handler {
	r := chi.NewRouter()
	r.Use(chimid.RequestID)
	r.Use(chimid.RealIP)
	r.Use(chimid.Recoverer)

	r.Get("/healthz", Healthz())
	r.Get("/readyz", Readyz(ready))

	r.Route("/api", func(r chi.Router) {
		r.Get("/version", Version())
		r.Get("/uiconfig", UIConfig(cfg))
	})

	r.NotFound(spaFallback())
	return r
}

// spaFallback serves the embedded SPA's index.html for any unmatched non-/api path,
// and serves static SPA assets for paths that exist in the embedded FS.
// /api/* paths return a plain 404.
func spaFallback() http.HandlerFunc {
	fileServer := http.FileServer(http.FS(webdist.FS()))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// If the path doesn't exist as a static asset, rewrite to "/" so the SPA loads index.html
		// and lets react-router handle the route.
		if !assetExists(r.URL.Path) {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}

func assetExists(p string) bool {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return true
	}
	f, err := webdist.FS().Open(p)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
