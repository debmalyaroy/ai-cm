package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockLLMClient satisfies llm.Client for dashboard tests.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Generate(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) GenerateStream(_ context.Context, _, _ string) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, m.err
}

func (m *mockLLMClient) Name() string                      { return "mock" }
func (m *mockLLMClient) WithModel(_ string) llm.Client     { return m }
func (m *mockLLMClient) WithMaxTokens(_ int) llm.Client    { return m }

// --- explainCard ---

func TestExplainCard_Success(t *testing.T) {
	mockLLM := &mockLLMClient{response: "Sales are up 10% due to seasonal demand."}

	r := gin.New()
	r.POST("/dashboard/explain", explainCard(mockLLM))

	body, _ := json.Marshal(map[string]any{
		"card_type": "kpi",
		"card_data": map[string]any{"total_gmv": 1000000.0, "avg_margin_pct": 12.5},
	})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/explain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("could not parse response JSON: %v", err)
	}
	if resp["explanation"] != mockLLM.response {
		t.Errorf("explanation = %q, want %q", resp["explanation"], mockLLM.response)
	}
}

func TestExplainCard_MissingCardType(t *testing.T) {
	mockLLM := &mockLLMClient{response: "some explanation"}

	r := gin.New()
	r.POST("/dashboard/explain", explainCard(mockLLM))

	// card_type is missing — ShouldBindJSON binding:"required" should fail.
	body, _ := json.Marshal(map[string]any{
		"card_data": map[string]any{"value": 42},
	})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/explain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestExplainCard_MissingCardData(t *testing.T) {
	mockLLM := &mockLLMClient{response: "some explanation"}

	r := gin.New()
	r.POST("/dashboard/explain", explainCard(mockLLM))

	// card_data is missing — binding:"required" should fail.
	body, _ := json.Marshal(map[string]any{
		"card_type": "kpi",
	})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/explain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestExplainCard_EmptyBody(t *testing.T) {
	mockLLM := &mockLLMClient{response: ""}

	r := gin.New()
	r.POST("/dashboard/explain", explainCard(mockLLM))

	req := httptest.NewRequest(http.MethodPost, "/dashboard/explain", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestExplainCard_LLMError(t *testing.T) {
	mockLLM := &mockLLMClient{err: &testError{"LLM unavailable"}}

	r := gin.New()
	r.POST("/dashboard/explain", explainCard(mockLLM))

	body, _ := json.Marshal(map[string]any{
		"card_type": "sales_trend",
		"card_data": []map[string]any{{"month": "2026-01", "revenue": 5000.0}},
	})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/explain", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// testError is a simple error type used in dashboard tests.
type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// --- cardActions ---

func TestCardActions_Success_ParsesNumberedList(t *testing.T) {
	llmResp := "1. Reorder top-selling SKUs before stock-out\n2. Launch a flash sale for slow movers\n3. Negotiate better margins with key suppliers"
	mockLLM := &mockLLMClient{response: llmResp}

	r := gin.New()
	r.POST("/dashboard/card-actions", cardActions(mockLLM))

	body, _ := json.Marshal(map[string]any{
		"card_type": "Total GMV",
		"card_data": map[string]any{"total_gmv": 5000000.0},
	})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/card-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	actionsRaw, ok := resp["actions"].([]any)
	if !ok {
		t.Fatalf("actions not an array: %v", resp["actions"])
	}
	if len(actionsRaw) != 3 {
		t.Errorf("len(actions) = %d, want 3", len(actionsRaw))
	}
}

func TestCardActions_MissingCardType(t *testing.T) {
	r := gin.New()
	r.POST("/dashboard/card-actions", cardActions(&mockLLMClient{}))

	body, _ := json.Marshal(map[string]any{"card_data": map[string]any{"value": 1}})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/card-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCardActions_LLMError_Returns500(t *testing.T) {
	r := gin.New()
	r.POST("/dashboard/card-actions", cardActions(&mockLLMClient{err: &testError{"llm down"}}))

	body, _ := json.Marshal(map[string]any{"card_type": "Avg Margin"})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/card-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestCardActions_ShortLLMResponse_PadsWithDefaults(t *testing.T) {
	// LLM returns only 1 numbered item — handler should pad to 3.
	mockLLM := &mockLLMClient{response: "1. Focus on high-margin products"}

	r := gin.New()
	r.POST("/dashboard/card-actions", cardActions(mockLLM))

	body, _ := json.Marshal(map[string]any{"card_type": "Avg Margin"})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/card-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp) //nolint:errcheck
	actionsRaw := resp["actions"].([]any)
	if len(actionsRaw) != 3 {
		t.Errorf("len(actions) = %d, want 3", len(actionsRaw))
	}
}

func TestCardActions_NoCardData_StillWorks(t *testing.T) {
	llmResp := "1. Action A\n2. Action B\n3. Action C"
	r := gin.New()
	r.POST("/dashboard/card-actions", cardActions(&mockLLMClient{response: llmResp}))

	// card_data omitted entirely
	body, _ := json.Marshal(map[string]any{"card_type": "Units Sold"})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/card-actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// --- RegisterDashboardRoutes route registration ---

func TestRegisterDashboardRoutes_AllEndpointsRegistered(t *testing.T) {
	r := gin.New()
	api := r.Group("/api")
	RegisterDashboardRoutes(api, nil, &mockLLMClient{response: ""})

	routes := r.Routes()
	type routeKey struct{ method, path string }
	registered := make(map[routeKey]bool, len(routes))
	for _, route := range routes {
		registered[routeKey{route.Method, route.Path}] = true
	}

	expected := []routeKey{
		{http.MethodPost, "/api/dashboard/kpis"},
		{http.MethodPost, "/api/dashboard/sales-trend"},
		{http.MethodPost, "/api/dashboard/category-breakdown"},
		{http.MethodPost, "/api/dashboard/regional-performance"},
		{http.MethodPost, "/api/dashboard/top-products"},
		{http.MethodPost, "/api/dashboard/explain"},
		{http.MethodPost, "/api/dashboard/card-actions"},
	}

	for _, e := range expected {
		if !registered[e] {
			t.Errorf("route not registered: %s %s", e.method, e.path)
		}
	}
}

// --- DB-dependent handlers: nil-pool returns 500 via Recovery ---
// When db is nil, calling db.Query() panics; gin.Recovery() catches it and
// writes a 500. This verifies the happy-path invocation reaches the DB call.

func TestGetKPIs_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/dashboard/kpis", getKPIs(nil))

	req := httptest.NewRequest(http.MethodPost, "/dashboard/kpis", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (nil DB panic recovered)", w.Code)
	}
}

func TestGetSalesTrend_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/dashboard/sales-trend", getSalesTrend(nil))

	req := httptest.NewRequest(http.MethodPost, "/dashboard/sales-trend", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGetCategoryBreakdown_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/dashboard/category-breakdown", getCategoryBreakdown(nil))

	req := httptest.NewRequest(http.MethodPost, "/dashboard/category-breakdown", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGetRegionalPerformance_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/dashboard/regional-performance", getRegionalPerformance(nil))

	req := httptest.NewRequest(http.MethodPost, "/dashboard/regional-performance", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGetTopProducts_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/dashboard/top-products", getTopProducts(nil))

	req := httptest.NewRequest(http.MethodPost, "/dashboard/top-products", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
