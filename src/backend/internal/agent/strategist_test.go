package agent

import (
	"context"
	"testing"
)

func TestNewStrategistAgent(t *testing.T) {
	s := NewStrategistAgent(nil, nil, nil)
	if s == nil {
		t.Fatal("constructor should return non-nil")
	}
	if s.Name() != "strategist" {
		t.Errorf("name = %q, want 'strategist'", s.Name())
	}
}

// emptyToolSet creates a ToolSet with no tools registered.
// gatherContext checks `if !ok { return data }` after Get, so an empty
// ToolSet causes gatherContext to return immediately with empty data —
// no DB calls, no panics.
func emptyToolSet() *ToolSet {
	return &ToolSet{tools: make(map[string]Tool)}
}

// TestStrategistProcess_EmptyTools verifies that when the ToolSet has no
// registered tools, gatherContext returns empty data immediately (no SQL
// calls), and the LLM Generate call is still invoked.
func TestStrategistProcess_EmptyTools(t *testing.T) {
	mock := &mockLLM{response: "Strategic insight: margins are healthy."}
	s := NewStrategistAgent(mock, nil, emptyToolSet())

	input := &Input{
		Query:     "Why did margins drop in East India?",
		SessionID: "sess-001",
	}

	output, err := s.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if output == nil {
		t.Fatal("Process returned nil output")
	}
	if output.Response != mock.response {
		t.Errorf("response = %q, want %q", output.Response, mock.response)
	}
	if output.AgentName != "strategist" {
		t.Errorf("AgentName = %q, want 'strategist'", output.AgentName)
	}
}

// TestStrategistProcess_LLMError verifies that an LLM error is propagated.
func TestStrategistProcess_LLMError(t *testing.T) {
	badLLM := &mockLLM{err: &testStrategistError{"LLM timeout"}}
	s := NewStrategistAgent(badLLM, nil, emptyToolSet())

	_, err := s.Process(context.Background(), &Input{Query: "any query"})
	if err == nil {
		t.Fatal("expected error from LLM failure, got nil")
	}
}

// TestStrategistProcess_WithAnalystContext verifies that analyst_data in
// input.Context is appended to the user prompt (no panic).
func TestStrategistProcess_WithAnalystContext(t *testing.T) {
	mock := &mockLLM{response: "Insight with analyst data."}
	s := NewStrategistAgent(mock, nil, emptyToolSet())

	input := &Input{
		Query: "Analyse sales drop",
		Context: map[string]any{
			"analyst_data": map[string]any{"rows": []any{"row1", "row2"}, "row_count": 2},
		},
	}

	output, err := s.Process(context.Background(), input)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if output.Response != mock.response {
		t.Errorf("response = %q, want %q", output.Response, mock.response)
	}
}

// testStrategistError is a local error type used only in strategist tests.
type testStrategistError struct{ msg string }

func (e *testStrategistError) Error() string { return e.msg }
