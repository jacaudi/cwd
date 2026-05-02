// Package fetcher runs one Source per goroutine: timer → Fetch → diff →
// cache.Set → store.Append. Exponential backoff on errors.
package fetcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

// Health is the per-source snapshot returned by /api/sources.
type Health struct {
	IntervalSec         int       `json:"intervalSec"`
	LastAttempt         time.Time `json:"lastAttempt"`
	LastSuccess         time.Time `json:"lastSuccess"`
	LastError           string    `json:"lastError,omitempty"`
	ETag                string    `json:"etag,omitempty"`
	AgeSec              int       `json:"ageSec"`
	ConsecutiveFailures int       `json:"consecutiveFailures"`
}

// Fetcher runs a single Source on a timer, broadcasting to cache and persisting to store.
type Fetcher struct {
	src    sources.Source
	cache  *cache.Cache
	store  *store.Store
	logger *slog.Logger

	mu     sync.RWMutex
	health Health
}

// New constructs a Fetcher for the given source, cache, and store.
func New(src sources.Source, c *cache.Cache, st *store.Store, opts ...Option) *Fetcher {
	f := &Fetcher{
		src:    src,
		cache:  c,
		store:  st,
		logger: slog.Default(),
		health: Health{IntervalSec: int(src.Interval().Seconds())},
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// Option mutates a Fetcher at construction time.
type Option func(*Fetcher)

// WithLogger overrides the slog.Logger used for fetcher events.
func WithLogger(l *slog.Logger) Option { return func(f *Fetcher) { f.logger = l } }

// Run blocks until ctx is done. Fires immediately, then on Interval.
func (f *Fetcher) Run(ctx context.Context) {
	base := f.src.Interval()
	if base <= 0 {
		base = 30 * time.Second
	}
	maxBackoff := 5 * base
	delay := time.Duration(0) // first tick fires immediately
	timer := time.NewTimer(delay)
	defer timer.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		ok := f.tick(ctx)
		if ok {
			failures = 0
			delay = base
		} else {
			failures++
			delay = base
			for i := 0; i < failures-1; i++ {
				delay *= 2
				if delay > maxBackoff {
					delay = maxBackoff
					break
				}
			}
		}
		f.mu.Lock()
		f.health.ConsecutiveFailures = failures
		f.mu.Unlock()
		timer.Reset(delay)
	}
}

func (f *Fetcher) tick(ctx context.Context) bool {
	now := time.Now().UTC()
	f.mu.Lock()
	f.health.LastAttempt = now
	f.mu.Unlock()

	res, err := f.src.Fetch(ctx)
	if err != nil {
		f.mu.Lock()
		f.health.LastError = truncate(err.Error(), 256)
		f.mu.Unlock()
		f.logger.Warn("fetcher.error", "source", f.src.Name(), "err", err.Error())
		return false
	}

	env := cache.Envelope{
		Source:    f.src.Name(),
		FetchedAt: now,
		Validator: res.Validator,
		Payload:   res.Payload,
	}
	prev, hadPrev := f.cache.Get(f.src.Name())

	// Persist to store before broadcasting so that any subscriber that reads
	// Latest() immediately upon receiving the envelope sees the row.
	if !hadPrev || prev.Validator != res.Validator {
		payload, err := encodePayloadJSON(res.Payload)
		if err != nil {
			f.logger.Error("fetcher.encode", "source", f.src.Name(), "err", err.Error())
			// Cache was not yet updated; update it now so subscribers still get
			// the new data even though we couldn't persist it.
			f.cache.Set(env)
			f.mu.Lock()
			f.health.LastSuccess = now
			f.health.LastError = ""
			f.health.ETag = res.Validator
			f.mu.Unlock()
			return true
		}
		if err := f.store.Append(ctx, f.src.Name(), now, res.Validator, payload); err != nil {
			f.logger.Error("fetcher.store_append", "source", f.src.Name(), "err", err.Error())
		}
	}

	f.cache.Set(env)
	f.mu.Lock()
	f.health.LastSuccess = now
	f.health.LastError = ""
	f.health.ETag = res.Validator
	f.mu.Unlock()
	return true
}

// Health returns a copy of the current per-source health snapshot.
func (f *Fetcher) Health() Health {
	f.mu.RLock()
	defer f.mu.RUnlock()
	h := f.health
	if !h.LastSuccess.IsZero() {
		h.AgeSec = int(time.Since(h.LastSuccess).Seconds())
	}
	return h
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// encodePayloadJSON serializes the payload to JSON bytes for store.Append.
// Lives here (not in store) because the store is payload-agnostic.
func encodePayloadJSON(p any) ([]byte, error) {
	return jsonMarshal(p)
}
