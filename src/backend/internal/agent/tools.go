package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tool is the interface for agent tools.
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, params map[string]any) (any, error)
}

// --- SQL Sanitizer ---

var blockedKeywords = []string{
	"DROP", "DELETE", "UPDATE", "INSERT", "ALTER", "TRUNCATE",
	"CREATE", "GRANT", "REVOKE", "EXEC", "EXECUTE",
}

// SanitizeSQL checks if the SQL contains any write operations.
func SanitizeSQL(sql string) error {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	for _, kw := range blockedKeywords {
		// Check for keyword at start of statement or after semicolons
		if strings.HasPrefix(upper, kw+" ") || strings.HasPrefix(upper, kw+"\n") || strings.HasPrefix(upper, kw+"\t") {
			return fmt.Errorf("blocked SQL operation: %s", kw)
		}
		// Check within the query (for multi-statement injection)
		if strings.Contains(upper, "; "+kw+" ") || strings.Contains(upper, ";"+kw+" ") {
			return fmt.Errorf("blocked SQL operation: %s (multi-statement)", kw)
		}
	}
	return nil
}

// --- RunSQLTool ---

// RunSQLTool executes read-only SQL queries.
type RunSQLTool struct {
	DB *pgxpool.Pool
}

func (t *RunSQLTool) Name() string { return "run_sql" }
func (t *RunSQLTool) Description() string {
	return "Execute a read-only SQL query against the PostgreSQL database and return results as JSON rows. Only SELECT queries are allowed."
}

func (t *RunSQLTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	sqlStr, ok := params["sql"].(string)
	if !ok || sqlStr == "" {
		return nil, fmt.Errorf("'sql' parameter is required")
	}

	// Sanitize SQL
	if err := SanitizeSQL(sqlStr); err != nil {
		return nil, err
	}

	rows, err := t.DB.Query(ctx, sqlStr)
	if err != nil {
		return nil, fmt.Errorf("execute SQL: %w", err)
	}
	defer rows.Close()

	// Get column names
	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = string(f.Name)
	}

	// Collect results
	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]any{}
	}

	return map[string]any{
		"columns":   columns,
		"rows":      results,
		"row_count": len(results),
	}, nil
}

// --- ToolSet ---

// ToolSet holds all available tools for agents.
type ToolSet struct {
	tools map[string]Tool
}

// NewToolSet creates a tool set with all available tools.
func NewToolSet(db *pgxpool.Pool, llmClient llm.Client) *ToolSet {
	ts := &ToolSet{tools: make(map[string]Tool)}

	ts.Register(&RunSQLTool{DB: db})

	return ts
}

func (ts *ToolSet) Register(t Tool) {
	ts.tools[t.Name()] = t
}

func (ts *ToolSet) Get(name string) (Tool, bool) {
	t, ok := ts.tools[name]
	return t, ok
}

func (ts *ToolSet) List() []Tool {
	tools := make([]Tool, 0, len(ts.tools))
	for _, t := range ts.tools {
		tools = append(tools, t)
	}
	return tools
}
