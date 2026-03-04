package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaCache caches a prompt-ready representation of the database schema.
// It queries information_schema, pg_catalog, and pg_indexes on startup,
// formats the metadata with descriptions and index info, and caches the
// output with a configurable TTL. Thread-safe via sync.RWMutex.
type SchemaCache struct {
	db        *pgxpool.Pool
	mu        sync.RWMutex
	formatted string
	loadedAt  time.Time
	ttl       time.Duration
	// allowedPrefixes defines which tables are exposed to the LLM.
	allowedPrefixes []string
}

// NewSchemaCache creates a SchemaCache with the given TTL.
// It does NOT load eagerly — the first call to GetFormattedSchema triggers
// the initial load.
func NewSchemaCache(db *pgxpool.Pool, ttl time.Duration) *SchemaCache {
	return &SchemaCache{
		db:  db,
		ttl: ttl,
		allowedPrefixes: []string{
			"dim_",
			"fact_",
			"alerts",
			"action_log",
		},
	}
}

// GetFormattedSchema returns the cached prompt-ready schema string.
// If the cache is empty or expired, it refreshes from the database.
func (sc *SchemaCache) GetFormattedSchema(ctx context.Context) (string, error) {
	sc.mu.RLock()
	if sc.formatted != "" && time.Since(sc.loadedAt) < sc.ttl {
		defer sc.mu.RUnlock()
		return sc.formatted, nil
	}
	sc.mu.RUnlock()

	return sc.Refresh(ctx)
}

// Refresh forces a re-read of the schema from the database.
func (sc *SchemaCache) Refresh(ctx context.Context) (string, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Double-check: another goroutine may have refreshed while we waited for the lock.
	if sc.formatted != "" && time.Since(sc.loadedAt) < sc.ttl {
		return sc.formatted, nil
	}

	tables, err := sc.loadMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("schema cache refresh: %w", err)
	}

	sc.formatted = sc.formatForPrompt(tables)
	sc.loadedAt = time.Now()

	slog.InfoContext(ctx, "SchemaCache: refreshed", "tables", len(tables))
	return sc.formatted, nil
}

// SetFormattedSchemaForTest allows injecting a static schema string for unit tests
// to bypass database queries.
func (sc *SchemaCache) SetFormattedSchemaForTest(schema string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.formatted = schema
	sc.loadedAt = time.Now().Add(24 * time.Hour) // ensure it doesn't expire during tests
}

// --- Internal types ---

type tableMetadata struct {
	Name        string
	Description string
	Columns     []columnMetadata
	Indexes     []indexMetadata
}

type columnMetadata struct {
	Name        string
	DataType    string
	IsNullable  bool
	Default     string
	IsPK        bool
	FKRef       string // e.g. "dim_products.id"
	Description string
}

type indexMetadata struct {
	Name     string
	Columns  string // comma-separated
	IsUnique bool
}

// --- Data loading ---

