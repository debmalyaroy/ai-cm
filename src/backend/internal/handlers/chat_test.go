package handlers

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestChatRequestBinding(t *testing.T) {
	t.Run("valid request unmarshals", func(t *testing.T) {
		data := `{"message": "test query", "session_id": "abc-123"}`
		var req ChatRequest
		err := json.Unmarshal([]byte(data), &req)
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if req.Message != "test query" {
			t.Errorf("message = %q, want 'test query'", req.Message)
		}
		if req.SessionID != "abc-123" {
			t.Errorf("session_id = %q, want 'abc-123'", req.SessionID)
		}
	})

	t.Run("missing message", func(t *testing.T) {
		data := `{"session_id": "abc-123"}`
		var req ChatRequest
		err := json.Unmarshal([]byte(data), &req)
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if req.Message != "" {
			t.Errorf("message should be empty, got %q", req.Message)
		}
	})

	t.Run("optional session_id", func(t *testing.T) {
		data := `{"message": "hello"}`
		var req ChatRequest
		err := json.Unmarshal([]byte(data), &req)
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if req.SessionID != "" {
			t.Errorf("session_id should be empty, got %q", req.SessionID)
		}
	})
}

func TestSendSSE(t *testing.T) {
	t.Run("writes correct SSE format", func(t *testing.T) {
		var buf bytes.Buffer
		sendSSE(&buf, "test_event", map[string]string{"key": "value"})
		output := buf.String()
		if !bytes.Contains([]byte(output), []byte("event: test_event")) {
			t.Errorf("should contain event name, got: %s", output)
		}
		if !bytes.Contains([]byte(output), []byte("data:")) {
			t.Errorf("should contain data field, got: %s", output)
		}
	})

	t.Run("handles string data", func(t *testing.T) {
		var buf bytes.Buffer
		sendSSE(&buf, "msg", "hello world")
		output := buf.String()
		if !bytes.Contains([]byte(output), []byte("hello world")) {
			t.Errorf("should contain string data, got: %s", output)
		}
	})

	t.Run("handles nil data", func(t *testing.T) {
		var buf bytes.Buffer
		sendSSE(&buf, "empty", nil)
		output := buf.String()
		if !bytes.Contains([]byte(output), []byte("event: empty")) {
			t.Errorf("should handle nil data, got: %s", output)
		}
	})
}
