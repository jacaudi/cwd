package api

import (
	"io"
	"net/http"
)

// Healthz returns a handler that always responds 200 OK — proves the process is up.
func Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	}
}

// Readyz returns a handler that responds 200 only when ready() reports true.
// Phase 1+ wires ready() to "all enabled sources have produced at least one snapshot".
// In Phase 0 the operator passes a no-op true predicate.
func Readyz(ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "not ready\n")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ready\n")
	}
}
