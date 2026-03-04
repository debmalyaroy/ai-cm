package memory

import (
	"context"
	"time"
)

// MemoryType classifies the type of memory being stored/retrieved.
type MemoryType string

const (
	// MemoryTypeSTM is Short-Term Memory — current session chat history.
	MemoryTypeSTM MemoryType = "stm"
	// MemoryTypeEpisodic is Long-Term Episodic Memory — past query/response pairs.
	MemoryTypeEpisodic MemoryType = "episodic"
	// MemoryTypeSemantic is Long-Term Semantic Memory — business rules/context.
	MemoryTypeSemantic MemoryType = "semantic"
)

// Episode represents a stored past interaction (Episodic LTM).
type Episode struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Query     string    `json:"query"`
	Response  string    `json:"response"`
	AgentName string    `json:"agent_name"`
	Score     float64   `json:"score,omitempty"` // similarity score
	CreatedAt time.Time `json:"created_at"`
}

// Fact represents a business knowledge item (Semantic LTM).
type Fact struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Content   string    `json:"content"`
	Score     float64   `json:"score,omitempty"` // similarity score
	CreatedAt time.Time `json:"created_at"`
}

// ChatMessage represents a single message in session history (STM).
type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryContext is the unified context built from all 3 tiers.
// It is injected into agent Input for context-aware processing.
type MemoryContext struct {
	// STM: recent session messages
	SessionHistory []ChatMessage `json:"session_history,omitempty"`
	// Episodic LTM: similar past interactions
	SimilarEpisodes []Episode `json:"similar_episodes,omitempty"`
	// Semantic LTM: relevant business facts
	RelevantFacts []Fact `json:"relevant_facts,omitempty"`
}

// Manager is the interface for the 3-tier memory system.
// It follows the design from plan/design.md §5.
type Manager interface {
	// --- STM: Session-scoped chat history ---

	// GetSessionHistory retrieves the last N messages for a session.
	GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]ChatMessage, error)

	// AddMessage stores a new message in the session history.
	AddMessage(ctx context.Context, sessionID, role, content string) error

	// --- Episodic LTM: Past experiences ---

	// StoreEpisode saves a query+response pair with its vector embedding.
	StoreEpisode(ctx context.Context, sessionID, query, response, agentName string) error

	// RetrieveSimilarEpisodes finds past interactions similar to the given query.
	RetrieveSimilarEpisodes(ctx context.Context, query string, limit int) ([]Episode, error)

	// --- Semantic LTM: Business knowledge ---

	// StoreFact stores a business knowledge item with its embedding.
	StoreFact(ctx context.Context, category, content string) error

	// RetrieveRelevantFacts finds business facts relevant to the given query.
	RetrieveRelevantFacts(ctx context.Context, query string, limit int) ([]Fact, error)

	// --- Unified context builder ---

	// BuildContext assembles an enriched context from all 3 memory tiers.
	BuildContext(ctx context.Context, sessionID, query string) (*MemoryContext, error)
}
