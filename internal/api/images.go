package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

// ImageProxy is the dependency surface for the GET /img/{source}/{name} handler.
type ImageProxy interface {
	Get(ctx context.Context, key string) (body []byte, contentType, validator string, fetchedAt time.Time, isStale bool, staleSince time.Duration, err error)
}

// imageCacheControl is the outbound Cache-Control. 60s freshness window plus
// 15-minute stale-while-revalidate gives browsers and SW caches latitude to
// serve quickly while a background refresh runs.
const imageCacheControl = "public, max-age=60, stale-while-revalidate=900"

// NewImagesHandler returns the GET /img/{source}/{name} handler.
func NewImagesHandler(p ImageProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source := chi.URLParam(r, "source")
		name := chi.URLParam(r, "name")
		if source == "" || name == "" {
			http.NotFound(w, r)
			return
		}
		key := source + "." + name

		body, ct, validator, fetchedAt, isStale, since, err := p.Get(r.Context(), key)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", imageCacheControl)
		if validator != "" {
			w.Header().Set("ETag", validator)
		}
		if !fetchedAt.IsZero() {
			w.Header().Set("Last-Modified", fetchedAt.UTC().Format(http.TimeFormat))
		}
		if isStale {
			w.Header().Set("Warning", fmt.Sprintf(`110 cwd "stale %s"`, formatStale(since)))
		}

		// Honor browser conditional GET against our stored validator.
		if inm := r.Header.Get("If-None-Match"); validator != "" && inm == validator {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
	})
}

// formatStale renders the duration as Xm or Xs depending on size, matching
// the Warning header convention from design §6.6.
func formatStale(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return fmt.Sprintf("%ds", int(d/time.Second))
}
