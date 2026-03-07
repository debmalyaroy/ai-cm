package llm

import (
	"context"
	"fmt"
	"io"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIClient implements the Client interface for OpenAI.
type OpenAIClient struct {
	client      *openai.Client
	model       string
	temperature float32
	maxTokens   int // 0 = not set (API uses model default)
}

// NewOpenAIClient creates a new OpenAI LLM client.
func NewOpenAIClient(temperature float32) (*OpenAIClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	client := openai.NewClient(apiKey)

	return &OpenAIClient{
		client:      client,
		model:       model,
		temperature: temperature,
	}, nil
}

func (o *OpenAIClient) Name() string {
	return "openai"
}

func (o *OpenAIClient) WithModel(model string) Client {
	if model == "" {
		return o
	}
	return &OpenAIClient{
		client:      o.client,
		model:       model,
		temperature: o.temperature,
		maxTokens:   o.maxTokens,
	}
}

func (o *OpenAIClient) WithMaxTokens(n int) Client {
	c := *o
	c.maxTokens = n
	return &c
}

func (o *OpenAIClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.0,
		TopP:        0.95,
	}
	if o.maxTokens > 0 {
		req.MaxTokens = o.maxTokens
	}
	resp, err := o.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("openai generate: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIClient) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	streamReq := openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.0,
		TopP:        0.95,
		Stream:      true,
	}
	if o.maxTokens > 0 {
		streamReq.MaxTokens = o.maxTokens
	}
	stream, err := o.client.CreateChatCompletionStream(ctx, streamReq)
	if err != nil {
		return nil, fmt.Errorf("openai stream: %w", err)
	}

	ch := make(chan StreamChunk, 10)
	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				ch <- StreamChunk{Done: true}
				return
			}
			if err != nil {
				ch <- StreamChunk{Error: err, Done: true}
				return
			}

			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta.Content
				if delta != "" {
					ch <- StreamChunk{Content: delta}
				}
			}
		}
	}()

	return ch, nil
}
