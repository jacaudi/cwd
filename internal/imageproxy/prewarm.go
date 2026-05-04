package imageproxy

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"
)

// StartPrewarm spawns one goroutine per key that calls proxy.Refresh on the
// per-image interval (registry default unless overridden by operator config).
// Returns a stop function that cancels the pollers and blocks until all of
// them have exited. The stop function is idempotent and safe to call from any
// goroutine; cancelling the parent ctx also stops the pollers.
//
// Unknown keys are logged at WARN and silently dropped (the operator's prewarm
// list may include a stale key after a registry change; non-fatal).
//
// Backoff: matches internal/fetcher.Fetcher.Run shape — base interval on
// success, doubling backoff on consecutive failures up to 5x base.
func StartPrewarm(ctx context.Context, p *Proxy, keys []string, logger *slog.Logger) func() {
	if logger == nil {
		logger = slog.Default()
	}
	registered := p.Keys()
	regSet := make(map[string]struct{}, len(registered))
	for _, k := range registered {
		regSet[k] = struct{}{}
	}
	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, k := range keys {
		if _, ok := regSet[k]; !ok {
			logger.Warn("imageproxy.prewarm.unknown_key", "key", k)
			continue
		}
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			runOne(runCtx, p, key, logger)
		}(k)
	}
	return func() {
		cancel()
		wg.Wait()
	}
}

func runOne(ctx context.Context, p *Proxy, key string, logger *slog.Logger) {
	base := p.Interval(key)
	if base <= 0 {
		base = 5 * time.Minute
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
		err := p.Refresh(ctx, key)
		switch {
		case err == nil:
			failures = 0
			delay = base
		case errors.Is(err, os.ErrNotExist):
			logger.Warn("imageproxy.prewarm.unknown_key_runtime", "key", key)
			return
		default:
			failures++
			delay = base
			for i := 0; i < failures-1; i++ {
				delay *= 2
				if delay > maxBackoff {
					delay = maxBackoff
					break
				}
			}
			logger.Warn("imageproxy.prewarm.error", "key", key, "failures", failures, "next_delay", delay.String(), "err", err.Error())
		}
		timer.Reset(delay)
	}
}
