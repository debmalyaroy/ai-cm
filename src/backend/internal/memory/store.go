package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

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

	// Compute the embedding once — reused by both vector similarity queries below.
	embedding, embErr := s.getEmbedding(ctx, query)
	if embErr != nil {
		slog.WarnContext(ctx, "BuildContext: embedding failed, skipping vector queries", "error", embErr)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	// 1. STM: recent chat history (no embedding needed)
	go func() {
		defer wg.Done()
		history, err := s.GetSessionHistory(ctx, sessionID, 10)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get session history", "error", err)
			return
		}
		mu.Lock()
		mc.SessionHistory = history
		mu.Unlock()
	}()

	// 2. Episodic LTM: similar past experiences
	go func() {
		defer wg.Done()
		if embedding == "" {
			return
		}
		episodes, err := s.retrieveEpisodesWithEmbedding(ctx, embedding, 3)
		if err != nil {
			slog.ErrorContext(ctx, "failed to retrieve episodes", "error", err)
			return
		}
		mu.Lock()
		mc.SimilarEpisodes = episodes
		mu.Unlock()
	}()

	// 3. Semantic LTM: relevant business knowledge
	go func() {
		defer wg.Done()
		if embedding == "" {
			return
		}
		facts, err := s.retrieveFactsWithEmbedding(ctx, embedding, 3)
		if err != nil {
			slog.ErrorContext(ctx, "failed to retrieve facts", "error", err)
			return
		}
		mu.Lock()
		mc.RelevantFacts = facts
		mu.Unlock()
	}()

	wg.Wait()
	return mc, nil
}

// retrieveEpisodesWithEmbedding runs the episodic similarity query with a pre-computed embedding.
func (s *PgStore) retrieveEpisodesWithEmbedding(ctx context.Context, embedding string, limit int) ([]Episode, error) {
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
		parts := strings.SplitN(e.Response, "\nA: ", 2)
		if len(parts) == 2 {
			e.Query = strings.TrimPrefix(parts[0], "Q: ")
			e.Response = parts[1]
		}
		episodes = append(episodes, e)
	}
	return episodes, nil
}

// retrieveFactsWithEmbedding runs the semantic fact query with a pre-computed embedding.
func (s *PgStore) retrieveFactsWithEmbedding(ctx context.Context, embedding string, limit int) ([]Fact, error) {
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

// --- SQL Cache: vector-backed SQL template cache ---

// StoreSQL saves a successful query→SQL mapping in agent_memory using its semantic embedding.
// Stored with agent_type='analyst', memory_type='sql_cache' so it doesn't pollute episodic memory.
// Content format: "Q: {queryText}\nSQL: {sqlText}"
func (s *PgStore) StoreSQL(ctx context.Context, queryText, sqlText string) error {
	embedding, err := s.getEmbedding(ctx, queryText)
	if err != nil {
		slog.WarnContext(ctx, "StoreSQL: embedding failed, storing without vector", "error", err)
		_, err = s.db.Exec(ctx, `
			INSERT INTO agent_memory (id, agent_type, memory_type, content, metadata)
			VALUES ($1, 'analyst', 'sql_cache', $2, '{}')
			ON CONFLICT DO NOTHING`,
			uuid.New().String(),
			fmt.Sprintf("Q: %s\nSQL: %s", queryText, sqlText))
		return err
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO agent_memory (id, agent_type, memory_type, content, embedding, metadata)
		VALUES ($1, 'analyst', 'sql_cache', $2, $3, '{}')`,
		uuid.New().String(),
		fmt.Sprintf("Q: %s\nSQL: %s", queryText, sqlText),
		embedding)
	return err
}

// RetrieveSQL finds the semantically closest cached SQL template for a query.
// Returns the cached SQL and true only when cosine similarity >= threshold (recommended: 0.92).
// Only considers entries from the last 24 hours to avoid stale SQL patterns.
func (s *PgStore) RetrieveSQL(ctx context.Context, queryText string, threshold float64) (string, bool, error) {
	embedding, err := s.getEmbedding(ctx, queryText)
	if err != nil || embedding == "" {
		return "", false, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT content, 1 - (embedding <=> $1) AS score
		FROM agent_memory
		WHERE memory_type = 'sql_cache'
		  AND embedding IS NOT NULL
		  AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY embedding <=> $1
		LIMIT 1`, embedding)
	if err != nil {
		return "", false, fmt.Errorf("retrieve sql cache: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return "", false, nil
	}

	var content string
	var score float64
	if err := rows.Scan(&content, &score); err != nil {
		return "", false, err
	}

	if score < threshold {
		slog.DebugContext(ctx, "VectorSQLCache: nearest match below threshold", "score", score, "threshold", threshold)
		return "", false, nil
	}

	// Parse "Q: ...\nSQL: ..." content
	if parts := strings.SplitN(content, "\nSQL: ", 2); len(parts) == 2 {
		slog.DebugContext(ctx, "VectorSQLCache: hit", "score", score)
		return strings.TrimSpace(parts[1]), true, nil
	}
	return "", false, nil
}

// getEmbedding generates a vector embedding for the given text.
// Uses the real embedding API when the LLM client implements llm.Embedder (e.g. Bedrock Titan v1).
// Falls back to a deterministic hash-based fake embedding for providers without embedding support
// (local dev / OpenAI / Gemini). The fake preserves the 1536-dim schema but similarity scores
// will be random — real embeddings are required for meaningful vector search.
func (s *PgStore) getEmbedding(ctx context.Context, text string) (string, error) {
	if embedder, ok := s.llm.(llm.Embedder); ok {
		vec, err := embedder.Embed(ctx, text)
		if err != nil {
			slog.WarnContext(ctx, "real embedding failed, falling back to fake", "error", err)
		} else {
			return float32SliceToVector(vec), nil
		}
	}
	// Fallback: deterministic hash-based fake (no semantic meaning, but consistent dimension)
	return generateFakeEmbedding(text, 1536), nil
}

// float32SliceToVector formats a []float32 as a pgvector literal: "[0.1,0.2,...]"
func float32SliceToVector(vec []float32) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%.6f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// generateFakeEmbedding creates a deterministic pseudo-embedding vector from text.
// Kept as fallback for providers without embedding support. Not semantically meaningful.
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
