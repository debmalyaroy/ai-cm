package agent

import (
	"context"
	"testing"

	"github.com/debmalyaroy/ai-cm/internal/llm"
)

type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockLLM) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}

func (m *mockLLM) Name() string                      { return "mock" }
func (m *mockLLM) WithModel(model string) llm.Client { return m }

func TestNewSupervisorAgent(t *testing.T) {
	s := NewSupervisorAgent(&mockLLM{response: "test"}, nil, nil)
	if s == nil {
		t.Fatal("constructor should return non-nil")
	}
	if s.Name() != "supervisor" {
		t.Errorf("name = %q, want 'supervisor'", s.Name())
	}
}

func TestSupervisorClassifyIntent(t *testing.T) {
	s := &SupervisorAgent{}

	tests := []struct {
		query string
		want  IntentType
	}{
		{"hello", IntentGeneral},
		{"hi there", IntentGeneral},
		{"what can you do", IntentGeneral},
		{"show sales data", IntentQuery},
		{"what is the revenue", IntentQuery},
		{"why did sales drop", IntentInsight},
		{"explain the trend", IntentInsight},
		{"analyze margin changes", IntentInsight},
		{"recommend an action", IntentPlan},
		{"what should I do", IntentPlan},
		{"propose a plan", IntentPlan},
		{"draft an email to seller", IntentCommunicate},
		{"send compliance notice", IntentCommunicate},
		{"check for anomalies", IntentMonitor},
		{"system health status", IntentMonitor},
		{"any alerts today", IntentMonitor},
	}

	for _, tc := range tests {
		got := s.classifyIntent(context.Background(), tc.query)
		if got != tc.want {
			t.Errorf("classifyIntent(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestIntentTypes(t *testing.T) {
	types := map[IntentType]string{
		IntentQuery:       "query",
		IntentInsight:     "insight",
		IntentPlan:        "plan",
		IntentCommunicate: "communicate",
		IntentMonitor:     "monitor",
		IntentGeneral:     "general",
	}

	for it, expected := range types {
		if string(it) != expected {
			t.Errorf("IntentType %v = %q, want %q", it, string(it), expected)
		}
	}
}
