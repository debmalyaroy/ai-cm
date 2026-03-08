package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// --- Middleware tests ---

func TestRequestLogger_SetsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestLogger())
	router.GET("/test", func(c *gin.Context) {
		// Verify request_id was set in context
		id, exists := c.Get("request_id")
		if !exists {
			t.Error("request_id should be set in context")
		}
		if id.(string) == "" {
			t.Error("request_id should not be empty")
		}
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// X-Request-ID header should be set
	reqID := w.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("X-Request-ID header should be set")
	}
	if len(reqID) != 8 {
		t.Errorf("request ID length = %d, want 8", len(reqID))
	}
}

func TestRequestLogger_DifferentIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestLogger())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w2, req2)

	id1 := w1.Header().Get("X-Request-ID")
	id2 := w2.Header().Get("X-Request-ID")
	if id1 == id2 {
		t.Error("consecutive requests should have different request IDs")
	}
}

func TestRecovery_NormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/safe", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/safe", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecovery_PanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("cannot parse body: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("error = %q", body["error"])
	}
	if body["message"] != "an unexpected error occurred" {
		t.Errorf("message = %q", body["message"])
	}
}

func TestRecovery_IntPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic-int", func(c *gin.Context) {
		panic(42)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic-int", nil)
	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// --- SSE tests (enhanced) ---

func TestSendSSE_JSONObject(t *testing.T) {
	var buf bytes.Buffer
	sendSSE(&buf, "test_event", map[string]interface{}{
		"key":    "value",
		"number": 42,
	})
	output := buf.String()

	if !strings.Contains(output, "event: test_event") {
		t.Errorf("missing event name in: %s", output)
	}
	if !strings.Contains(output, "data:") {
		t.Errorf("missing data field in: %s", output)
	}
	if !strings.Contains(output, "value") {
		t.Errorf("missing value in: %s", output)
	}
}

func TestSendSSE_String(t *testing.T) {
	var buf bytes.Buffer
	sendSSE(&buf, "msg", "hello world")
	output := buf.String()

	if !strings.Contains(output, "hello world") {
		t.Errorf("missing string data in: %s", output)
	}
}

func TestSendSSE_Nil(t *testing.T) {
	var buf bytes.Buffer
	sendSSE(&buf, "empty", nil)
	output := buf.String()

	if !strings.Contains(output, "event: empty") {
		t.Errorf("missing event in: %s", output)
	}
	if !strings.Contains(output, "null") {
		t.Errorf("nil should marshal to 'null': %s", output)
	}
}

func TestSendSSE_Array(t *testing.T) {
	var buf bytes.Buffer
	sendSSE(&buf, "list", []string{"a", "b", "c"})
	output := buf.String()

	if !strings.Contains(output, "event: list") {
		t.Errorf("missing event in: %s", output)
	}
}

func TestSendSSE_DoubleNewline(t *testing.T) {
	var buf bytes.Buffer
	sendSSE(&buf, "test", "data")
	output := buf.String()

	// SSE requires double newline separator
	if !strings.HasSuffix(output, "\n\n") {
		t.Errorf("SSE should end with double newline: %q", output)
	}
}

func TestSendSSE_UnmarshalableType(t *testing.T) {
	var buf bytes.Buffer
	// Channels can't be marshaled to JSON
	ch := make(chan int)
	sendSSE(&buf, "bad", ch)
	output := buf.String()

	// Should not write event on marshal error
	if strings.Contains(output, "event: bad") {
		t.Errorf("should not write event on marshal error: %s", output)
	}
}

// --- ChatRequest binding tests ---

func TestChatRequestBinding_Valid(t *testing.T) {
	data := `{"message": "test query", "session_id": "abc-123"}`
	var req ChatRequest
	err := json.Unmarshal([]byte(data), &req)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.Message != "test query" {
		t.Errorf("message = %q, want 'test query'", req.Message)
	}
	if req.SessionID != "abc-123" {
		t.Errorf("session_id = %q, want 'abc-123'", req.SessionID)
	}
}

func TestChatRequestBinding_NoMessage(t *testing.T) {
	data := `{"session_id": "abc-123"}`
	var req ChatRequest
	err := json.Unmarshal([]byte(data), &req)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.Message != "" {
		t.Errorf("message should be empty, got %q", req.Message)
	}
}

func TestChatRequestBinding_OnlyMessage(t *testing.T) {
	data := `{"message": "hello"}`
	var req ChatRequest
	_ = json.Unmarshal([]byte(data), &req)
	if req.SessionID != "" {
		t.Errorf("session_id should be empty, got %q", req.SessionID)
	}
}

func TestChatRequestBinding_EmptyJSON(t *testing.T) {
	data := `{}`
	var req ChatRequest
	_ = json.Unmarshal([]byte(data), &req)
	if req.Message != "" || req.SessionID != "" {
		t.Error("empty JSON should result in empty fields")
	}
}

// --- Register route existence tests ---

func TestRegisterDashboardRoutes_CreatesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	RegisterDashboardRoutes(api, nil, nil)

	// Verify routes exist by checking the router's routes
	routes := router.Routes()
	expectedPaths := map[string]bool{
		"/api/dashboard/kpis":                 false,
		"/api/dashboard/sales-trend":          false,
		"/api/dashboard/category-breakdown":   false,
		"/api/dashboard/regional-performance": false,
		"/api/dashboard/top-products":         false,
		"/api/dashboard/explain":              false,
	}

	for _, r := range routes {
		if _, ok := expectedPaths[r.Path]; ok {
			expectedPaths[r.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("route not registered: %s", path)
		}
	}
}

func TestRegisterActionRoutes_CreatesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	RegisterActionRoutes(api, nil, nil)

	routes := router.Routes()
	expectedPaths := map[string]bool{
		"/api/actions":             false,
		"/api/actions/generate":    false,
		"/api/actions/:id/approve": false,
		"/api/actions/:id/reject":  false,
	}

	for _, r := range routes {
		if _, ok := expectedPaths[r.Path]; ok {
			expectedPaths[r.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("route not registered: %s", path)
		}
	}
}

func TestRegisterChatRoutes_CreatesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	RegisterChatRoutes(api, nil, nil, nil)

	routes := router.Routes()
	expectedPaths := map[string]bool{
		"/api/chat":                       false,
		"/api/chat/sessions":              false,
		"/api/chat/sessions/:id/messages": false,
	}

	for _, r := range routes {
		if _, ok := expectedPaths[r.Path]; ok {
			expectedPaths[r.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("route not registered: %s", path)
		}
	}
}

// --- Full middleware stack test ---

func TestMiddlewareStack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())
	router.Use(RequestLogger())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID should be set even with Recovery middleware")
	}
}
