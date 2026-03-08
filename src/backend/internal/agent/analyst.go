package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/memory"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalystAgent implements the ReAct pattern for Text-to-SQL.
type AnalystAgent struct {
	llmClient   llm.Client
	tools       *ToolSet
	db          *pgxpool.Pool
	sqlCache    *SQLCache       // L1: in-process exact-match cache (bag-of-words key, fast)
	schemaCache *SchemaCache
	mem         memory.Manager  // L2: pgvector semantic SQL cache + shared agent memory
}

// NewAnalystAgent creates a new Analyst agent.
func NewAnalystAgent(llmClient llm.Client, db *pgxpool.Pool, tools *ToolSet, mem memory.Manager) *AnalystAgent {
	return &AnalystAgent{
		llmClient:   llmClient,
		tools:       tools,
		db:          db,
		sqlCache:    NewSQLCache(15*time.Minute, 100),
		schemaCache: NewSchemaCache(db, 30*time.Minute),
		mem:         mem,
	}
}

func (a *AnalystAgent) Name() string { return "analyst" }

func (a *AnalystAgent) Process(ctx context.Context, input *Input) (*Output, error) {
	output := &Output{
		AgentName: a.Name(),
		Reasoning: []ReasoningStep{},
	}

	// Step 1: Get database schemas
	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type:    "thought",
		Content: "I need to understand the database schema to write an accurate SQL query.",
	})

	slog.DebugContext(ctx, "Analyst: starting Text-to-SQL logic block")

	// Step 1: Get cached schema metadata
	schemaText, err := a.schemaCache.GetFormattedSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	output.Reasoning = append(output.Reasoning, ReasoningStep{
		Type:    "action",
		Content: "Retrieved database schema",
	})

	slog.DebugContext(ctx, "Analyst: retrieved cached DB schema")

	// Step 2a: L2 — Vector SQL cache (semantic similarity via pgvector)
	// Catches "Show revenue by region" when "Display sales per region" was previously answered.
	if a.mem != nil {
		if cachedSQL, found, err := a.mem.RetrieveSQL(ctx, input.Query, 0.92); found {
			slog.DebugContext(ctx, "Analyst: vector SQL cache hit", "query", input.Query)
			output.Reasoning = append(output.Reasoning, ReasoningStep{
				Type:    "action",
				Content: "Found semantically similar cached SQL from vector store",
			})
			sqlTool, _ := a.tools.Get("run_sql")
			queryResult, execErr := sqlTool.Execute(ctx, map[string]any{"sql": cachedSQL})
			if execErr == nil {
				output.Reasoning = append(output.Reasoning, ReasoningStep{
					Type:    "observation",
					Content: "Vector-cached SQL executed successfully",
				})
				output.ConfidenceScore = 0.90
				output.DataSource = "database (cached)"
				return a.summarizeResults(ctx, input, output, cachedSQL, queryResult)
			}
			slog.WarnContext(ctx, "Analyst: vector-cached SQL failed, falling through", "error", execErr)
		} else if err != nil {
			slog.WarnContext(ctx, "Analyst: vector SQL cache lookup failed (non-fatal)", "error", err)
		}
	}

	// Step 2b: L1 — In-process exact-match cache (bag-of-words key, instant)
	if cachedSQL, ok := a.sqlCache.Get(input.Query); ok {
		slog.DebugContext(ctx, "Analyst: in-process SQL cache hit", "query", input.Query)
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Found cached SQL template for this query pattern",
		})

		// Execute the cached SQL directly
		sqlTool, _ := a.tools.Get("run_sql")
		queryResult, err := sqlTool.Execute(ctx, map[string]any{"sql": cachedSQL})
		if err == nil {
			output.Reasoning = append(output.Reasoning, ReasoningStep{
				Type:    "observation",
				Content: "Cached SQL executed successfully",
			})
			output.ConfidenceScore = 0.90
			output.DataSource = "database (cached)"
			// Jump to summarization with cachedSQL and queryResult
			return a.summarizeResults(ctx, input, output, cachedSQL, queryResult)
		}
		slog.WarnContext(ctx, "Analyst: cached SQL failed, falling through to LLM generation", "error", err)
	}

	// Step 3: Generate SQL using LLM (ReAct loop with retries)
	var sqlQuery string
	var queryResult any
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {

		promptTemplate := prompts.Get("analyst_sql.md")
		systemPrompt := fmt.Sprintf(promptTemplate, schemaText)

		var userPrompt string
		if attempt == 0 {
			userPrompt = input.Query
		} else {
			userPrompt = fmt.Sprintf("Previous SQL query: %s\nFailed with error: %v\n\nOriginal question: %s\n\nPlease fix the SQL query.", sqlQuery, err, input.Query)
		}
		userPrompt += formatMemoryContext(input.MemoryContext)

		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "thought",
			Content: fmt.Sprintf("Generating SQL query (attempt %d/%d)", attempt+1, maxRetries),
		})

		slog.DebugContext(ctx, "Analyst: generating SQL", "attempt", attempt+1, "query", input.Query)

		sqlQuery, err = a.llmClient.Generate(ctx, systemPrompt, userPrompt)
		if err != nil {
			slog.ErrorContext(ctx, "LLM generate error", "attempt", attempt+1, "error", err)
			continue
		}

		// Clean up the SQL query (remove markdown code blocks if present)
		sqlQuery = cleanSQL(ctx, sqlQuery)

		if strings.TrimSpace(sqlQuery) == "OUT_OF_PURVIEW" {
			output.Response = "I am a data analyst assistant specialized in retail analytics. I cannot answer questions outside of my purview or the provided database schema."
			output.Reasoning = append(output.Reasoning, ReasoningStep{
				Type:    "action",
				Content: "Identified question as out-of-purview. Aborting SQL generation.",
			})
			return output, nil
		}

		// If the LLM returned prose instead of SQL, retry with a stronger nudge.
		// Only give up on the very last attempt.
		if !looksLikeSQL(sqlQuery) {
			if attempt == maxRetries-1 {
				slog.WarnContext(ctx, "Analyst: final attempt still non-SQL, returning error")
				output.Response = "I was unable to generate a valid SQL query for your request. Please try rephrasing your question with more specific details."
				output.Reasoning = append(output.Reasoning, ReasoningStep{
					Type:    "observation",
					Content: "All attempts returned prose instead of SQL. Returning graceful error.",
				})
				return output, nil
			}
			slog.WarnContext(ctx, "Analyst: LLM returned non-SQL response, will retry", "attempt", attempt+1)
			output.Reasoning = append(output.Reasoning, ReasoningStep{
				Type:    "observation",
				Content: fmt.Sprintf("LLM returned prose instead of SQL (attempt %d). Retrying...", attempt+1),
			})
			err = fmt.Errorf("LLM did not produce SQL output")
			continue
		}

		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: fmt.Sprintf("Generated SQL: %s", sqlQuery),
		})

		// Step 3: Execute the SQL
		slog.DebugContext(ctx, "Analyst: executing SQL", "sql", sqlQuery)
		sqlTool, _ := a.tools.Get("run_sql")
		queryResult, err = sqlTool.Execute(ctx, map[string]any{"sql": sqlQuery})
		if err != nil {
			output.Reasoning = append(output.Reasoning, ReasoningStep{
				Type:    "observation",
				Content: fmt.Sprintf("SQL Error: %v. Will retry...", err),
			})
			slog.ErrorContext(ctx, "SQL execution error", "attempt", attempt+1, "error", err)
			continue
		}

		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "observation",
			Content: "SQL executed successfully",
		})
		if resultMap, ok := queryResult.(map[string]any); ok {
			if rows, ok2 := resultMap["rows"].([]map[string]any); ok2 {
				slog.DebugContext(ctx, "Analyst: SQL returned rows", "row_count", len(rows))
			}
		}
		break
	}

	if err != nil {
		return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, err)
	}

	// Set confidence based on fresh LLM-generated SQL
	output.ConfidenceScore = 0.85
	output.DataSource = "database"

	// Cache the successful SQL in both L1 (in-process) and L2 (vector store)
	a.sqlCache.Put(input.Query, sqlQuery)
	if a.mem != nil {
		go func(q, sql string) {
			if err := a.mem.StoreSQL(context.Background(), q, sql); err != nil {
				slog.Warn("Analyst: failed to store SQL in vector cache", "error", err)
			}
		}(input.Query, sqlQuery)
	}

	return a.summarizeResults(ctx, input, output, sqlQuery, queryResult)
}

