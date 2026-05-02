package api

import (
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"

	"github.com/jacaudi/cwd/internal/config"
	"github.com/jacaudi/cwd/internal/webdist"
)

// jsonRouteTimeout is the WriteTimeout applied to every JSON / probe endpoint.
// SSE (/api/stream) is explicitly excluded — see NewRouter.
const jsonRouteTimeout = 5 * time.Second

// WithJSONTimeout wraps h with http.TimeoutHandler at the given duration.
// Apply to any JSON endpoint; never apply to SSE (the timeout would kill the stream).
// The duration is parameterized so tests can use a short value to trigger the timeout
// without relying on the 5-second production default.
func WithJSONTimeout(d time.Duration, h http.Handler) http.Handler {
	return http.TimeoutHandler(h, d, `{"error":"upstream timeout"}`)
}

// jsonTimeoutMiddleware is the chi-compatible middleware adapter for WithJSONTimeout.
// It uses the production jsonRouteTimeout constant.
func jsonTimeoutMiddleware(next http.Handler) http.Handler {
	return WithJSONTimeout(jsonRouteTimeout, next)
}

// staticExtensions are file extensions considered "asset-like". A request for
// any path bearing one of these extensions that misses the embedded FS returns
// 404 instead of falling back to the SPA — this prevents stale <script src>
// references after a redeploy from silently executing index.html as JS.
var staticExtensions = map[string]struct{}{
	".js":    {},
	".css":   {},
	".map":   {},
	".png":   {},
	".jpg":   {},
	".jpeg":  {},
	".gif":   {},
	".svg":   {},
	".ico":   {},
	".webp":  {},
	".woff":  {},
	".woff2": {},
	".ttf":   {},
	".otf":   {},
	".wasm":  {},
	".json":  {},
	".txt":   {},
	".xml":   {},
}

// RouterDeps wires the HTTP surface to the rest of the server.
// Each handler is constructed in server.Run() and passed in here.
type RouterDeps struct {
	Config          *config.Config
	Ready           func() bool
	SourcesHandler  http.Handler
	SnapshotHandler http.Handler
	HistoryHandler  http.Handler
	StreamHandler   http.Handler
}

// NewRouter assembles the HTTP surface from the given dependencies:
//   - /healthz, /readyz             — liveness + readiness probes
//   - /api/version, /api/uiconfig   — JSON endpoints
//   - /api/sources                  — per-source fetcher health (when SourcesHandler is set)
//   - /api/snapshot                 — latest cache snapshot with region filter (when SnapshotHandler is set)
//   - /api/history                  — nearest-prior historical snapshot at ?at=RFC3339 (when HistoryHandler is set)
//   - /api/stream                   — SSE fan-out of live cache updates (when StreamHandler is set)
//   - /                             — embedded SPA (with SPA-fallback for client-side routes)
//
// WriteTimeout policy (Phase 0 reviewer carryover):
//   - All JSON/probe endpoints are wrapped in a 5-second http.TimeoutHandler via
//     jsonTimeoutMiddleware. A slow upstream causes a 503 with {"error":"upstream timeout"}.
//   - /api/stream is registered OUTSIDE the timeout group — applying a write timeout
//     to an SSE connection would kill long-lived streams.
//
// /api/* paths that don't match return 404 (no SPA fallback for the API namespace).
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimid.RequestID)
	r.Use(chimid.RealIP)
	r.Use(chimid.Recoverer)

	// Per-route WriteTimeout policy:
	//   - JSON / probe endpoints: 5 s timeout via http.TimeoutHandler (inner group).
	//   - /api/stream: NO timeout — registered outside this group so long-lived SSE
	//     connections are never interrupted by the middleware.
	r.Group(func(r chi.Router) {
		r.Use(jsonTimeoutMiddleware)
		r.Method(http.MethodGet, "/healthz", Healthz())
		r.Method(http.MethodHead, "/healthz", Healthz())
		r.Method(http.MethodGet, "/readyz", Readyz(deps.Ready))
		r.Method(http.MethodHead, "/readyz", Readyz(deps.Ready))
		r.Route("/api", func(r chi.Router) {
			r.Get("/version", Version())
			r.Get("/uiconfig", UIConfig(deps.Config))
			if deps.SourcesHandler != nil {
				r.Method(http.MethodGet, "/sources", deps.SourcesHandler)
			}
			if deps.SnapshotHandler != nil {
				r.Method(http.MethodGet, "/snapshot", deps.SnapshotHandler)
			}
			if deps.HistoryHandler != nil {
				r.Method(http.MethodGet, "/history", deps.HistoryHandler)
			}
		})
	})

	// SSE registered OUTSIDE the timeout group — no WriteTimeout on streaming responses.
	if deps.StreamHandler != nil {
		r.Method(http.MethodGet, "/api/stream", deps.StreamHandler)
	}

	r.NotFound(spaFallback())
	return r
}

// spaFallback serves the embedded SPA's index.html for unmatched
// extensionless paths, and serves static assets verbatim from the embedded FS.
// It returns 404 for:
//   - /api or any /api/... path that doesn't match a registered route
//     (no SPA fallback for the API namespace)
//   - paths that look like static assets (have a recognized file extension)
//     but don't exist in the embedded FS — preventing stale asset URLs from
//     silently executing the SPA index.html as the wrong content type.
func spaFallback() http.HandlerFunc {
	fileServer := http.FileServer(http.FS(webdist.FS()))
	return func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if assetExists(r.URL.Path) {
			fileServer.ServeHTTP(w, r)
			return
		}
		if looksLikeAsset(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		// Client-side route: rewrite to "/" so react-router takes over.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	}
}

// isAPIPath reports whether p is the API namespace (covers both "/api" with
// no trailing slash and "/api/..." sub-paths).
func isAPIPath(p string) bool {
	return p == "/api" || strings.HasPrefix(p, "/api/")
}

// looksLikeAsset reports whether p has a recognized static-asset extension.
func looksLikeAsset(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	if ext == "" {
		return false
	}
	_, ok := staticExtensions[ext]
	return ok
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
