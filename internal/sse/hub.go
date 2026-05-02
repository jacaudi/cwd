package sse

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
)

// Hub fan-outs cached envelopes to SSE clients via per-client Serve calls.
type Hub struct {
	cache        *cache.Cache
	sources      []string
	pingInterval time.Duration
	flusher      func(io.Writer)
}

// Option configures a Hub at construction time.
type Option func(*Hub)

// WithPingInterval overrides the SSE comment-ping cadence (default 25s).
func WithPingInterval(d time.Duration) Option { return func(h *Hub) { h.pingInterval = d } }

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
			snap.Sources[name] = env
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
			if err := writeEvent(w, msg.name+".update", msg.env); err != nil {
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

type envWithName struct {
	name string
	env  cache.Envelope
}