// summarizeResults handles Step 4 of the analyst pipeline: summarizing SQL results
// into a user-friendly response. Shared between cache-hit and fresh-generation paths.
func (a *AnalystAgent) summarizeResults(ctx context.Context, input *Input, output *Output, sqlQuery string, queryResult any) (*Output, error) {
	// Graceful fallback for empty responses.
	// RunSQLTool returns map[string]any{columns, rows, row_count}.
	if resultMap, ok := queryResult.(map[string]any); ok {
		if rows, ok2 := resultMap["rows"].([]map[string]any); ok2 && len(rows) == 0 {
			output.Response = "I queried the database but could not find any data matching your criteria."
			output.Data = queryResult
			return output, nil
		}
	}

	resultJSON, _ := json.MarshalIndent(queryResult, "", "  ")
	output.Data = queryResult

	summaryPrompt := prompts.Get("analyst_summary.md")

	userSummaryPrompt := fmt.Sprintf("User Question: %s\n\nSQL Query: %s\n\nResults:\n%s",
		input.Query, sqlQuery, string(resultJSON))
	userSummaryPrompt += formatMemoryContext(input.MemoryContext)

	summary, err := a.llmClient.Generate(ctx, summaryPrompt, userSummaryPrompt)
	if err != nil {
		// Fallback to raw data if summarization fails
		output.Response = fmt.Sprintf("Here are the results:\n\n```json\n%s\n```", string(resultJSON))
	} else {
		output.Response = summary
	}

	return output, nil
}

