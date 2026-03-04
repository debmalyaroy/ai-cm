package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
)

// LiaisonAgent handles communication with sellers, generates reports,
// and manages notifications. Design §5.
type LiaisonAgent struct {
	llm llm.Client
}

// NewLiaisonAgent creates a new liaison agent.
func NewLiaisonAgent(llmClient llm.Client) *LiaisonAgent {
	return &LiaisonAgent{llm: llmClient}
}

func (l *LiaisonAgent) Name() string { return "liaison" }

func (l *LiaisonAgent) Process(ctx context.Context, input *Input) (*Output, error) {
	output := &Output{AgentName: l.Name()}

	commType := classifyCommunicationType(input.Query)

	slog.DebugContext(ctx, "Liaison: classified comm type", "type", commType, "query", input.Query)

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type: "thought", Content: fmt.Sprintf("Classified communication type: %s", commType),
	})

	systemPrompt := l.buildSystemPrompt(commType)

	userPrompt := fmt.Sprintf("User request: %s", input.Query)
	if input.Context != nil {
		if data, ok := input.Context["data"]; ok {
			userPrompt += fmt.Sprintf("\n\nContext data:\n%v", data)
		}
	}
	userPrompt += formatMemoryContext(input.MemoryContext)

	slog.DebugContext(ctx, "Liaison: calling LLM", "comm_type", commType)
	resp, err := l.llm.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("liaison LLM generate: %w", err)
	}

	output.Response = resp
	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type: "action", Content: fmt.Sprintf("Generated %s draft", commType),
	})

	slog.InfoContext(ctx, "Liaison: successfully generated communication draft", "comm_type", commType)
	return output, nil
}

// CommunicationType classifies the type of communication requested.
type CommunicationType string

const (
	CommTypeComplianceAlert   CommunicationType = "compliance_alert"
	CommTypePerformanceReport CommunicationType = "performance_report"
	CommTypeSellerFeedback    CommunicationType = "seller_feedback"
	CommTypeExecutiveSummary  CommunicationType = "executive_summary"
	CommTypeGeneral           CommunicationType = "general"
)

func classifyCommunicationType(query string) CommunicationType {
	q := toLower(query)
	switch {
	case containsAny(q, "compliance", "violation", "policy"):
		return CommTypeComplianceAlert
	case containsAny(q, "report", "summary", "overview"):
		return CommTypePerformanceReport
	case containsAny(q, "feedback", "review", "seller performance"):
		return CommTypeSellerFeedback
	case containsAny(q, "executive", "leadership", "board"):
		return CommTypeExecutiveSummary
	default:
		return CommTypeGeneral
	}
}

func (l *LiaisonAgent) buildSystemPrompt(ct CommunicationType) string {
	base := `You are a Communication Specialist for an AI Category Manager platform.
You draft professional communications for category managers.`

	switch ct {
	case CommTypeComplianceAlert:
		return base + `
Draft a compliance alert email to a seller. Include:
- Subject line
- Specific violation details
- Required corrective actions
- Deadline for response
Format as a professional email in markdown.`

	case CommTypeSellerFeedback:
		return prompts.Get("liaison_email.md")
	case CommTypePerformanceReport:
		return prompts.Get("liaison_report.md")
	case CommTypeExecutiveSummary:
		return prompts.Get("liaison_slack.md")
	default:
		return prompts.Get("liaison_email.md")
	}
}

// toLower converts a string to lowercase without importing strings.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// containsAny checks if the string contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
