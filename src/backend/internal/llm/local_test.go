package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	cfgpkg "github.com/debmalyaroy/ai-cm/internal/config"
)

// newTestLocalClient creates a LocalClient pointed at the given test server URL.
func newTestLocalClient(serverURL string) *LocalClient {
	return &LocalClient{
		url:    serverURL + "/api/generate",
		model:  "test-model",
		client: http.DefaultClient,
	}
}

// --- Name ---

func TestLocalClient_Name(t *testing.T) {
	c := &LocalClient{}
	if c.Name() != "local" {
		t.Errorf("Name() = %q, want 'local'", c.Name())
	}
}

// --- NewLocalClient ---

func TestNewLocalClient_Defaults(t *testing.T) {
	cfg := &cfgpkg.Config{}
	c, err := NewLocalClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.url != "http://localhost:11434/api/generate" {
		t.Errorf("default url = %q", c.url)
	}
	if c.model != "llama3" {
		t.Errorf("default model = %q", c.model)
	}
}

func TestNewLocalClient_CustomValues(t *testing.T) {
	cfg := &cfgpkg.Config{}
	cfg.LLM.LocalURL = "http://myollama:11434/api/generate"
	cfg.LLM.LocalModel = "mistral"
	c, err := NewLocalClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.url != cfg.LLM.LocalURL {
		t.Errorf("url = %q, want %q", c.url, cfg.LLM.LocalURL)
	}
	if c.model != "mistral" {
		t.Errorf("model = %q, want 'mistral'", c.model)
	}
}

// --- WithModel ---

func TestLocalClient_WithModel_ValidModel(t *testing.T) {
	c := &LocalClient{url: "http://x", model: "original", client: http.DefaultClient}
	c2 := c.WithModel("llama3.2")
	lc, ok := c2.(*LocalClient)
	if !ok {
		t.Fatal("WithModel should return *LocalClient")
	}
	if lc.model != "llama3.2" {
		t.Errorf("model = %q, want 'llama3.2'", lc.model)
	}
	if lc.url != c.url {
		t.Errorf("url should be inherited; got %q", lc.url)
	}
}

func TestLocalClient_WithModel_CloudIDIgnored(t *testing.T) {
	c := &LocalClient{url: "http://x", model: "original"}
	for _, cloudID := range []string{"claude-3-5-sonnet", "gpt-4o", "gemini-pro", ""} {
		got := c.WithModel(cloudID)
		if got != Client(c) {
			t.Errorf("WithModel(%q) should return same instance for cloud/empty ID", cloudID)
		}
	}
}

// --- WithMaxTokens ---

func TestLocalClient_WithMaxTokens(t *testing.T) {
	c := &LocalClient{model: "m", url: "u"}
	c2 := c.WithMaxTokens(512)
	lc, ok := c2.(*LocalClient)
	if !ok {
		t.Fatal("WithMaxTokens should return *LocalClient")
	}
	if lc.maxTokens != 512 {
		t.Errorf("maxTokens = %d, want 512", lc.maxTokens)
	}
	// original unchanged
	if c.maxTokens != 0 {
		t.Error("original client should be unchanged")
	}
}

// --- Generate ---

func TestLocalClient_Generate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		// Decode request and verify fields.
		var req LocalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if req.Stream {
			t.Error("stream should be false for Generate")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LocalResponse{Response: "generated text", Done: true})
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	got, err := c.Generate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "generated text" {
		t.Errorf("response = %q, want 'generated text'", got)
	}
}

func TestLocalClient_Generate_WithMaxTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LocalRequest
		json.NewDecoder(r.Body).Decode(&req)
		numPredict, ok := req.Options["num_predict"]
		if !ok {
			t.Error("num_predict option should be set when maxTokens > 0")
		}
		// JSON numbers decode as float64.
		if numPredict.(float64) != 100 {
			t.Errorf("num_predict = %v, want 100", numPredict)
		}
		json.NewEncoder(w).Encode(LocalResponse{Response: "ok", Done: true})
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	c.maxTokens = 100
	c.Generate(context.Background(), "", "prompt") //nolint:errcheck
}

