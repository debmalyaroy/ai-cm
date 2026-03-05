package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
)

// generateSuggestions asks the LLM for 5 follow-up buttons, then falls back to
// static context-appropriate suggestions if the LLM fails or returns invalid JSON.
// It always returns a non-nil, non-empty slice — callers can send it unconditionally.
func generateSuggestions(agentName, question, response string, llmClient llm.Client) []map[string]interface{} {
	suggestPrompt := prompts.Get("chat_suggestions.md")
	if suggestPrompt != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		snippet := response
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		userText := fmt.Sprintf("User question: %s\n\nAssistant response: %s", question, snippet)

		slog.DebugContext(ctx, "Generating follow-up suggestions", "agent", agentName)
		result, err := llmClient.Generate(ctx, suggestPrompt, userText)
		if err != nil {
			slog.WarnContext(ctx, "generateSuggestions: LLM failed, using fallback", "agent", agentName, "error", err)
		} else if suggestions := parseSuggestionsJSON(result); suggestions != nil {
			return suggestions
		} else {
			slog.WarnContext(ctx, "generateSuggestions: non-JSON response, using fallback", "agent", agentName)
		}
	}

	return fallbackSuggestions(agentName)
}

// parseSuggestionsJSON extracts a JSON array of suggestion objects from raw LLM output.
// Returns nil if the output cannot be parsed as an array of objects or strings.
func parseSuggestionsJSON(raw string) []map[string]interface{} {
	clean := strings.TrimSpace(raw)

	// Robustly extract the JSON array — ignore surrounding prose
	startIdx := strings.Index(clean, "[")
	endIdx := strings.LastIndex(clean, "]")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		clean = clean[startIdx : endIdx+1]
	} else {
		// Strip markdown code fences
		if strings.HasPrefix(clean, "```json") {
			clean = strings.TrimPrefix(clean, "```json")
		} else if strings.HasPrefix(clean, "```") {
			clean = strings.TrimPrefix(clean, "```")
		}
		clean = strings.TrimSuffix(clean, "```")
	}
	clean = strings.TrimSpace(clean)

	var suggestions []map[string]interface{}
	if err := json.Unmarshal([]byte(clean), &suggestions); err == nil {
		return suggestions
	}

	// Fallback: plain string array → wrap as question items
	var questions []string
	if err := json.Unmarshal([]byte(clean), &questions); err == nil {
		mapped := make([]map[string]interface{}, len(questions))
		for i, q := range questions {
			mapped[i] = map[string]interface{}{"label": q, "type": "question", "value": q}
		}
		return mapped
	}

	return nil
}

// fallbackSuggestions returns hard-coded agent-appropriate follow-up buttons.
func fallbackSuggestions(agentName string) []map[string]interface{} {
	switch agentName {
	case "analyst":
		return []map[string]interface{}{
			{"label": "📊 Break down by region", "type": "question", "value": "Break this data down by region"},
			{"label": "📅 Compare last month", "type": "question", "value": "How does this compare to last month?"},
			{"label": "🔍 Top performers", "type": "question", "value": "Show me the top performing products"},
			{"label": "⚡ Spot issues", "type": "question", "value": "Which areas need immediate attention?"},
			{"label": "📋 Generate report", "type": "download", "value": "Generate a detailed report for this data"},
		}
	case "strategist":
		return []map[string]interface{}{
			{"label": "⚡ Build action plan", "type": "action", "value": "Create an action plan to address the key issues identified"},
			{"label": "📊 Show the data", "type": "question", "value": "Show me the underlying data supporting this insight"},
			{"label": "📅 3-month trend", "type": "question", "value": "What is the trend over the last 3 months?"},
			{"label": "🔔 Create an alert", "type": "action", "value": "Create an alert to monitor this metric"},
			{"label": "📧 Brief the team", "type": "email", "value": "Draft an email to the team summarising this insight"},
		}
	case "planner":
		return []map[string]interface{}{
			{"label": "📋 View pending actions", "type": "question", "value": "Show all pending actions"},
			{"label": "✅ Prioritise actions", "type": "question", "value": "Which of these actions should I approve first?"},
			{"label": "📊 Expected impact", "type": "question", "value": "What is the expected impact of these actions?"},
			{"label": "📅 Recommended timeline", "type": "question", "value": "What is the recommended timeline for these actions?"},
			{"label": "📧 Share with team", "type": "email", "value": "Draft a summary of proposed actions for the team"},
		}
	case "liaison":
		return []map[string]interface{}{
			{"label": "📧 Send via email", "type": "email", "value": "Help me send this communication via email"},
			{"label": "📋 Add to report", "type": "download", "value": "Add this to the weekly performance report"},
			{"label": "🔄 Adjust tone", "type": "question", "value": "Can you make this communication more formal?"},
			{"label": "📊 Add data points", "type": "question", "value": "Add relevant metrics and data to this communication"},
			{"label": "✅ Follow-up plan", "type": "action", "value": "What follow-up actions should I take after sending this?"},
		}
	case "watchdog":
		return []map[string]interface{}{
			{"label": "🔍 Drill into anomaly", "type": "question", "value": "Tell me more about the anomaly detected"},
			{"label": "📊 Historical baseline", "type": "question", "value": "What is the historical baseline for these metrics?"},
			{"label": "⚡ Recommended fix", "type": "action", "value": "What actions should I take to resolve this alert?"},
			{"label": "📧 Alert the team", "type": "email", "value": "Draft an alert email for the relevant team"},
			{"label": "📅 Recent trend", "type": "question", "value": "Show me the trend for the last 7 days"},
		}
	default: // supervisor, general
		return []map[string]interface{}{
			{"label": "📊 Analyse sales data", "type": "question", "value": "Show me the latest sales performance data"},
			{"label": "🔍 Top insights", "type": "question", "value": "What are the top insights from this month's data?"},
			{"label": "⚡ Propose actions", "type": "action", "value": "What are the top actions I should take this week?"},
			{"label": "📈 Sales trend", "type": "question", "value": "Show me sales trends for the last 3 months"},
			{"label": "📧 Brief the team", "type": "email", "value": "Draft a brief performance update for the category team"},
		}
	}
}
