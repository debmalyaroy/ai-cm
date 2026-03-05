package handlers

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterReportRoutes registers report-related API routes.
func RegisterReportRoutes(rg *gin.RouterGroup, db *pgxpool.Pool) {
	reports := rg.Group("/reports")
	{
		reports.GET("/download", downloadReport(db))
	}
}

// @Summary Download performance report
// @Description Stream a CSV report of regional and product performance
// @Tags reports
// @Produce text/csv
// @Success 200 {file} file
// @Failure 500 {object} map[string]string
// @Router /reports/download [get]
func downloadReport(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Starting report download")

		rows, err := db.Query(c, `
			SELECT p.name AS product, p.category, 
				l.region, l.city, 
				s.sale_date, s.quantity, s.revenue, s.margin, s.discount_pct
			FROM fact_sales s
			JOIN dim_products p ON s.product_id = p.id
			JOIN dim_locations l ON s.location_id = l.id
			ORDER BY s.sale_date DESC, l.region, p.name
			LIMIT 1000
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to query for report", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate report"})
			return
		}
		defer rows.Close()

		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment;filename=performance_report_%s.csv", time.Now().Format("20060102_150405")))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Write Header
		if err := writer.Write([]string{"Product", "Category", "Region", "City", "Sale Date", "Quantity", "Revenue", "Margin", "Discount %"}); err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to write CSV header", "error", err)
			return
		}

		// Write Data
		for rows.Next() {
			var product, category, region, city string
			var saleDate time.Time
			var quantity int
			var revenue, margin, discountPct float64

			if err := rows.Scan(&product, &category, &region, &city, &saleDate, &quantity, &revenue, &margin, &discountPct); err != nil {
				slog.WarnContext(c.Request.Context(), "Failed to scan report row", "error", err)
				continue
			}

			row := []string{
				product,
				category,
				region,
				city,
				saleDate.Format("2006-01-02"),
				fmt.Sprintf("%d", quantity),
				fmt.Sprintf("%.2f", revenue),
				fmt.Sprintf("%.2f", margin),
				fmt.Sprintf("%.1f", discountPct),
			}
			if err := writer.Write(row); err != nil {
				slog.ErrorContext(c.Request.Context(), "Failed to write CSV row", "error", err)
				return
			}
		}

		slog.InfoContext(c.Request.Context(), "Successfully generated CSV report")
	}
}
