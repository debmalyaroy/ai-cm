package agent

import (
	"log/slog"
	"sync"
	"time"
)

// resultCacheEntry stores a cached result with its creation time.
type resultCacheEntry struct {
	Data      any
	CreatedAt time.Time
}

// ResultCache is a thread-safe, TTL-based cache for query results.
// Used by Strategist (context queries) and Watchdog (anomaly checks)
// to avoid redundant database calls for slowly-changing data.
type ResultCache struct {
	mu      sync.RWMutex
	entries map[string]resultCacheEntry
	ttl     time.Duration
}

// NewResultCache creates a new result cache with the given TTL.
func NewResultCache(ttl time.Duration) *ResultCache {
	return &ResultCache{
		entries: make(map[string]resultCacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a cached result by key.
// Returns the data and true if found and not expired; nil and false otherwise.
func (c *ResultCache) Get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Since(entry.CreatedAt) > c.ttl {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		slog.Debug("ResultCache: entry expired", "key", key)
		return nil, false
	}

	slog.Debug("ResultCache: cache hit", "key", key)
	return entry.Data, true
}

// Put stores a result in the cache.
func (c *ResultCache) Put(key string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = resultCacheEntry{
		Data:      data,
		CreatedAt: time.Now(),
	}
	slog.Debug("ResultCache: stored entry", "key", key)
}

// Size returns the current number of entries in the cache.
func (c *ResultCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
