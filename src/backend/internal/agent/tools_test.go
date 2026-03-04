package agent

import (
	"context"
	"testing"
)

func TestSanitizeSQL_AllowsSelect(t *testing.T) {
	cases := []string{
		"SELECT * FROM fact_sales",
		"SELECT p.name, SUM(s.revenue) FROM fact_sales s JOIN dim_products p ON s.product_id = p.id GROUP BY p.name",
		"  SELECT COUNT(*) FROM dim_products  ",
		"SELECT * FROM fact_sales WHERE sale_date >= CURRENT_DATE - INTERVAL '3 months'",
		"WITH cte AS (SELECT 1) SELECT * FROM cte",
	}

	for _, sql := range cases {
		if err := SanitizeSQL(sql); err != nil {
			t.Errorf("SanitizeSQL(%q) should allow SELECT but got error: %v", sql, err)
		}
	}
}

func TestSanitizeSQL_BlocksWriteOps(t *testing.T) {
	cases := []struct {
		sql     string
		keyword string
	}{
		{"DROP TABLE fact_sales", "DROP"},
		{"DELETE FROM fact_sales WHERE id = 1", "DELETE"},
		{"UPDATE dim_products SET mrp = 0", "UPDATE"},
		{"INSERT INTO dim_products VALUES ('test')", "INSERT"},
		{"ALTER TABLE fact_sales ADD col TEXT", "ALTER"},
		{"TRUNCATE TABLE fact_sales", "TRUNCATE"},
		{"CREATE TABLE evil (id INT)", "CREATE"},
		{"GRANT ALL ON fact_sales TO evil", "GRANT"},
		{"REVOKE ALL ON fact_sales FROM user1", "REVOKE"},
		{"EXEC sp_help", "EXEC"},
		{"EXECUTE sp_help", "EXECUTE"},
	}

	for _, tc := range cases {
		err := SanitizeSQL(tc.sql)
		if err == nil {
			t.Errorf("SanitizeSQL(%q) should block %s but allowed it", tc.sql, tc.keyword)
		}
	}
}

func TestSanitizeSQL_BlocksMultiStatement(t *testing.T) {
	cases := []string{
		"SELECT * FROM fact_sales; DROP TABLE fact_sales",
		"SELECT 1; DELETE FROM dim_products",
		"SELECT 1;INSERT INTO dim_products VALUES ('x')",
	}

	for _, sql := range cases {
		err := SanitizeSQL(sql)
		if err == nil {
			t.Errorf("SanitizeSQL(%q) should block multi-statement injection but allowed it", sql)
		}
	}
}

func TestSanitizeSQL_EdgeCases(t *testing.T) {
	// Empty and whitespace-only
	if err := SanitizeSQL(""); err != nil {
		t.Errorf("empty SQL should be allowed: %v", err)
	}
	if err := SanitizeSQL("   "); err != nil {
		t.Errorf("whitespace SQL should be allowed: %v", err)
	}

	// Tab-separated keywords
	err := SanitizeSQL("DROP\ttable_name")
	if err == nil {
		t.Error("tab-separated DROP should be blocked")
	}

	// Newline-separated keywords
	err = SanitizeSQL("DROP\ntable_name")
	if err == nil {
		t.Error("newline-separated DROP should be blocked")
	}
}

// --- ToolSet tests ---

func TestNewToolSet(t *testing.T) {
	ts := NewToolSet(nil, nil)
	if ts == nil {
		t.Fatal("NewToolSet should return non-nil")
	}

	tools := ts.List()
	if len(tools) != 1 {
		t.Errorf("expected 1 default tool, got %d", len(tools))
	}
}

func TestToolSet_Register(t *testing.T) {
	ts := &ToolSet{tools: make(map[string]Tool)}
	ts.Register(&RunSQLTool{})

	if len(ts.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(ts.tools))
	}
}

func TestToolSet_Get(t *testing.T) {
	ts := NewToolSet(nil, nil)

	tool, ok := ts.Get("run_sql")
	if !ok {
		t.Error("should find run_sql")
	}
	if tool.Name() != "run_sql" {
		t.Errorf("name = %q", tool.Name())
	}

	_, ok = ts.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent tool")
	}
}

func TestToolSet_List(t *testing.T) {
	ts := NewToolSet(nil, nil)
	tools := ts.List()
	if len(tools) < 1 {
		t.Errorf("expected at least 1 tool, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	if !names["run_sql"] {
		t.Error("missing run_sql in tool list")
	}
}

// --- Tool name and description tests ---

func TestRunSQLToolMeta(t *testing.T) {
	tool := &RunSQLTool{}
	if tool.Name() != "run_sql" {
		t.Errorf("name = %q", tool.Name())
	}
	desc := tool.Description()
	if len(desc) < 20 {
		t.Errorf("description too short: %q", desc)
	}
}

func TestRunSQLTool_MissingSQLParam(t *testing.T) {
	tool := &RunSQLTool{}
	_, err := tool.Execute(context.TODO(), map[string]any{})
	if err == nil {
		t.Error("should error on missing 'sql' parameter")
	}
}

func TestRunSQLTool_EmptySQLParam(t *testing.T) {
	tool := &RunSQLTool{}
	_, err := tool.Execute(context.TODO(), map[string]any{"sql": ""})
	if err == nil {
		t.Error("should error on empty 'sql' parameter")
	}
}

func TestRunSQLTool_BlockedSQL(t *testing.T) {
	tool := &RunSQLTool{}
	_, err := tool.Execute(context.TODO(), map[string]any{"sql": "DROP TABLE fact_sales"})
	if err == nil {
		t.Error("should block DROP TABLE via SanitizeSQL")
	}
}
