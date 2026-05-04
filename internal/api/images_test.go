package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type stubProxy struct {
	body        []byte
	contentType string
	validator   string
	fetchedAt   time.Time
	isStale     bool
	staleSince  time.Duration
	err         error
	calls       int
}

func (s *stubProxy) Get(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error) {
	s.calls++
	return s.body, s.contentType, s.validator, s.fetchedAt, s.isStale, s.staleSince, s.err
}

func newImagesRouter(p ImageProxy) http.Handler {
	r := chi.NewRouter()
	r.Method(http.MethodGet, "/img/{source}/{name}", NewImagesHandler(p))
	return r
}

func TestImagesHandler_HappyPath(t *testing.T) {
	p := &stubProxy{
		body: []byte("img-bytes"), contentType: "image/png", validator: `"e1"`,
		fetchedAt: time.Now().UTC(),
	}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "max-age=60") {
		t.Errorf("Cache-Control = %q, want max-age=60", resp.Header.Get("Cache-Control"))
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "stale-while-revalidate=900") {
		t.Errorf("Cache-Control missing stale-while-revalidate=900")
	}
	if got := resp.Header.Get("ETag"); got != `"e1"` {
		t.Errorf("ETag = %q", got)
	}
	if got := resp.Header.Get("Warning"); got != "" {
		t.Errorf("Warning header should be absent on fresh, got %q", got)
	}
}

func TestImagesHandler_404OnUnknownKey(t *testing.T) {
	p := &stubProxy{err: os.ErrNotExist}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/no_such")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestImagesHandler_502OnUpstreamFailureNoCache(t *testing.T) {
	p := &stubProxy{err: errors.New("upstream 503")}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

func TestImagesHandler_AddsWarningWhenStale(t *testing.T) {
	p := &stubProxy{
		body: []byte("stale-bytes"), contentType: "image/png", validator: `"e1"`,
		fetchedAt: time.Now().UTC().Add(-12 * time.Minute),
		isStale:   true, staleSince: 12 * time.Minute,
	}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (stale-but-served)", resp.StatusCode)
	}
	w := resp.Header.Get("Warning")
	if !strings.Contains(w, "110 cwd") {
		t.Errorf("Warning header = %q, want 110 cwd ...", w)
	}
	if !strings.Contains(w, "stale 12m") && !strings.Contains(w, "stale 720s") {
		t.Errorf("Warning header should describe stale duration, got %q", w)
	}
}

func TestImagesHandler_304OnIfNoneMatchMatch(t *testing.T) {
	p := &stubProxy{
		body: []byte("img-bytes"), contentType: "image/png", validator: `"e1"`,
		fetchedAt: time.Now().UTC(),
	}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/img/spc/day1otlk", nil)
	req.Header.Set("If-None-Match", `"e1"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp.StatusCode)
	}
}

func TestImagesHandler_KeyComposition(t *testing.T) {
	p := &stubProxy{
		body: []byte("x"), contentType: "image/png", validator: `"e"`,
		fetchedAt: time.Now().UTC(),
	}
	got := ""
	wrap := imageProxyFunc(func(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error) {
		got = key
		return p.body, p.contentType, p.validator, p.fetchedAt, p.isStale, p.staleSince, p.err
	})
	r := newImagesRouter(wrap)
	srv := httptest.NewServer(r)
	defer srv.Close()
	_, _ = http.Get(srv.URL + "/img/spc/day1otlk_fire")
	if got != "spc.day1otlk_fire" {
		t.Errorf("composed key = %q, want spc.day1otlk_fire", got)
	}
}

// imageProxyFunc adapts a func to the ImageProxy interface for tests.
type imageProxyFunc func(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error)

func (f imageProxyFunc) Get(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error) {
	return f(ctx, key)
}
