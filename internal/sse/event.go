package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// writeEvent writes one SSE event to w. name == "" emits a comment ping.
func writeEvent(w io.Writer, name string, payload any) error {
	if name == "" {
		_, err := fmt.Fprint(w, ": ping\n\n")
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	return err
}

// Snapshot is the wire shape for the initial frame.
// Sources is a free-form map so Phase 1 ships only nws_alerts and Phase 2 adds more
// without reshaping the type.
type Snapshot struct {
	ServerTime time.Time      `json:"serverTime"`
	Sources    map[string]any `json:"sources"`
}
