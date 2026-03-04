package handlers

import (
	"log/slog"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestLogger logs each request with method, path, status, and duration.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()[:8]

		// Set on Gin context
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Set on Go context for slog handler
		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		duration := time.Since(start)
		slog.InfoContext(c.Request.Context(), "request handled",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", duration.Milliseconds(),
		)
	}
}

// Recovery recovers from panics and returns a 500 JSON error.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(c.Request.Context(), "panic recovered", "error", err)
				c.AbortWithStatusJSON(500, gin.H{
					"error":   "internal server error",
					"message": "an unexpected error occurred",
				})
			}
		}()
		c.Next()
	}
}
