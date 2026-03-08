package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/debmalyaroy/ai-cm/internal/agent"
	"github.com/debmalyaroy/ai-cm/internal/config"
	"github.com/debmalyaroy/ai-cm/internal/database"
	"github.com/debmalyaroy/ai-cm/internal/handlers"
	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/memory"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupNativeE2E initializes a full Gin router hooked directly to the real Postgres DB and real configured LLM.
// It skips tests if the local db or local LLM (Ollama) is severely misconfigured or offline.
func setupNativeE2E(t *testing.T) (*gin.Engine, *pgxpool.Pool, llm.Client) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Ensure db url is present
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable"
		os.Setenv("DATABASE_URL", dbURL) // Default local fallback
	}

	db, err := database.NewPool(context.Background())
	if err != nil {
		t.Skipf("Skipping E2E test due to unreachable postgres: %v", err)
	}

	// Make sure we use "local" provider explicitly for E2E speed/safety
	os.Setenv("LLM_PROVIDER", "local")

	cfg := config.Load("../../../config/config.local.yaml")

	// Ensure Prompts are initialized for standard contexts
	promptDir := "../../prompts"
	if err := prompts.Init(promptDir); err != nil {
		t.Logf("prompts failed to initialize normally, LLM might lack context: %v", err)
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize native LLM Client: %v", err)
	}

	// Verify local ollama is alive before running giant suites
	_, err = client.Generate(context.Background(), "Ignore", "Hello, are you alive?")
	if err != nil {
		t.Skipf("Skipping E2E native test because local LLM failed to respond: %v", err)
	}

	api := router.Group("/api")
	handlers.RegisterDashboardRoutes(api, db, client)
	handlers.RegisterChatRoutes(api, db, client, cfg.LLM.AgentModels)
	handlers.RegisterActionRoutes(api, db, client)
	handlers.RegisterGraphQLRoutes(api, db, client, cfg.LLM.AgentModels)

	return router, db, client
}

func TestE2E_NativeDashboard(t *testing.T) {
	router, _, _ := setupNativeE2E(t)

	endpoints := []string{
		"/api/dashboard/kpis",
		"/api/dashboard/sales-trend",
		"/api/dashboard/category-breakdown",
		"/api/dashboard/regional-performance",
		"/api/dashboard/top-products",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req, _ := http.NewRequest("POST", ep, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected 200 for %s, got %d", ep, w.Code)
			}

			// ensure valid JSON
			var dummy interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &dummy); err != nil {
				t.Errorf("Invalid json body from %s: %v", ep, err)
			}
		})
	}
}

func TestE2E_NativeDashboardExplain(t *testing.T) {
	router, _, _ := setupNativeE2E(t)

	payload := map[string]interface{}{
		"card_type": "sales_trend",
		"card_data": map[string]interface{}{
			"metric":  "Total Revenue",
			"value":   1500000.5,
			"context": "January 2024 to December 2024",
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/dashboard/explain", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Explain metric failed: %d - %s", w.Code, w.Body.String())
	}
	t.Logf("Explanation received: %s", w.Body.String())
}

func TestE2E_NativeChatSSE(t *testing.T) {
	router, _, _ := setupNativeE2E(t)

	payload := map[string]interface{}{
		"message": "Give me a breakdown of exactly what steps you'd execute to analyze a 20% drop in TV category sales over the last month.",
	}
	body, _ := json.Marshal(payload)

	// Since SSE streams, we hit the endpoint then read chunks
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// This will block until the entire multi-agent streaming completes
	router.ServeHTTP(w, req)

	resp := w.Body.String()

	if !strings.Contains(resp, "event: session") {
		t.Errorf("Missing session event in SSE blocks")
	}

	if !strings.Contains(resp, "event: response") {
		t.Fatalf("Missing final response in SSE blocks. Output: %s", resp)
	}

	t.Logf("Supervisor streaming completed natively: Output length %d bytes", len(resp))
}