// cleanSQL removes markdown code blocks and extra whitespace from LLM-generated SQL.
// It extracts blocks explicitly tagged ```sql, untagged ```, or any SQL statement
// (SELECT, WITH, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP) embedded in prose.
func cleanSQL(ctx context.Context, sql string) string {
	slog.DebugContext(ctx, "Raw LLM output before cleaning", "raw_output", sql)
	sql = strings.TrimSpace(sql)

	// First priority: explicit ```sql ... ``` block
	reSQL := regexp.MustCompile("(?is)```sql\\r?\\n(.*?)\\r?\\n?```")
	if m := reSQL.FindStringSubmatch(sql); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Second priority: untagged ``` ... ``` block (backticks followed immediately by a newline)
	reUntagged := regexp.MustCompile("(?is)```\\r?\\n(.*?)\\r?\\n?```")
	if m := reUntagged.FindStringSubmatch(sql); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Third priority: extract any SQL statement embedded in prose text.
	// Matches SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, WITH
	// even when surrounded by conversational filler.
	// Order matters: SELECT first (most common, least ambiguous), WITH last
	upperSQL := strings.ToUpper(sql)
	sqlKeywords := []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "CREATE ", "ALTER ", "DROP ", "WITH "}
	for _, kw := range sqlKeywords {
		if startIdx := strings.Index(upperSQL, kw); startIdx >= 0 {
			extracted := sql[startIdx:]
			// Trim trailing prose after the last semicolon
			if endIdx := strings.LastIndex(extracted, ";"); endIdx >= 0 {
				extracted = extracted[:endIdx+1]
			}
			result := strings.TrimSpace(extracted)
			// Validate: the extracted text must itself look like SQL
			if result != "" && looksLikeSQL(result) {
				slog.DebugContext(ctx, "cleanSQL: extracted SQL from prose", "keyword", strings.TrimSpace(kw))
				return result
			}
		}
	}

	return strings.TrimSpace(sql)
}

// looksLikeSQL returns true if the string starts with a SQL keyword (ignoring comments).
func looksLikeSQL(s string) bool {
	lines := strings.Split(s, "\n")
	var firstCodeLine string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		// Very basic multi-line comment ignore for the start of the string
		if strings.HasPrefix(trimmed, "/*") {
			continue
		}
		firstCodeLine = trimmed
		break
	}

	upper := strings.ToUpper(firstCodeLine)
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return false
	}
	firstWord := fields[0]

	for _, kw := range []string{"SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER"} {
		if firstWord == kw {
			if kw == "WITH" {
				// Require strict CTE pattern to avoid conversational "With the data as..."
				match, _ := regexp.MatchString(`(?i)^WITH\s+[a-zA-Z_0-9]+\s+AS`, strings.TrimSpace(firstCodeLine))
				if !match {
					return false
				}
			}
			return true
		}
	}
	return false
}
