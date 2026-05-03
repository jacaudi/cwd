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

// snapshotSourceNames is the canonical wire-order for the snapshot map.
// Iteration order doesn't change correctness (map encoding is unordered) but
// the slice keeps the 5-source contract close to the handler so adding a
// 6th source in a future phase only touches one place here.
var snapshotSourceNames = []string{
	sources.NWSAlertsName,
	sources.SWPCScalesName,
	sources.SWPCAlertsName,
	sources.USGSQuakesName,
	sources.USGSVolcanoesName,
}

func (h *snapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := SnapshotResponse{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]Envelope{},
	}
	for _, name := range snapshotSourceNames {
		env, ok := h.cache.Get(name)
		if !ok {
			continue
		}
		payload := env.Payload
		// Region filter applies only to nws_alerts (design §9 decision 9 —
		// space-weather and global quake/volcano feeds aren't UGC/WFO-coded).
		if name == sources.NWSAlertsName {
			if alerts, ok := env.Payload.([]sources.Alert); ok {
				payload = h.filter.Apply(alerts)
			}
		}
		resp.Sources[name] = Envelope{
			Source:    env.Source,
			FetchedAt: env.FetchedAt,
			ETag:      env.Validator,
			Payload:   payload,
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
