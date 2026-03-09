package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
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
		actions.POST("", createAction(db))
		actions.POST("/generate", generateActions(db, recommender))
		actions.POST("/draft", draftAction(llmClient))
		actions.PATCH("/:id", updateActionDetails(db))
		actions.POST("/:id/approve", updateActionStatus(db, "approved"))
		actions.POST("/:id/reject", updateActionStatus(db, "rejected"))
		actions.POST("/:id/revert", updateActionStatus(db, "pending"))
		actions.GET("/:id/comments", getActionComments(db))
		actions.POST("/:id/comments", addActionComment(db))
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
					COALESCE(p.name, '') as product_name,
					COALESCE(a.priority, 'medium') as priority,
					COALESCE(a.expected_impact, '') as expected_impact
				FROM action_log a
				LEFT JOIN dim_products p ON a.product_id = p.id
				WHERE a.status = $1
				ORDER BY
					CASE COALESCE(a.priority,'medium') WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END,
					a.confidence_score DESC, a.created_at DESC`
			args = []any{status}
		} else {
			query = `
				SELECT a.id, a.title, a.description, a.action_type, a.category,
					a.confidence_score, a.status, a.created_at, a.updated_at,
					COALESCE(p.name, '') as product_name,
					COALESCE(a.priority, 'medium') as priority,
					COALESCE(a.expected_impact, '') as expected_impact
				FROM action_log a
				LEFT JOIN dim_products p ON a.product_id = p.id
				ORDER BY
					CASE a.status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END,
					CASE COALESCE(a.priority,'medium') WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END,
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
			Priority        string  `json:"priority"`
			ExpectedImpact  string  `json:"expected_impact"`
		}

		var actions []Action
		for rows.Next() {
			var a Action
			var createdAt, updatedAt time.Time
			if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.ActionType, &a.Category,
				&a.ConfidenceScore, &a.Status, &createdAt, &updatedAt, &a.ProductName,
				&a.Priority, &a.ExpectedImpact); err != nil {
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

		// Insert generated actions (skip duplicates already pending with same title)
		count := 0
		for _, s := range suggestions {
			priority := s.Priority
			if priority == "" {
				switch {
				case s.Confidence >= 0.85:
					priority = "high"
				case s.Confidence >= 0.70:
					priority = "medium"
				default:
					priority = "low"
				}
			}
			result, err := db.Exec(c, `
				INSERT INTO action_log (id, title, description, action_type, category, confidence_score, status, priority, expected_impact)
				SELECT $1::uuid, $2::text, $3::text, $4::text, $5::text, $6::numeric, 'pending', $7::text, $8::text
				WHERE NOT EXISTS (
					SELECT 1 FROM action_log WHERE title = $2::text AND status = 'pending'
				)
			`, uuid.New().String(), s.Title, s.Description, s.ActionType, "General", s.Confidence, priority, s.ExpectedImpact)
			if err != nil {
				slog.WarnContext(c.Request.Context(), "Failed to insert generated action", "error", err, "title", s.Title)
				continue
			}
			if result.RowsAffected() > 0 {
				count++
			}
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

// @Summary Draft an action
// @Description Use the LLM to draft an action from natural language input
// @Tags actions
// @Produce json
// @Param request body map[string]string true "User Input"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /actions/draft [post]
func draftAction(llmClient llm.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Input == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
			return
		}

		systemPrompt := `You are an AI assistant helping a Category Manager draft an action.
Given the user's input, extract and structure it into a JSON object representing the action.
The action must contain:
{
  "title": "Short descriptive title",
  "description": "Detailed explanation of what needs to be done and why",
  "action_type": "One of: price_match, restock, promotion, delist, manual_execution",
  "category": "The product category, or 'General' if not specific",
  "confidence_score": 0.95
}
Return ONLY valid JSON.`

		result, err := llmClient.Generate(c.Request.Context(), systemPrompt, req.Input)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "LLM draft action failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to draft action"})
			return
		}

		// Try to extract JSON from result
		clean := result
		startIdx := strings.Index(clean, "{")
		endIdx := strings.LastIndex(clean, "}")
		if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
			clean = clean[startIdx : endIdx+1]
		}

		var action map[string]interface{}
		if err := json.Unmarshal([]byte(clean), &action); err != nil {
			slog.WarnContext(c.Request.Context(), "Failed to parse LLM JSON, using fallback", "error", err, "raw", result)
			// Fallback: create a structured action from the raw input
			action = map[string]interface{}{
				"title":            req.Input,
				"description":      result,
				"action_type":      "manual_execution",
				"category":         "General",
				"confidence_score": 0.8,
			}
		}

		c.JSON(http.StatusOK, action)
	}
}

// @Summary Get action comments
// @Description Retrieve comments for a specific action
// @Tags actions
// @Produce json
// @Param id path string true "Action ID"
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /actions/{id}/comments [get]
func getActionComments(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		actionID := c.Param("id")
		rows, err := db.Query(c, `
			SELECT id, content, user_name, created_at 
			FROM action_comments 
			WHERE action_id = $1 
			ORDER BY created_at ASC
		`, actionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Comment struct {
			ID        string `json:"id"`
			Text      string `json:"comment_text"`
			CreatedBy string `json:"created_by"`
			CreatedAt string `json:"created_at"`
		}

		var comments []Comment
		for rows.Next() {
			var cm Comment
			var createdAt time.Time
			if err := rows.Scan(&cm.ID, &cm.Text, &cm.CreatedBy, &createdAt); err == nil {
				cm.CreatedAt = createdAt.Format(time.RFC3339)
				comments = append(comments, cm)
			}
		}
		if comments == nil {
			comments = []Comment{}
		}
		c.JSON(http.StatusOK, comments)
	}
}

// @Summary Add action comment
// @Description Add a new comment to a specific action
// @Tags actions
// @Produce json
// @Param id path string true "Action ID"
// @Param request body map[string]string true "Comment Text"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /actions/{id}/comments [post]
func addActionComment(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		actionID := c.Param("id")
		var req struct {
			Text string `json:"comment_text"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment text"})
			return
		}

		commentID := uuid.New().String()
		_, err := db.Exec(c, `
			INSERT INTO action_comments (id, action_id, content, user_name)
			VALUES ($1, $2, $3, 'user')
		`, commentID, actionID, req.Text)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Comment added", "id": commentID})
	}
}

// @Summary Update action details
// @Description Update the title and description of a pending action
// @Tags actions
// @Produce json
// @Param id path string true "Action ID"
// @Param request body map[string]string true "Updated fields"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /actions/{id} [patch]
func updateActionDetails(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		actionID := c.Param("id")
		var req struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.Title == "" && req.Description == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of title or description is required"})
			return
		}

		slog.DebugContext(c.Request.Context(), "Updating action details", "action_id", actionID)
		result, err := db.Exec(c, `
			UPDATE action_log
			SET title = CASE WHEN $1 <> '' THEN $1 ELSE title END,
			    description = CASE WHEN $2 <> '' THEN $2 ELSE description END,
			    updated_at = NOW()
			WHERE id = $3 AND status = 'pending'
		`, req.Title, req.Description, actionID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to update action details", "error", err, "action_id", actionID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if result.RowsAffected() == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "action not found or is not in pending state"})
			return
		}

		slog.InfoContext(c.Request.Context(), "Action details updated", "action_id", actionID)
		c.JSON(http.StatusOK, gin.H{"message": "Action updated"})
	}
}
