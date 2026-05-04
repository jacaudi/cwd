package imageproxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testProxy returns a Proxy with a single fake registry of one entry pointing
// at the supplied test URL, plus a recording invalidator. Hot+disk are fresh
// per call; the test url replaces the entry's URL (registry is otherwise
// immutable, so we override per-test via a custom registry passed to New).
func testProxy(t *testing.T, key, url, mime string, interval time.Duration) (*Proxy, *atomic.Int64, *[]ImageInvalidate, *sync.Mutex) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, err := NewDiskStore(t.TempDir(), 1<<20, logger)
	if err != nil {
		t.Fatal(err)
	}
	reg := []Image{{Source: splitKey(key, 0), Name: splitKey(key, 1), URL: url, MIME: mime, DefaultInterval: interval, Description: "test"}}
	var calls atomic.Int64
	var invs []ImageInvalidate
	var mu sync.Mutex
	p := New(Config{
		Registry:  reg,
		Hot:       hot,
		Disk:      disk,
		UserAgent: "cwd-test/0",
		Logger:    logger,
		OnInvalidate: func(ev ImageInvalidate) error {
			mu.Lock()
			invs = append(invs, ev)
			mu.Unlock()
			return nil
		},
		HTTPTimeout: 2 * time.Second,
	})
	return p, &calls, &invs, &mu
}

func splitKey(key string, idx int) string {
	for i := 0; i < len(key); i++ {
		if key[i] == '.' {
			if idx == 0 {
				return key[:i]
			}
			return key[i+1:]
		}
	}
	return ""
}

func TestProxy_Get_UnknownKey_ReturnsNotExist(t *testing.T) {
	p, _, _, _ := testProxy(t, "spc.day1otlk", "http://x", "image/png", time.Minute)
	_, _, _, _, _, _, err := p.Get(context.Background(), "no.such")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}

func TestProxy_Get_LazyFetch_HappyPath(t *testing.T) {
	body := []byte("PNG-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "cwd-test/0" {
			t.Errorf("UA = %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, invs, mu := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Minute)
	got, ct, val, _, isStale, _, err := p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("body mismatch")
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q", ct)
	}
	if val != `"e1"` {
		t.Errorf("validator = %q, want quoted ETag", val)
	}
	if isStale {
		t.Errorf("expected fresh, got stale")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*invs) != 1 {
		t.Errorf("expected 1 invalidate, got %d", len(*invs))
	}
}

func TestProxy_Get_HotTierHit_NoUpstreamCall(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("first"))
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	// First call: lazy fetch.
	_, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("after first Get, upstream calls = %d, want 1", calls.Load())
	}
	// Second call: hot-tier hit, no upstream.
	_, _, _, _, _, _, err = p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("after second Get, upstream calls = %d, want 1 (hot-tier hit)", calls.Load())
	}
}

func TestProxy_Refresh_304_NoBroadcastNoBytes(t *testing.T) {
	var calls atomic.Int64
	body := []byte("v1-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("If-None-Match") == `"e1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, invs, mu := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	if err := p.Refresh(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(*invs) != 1 {
		t.Errorf("after first Refresh, invalidates = %d, want 1", len(*invs))
	}
	mu.Unlock()
	if err := p.Refresh(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(*invs) != 1 {
		t.Errorf("after 304 Refresh, invalidates = %d, want still 1 (no spurious)", len(*invs))
	}
	mu.Unlock()
	if calls.Load() != 2 {
		t.Errorf("upstream calls = %d, want 2", calls.Load())
	}
}

func TestProxy_DiffOnWrite_NoBroadcastOnIdenticalBody(t *testing.T) {
	body := []byte("identical-bytes")
	// Simulate an upstream that does not send ETag/Last-Modified — every poll
	// is a full 200 — but the body never changes. Validator falls back to
	// sha256; the proxy must NOT broadcast invalidate on the second poll.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, invs, mu := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	for i := 0; i < 3; i++ {
		if err := p.Refresh(context.Background(), "spc.day1otlk"); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*invs) != 1 {
		t.Errorf("invalidates = %d, want exactly 1 (only first poll changes bytes)", len(*invs))
	}
}

func TestProxy_UpstreamFailureWithCache_ServesStale(t *testing.T) {
	var calls atomic.Int64
	body := []byte("good-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Nanosecond) // immediately stale
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	// Force a refresh (would happen async; here we call sync to be deterministic).
	_ = p.Refresh(context.Background(), "spc.day1otlk")

	got, _, _, _, isStale, since, err := p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatalf("expected stale-but-served, got err=%v", err)
	}
	if string(got) != string(body) {
		t.Errorf("expected last-good bytes, got %q", got)
	}
	if !isStale {
		t.Errorf("isStale should be true after upstream failure")
	}
	if since <= 0 {
		t.Errorf("staleSince should be > 0, got %s", since)
	}
}

func TestProxy_UpstreamFailureNoCache_ReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	_, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk")
	if err == nil {
		t.Errorf("expected error when no cached copy + upstream fails")
	}
}

func TestProxy_DiskHit_WarmsHotTier(t *testing.T) {
	body := []byte("disk-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	// Lazy fetch populates hot+disk.
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	// Evict from hot tier directly to simulate hot-tier eviction under load.
	p.hot = NewHotTier(1<<20, 100)
	if p.hot.Has("spc.day1otlk") {
		t.Fatal("hot should be empty after rebuild")
	}
	// Get should hit disk and warm the hot tier.
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	if !p.hot.Has("spc.day1otlk") {
		t.Errorf("hot tier should have been warmed from disk")
	}
}

func TestProxy_Stats_TracksRegisteredKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Minute)
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	stats := p.Stats()
	h, ok := stats["spc.day1otlk"]
	if !ok {
		t.Fatal("expected stats entry for spc.day1otlk after Get")
	}
	if h.LastSuccess.IsZero() {
		t.Errorf("LastSuccess should be set")
	}
	if h.IntervalSec <= 0 {
		t.Errorf("IntervalSec = %d", h.IntervalSec)
	}
}
