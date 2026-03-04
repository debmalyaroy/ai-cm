package agent

import (
	"fmt"
	"strings"

	"github.com/debmalyaroy/ai-cm/internal/memory"
)

// maxContentLen is the maximum character length for a single memory item's content
// when included in an LLM prompt. Keeps prompts manageable for smaller local models.
const maxContentLen = 300

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// formatMemoryContext builds a formatted string block from the 3-tier memory context
// to be appended to agent user prompts. Returns "" if mc is nil or all slices empty.
// Content is truncated to prevent prompt overflow on smaller local LLMs.
func formatMemoryContext(mc *memory.MemoryContext) string {
	if mc == nil {
		return ""
	}
	if len(mc.SessionHistory) == 0 && len(mc.SimilarEpisodes) == 0 && len(mc.RelevantFacts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n--- Conversation Context ---\n")

	if len(mc.SessionHistory) > 0 {
		sb.WriteString("[Recent Session History]\n")
		for _, msg := range mc.SessionHistory {
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, truncate(msg.Content, maxContentLen)))
		}
		sb.WriteString("\n")
	}

	if len(mc.SimilarEpisodes) > 0 {
		sb.WriteString("[Similar Past Interactions]\n")
		for _, ep := range mc.SimilarEpisodes {
			sb.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n",
				truncate(ep.Query, maxContentLen),
				truncate(ep.Response, maxContentLen)))
		}
	}

	if len(mc.RelevantFacts) > 0 {
		sb.WriteString("[Relevant Business Facts]\n")
		for _, fact := range mc.RelevantFacts {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", fact.Category, truncate(fact.Content, maxContentLen)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("--- End Context ---")
	return sb.String()
}
