package imageproxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartPrewarm_PollsEachKeyOnInterval(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write([]byte("bytes"))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, err := NewDiskStore(t.TempDir(), 1<<20, logger)
	if err != nil {
		t.Fatal(err)
	}
	reg := []Image{
		{Source: "a", Name: "x", URL: srv.URL, MIME: "image/png", DefaultInterval: 30 * time.Millisecond, Description: "d"},
		{Source: "b", Name: "y", URL: srv.URL, MIME: "image/png", DefaultInterval: 30 * time.Millisecond, Description: "d"},
	}
	p := New(Config{Registry: reg, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"a.x", "b.y"}, logger)
	defer stop()
	defer cancel()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("did not see >=4 polls within deadline; got %d", calls.Load())
		default:
		}
		if calls.Load() >= 4 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartPrewarm_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, _ := NewDiskStore(t.TempDir(), 1<<20, logger)
	reg := []Image{{Source: "a", Name: "x", URL: srv.URL, MIME: "image/png", DefaultInterval: 50 * time.Millisecond, Description: "d"}}
	p := New(Config{Registry: reg, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"a.x"}, logger)
	cancel()
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartPrewarm did not return after context cancel")
	}
}

func TestStartPrewarm_BacksOffOnFailure(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, _ := NewDiskStore(t.TempDir(), 1<<20, logger)
	reg := []Image{{Source: "a", Name: "x", URL: srv.URL, MIME: "image/png", DefaultInterval: 30 * time.Millisecond, Description: "d"}}
	p := New(Config{Registry: reg, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"a.x"}, logger)

	time.Sleep(500 * time.Millisecond)
	stop()
	cancel()

	// On constant failures with 30ms base + cap 5x = 150ms, in 500ms we'd see
	// roughly: t=0, t=30, t=60, t=120, t=270, t=420 → ~6 attempts.
	// Without backoff at 30ms steady cadence we'd see ~16. Assert <= 10 to
	// prove backoff fired.
	if got := calls.Load(); got > 10 {
		t.Errorf("calls = %d, want <=10 (backoff should slow polling)", got)
	}
}

func TestStartPrewarm_UnknownKey_LoggedNotFatal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, _ := NewDiskStore(t.TempDir(), 1<<20, logger)
	p := New(Config{Registry: []Image{}, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"unknown.key"}, logger)
	// Just confirm it doesn't panic and stops cleanly.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); stop() }()
	cancel()
	wg.Wait()
}
