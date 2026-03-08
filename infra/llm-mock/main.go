// Package main implements a lightweight mock Ollama server for E2E tests.
// It listens on port 11434 (Ollama default), handles POST /api/generate,
// and returns canned responses based on keyword matching in the prompt.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// OllamaRequest mirrors the Ollama /api/generate request body.
type OllamaRequest struct {
	Model  string         `json:"model"`
	Prompt string         `json:"prompt"`
	Stream bool           `json:"stream"`
	Option map[string]any `json:"options,omitempty"`
}

// OllamaResponse mirrors the Ollama /api/generate response body.
type OllamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// promptType identifies what kind of agent prompt is being handled.
type promptType int

const (
	ptIntent      promptType = iota // Intent classification
	ptSQL                           // SQL generation
	ptInsight                       // Strategist / explain-card insight
	ptPlan                          // Planner action generation
	ptCommunicate                   // Liaison email / report
	ptSuggestions                   // Chat suggestions
	ptGeneral                       // Fallback
)

// classify inspects the prompt text and returns the matching prompt type.
// The keyword checks mirror the patterns used by each agent's prompt template.
func classify(prompt string) promptType {
	lower := strings.ToLower(prompt)

	// Intent classifier prompt always asks for one-word classification.
	if strings.Contains(lower, "reply with only one word") ||
		strings.Contains(lower, "intent classifier") ||
		strings.Contains(lower, "query, insight, plan, communicate, monitor, general") {
		return ptIntent
	}

	// SQL / Analyst prompt contains the schema placeholder text that gets filled in.
	if strings.Contains(lower, "sqlforge") ||
		strings.Contains(lower, "```sql") ||
		strings.Contains(lower, "your entire response must be a single") ||
		strings.Contains(lower, "select statement") ||
		strings.Contains(lower, "read-only") && strings.Contains(lower, "schema") {
		return ptSQL
	}

	// Planner prompt always contains "actionforge" or the ACTION: format marker.
	if strings.Contains(lower, "actionforge") ||
		strings.Contains(lower, "action:\ntitle:") ||
		strings.Contains(lower, "propose concrete, executable business actions") {
		return ptPlan
	}

	// Liaison prompt drafts emails or reports.
	if strings.Contains(lower, "liaison") ||
		strings.Contains(lower, "draft") && (strings.Contains(lower, "email") || strings.Contains(lower, "report")) ||
		strings.Contains(lower, "seller communication") {
		return ptCommunicate
	}

	// Chat suggestions prompt asks for follow-up suggestions.
	if strings.Contains(lower, "suggestion") && strings.Contains(lower, "json") ||
		strings.Contains(lower, "follow-up question") ||
		strings.Contains(lower, "next logical question") {
		return ptSuggestions
	}

	// Strategist / explain-card insight prompt.
	if strings.Contains(lower, "strategist") ||
		strings.Contains(lower, "chain-of-thought") ||
		strings.Contains(lower, "explain") && strings.Contains(lower, "metric") ||
		strings.Contains(lower, "analyze") || strings.Contains(lower, "analyse") {
		return ptInsight
	}

	return ptGeneral
}

