package api

import (
	"net/http"

	"github.com/jacaudi/cwd/internal/sse"
)

type streamHandler struct {
	hub *sse.Hub
}

// NewStreamHandler returns the GET /api/stream SSE handler.
func NewStreamHandler(hub *sse.Hub) http.Handler {
	return &streamHandler{hub: hub}
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if proxied
	h.hub.Serve(r.Context(), w)
}
