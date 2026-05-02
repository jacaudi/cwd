package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

type historyHandler struct {
	store  *store.Store
	filter sources.Filter
}

// NewHistoryHandler returns the GET /api/history?at=RFC3339 handler.
func NewHistoryHandler(s *store.Store, f sources.Filter) http.Handler {
	return &historyHandler{store: s, filter: f}
}

func (h *historyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atStr := r.URL.Query().Get("at")
	if atStr == "" {
		http.Error(w, "missing 'at' query parameter", http.StatusBadRequest)
		return
	}
	at, err := time.Parse(time.RFC3339, atStr)
	if err != nil {
		http.Error(w, "invalid 'at' (want RFC3339): "+err.Error(), http.StatusBadRequest)
		return
	}
	resp := SnapshotResponse{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]Envelope{},
	}
	row, ok, err := h.store.At(r.Context(), sources.NWSAlertsName, at)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if ok {
		var alerts []sources.Alert
		if err := json.Unmarshal(row.Payload, &alerts); err != nil {
			http.Error(w, "decode payload: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp.Sources[sources.NWSAlertsName] = Envelope{
			Source:    row.Source,
			FetchedAt: row.FetchedAt,
			ETag:      row.Validator,
			Payload:   h.filter.Apply(alerts),
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_ = json.NewEncoder(w).Encode(resp)
}