func (sc *SchemaCache) loadMetadata(ctx context.Context) ([]tableMetadata, error) {
	// 1. Load columns with descriptions
	columns, err := sc.loadColumns(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Load primary keys
	pks, err := sc.loadPrimaryKeys(ctx)
	if err != nil {
		return nil, err
	}

	// 3. Load foreign keys
	fks, err := sc.loadForeignKeys(ctx)
	if err != nil {
		return nil, err
	}

	// 4. Load indexes
	indexes, err := sc.loadIndexes(ctx)
	if err != nil {
		return nil, err
	}

	// 5. Load table descriptions
	tableDescs, err := sc.loadTableDescriptions(ctx)
	if err != nil {
		return nil, err
	}

	// 6. Assemble per-table metadata
	tableMap := make(map[string]*tableMetadata)
	tableOrder := []string{}

	for _, col := range columns {
		if !sc.isAllowed(col.TableName) {
			continue
		}
		tm, exists := tableMap[col.TableName]
		if !exists {
			tm = &tableMetadata{Name: col.TableName}
			tableMap[col.TableName] = tm
			tableOrder = append(tableOrder, col.TableName)
		}

		cm := columnMetadata{
			Name:        col.ColumnName,
			DataType:    col.DataType,
			IsNullable:  col.IsNullable == "YES",
			Default:     col.Default,
			Description: col.Description,
		}

		// Mark PK
		if pks[col.TableName] != nil && pks[col.TableName][col.ColumnName] {
			cm.IsPK = true
		}

		// Mark FK
		fkKey := col.TableName + "." + col.ColumnName
		if ref, ok := fks[fkKey]; ok {
			cm.FKRef = ref
		}

		tm.Columns = append(tm.Columns, cm)
	}

	// Attach table descriptions
	for name, tm := range tableMap {
		if desc, ok := tableDescs[name]; ok {
			tm.Description = desc
		}
	}

	// Attach indexes
	for tableName, idxList := range indexes {
		if tm, ok := tableMap[tableName]; ok {
			tm.Indexes = idxList
		}
	}

	// Sort tables: dim_ first, then fact_, then others
	sort.Slice(tableOrder, func(i, j int) bool {
		return tableOrder[i] < tableOrder[j]
	})

	result := make([]tableMetadata, 0, len(tableOrder))
	for _, name := range tableOrder {
		result = append(result, *tableMap[name])
	}
	return result, nil
}

// columnRow holds raw query results.
type columnRow struct {
	TableName   string
	ColumnName  string
	DataType    string
	IsNullable  string
	Default     string
	Description string
}

func (sc *SchemaCache) loadColumns(ctx context.Context) ([]columnRow, error) {
	query := `
		SELECT
			c.table_name,
			c.column_name,
			c.udt_name,
			c.is_nullable,
			COALESCE(c.column_default, ''),
			COALESCE(col_description(
				(quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass,
				c.ordinal_position
			), '')
		FROM information_schema.columns c
		JOIN information_schema.tables t
			ON c.table_name = t.table_name AND c.table_schema = t.table_schema
		WHERE c.table_schema = 'public'
			AND t.table_type = 'BASE TABLE'
		ORDER BY c.table_name, c.ordinal_position
	`
	rows, err := sc.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load columns: %w", err)
	}
	defer rows.Close()

	var result []columnRow
	for rows.Next() {
		var r columnRow
		if err := rows.Scan(&r.TableName, &r.ColumnName, &r.DataType, &r.IsNullable, &r.Default, &r.Description); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func (sc *SchemaCache) loadTableDescriptions(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT c.relname, COALESCE(obj_description(c.oid), '')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
	`
	rows, err := sc.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load table descriptions: %w", err)
	}
	defer rows.Close()

	descs := make(map[string]string)
	for rows.Next() {
		var name, desc string
		if err := rows.Scan(&name, &desc); err != nil {
			return nil, err
		}
		if desc != "" {
			descs[name] = desc
		}
	}
	return descs, nil
}

func (sc *SchemaCache) loadPrimaryKeys(ctx context.Context) (map[string]map[string]bool, error) {
	query := `
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = 'public'
	`
	rows, err := sc.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load primary keys: %w", err)
	}
	defer rows.Close()

	pks := make(map[string]map[string]bool)
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			return nil, err
		}
		if pks[table] == nil {
			pks[table] = make(map[string]bool)
		}
		pks[table][col] = true
	}
	return pks, nil
}

func (sc *SchemaCache) loadForeignKeys(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT
			kcu.table_name || '.' || kcu.column_name AS fk_col,
			ccu.table_name || '.' || ccu.column_name AS ref_col
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'
	`
	rows, err := sc.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load foreign keys: %w", err)
	}
	defer rows.Close()

	fks := make(map[string]string)
	for rows.Next() {
		var fkCol, refCol string
		if err := rows.Scan(&fkCol, &refCol); err != nil {
			return nil, err
		}
		fks[fkCol] = refCol
	}
	return fks, nil
}

func (sc *SchemaCache) loadIndexes(ctx context.Context) (map[string][]indexMetadata, error) {
	query := `
		SELECT
			tablename,
			indexname,
			indexdef
		FROM pg_indexes
		WHERE schemaname = 'public'
		ORDER BY tablename, indexname
	`
	rows, err := sc.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load indexes: %w", err)
	}
	defer rows.Close()

	indexes := make(map[string][]indexMetadata)
	for rows.Next() {
		var table, name, def string
		if err := rows.Scan(&table, &name, &def); err != nil {
			return nil, err
		}
		if !sc.isAllowed(table) {
			continue
		}

		isUnique := strings.Contains(strings.ToUpper(def), "UNIQUE")
		// Extract columns from the index definition: ... (col1, col2)
		cols := extractIndexColumns(def)

		indexes[table] = append(indexes[table], indexMetadata{
			Name:     name,
			Columns:  cols,
			IsUnique: isUnique,
		})
	}
	return indexes, nil
}

