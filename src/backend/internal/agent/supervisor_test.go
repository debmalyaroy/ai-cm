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

func (m *mockLLM) Name() string                           { return "mock" }
func (m *mockLLM) WithModel(model string) llm.Client      { return m }
func (m *mockLLM) WithMaxTokens(n int) llm.Client         { return m }

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
		{"recommend an action", IntentInsight}, // "recommend" matches insightPatterns; "recommend action" (no "an") matches planPatterns
		{"what should I do", IntentPlan},
		{"propose a plan", IntentPlan},
		{"draft an email to seller", IntentCommunicate},
		{"send compliance notice", IntentCommunicate},
		{"check for anomalies", IntentMonitor},
		{"system health status", IntentMonitor},
		{"any alerts today", IntentMonitor},
		// Action/plan creation patterns
		{"Create a replenishment order for Pampers Active Baby Medium 62pc in Kolkata", IntentPlan},
		{"restock MamyPoko diapers in East India", IntentPlan},
		{"schedule a flash sale for underperforming SKUs", IntentPlan},
		{"launch a promotion for baby wipes", IntentPlan},
		{"place an order to replenish inventory", IntentPlan},
		{"adjust pricing for the top 5 diaper brands", IntentPlan},
		// Follow-up context should not corrupt intent
		{"What are the top 3 cities in East India contributing to the underperformance, and what are the sales numbers for each?\n\n[Context from previous response: East India at ₹23Cr is underperforming. Investigate distribution gaps.]", IntentInsight},
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

// TestSupervisorProcess_Greeting verifies that a greeting query is handled
// entirely in-process (no DB calls) and returns the introductory response.
func TestSupervisorProcess_Greeting(t *testing.T) {
	mock := &mockLLM{response: "test"}
	s := NewSupervisorAgent(mock, nil, nil)

	output, err := s.Process(context.Background(), &Input{
		Query:     "hello",
		SessionID: "sess-greeting",
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if output == nil {
		t.Fatal("Process returned nil output")
	}
	if output.Response == "" {
		t.Error("expected non-empty greeting response")
	}
}

// TestSupervisorProcess_NeedsClarification verifies that a too-vague query
// triggers the clarification prompt without hitting any agent or DB.
func TestSupervisorProcess_NeedsClarification(t *testing.T) {
	mock := &mockLLM{response: "insight"}
	s := NewSupervisorAgent(mock, nil, nil)

	output, err := s.Process(context.Background(), &Input{
		Query:     "sales",
		SessionID: "",
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if output == nil {
		t.Fatal("Process returned nil output")
	}
	// The clarification response should ask for more detail.
	if output.Response == "" {
		t.Error("expected clarification response, got empty string")
	}
}

// TestSupervisorProcess_LiaisonRoute verifies that a communicate-intent query
// routes to the LiaisonAgent (which requires no DB).
func TestSupervisorProcess_LiaisonRoute(t *testing.T) {
	mock := &mockLLM{response: "Dear seller, please comply with..."}
	s := NewSupervisorAgent(mock, nil, nil)

	output, err := s.Process(context.Background(), &Input{
		Query:     "draft an email to seller regarding compliance",
		SessionID: "sess-liaison",
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if output == nil {
		t.Fatal("Process returned nil output")
	}
	if output.Response == "" {
		t.Error("expected non-empty liaison response")
	}
}

// TestSupervisorStoreEpisodicMemory_NilMemory verifies that StoreEpisodicMemory
// is a no-op (returns nil) when the supervisor has no memory manager.
func TestSupervisorStoreEpisodicMemory_NilMemory(t *testing.T) {
	mock := &mockLLM{response: "ok"}
	s := NewSupervisorAgent(mock, nil, nil)

	err := s.StoreEpisodicMemory(context.Background(), "sess", "query", "response", "analyst")
	if err != nil {
		t.Errorf("StoreEpisodicMemory with nil memory should return nil, got: %v", err)
	}
}

// TestSupervisorClassifyWithLLM_NoIntentLLM verifies that when intentLLM is nil
// the heuristic path is taken (classifyWithLLM is not called).
func TestSupervisorClassifyIntent_FallsBackToHeuristics(t *testing.T) {
	// Supervisor constructed with no per-agent model map → intentLLM = nil
	mock := &mockLLM{response: "ok"}
	s := NewSupervisorAgent(mock, nil, nil)
	// intentLLM is nil because no "supervisor" key in agentModels
	// classifyIntent should fall through to heuristics directly.
	intent := s.classifyIntent(context.Background(), "show sales by region for last month")
	if intent == "" {
		t.Error("classifyIntent returned empty intent")
	}
}

// TestNeedsClarification covers the needsClarification helper.
func TestNeedsClarification(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"sales", true},
		{"inventory", true},
		{"revenue", true},
		{"show me sales", true},
		{"show me diaper sales this month", false}, // too long
		{"what is the revenue in Q4", false},        // too long (> 20 chars)
		{"hello", false},
		{"performance", true},
	}
	for _, tc := range tests {
		got := needsClarification(tc.query)
		if got != tc.want {
			t.Errorf("needsClarification(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

// TestIsGreeting covers the isGreeting helper.
func TestIsGreeting(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"hello", true},
		{"hi", true},
		{"hey", true},
		{"good morning", true},
		{"what can you do", true},
		{"who are you", true},
		{"show me sales data", false},
		{"why did margins drop", false},
	}
	for _, tc := range tests {
		got := isGreeting(tc.query)
		if got != tc.want {
			t.Errorf("isGreeting(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

// TestIsMonitoringQuery covers the isMonitoringQuery helper.
func TestIsMonitoringQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"check for anomalies", true},
		{"system health check", true},
		{"any new alerts today", true},
		{"watchdog status", true},
		{"why did sales drop in East India", false},
		{"recommend a pricing plan", false},
	}
	for _, tc := range tests {
		got := isMonitoringQuery(tc.query)
		if got != tc.want {
			t.Errorf("isMonitoringQuery(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}
