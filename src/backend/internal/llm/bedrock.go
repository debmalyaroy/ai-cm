package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	cfgpkg "github.com/debmalyaroy/ai-cm/internal/config"
)

type BedrockClient struct {
	client      *bedrockruntime.Client
	model       string
	temperature float64
	maxTokens   int
}

// LlamaRequest represents the Meta Llama API format
type LlamaRequest struct {
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	MaxGenLen   int     `json:"max_gen_len"`
}

type LlamaResponse struct {
	Generation string `json:"generation"`
}

// ClaudeMessage represents the Anthropic Messages API format used by Bedrock
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeRequest struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	System           string          `json:"system,omitempty"`
	Messages         []ClaudeMessage `json:"messages"`
	// Temperature must NOT use omitempty — 0.0 is a valid value and omitting it
	// would cause Claude to use its default (1.0), producing non-deterministic SQL.
	Temperature float64 `json:"temperature"`
}

type ClaudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// NewBedrockClient creates a new Bedrock client utilizing AWS SDK Go v2
func NewBedrockClient(cfg *cfgpkg.Config) (*BedrockClient, error) {
	// Assumes standard AWS credentials resolution (Env Vars, Profile, EC2 IAM role)
	awscfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(cfg.LLM.AWSRegion))
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %v", err)
	}

	client := bedrockruntime.NewFromConfig(awscfg)

	// Default to haiku or what's assigned in the active configuration
	model := cfg.LLM.AWSModel
	if model == "" {
		model = "anthropic.claude-3-haiku-20240307-v1:0"
	}

	return &BedrockClient{
		client:      client,
		model:       model,
		temperature: float64(cfg.LLM.Temperature),
		maxTokens:   4096,
	}, nil
}

func (b *BedrockClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var payload []byte
	var err error

	genMaxTok := b.maxTokens
	if genMaxTok <= 0 {
		genMaxTok = 4096
	}

	if strings.Contains(strings.ToLower(b.model), "llama") {
		prompt := fmt.Sprintf("<|begin_of_text|><|start_header_id|>system<|end_header_id|>\n\n%s<|eot_id|><|start_header_id|>user<|end_header_id|>\n\n%s<|eot_id|><|start_header_id|>assistant<|end_header_id|>", systemPrompt, userPrompt)
		req := LlamaRequest{
			Prompt:      prompt,
			Temperature: b.temperature,
			MaxGenLen:   genMaxTok,
		}
		payload, err = json.Marshal(req)
	} else {
		req := ClaudeRequest{
			AnthropicVersion: "bedrock-2023-05-31",
			MaxTokens:        genMaxTok,
			System:           systemPrompt,
			Messages: []ClaudeMessage{
				{
					Role:    "user",
					Content: userPrompt,
				},
			},
			Temperature: b.temperature,
		}
		payload, err = json.Marshal(req)
	}

	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	output, err := b.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(b.model),
		ContentType: aws.String("application/json"),
		Body:        payload,
	})
	if err != nil {
		return "", fmt.Errorf("bedrock invoke failed: %v", err)
	}

	if strings.Contains(strings.ToLower(b.model), "llama") {
		var res LlamaResponse
		if err := json.Unmarshal(output.Body, &res); err != nil {
			return "", fmt.Errorf("failed to unmarshal Llama response: %v", err)
		}
		return res.Generation, nil
	}

	var res ClaudeResponse
	if err := json.Unmarshal(output.Body, &res); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(res.Content) == 0 {
		return "", errors.New("bedrock returned empty content array")
	}

	return res.Content[0].Text, nil
}

