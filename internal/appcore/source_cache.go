package appcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SourceContentCacheValue struct {
	Raw                  string
	ConditionalHit       bool
	PersistentCacheHit   bool
	PersistentCacheWrite bool
	StatusCode           int
	UsedURL              string
}

type SourceContentCache interface {
	Load(key string, load func() (SourceContentCacheValue, error)) (SourceContentCacheValue, bool, error)
}

type memorySourceContentCacheEntry struct {
	ready chan struct{}
	value SourceContentCacheValue
	err   error
}

type MemorySourceContentCache struct {
	mu      sync.Mutex
	entries map[string]*memorySourceContentCacheEntry
}

func NewMemorySourceContentCache() *MemorySourceContentCache {
	return &MemorySourceContentCache{entries: make(map[string]*memorySourceContentCacheEntry)}
}

func (cache *MemorySourceContentCache) Load(key string, load func() (SourceContentCacheValue, error)) (value SourceContentCacheValue, hit bool, err error) {
	key = strings.TrimSpace(key)
	if cache == nil || key == "" {
		value, err = load()
		return value, false, err
	}

	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[string]*memorySourceContentCacheEntry)
	}
	if entry, ok := cache.entries[key]; ok {
		cache.mu.Unlock()
		<-entry.ready
		return entry.value, true, entry.err
	}
	entry := &memorySourceContentCacheEntry{ready: make(chan struct{})}
	cache.entries[key] = entry
	cache.mu.Unlock()

	defer func() {
		if recovered := recover(); recovered != nil {
			entry.err = fmt.Errorf("输入源读取异常：%v", recovered)
		}
		close(entry.ready)
		value = entry.value
		err = entry.err
	}()
	entry.value, entry.err = load()
	return entry.value, false, entry.err
}

type SourceURLCacheEntry struct {
	ContentHash  string `json:"content_hash,omitempty"`
	ETag         string `json:"etag,omitempty"`
	FetchedAt    string `json:"fetched_at,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	Raw          string `json:"raw,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	URL          string `json:"url"`
}

type SourceURLCache interface {
	Get(url string) (SourceURLCacheEntry, bool)
	Put(entry SourceURLCacheEntry) error
	Invalidate(url string) error
}

type fileSourceURLCacheData struct {
	Entries       map[string]SourceURLCacheEntry `json:"entries"`
	SchemaVersion string                         `json:"schema_version"`
}

type FileSourceURLCache struct {
	mu      sync.Mutex
	path    string
	loaded  bool
	entries map[string]SourceURLCacheEntry
}

const sourceURLCacheSchemaVersion = "cfst-gui-source-url-cache-v1"

func NewFileSourceURLCache(path string) *FileSourceURLCache {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return &FileSourceURLCache{path: path}
}

func (cache *FileSourceURLCache) Get(rawURL string) (SourceURLCacheEntry, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if cache == nil || rawURL == "" {
		return SourceURLCacheEntry{}, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.loadLocked()
	entry, ok := cache.entries[rawURL]
	if !ok || strings.TrimSpace(entry.Raw) == "" {
		return SourceURLCacheEntry{}, false
	}
	return entry, true
}

func (cache *FileSourceURLCache) Put(entry SourceURLCacheEntry) error {
	entry.URL = strings.TrimSpace(entry.URL)
	if cache == nil || entry.URL == "" {
		return nil
	}
	entry.ContentHash = sourceContentHash(entry.Raw)
	entry.FetchedAt = time.Now().Format(time.RFC3339)

	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.loadLocked()
	cache.entries[entry.URL] = entry
	return cache.saveLocked()
}

func (cache *FileSourceURLCache) Invalidate(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if cache == nil || rawURL == "" {
		return nil
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.loadLocked()
	delete(cache.entries, rawURL)
	return cache.saveLocked()
}

func (cache *FileSourceURLCache) loadLocked() {
	if cache.loaded {
		return
	}
	cache.loaded = true
	cache.entries = make(map[string]SourceURLCacheEntry)
	raw, err := os.ReadFile(cache.path)
	if err != nil {
		return
	}
	var data fileSourceURLCacheData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	for key, entry := range data.Entries {
		key = strings.TrimSpace(key)
		if key == "" || strings.TrimSpace(entry.Raw) == "" {
			continue
		}
		entry.URL = key
		cache.entries[key] = entry
	}
}

func (cache *FileSourceURLCache) saveLocked() error {
	if cache == nil || strings.TrimSpace(cache.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cache.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(fileSourceURLCacheData{
		Entries:       cache.entries,
		SchemaVersion: sourceURLCacheSchemaVersion,
	}, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(cache.path, raw, 0o600)
}

func SourceContentCacheKey(source Source) string {
	switch SourceKind(source) {
	case "inline":
		return "inline:" + sourceContentHash(strings.TrimSpace(source.Content))
	case "file":
		path := strings.TrimSpace(source.Path)
		if path == "" {
			return ""
		}
		return "file:" + filepath.Clean(path)
	default:
		normalized, err := NormalizeSourceURLInput(source.URL)
		if err != nil {
			return ""
		}
		return "url:" + normalized
	}
}

func sourceContentHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
