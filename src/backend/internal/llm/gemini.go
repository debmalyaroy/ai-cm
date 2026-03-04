package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient implements the Client interface for Google Gemini.
type GeminiClient struct {
	client      *genai.Client
	model       string
	temperature float32
}

// NewGeminiClient creates a new Gemini LLM client.
func NewGeminiClient(temperature float32) (*GeminiClient, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("create Gemini client: %w", err)
	}

	return &GeminiClient{
		client:      client,
		model:       model,
		temperature: temperature,
	}, nil
}

func (g *GeminiClient) Name() string {
	return "gemini"
}

func (g *GeminiClient) WithModel(model string) Client {
	if model == "" {
		return g
	}
	return &GeminiClient{
		client:      g.client,
		model:       model,
		temperature: g.temperature,
	}
}

func (g *GeminiClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	model := g.client.GenerativeModel(g.model)
	model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	model.SetTemperature(0.0)
	model.SetTopP(0.95)

	resp, err := model.GenerateContent(ctx, genai.Text(userPrompt))
	if err != nil {
		return "", fmt.Errorf("gemini generate: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}

	result := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	return result, nil
}

func (g *GeminiClient) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	model := g.client.GenerativeModel(g.model)
	model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	model.SetTemperature(0.0)
	model.SetTopP(0.95)

	iter := model.GenerateContentStream(ctx, genai.Text(userPrompt))

	ch := make(chan StreamChunk, 10)
	go func() {
		defer close(ch)
		for {
			resp, err := iter.Next()
			if err != nil {
				// Iterator exhausted
				ch <- StreamChunk{Done: true}
				return
			}

			for _, candidate := range resp.Candidates {
				if candidate.Content == nil {
					continue
				}
				for _, part := range candidate.Content.Parts {
					if text, ok := part.(genai.Text); ok {
						ch <- StreamChunk{Content: string(text)}
					}
				}
			}
		}
	}()

	return ch, nil
}
