package agent

import (
	"context"

	"github.com/debmalyaroy/ai-cm/internal/memory"
)

// IntentType classifies the user's request for routing.
type IntentType string

const (
	IntentQuery       IntentType = "query"       // Data retrieval
	IntentInsight     IntentType = "insight"     // Why analysis
	IntentPlan        IntentType = "plan"        // Action proposals
	IntentCommunicate IntentType = "communicate" // Seller comms
	IntentMonitor     IntentType = "monitor"     // System health
	IntentGeneral     IntentType = "general"     // General chat
)

// Agent is the interface that all agents must implement.
type Agent interface {
	// Process handles a user input and returns the agent's response.
	Process(ctx context.Context, input *Input) (*Output, error)

	// Name returns the agent's identifier.
	Name() string
}

// Input represents the input to an agent.
type Input struct {
	Query         string                `json:"query"`
	SessionID     string                `json:"session_id"`
	Context       map[string]any        `json:"context,omitempty"`
	History       []Message             `json:"history,omitempty"`
	MemoryContext *memory.MemoryContext `json:"memory_context,omitempty"`
}

// Output represents the result from an agent.
type Output struct {
	Response        string             `json:"response"`
	Data            any                `json:"data,omitempty"`
	AgentName       string             `json:"agent_name"`
	Reasoning       []ReasoningStep    `json:"reasoning,omitempty"`
	Actions         []ActionSuggestion `json:"actions,omitempty"`
	ConfidenceScore float64            `json:"confidence_score,omitempty"`
	DataSource      string             `json:"data_source,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ReasoningStep captures one step in the agent's thought process.
type ReasoningStep struct {
	Type    string `json:"type"` // "thought", "action", "observation"
	Content string `json:"content"`
}

// ActionSuggestion represents a recommended action.
type ActionSuggestion struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	ActionType     string  `json:"action_type"`
	Confidence     float64 `json:"confidence"`
	Priority       string  `json:"priority,omitempty"`        // "high", "medium", "low"
	ExpectedImpact string  `json:"expected_impact,omitempty"` // e.g. "+₹1.2L revenue / month"
}
