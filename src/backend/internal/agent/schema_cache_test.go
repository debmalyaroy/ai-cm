package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewSchemaCache(t *testing.T) {
	sc := NewSchemaCache(nil, 30*time.Minute)
	if sc == nil {
		t.Fatal("NewSchemaCache should return non-nil")
	}
	if sc.ttl != 30*time.Minute {
		t.Errorf("ttl = %v, want 30m", sc.ttl)
	}
	if len(sc.allowedPrefixes) == 0 {
		t.Error("allowedPrefixes should not be empty")
	}
}

func TestSchemaCache_SetAndGetFormattedSchema(t *testing.T) {
	sc := NewSchemaCache(nil, 30*time.Minute)
	const testSchema = "## dim_products\nColumns:\n  - id (UUID, PK)\n"
	sc.SetFormattedSchemaForTest(testSchema)

	got, err := sc.GetFormattedSchema(context.Background())
	if err != nil {
		t.Fatalf("GetFormattedSchema returned error: %v", err)
	}
	if got != testSchema {
		t.Errorf("GetFormattedSchema = %q, want %q", got, testSchema)
	}
}

func TestSchemaCache_GetFormattedSchema_CacheHit(t *testing.T) {
	sc := NewSchemaCache(nil, 30*time.Minute)
	sc.SetFormattedSchemaForTest("cached schema")

	// Call twice to exercise the cache-hit branch.
	for i := 0; i < 2; i++ {
		got, err := sc.GetFormattedSchema(context.Background())
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if got != "cached schema" {
			t.Errorf("call %d: got %q, want %q", i+1, got, "cached schema")
		}
	}
}

// TestSchemaCache_GetFormattedSchema_NilDB verifies that when the cache is
// empty and the DB is nil, Refresh panics (nil pointer dereference on the pool).
// We use recover() to catch the panic and assert it occurred.
func TestSchemaCache_GetFormattedSchema_NilDB(t *testing.T) {
	sc := NewSchemaCache(nil, 30*time.Minute)
	// Do NOT call SetFormattedSchemaForTest — cache is empty, must hit Refresh.

	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		sc.GetFormattedSchema(context.Background()) //nolint:errcheck
	}()

	if !didPanic {
		t.Error("expected panic when refreshing with nil DB")
	}
}

// --- isAllowed ---

func TestSchemaCache_IsAllowed_DimPrefix(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	if !sc.isAllowed("dim_products") {
		t.Error("dim_products should be allowed")
	}
	if !sc.isAllowed("dim_locations") {
		t.Error("dim_locations should be allowed")
	}
}

func TestSchemaCache_IsAllowed_FactPrefix(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	if !sc.isAllowed("fact_sales") {
		t.Error("fact_sales should be allowed")
	}
	if !sc.isAllowed("fact_inventory") {
		t.Error("fact_inventory should be allowed")
	}
}

func TestSchemaCache_IsAllowed_ExactMatch(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	if !sc.isAllowed("alerts") {
		t.Error("'alerts' exact match should be allowed")
	}
	if !sc.isAllowed("action_log") {
		t.Error("'action_log' exact match should be allowed")
	}
}

func TestSchemaCache_IsAllowed_NotAllowed(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	disallowed := []string{"agent_memory", "business_context", "chat_messages", "chat_sessions", "cron_jobs", "user_preferences"}
	for _, tbl := range disallowed {
		if sc.isAllowed(tbl) {
			t.Errorf("table %q should NOT be allowed", tbl)
		}
	}
}

// --- extractIndexColumns ---

func TestExtractIndexColumns_Standard(t *testing.T) {
	def := "CREATE INDEX idx_fact_sales_date ON public.fact_sales USING btree (sale_date)"
	got := extractIndexColumns(def)
	if got != "sale_date" {
		t.Errorf("extractIndexColumns = %q, want %q", got, "sale_date")
	}
}

func TestExtractIndexColumns_MultiColumn(t *testing.T) {
	def := "CREATE INDEX idx_multi ON public.tbl USING btree (col1, col2)"
	got := extractIndexColumns(def)
	if got != "col1, col2" {
		t.Errorf("extractIndexColumns = %q, want %q", got, "col1, col2")
	}
}

func TestExtractIndexColumns_NoParens(t *testing.T) {
	def := "CREATE INDEX idx_none ON public.tbl USING btree"
	got := extractIndexColumns(def)
	if got != "" {
		t.Errorf("extractIndexColumns with no parens = %q, want \"\"", got)
	}
}

func TestExtractIndexColumns_Empty(t *testing.T) {
	got := extractIndexColumns("")
	if got != "" {
		t.Errorf("extractIndexColumns(\"\") = %q, want \"\"", got)
	}
}

func TestExtractIndexColumns_UniqueIndex(t *testing.T) {
	def := "CREATE UNIQUE INDEX idx_unique ON public.tbl USING btree (email)"
	got := extractIndexColumns(def)
	if got != "email" {
		t.Errorf("extractIndexColumns (unique) = %q, want %q", got, "email")
	}
}

