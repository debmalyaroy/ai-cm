package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/memory"
)

// --- truncate ---

func TestTruncate_WithinLimit(t *testing.T) {
	s := "short string"
	got := truncate(s, 300)
	if got != s {
		t.Errorf("truncate(%q, 300) = %q, want %q", s, got, s)
	}
}

func TestTruncate_AtLimit(t *testing.T) {
	s := strings.Repeat("a", 300)
	got := truncate(s, 300)
	if got != s {
		t.Errorf("truncate at-limit should return the string unchanged")
	}
}

func TestTruncate_ExceedsLimit(t *testing.T) {
	s := strings.Repeat("a", 301)
	got := truncate(s, 300)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncate result should end with ellipsis, got %q", got)
	}
	// The ellipsis is a multi-byte rune (3 bytes in UTF-8) so len(got) > 300
	// but the prefix before the ellipsis must be exactly 300 bytes.
	prefix := strings.TrimSuffix(got, "…")
	if len(prefix) != 300 {
		t.Errorf("truncated prefix length = %d, want 300", len(prefix))
	}
}

func TestTruncate_EmptyString(t *testing.T) {
	got := truncate("", 300)
	if got != "" {
		t.Errorf("truncate(\"\", 300) = %q, want \"\"", got)
	}
}

func TestTruncate_ZeroMax(t *testing.T) {
	got := truncate("hello", 0)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncate with zero max should add ellipsis, got %q", got)
	}
}

// --- formatMemoryContext ---

func TestFormatMemoryContext_Nil(t *testing.T) {
	got := formatMemoryContext(nil)
	if got != "" {
		t.Errorf("formatMemoryContext(nil) = %q, want \"\"", got)
	}
}

func TestFormatMemoryContext_Empty(t *testing.T) {
	mc := &memory.MemoryContext{}
	got := formatMemoryContext(mc)
	if got != "" {
		t.Errorf("formatMemoryContext(empty) = %q, want \"\"", got)
	}
}

func TestFormatMemoryContext_EmptySlices(t *testing.T) {
	mc := &memory.MemoryContext{
		SessionHistory:  []memory.ChatMessage{},
		SimilarEpisodes: []memory.Episode{},
		RelevantFacts:   []memory.Fact{},
	}
	got := formatMemoryContext(mc)
	if got != "" {
		t.Errorf("formatMemoryContext(empty slices) = %q, want \"\"", got)
	}
}

func TestFormatMemoryContext_SessionHistoryOnly(t *testing.T) {
	mc := &memory.MemoryContext{
		SessionHistory: []memory.ChatMessage{
			{Role: "user", Content: "what are the sales?", CreatedAt: time.Now()},
			{Role: "assistant", Content: "Sales are good", CreatedAt: time.Now()},
		},
	}
	got := formatMemoryContext(mc)

	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "Conversation Context") {
		t.Error("output missing 'Conversation Context' header")
	}
	if !strings.Contains(got, "Recent Session History") {
		t.Error("output missing 'Recent Session History' section")
	}
	if !strings.Contains(got, "user:") {
		t.Error("output missing 'user:' role label")
	}
	if !strings.Contains(got, "assistant:") {
		t.Error("output missing 'assistant:' role label")
	}
	if !strings.Contains(got, "what are the sales?") {
		t.Error("output missing message content")
	}
	if !strings.Contains(got, "--- End Context ---") {
		t.Error("output missing end marker")
	}
	// Episodes/Facts sections should NOT appear
	if strings.Contains(got, "Similar Past Interactions") {
		t.Error("output should not contain 'Similar Past Interactions' when episodes empty")
	}
	if strings.Contains(got, "Relevant Business Facts") {
		t.Error("output should not contain 'Relevant Business Facts' when facts empty")
	}
}

func TestFormatMemoryContext_EpisodesOnly(t *testing.T) {
	mc := &memory.MemoryContext{
		SimilarEpisodes: []memory.Episode{
			{
				Query:    "top selling products?",
				Response: "Pampers is the top seller",
				Score:    0.95,
			},
		},
	}
	got := formatMemoryContext(mc)

	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "Similar Past Interactions") {
		t.Error("output missing 'Similar Past Interactions' section")
	}
	if !strings.Contains(got, "Q: top selling products?") {
		t.Error("output missing episode query")
	}
	if !strings.Contains(got, "A: Pampers is the top seller") {
		t.Error("output missing episode response")
	}
	if !strings.Contains(got, "--- End Context ---") {
		t.Error("output missing end marker")
	}
}

