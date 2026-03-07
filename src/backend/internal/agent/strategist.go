package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StrategistAgent implements Chain-of-Thought reasoning for insights.
type StrategistAgent struct {
	llmClient    llm.Client
	tools        *ToolSet
	db           *pgxpool.Pool
	contextCache *ResultCache
}

// NewStrategistAgent creates a new Strategist agent.
func NewStrategistAgent(llmClient llm.Client, db *pgxpool.Pool, tools *ToolSet) *StrategistAgent {
	return &StrategistAgent{
		llmClient:    llmClient,
		tools:        tools,
		db:           db,
		contextCache: NewResultCache(5 * time.Minute),
	}
}

func (s *StrategistAgent) Name() string { return "strategist" }

func (s *StrategistAgent) Process(ctx context.Context, input *Input) (*Output, error) {
	output := &Output{
		AgentName: s.Name(),
		Reasoning: []ReasoningStep{},
	}

	// Gather supporting data for reasoning
	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type:    "thought",
		Content: "I need to gather relevant data to provide a thorough analysis.",
	})

	// Get relevant context data via SQL
	slog.DebugContext(ctx, "Strategist: starting chain-of-thought analysis")
	contextData := s.gatherContext(ctx, input.Query)

	keys := make([]string, 0, len(contextData))
	for k := range contextData {
		keys = append(keys, k)
	}
	slog.DebugContext(ctx, "Strategist: context data gathered", "keys", keys)

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type:    "observation",
		Content: fmt.Sprintf("Gathered %d data points for context", len(contextData)),
	})

	// Build Chain-of-Thought prompt
	systemPrompt := prompts.Get("strategist.md")

	contextJSON, _ := json.MarshalIndent(contextData, "", "  ")
	userPrompt := fmt.Sprintf("User Question: %s\n\nRelevant Data Context:\n%s", input.Query, string(contextJSON))

	// Add data from previous agent if available
	if input.Context != nil {
		if prevData, ok := input.Context["analyst_data"]; ok {
			prevJSON, _ := json.MarshalIndent(prevData, "", "  ")
			userPrompt += fmt.Sprintf("\n\nAnalyst Data (from previous step):\n%s", string(prevJSON))
		}
	}

	userPrompt += formatMemoryContext(input.MemoryContext)

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type:    "thought",
		Content: "Applying Chain-of-Thought reasoning to generate strategic insight",
	})

	slog.DebugContext(ctx, "Strategist: calling LLM for insight", "query", input.Query)
	response, err := s.llmClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("strategist generate: %w", err)
	}

	output.Response = response
	return output, nil
}

// gatherContext runs contextual queries to support the strategist's reasoning.
func (s *StrategistAgent) gatherContext(ctx context.Context, query string) map[string]any {
	data := make(map[string]any)

	queries := map[string]string{
		"sales_by_region": `SELECT l.region, SUM(s.revenue) as total_revenue, AVG(s.margin/NULLIF(s.revenue,0)*100) as avg_margin_pct
			FROM fact_sales s JOIN dim_locations l ON s.location_id = l.id
			WHERE s.sale_date >= CURRENT_DATE - INTERVAL '3 months'
			GROUP BY l.region ORDER BY total_revenue DESC`,

		"category_performance": `SELECT p.category, SUM(s.revenue) as total_revenue, SUM(s.quantity) as total_units,
			AVG(s.discount_pct) as avg_discount
			FROM fact_sales s JOIN dim_products p ON s.product_id = p.id
			WHERE s.sale_date >= CURRENT_DATE - INTERVAL '3 months'
			GROUP BY p.category ORDER BY total_revenue DESC`,

		"inventory_alerts": `SELECT p.name, p.category, l.region, i.quantity_on_hand, i.reorder_level, i.days_of_supply
			FROM fact_inventory i 
			JOIN dim_products p ON i.product_id = p.id
			JOIN dim_locations l ON i.location_id = l.id
			WHERE i.quantity_on_hand < i.reorder_level
			ORDER BY i.days_of_supply ASC LIMIT 10`,

		"competitor_prices": `SELECT p.name, cp.competitor_name, p.mrp, cp.competitor_price, cp.price_diff_pct
			FROM fact_competitor_prices cp
			JOIN dim_products p ON cp.product_id = p.id
			WHERE cp.price_date >= CURRENT_DATE - INTERVAL '7 days'
			AND cp.price_diff_pct < -5
			ORDER BY cp.price_diff_pct ASC LIMIT 10`,
	}

	sqlTool, ok := s.tools.Get("run_sql")
	if !ok {
		return data
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for key, sql := range queries {
		// Serve from cache without spawning a goroutine
		if cached, ok := s.contextCache.Get(key); ok {
			data[key] = cached
			slog.DebugContext(ctx, "Strategist: context cache hit", "key", key)
			continue
		}

		// Run independent DB queries in parallel
		wg.Add(1)
		go func(k, q string) {
			defer wg.Done()
			result, err := sqlTool.Execute(ctx, map[string]any{"sql": q})
			if err != nil {
				slog.WarnContext(ctx, "Strategist context query failed", "query_key", k, "error", err)
				return
			}
			mu.Lock()
			data[k] = result
			s.contextCache.Put(k, result)
			mu.Unlock()
		}(key, sql)
	}

	wg.Wait()
	return data
}
