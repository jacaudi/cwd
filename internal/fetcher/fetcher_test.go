package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

type fakeSource struct {
	name     string
	interval time.Duration
	hits     atomic.Int64
	respFn   func(hit int64) (sources.FetchResult, error)
}

func (f *fakeSource) Name() string            { return f.name }
func (f *fakeSource) Interval() time.Duration { return f.interval }
func (f *fakeSource) Fetch(ctx context.Context) (sources.FetchResult, error) {
	h := f.hits.Add(1)
	return f.respFn(h)
}

func newTempStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestFetcher_FirstSuccessAppendsAndBroadcasts(t *testing.T) {
	c := cache.New()
	st := newTempStore(t)
	src := &fakeSource{
		name:     "test",
		interval: 50 * time.Millisecond,
		respFn: func(_ int64) (sources.FetchResult, error) {
			return sources.FetchResult{Payload: []int{1}, Validator: "vA"}, nil
		},
	}
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	sub := c.Subscribe(ctx, "test")

	go f.Run(ctx)
	select {
	case env := <-sub:
		if env.Validator != "vA" {
			t.Errorf("validator: got %q", env.Validator)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("no broadcast")
	}
	row, ok, err := st.Latest(context.Background(), "test")
	if err != nil || !ok {
		t.Fatalf("latest: ok=%v err=%v", ok, err)
	}
	if row.Validator != "vA" {
		t.Errorf("store validator: got %q", row.Validator)
	}
}

func TestFetcher_NoBroadcastOnIdenticalValidator(t *testing.T) {
	c := cache.New()
	st := newTempStore(t)
	src := &fakeSource{
		name:     "test",
		interval: 30 * time.Millisecond,
		respFn: func(_ int64) (sources.FetchResult, error) {
			return sources.FetchResult{Payload: []int{1}, Validator: "vSame"}, nil
		},
	}
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	sub := c.Subscribe(ctx, "test")
	go f.Run(ctx)

	<-sub
	timer := time.After(150 * time.Millisecond)
	for {
		select {
		case unwanted := <-sub:
			t.Fatalf("unexpected second broadcast: %+v", unwanted)
		case <-timer:
			return
		}
	}
}

func TestFetcher_BackoffDoublesOnError(t *testing.T) {
	c := cache.New()
	st := newTempStore(t)
	src := &fakeSource{
		name:     "test",
		interval: 20 * time.Millisecond,
		respFn: func(_ int64) (sources.FetchResult, error) {
			return sources.FetchResult{}, errors.New("boom")
		},
	}
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	go f.Run(ctx)
	<-ctx.Done()
	h := f.Health()
	if h.ConsecutiveFailures < 2 {
		t.Errorf("expected ≥2 failures, got %d", h.ConsecutiveFailures)
	}
	if h.LastError == "" {
		t.Errorf("expected lastError populated")
	}
	if src.hits.Load() > 25 {
		t.Errorf("too many hits, backoff not engaged: %d", src.hits.Load())
	}
}

func TestFetcher_NWSAlertsHTTPIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, `{"type":"FeatureCollection","features":[]}`)
	}))
	defer srv.Close()
	src := sources.NewNWSAlerts(srv.URL, "cwd-self-host/test (test@example.com)", nil)
	c := cache.New()
	st := newTempStore(t)
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	sub := c.Subscribe(ctx, "nws_alerts")
	go f.Run(ctx)
	select {
	case env := <-sub:
		alerts, ok := env.Payload.([]sources.Alert)
		if !ok {
			t.Fatalf("payload type %T", env.Payload)
		}
		if len(alerts) != 0 {
			t.Errorf("want 0 alerts, got %d", len(alerts))
		}
		if env.Validator == "" {
			t.Errorf("validator empty")
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("no broadcast")
	}
	if _, err := json.Marshal(c); err == nil { //nolint:staticcheck // intentional: verifying marshaling of Cache (no exported fields) succeeds
		got, _ := c.Get("nws_alerts")
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("marshal envelope: %v", err)
		}
	}
}