// extractIndexColumns parses columns from a CREATE INDEX definition.
// e.g. "CREATE INDEX idx_foo ON public.bar USING btree (col1, col2)" → "col1, col2"
func extractIndexColumns(indexDef string) string {
	start := strings.LastIndex(indexDef, "(")
	end := strings.LastIndex(indexDef, ")")
	if start >= 0 && end > start {
		return strings.TrimSpace(indexDef[start+1 : end])
	}
	return ""
}

// --- Formatting ---

func (sc *SchemaCache) formatForPrompt(tables []tableMetadata) string {
	var sb strings.Builder

	for i, t := range tables {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Table header with description
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("## %s — %s\n", t.Name, t.Description))
		} else {
			sb.WriteString(fmt.Sprintf("## %s\n", t.Name))
		}

		// Columns
		sb.WriteString("Columns:\n")
		for _, c := range t.Columns {
			sb.WriteString("  - ")
			sb.WriteString(c.Name)

			// Type
			typeName := normalizeType(c.DataType)
			if c.IsPK {
				sb.WriteString(fmt.Sprintf(" (%s, PK)", typeName))
			} else if c.FKRef != "" {
				sb.WriteString(fmt.Sprintf(" (%s → %s)", typeName, c.FKRef))
			} else {
				attrs := []string{typeName}
				if c.IsNullable {
					attrs = append(attrs, "nullable")
				}
				if c.Default != "" && !strings.Contains(c.Default, "uuid_generate") && !strings.Contains(c.Default, "now()") {
					attrs = append(attrs, fmt.Sprintf("default: %s", cleanDefault(c.Default)))
				}
				sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(attrs, ", ")))
			}

			// Description
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf(" — %s", c.Description))
			}
			sb.WriteString("\n")
		}

		// Indexes (skip auto PKs for brevity)
		var idxParts []string
		for _, idx := range t.Indexes {
			// Skip the primary key index (already shown via PK tag)
			if strings.HasSuffix(idx.Name, "_pkey") {
				continue
			}
			if idx.IsUnique {
				idxParts = append(idxParts, fmt.Sprintf("UNIQUE(%s)", idx.Columns))
			} else {
				idxParts = append(idxParts, fmt.Sprintf("%s(%s)", idx.Name, idx.Columns))
			}
		}
		if len(idxParts) > 0 {
			sb.WriteString(fmt.Sprintf("Indexes: %s\n", strings.Join(idxParts, ", ")))
		}
	}

	return sb.String()
}

// isAllowed checks if a table is in the allowed list for the LLM.
func (sc *SchemaCache) isAllowed(tableName string) bool {
	for _, prefix := range sc.allowedPrefixes {
		if strings.HasPrefix(tableName, prefix) || tableName == prefix {
			return true
		}
	}
	return false
}

// normalizeType maps PostgreSQL internal type names to readable names.
func normalizeType(pgType string) string {
	switch pgType {
	case "uuid":
		return "UUID"
	case "varchar", "text", "bpchar":
		return "VARCHAR"
	case "int4", "int8", "int2":
		return "INTEGER"
	case "numeric", "float8", "float4":
		return "DECIMAL"
	case "bool":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "timestamptz", "timestamp":
		return "TIMESTAMPTZ"
	case "jsonb", "json":
		return "JSONB"
	case "vector":
		return "VECTOR"
	default:
		return strings.ToUpper(pgType)
	}
}

// cleanDefault removes common noise from default value strings.
func cleanDefault(def string) string {
	def = strings.TrimSuffix(def, "::character varying")
	def = strings.TrimSuffix(def, "::numeric")
	def = strings.TrimSuffix(def, "::text")
	def = strings.TrimSpace(def)
	return def
}