// buildResponse returns a canned answer for the given prompt type.
// Each response is tailored so that the corresponding agent can parse it successfully.
func buildResponse(pt promptType, prompt string) string {
	lower := strings.ToLower(prompt)

	switch pt {
	case ptIntent:
		// Determine intent from user message keywords embedded in the prompt.
		switch {
		case strings.Contains(lower, "plan") || strings.Contains(lower, "recommend") ||
			strings.Contains(lower, "strategy") || strings.Contains(lower, "action"):
			return "plan"
		case strings.Contains(lower, "email") || strings.Contains(lower, "draft") ||
			strings.Contains(lower, "communicate") || strings.Contains(lower, "report"):
			return "communicate"
		case strings.Contains(lower, "monitor") || strings.Contains(lower, "alert") ||
			strings.Contains(lower, "anomal") || strings.Contains(lower, "watchdog"):
			return "monitor"
		case strings.Contains(lower, "why") || strings.Contains(lower, "explain") ||
			strings.Contains(lower, "analys") || strings.Contains(lower, "insight") ||
			strings.Contains(lower, "trend") || strings.Contains(lower, "compare") ||
			strings.Contains(lower, "breakdown") || strings.Contains(lower, "understand"):
			return "insight"
		case strings.Contains(lower, "hello") || strings.Contains(lower, "hi,") ||
			strings.Contains(lower, "what can you"):
			return "general"
		default:
			// Most E2E test queries are data-retrieval oriented.
			return "query"
		}

	case ptSQL:
		// Return a syntactically valid SQL block that the analyst can execute.
		// The query always succeeds and returns at least one row so the agent
		// produces a non-empty response without hitting the retry loop.
		switch {
		case strings.Contains(lower, "revenue") || strings.Contains(lower, "sales"):
			return "```sql\nSELECT 'Mock Category' AS category, 1000000.00 AS total_revenue, 500 AS total_units FROM fact_sales LIMIT 1;\n```"
		case strings.Contains(lower, "inventory") || strings.Contains(lower, "stock"):
			return "```sql\nSELECT 'Mock Product' AS product_name, 250 AS quantity_on_hand FROM fact_inventory LIMIT 1;\n```"
		case strings.Contains(lower, "margin"):
			return "```sql\nSELECT 'Electronics' AS category, 18.5 AS avg_margin_pct FROM fact_sales LIMIT 1;\n```"
		case strings.Contains(lower, "region"):
			return "```sql\nSELECT 'North' AS region, 750000.00 AS total_revenue FROM fact_sales LIMIT 1;\n```"
		case strings.Contains(lower, "seller"):
			return "```sql\nSELECT 'Mock Seller' AS seller_name, 'North' AS region FROM dim_sellers LIMIT 1;\n```"
		case strings.Contains(lower, "product"):
			return "```sql\nSELECT 'Mock Product A' AS product_name, 95000.00 AS revenue FROM fact_sales JOIN dim_products ON true LIMIT 1;\n```"
		default:
			return "```sql\nSELECT 1 AS result, 'mock_data' AS label;\n```"
		}

	case ptInsight:
		return "Based on the available data, sales performance shows a stable trend with minor fluctuations. " +
			"The primary driver of any observed variance is seasonal demand shifts in the baby care segment. " +
			"Margins remain healthy at approximately 18-22% across core categories. " +
			"The East region contributes roughly 35% of total revenue, making it the highest-performing zone. " +
			"No critical anomalies were detected in the current reporting period."

	case ptPlan:
		return `ACTION:
Title: Optimise Pricing for High-Margin SKUs
Description: Current data shows top-10 SKUs contributing 45% of revenue with margins above 20%. A 5% price increase on these SKUs is expected to increase overall margin by 3-5% without significant volume impact based on historical elasticity data.
Type: price_update
Confidence: 0.85
---
ACTION:
Title: Replenish Low-Stock Inventory in East Region
Description: Inventory levels for 12 SKUs in the East region have fallen below reorder threshold (avg 45 units vs 100-unit reorder point). Restocking these SKUs is expected to prevent an estimated $50,000 revenue loss over the next 30 days.
Type: inventory_adjustment
Confidence: 0.80
---`

	case ptCommunicate:
		return `Subject: Performance Update - Baby Care Category Q1 2026

Dear Partner,

This email summarises the current performance metrics for the Baby Care category as of March 2026.

Key highlights:
- Total revenue: INR 10,00,000 (on track with quarterly target)
- Units sold: 5,200 across all regions
- Top performing product: Mock Product A with 15% above-average margin

Please review the attached report and revert with any queries by end of week.

Best regards,
AI-CM Category Management Team`

	case ptSuggestions:
		return `[
  {"label": "Show top products by revenue", "type": "query", "value": "Show me the top 5 products by revenue this month"},
  {"label": "Analyse margin trends", "type": "insight", "value": "Why did margins change in the last quarter?"},
  {"label": "Check low stock items", "type": "query", "value": "Which products are below reorder level?"}
]`

	default:
		return "Hello! I am the AI-CM Copilot. I can help you analyse sales data, manage inventory, " +
			"generate pricing recommendations, and draft seller communications. " +
			"What would you like to explore today?"
	}
}

// writeJSON serialises v and writes it as a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleGenerate is the main handler for POST /api/generate.
func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OllamaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	pt := classify(req.Prompt)
	responseText := buildResponse(pt, req.Prompt)

	log.Printf("[mock-llm] model=%s stream=%v type=%d prompt_len=%d", req.Model, req.Stream, pt, len(req.Prompt))

	if req.Stream {
		// Streaming: emit token chunks then a done frame, matching Ollama's NDJSON format.
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Split response into word-level chunks for realistic streaming.
		words := strings.Fields(responseText)
		for i, word := range words {
			chunk := word
			if i < len(words)-1 {
				chunk += " "
			}
			frame := OllamaResponse{Model: req.Model, Response: chunk, Done: false}
			b, _ := json.Marshal(frame)
			fmt.Fprintf(w, "%s\n", b)
			flusher.Flush()
		}

		// Terminal done frame.
		done := OllamaResponse{Model: req.Model, Response: "", Done: true}
		b, _ := json.Marshal(done)
		fmt.Fprintf(w, "%s\n", b)
		flusher.Flush()
		return
	}

	// Non-streaming: return a single JSON object.
	writeJSON(w, http.StatusOK, OllamaResponse{
		Model:    req.Model,
		Response: responseText,
		Done:     true,
	})
}

// handleEmbeddings handles POST /api/embeddings (used by memory store when using local provider).
// Returns a fixed-length zero vector that satisfies the 1536-dimension schema.
func handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Consume request body (we don't need it for a mock).
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)

	// Return a 1536-dimension zero vector.
	embedding := make([]float32, 1536)
	writeJSON(w, http.StatusOK, map[string]any{
		"embedding": embedding,
	})
}

// handleHealth returns a simple health check response (mirrors the Ollama /api/health endpoint).
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/generate", handleGenerate)
	mux.HandleFunc("/api/embeddings", handleEmbeddings)
	mux.HandleFunc("/api/health", handleHealth)

	// Catch-all for any other Ollama endpoints (tags, version, etc.)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": "mock-0.1.0"})
	})

	addr := ":11434"
	log.Printf("[mock-llm] Starting mock LLM server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[mock-llm] Server failed: %v", err)
	}
}
