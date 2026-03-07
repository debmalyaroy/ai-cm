package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cfgpkg "github.com/debmalyaroy/ai-cm/internal/config"
)

type LocalClient struct {
	url       string
	model     string
	client    *http.Client
	maxTokens int // 0 = unlimited (Ollama default)
}

type LocalRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system,omitempty"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options"`
}

type LocalResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewLocalClient(cfg *cfgpkg.Config) (*LocalClient, error) {
	url := cfg.LLM.LocalURL
	if url == "" {
		url = "http://localhost:11434/api/generate" // default to ollama
	}

	model := cfg.LLM.LocalModel
	if model == "" {
		model = "llama3"
	}

	return &LocalClient{
		url:    url,
		model:  model,
		client: &http.Client{Timeout: 300 * time.Second},
	}, nil
}

func (l *LocalClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	opts := map[string]any{"temperature": 0.0}
	if l.maxTokens > 0 {
		opts["num_predict"] = l.maxTokens
	}
	reqBody := LocalRequest{
		Model:   l.model,
		Prompt:  userPrompt,
		System:  systemPrompt,
		Stream:  false,
		Options: opts,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.url, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("local LLM failed: %s %s", resp.Status, string(body))
	}

	var res LocalResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.Response, nil
}

func (l *LocalClient) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	streamOpts := map[string]any{"temperature": 0.0}
	if l.maxTokens > 0 {
		streamOpts["num_predict"] = l.maxTokens
	}
	reqBody := LocalRequest{
		Model:   l.model,
		Prompt:  userPrompt,
		System:  systemPrompt,
		Stream:  true,
		Options: streamOpts,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("local LLM stream failed: %s %s", resp.Status, string(body))
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					ch <- StreamChunk{Error: err}
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var res LocalResponse
			if err := json.Unmarshal([]byte(line), &res); err != nil {
				continue
			}

			ch <- StreamChunk{Content: res.Response, Done: res.Done}
			if res.Done {
				break
			}
		}
	}()

	return ch, nil
}

// Embed implements the llm.Embedder interface using Ollama's /api/embeddings endpoint.
// The embedding dimension depends on the model (e.g. nomic-embed-text=768, mxbai-embed-large=1024).
// NOTE: The DB schema uses vector(1536). If using a local embedding model, run the migration:
//
//	ALTER TABLE agent_memory ALTER COLUMN embedding TYPE vector(<dim>);
//	ALTER TABLE business_context ALTER COLUMN embedding TYPE vector(<dim>);
func (l *LocalClient) Embed(ctx context.Context, text string) ([]float32, error) {
	embedURL := strings.Replace(l.url, "/api/generate", "/api/embeddings", 1)

	type embedRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	type embedResponse struct {
		Embedding []float32 `json:"embedding"`
	}

	payload, err := json.Marshal(embedRequest{Model: l.model, Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("local embed marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", embedURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("local embed failed: %s %s", resp.Status, string(body))
	}

	var res embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("local embed decode: %w", err)
	}
	return res.Embedding, nil
}

func (l *LocalClient) Name() string {
	return "local"
}

func (l *LocalClient) WithModel(model string) Client {
	// If the config tries to override a local instance with a cloud provider ID, ignore it.
	if model == "" || strings.Contains(model, "claude") || strings.Contains(model, "gpt") || strings.Contains(model, "gemini") {
		return l
	}
	return &LocalClient{
		url:       l.url,
		model:     model,
		client:    l.client,
		maxTokens: l.maxTokens,
	}
}

func (l *LocalClient) WithMaxTokens(n int) Client {
	c := *l
	c.maxTokens = n
	return &c
}
