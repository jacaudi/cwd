package imageproxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// ImageInvalidate is the SSE payload broadcast when a refresh changes bytes.
type ImageInvalidate struct {
	Source    string    `json:"source"`
	Name      string    `json:"name"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// ImageHealth is the per-key health surfaced by /api/sources at "image:<key>".
// JSON tags match fetcher.Health for the Phase 1/2 5 source entries; the
// extra fields (BytesOnDisk, InHotTier) are appended.
type ImageHealth struct {
	IntervalSec         int       `json:"intervalSec"`
	LastAttempt         time.Time `json:"lastAttempt"`
	LastSuccess         time.Time `json:"lastSuccess"`
	LastError           string    `json:"lastError,omitempty"`
	ETag                string    `json:"etag,omitempty"`
	AgeSec              int       `json:"ageSec"`
	ConsecutiveFailures int       `json:"consecutiveFailures"`
	BytesOnDisk         int64     `json:"bytesOnDisk"`
	InHotTier           bool      `json:"inHotTier"`
}

// Config bundles Proxy dependencies. All fields are required unless commented.
type Config struct {
	Registry     []Image
	Hot          *HotTier
	Disk         *DiskStore
	UserAgent    string
	Logger       *slog.Logger
	OnInvalidate func(ImageInvalidate) error // nil disables SSE invalidation
	HTTPTimeout  time.Duration               // default 15s
	NowFn        func() time.Time            // for tests; default time.Now().UTC
	Intervals    map[string]time.Duration    // operator overrides; nil = registry defaults
}

// Proxy is the public API for image fetch + cache.
type Proxy struct {
	registry     map[string]Image
	hot          *HotTier
	disk         *DiskStore
	client       *http.Client
	userAgent    string
	logger       *slog.Logger
	onInvalidate func(ImageInvalidate) error
	nowFn        func() time.Time
	intervals    map[string]time.Duration

	mu    sync.Mutex
	state map[string]*entryState // key -> per-key mutable state
}

// entryState holds Refresh-time mutable state per registry key.
//
// etag holds the upstream ETag verbatim (with quotes). It is only ever set
// from the upstream response header — never overwritten with a synthesized
// validator — so If-None-Match never echoes a body-hash to an upstream that
// doesn't speak ETags. The public validator (returned from Stats() and used
// as the cache.Envelope validator on writes) falls back to bodyHash when
// st.etag is empty; see publicValidatorLocked.
type entryState struct {
	lastAttempt         time.Time
	lastSuccess         time.Time
	lastError           string
	etag                string // upstream ETag verbatim (with quotes); empty when upstream omits it
	lastModified        string // upstream Last-Modified verbatim
	bodyHash            string // sha256:<hex> of last-known body
	consecutiveFailures int
	refreshMu           sync.Mutex // serializes Refresh per key
}

// New constructs a Proxy from cfg. Panics if Registry/Hot/Disk are nil.
func New(cfg Config) *Proxy {
	if cfg.Hot == nil || cfg.Disk == nil {
		panic("imageproxy.New: Hot and Disk required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 15 * time.Second
	}
	if cfg.NowFn == nil {
		cfg.NowFn = func() time.Time { return time.Now().UTC() }
	}
	reg := map[string]Image{}
	for _, img := range cfg.Registry {
		reg[img.Key()] = img
	}
	state := map[string]*entryState{}
	for k := range reg {
		state[k] = &entryState{}
	}
	return &Proxy{
		registry:     reg,
		hot:          cfg.Hot,
		disk:         cfg.Disk,
		client:       &http.Client{Timeout: cfg.HTTPTimeout},
		userAgent:    cfg.UserAgent,
		logger:       cfg.Logger,
		onInvalidate: cfg.OnInvalidate,
		nowFn:        cfg.NowFn,
		intervals:    cfg.Intervals,
		state:        state,
	}
}

// Keys returns all registered keys in iteration order (for stats walks etc).
func (p *Proxy) Keys() []string {
	out := make([]string, 0, len(p.registry))
	for k := range p.registry {
		out = append(out, k)
	}
	return out
}

// Interval returns the effective interval for the key (operator override or
// registry default).
func (p *Proxy) Interval(key string) time.Duration {
	img, ok := p.registry[key]
	if !ok {
		return 0
	}
	if d, ok := p.intervals[key]; ok && d > 0 {
		return d
	}
	return img.DefaultInterval
}

// Get returns the cached or freshly-fetched bytes for a registered key.
//
// Lookup order:
//  1. Hot tier hit + fresh (age < interval): serve immediately.
//  2. Disk hit + fresh: warm hot tier, serve.
//  3. Hot or disk hit + stale: serve cached, fire async Refresh, set
//     isStale + staleSince.
//  4. No cached copy: synchronous Refresh, then serve fresh bytes.
//  5. No cached copy and upstream fails: return error (handler returns 502).
//
// Returns os.ErrNotExist if the key is unknown.
func (p *Proxy) Get(ctx context.Context, key string) (body []byte, contentType, validator string, fetchedAt time.Time, isStale bool, staleSince time.Duration, err error) {
	img, ok := p.registry[key]
	if !ok {
		return nil, "", "", time.Time{}, false, 0, os.ErrNotExist
	}
	st := p.stateOf(key)

	if b, ct, v, ts, hit := p.hot.Get(key); hit {
		stale, since := p.staleness(key, st, ts)
		if stale {
			go p.refreshAsync(key)
		}
		return b, ct, v, ts, stale, since, nil
	}

	if b, ct, v, ts, derr := p.disk.Get(key); derr == nil {
		p.hot.Put(key, b, ct, v, ts)
		stale, since := p.staleness(key, st, ts)
		if stale {
			go p.refreshAsync(key)
		}
		return b, ct, v, ts, stale, since, nil
	}

	if rerr := p.Refresh(ctx, key); rerr != nil {
		return nil, "", "", time.Time{}, false, 0, fmt.Errorf("imageproxy.Get %q: %w", key, rerr)
	}
	if b, ct, v, ts, hit := p.hot.Get(key); hit {
		return b, ct, v, ts, false, 0, nil
	}
	// Refresh succeeded but cache is somehow empty (extremely unlikely; defensive).
	return nil, "", "", time.Time{}, false, 0, fmt.Errorf("imageproxy.Get %q (%s): refresh succeeded but cache empty", key, img.URL)
}

// Refresh attempts a conditional GET for the key. On success-with-change,
// writes hot+disk and broadcasts image.invalidate. On 304 / unchanged body,
// no-ops the cache. On upstream failure, updates failure stats but does NOT
// mutate the cache.
func (p *Proxy) Refresh(ctx context.Context, key string) error {
	img, ok := p.registry[key]
	if !ok {
		return os.ErrNotExist
	}
	st := p.stateOf(key)
	st.refreshMu.Lock()
	defer st.refreshMu.Unlock()

	now := p.nowFn()
	p.markAttempt(st, now)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, img.URL, nil)
	if err != nil {
		p.markFailure(st, err)
		return err
	}
	req.Header.Set("User-Agent", p.userAgent)
	if st.etag != "" {
		req.Header.Set("If-None-Match", st.etag)
	}
	if st.lastModified != "" {
		req.Header.Set("If-Modified-Since", st.lastModified)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.markFailure(st, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		p.markSuccess(st, now)
		return nil
	case resp.StatusCode/100 != 2:
		ferr := fmt.Errorf("upstream status %d", resp.StatusCode)
		p.markFailure(st, ferr)
		return ferr
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.markFailure(st, err)
		return err
	}

	// N3 sanity checks: an upstream that returns 200 OK with text/html (e.g. a
	// CDN rate-limit page) or with an empty body must NOT poison the cache.
	// Treat either as a failure — the prior cached bytes (if any) stay intact.
	// When upstream omits Content-Type entirely, the registry MIME fallback
	// applies (img.MIME); see README "MIME fallback" note.
	if len(body) == 0 {
		ferr := fmt.Errorf("upstream returned empty body")
		p.markFailure(st, ferr)
		return ferr
	}
	upstreamCT := resp.Header.Get("Content-Type")
	if upstreamCT != "" && !isImageContentType(upstreamCT) {
		ferr := fmt.Errorf("upstream returned non-image Content-Type %q", upstreamCT)
		p.markFailure(st, ferr)
		return ferr
	}

	contentType := upstreamCT
	if contentType == "" {
		contentType = img.MIME
	}
	upstreamETag := resp.Header.Get("ETag")
	upstreamLM := resp.Header.Get("Last-Modified")

	sum := sha256.Sum256(body)
	bodyHash := "sha256:" + hex.EncodeToString(sum[:])

	// Diff-on-write predicate: did the body sha256 actually change?
	// Read the current bodyHash under p.mu so we don't race with Stats().
	prevBodyHash := p.readBodyHash(st)
	if prevBodyHash != "" && prevBodyHash == bodyHash {
		// ETag may have churned without a content change (WPC mtime ETags do this);
		// refresh stored validator metadata so future If-None-Match works, but do
		// NOT broadcast. All writes to st.etag/st.lastModified/st.bodyHash MUST
		// hold p.mu — Stats() reads them under the same lock.
		p.markValidator(st, upstreamETag, upstreamLM, "")
		p.markSuccess(st, now)
		return nil
	}

	// Public validator: prefer upstream ETag, fall back to body hash. Used by
	// disk/hot tier metadata and downstream caches. The synthesized fallback
	// is NEVER stored in st.etag — that field is upstream-only so we don't
	// echo a sha256: validator back as If-None-Match on the next request.
	validator := upstreamETag
	if validator == "" {
		validator = bodyHash
	}

	if err := p.disk.Put(key, body, contentType, validator, now); err != nil {
		p.markFailure(st, err)
		return err
	}
	p.hot.Put(key, body, contentType, validator, now)

	p.markValidator(st, upstreamETag, upstreamLM, bodyHash)
	p.markSuccess(st, now)

	if p.onInvalidate != nil {
		ev := ImageInvalidate{Source: img.Source, Name: img.Name, FetchedAt: now}
		if err := p.onInvalidate(ev); err != nil {
			p.logger.Warn("imageproxy.invalidate_callback", "key", key, "err", err.Error())
		}
	}
	return nil
}

// markValidator atomically updates the per-key validator state. Required so
// Stats() and Refresh() agree on the lock guarding st.etag/lastModified/bodyHash.
// Pass bodyHash="" to leave the existing bodyHash unchanged (used by the
// diff-on-write branch where the body did not change).
// Caller must NOT hold p.mu.
func (p *Proxy) markValidator(st *entryState, etag, lastModified, bodyHash string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.etag = etag
	st.lastModified = lastModified
	if bodyHash != "" {
		st.bodyHash = bodyHash
	}
}

// readBodyHash returns st.bodyHash under p.mu.
func (p *Proxy) readBodyHash(st *entryState) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return st.bodyHash
}

// Stats returns per-key health for /api/sources surfacing.
// Lazy-only keys with no observations yet are still listed (with zero times)
// so operators see the registry in /api/sources from boot.
func (p *Proxy) Stats() map[string]ImageHealth {
	out := make(map[string]ImageHealth, len(p.registry))
	for key, img := range p.registry {
		st := p.stateOf(key)
		p.mu.Lock()
		h := ImageHealth{
			IntervalSec:         int(p.Interval(key).Seconds()),
			LastAttempt:         st.lastAttempt,
			LastSuccess:         st.lastSuccess,
			LastError:           st.lastError,
			ETag:                p.publicValidatorLocked(st, st.bodyHash),
			ConsecutiveFailures: st.consecutiveFailures,
			InHotTier:           p.hot.Has(key),
		}
		p.mu.Unlock()
		if !h.LastSuccess.IsZero() {
			h.AgeSec = int(p.nowFn().Sub(h.LastSuccess).Seconds())
		}
		// Best-effort disk size; non-fatal on miss.
		if b, _, _, _, derr := p.disk.Get(key); derr == nil {
			h.BytesOnDisk = int64(len(b))
		}
		_ = img
		out[key] = h
	}
	return out
}

// publicValidatorLocked is the lock-free variant of publicValidatorOf — caller
// must already hold p.mu. Used by Stats() which already holds the lock.
func (p *Proxy) publicValidatorLocked(st *entryState, bodyHash string) string {
	if st.etag != "" {
		return st.etag
	}
	return bodyHash
}

func (p *Proxy) stateOf(key string) *entryState {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s, ok := p.state[key]; ok {
		return s
	}
	s := &entryState{}
	p.state[key] = s
	return s
}

func (p *Proxy) markAttempt(st *entryState, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.lastAttempt = now
}

// markSuccess updates lastSuccess/lastError/consecutiveFailures. It does NOT
// touch st.etag — upstream-validator writes go through markValidator, and
// the public validator (with body-hash fallback) is synthesized at read time
// in publicValidatorLocked. See N1 — keeping these split prevents the proxy
// from echoing a synthesized sha256: validator back to upstreams that don't
// speak ETags.
func (p *Proxy) markSuccess(st *entryState, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.lastSuccess = now
	st.lastError = ""
	st.consecutiveFailures = 0
}

func (p *Proxy) markFailure(st *entryState, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.lastError = truncate(err.Error(), 256)
	st.consecutiveFailures++
}

// staleness reports whether ts is older than the per-key interval. Also true
// if a recent failure has been recorded since lastSuccess. The caller passes
// the key directly — every caller already knows it, so there's no need to
// reverse-lookup it from st.
func (p *Proxy) staleness(key string, st *entryState, ts time.Time) (bool, time.Duration) {
	p.mu.Lock()
	failures := st.consecutiveFailures
	lastSuccess := st.lastSuccess
	p.mu.Unlock()
	now := p.nowFn()
	if failures > 0 && !lastSuccess.IsZero() {
		return true, now.Sub(lastSuccess)
	}
	if !ts.IsZero() && now.Sub(ts) >= p.Interval(key) {
		return true, now.Sub(ts)
	}
	return false, 0
}

func (p *Proxy) refreshAsync(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.Refresh(ctx, key); err != nil && !errors.Is(err, context.Canceled) {
		p.logger.Warn("imageproxy.refresh_async", "key", key, "err", err.Error())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// isImageContentType reports whether ct (a raw Content-Type header value, possibly
// with parameters like "; charset=utf-8") names an image media type. The match
// is case-insensitive on the type and ignores anything after a ";" or whitespace.
func isImageContentType(ct string) bool {
	// Trim leading whitespace.
	i := 0
	for i < len(ct) && (ct[i] == ' ' || ct[i] == '\t') {
		i++
	}
	ct = ct[i:]
	// Cut at the first ";" or whitespace to isolate the media type.
	end := len(ct)
	for j := 0; j < len(ct); j++ {
		if ct[j] == ';' || ct[j] == ' ' || ct[j] == '\t' {
			end = j
			break
		}
	}
	mediaType := ct[:end]
	const prefix = "image/"
	if len(mediaType) < len(prefix) {
		return false
	}
	for k := 0; k < len(prefix); k++ {
		c := mediaType[k]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[k] {
			return false
		}
	}
	return true
}
