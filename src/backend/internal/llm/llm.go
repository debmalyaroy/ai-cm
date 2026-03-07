package llm

import (
	"context"
	"fmt"
	"os"
	"strconv"

	cfgpkg "github.com/debmalyaroy/ai-cm/internal/config"
)

// Client is the interface that all LLM providers must implement.
type Client interface {
	// Generate sends a prompt and returns a complete response.
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// GenerateStream sends a prompt and returns a channel of response chunks.
	GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error)

	// WithModel returns a new client instance configured to use the specified model.
	// This allows agent-specific model routing for cost/performance optimization.
	WithModel(model string) Client

	// WithMaxTokens returns a new client instance that caps output to n tokens.
	// Use this for calls needing short responses (e.g., intent classification, suggestions).
	// Pass 0 to use the provider/model default.
	WithMaxTokens(n int) Client

	// Name returns the provider name.
	Name() string
}

// StreamChunk represents a single chunk from a streaming response.
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// Embedder is an optional interface for providers that support text-to-vector embedding.
// Check for support with: embedder, ok := client.(llm.Embedder)
type Embedder interface {
	// Embed converts text to a float32 vector for semantic similarity search.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// temperature reads from env or returns the config default.
func temperature(cfg *cfgpkg.Config) float32 {
	if v := os.Getenv("LLM_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			return float32(f)
		}
	}
	if cfg != nil {
		return cfg.LLM.Temperature
	}
	return 0.0
}

// NewClient creates an LLM client based on the provider name from config.
func NewClient(cfg *cfgpkg.Config) (Client, error) {
	temp := temperature(cfg)
	switch cfg.LLM.Provider {
	case "gemini":
		return NewGeminiClient(temp)
	case "openai":
		return NewOpenAIClient(temp)
	case "aws":
		return NewBedrockClient(cfg)
	case "local":
		return NewLocalClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (choose 'gemini', 'openai', 'aws', or 'local')", cfg.LLM.Provider)
	}
}
