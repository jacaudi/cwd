package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

// Envelope is the wire wrapper for a single source's payload. Mirrors design §5.
type Envelope struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetchedAt"`
	ETag      string    `json:"etag,omitempty"`
	Payload   any       `json:"payload"`
}

// SnapshotResponse is the wire shape of /api/snapshot. Sources is a free-form
// map keyed by source name; only present sources appear (absence == omitted).
type SnapshotResponse struct {
	ServerTime time.Time           `json:"serverTime"`
	Sources    map[string]Envelope `json:"sources"`
}

type snapshotHandler struct {
	cache  *cache.Cache
	filter sources.Filter
}

// NewSnapshotHandler returns the GET /api/snapshot handler.
func NewSnapshotHandler(c *cache.Cache, f sources.Filter) http.Handler {
	return &snapshotHandler{cache: c, filter: f}
}

func (h *snapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := SnapshotResponse{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]Envelope{},
	}
	if env, ok := h.cache.Get(sources.NWSAlertsName); ok {
		alerts, _ := env.Payload.([]sources.Alert)
		resp.Sources[sources.NWSAlertsName] = Envelope{
			Source:    env.Source,
			FetchedAt: env.FetchedAt,
			ETag:      env.Validator,
			Payload:   h.filter.Apply(alerts),
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
