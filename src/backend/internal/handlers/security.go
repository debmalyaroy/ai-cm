package handlers

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig holds configurable rate limit parameters.
type RateLimiterConfig struct {
	RequestsPerMinute int
	Enabled           bool
}

// tokenBucket implements a simple per-IP token bucket rate limiter.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func (tb *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimiter returns middleware that limits requests per IP.
func RateLimiter(cfg RateLimiterConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	var mu sync.Mutex
	buckets := make(map[string]*tokenBucket)

	maxTokens := float64(cfg.RequestsPerMinute)
	refillRate := maxTokens / 60.0

	// Cleanup old buckets periodically
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			for ip, tb := range buckets {
				if time.Since(tb.lastRefill) > 10*time.Minute {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		tb, exists := buckets[ip]
		if !exists {
			tb = &tokenBucket{
				tokens:     maxTokens,
				maxTokens:  maxTokens,
				refillRate: refillRate,
				lastRefill: time.Now(),
			}
			buckets[ip] = tb
		}
		allowed := tb.allow()
		mu.Unlock()

		if !allowed {
			slog.WarnContext(c.Request.Context(), "rate limit exceeded", "ip", ip)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again in a moment",
			})
			return
		}
		c.Next()
	}
}

// APIKeyAuth returns middleware requiring a valid API key in the Authorization header.
// If no keys are configured, the middleware is disabled (allowing all requests).
func APIKeyAuth(validKeys []string) gin.HandlerFunc {
	if len(validKeys) == 0 {
		return func(c *gin.Context) { c.Next() }
	}

	keySet := make(map[string]bool, len(validKeys))
	for _, k := range validKeys {
		if k != "" {
			keySet[k] = true
		}
	}

	// If all keys were empty strings, disable
	if len(keySet) == 0 {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		key := c.GetHeader("Authorization")
		// Support "Bearer <key>" format
		if len(key) > 7 && key[:7] == "Bearer " {
			key = key[7:]
		}

		if !keySet[key] {
			slog.WarnContext(c.Request.Context(), "unauthorized API access attempt",
				"ip", c.ClientIP(), "path", c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or missing API key",
			})
			return
		}
		c.Next()
	}
}
