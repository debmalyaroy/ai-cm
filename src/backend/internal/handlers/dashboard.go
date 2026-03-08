package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	numberedItemRe     = regexp.MustCompile(`^\d+\.\s+(.+)`)
	defaultCardActions = []string{
		"Review pricing strategy for underperforming SKUs",
		"Analyze inventory levels and reorder points",
		"Evaluate competitor positioning and market trends",
	}
)

// RegisterDashboardRoutes registers all dashboard API routes.
func RegisterDashboardRoutes(rg *gin.RouterGroup, db *pgxpool.Pool, llmClient llm.Client) {
	dash := rg.Group("/dashboard")
	{
		dash.POST("/kpis", getKPIs(db))
		dash.POST("/sales-trend", getSalesTrend(db))
		dash.POST("/category-breakdown", getCategoryBreakdown(db))
		dash.POST("/regional-performance", getRegionalPerformance(db))
		dash.POST("/top-products", getTopProducts(db))
		dash.POST("/explain", explainCard(llmClient))
		dash.POST("/card-actions", cardActions(llmClient))
	}
}

// @Summary Get dashboard KPIs
// @Description Retrieve high-level key performance indicators (GMV, Margin, Units, etc.)
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /dashboard/kpis [get]
func getKPIs(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		type KPIs struct {
			TotalGMV     float64 `json:"total_gmv"`
			AvgMargin    float64 `json:"avg_margin_pct"`
			TotalUnits   int64   `json:"total_units"`
			ActiveSKUs   int64   `json:"active_skus"`
			GMVChange    float64 `json:"gmv_change_pct"`
			MarginChange float64 `json:"margin_change_pct"`
		}

		var kpis KPIs

		slog.DebugContext(c.Request.Context(), "Fetching dashboard KPIs (last 3 months)")
		// Current period (last 3 months)
		err := db.QueryRow(c, `
			SELECT 
				COALESCE(SUM(revenue), 0),
				COALESCE(AVG(margin / NULLIF(revenue, 0) * 100), 0),
				COALESCE(SUM(quantity), 0)
			FROM fact_sales
			WHERE sale_date >= CURRENT_DATE - INTERVAL '3 months'
		`).Scan(&kpis.TotalGMV, &kpis.AvgMargin, &kpis.TotalUnits)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch dashboard KPIs current period", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Active SKUs
		_ = db.QueryRow(c, `SELECT COUNT(*) FROM dim_products WHERE status = 'active'`).Scan(&kpis.ActiveSKUs)

		// Previous period for change calculation
		var prevGMV, prevMargin float64
		if err := db.QueryRow(c, `
			SELECT
				COALESCE(SUM(revenue), 0),
				COALESCE(AVG(margin / NULLIF(revenue, 0) * 100), 0)
			FROM fact_sales
			WHERE sale_date >= CURRENT_DATE - INTERVAL '6 months'
			AND sale_date < CURRENT_DATE - INTERVAL '3 months'
		`).Scan(&prevGMV, &prevMargin); err != nil {
			slog.WarnContext(c.Request.Context(), "failed to query previous period KPIs", "error", err)
		}

		if prevGMV > 0 {
			kpis.GMVChange = ((kpis.TotalGMV - prevGMV) / prevGMV) * 100
		}
		if prevMargin > 0 {
			kpis.MarginChange = kpis.AvgMargin - prevMargin
		}

		c.JSON(http.StatusOK, kpis)
	}
}

// @Summary Get sales trend
// @Description Retrieve monthly sales trend data for the last 12 months
// @Tags dashboard
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /dashboard/sales-trend [get]
func getSalesTrend(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Fetching sales trend for the last 12 months")
		rows, err := db.Query(c, `
			SELECT 
				TO_CHAR(sale_date, 'YYYY-MM') as month,
				SUM(revenue) as revenue,
				SUM(margin) as margin,
				SUM(quantity) as units
			FROM fact_sales
			WHERE sale_date >= CURRENT_DATE - INTERVAL '12 months'
			GROUP BY TO_CHAR(sale_date, 'YYYY-MM')
			ORDER BY month
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch sales trend", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type DataPoint struct {
			Month   string  `json:"month"`
			Revenue float64 `json:"revenue"`
			Margin  float64 `json:"margin"`
			Units   int64   `json:"units"`
		}

		var data []DataPoint
		for rows.Next() {
			var dp DataPoint
			if err := rows.Scan(&dp.Month, &dp.Revenue, &dp.Margin, &dp.Units); err != nil {
				continue
			}
			data = append(data, dp)
		}

		c.JSON(http.StatusOK, data)
	}
}

// @Summary Get category breakdown
// @Description Retrieve sales breakdown by product category for the last 3 months
// @Tags dashboard
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /dashboard/category-breakdown [get]
func getCategoryBreakdown(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Fetching category breakdown")
		rows, err := db.Query(c, `
			SELECT 
				p.category,
				SUM(s.revenue) as revenue,
				SUM(s.margin) as margin,
				SUM(s.quantity) as units,
				COUNT(DISTINCT p.id) as sku_count
			FROM fact_sales s
			JOIN dim_products p ON s.product_id = p.id
			WHERE s.sale_date >= CURRENT_DATE - INTERVAL '3 months'
			GROUP BY p.category
			ORDER BY revenue DESC
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch category breakdown", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Category struct {
			Name     string  `json:"name"`
			Revenue  float64 `json:"revenue"`
			Margin   float64 `json:"margin"`
			Units    int64   `json:"units"`
			SKUCount int64   `json:"sku_count"`
		}

		var data []Category
		for rows.Next() {
			var cat Category
			if err := rows.Scan(&cat.Name, &cat.Revenue, &cat.Margin, &cat.Units, &cat.SKUCount); err != nil {
				continue
			}
			data = append(data, cat)
		}

		c.JSON(http.StatusOK, data)
	}
}

