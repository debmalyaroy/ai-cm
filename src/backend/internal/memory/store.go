package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/debmalyaroy/ai-cm/internal/llm"
)

// PgStore implements the Manager interface using PostgreSQL + pgvector.
type PgStore struct {
	db  *pgxpool.Pool
	llm llm.Client
}

// NewPgStore creates a new PostgreSQL-backed memory store.
func NewPgStore(db *pgxpool.Pool, llmClient llm.Client) *PgStore {
	return &PgStore{db: db, llm: llmClient}
}

// --- STM: Session-scoped chat history ---

func (s *PgStore) GetSessionHistory(ctx context.Context, sessionID string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(ctx, `
		SELECT role, content, created_at
		FROM chat_messages
		WHERE session_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("get session history: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.Role, &m.Content, &m.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, m)
	}
	// Reverse to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (s *PgStore) AddMessage(ctx context.Context, sessionID, role, content string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO chat_messages (id, session_id, role, content)
		VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), sessionID, role, content)
	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}
	return nil
}

// --- Episodic LTM: Past experiences ---

func (s *PgStore) StoreEpisode(ctx context.Context, sessionID, query, response, agentName string) error {
	embedding, err := s.getEmbedding(ctx, query)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate embedding, storing without vector", "error", err)
		// Store without embedding — still useful for text search
		_, err = s.db.Exec(ctx, `
			INSERT INTO agent_memory (id, agent_type, memory_type, content, metadata)
			VALUES ($1, $2, 'episodic', $3, $4)`,
			uuid.New().String(), agentName,
			fmt.Sprintf("Q: %s\nA: %s", query, response),
			fmt.Sprintf(`{"session_id": "%s"}`, sessionID))
		return err
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO agent_memory (id, agent_type, memory_type, content, embedding, metadata)
		VALUES ($1, $2, 'episodic', $3, $4, $5)`,
		uuid.New().String(), agentName,
		fmt.Sprintf("Q: %s\nA: %s", query, response),
		embedding,
		fmt.Sprintf(`{"session_id": "%s"}`, sessionID))
	return err
}

func (s *PgStore) RetrieveSimilarEpisodes(ctx context.Context, query string, limit int) ([]Episode, error) {
	if limit <= 0 {
		limit = 5
	}

	embedding, err := s.getEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, agent_type, content, 
		       1 - (embedding <=> $1) AS score, created_at
		FROM agent_memory
		WHERE memory_type = 'episodic'
		  AND embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $2`, embedding, limit)
	if err != nil {
		return nil, fmt.Errorf("retrieve episodes: %w", err)
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var e Episode
		if err := rows.Scan(&e.ID, &e.AgentName, &e.Response, &e.Score, &e.CreatedAt); err != nil {
			continue
		}
		// Parse Q/A from content
		parts := strings.SplitN(e.Response, "\nA: ", 2)
		if len(parts) == 2 {
			e.Query = strings.TrimPrefix(parts[0], "Q: ")
			e.Response = parts[1]
		}
		episodes = append(episodes, e)
	}
	return episodes, nil
}

// --- Semantic LTM: Business knowledge ---

func (s *PgStore) StoreFact(ctx context.Context, category, content string) error {
	embedding, err := s.getEmbedding(ctx, content)
	if err != nil {
		slog.ErrorContext(ctx, "failed to embed fact, storing without vector", "error", err)
		_, err = s.db.Exec(ctx, `
			INSERT INTO business_context (id, content, metadata)
			VALUES ($1, $2, $3)`,
			uuid.New().String(), content,
			fmt.Sprintf(`{"category": "%s"}`, category))
		return err
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO business_context (id, content, embedding, metadata)
		VALUES ($1, $2, $3, $4)`,
		uuid.New().String(), content, embedding,
		fmt.Sprintf(`{"category": "%s"}`, category))
	return err
}

func (s *PgStore) RetrieveRelevantFacts(ctx context.Context, query string, limit int) ([]Fact, error) {
	if limit <= 0 {
		limit = 5
	}

	embedding, err := s.getEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, content, metadata->>'category' AS category,
		       1 - (embedding <=> $1) AS score, created_at
		FROM business_context
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $2`, embedding, limit)
	if err != nil {
		return nil, fmt.Errorf("retrieve facts: %w", err)
	}
	defer rows.Close()

	var facts []Fact
	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.ID, &f.Content, &f.Category, &f.Score, &f.CreatedAt); err != nil {
			continue
		}
		facts = append(facts, f)
	}
	return facts, nil
}

// --- Unified context builder ---

func (s *PgStore) BuildContext(ctx context.Context, sessionID, query string) (*MemoryContext, error) {
	mc := &MemoryContext{}

	// 1. STM: recent chat history
	history, err := s.GetSessionHistory(ctx, sessionID, 10)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get session history", "error", err)
	} else {
		mc.SessionHistory = history
	}

	// 2. Episodic LTM: similar past experience
	episodes, err := s.RetrieveSimilarEpisodes(ctx, query, 3)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve episodes", "error", err)
	} else {
		mc.SimilarEpisodes = episodes
	}

	// 3. Semantic LTM: relevant business knowledge
	facts, err := s.RetrieveRelevantFacts(ctx, query, 3)
	if err != nil {
		slog.ErrorContext(ctx, "failed to retrieve facts", "error", err)
	} else {
		mc.RelevantFacts = facts
	}

	return mc, nil
}

// getEmbedding generates a vector embedding for the given text using the LLM.
// For hackathon: uses a simple hash-based fake embedding if LLM doesn't support embeddings.
func (s *PgStore) getEmbedding(ctx context.Context, text string) (string, error) {
	// Generate a deterministic fake embedding from text hash for hackathon prototype.
	// In production, this would call an embedding API (e.g., text-embedding-3-small).
	embedding := generateFakeEmbedding(text, 1536)
	return embedding, nil
}

// generateFakeEmbedding creates a deterministic pseudo-embedding vector from text.
// This is a hackathon shortcut — production would use a real embedding model.
func generateFakeEmbedding(text string, dims int) string {
	values := make([]string, dims)
	hash := uint64(0)
	for i, c := range text {
		hash = hash*31 + uint64(c) + uint64(i)
	}
	for i := 0; i < dims; i++ {
		hash = hash*6364136223846793005 + 1442695040888963407 // LCG
		val := float64(int64(hash>>33)-int64(1<<30)) / float64(1<<30)
		values[i] = fmt.Sprintf("%.6f", val)
	}
	return "[" + strings.Join(values, ",") + "]"
}
