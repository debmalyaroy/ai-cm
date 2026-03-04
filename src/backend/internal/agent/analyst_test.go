package agent

import (
	"context"
	"testing"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
)

// ---------------------------------------------------------------
// cleanSQL tests
// ---------------------------------------------------------------

func TestCleanSQL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"plain SQL unchanged",
			"SELECT * FROM fact_sales",
			"SELECT * FROM fact_sales",
		},
		{
			"removes sql code block",
			"```sql\nSELECT * FROM fact_sales\n```",
			"SELECT * FROM fact_sales",
		},
		{
			"removes SQL code block (uppercase)",
			"```SQL\nSELECT * FROM fact_sales\n```",
			"SELECT * FROM fact_sales",
		},
		{
			"removes plain code block",
			"```\nSELECT * FROM fact_sales\n```",
			"SELECT * FROM fact_sales",
		},
		{
			"trims whitespace",
			"  SELECT * FROM fact_sales  ",
			"SELECT * FROM fact_sales",
		},
		{
			"handles empty string",
			"",
			"",
		},
		// New: prose with embedded SELECT
		{
			"extracts SELECT from prose",
			"To check inventory levels, we can use the following query:\nSELECT dp.name, fi.quantity_on_hand FROM fact_inventory fi JOIN dim_products dp ON fi.product_id = dp.id;",
			"SELECT dp.name, fi.quantity_on_hand FROM fact_inventory fi JOIN dim_products dp ON fi.product_id = dp.id;",
		},
		// New: prose with embedded WITH clause — SELECT is extracted first since
		// WITH appears commonly in English prose
		{
			"extracts WITH from prose",
			"Here is how to calculate the running average:\nWITH avg_sales AS (SELECT product_id, AVG(revenue) as avg_rev FROM fact_sales GROUP BY product_id) SELECT * FROM avg_sales;",
			"SELECT product_id, AVG(revenue) as avg_rev FROM fact_sales GROUP BY product_id) SELECT * FROM avg_sales;",
		},
		// New: multi-paragraph prose with SQL buried inside
		{
			"extracts SQL from multi-paragraph prose",
			"To execute the query to check inventory levels for Active SKUs, we can follow these steps:\n\n1. Get the total quantity of stock for Active SKUs from the fact_inventory table.\n2. Filter the results to include only products with a valid stock date.\n\nSELECT dp.name, SUM(fi.quantity_on_hand) FROM fact_inventory fi JOIN dim_products dp ON fi.product_id = dp.id GROUP BY dp.name;\n\nThis will give you the required data.",
			"SELECT dp.name, SUM(fi.quantity_on_hand) FROM fact_inventory fi JOIN dim_products dp ON fi.product_id = dp.id GROUP BY dp.name;",
		},
		// New: SQL code block takes priority over embedded SQL
		{
			"code block takes priority over prose SQL",
			"Here is the query:\n```sql\nSELECT * FROM fact_sales LIMIT 10\n```\nAlternatively you could SELECT * FROM dim_products;",
			"SELECT * FROM fact_sales LIMIT 10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanSQL(context.Background(), tc.input)
			if got != tc.want {
				t.Errorf("cleanSQL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestLooksLikeSQL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"SELECT statement", "SELECT * FROM foo", true},
		{"WITH clause", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"lowercase select", "select * from foo", true}, // looksLikeSQL uppercases before checking
		{"INSERT statement", "INSERT INTO foo VALUES (1)", true},
		{"prose text", "To check inventory levels, we need to...", false},
		{"empty string", "", false},
		{"SQL with leading comment", "-- this is a comment\nSELECT 1", true},
		{"SQL with leading block comment", "/* block comment */\nSELECT 1", true}, // skips comment line correctly
		{"conversational with", "With the data provided, the margin is...", false},
		{"select without trailing space", "SELECT\n* FROM foo", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikeSQL(tc.input)
			if got != tc.want {
				t.Errorf("looksLikeSQL(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------
// SQLCache tests
// ---------------------------------------------------------------

func TestSQLCache_BasicOperations(t *testing.T) {
	cache := NewSQLCache(1*time.Minute, 10)

	// Miss
	if _, ok := cache.Get("show sales by region"); ok {
		t.Fatal("expected cache miss on empty cache")
	}

	// Put + Hit
	cache.Put("show sales by region", "SELECT region, SUM(revenue) FROM fact_sales GROUP BY region")
	sql, ok := cache.Get("show sales by region")
	if !ok {
		t.Fatal("expected cache hit after Put")
	}
	if sql != "SELECT region, SUM(revenue) FROM fact_sales GROUP BY region" {
		t.Errorf("got %q, want the cached SQL", sql)
	}

	// Normalized key: same query with different word order should hit
	sql2, ok2 := cache.Get("by region show sales")
	if !ok2 {
		t.Fatal("expected cache hit for reordered query")
	}
	if sql2 != sql {
		t.Error("expected same SQL for reordered query")
	}
}

func TestSQLCache_TTLExpiry(t *testing.T) {
	cache := NewSQLCache(50*time.Millisecond, 10)

	cache.Put("test query", "SELECT 1")
	time.Sleep(100 * time.Millisecond)

	_, ok := cache.Get("test query")
	if ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

func TestSQLCache_LRUEviction(t *testing.T) {
	cache := NewSQLCache(1*time.Minute, 3)

	cache.Put("query one", "SELECT 1")
	time.Sleep(10 * time.Millisecond) // ensure different timestamps
	cache.Put("query two", "SELECT 2")
	time.Sleep(10 * time.Millisecond)
	cache.Put("query three", "SELECT 3")

	// At capacity — adding a 4th should evict "query one" (oldest)
	cache.Put("query four", "SELECT 4")

	if cache.Size() != 3 {
		t.Errorf("expected cache size 3, got %d", cache.Size())
	}

	if _, ok := cache.Get("query one"); ok {
		t.Fatal("expected 'query one' to be evicted")
	}
	if _, ok := cache.Get("query four"); !ok {
		t.Fatal("expected 'query four' to be present")
	}
}

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		expect string
	}{
		{
			"removes stop words and sorts",
			"Check inventory levels for Avg Margin",
			"avg,check,inventory,levels,margin",
		},
		{
			"case insensitive",
			"SHOW Sales By Region",
			"region,sales",
		},
		{
			"strips punctuation",
			"What's the top 5 products?",
			"5,products,top,what's", // mid-word apostrophe preserved
		},
		{
			"same key for reordered words",
			"for inventory levels check margin avg",
			"avg,check,inventory,levels,margin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeQuery(tc.query)
			if got != tc.expect {
				t.Errorf("normalizeQuery(%q) = %q, want %q", tc.query, got, tc.expect)
			}
		})
	}
}

// ---------------------------------------------------------------
// ResultCache tests
// ---------------------------------------------------------------

func TestResultCache_BasicOperations(t *testing.T) {
	cache := NewResultCache(1 * time.Minute)

	// Miss
	if _, ok := cache.Get("key1"); ok {
		t.Fatal("expected cache miss")
	}

	// Put + Hit
	cache.Put("key1", map[string]any{"result": "data"})
	data, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestResultCache_TTLExpiry(t *testing.T) {
	cache := NewResultCache(50 * time.Millisecond)

	cache.Put("key1", "value")
	time.Sleep(100 * time.Millisecond)

	_, ok := cache.Get("key1")
	if ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

// ---------------------------------------------------------------
// AnalystAgent.Process tests (with non-SQL retry)
// ---------------------------------------------------------------

func TestAnalystAgent_Process_NonSQLRetry(t *testing.T) {
	prompts.Init("../../../prompts")

	schemaTool := &mockSchemaTool{}
	sqlTool := &mockSQLTool{}

	// Mock LLM: first call returns prose, second call returns valid SQL
	callCount := 0
	llmc := &mockLLMCallbackClient{
		callback: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			callCount++
			if callCount == 1 {
				return "To check inventory levels, first query the fact_inventory table...", nil
			}
			return "SELECT * FROM fact_inventory", nil
		},
	}

	tools := NewToolSet(nil, llmc)
	tools.Register(schemaTool)
	tools.Register(sqlTool)

	agent := NewAnalystAgent(llmc, nil, tools)
	agent.schemaCache.SetFormattedSchemaForTest("mock schema")

	out, err := agent.Process(context.Background(), &Input{Query: "check inventory levels"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.AgentName != "analyst" {
		t.Errorf("expected agent name analyst, got %s", out.AgentName)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 LLM calls (retry), got %d", callCount)
	}
}

func TestAnalystAgent_Process_AllRetriesFail(t *testing.T) {
	prompts.Init("../../../prompts")

	schemaTool := &mockSchemaTool{}

	// Mock LLM: always returns prose
	llmc := &mockLLMCallbackClient{
		callback: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			return "Here is my analysis of the data...", nil
		},
	}

	tools := NewToolSet(nil, llmc)
	tools.Register(schemaTool)

	agent := NewAnalystAgent(llmc, nil, tools)
	agent.schemaCache.SetFormattedSchemaForTest("mock schema")

	out, err := agent.Process(context.Background(), &Input{Query: "check inventory levels"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Response == "" {
		t.Fatal("expected graceful error message in response")
	}
	if out.Response != "I was unable to generate a valid SQL query for your request. Please try rephrasing your question with more specific details." {
		t.Errorf("unexpected response: %s", out.Response)
	}
}

// mockLLMCallbackClient allows per-call control over LLM responses.
type mockLLMCallbackClient struct {
	callback func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

func (m *mockLLMCallbackClient) Name() string                      { return "mock-callback" }
func (m *mockLLMCallbackClient) WithModel(model string) llm.Client { return m }
func (m *mockLLMCallbackClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.callback(ctx, systemPrompt, userPrompt)
}
func (m *mockLLMCallbackClient) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan llm.StreamChunk, error) {
	resp, err := m.callback(ctx, systemPrompt, userPrompt)
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Content: resp, Done: true, Error: err}
	close(ch)
	return ch, err
}

// ---------------------------------------------------------------
// Shared mocks (used by multiple test files in the package)
// ---------------------------------------------------------------

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Name() string                      { return "mock" }
func (m *mockLLMClient) WithModel(model string) llm.Client { return m }
func (m *mockLLMClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.response, m.err
}
func (m *mockLLMClient) GenerateStream(ctx context.Context, systemPrompt, userPrompt string) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Content: m.response, Done: true, Error: m.err}
	close(ch)
	return ch, m.err
}

// Mock tool for get_table_schemas
type mockSchemaTool struct {
	err error
}

func (m *mockSchemaTool) Name() string        { return "get_table_schemas" }
func (m *mockSchemaTool) Description() string { return "Mock info" }
func (m *mockSchemaTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []string{"table1", "table2"}, nil
}

// Mock tool for run_sql
type mockSQLTool struct {
	err error
}

func (m *mockSQLTool) Name() string        { return "run_sql" }
func (m *mockSQLTool) Description() string { return "Mock run" }
func (m *mockSQLTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return map[string]any{
		"columns":   []string{"col1"},
		"rows":      []map[string]any{{"col1": "val1"}},
		"row_count": 1,
	}, nil
}