func (b *BedrockClient) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	var payload []byte
	var err error

	isLlama := strings.Contains(strings.ToLower(b.model), "llama")

	streamMaxTok := b.maxTokens
	if streamMaxTok <= 0 {
		streamMaxTok = 4096
	}

	if isLlama {
		prompt := fmt.Sprintf("<|begin_of_text|><|start_header_id|>system<|end_header_id|>\n\n%s<|eot_id|><|start_header_id|>user<|end_header_id|>\n\n%s<|eot_id|><|start_header_id|>assistant<|end_header_id|>", systemPrompt, userPrompt)
		req := LlamaRequest{
			Prompt:      prompt,
			Temperature: b.temperature,
			MaxGenLen:   streamMaxTok,
		}
		payload, err = json.Marshal(req)
	} else {
		req := ClaudeRequest{
			AnthropicVersion: "bedrock-2023-05-31",
			MaxTokens:        streamMaxTok,
			System:           systemPrompt,
			Messages: []ClaudeMessage{
				{
					Role:    "user",
					Content: userPrompt,
				},
			},
			Temperature: b.temperature,
		}
		payload, err = json.Marshal(req)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	output, err := b.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(b.model),
		ContentType: aws.String("application/json"),
		Body:        payload,
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock invoke stream failed: %v", err)
	}

	go func() {
		defer close(ch)

		stream := output.GetStream()
		defer stream.Close()

		for event := range stream.Events() {
			switch e := event.(type) {
			case *types.ResponseStreamMemberChunk:
				if isLlama {
					var chunk struct {
						Generation string `json:"generation"`
					}
					if err := json.Unmarshal(e.Value.Bytes, &chunk); err != nil {
						slog.ErrorContext(ctx, "Bedrock Llama stream decode error", "error", err)
						continue
					}
					if chunk.Generation != "" {
						ch <- StreamChunk{Content: chunk.Generation, Done: false}
					}
				} else {
					var chunk struct {
						Type  string `json:"type"`
						Delta struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"delta"`
					}

					if err := json.Unmarshal(e.Value.Bytes, &chunk); err != nil {
						slog.ErrorContext(ctx, "Bedrock stream decode error", "error", err)
						continue
					}

					if chunk.Type == "content_block_delta" && chunk.Delta.Type == "text_delta" {
						ch <- StreamChunk{Content: chunk.Delta.Text, Done: false}
					}
				}
			case *types.UnknownUnionMember:
				slog.WarnContext(ctx, "Unknown stream event type", "event_type", event)
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("bedrock stream error: %v", err)}
		} else {
			ch <- StreamChunk{Done: true}
		}
	}()

	return ch, nil
}

// Embed implements the llm.Embedder interface using Amazon Titan Embed Text v1.
// Titan v1 produces 1536-dimensional vectors — matches the vector(1536) DB schema.
func (b *BedrockClient) Embed(ctx context.Context, text string) ([]float32, error) {
	type titanEmbedRequest struct {
		InputText string `json:"inputText"`
	}
	type titanEmbedResponse struct {
		Embedding []float32 `json:"embedding"`
	}

	payload, err := json.Marshal(titanEmbedRequest{InputText: text})
	if err != nil {
		return nil, fmt.Errorf("titan embed marshal: %w", err)
	}

	out, err := b.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("amazon.titan-embed-text-v1"),
		ContentType: aws.String("application/json"),
		Body:        payload,
	})
	if err != nil {
		return nil, fmt.Errorf("titan embed invoke: %w", err)
	}

	var res titanEmbedResponse
	if err := json.Unmarshal(out.Body, &res); err != nil {
		return nil, fmt.Errorf("titan embed unmarshal: %w", err)
	}
	return res.Embedding, nil
}

func (b *BedrockClient) Name() string {
	return "aws_bedrock"
}

func (b *BedrockClient) WithModel(model string) Client {
	if model == "" {
		return b
	}
	return &BedrockClient{
		client:      b.client,
		model:       model,
		temperature: b.temperature,
		maxTokens:   b.maxTokens,
	}
}

func (b *BedrockClient) WithMaxTokens(n int) Client {
	c := *b
	c.maxTokens = n
	return &c
}