// @Summary Get regional performance
// @Description Retrieve sales performance by region for the last 3 months
// @Tags dashboard
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /dashboard/regional-performance [get]
func getRegionalPerformance(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Fetching regional performance")
		rows, err := db.Query(c, `
			SELECT 
				l.region,
				SUM(s.revenue) as revenue,
				SUM(s.margin) as margin,
				SUM(s.quantity) as units,
				AVG(s.discount_pct) as avg_discount
			FROM fact_sales s
			JOIN dim_locations l ON s.location_id = l.id
			WHERE s.sale_date >= CURRENT_DATE - INTERVAL '3 months'
			GROUP BY l.region
			ORDER BY revenue DESC
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch regional performance", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Region struct {
			Name        string  `json:"name"`
			Revenue     float64 `json:"revenue"`
			Margin      float64 `json:"margin"`
			Units       int64   `json:"units"`
			AvgDiscount float64 `json:"avg_discount"`
		}

		var data []Region
		for rows.Next() {
			var reg Region
			if err := rows.Scan(&reg.Name, &reg.Revenue, &reg.Margin, &reg.Units, &reg.AvgDiscount); err != nil {
				continue
			}
			data = append(data, reg)
		}

		c.JSON(http.StatusOK, data)
	}
}

// @Summary Get top products
// @Description Retrieve top 10 performing products by revenue for the last 3 months
// @Tags dashboard
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /dashboard/top-products [get]
func getTopProducts(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Fetching top products")
		rows, err := db.Query(c, `
			SELECT 
				p.name,
				p.category,
				p.brand,
				SUM(s.revenue) as revenue,
				SUM(s.quantity) as units,
				AVG(s.margin / NULLIF(s.revenue, 0) * 100) as margin_pct
			FROM fact_sales s
			JOIN dim_products p ON s.product_id = p.id
			WHERE s.sale_date >= CURRENT_DATE - INTERVAL '3 months'
			GROUP BY p.id, p.name, p.category, p.brand
			ORDER BY revenue DESC
			LIMIT 10
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch top products", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Product struct {
			Name      string  `json:"name"`
			Category  string  `json:"category"`
			Brand     string  `json:"brand"`
			Revenue   float64 `json:"revenue"`
			Units     int64   `json:"units"`
			MarginPct float64 `json:"margin_pct"`
		}

		var data []Product
		for rows.Next() {
			var prod Product
			if err := rows.Scan(&prod.Name, &prod.Category, &prod.Brand, &prod.Revenue, &prod.Units, &prod.MarginPct); err != nil {
				continue
			}
			data = append(data, prod)
		}

		c.JSON(http.StatusOK, data)
	}
}

// cardActions uses the LLM to generate exactly 3 recommended actions for a dashboard card.
func cardActions(llmClient llm.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CardType string `json:"card_type" binding:"required"`
			CardData any    `json:"card_data"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "card_type is required"})
			return
		}

		cardJSON := "N/A"
		if req.CardData != nil {
			if b, err := json.MarshalIndent(req.CardData, "", "  "); err == nil {
				cardJSON = string(b)
			}
		}

		systemPrompt := prompts.Get("card_actions.md")
		userPrompt := fmt.Sprintf("Dashboard Card: %s\n\nData:\n%s", req.CardType, cardJSON)

		slog.InfoContext(c.Request.Context(), "generating card actions", "card_type", req.CardType)

		response, err := llmClient.Generate(c.Request.Context(), systemPrompt, userPrompt)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "LLM card actions failed",
				"card_type", req.CardType, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate actions"})
			return
		}

		var actions []string
		for _, line := range strings.Split(response, "\n") {
			line = strings.TrimSpace(line)
			if m := numberedItemRe.FindStringSubmatch(line); m != nil {
				action := strings.TrimSpace(strings.ReplaceAll(m[1], "**", ""))
				if action != "" {
					actions = append(actions, action)
				}
			}
		}

		// Pad with defaults if LLM returned fewer than 3 items, cap at 3.
		for len(actions) < 3 {
			actions = append(actions, defaultCardActions[len(actions)])
		}
		actions = actions[:3]

		c.JSON(http.StatusOK, gin.H{"actions": actions})
	}
}

// explainCard uses the StrategistAgent (LLM) to analyze and explain a dashboard card's data.
func explainCard(llmClient llm.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CardType string `json:"card_type" binding:"required"`
			CardData any    `json:"card_data" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "card_type and card_data are required"})
			return
		}

		cardJSON, err := json.MarshalIndent(req.CardData, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize card data"})
			return
		}

		systemPrompt := prompts.Get("explain_card.md")
		userPrompt := fmt.Sprintf("Dashboard Card: %s\n\nData:\n%s", req.CardType, string(cardJSON))

		slog.InfoContext(c.Request.Context(), "explaining dashboard card",
			"card_type", req.CardType)

		explanation, err := llmClient.Generate(c.Request.Context(), systemPrompt, userPrompt)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "LLM explain card failed",
				"card_type", req.CardType, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate explanation"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"explanation": explanation})
	}
}
