package agent

import (
	"context"
	"testing"

	"github.com/debmalyaroy/ai-cm/internal/prompts"
)

func TestClassifyCommunicationType(t *testing.T) {
	tests := []struct {
		query string
		want  CommunicationType
	}{
		{"Send compliance alert to seller", CommTypeComplianceAlert},
		{"check policy violation", CommTypeComplianceAlert},
		{"generate performance report", CommTypePerformanceReport},
		{"monthly summary for team", CommTypePerformanceReport},
		{"Send feedback to BabyWorld seller", CommTypeSellerFeedback},
		{"Draft review for seller", CommTypeSellerFeedback},
		{"Prepare executive briefing", CommTypeExecutiveSummary},
		{"leadership update needed", CommTypeExecutiveSummary},
		{"send a message", CommTypeGeneral},
		{"hello", CommTypeGeneral},
	}

	for _, tc := range tests {
		got := classifyCommunicationType(tc.query)
		if got != tc.want {
			t.Errorf("classifyCommunicationType(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestToLower(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Hello", "hello"},
		{"UPPER CASE", "upper case"},
		{"already lower", "already lower"},
		{"MiXeD123", "mixed123"},
		{"", ""},
	}
	for _, tc := range tests {
		got := toLower(tc.in)
		if got != tc.want {
			t.Errorf("toLower(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", "world", "foo") {
		t.Error("should contain 'world'")
	}
	if containsAny("hello world", "foo", "bar") {
		t.Error("should not contain 'foo' or 'bar'")
	}
	if containsAny("", "anything") {
		t.Error("empty string should not contain anything")
	}
}

func TestContainsStr(t *testing.T) {
	if !containsStr("hello world", "world") {
		t.Error("should find 'world'")
	}
	if containsStr("hello", "world") {
		t.Error("should not find 'world' in 'hello'")
	}
	if containsStr("hi", "longer") {
		t.Error("should not find longer substring")
	}
}

func TestNewLiaisonAgent(t *testing.T) {
	l := NewLiaisonAgent(nil)
	if l == nil {
		t.Fatal("constructor should return non-nil")
	}
	if l.Name() != "liaison" {
		t.Errorf("name = %q, want 'liaison'", l.Name())
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	// Initialize the prompts module pointing to the actual test prompts directory
	err := prompts.Init("../../../prompts")
	if err != nil {
		t.Logf("Warning: could not init prompts for testing: %v", err)
	}

	l := NewLiaisonAgent(nil)

	types := []CommunicationType{
		CommTypeComplianceAlert,
		CommTypePerformanceReport,
		CommTypeSellerFeedback,
		CommTypeExecutiveSummary,
		CommTypeGeneral,
	}

	for _, ct := range types {
		prompt := l.buildSystemPrompt(ct)
		if len(prompt) < 50 {
			t.Errorf("system prompt for %q is too short (%d chars)", ct, len(prompt))
		}
	}
}

func TestLiaisonAgent_Process(t *testing.T) {
	// Success block
	llmc := &mockLLMClient{
		response: "Dear seller, Please note...",
	}
	agent := NewLiaisonAgent(llmc)

	out, err := agent.Process(context.Background(), &Input{
		Query: "Draft compliance notice",
		Context: map[string]any{
			"data": "violation XYZ",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.AgentName != "liaison" {
		t.Errorf("expected agent name liaison, got %s", out.AgentName)
	}
	if out.Response != llmc.response {
		t.Errorf("expected response %s, got %s", llmc.response, out.Response)
	}

	// Failure block
	llmc.err = context.DeadlineExceeded
	_, err = agent.Process(context.Background(), &Input{Query: "test"})
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
}
