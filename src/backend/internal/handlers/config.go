package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterConfigRoutes registers user configuration API routes.
func RegisterConfigRoutes(rg *gin.RouterGroup, db *pgxpool.Pool) {
	rg.GET("/config/preferences", getPreferences(db))
	rg.PUT("/config/preferences", savePreferences(db))
}

// @Summary Get user preferences
// @Description Retrieve all stored preferences for the demo user
// @Tags config
// @Produce json
// @Success 200 {object} map[string]string
// @Router /config/preferences [get]
func getPreferences(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(c.Request.Context(), `
			SELECT key, value FROM user_preferences WHERE user_id = 'demo_user'
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch preferences", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		prefs := make(map[string]string)
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err == nil {
				prefs[key] = value
			}
		}
		c.JSON(http.StatusOK, prefs)
	}
}

// @Summary Save user preferences
// @Description Upsert user preferences (key-value pairs) for the demo user
// @Tags config
// @Accept json
// @Produce json
// @Param request body map[string]string true "Preference key-value pairs"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /config/preferences [put]
func savePreferences(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var prefs map[string]string
		if err := c.ShouldBindJSON(&prefs); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		for key, value := range prefs {
			_, err := db.Exec(c.Request.Context(), `
				INSERT INTO user_preferences (user_id, key, value)
				VALUES ('demo_user', $1, $2)
				ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
			`, key, value)
			if err != nil {
				slog.ErrorContext(c.Request.Context(), "Failed to save preference", "key", key, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		slog.DebugContext(c.Request.Context(), "Preferences saved", "count", len(prefs))
		c.JSON(http.StatusOK, gin.H{"message": "preferences saved", "count": len(prefs)})
	}
}
