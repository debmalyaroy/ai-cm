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
	url    string
	model  string
	client *http.Client
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
	reqBody := LocalRequest{
		Model:   l.model,
		Prompt:  userPrompt,
		System:  systemPrompt,
		Stream:  false,
		Options: map[string]any{"temperature": 0.0},
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

	reqBody := LocalRequest{
		Model:   l.model,
		Prompt:  userPrompt,
		System:  systemPrompt,
		Stream:  true,
		Options: map[string]any{"temperature": 0.0},
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

func (l *LocalClient) Name() string {
	return "local"
}

func (l *LocalClient) WithModel(model string) Client {
	// If the config tries to override a local instance with a cloud provider ID, ignore it.
	if model == "" || strings.Contains(model, "claude") || strings.Contains(model, "gpt") || strings.Contains(model, "gemini") {
		return l
	}
	return &LocalClient{
		url:    l.url,
		model:  model,
		client: l.client,
	}
}
