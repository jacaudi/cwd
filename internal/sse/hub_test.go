package sse

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

type fakeSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (f *fakeSink) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}

func (f *fakeSink) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

func TestHub_BroadcastsCacheUpdates(t *testing.T) {
	c := cache.New()
	h := NewHub(c, []string{"nws_alerts"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go h.Run(ctx)

	sink := &fakeSink{}
	clientCtx, clientCancel := context.WithCancel(ctx)
	go h.Serve(clientCtx, sink)
	defer clientCancel()
	time.Sleep(20 * time.Millisecond)

	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: []string{"x"}})

	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no event in sink: %q", sink.String())
		default:
			if strings.Contains(sink.String(), "event: nws_alerts.update") {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHub_EmitsSnapshotOnConnect(t *testing.T) {
	c := cache.New()
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: []string{"x"}})
	h := NewHub(c, []string{"nws_alerts"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go h.Run(ctx)

	sink := &fakeSink{}
	go h.Serve(ctx, sink)

	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no snapshot: %q", sink.String())
		default:
			if strings.Contains(sink.String(), "event: snapshot") {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

type matchOnlyA struct{}

func (matchOnlyA) Apply(in []sources.Alert) []sources.Alert {
	out := make([]sources.Alert, 0, len(in))
	for _, a := range in {
		if a.ID == "a1" {
			out = append(out, a)
		}
	}
	return out
}

func TestHub_AppliesFilterToSnapshotAndUpdates(t *testing.T) {
	c := cache.New()
	payload := []sources.Alert{{ID: "a1"}, {ID: "a2"}}
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: payload})

	h := NewHub(c, []string{"nws_alerts"}, WithFilter(matchOnlyA{}))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go h.Run(ctx)

	sink := &fakeSink{}
	go h.Serve(ctx, sink)

	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no snapshot containing a1 only: %q", sink.String())
		default:
			body := sink.String()
			if strings.Contains(body, `"a1"`) && !strings.Contains(body, `"a2"`) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHub_PingComment(t *testing.T) {
	c := cache.New()
	h := NewHub(c, []string{"nws_alerts"}, WithPingInterval(40*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go h.Run(ctx)
	sink := &fakeSink{}
	go h.Serve(ctx, sink)

	deadline := time.After(180 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no ping: %q", sink.String())
		default:
			if strings.Contains(sink.String(), ": ping") {
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
	}
}
