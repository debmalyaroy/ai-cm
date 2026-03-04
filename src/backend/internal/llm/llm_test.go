package llm

import (
	"testing"

	cfgpkg "github.com/debmalyaroy/ai-cm/internal/config"
)

func TestNewClient_Gemini(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	cfg := &cfgpkg.Config{LLM: cfgpkg.LLMConfig{Provider: "gemini"}}
	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping gemini client creation (needs real API key): %v", err)
	}
	if client.Name() != "gemini" {
		t.Errorf("name = %q, want 'gemini'", client.Name())
	}
}

func TestNewClient_OpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg := &cfgpkg.Config{LLM: cfgpkg.LLMConfig{Provider: "openai"}}
	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping openai client creation (needs real API key): %v", err)
	}
	if client.Name() != "openai" {
		t.Errorf("name = %q, want 'openai'", client.Name())
	}
}

func TestNewClient_Unsupported(t *testing.T) {
	cfg := &cfgpkg.Config{LLM: cfgpkg.LLMConfig{Provider: "unsupported_provider"}}
	_, err := NewClient(cfg)
	if err == nil {
		t.Fatal("should return error for unsupported provider")
	}
}

func TestStreamChunkStruct(t *testing.T) {
	chunk := StreamChunk{
		Content: "test",
		Done:    true,
		Error:   nil,
	}
	if chunk.Content != "test" {
		t.Error("content mismatch")
	}
	if !chunk.Done {
		t.Error("should be done")
	}
	if chunk.Error != nil {
		t.Error("should have no error")
	}
}
