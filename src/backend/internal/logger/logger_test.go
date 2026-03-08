package logger

import (
	"context"
	"testing"
)

func TestWithRequestID_And_RequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	got := RequestID(ctx)
	if got != "req-123" {
		t.Errorf("RequestID = %q, want 'req-123'", got)
	}
}

func TestRequestID_MissingFromContext(t *testing.T) {
	got := RequestID(context.Background())
	if got != "" {
		t.Errorf("RequestID without value = %q, want ''", got)
	}
}

func TestInit_DoesNotPanic(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "unknown"} {
		Init(level)
	}
}
