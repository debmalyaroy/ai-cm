package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterAlertRoutes registers alert API routes.
func RegisterAlertRoutes(rg *gin.RouterGroup, db *pgxpool.Pool) {
	alerts := rg.Group("/alerts")
	{
		alerts.GET("", getAlerts(db))
		alerts.POST("/:id/acknowledge", acknowledgeAlert(db))
		alerts.POST("", addAlert(db))
	}
}

func getAlerts(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Fetching alerts")
		rows, err := db.Query(c, `
			SELECT id, title, severity, category, message, acknowledged, created_at
			FROM alerts
			ORDER BY created_at DESC
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch alerts", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Alert struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			Severity     string `json:"severity"`
			Category     string `json:"category"`
			Message      string `json:"message"`
			Acknowledged bool   `json:"acknowledged"`
			CreatedAt    string `json:"created_at"`
		}

		var alerts []Alert
		for rows.Next() {
			var a Alert
			var createdAt time.Time
			if err := rows.Scan(&a.ID, &a.Title, &a.Severity, &a.Category, &a.Message, &a.Acknowledged, &createdAt); err != nil {
				continue
			}
			a.CreatedAt = createdAt.Format(time.RFC3339)
			alerts = append(alerts, a)
		}

		if alerts == nil {
			alerts = []Alert{}
		}

		c.JSON(http.StatusOK, alerts)
	}
}

func acknowledgeAlert(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		alertID := c.Param("id")
		slog.DebugContext(c.Request.Context(), "Acknowledging alert", "alert_id", alertID)

		result, err := db.Exec(c, `
			UPDATE alerts SET acknowledged = TRUE, updated_at = NOW() WHERE id = $1
		`, alertID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to acknowledge alert", "error", err, "alert_id", alertID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if result.RowsAffected() == 0 {
			slog.WarnContext(c.Request.Context(), "Alert not found for acknowledgment", "alert_id", alertID)
			c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
			return
		}

		slog.InfoContext(c.Request.Context(), "Alert acknowledged successfully", "alert_id", alertID)
		c.JSON(http.StatusOK, gin.H{"message": "Alert acknowledged", "id": alertID})
	}
}

func addAlert(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title    string `json:"title"`
			Severity string `json:"severity"`
			Category string `json:"category"`
			Message  string `json:"message"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Severity == "" {
			req.Severity = "info"
		}
		if req.Category == "" {
			req.Category = "General"
		}

		slog.DebugContext(c.Request.Context(), "Adding new alert", "title", req.Title, "severity", req.Severity)

		_, err := db.Exec(c, `
			INSERT INTO alerts (title, severity, category, message, acknowledged)
			VALUES ($1, $2, $3, $4, FALSE)
		`, req.Title, req.Severity, req.Category, req.Message)

		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to add alert", "error", err, "title", req.Title)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		slog.InfoContext(c.Request.Context(), "Alert added successfully", "title", req.Title)
		c.JSON(http.StatusOK, gin.H{"message": "Alert created successfully"})
	}
}
