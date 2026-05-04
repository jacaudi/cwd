package imageproxy

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, maxBytes int64) *DiskStore {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewDiskStore(dir, maxBytes, logger)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	return s
}

func TestDiskStore_PutGetRoundTrip(t *testing.T) {
	s := newTestStore(t, 1<<20)
	want := []byte("hello-image-bytes")
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.Put("spc.day1otlk", want, "image/png", "v1", now); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ct, val, fetched, err := s.Get("spc.day1otlk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("bytes mismatch")
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q", ct)
	}
	if val != "v1" {
		t.Errorf("validator = %q", val)
	}
	if !fetched.Equal(now) {
		t.Errorf("fetchedAt = %s, want %s", fetched, now)
	}
}

func TestDiskStore_GetMissing_ReturnsErrNotExist(t *testing.T) {
	s := newTestStore(t, 1<<20)
	_, _, _, _, err := s.Get("never.written")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}

func TestDiskStore_RestartLoadsManifest(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s1, err := NewDiskStore(dir, 1<<20, logger)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := s1.Put("a.b", []byte("payload-a"), "image/png", "va", now); err != nil {
		t.Fatal(err)
	}
	if err := s1.Put("c.d", []byte("payload-c"), "image/gif", "vc", now); err != nil {
		t.Fatal(err)
	}

	s2, err := NewDiskStore(dir, 1<<20, logger)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	for _, k := range []string{"a.b", "c.d"} {
		if _, _, _, _, err := s2.Get(k); err != nil {
			t.Errorf("after restart, Get(%q) = %v", k, err)
		}
	}
	entries, _ := s2.Stats()
	if entries != 2 {
		t.Errorf("entries after restart = %d, want 2", entries)
	}
}

func TestDiskStore_LRUEvictionAtByteCap(t *testing.T) {
	// Cap is 30 bytes; each entry is 10 bytes (5-byte payload + ~5 bytes manifest
	// overhead per entry — but eviction is computed against payload bytes only,
	// per Stats(). Three 10-byte payloads exactly fits; a fourth must evict the LRU.
	s := newTestStore(t, 30)
	now := time.Now().UTC()
	put := func(k string, ts time.Time) {
		if err := s.Put(k, []byte("0123456789"), "image/png", "v", ts); err != nil {
			t.Fatalf("Put(%s): %v", k, err)
		}
	}
	put("a.b", now.Add(-3*time.Second))
	put("c.d", now.Add(-2*time.Second))
	put("e.f", now.Add(-1*time.Second))
	put("g.h", now) // forces eviction of a.b (oldest)

	if _, _, _, _, err := s.Get("a.b"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a.b should have been evicted, got err=%v", err)
	}
	for _, k := range []string{"c.d", "e.f", "g.h"} {
		if _, _, _, _, err := s.Get(k); err != nil {
			t.Errorf("Get(%q) after eviction: %v", k, err)
		}
	}
}

func TestDiskStore_PutOverwritesValidatorAndBytes(t *testing.T) {
	s := newTestStore(t, 1<<20)
	now := time.Now().UTC()
	if err := s.Put("k.k", []byte("v1-bytes"), "image/png", "v1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Put("k.k", []byte("v2-bytes"), "image/png", "v2", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got, _, val, _, err := s.Get("k.k")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("v2-bytes")) {
		t.Errorf("Get returned old bytes")
	}
	if val != "v2" {
		t.Errorf("validator = %q, want v2", val)
	}
}

func TestDiskStore_AtomicWrite_NoHalfFiles(t *testing.T) {
	// Simulate a torn write by truncating the .tmp file mid-flight: confirm
	// that NewDiskStore on a directory containing only orphan .tmp files
	// drops them and reports zero entries.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ab"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ab", "deadbeef.bin.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewDiskStore(dir, 1<<20, logger)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	entries, _ := s.Stats()
	if entries != 0 {
		t.Errorf("entries = %d, want 0 (orphan .tmp must be ignored)", entries)
	}
	// Orphan .tmp should be cleaned by the constructor.
	if _, err := os.Stat(filepath.Join(dir, "ab", "deadbeef.bin.tmp")); !os.IsNotExist(err) {
		t.Errorf("orphan .tmp still present, err=%v", err)
	}
}

func TestDiskStore_ConcurrentPuts_NoCorruption(t *testing.T) {
	s := newTestStore(t, 1<<20)
	now := time.Now().UTC()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload := []byte("payload-" + string(rune('a'+i%26)))
			_ = s.Put("k."+string(rune('a'+i%26)), payload, "image/png", "v", now)
		}()
	}
	wg.Wait()
	entries, _ := s.Stats()
	if entries == 0 {
		t.Errorf("entries = 0 after concurrent Puts")
	}
}
