package imageproxy

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"os"
	"sync"
	"time"
)

// startupJitter returns a non-negative duration in [0, base/4). It's the
// per-key offset added to the *first* prewarm tick so a list of N keys with
// the same interval doesn't burst all upstream requests at boot. Subsequent
// ticks fire at the steady cadence with no jitter.
//
// Variable so tests can disable jitter for deterministic cadence assertions
// (TestStartPrewarm_PollsEachKeyOnInterval relies on tight timing).
var startupJitter = func(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(base / 4)))
}

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
	// First tick fires after a small per-key jitter in [0, base/4). Spreads
	// the boot burst across multiple keys on the same upstream host instead
	// of firing them all simultaneously at t=0.
	delay := startupJitter(base)
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
