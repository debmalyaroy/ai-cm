package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WatchdogAgent monitors data quality, detects anomalies, and alerts on issues.
// Design §6 — Continuous Monitoring Pattern.
type WatchdogAgent struct {
	db           *pgxpool.Pool
	anomalyCache *ResultCache
}

// NewWatchdogAgent creates a new watchdog agent.
func NewWatchdogAgent(db *pgxpool.Pool) *WatchdogAgent {
	return &WatchdogAgent{
		db:           db,
		anomalyCache: NewResultCache(2 * time.Minute),
	}
}

func (w *WatchdogAgent) Name() string { return "watchdog" }

// AnomalyType classifies detected anomalies.
type AnomalyType string

const (
	AnomalyTypePriceDrop     AnomalyType = "price_drop"
	AnomalyTypeStockout      AnomalyType = "stockout_risk"
	AnomalyTypeSalesAnomaly  AnomalyType = "sales_anomaly"
	AnomalyTypeHighInventory AnomalyType = "high_inventory"
)

// Anomaly represents a detected issue.
type Anomaly struct {
	Type        AnomalyType `json:"type"`
	Severity    string      `json:"severity"` // "critical", "warning", "info"
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Confidence  float64     `json:"confidence"`
}

func (w *WatchdogAgent) Process(ctx context.Context, input *Input) (*Output, error) {
	output := &Output{AgentName: w.Name()}

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type: "thought", Content: "Running anomaly detection checks across all data sources",
	})

	slog.InfoContext(ctx, "Watchdog: starting anomaly detection checks")

	var anomalies []Anomaly

	// Determine whether to run standard interval checks or time-based checks
	isTimeBased := input != nil && input.Query == "time-based-check"

	if isTimeBased {
		slog.InfoContext(ctx, "Watchdog: executing specific time-based alerts")
		timeAnomalies, err := w.checkTimeBasedAlerts(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Watchdog: time-based check failed", "error", err)
		} else {
			anomalies = append(anomalies, timeAnomalies...)
			w.anomalyCache.Put("time_anomalies", timeAnomalies)
		}
	} else {
		// Check 1: Competitor price drops
		priceAnomalies, err := w.checkPriceAnomalies(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Watchdog: price check failed", "error", err)
		} else {
			anomalies = append(anomalies, priceAnomalies...)
			w.anomalyCache.Put("price_anomalies", priceAnomalies)
		}

		// Persist expected anomalies to the Alerts database
		if len(anomalies) > 0 {
			for _, a := range anomalies {
				// Skip saving if confidence is too low or it's just 'info' to avoid spam, but here we save all for Issue 2 testability
				_, err := w.db.Exec(ctx, `
				INSERT INTO alerts (title, severity, category, message, acknowledged)
				VALUES ($1, $2, $3, $4, FALSE)
			`, a.Title, a.Severity, string(a.Type), a.Description)
				if err != nil {
					slog.ErrorContext(ctx, "Watchdog: failed to save alert to db", "error", err, "title", a.Title)
				}
			}
		}

		// Check 2: Stockout risks
		stockAnomalies, err := w.checkStockoutRisks(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Watchdog: stock check failed", "error", err)
		} else {
			anomalies = append(anomalies, stockAnomalies...)
			w.anomalyCache.Put("stockout_risks", stockAnomalies)
		}

		// Check 3: Sales anomalies (sudden drops)
		salesAnomalies, err := w.checkSalesAnomalies(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Watchdog: sales check failed", "error", err)
		} else {
			anomalies = append(anomalies, salesAnomalies...)
			w.anomalyCache.Put("sales_anomalies", salesAnomalies)
		}

		// Check 4: Excess inventory
		invAnomalies, err := w.checkExcessInventory(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Watchdog: inventory check failed", "error", err)
		} else {
			anomalies = append(anomalies, invAnomalies...)
			w.anomalyCache.Put("excess_inventory", invAnomalies)
		}
	}

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type: "observation", Content: fmt.Sprintf("Detected %d anomalies across 4 checks", len(anomalies)),
	})

	// Build response
	if len(anomalies) == 0 {
		output.Response = "✅ **All systems nominal.** No anomalies detected across pricing, inventory, or sales data."
	} else {
		output.Response = fmt.Sprintf("🔔 **%d anomalies detected:**\n\n", len(anomalies))
		for i, a := range anomalies {
			icon := "ℹ️"
			if a.Severity == "critical" {
				icon = "🚨"
			} else if a.Severity == "warning" {
				icon = "⚠️"
			}
			output.Response += fmt.Sprintf("%d. %s **%s** — %s (Confidence: %.0f%%)\n",
				i+1, icon, a.Title, a.Description, a.Confidence*100)
		}
	}

	slog.InfoContext(ctx, "Watchdog: anomaly detection completed", "anomalies_count", len(anomalies))

	output.Data = anomalies
	return output, nil
}

