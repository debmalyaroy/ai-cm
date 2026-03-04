package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	cfgpkg "github.com/debmalyaroy/ai-cm/internal/config"
)

type BedrockClient struct {
	client *bedrockruntime.Client
	model  string
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
	Temperature      float64         `json:"temperature,omitempty"`
}

type ClaudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// NewBedrockClient creates a new Bedrock client utilizing AWS SDK Go v2
func NewBedrockClient(cfg *cfgpkg.Config) (*BedrockClient, error) {
	// Assumes standard AWS credentials resolution (Env Vars, Profile, EC2 IAM)
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
		client: client,
		model:  model,
	}, nil
}

func (b *BedrockClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := ClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
		System:           systemPrompt,
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: 0.0,
	}

	payload, err := json.Marshal(req)
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

	req := ClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
		System:           systemPrompt,
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: 0.0,
	}

	payload, err := json.Marshal(req)
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

func (b *BedrockClient) Name() string {
	return "aws_bedrock"
}

func (b *BedrockClient) WithModel(model string) Client {
	if model == "" {
		return b
	}
	return &BedrockClient{
		client: b.client,
		model:  model,
	}
}
