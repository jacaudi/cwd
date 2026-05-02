package api

import (
	"encoding/json"
	"net/http"

	"github.com/jacaudi/cwd/internal/fetcher"
)

// HealthProvider is what the server exposes to the API: a snapshot of all
// per-source fetcher health. Decouples this handler from the server type.
type HealthProvider interface {
	Health() map[string]fetcher.Health
}

type sourcesHandler struct {
	hp HealthProvider
}

// NewSourcesHandler returns the GET /api/sources handler.
func NewSourcesHandler(hp HealthProvider) http.Handler {
	return &sourcesHandler{hp: hp}
}

func (h *sourcesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(h.hp.Health()); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
