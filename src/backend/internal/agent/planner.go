package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
)

// PlannerAgent implements the Human-in-the-Loop pattern from design §4.
// It proposes actions based on insights and stores them for user approval.
type PlannerAgent struct {
	db  *pgxpool.Pool
	llm llm.Client
}

// NewPlannerAgent creates a new planner agent.
func NewPlannerAgent(db *pgxpool.Pool, llmClient llm.Client) *PlannerAgent {
	return &PlannerAgent{db: db, llm: llmClient}
}

func (p *PlannerAgent) Name() string { return "planner" }

func (p *PlannerAgent) Process(ctx context.Context, input *Input) (*Output, error) {
	output := &Output{AgentName: p.Name()}

	// Build prompt for action generation
	systemPrompt := prompts.Get("planner.md")

	userPrompt := fmt.Sprintf("User query: %s", input.Query)
	if input.Context != nil {
		if data, ok := input.Context["data"]; ok {
			userPrompt += fmt.Sprintf("\n\nAvailable data:\n%v", data)
		}
		if insight, ok := input.Context["insight"]; ok {
			userPrompt += fmt.Sprintf("\n\nInsight:\n%v", insight)
		}
	}

	userPrompt += formatMemoryContext(input.MemoryContext)

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type: "thought", Content: "Analyzing query to propose actionable business decisions",
	})

	slog.DebugContext(ctx, "Planner: analyzing query to propose actionable business decisions")
	slog.DebugContext(ctx, "Planner: calling LLM for action proposals", "query", input.Query)

	resp, err := p.llm.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("planner LLM generate: %w", err)
	}

	// Parse and store actions
	actions := parseActions(resp)
	slog.DebugContext(ctx, "Planner: actions parsed", "count", len(actions))
	stored := 0
	for _, a := range actions {
		_, err := p.db.Exec(ctx, `
			INSERT INTO action_log (id, title, description, action_type, category, confidence_score, status)
			VALUES ($1, $2, $3, $4, 'AI Generated', $5, 'pending')`,
			uuid.New().String(), a.Title, a.Description, a.ActionType, a.Confidence)
		if err != nil {
			slog.ErrorContext(ctx, "Planner: failed to store action", "error", err)
			continue
		}
		stored++
	}

	output.Actions = actions
	output.Response = resp
	if stored > 0 {
		output.Response += fmt.Sprintf("\n\n✅ %d action(s) submitted to the approval queue.", stored)
	}

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type: "action", Content: fmt.Sprintf("Proposed %d actions, stored %d for approval", len(actions), stored),
	})

	slog.InfoContext(ctx, "Planner: successfully processed query and generated actions", "proposed", len(actions), "stored", stored)
	return output, nil
}

// parseActions extracts ActionSuggestion items from LLM output.
func parseActions(text string) []ActionSuggestion {
	var actions []ActionSuggestion
	var current ActionSuggestion
	inAction := false

	for _, line := range splitLines(text) {
		if line == "ACTION:" {
			if inAction && current.Title != "" {
				actions = append(actions, current)
			}
			current = ActionSuggestion{}
			inAction = true
			continue
		}
		if line == "---" {
			if inAction && current.Title != "" {
				actions = append(actions, current)
			}
			current = ActionSuggestion{}
			inAction = false
			continue
		}
		if !inAction {
			continue
		}

		if val, ok := trimPrefix(line, "Title:"); ok {
			current.Title = val
		} else if val, ok := trimPrefix(line, "Description:"); ok {
			current.Description = val
		} else if val, ok := trimPrefix(line, "Type:"); ok {
			current.ActionType = val
		} else if val, ok := trimPrefix(line, "Confidence:"); ok {
			_, _ = fmt.Sscanf(val, "%f", &current.Confidence)
		}
	}
	// Capture last action if not terminated by ---
	if inAction && current.Title != "" {
		actions = append(actions, current)
	}
	return actions
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, trimSpace(line))
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, trimSpace(s[start:]))
	}
	return lines
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

func trimPrefix(line, prefix string) (string, bool) {
	if len(line) >= len(prefix) && line[:len(prefix)] == prefix {
		return trimSpace(line[len(prefix):]), true
	}
	return "", false
}
