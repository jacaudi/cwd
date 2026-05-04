package api

import (
	"encoding/json"
	"net/http"

	"github.com/jacaudi/cwd/internal/fetcher"
	"github.com/jacaudi/cwd/internal/imageproxy"
)

// HealthProvider is the per-source fetcher health snapshot (Phase 1/2).
type HealthProvider interface {
	Health() map[string]fetcher.Health
}

// ImageHealthProvider is the per-image-key health snapshot (Phase 3).
// Returned entries are emitted alongside HealthProvider entries with each
// key prefixed by "image:" (e.g. "image:spc.day1otlk").
type ImageHealthProvider interface {
	ImageHealth() map[string]imageproxy.ImageHealth
}

// SourcesOption configures NewSourcesHandler.
type SourcesOption func(*sourcesHandler)

// WithImageHealth attaches an ImageHealthProvider to the handler. When set,
// each image-key entry is emitted as "image:<key>" in the JSON output.
func WithImageHealth(ihp ImageHealthProvider) SourcesOption {
	return func(h *sourcesHandler) { h.ihp = ihp }
}

type sourcesHandler struct {
	hp  HealthProvider
	ihp ImageHealthProvider
}

// NewSourcesHandler returns the GET /api/sources handler.
func NewSourcesHandler(hp HealthProvider, opts ...SourcesOption) http.Handler {
	h := &sourcesHandler{hp: hp}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *sourcesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	out := map[string]any{}
	for k, v := range h.hp.Health() {
		out[k] = v
	}
	if h.ihp != nil {
		for k, v := range h.ihp.ImageHealth() {
			out["image:"+k] = v
		}
	}
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