func (w *WatchdogAgent) checkPriceAnomalies(ctx context.Context) ([]Anomaly, error) {
	slog.DebugContext(ctx, "Watchdog: executing price anomaly check")
	rows, err := w.db.Query(ctx, `
		SELECT p.name, cp.competitor_name, cp.price_diff_pct
		FROM fact_competitor_prices cp
		JOIN dim_products p ON cp.product_id = p.id
		WHERE cp.price_date >= CURRENT_DATE - INTERVAL '7 days'
		AND cp.price_diff_pct < -10
		ORDER BY cp.price_diff_pct ASC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var name, competitor string
		var diffPct float64
		if err := rows.Scan(&name, &competitor, &diffPct); err != nil {
			continue
		}
		anomalies = append(anomalies, Anomaly{
			Type:        AnomalyTypePriceDrop,
			Severity:    severityForPriceDrop(diffPct),
			Title:       fmt.Sprintf("Price Drop: %s", name),
			Description: fmt.Sprintf("%s undercut by %.1f%% by %s", name, -diffPct, competitor),
			Confidence:  0.9,
		})
	}
	return anomalies, nil
}

func (w *WatchdogAgent) checkStockoutRisks(ctx context.Context) ([]Anomaly, error) {
	slog.DebugContext(ctx, "Watchdog: executing stockout risk check")
	rows, err := w.db.Query(ctx, `
		SELECT p.name, l.region, i.days_of_supply, i.quantity_on_hand
		FROM fact_inventory i
		JOIN dim_products p ON i.product_id = p.id
		JOIN dim_locations l ON i.location_id = l.id
		WHERE i.days_of_supply < 7
		AND i.quantity_on_hand < i.reorder_level
		ORDER BY i.days_of_supply ASC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var name, region string
		var dos, qty int
		if err := rows.Scan(&name, &region, &dos, &qty); err != nil {
			continue
		}
		anomalies = append(anomalies, Anomaly{
			Type:        AnomalyTypeStockout,
			Severity:    "critical",
			Title:       fmt.Sprintf("Stockout Risk: %s (%s)", name, region),
			Description: fmt.Sprintf("Only %d units left, %d days of supply", qty, dos),
			Confidence:  0.95,
		})
	}
	return anomalies, nil
}

func (w *WatchdogAgent) checkSalesAnomalies(ctx context.Context) ([]Anomaly, error) {
	slog.DebugContext(ctx, "Watchdog: executing sales anomaly check")
	rows, err := w.db.Query(ctx, `
		WITH weekly AS (
			SELECT product_id,
			       SUM(CASE WHEN sale_date >= CURRENT_DATE - INTERVAL '7 days' THEN revenue ELSE 0 END) AS this_week,
			       SUM(CASE WHEN sale_date >= CURRENT_DATE - INTERVAL '14 days' 
			                 AND sale_date < CURRENT_DATE - INTERVAL '7 days' THEN revenue ELSE 0 END) AS last_week
			FROM fact_sales
			WHERE sale_date >= CURRENT_DATE - INTERVAL '14 days'
			GROUP BY product_id
			HAVING SUM(CASE WHEN sale_date >= CURRENT_DATE - INTERVAL '14 days' 
			                 AND sale_date < CURRENT_DATE - INTERVAL '7 days' THEN revenue ELSE 0 END) > 0
		)
		SELECT p.name, w.this_week, w.last_week,
		       ((w.this_week - w.last_week) / w.last_week * 100) AS pct_change
		FROM weekly w
		JOIN dim_products p ON w.product_id = p.id
		WHERE ((w.this_week - w.last_week) / w.last_week * 100) < -20
		ORDER BY pct_change ASC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var name string
		var thisWeek, lastWeek, pctChange float64
		if err := rows.Scan(&name, &thisWeek, &lastWeek, &pctChange); err != nil {
			continue
		}
		anomalies = append(anomalies, Anomaly{
			Type:        AnomalyTypeSalesAnomaly,
			Severity:    "warning",
			Title:       fmt.Sprintf("Sales Drop: %s", name),
			Description: fmt.Sprintf("Revenue dropped %.1f%% week-over-week (₹%.0f → ₹%.0f)", -pctChange, lastWeek, thisWeek),
			Confidence:  0.8,
		})
	}
	return anomalies, nil
}

func (w *WatchdogAgent) checkExcessInventory(ctx context.Context) ([]Anomaly, error) {
	slog.DebugContext(ctx, "Watchdog: executing excess inventory check")
	rows, err := w.db.Query(ctx, `
		SELECT p.name, l.region, i.quantity_on_hand, i.days_of_supply
		FROM fact_inventory i
		JOIN dim_products p ON i.product_id = p.id
		JOIN dim_locations l ON i.location_id = l.id
		WHERE i.days_of_supply > 90
		AND i.quantity_on_hand > 300
		ORDER BY i.days_of_supply DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var name, region string
		var qty, dos int
		if err := rows.Scan(&name, &region, &qty, &dos); err != nil {
			continue
		}
		anomalies = append(anomalies, Anomaly{
			Type:        AnomalyTypeHighInventory,
			Severity:    "info",
			Title:       fmt.Sprintf("Excess Inventory: %s (%s)", name, region),
			Description: fmt.Sprintf("%d units with %d days of supply", qty, dos),
			Confidence:  0.75,
		})
	}
	return anomalies, nil
}

func severityForPriceDrop(pct float64) string {
	if pct <= -20 {
		return "critical"
	}
	if pct <= -15 {
		return "warning"
	}
	return "info"
}

// checkTimeBasedAlerts simulates a daily goal or summary check that runs on a specific time schedule.
func (w *WatchdogAgent) checkTimeBasedAlerts(ctx context.Context) ([]Anomaly, error) {
	slog.DebugContext(ctx, "Watchdog: executing daily time-based alert check")
	// Example: Daily check for pending actions that need attention
	rows, err := w.db.Query(ctx, `
		SELECT COUNT(*) 
		FROM action_log 
		WHERE status = 'pending' 
		AND created_at < NOW() - INTERVAL '2 days'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var staleCount int
	if rows.Next() {
		if err := rows.Scan(&staleCount); err == nil && staleCount > 0 {
			return []Anomaly{{
				Type:        "stale_actions",
				Severity:    "warning",
				Title:       "Stale Pending Actions",
				Description: fmt.Sprintf("You have %d pending actions older than 48 hours. Please review them.", staleCount),
				Confidence:  1.0,
			}}, nil
		}
	}
	return nil, nil
}
