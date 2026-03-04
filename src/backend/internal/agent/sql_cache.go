package agent

import (
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// sqlCacheEntry stores a cached SQL template with its creation time.
type sqlCacheEntry struct {
	SQL       string
	CreatedAt time.Time
}

// SQLCache is a thread-safe, TTL-based, LRU cache for generated SQL templates.
// It maps normalized query fingerprints to their generated SQL strings.
type SQLCache struct {
	mu         sync.RWMutex
	entries    map[string]sqlCacheEntry
	ttl        time.Duration
	maxEntries int
}

// NewSQLCache creates a new SQL cache with the given TTL and max entries.
func NewSQLCache(ttl time.Duration, maxEntries int) *SQLCache {
	return &SQLCache{
		entries:    make(map[string]sqlCacheEntry),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// Get retrieves a cached SQL template for the given user query.
// Returns the SQL string and true if found and not expired; empty string and false otherwise.
func (c *SQLCache) Get(query string) (string, bool) {
	key := normalizeQuery(query)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return "", false
	}

	if time.Since(entry.CreatedAt) > c.ttl {
		// Expired — remove lazily
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		slog.Debug("SQLCache: entry expired", "key", key)
		return "", false
	}

	slog.Debug("SQLCache: cache hit", "key", key)
	return entry.SQL, true
}

// Put stores a SQL template for the given user query.
// If the cache exceeds maxEntries, the oldest entry is evicted.
func (c *SQLCache) Put(query, sql string) {
	key := normalizeQuery(query)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[key] = sqlCacheEntry{
		SQL:       sql,
		CreatedAt: time.Now(),
	}
	slog.Debug("SQLCache: stored entry", "key", key, "cache_size", len(c.entries))
}

// Size returns the current number of entries in the cache.
func (c *SQLCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictOldest removes the oldest entry from the cache. Must be called with mu held.
func (c *SQLCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range c.entries {
		if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		slog.Debug("SQLCache: evicted oldest entry", "key", oldestKey)
	}
}

// normalizeQuery produces a cache key from a user query by:
// 1. Lowercasing
// 2. Removing common stop words
// 3. Sorting remaining tokens alphabetically
// This ensures semantically similar queries like "Check inventory levels for Avg Margin"
// and "check avg margin for inventory levels" produce the same key.
func normalizeQuery(query string) string {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "for": true, "of": true,
		"in": true, "on": true, "at": true, "to": true, "by": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"and": true, "or": true, "me": true, "my": true, "i": true,
		"show": true, "give": true, "get": true, "what": true, "how": true,
		"tell": true, "please": true, "can": true, "you": true, "do": true,
	}

	lower := strings.ToLower(query)
	words := strings.Fields(lower)

	var tokens []string
	for _, w := range words {
		// Strip punctuation
		cleaned := strings.Trim(w, ".,!?;:'\"()[]{}")
		if cleaned == "" || stopWords[cleaned] {
			continue
		}
		tokens = append(tokens, cleaned)
	}

	sort.Strings(tokens)
	return strings.Join(tokens, ",")
}