// --- normalizeType ---

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		pgType string
		want   string
	}{
		{"uuid", "UUID"},
		{"varchar", "VARCHAR"},
		{"text", "VARCHAR"},
		{"bpchar", "VARCHAR"},
		{"int4", "INTEGER"},
		{"int8", "INTEGER"},
		{"int2", "INTEGER"},
		{"numeric", "DECIMAL"},
		{"float8", "DECIMAL"},
		{"float4", "DECIMAL"},
		{"bool", "BOOLEAN"},
		{"date", "DATE"},
		{"timestamptz", "TIMESTAMPTZ"},
		{"timestamp", "TIMESTAMPTZ"},
		{"jsonb", "JSONB"},
		{"json", "JSONB"},
		{"vector", "VECTOR"},
		{"unknown_type", "UNKNOWN_TYPE"}, // default: ToUpper
	}
	for _, tc := range tests {
		got := normalizeType(tc.pgType)
		if got != tc.want {
			t.Errorf("normalizeType(%q) = %q, want %q", tc.pgType, got, tc.want)
		}
	}
}

// --- cleanDefault ---

func TestCleanDefault(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'medium'::character varying", "'medium'"},
		{"0::numeric", "0"},
		{"''::text", "''"},
		{"'hello'", "'hello'"},     // no suffix to strip
		{"  spaced  ", "spaced"},   // trimmed
		{"", ""},
	}
	for _, tc := range tests {
		got := cleanDefault(tc.input)
		if got != tc.want {
			t.Errorf("cleanDefault(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- formatForPrompt ---

func TestFormatForPrompt_SimpleTable(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name:        "dim_products",
			Description: "Product catalogue",
			Columns: []columnMetadata{
				{Name: "id", DataType: "uuid", IsPK: true},
				{Name: "name", DataType: "varchar", IsNullable: false},
				{Name: "price", DataType: "numeric", IsNullable: true, Default: "0::numeric"},
			},
		},
	}

	got := sc.formatForPrompt(tables)

	if !strings.Contains(got, "## dim_products — Product catalogue") {
		t.Error("output missing table header with description")
	}
	if !strings.Contains(got, "id (UUID, PK)") {
		t.Error("output missing PK column")
	}
	if !strings.Contains(got, "name (VARCHAR)") {
		t.Error("output missing non-nullable column")
	}
	if !strings.Contains(got, "nullable") {
		t.Error("output missing nullable annotation")
	}
}

func TestFormatForPrompt_NoDescription(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name: "alerts",
			Columns: []columnMetadata{
				{Name: "id", DataType: "uuid", IsPK: true},
			},
		},
	}
	got := sc.formatForPrompt(tables)

	if !strings.Contains(got, "## alerts\n") {
		t.Errorf("output should have plain header without description; got:\n%s", got)
	}
	if strings.Contains(got, " — ") {
		t.Error("output should not have ' — ' separator when no description")
	}
}

func TestFormatForPrompt_FKColumn(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name: "fact_sales",
			Columns: []columnMetadata{
				{Name: "product_id", DataType: "uuid", FKRef: "dim_products.id"},
			},
		},
	}
	got := sc.formatForPrompt(tables)

	if !strings.Contains(got, "→ dim_products.id") {
		t.Errorf("output missing FK reference; got:\n%s", got)
	}
}

func TestFormatForPrompt_IndexesSkipPkey(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name: "dim_products",
			Columns: []columnMetadata{
				{Name: "id", DataType: "uuid", IsPK: true},
			},
			Indexes: []indexMetadata{
				{Name: "dim_products_pkey", Columns: "id", IsUnique: true},          // should be skipped
				{Name: "idx_products_name", Columns: "name", IsUnique: false},        // should appear
				{Name: "idx_products_uniq_sku", Columns: "sku", IsUnique: true},      // unique, should appear
			},
		},
	}
	got := sc.formatForPrompt(tables)

	if strings.Contains(got, "dim_products_pkey") {
		t.Error("_pkey index should be omitted from output")
	}
	if !strings.Contains(got, "idx_products_name(name)") {
		t.Error("non-unique index should appear in output")
	}
	if !strings.Contains(got, "UNIQUE(sku)") {
		t.Error("unique index should appear as UNIQUE(...)")
	}
}

func TestFormatForPrompt_MultipleTablesSeparated(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name:    "dim_products",
			Columns: []columnMetadata{{Name: "id", DataType: "uuid", IsPK: true}},
		},
		{
			Name:    "fact_sales",
			Columns: []columnMetadata{{Name: "id", DataType: "uuid", IsPK: true}},
		},
	}
	got := sc.formatForPrompt(tables)

	if !strings.Contains(got, "## dim_products") {
		t.Error("output missing first table")
	}
	if !strings.Contains(got, "## fact_sales") {
		t.Error("output missing second table")
	}
}

func TestFormatForPrompt_DefaultValueShown(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name: "action_log",
			Columns: []columnMetadata{
				{Name: "priority", DataType: "varchar", Default: "'medium'::character varying"},
			},
		},
	}
	got := sc.formatForPrompt(tables)

	// cleanDefault strips '::character varying', so we expect "default: 'medium'"
	if !strings.Contains(got, "default: 'medium'") {
		t.Errorf("expected cleaned default in output; got:\n%s", got)
	}
}

func TestFormatForPrompt_ColumnDescription(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	tables := []tableMetadata{
		{
			Name: "dim_products",
			Columns: []columnMetadata{
				{Name: "status", DataType: "varchar", Description: "active or inactive"},
			},
		},
	}
	got := sc.formatForPrompt(tables)

	if !strings.Contains(got, "— active or inactive") {
		t.Errorf("column description should appear with '— ' prefix; got:\n%s", got)
	}
}

func TestFormatForPrompt_EmptyTables(t *testing.T) {
	sc := NewSchemaCache(nil, time.Minute)
	got := sc.formatForPrompt([]tableMetadata{})
	if got != "" {
		t.Errorf("formatForPrompt([]) = %q, want \"\"", got)
	}
}