func TestFormatMemoryContext_FactsOnly(t *testing.T) {
	mc := &memory.MemoryContext{
		RelevantFacts: []memory.Fact{
			{Category: "pricing", Content: "MRP for diapers is 499", Score: 0.88},
		},
	}
	got := formatMemoryContext(mc)

	if got == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(got, "Relevant Business Facts") {
		t.Error("output missing 'Relevant Business Facts' section")
	}
	if !strings.Contains(got, "- pricing:") {
		t.Error("output missing fact category label")
	}
	if !strings.Contains(got, "MRP for diapers is 499") {
		t.Error("output missing fact content")
	}
	if !strings.Contains(got, "--- End Context ---") {
		t.Error("output missing end marker")
	}
}

func TestFormatMemoryContext_AllThree(t *testing.T) {
	mc := &memory.MemoryContext{
		SessionHistory: []memory.ChatMessage{
			{Role: "user", Content: "hello", CreatedAt: time.Now()},
		},
		SimilarEpisodes: []memory.Episode{
			{Query: "past q", Response: "past a"},
		},
		RelevantFacts: []memory.Fact{
			{Category: "cat1", Content: "some fact"},
		},
	}
	got := formatMemoryContext(mc)

	sections := []string{
		"Conversation Context",
		"Recent Session History",
		"Similar Past Interactions",
		"Relevant Business Facts",
		"--- End Context ---",
	}
	for _, sec := range sections {
		if !strings.Contains(got, sec) {
			t.Errorf("output missing section %q", sec)
		}
	}
}

func TestFormatMemoryContext_LongContentTruncated(t *testing.T) {
	longContent := strings.Repeat("x", 400) // exceeds maxContentLen=300
	mc := &memory.MemoryContext{
		SessionHistory: []memory.ChatMessage{
			{Role: "user", Content: longContent, CreatedAt: time.Now()},
		},
	}
	got := formatMemoryContext(mc)

	// The full 400-char string should NOT appear verbatim
	if strings.Contains(got, longContent) {
		t.Error("long content was not truncated")
	}
	// The ellipsis should be present indicating truncation occurred
	if !strings.Contains(got, "…") {
		t.Error("truncated content should contain ellipsis")
	}
}

func TestFormatMemoryContext_StartsAndEndsWithMarkers(t *testing.T) {
	mc := &memory.MemoryContext{
		RelevantFacts: []memory.Fact{
			{Category: "general", Content: "a fact"},
		},
	}
	got := formatMemoryContext(mc)

	if !strings.HasPrefix(got, "\n\n--- Conversation Context ---\n") {
		t.Errorf("output should start with context header, got: %q", got[:min(50, len(got))])
	}
	if !strings.HasSuffix(got, "--- End Context ---") {
		t.Errorf("output should end with end marker")
	}
}

func TestFormatMemoryContext_MultipleEpisodes(t *testing.T) {
	mc := &memory.MemoryContext{
		SimilarEpisodes: []memory.Episode{
			{Query: "query one", Response: "response one"},
			{Query: "query two", Response: "response two"},
		},
	}
	got := formatMemoryContext(mc)

	if !strings.Contains(got, "Q: query one") {
		t.Error("missing first episode query")
	}
	if !strings.Contains(got, "Q: query two") {
		t.Error("missing second episode query")
	}
}

func TestFormatMemoryContext_MultipleFacts(t *testing.T) {
	mc := &memory.MemoryContext{
		RelevantFacts: []memory.Fact{
			{Category: "pricing", Content: "fact one"},
			{Category: "inventory", Content: "fact two"},
		},
	}
	got := formatMemoryContext(mc)

	if !strings.Contains(got, "- pricing: fact one") {
		t.Error("missing first fact")
	}
	if !strings.Contains(got, "- inventory: fact two") {
		t.Error("missing second fact")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
