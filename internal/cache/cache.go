// Package cache holds the latest Envelope per source name and broadcasts
// changes to subscribers. Diff-on-write: identical validators do not broadcast.
package cache

import (
	"context"
	"sync"
	"time"
)

// Envelope is the payload-with-metadata stored per source. The Payload is
// source-typed (`[]sources.Alert` for nws_alerts) and downcast at the consumer.
type Envelope struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetchedAt"`
	Validator string    `json:"etag,omitempty"` // serialized as "etag" for the wire shape
	Payload   any       `json:"payload"`
}

type subscriber struct {
	mu     sync.Mutex
	ch     chan Envelope
	closed bool
	cancel <-chan struct{}
}

// send delivers env to the subscriber's channel. Returns false if the subscriber
// is already closed. Protected by s.mu to prevent a race with close.
func (s *subscriber) send(env Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- env:
	case <-s.cancel:
		// subscriber's context is done; will be GC'd next time we prune
	default:
		// slow subscriber; skip rather than block
	}
}

// closeOnce closes the channel exactly once under s.mu.
func (s *subscriber) closeOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

// Cache is the typed in-memory store + change broadcaster.
type Cache struct {
	mu          sync.RWMutex
	latest      map[string]Envelope
	subscribers map[string][]*subscriber
}

// New returns an empty Cache.
func New() *Cache {
	return &Cache{
		latest:      map[string]Envelope{},
		subscribers: map[string][]*subscriber{},
	}
}

// Get returns the latest envelope for source name and a hit indicator.
func (c *Cache) Get(name string) (Envelope, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.latest[name]
	return e, ok
}

// Set stores an envelope and broadcasts if the validator changed.
// Identical-validator writes update the stored envelope (FetchedAt) but do not broadcast.
func (c *Cache) Set(env Envelope) {
	c.mu.Lock()
	prev, ok := c.latest[env.Source]
	c.latest[env.Source] = env
	subs := c.subscribers[env.Source]
	c.mu.Unlock()

	if ok && prev.Validator == env.Validator && env.Validator != "" {
		return
	}
	for _, s := range subs {
		s.send(env)
	}
}

// Subscribe returns a channel of envelopes for the given source. Closes when
// ctx is done. Channel is buffered (size 4) for mild burst tolerance.
func (c *Cache) Subscribe(ctx context.Context, name string) <-chan Envelope {
	s := &subscriber{
		ch:     make(chan Envelope, 4),
		cancel: ctx.Done(),
	}
	c.mu.Lock()
	c.subscribers[name] = append(c.subscribers[name], s)
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		c.mu.Lock()
		subs := c.subscribers[name]
		out := subs[:0]
		for _, x := range subs {
			if x != s {
				out = append(out, x)
			}
		}
		c.subscribers[name] = out
		c.mu.Unlock()
		s.closeOnce()
	}()

	return s.ch
}
