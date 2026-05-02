package cache

import (
	"context"
	"testing"
	"time"
)

func TestCache_GetMissReturnsZero(t *testing.T) {
	c := New()
	got, ok := c.Get("nws_alerts")
	if ok {
		t.Errorf("expected miss, got hit: %+v", got)
	}
}

func TestCache_SetThenGet(t *testing.T) {
	c := New()
	env := Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "sha256:abc", Payload: []int{1, 2, 3}}
	c.Set(env)
	got, ok := c.Get("nws_alerts")
	if !ok {
		t.Fatalf("expected hit")
	}
	if got.Validator != env.Validator {
		t.Errorf("validator mismatch: %q vs %q", got.Validator, env.Validator)
	}
}

func TestCache_SubscribeReceivesSet(t *testing.T) {
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := c.Subscribe(ctx, "nws_alerts")
	env := Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1"}
	c.Set(env)
	select {
	case got := <-ch:
		if got.Validator != "v1" {
			t.Errorf("got validator %q, want v1", got.Validator)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("subscriber did not receive event")
	}
}

func TestCache_NoBroadcastOnIdenticalValidator(t *testing.T) {
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := c.Subscribe(ctx, "nws_alerts")
	env := Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "vA"}
	c.Set(env)
	<-ch       // drain first
	c.Set(env) // identical validator
	select {
	case unwanted := <-ch:
		t.Fatalf("did not expect a second broadcast, got %+v", unwanted)
	case <-time.After(150 * time.Millisecond):
		// success
	}
}

func TestCache_SubscribeUnsubsOnContextDone(t *testing.T) {
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	ch := c.Subscribe(ctx, "nws_alerts")
	cancel()
	c.Set(Envelope{Source: "nws_alerts", Validator: "v"})
	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case _, open := <-ch:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("channel never closed after ctx cancel")
		}
	}
}
