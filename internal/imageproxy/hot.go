package imageproxy

import (
	"container/list"
	"sync"
	"time"
)

// HotTier is a bounded in-memory LRU of image bytes + metadata. Bound by both
// maxBytes and maxEntries — an entry is admitted only if it fits within both
// caps after eviction. Oversized entries (alone exceeding maxBytes) are
// silently dropped (no-op Put) since the disk store still has them.
type HotTier struct {
	mu         sync.Mutex
	maxBytes   int64
	maxEntries int
	bytes      int64
	order      *list.List               // front = MRU, back = LRU
	index      map[string]*list.Element // key -> *list.Element wrapping *hotEntry
}

type hotEntry struct {
	key         string
	bytes       []byte
	contentType string
	validator   string
	fetchedAt   time.Time
}

// NewHotTier returns a hot tier capped at the smaller of maxBytes (>=0; 0
// disables the tier) and maxEntries (>=0; 0 disables the tier).
func NewHotTier(maxBytes int64, maxEntries int) *HotTier {
	return &HotTier{
		maxBytes:   maxBytes,
		maxEntries: maxEntries,
		order:      list.New(),
		index:      map[string]*list.Element{},
	}
}

// Get returns bytes + metadata for the key, marking it MRU.
func (h *HotTier) Get(key string) (body []byte, contentType, validator string, fetchedAt time.Time, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	el, hit := h.index[key]
	if !hit {
		return nil, "", "", time.Time{}, false
	}
	h.order.MoveToFront(el)
	e := el.Value.(*hotEntry)
	return e.bytes, e.contentType, e.validator, e.fetchedAt, true
}

// Put inserts or replaces the entry. Oversized entries are dropped.
func (h *HotTier) Put(key string, body []byte, contentType, validator string, fetchedAt time.Time) {
	if h.maxBytes <= 0 || h.maxEntries <= 0 {
		return
	}
	if int64(len(body)) > h.maxBytes {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if el, ok := h.index[key]; ok {
		old := el.Value.(*hotEntry)
		h.bytes -= int64(len(old.bytes))
		h.order.Remove(el)
		delete(h.index, key)
	}
	e := &hotEntry{
		key: key, bytes: body, contentType: contentType,
		validator: validator, fetchedAt: fetchedAt,
	}
	el := h.order.PushFront(e)
	h.index[key] = el
	h.bytes += int64(len(body))
	h.evictLocked()
}

// Stats returns current count + total bytes.
func (h *HotTier) Stats() (entries int, bytesUsed int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.index), h.bytes
}

// Has reports presence without touching recency.
func (h *HotTier) Has(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.index[key]
	return ok
}

// evictLocked drops LRU entries until under both caps. Caller holds h.mu.
func (h *HotTier) evictLocked() {
	for h.bytes > h.maxBytes || len(h.index) > h.maxEntries {
		back := h.order.Back()
		if back == nil {
			return
		}
		e := back.Value.(*hotEntry)
		h.order.Remove(back)
		delete(h.index, e.key)
		h.bytes -= int64(len(e.bytes))
	}
}
