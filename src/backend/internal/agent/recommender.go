package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Recommender generates action recommendations based on heuristic analysis.
type Recommender struct {
	db *pgxpool.Pool
}

// NewRecommender creates a new action recommender.
func NewRecommender(db *pgxpool.Pool) *Recommender {
	return &Recommender{db: db}
}

// GenerateActions analyzes current data and creates action recommendations.
func (r *Recommender) GenerateActions(ctx context.Context) ([]ActionSuggestion, error) {
	var actions []ActionSuggestion
	slog.InfoContext(ctx, "Recommender: starting action generation")

	// Rule 1: Competitor price drops > 5% → Price Match
	priceActions, err := r.checkCompetitorPrices(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Price check failed", "error", err)
	} else {
		actions = append(actions, priceActions...)
	}

	// Rule 2: Low inventory → Restock
	restockActions, err := r.checkLowInventory(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Inventory check failed", "error", err)
	} else {
		actions = append(actions, restockActions...)
	}

	// Rule 3: High inventory → Run Promotion
	promoActions, err := r.checkHighInventory(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "High inventory check failed", "error", err)
	} else {
		actions = append(actions, promoActions...)
	}

	return actions, nil
}

func (r *Recommender) checkCompetitorPrices(ctx context.Context) ([]ActionSuggestion, error) {
	query := `
		SELECT p.name, p.category, cp.competitor_name, p.mrp, cp.competitor_price, cp.price_diff_pct
		FROM fact_competitor_prices cp
		JOIN dim_products p ON cp.product_id = p.id
		WHERE cp.price_date >= CURRENT_DATE - INTERVAL '7 days'
		AND cp.price_diff_pct < -5
		ORDER BY cp.price_diff_pct ASC
		LIMIT 5`

	slog.DebugContext(ctx, "Recommender: checking competitor prices")
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ActionSuggestion
	for rows.Next() {
		var name, category, competitor string
		var mrp, compPrice, diffPct float64
		if err := rows.Scan(&name, &category, &competitor, &mrp, &compPrice, &diffPct); err != nil {
			continue
		}

		confidence := 0.7 + ((-diffPct - 5) / 50) // Higher diff = higher confidence
		if confidence > 0.95 {
			confidence = 0.95
		}

		actions = append(actions, ActionSuggestion{
			Title:       fmt.Sprintf("Price Match: %s", name),
			Description: fmt.Sprintf("%s is selling at ₹%.0f (%.1f%% lower than MRP ₹%.0f). Recommend matching price to prevent market share loss.", competitor, compPrice, -diffPct, mrp),
			ActionType:  "price_match",
			Confidence:  confidence,
		})
	}
	return actions, nil
}

func (r *Recommender) checkLowInventory(ctx context.Context) ([]ActionSuggestion, error) {
	query := `
		SELECT p.name, p.category, l.region, i.quantity_on_hand, i.reorder_level, i.days_of_supply
		FROM fact_inventory i
		JOIN dim_products p ON i.product_id = p.id
		JOIN dim_locations l ON i.location_id = l.id
		WHERE i.quantity_on_hand < i.reorder_level
		AND i.days_of_supply < 10
		ORDER BY i.days_of_supply ASC
		LIMIT 5`

	slog.DebugContext(ctx, "Recommender: checking low inventory")
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ActionSuggestion
	for rows.Next() {
		var name, category, region string
		var qty, reorder, dos int
		if err := rows.Scan(&name, &category, &region, &qty, &reorder, &dos); err != nil {
			continue
		}

		confidence := 0.8 + float64(reorder-qty)/float64(reorder)*0.15
		if confidence > 0.95 {
			confidence = 0.95
		}

		actions = append(actions, ActionSuggestion{
			Title:       fmt.Sprintf("Restock Alert: %s (%s)", name, region),
			Description: fmt.Sprintf("Current stock is %d units (reorder level: %d) in %s. Predicted stockout in %d days. Immediate reorder recommended.", qty, reorder, region, dos),
			ActionType:  "restock",
			Confidence:  confidence,
		})
	}
	return actions, nil
}

func (r *Recommender) checkHighInventory(ctx context.Context) ([]ActionSuggestion, error) {
	query := `
		SELECT p.name, p.category, l.region, i.quantity_on_hand, i.days_of_supply
		FROM fact_inventory i
		JOIN dim_products p ON i.product_id = p.id
		JOIN dim_locations l ON i.location_id = l.id
		WHERE i.days_of_supply > 60
		AND i.quantity_on_hand > 200
		ORDER BY i.quantity_on_hand DESC
		LIMIT 5`

	slog.DebugContext(ctx, "Recommender: checking high inventory")
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ActionSuggestion
	for rows.Next() {
		var name, category, region string
		var qty, dos int
		if err := rows.Scan(&name, &category, &region, &qty, &dos); err != nil {
			continue
		}

		actions = append(actions, ActionSuggestion{
			Title:       fmt.Sprintf("Run Promotion: %s (%s)", name, region),
			Description: fmt.Sprintf("Inventory is %d units with %d days of supply in %s. Consider running a 10-15%% discount promotion to accelerate sell-through.", qty, dos, region),
			ActionType:  "promotion",
			Confidence:  0.72,
		})
	}
	return actions, nil
}
