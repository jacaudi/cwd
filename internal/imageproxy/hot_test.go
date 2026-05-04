package imageproxy

import (
	"bytes"
	"testing"
	"time"
)

func TestHotTier_PutGetRoundTrip(t *testing.T) {
	h := NewHotTier(1<<20, 100)
	now := time.Now().UTC()
	h.Put("k", []byte("hello"), "image/png", "v1", now)
	got, ct, val, fetched, ok := h.Get("k")
	if !ok {
		t.Fatal("expected hit")
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Errorf("bytes mismatch")
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q", ct)
	}
	if val != "v1" {
		t.Errorf("validator = %q", val)
	}
	if !fetched.Equal(now) {
		t.Errorf("fetchedAt mismatch")
	}
}

func TestHotTier_GetMiss_ReturnsFalse(t *testing.T) {
	h := NewHotTier(1<<20, 100)
	if _, _, _, _, ok := h.Get("never"); ok {
		t.Error("expected miss")
	}
}

func TestHotTier_EvictsLRUByByteCap(t *testing.T) {
	h := NewHotTier(20, 100) // 20 bytes; each entry is 10
	now := time.Now().UTC()
	h.Put("a", []byte("0123456789"), "ct", "v", now.Add(-2*time.Second))
	h.Put("b", []byte("0123456789"), "ct", "v", now.Add(-time.Second))
	h.Put("c", []byte("0123456789"), "ct", "v", now) // forces eviction of "a"

	if _, _, _, _, ok := h.Get("a"); ok {
		t.Error("a should have been evicted")
	}
	if _, _, _, _, ok := h.Get("b"); !ok {
		t.Error("b should still be present")
	}
	if _, _, _, _, ok := h.Get("c"); !ok {
		t.Error("c should still be present")
	}
}

func TestHotTier_EvictsLRUByEntryCap(t *testing.T) {
	h := NewHotTier(1<<20, 2) // generous bytes, but cap of 2 entries
	now := time.Now().UTC()
	h.Put("a", []byte("x"), "ct", "v", now.Add(-2*time.Second))
	h.Put("b", []byte("x"), "ct", "v", now.Add(-time.Second))
	h.Put("c", []byte("x"), "ct", "v", now)

	if _, _, _, _, ok := h.Get("a"); ok {
		t.Error("a should have been evicted (entry cap)")
	}
	entries, _ := h.Stats()
	if entries != 2 {
		t.Errorf("Stats entries = %d, want 2", entries)
	}
}

func TestHotTier_OversizedPutIsNoop(t *testing.T) {
	h := NewHotTier(10, 100)
	h.Put("big", []byte("01234567890123456789"), "ct", "v", time.Now())
	if _, _, _, _, ok := h.Get("big"); ok {
		t.Error("oversized entry should not have been admitted")
	}
}

func TestHotTier_GetUpdatesRecency(t *testing.T) {
	h := NewHotTier(20, 100)
	now := time.Now().UTC()
	h.Put("a", []byte("0123456789"), "ct", "v", now.Add(-2*time.Second))
	h.Put("b", []byte("0123456789"), "ct", "v", now.Add(-time.Second))
	// Touch a — now it's MRU.
	if _, _, _, _, ok := h.Get("a"); !ok {
		t.Fatal("expected a to be present")
	}
	// Insert c → forces eviction of b (now LRU).
	h.Put("c", []byte("0123456789"), "ct", "v", now)
	if _, _, _, _, ok := h.Get("a"); !ok {
		t.Error("a should have survived (touched recently)")
	}
	if _, _, _, _, ok := h.Get("b"); ok {
		t.Error("b should have been evicted")
	}
}

func TestHotTier_PutOverwriteReplacesEntry(t *testing.T) {
	h := NewHotTier(1<<20, 100)
	now := time.Now().UTC()
	h.Put("k", []byte("v1"), "ct", "1", now)
	h.Put("k", []byte("v2"), "ct", "2", now.Add(time.Second))
	got, _, val, _, ok := h.Get("k")
	if !ok || string(got) != "v2" || val != "2" {
		t.Errorf("expected overwritten v2/2, got %q/%s ok=%v", got, val, ok)
	}
	entries, b := h.Stats()
	if entries != 1 || b != 2 {
		t.Errorf("Stats = (%d, %d), want (1, 2)", entries, b)
	}
}
