package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// Init configures the global slog logger with JSON formatting.
func Init(levelStr string) {
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Configure the handler to include the source file and line
	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	// Wrap with custom handler to extract context
	logger := slog.New(ContextHandler{Handler: handler})
	slog.SetDefault(logger)
}

// WithRequestID injects a request ID into a context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestID retrieves the request ID from a context.
func RequestID(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey).(string); ok {
		return val
	}
	return ""
}

// ContextHandler is a custom slog.Handler that extracts request_id from context.
type ContextHandler struct {
	slog.Handler
}

// Handle adds the request_id attribute to the log record if it exists in the context.
func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	reqID := RequestID(ctx)
	if reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}