func TestLocalClient_Generate_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	_, err := c.Generate(context.Background(), "", "q")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestLocalClient_Generate_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not-json{{{") //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	_, err := c.Generate(context.Background(), "", "q")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestLocalClient_Generate_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until client gives up.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := newTestLocalClient(srv.URL)
	_, err := c.Generate(ctx, "", "q")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

// --- GenerateStream ---

func TestLocalClient_GenerateStream_Success(t *testing.T) {
	chunks := []LocalResponse{
		{Response: "Hello", Done: false},
		{Response: " world", Done: false},
		{Response: "!", Done: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LocalRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("stream should be true for GenerateStream")
		}
		enc := json.NewEncoder(w)
		for _, ch := range chunks {
			enc.Encode(ch) //nolint:errcheck
		}
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	ch, err := c.GenerateStream(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []string
	for sc := range ch {
		if sc.Error != nil {
			t.Errorf("unexpected chunk error: %v", sc.Error)
		}
		got = append(got, sc.Content)
		if sc.Done {
			break
		}
	}

	if len(got) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestLocalClient_GenerateStream_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	_, err := c.GenerateStream(context.Background(), "", "q")
	if err == nil {
		t.Fatal("expected error for non-200 stream status")
	}
}

func TestLocalClient_GenerateStream_BadJSONLinesSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send one bad line then a valid done chunk.
		fmt.Fprintln(w, "bad-json-line")
		enc := json.NewEncoder(w)
		enc.Encode(LocalResponse{Response: "final", Done: true}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestLocalClient(srv.URL)
	ch, err := c.GenerateStream(context.Background(), "", "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var received []StreamChunk
	for sc := range ch {
		received = append(received, sc)
	}
	// The bad JSON line is silently skipped; we should receive the valid chunk.
	if len(received) == 0 {
		t.Error("expected at least one valid chunk after bad JSON line")
	}
}

// --- Embed ---

func TestLocalClient_Embed_Success(t *testing.T) {
	want := []float32{0.1, 0.2, 0.3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("path = %q, want '/api/embeddings'", r.URL.Path)
		}
		type embedResp struct {
			Embedding []float32 `json:"embedding"`
		}
		json.NewEncoder(w).Encode(embedResp{Embedding: want}) //nolint:errcheck
	}))
	defer srv.Close()

	c := &LocalClient{
		url:    srv.URL + "/api/generate",
		model:  "nomic-embed-text",
		client: http.DefaultClient,
	}
	got, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLocalClient_Embed_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := &LocalClient{url: srv.URL + "/api/generate", model: "m", client: http.DefaultClient}
	_, err := c.Embed(context.Background(), "text")
	if err == nil {
		t.Fatal("expected error for non-200 embed status")
	}
}

func TestLocalClient_Embed_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "bad{json") //nolint:errcheck
	}))
	defer srv.Close()

	c := &LocalClient{url: srv.URL + "/api/generate", model: "m", client: http.DefaultClient}
	_, err := c.Embed(context.Background(), "text")
	if err == nil {
		t.Fatal("expected error for malformed embed JSON")
	}
}

// --- temperature helper ---

func TestTemperature_EnvOverride(t *testing.T) {
	t.Setenv("LLM_TEMPERATURE", "0.7")
	got := temperature(nil)
	if got < 0.69 || got > 0.71 {
		t.Errorf("temperature from env = %v, want ~0.7", got)
	}
}

func TestTemperature_FromConfig(t *testing.T) {
	cfg := &cfgpkg.Config{}
	cfg.LLM.Temperature = 0.5
	got := temperature(cfg)
	if got != 0.5 {
		t.Errorf("temperature = %v, want 0.5", got)
	}
}

func TestTemperature_DefaultZero(t *testing.T) {
	got := temperature(nil)
	if got != 0.0 {
		t.Errorf("default temperature = %v, want 0.0", got)
	}
}
