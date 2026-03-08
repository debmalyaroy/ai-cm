package memory

import (
	"testing"
	"time"
)

// --- Types tests ---

func TestMemoryTypes(t *testing.T) {
	if MemoryTypeSTM != "stm" {
		t.Errorf("STM = %q, want 'stm'", MemoryTypeSTM)
	}
	if MemoryTypeEpisodic != "episodic" {
		t.Errorf("Episodic = %q, want 'episodic'", MemoryTypeEpisodic)
	}
	if MemoryTypeSemantic != "semantic" {
		t.Errorf("Semantic = %q, want 'semantic'", MemoryTypeSemantic)
	}
}

func TestEpisodeStruct(t *testing.T) {
	e := Episode{
		ID:        "ep1",
		SessionID: "session-1",
		Query:     "test query",
		Response:  "test response",
		AgentName: "analyst",
		Score:     0.95,
		CreatedAt: time.Now(),
	}
	if e.ID != "ep1" {
		t.Errorf("ID = %q", e.ID)
	}
	if e.AgentName != "analyst" {
		t.Errorf("AgentName = %q", e.AgentName)
	}
}

func TestFactStruct(t *testing.T) {
	f := Fact{
		ID:        "f1",
		Category:  "pricing",
		Content:   "Max discount 20%",
		Score:     0.88,
		CreatedAt: time.Now(),
	}
	if f.Category != "pricing" {
		t.Errorf("Category = %q", f.Category)
	}
}

func TestChatMessageStruct(t *testing.T) {
	m := ChatMessage{
		Role:      "user",
		Content:   "hello",
		CreatedAt: time.Now(),
	}
	if m.Role != "user" {
		t.Errorf("Role = %q", m.Role)
	}
}

func TestMemoryContextStruct(t *testing.T) {
	mc := MemoryContext{
		SessionHistory: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		SimilarEpisodes: []Episode{
			{ID: "e1", Query: "q", Response: "r", AgentName: "analyst"},
		},
		RelevantFacts: []Fact{
			{ID: "f1", Category: "policy", Content: "rule"},
		},
	}
	if len(mc.SessionHistory) != 1 {
		t.Errorf("session history len = %d", len(mc.SessionHistory))
	}
	if len(mc.SimilarEpisodes) != 1 {
		t.Errorf("episodes len = %d", len(mc.SimilarEpisodes))
	}
	if len(mc.RelevantFacts) != 1 {
		t.Errorf("facts len = %d", len(mc.RelevantFacts))
	}
}

// --- Fake embedding tests ---

func TestGenerateFakeEmbedding(t *testing.T) {
	emb := generateFakeEmbedding("hello world", 10)
	if emb == "" {
		t.Fatal("should not be empty")
	}
	if emb[0] != '[' || emb[len(emb)-1] != ']' {
		t.Errorf("should be bracket-delimited, got: %.20s...", emb)
	}

	// Deterministic: same input → same output
	emb2 := generateFakeEmbedding("hello world", 10)
	if emb != emb2 {
		t.Error("should be deterministic")
	}

	// Different inputs → different outputs
	emb3 := generateFakeEmbedding("goodbye world", 10)
	if emb == emb3 {
		t.Error("different inputs should produce different embeddings")
	}

	// Full-size embedding
	emb1536 := generateFakeEmbedding("test", 1536)
	if emb1536[0] != '[' {
		t.Error("1536-dim should start with [")
	}
}

func TestGetEmbedding(t *testing.T) {
	store := &PgStore{}
	emb, err := store.getEmbedding(t.Context(), "test query")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if emb == "" {
		t.Fatal("embedding should not be empty")
	}
	if len(emb) < 100 {
		t.Errorf("embedding seems too short: %d chars", len(emb))
	}
}

func TestNewPgStore(t *testing.T) {
	store := NewPgStore(nil, nil)
	if store == nil {
		t.Fatal("should not be nil")
	}
	if store.db != nil {
		t.Error("db should be nil when passed nil")
	}
}

func TestFloat32SliceToVector(t *testing.T) {
	tests := []struct {
		name string
		in   []float32
		want string
	}{
		{"empty", []float32{}, "[]"},
		{"single", []float32{0.5}, "[0.500000]"},
		{"multiple", []float32{0.1, 0.2, 0.3}, "[0.100000,0.200000,0.300000]"},
		{"negative", []float32{-1.0, 0.0, 1.0}, "[-1.000000,0.000000,1.000000]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := float32SliceToVector(tc.in)
			if got != tc.want {
				t.Errorf("float32SliceToVector(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFloat32SliceToVector_StartsAndEndsWithBrackets(t *testing.T) {
	got := float32SliceToVector([]float32{1.0, 2.0})
	if got[0] != '[' || got[len(got)-1] != ']' {
		t.Errorf("vector should be bracket-delimited; got %q", got)
	}
}