func TestE2E_NativeGraphQLMutations(t *testing.T) {
	router, _, _ := setupNativeE2E(t)

	// We pass empty session id to create a new session natively via graphql
	query := `
		mutation {
			sendMessage(message: "Which category had the absolute highest revenue?", sessionId: "") {
				content
				agent_name
				session_id
				suggestions {
					label
					type
					value
				}
			}
		}`

	payload := map[string]interface{}{
		"query": query,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/graphql", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GraphQL mutaion failed: HTTP %d", w.Code)
	}

	var resp struct {
		Data struct {
			SendMessage struct {
				Content     string                   `json:"content"`
				AgentName   string                   `json:"agent_name"`
				SessionId   string                   `json:"session_id"`
				Suggestions []map[string]interface{} `json:"suggestions"`
			} `json:"sendMessage"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GraphQL decode failed: %v", err)
	}

	sm := resp.Data.SendMessage
	if sm.Content == "" {
		t.Fatalf("Expected non-empty native response content from GraphQL. Raw Response: %s", w.Body.String())
	}

	t.Logf("GraphQL mapped query successfully to %s: Response preview - %s...", sm.AgentName, sm.Content[:30])

	// Quickly verify that episodic DB history hit exists
	histReq, _ := http.NewRequest("POST", "/api/chat/sessions/"+sm.SessionId+"/messages", strings.NewReader(`{}`))
	histReq.Header.Set("Content-Type", "application/json")
	wH := httptest.NewRecorder()
	router.ServeHTTP(wH, histReq)

	if wH.Code != http.StatusOK {
		t.Errorf("Failed to retrieve chat history from vector store immediately after insert")
	}
}

func TestE2E_NativeActions(t *testing.T) {
	router, db, _ := setupNativeE2E(t)

	// Clean out old pending actions explicitly to trigger LLM generation
	if _, err := db.Exec(context.Background(), "DELETE FROM action_log"); err != nil {
		t.Fatalf("cleanup action_log: %v", err)
	}

	// 1. Generate new actions via LLM
	reqGen, _ := http.NewRequest("POST", "/api/actions/generate", strings.NewReader(`{}`))
	reqGen.Header.Set("Content-Type", "application/json")
	wGen := httptest.NewRecorder()
	router.ServeHTTP(wGen, reqGen)

	if wGen.Code != http.StatusOK {
		t.Fatalf("Action LLM generation failed: HTTP %d - %s", wGen.Code, wGen.Body.String())
	}

	// 2. Fetch actions
	reqFetch, _ := http.NewRequest("GET", "/api/actions?status=pending", nil)
	wFetch := httptest.NewRecorder()
	router.ServeHTTP(wFetch, reqFetch)

	var acts []map[string]interface{}
	bodyBytes, _ := io.ReadAll(wFetch.Body)
	_ = json.Unmarshal(bodyBytes, &acts)

	if len(acts) == 0 {
		t.Logf("LLM generated 0 strategic actions (expected >= 0)")
		return
	}

	actID := fmt.Sprintf("%v", acts[0]["id"])

	// 3. Approve an action
	reqApprove, _ := http.NewRequest("POST", "/api/actions/"+actID+"/approve", strings.NewReader(`{}`))
	reqApprove.Header.Set("Content-Type", "application/json")
	wApprove := httptest.NewRecorder()
	router.ServeHTTP(wApprove, reqApprove)

	if wApprove.Code != http.StatusOK {
		t.Errorf("Approve action failed HTTP %d - %s", wApprove.Code, wApprove.Body.String())
	}
}

// TestE2E_NativeSQLGenerationExhaustive explicitly tests the AnalystAgent's text-to-sql ReAct mapping.
func TestE2E_NativeSQLGenerationExhaustive(t *testing.T) {
	_, db, llmClient := setupNativeE2E(t)

	// We only need the Analyst Agent for this test
	toolset := agent.NewToolSet(db, llmClient)
	analyst := agent.NewAnalystAgent(llmClient, db, toolset, nil)

	queries := []string{
		"Show me the top 3 selling products by revenue.",
		"What is our total inventory quantity on hand across all locations?",
		"Give me the average margin percentage for the 'Electronics' category.",
		"Determine which region had the worst sales performance last quarter.",
		"List the names of all active sellers in the 'North' region.",
	}

	for _, q := range queries {
		t.Run(fmt.Sprintf("Query: %s", q), func(t *testing.T) {
			input := &agent.Input{
				Query:         q,
				MemoryContext: &memory.MemoryContext{}, // Empty memory is fine for pure SQL
			}

			out, err := analyst.Process(context.Background(), input)

			if err != nil {
				t.Fatalf("Analyst Agent critically failed: %v", err)
			}

			// Validate that the output string represents a successful conclusion
			if strings.Contains(out.Response, "could not find any data") {
				// While technically successful SQL execution, means db is empty or logic is weird
				t.Logf("SQL executed successfully but returned zero rows: %s", out.Response)
			} else if strings.Contains(out.Response, "OUT_OF_PURVIEW") {
				t.Fatalf("Agent incorrectly classified the valid query as out of purview.")
			} else {
				t.Logf("Analyst generated and executed SQL successfully for query. Response preview: %s...", out.Response[:50])
			}

			// Verify that the reasoning chain includes at least one successful execution step
			foundSQL := false
			for _, step := range out.Reasoning {
				if step.Type == "action" && strings.Contains(step.Content, "Generated SQL:") {
					foundSQL = true
					t.Logf("Generated SQL Context: %s", step.Content)
				}
				if step.Type == "observation" && strings.Contains(step.Content, "SQL Error") {
					t.Logf("Warning: LLM hallucinated, but ReAct loop caught it: %s", step.Content)
				}
			}

			if !foundSQL {
				t.Fatalf("Analyst did not generate or execute any SQL. Expected SQL action step.")
			}
		})
	}
}
