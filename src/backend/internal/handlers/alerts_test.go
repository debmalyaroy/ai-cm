package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// --- RegisterAlertRoutes route registration ---

func TestRegisterAlertRoutes_AllEndpointsRegistered(t *testing.T) {
	r := gin.New()
	api := r.Group("/api")
	RegisterAlertRoutes(api, nil)

	routes := r.Routes()
	type routeKey struct{ method, path string }
	registered := make(map[routeKey]bool, len(routes))
	for _, route := range routes {
		registered[routeKey{route.Method, route.Path}] = true
	}

	expected := []routeKey{
		{http.MethodGet, "/api/alerts"},
		{http.MethodPost, "/api/alerts/:id/acknowledge"},
		{http.MethodPost, "/api/alerts"},
	}

	for _, e := range expected {
		if !registered[e] {
			t.Errorf("route not registered: %s %s", e.method, e.path)
		}
	}
}

// --- addAlert validation (no DB required for bad-request path) ---

func TestAddAlert_InvalidBody(t *testing.T) {
	r := gin.New()
	r.POST("/alerts", addAlert(nil))

	// Send invalid JSON — ShouldBindJSON should fail.
	req := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (bad JSON); body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// --- DB-dependent handlers: nil pool panics, recovered as 500 ---

func TestGetAlerts_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/alerts", getAlerts(nil))

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (nil DB panic recovered)", w.Code)
	}
}

func TestAcknowledgeAlert_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/alerts/:id/acknowledge", acknowledgeAlert(nil))

	req := httptest.NewRequest(http.MethodPost, "/alerts/some-uuid/acknowledge", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (nil DB panic recovered)", w.Code)
	}
}

func TestAddAlert_NilDB_Returns500(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/alerts", addAlert(nil))

	body, _ := json.Marshal(map[string]string{
		"title":    "Test Alert",
		"severity": "warning",
		"category": "pricing",
		"message":  "Competitor price dropped",
	})

	req := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (nil DB panic recovered)", w.Code)
	}
}

// --- addAlert default values (severity and category defaults injected before DB call) ---
// This test confirms the handler sets defaults. With nil DB the panic occurs AFTER
// defaults are applied. We cannot observe that directly, but we can verify the
// handler progresses past the bind/validation stage (returns 500, not 400).

func TestAddAlert_EmptySeverityAndCategory_DefaultsApplied(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/alerts", addAlert(nil))

	// Omit severity and category — handler should fill defaults before hitting DB.
	body, _ := json.Marshal(map[string]string{
		"title":   "Bare Alert",
		"message": "something happened",
	})

	req := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should reach the DB exec step and panic → 500 (not 400 bad request).
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body: %s", w.Code, w.Body.String())
	}
}
