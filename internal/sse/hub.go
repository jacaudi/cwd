package sse

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

// AlertFilter is the predicate the hub applies to nws_alerts payloads before emitting.
// Set via WithFilter; nil means "match all" (Phase 0 behavior).
type AlertFilter interface {
	Apply(in []sources.Alert) []sources.Alert
}

// Hub fan-outs cached envelopes to SSE clients via per-client Serve calls.
type Hub struct {
	cache        *cache.Cache
	sources      []string
	pingInterval time.Duration
	flusher      func(io.Writer)
	filter       AlertFilter
}

// Option configures a Hub at construction time.
type Option func(*Hub)

// WithPingInterval overrides the SSE comment-ping cadence (default 25s).
func WithPingInterval(d time.Duration) Option { return func(h *Hub) { h.pingInterval = d } }

// WithFilter sets the alert filter the hub applies to nws_alerts payloads.
func WithFilter(f AlertFilter) Option { return func(h *Hub) { h.filter = f } }

// NewHub constructs a Hub bound to a Cache and the source-name allowlist.
func NewHub(c *cache.Cache, sources []string, opts ...Option) *Hub {
	h := &Hub{
		cache:        c,
		sources:      sources,
		pingInterval: 25 * time.Second,
		flusher: func(w io.Writer) {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		},
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Run is a no-op placeholder kept for symmetry with sources/store/fetcher.
// Each connected client runs its own Serve; no shared loop needed.
func (h *Hub) Run(ctx context.Context) { <-ctx.Done() }

// Serve writes SSE frames to w until ctx is done. Per-client.
// The caller (HTTP handler) is responsible for setting headers and Flusher
// behavior; this method writes raw frames + flushes.
func (h *Hub) Serve(ctx context.Context, w io.Writer) {
	snap := Snapshot{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]any{},
	}
	for _, name := range h.sources {
		if env, ok := h.cache.Get(name); ok {
			snap.Sources[name] = h.applyFilter(name, env)
		}
	}
	_ = writeEvent(w, "snapshot", snap)
	h.flusher(w)

	subs := make([]<-chan cache.Envelope, 0, len(h.sources))
	for _, name := range h.sources {
		subs = append(subs, h.cache.Subscribe(ctx, name))
	}

	ping := time.NewTicker(h.pingInterval)
	defer ping.Stop()

	mux := make(chan envWithName, 16)
	var wg sync.WaitGroup
	for i, ch := range subs {
		i := i
		ch := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for env := range ch {
				select {
				case mux <- envWithName{name: h.sources[i], env: env}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-mux:
			out := h.applyFilter(msg.name, msg.env)
			if err := writeEvent(w, msg.name+".update", out); err != nil {
				return
			}
			h.flusher(w)
		case <-ping.C:
			if err := writeEvent(w, "", nil); err != nil {
				return
			}
			h.flusher(w)
		}
	}
}

// applyFilter returns a filtered copy of env when the source is "nws_alerts" and
// a filter is configured. For all other sources (or when no filter is set) it
// returns env unchanged.
func (h *Hub) applyFilter(name string, env cache.Envelope) cache.Envelope {
	if h.filter == nil || name != "nws_alerts" {
		return env
	}
	alerts, ok := env.Payload.([]sources.Alert)
	if !ok {
		return env
	}
	return cache.Envelope{
		Source:    env.Source,
		FetchedAt: env.FetchedAt,
		Validator: env.Validator,
		Payload:   h.filter.Apply(alerts),
	}
}

type envWithName struct {
	name string
	env  cache.Envelope
}
