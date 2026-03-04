package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/agent"
	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterActionRoutes registers action API routes.
func RegisterActionRoutes(rg *gin.RouterGroup, db *pgxpool.Pool, llmClient llm.Client) {
	recommender := agent.NewRecommender(db)

	actions := rg.Group("/actions")
	{
		actions.GET("", getActions(db))
		actions.POST("", createAction(db)) // Issue 1 Fix: Explicitly save actions
		actions.POST("/generate", generateActions(db, recommender))
		actions.POST("/:id/approve", updateActionStatus(db, "approved"))
		actions.POST("/:id/reject", updateActionStatus(db, "rejected"))
	}
}

// @Summary Get all actions
// @Description Retrieve a list of actions, optionally filtered by status
// @Tags actions
// @Produce json
// @Param status query string false "Action status (e.g., pending, approved)"
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /actions [get]
func getActions(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read status from JSON body or query param for backward compat
		status := c.DefaultQuery("status", "")
		var body struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.Status != "" {
			status = body.Status
		}

		var query string
		var args []any

		if status != "" {
			query = `
				SELECT a.id, a.title, a.description, a.action_type, a.category, 
					a.confidence_score, a.status, a.created_at, a.updated_at,
					COALESCE(p.name, '') as product_name
				FROM action_log a
				LEFT JOIN dim_products p ON a.product_id = p.id
				WHERE a.status = $1
				ORDER BY a.confidence_score DESC, a.created_at DESC`
			args = []any{status}
		} else {
			query = `
				SELECT a.id, a.title, a.description, a.action_type, a.category, 
					a.confidence_score, a.status, a.created_at, a.updated_at,
					COALESCE(p.name, '') as product_name
				FROM action_log a
				LEFT JOIN dim_products p ON a.product_id = p.id
				ORDER BY 
					CASE a.status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END,
					a.confidence_score DESC, a.created_at DESC`
		}

		slog.DebugContext(c.Request.Context(), "Fetching actions", "status", status)
		rows, err := db.Query(c, query, args...)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch actions", "error", err, "status", status)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Action struct {
			ID              string  `json:"id"`
			Title           string  `json:"title"`
			Description     string  `json:"description"`
			ActionType      string  `json:"action_type"`
			Category        string  `json:"category"`
			ConfidenceScore float64 `json:"confidence_score"`
			Status          string  `json:"status"`
			CreatedAt       string  `json:"created_at"`
			UpdatedAt       string  `json:"updated_at"`
			ProductName     string  `json:"product_name"`
		}

		var actions []Action
		for rows.Next() {
			var a Action
			var createdAt, updatedAt time.Time
			if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.ActionType, &a.Category,
				&a.ConfidenceScore, &a.Status, &createdAt, &updatedAt, &a.ProductName); err != nil {
				continue
			}
			a.CreatedAt = createdAt.Format(time.RFC3339)
			a.UpdatedAt = updatedAt.Format(time.RFC3339)
			actions = append(actions, a)
		}

		if actions == nil {
			actions = []Action{}
		}

		c.JSON(http.StatusOK, actions)
	}
}

// @Summary Create a new action
// @Description Allows the frontend to explicitly save an executed action
// @Tags actions
// @Produce json
// @Param request body map[string]interface{} true "Action Data"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /actions [post]
func createAction(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title           string  `json:"title"`
			Description     string  `json:"description"`
			ActionType      string  `json:"action_type"`
			Category        string  `json:"category"`
			ConfidenceScore float64 `json:"confidence_score"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		// Defaults
		if req.ActionType == "" {
			req.ActionType = "manual_execution"
		}
		if req.Category == "" {
			req.Category = "User Initiated"
		}
		if req.ConfidenceScore == 0 {
			req.ConfidenceScore = 1.0 // High confidence for manual actions
		}

		slog.DebugContext(c.Request.Context(), "Creating manual action", "title", req.Title, "type", req.ActionType)
		actionID := uuid.New().String()
		_, err := db.Exec(c, `
			INSERT INTO action_log (id, title, description, action_type, category, confidence_score, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'approved')
		`, actionID, req.Title, req.Description, req.ActionType, req.Category, req.ConfidenceScore)

		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to create action", "error", err, "title", req.Title)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		slog.InfoContext(c.Request.Context(), "Action created successfully", "action_id", actionID)
		c.JSON(http.StatusOK, gin.H{"message": "Action created successfully", "id": actionID})
	}
}

// @Summary Generate new actions
// @Description Use the AI Recommender agent to generate new action suggestions
// @Tags actions
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /actions/generate [post]
func generateActions(db *pgxpool.Pool, recommender *agent.Recommender) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Generating AI actions via Recommender")
		suggestions, err := recommender.GenerateActions(c)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to generate AI actions", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Insert generated actions
		count := 0
		for _, s := range suggestions {
			_, err := db.Exec(c, `
				INSERT INTO action_log (id, title, description, action_type, category, confidence_score, status)
				VALUES ($1, $2, $3, $4, $5, $6, 'pending')
			`, uuid.New().String(), s.Title, s.Description, s.ActionType, "General", s.Confidence)
			if err != nil {
				slog.WarnContext(c.Request.Context(), "Failed to insert generated action", "error", err, "title", s.Title)
				continue
			}
			count++
		}

		slog.InfoContext(c.Request.Context(), "Successfully generated AI actions", "count", count)
		c.JSON(http.StatusOK, gin.H{
			"message": "Actions generated",
			"count":   count,
		})
	}
}

// @Summary Update action status
// @Description Approve or reject a specific action by ID
// @Tags actions
// @Produce json
// @Param id path string true "Action ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /actions/{id}/approve [post]
// @Router /actions/{id}/reject [post]
func updateActionStatus(db *pgxpool.Pool, status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actionID := c.Param("id")
		slog.DebugContext(c.Request.Context(), "Updating action status", "action_id", actionID, "status", status)

		result, err := db.Exec(c, `
			UPDATE action_log SET status = $1, updated_at = NOW() WHERE id = $2
		`, status, actionID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to update action status", "error", err, "action_id", actionID, "status", status)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if result.RowsAffected() == 0 {
			slog.WarnContext(c.Request.Context(), "Action not found for status update", "action_id", actionID)
			c.JSON(http.StatusNotFound, gin.H{"error": "action not found"})
			return
		}

		slog.InfoContext(c.Request.Context(), "Action status updated successfully", "action_id", actionID, "status", status)
		c.JSON(http.StatusOK, gin.H{
			"message": "Action " + status,
			"id":      actionID,
			"status":  status,
		})
	}
}
