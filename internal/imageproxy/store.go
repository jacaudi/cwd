package imageproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DiskStore is a filesystem-backed LRU cache for image bytes + metadata.
// Layout under root:
//
//	<root>/<hash[0:2]>/<hash[2:]>.bin   raw image bytes
//	<root>/<hash[0:2]>/<hash[2:]>.json  metadata (contentType, validator, fetchedAt, originalKey)
//	<root>/manifest.json                 full registry of (key -> entry); rewritten on every Put.
//
// Atomicity: every write is to <name>.tmp followed by os.Rename. Orphan .tmp
// files are removed on construction.
type DiskStore struct {
	root     string
	maxBytes int64
	logger   *slog.Logger

	mu      sync.Mutex
	entries map[string]*diskEntry // key -> entry
	bytes   int64
}

type diskEntry struct {
	Key         string    `json:"key"`
	ContentType string    `json:"contentType"`
	Validator   string    `json:"validator"`
	FetchedAt   time.Time `json:"fetchedAt"`
	BytesLen    int64     `json:"bytesLen"`
	LastAccess  time.Time `json:"lastAccess"`
}

// NewDiskStore opens (creating if necessary) a disk store rooted at dir.
// maxBytes is the LRU cap; <= 0 disables the cap (use with care).
func NewDiskStore(dir string, maxBytes int64, logger *slog.Logger) (*DiskStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("imageproxy.store: mkdir %q: %w", dir, err)
	}
	s := &DiskStore{
		root:     dir,
		maxBytes: maxBytes,
		logger:   logger,
		entries:  map[string]*diskEntry{},
	}
	s.loadManifest()
	s.cleanOrphans()
	return s, nil
}

// Get returns bytes + metadata for the key, or os.ErrNotExist.
// Updates lastAccess on hit (re-marks LRU recency) before returning.
func (s *DiskStore) Get(key string) (bytes []byte, contentType, validator string, fetchedAt time.Time, err error) {
	s.mu.Lock()
	e, ok := s.entries[key]
	s.mu.Unlock()
	if !ok {
		return nil, "", "", time.Time{}, os.ErrNotExist
	}
	binPath, _ := s.paths(key)
	b, err := os.ReadFile(binPath)
	if err != nil {
		return nil, "", "", time.Time{}, err
	}
	s.mu.Lock()
	e.LastAccess = time.Now().UTC()
	s.mu.Unlock()
	s.writeManifest() // best-effort
	return b, e.ContentType, e.Validator, e.FetchedAt, nil
}

// Put writes bytes + metadata atomically; evicts LRU entries if maxBytes is exceeded.
func (s *DiskStore) Put(key string, body []byte, contentType, validator string, fetchedAt time.Time) error {
	binPath, jsonPath := s.paths(key)
	if err := os.MkdirAll(filepath.Dir(binPath), 0o700); err != nil {
		return fmt.Errorf("imageproxy.store: mkdir: %w", err)
	}
	if err := atomicWrite(binPath, body); err != nil {
		return fmt.Errorf("imageproxy.store: write bin: %w", err)
	}
	meta := diskEntry{
		Key:         key,
		ContentType: contentType,
		Validator:   validator,
		FetchedAt:   fetchedAt.UTC(),
		BytesLen:    int64(len(body)),
		LastAccess:  time.Now().UTC(),
	}
	mb, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("imageproxy.store: marshal meta: %w", err)
	}
	if err := atomicWrite(jsonPath, mb); err != nil {
		return fmt.Errorf("imageproxy.store: write meta: %w", err)
	}

	s.mu.Lock()
	if prev, ok := s.entries[key]; ok {
		s.bytes -= prev.BytesLen
	}
	s.entries[key] = &meta
	s.bytes += meta.BytesLen
	s.mu.Unlock()

	s.evictIfOverCap()
	s.writeManifest()
	return nil
}

// Stats returns current usage for /api/sources surfacing.
func (s *DiskStore) Stats() (entries int, bytesUsed int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries), s.bytes
}

// paths returns (binPath, jsonPath) for the given key.
func (s *DiskStore) paths(key string) (string, string) {
	sum := sha256.Sum256([]byte(key))
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(s.root, hash[:2])
	base := hash[2:]
	return filepath.Join(dir, base+".bin"), filepath.Join(dir, base+".json")
}

func (s *DiskStore) loadManifest() {
	mPath := filepath.Join(s.root, "manifest.json")
	b, err := os.ReadFile(mPath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		s.logger.Warn("imageproxy.store.manifest_read", "err", err.Error())
		return
	}
	var entries map[string]*diskEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		s.logger.Warn("imageproxy.store.manifest_parse", "err", err.Error())
		return
	}
	for k, e := range entries {
		// Validate against the actual on-disk pair.
		binPath, jsonPath := s.paths(k)
		bi, err1 := os.Stat(binPath)
		_, err2 := os.Stat(jsonPath)
		if err1 != nil || err2 != nil {
			s.logger.Warn("imageproxy.store.manifest_orphan", "key", k)
			continue
		}
		e.BytesLen = bi.Size()
		s.entries[k] = e
		s.bytes += e.BytesLen
	}
}

func (s *DiskStore) writeManifest() {
	s.mu.Lock()
	snapshot := make(map[string]diskEntry, len(s.entries))
	for k, e := range s.entries {
		snapshot[k] = *e
	}
	s.mu.Unlock()
	b, err := json.Marshal(snapshot)
	if err != nil {
		s.logger.Warn("imageproxy.store.manifest_marshal", "err", err.Error())
		return
	}
	if err := atomicWrite(filepath.Join(s.root, "manifest.json"), b); err != nil {
		s.logger.Warn("imageproxy.store.manifest_write", "err", err.Error())
	}
}

// cleanOrphans removes any *.tmp files left behind by torn writes from a
// previous process. Called once at construction.
func (s *DiskStore) cleanOrphans() {
	_ = filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".tmp" {
			_ = os.Remove(p)
		}
		return nil
	})
}

func (s *DiskStore) evictIfOverCap() {
	if s.maxBytes <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bytes <= s.maxBytes {
		return
	}
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return s.entries[keys[i]].LastAccess.Before(s.entries[keys[j]].LastAccess)
	})
	for _, k := range keys {
		if s.bytes <= s.maxBytes {
			return
		}
		e := s.entries[k]
		binPath, jsonPath := s.paths(k)
		_ = os.Remove(binPath)
		_ = os.Remove(jsonPath)
		s.bytes -= e.BytesLen
		delete(s.entries, k)
		s.logger.Debug("imageproxy.store.evict", "key", k, "bytes_freed", e.BytesLen)
	}
}

// atomicWrite writes data to path via a sibling .tmp + rename. Removes the
// .tmp on error so a follow-up cleanOrphans pass can sweep it later if needed.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
