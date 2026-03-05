# AI Category Manager: Low Level Design (LLD)

## 1. High-Level Architecture & Component Boundaries

The AI Category Manager (AI-CM) is structured as a modular Monolith utilizing a highly specialized Agentic architecture interconnected with a Logical Data Lakehouse.

### 1.1 Architectural Overview

```mermaid
graph TD
    %% Boundaries
    subgraph Frontend [Next.js Web Application]
        UI_Dash[Dashboard & KPI Metrics]
        UI_Chat[Chat Interface & SSE Streaming]
        UI_Actions[Action Center & Approval Workflow]
        UI_Alerts[Alerts & Notifications Page]
        UI_Reports[Reports & Download Page]
    end

    subgraph Backend [Go Micro-Monolith API]
        R_Dash[Dashboard Handlers]
        R_Chat[Chat Handlers - SSE]
        R_Graph[GraphQL Handlers]
        R_Action[Actions Handlers]
        R_Alert[Alert Handlers]
        R_Report[Report Handlers - CSV Download]

        Cron[Distributed Cron Scheduler]

        %% Core Business Logic
        subgraph Cognitive_Layer [Multi-Agent System]
            A_Supervisor((Supervisor Agent))
            A_Analyst((Analyst Agent))
            A_Strategist((Strategist Agent))
            A_Planner((Planner Agent))
            A_Liaison((Liaison Agent))
            A_Watchdog((Watchdog Agent))
            A_Recommender((Recommender))
        end
    end

    subgraph Infrastructure [Data & LLM Layer]
        DB_Meta[(PostgreSQL - Meta & Memory)]
        DB_Warehouse[(PostgreSQL - Fact/Dim Sales)]
        LLM[Amazon Bedrock / Local Ollama / OpenAI]
    end

    %% Flow
    Frontend -->|HTTP REST / GraphQL| Backend
    R_Dash --> DB_Warehouse
    R_Chat --> A_Supervisor
    R_Graph --> A_Supervisor
    R_Report -->|Query & Stream CSV| DB_Warehouse

    A_Supervisor -->|Delegates| A_Analyst
    A_Supervisor -->|Delegates| A_Strategist
    A_Supervisor -->|Delegates| A_Planner

    A_Analyst -->|Text-to-SQL| DB_Warehouse
    A_Strategist -->|RAG Queries| DB_Meta
    A_Planner -->|Writes Actions| DB_Meta
    A_Watchdog -->|Writes Alerts| DB_Meta
    A_Recommender -->|Heuristic Queries| DB_Warehouse

    Cron -->|Distributed Lock via cron_jobs| DB_Meta
    Cron -->|Triggers| A_Watchdog

    Cognitive_Layer <--> LLM
```

### 1.2 Component Definitions

| Component | Responsibility | Tech Stack |
| :--- | :--- | :--- |
| **Next.js Frontend** | Manages UI state, renders Server Components for fast loading, handles SSE parsing. Chat panel is resizable via drag handle; transitions are suppressed during resize via `data-resizing` attribute. CSS animations use `transform: none` (not `translateY(0)`) as final keyframe to avoid creating CSS stacking contexts that would clip `position: fixed` modals. | React, TypeScript, Tailwind |
| **Go Handlers** | Serves REST endpoints, validates authentication API keys, executes Rate Limits. | Golang, Gin, pgx |
| **Supervisor Agent** | Classifies incoming chat intent and routes to specialized worker agents. | LangChain-patterns (Go) |
| **Analyst Agent** | Converts natural language definitions into structured Postgres SQL metrics. | ReAct pattern (Go) |
| **Strategist Agent** | Generates reasoning, explanations, and strategic context on data anomalies. | Chain-of-Thought (Go) |
| **Planner Agent** | Emits atomic "Action" objects requiring human approval. | Go Struct Parsing |
| **Watchdog Agent** | Runs threshold-based anomaly detection; persists alerts to DB for display. | Rule-based + Go |
| **Recommender** | Heuristic rules engine that generates action recommendations from inventory/pricing data. | Pure Go, SQL |
| **Distributed Cron** | Node-safe distributed scheduler using DB-level locking for multi-instance safety. | `internal/cron`, pgxpool |
| **Vector Store** | Persists chat history, background context, and system metadata. | PostgreSQL `pgvector` |
| **Data Warehouse** | The Star-Schema database feeding real-time metric aggregates. | PostgreSQL |


## 2. Database Schema Design (Key Tables)
The system relies on PostgreSQL for analytical data, agent memory, and operational logs.

- **`agent_memory` / `business_context`**: Stores pgvector embeddings for RAG (Retrieval-Augmented Generation) memory recall.
- **`action_log`**: Records actions suggested by the Planner, Recommender, or manually created (`id, title, description, action_type, category, confidence_score, status, created_at, updated_at`).
- **`action_comments`**: Stores user comments attached to specific actions, enabling audit trail and collaborative review (`id, action_id, user_name, content, created_at`).
- **`alerts`**: Stores real-time anomalies discovered by the Watchdog agent (`id, title, severity, category, message, acknowledged, created_at`). Severity values: `critical`, `warning`, `info`.
- **`cron_jobs`**: Distributed scheduler lock table. Each scheduled job has a row with `id, locked_by, locked_at, last_run, next_run, status`. Prevents duplicate execution across multiple backend instances.
- **`chat_sessions` / `chat_messages`**: Stores conversation history with JSONB metadata column (used to persist follow-up suggestions).
- **Fact / Dim Tables**: `fact_sales`, `fact_inventory`, `fact_competitor_prices`, `dim_products`, `dim_locations` hold the core retail data.

### 2.1 Schema: cron_jobs

```sql
CREATE TABLE cron_jobs (
    id         VARCHAR(100) PRIMARY KEY,
    locked_by  VARCHAR(100),
    locked_at  TIMESTAMPTZ,
    last_run   TIMESTAMPTZ,
    next_run   TIMESTAMPTZ,
    status     VARCHAR(20) DEFAULT 'idle',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 2.2 Schema: action_comments

```sql
CREATE TABLE action_comments (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action_id  UUID REFERENCES action_log(id) ON DELETE CASCADE,
    user_name  VARCHAR(100) DEFAULT 'demo_user',
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```


## 3. Top-Level Agent Breakdown

### 3.1 Supervisor (Orchestrator)
The core controller located at `src/backend/internal/agent/supervisor.go`. 
- Relies on few-shot prompting to classify user queries into Intents (`IntentSQL`, `IntentPlan`, `IntentInsight`, `IntentChat`).
- Delegates the request and memory context to the specific worker agent.

### 3.2 Watchdog (Anomaly Detection)
- Operates independently or triggered via the Distributed Cron Scheduler (`src/backend/internal/agent/watchdog.go`).
- Queries database thresholds for four anomaly types:
  - **Price Drops** (`price_diff_pct < -10%` against competitor prices)
  - **Stockout Risks** (`days_of_supply < 7 AND qty < reorder_level`)
  - **Sales Anomalies** (week-over-week revenue drop `> 20%`)
  - **Excess Inventory** (`days_of_supply > 90 AND qty > 300`)
- Persists detected anomalies into the `alerts` table using severity `critical`, `warning`, or `info`.
- Supports two execution modes: **interval checks** (every 5 minutes) and **time-based daily alerts** (at 08:00 AM) via the scheduler.

### 3.3 Analyst (Data Retrieval & Text-to-SQL)
- High-reasoning agent utilizing the smartest configured model (e.g., `gpt-4o` or `claude-3-5-sonnet`).
- Executes a **ReAct loop (max 3 retries)**. If a generated SQL query fails, the database error and the exact failing SQL query are passed *back* into the LLM context to self-correct hallucinated schemas or syntax errors.
- Strict read-only output enforcement.

### 3.4 Planner (Action Engine)
- Receives complex strategies and breaks them down into discrete execution steps.
- Outputs JSON conforming to the `action_log` schema, which is parsed and saved to the DB as actionable recommendations.

### 3.5 Recommender (Heuristic Rule Engine)
- Operates without LLM inference; queries the DB directly using business rules.
- Generates up to 5 actions per rule type per invocation (configurable via SQL `LIMIT`):
  - **Price Match**: competitor `price_diff_pct < -5%` in last 7 days
  - **Restock**: `quantity_on_hand < reorder_level AND days_of_supply < 10`
  - **Promotion**: `days_of_supply > 60 AND quantity_on_hand > 200`
- Deduplication: uses `WHERE NOT EXISTS (SELECT 1 FROM action_log WHERE title = $title AND status = 'pending')` to avoid duplicate pending items.

### 3.6 Distributed Cron Scheduler
- Located at `src/backend/internal/cron/scheduler.go`.
- Uses PostgreSQL `cron_jobs` table as a distributed lock mechanism:
  1. On each tick, the scheduler tries to acquire a lock for the job (`UPDATE ... WHERE status = 'idle' OR locked_at < NOW() - INTERVAL '10 minutes'`).
  2. If the lock is acquired, the job handler executes with the job's context.
  3. On completion, the lock is released and `last_run` / `next_run` timestamps are updated.
- Supports two job types:
  - **IntervalJob**: runs every N minutes/seconds (e.g., Watchdog anomaly check every 5 min)
  - **DailyJob**: runs at a specific hour:minute UTC (e.g., Watchdog daily summary at 08:00)
- Gracefully shut down via `scheduler.Stop()` during OS signal handling.


## 4. New Feature Sequence Diagrams

### 4.0 Distributed Cron Scheduler Flow

```mermaid
sequenceDiagram
    participant Main as main.go
    participant Sched as CronScheduler
    participant DB as cron_jobs (Postgres)
    participant Watchdog as WatchdogAgent

    Main->>Sched: Register(IntervalJob "watchdog-anomaly", 5m)
    Main->>Sched: Register(DailyJob "watchdog-daily", 08:00)
    Main->>Sched: Start(ctx)

    loop Every tick
        Sched->>DB: UPDATE cron_jobs SET locked_by=nodeID WHERE id=job AND status='idle'
        DB-->>Sched: RowsAffected=1 (lock acquired)
        Sched->>Watchdog: Process(ctx, "interval-check")
        Watchdog->>DB: Query price/stock/sales anomalies
        Watchdog->>DB: INSERT INTO alerts (anomalies)
        Watchdog-->>Sched: Done
        Sched->>DB: UPDATE cron_jobs SET status='idle', last_run=NOW()
    end

    Main->>Sched: Stop() on SIGTERM
```

### 4.1 Action Center: View Modes, Details, Comments & Revert Flow

The Action Center supports three view modes selectable via a toggle (**⊞ Grid / ≡ List / ☰ Details**). All modes share the same data source but render differently:
- **Grid**: Responsive `repeat(auto-fill, minmax(340px, 1fr))` card layout — default view for high-level overview.
- **List**: Sortable table with columns for title, type, category, confidence, and status.
- **Details**: Same table with expanded description and timestamp columns.

Sort options: **Newest** (created_at DESC), **Oldest** (created_at ASC), **Updated** (updated_at DESC), **Status** (status grouping).

Both `created_at` and `updated_at` are surfaced in the UI. `updated_at` is shown distinctly when it differs from `created_at`.

```mermaid
sequenceDiagram
    participant UI as Actions Page
    participant API as /api/actions
    participant DB as action_log / action_comments

    UI->>API: GET /api/actions
    API->>DB: SELECT * FROM action_log ORDER BY status, created_at DESC
    DB-->>API: Action rows
    API-->>UI: JSON Action list

    UI->>UI: User selects view mode (Grid / List / Details)
    UI->>UI: User clicks Action card/row → opens modal

    alt Pending Action - Edit Description
        UI->>API: PATCH /api/actions/:id {"title","description"}
        API->>DB: UPDATE action_log SET title=$1, description=$2, updated_at=NOW() WHERE id=$3 AND status='pending'
        DB-->>API: 1 row updated
        API-->>UI: 200 OK
        UI->>UI: Update local state inline (no re-fetch)
    end

    alt Add Comment
        UI->>API: POST /api/actions/:id/comments {"comment_text":"..."}
        API->>DB: INSERT INTO action_comments (action_id, content, user_name)
        API-->>UI: {"id":"..."}
        UI->>API: GET /api/actions/:id/comments
        API->>DB: SELECT id, content, user_name, created_at FROM action_comments WHERE action_id=$1
        DB-->>API: Comment rows
        API-->>UI: JSON Comments list
    end

    alt Approve / Reject / Revert
        UI->>API: POST /api/actions/:id/approve
        API->>DB: UPDATE action_log SET status='approved', updated_at=NOW() WHERE id=$1
        API-->>UI: 200 OK
        UI->>UI: Update local state (optimistic update, sets updated_at locally)
    end
```

### 4.1a Draft Action with AI Flow

Category Managers can describe an intended action in plain language and have the LLM draft a formal proposal.

```mermaid
sequenceDiagram
    participant UI as Create Action Modal
    participant API as /api/actions/draft
    participant LLM as LLM Provider
    participant ActionsAPI as /api/actions

    UI->>UI: User enters Heading (required) + Details (optional context)
    UI->>API: POST /api/actions/draft {"input": "heading\n\nContext: details"}
    API->>LLM: Draft action proposal (title, description, action_type, category, confidence_score)
    LLM-->>API: JSON action draft
    API-->>UI: Partial Action JSON

    UI->>UI: Show preview with AI confidence badge (green ≥80%, yellow ≥60%, red <60%)
    UI->>UI: User can edit title, type, category, description

    UI->>ActionsAPI: POST /api/actions {...draft, status: 'pending'}
    ActionsAPI-->>UI: {id: "..."}
    UI->>UI: Add new action to local state, close modal
```

### 4.1b Create Alert from Chat

When a chat suggestion has `type === "action"` and matches `/create.*(an?|the)?\s*alert/i`, the frontend intercepts it and renders an inline form rather than sending the text to the LLM.

```mermaid
sequenceDiagram
    participant UI as Chat Panel
    participant AlertsAPI as /api/alerts
    participant DB as alerts table

    UI->>UI: User clicks "Create an Alert" suggestion chip
    UI->>UI: Detect type=action + regex match → show inline CreateAlertForm
    UI->>UI: User fills: title, message, severity (critical/warning/info), category

    UI->>AlertsAPI: POST /api/alerts {title, message, severity, category}
    AlertsAPI->>DB: INSERT INTO alerts (...) RETURNING id
    DB-->>AlertsAPI: New alert row
    AlertsAPI-->>UI: {id: "..."}
    UI->>UI: Append confirmation message in chat: "Alert '{title}' created..."
    UI->>UI: Hide inline form, show success
```

### 4.1c Chat Session Persistence

Chat sessions are persisted to PostgreSQL and restored when the user navigates back to the chat. Sessions are ordered by `updated_at` so recently active conversations float to the top.

```mermaid
sequenceDiagram
    participant UI as Chat Panel
    participant API as /api/chat/*
    participant DB as chat_sessions / chat_messages

    UI->>API: GET /api/chat/sessions
    API->>DB: SELECT * FROM chat_sessions ORDER BY updated_at DESC LIMIT 20
    DB-->>API: Session list
    API-->>UI: [{id, title, updated_at}, ...]

    UI->>UI: User selects session

    UI->>API: GET /api/chat/sessions/:id/messages
    API->>DB: SELECT role, content FROM chat_messages WHERE session_id=$1 ORDER BY created_at ASC LIMIT 10
    DB-->>API: Message history (ASC for correct conversation order)
    API-->>UI: [{role, content}, ...]

    UI->>API: POST /api/chat {message, session_id}
    API->>DB: INSERT INTO chat_messages (user message)
    API->>DB: UPDATE chat_sessions SET updated_at = NOW() WHERE id=$1
    Note over API: SSE stream begins (agent processing)
    API->>DB: INSERT INTO chat_messages (assistant response)
    API->>DB: UPDATE chat_sessions SET updated_at = NOW() WHERE id=$1
    API-->>UI: event: done
```

### 4.2 Report Download Flow

```mermaid
sequenceDiagram
    participant UI as Reports Page / Chat
    participant API as /api/reports/download
    participant DB as Postgres

    UI->>API: GET /api/reports/download
    API->>DB: Run aggregate queries (top products, sales by region, etc.)
    DB-->>API: Result sets
    API->>API: Stream rows as CSV
    API-->>UI: Content-Disposition: attachment; filename=report_YYYYMMDD.csv
    UI->>UI: Browser triggers file download
```

### 4.3 Supervisor Orchestration Flow

```mermaid
sequenceDiagram
    participant User
    participant ChatHandler
    participant DB as Postgres (Session)
    participant Supervisor
    participant LLM as LLM Provider
    participant Analyst
    participant Strategist

    User->>ChatHandler: "Why did my sales drop 20%?"
    ChatHandler->>DB: Save User context (Session ID)
    ChatHandler->>Supervisor: Prompt(User Query, History)
    
    Supervisor->>LLM: Intent Classification Prompt
    LLM-->>Supervisor: `STRATEGIC_ANALYSIS`
    
    alt is STRATEGIC_ANALYSIS
        Supervisor->>Strategist: Delegate(Query, History)
        Strategist->>Analyst: Request Raw Data (Sub-Delegate)
        Analyst->>LLM: Generate SQL
        LLM-->>Analyst: `SELECT ...`
        Analyst->>DB: Execute SQL
        DB-->>Analyst: Data Results
        Analyst-->>Strategist: Extracted Metrics
        Strategist->>LLM: Chain of Thought Reasoning (Why)
        LLM-->>Strategist: Formatted Markdown Explanation
        Strategist-->>Supervisor: Final Answer
    else is DATA_FETCH
        Supervisor->>Analyst: Delegate(Query, History)
        Analyst->>LLM: Generate SQL
        LLM-->>Analyst: `SELECT ...`
        Analyst->>DB: Execute SQL
        DB-->>Analyst: Data Matrix
        Analyst-->>Supervisor: Answer Wrapper
    end
    
    Supervisor-->>ChatHandler: Final Content Stream
    ChatHandler-->>User: Server-Sent Events (SSE) Stream
```

### 4.4 ReAct Pattern: Analyst Agent Workflow

The Analyst heavily utilizes the ReAct pattern to iteratively probe the database without failing outright.

```mermaid
sequenceDiagram
    participant Supervisor
    participant Analyst
    participant SchemaCache
    participant LLM
    participant DB

    Supervisor->>Analyst: "Get margin for Electronics"
    Analyst->>SchemaCache: Get Current DB Schema Dictionary
    SchemaCache-->>Analyst: [fact_sales, dim_products, dim_store]
    
    loop ReAct Loop (Max 5 tries)
        Analyst->>LLM: System: Schema. User: Query. Output JSON Tool Call
        LLM-->>Analyst: {"tool": "run_sql", "query": "SELECT margin FROM fact_sales WHERE category='Electronics'"}
        
        Analyst->>DB: Execute "SELECT margin FROM fact_sales..."
        
        alt Execution Success
            DB-->>Analyst: [{margin: 25000}]
            Analyst->>LLM: Observation: Rows Returned
            LLM-->>Analyst: Final Answer: "The margin was $25,000"
            Analyst-->>Supervisor: Final Answer
        else Execution Failure (Semantic Error)
            DB-->>Analyst: ERROR: column 'category' does not exist
            Analyst->>LLM: Observation: Error message. Re-plan.
            LLM-->>Analyst: {"tool": "run_sql", "query": "SELECT margin FROM fact_sales JOIN dim_products ..."}
        end
    end
```


## 5. REST API Reference

### 5.0 Complete Endpoint Map

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| GET | `/api/health` | Health check including DB ping |
| GET | `/api/dashboard/kpis` | Aggregate KPI metrics |
| GET | `/api/dashboard/sales-trend` | Monthly sales trend data |
| GET | `/api/dashboard/category-breakdown` | Revenue by category |
| GET | `/api/dashboard/regional-performance` | Revenue by region |
| GET | `/api/dashboard/top-products` | Top 10 products by revenue |
| POST | `/api/dashboard/explain` | LLM explanation of a dashboard card |
| POST | `/api/chat` | SSE streaming chat (multi-agent) |
| GET | `/api/chat/sessions` | List chat sessions (ordered by updated_at DESC) |
| GET | `/api/chat/sessions/:id/messages` | Get messages for a session (ordered ASC for context) |
| GET | `/api/actions` | List actions (optional `?status=` filter) |
| POST | `/api/actions` | Create manual action |
| POST | `/api/actions/generate` | Auto-generate actions via Recommender |
| POST | `/api/actions/draft` | LLM-draft action from natural language |
| **PATCH** | **`/api/actions/:id`** | **Update title/description of pending action** |
| POST | `/api/actions/:id/approve` | Approve action |
| POST | `/api/actions/:id/reject` | Reject action |
| POST | `/api/actions/:id/revert` | Revert approved/rejected action to pending |
| GET | `/api/actions/:id/comments` | Get comments for action |
| POST | `/api/actions/:id/comments` | Add comment to action |
| GET | `/api/alerts` | List all alerts |
| POST | `/api/alerts/:id/acknowledge` | Acknowledge an alert |
| **GET** | **`/api/reports/download`** | **Download CSV report (streams response)** |
| POST | `/api/graphql` | GraphQL endpoint (chat suggestions, etc.) |

## 6. API Sequence Diagrams

These diagrams map out the system boundaries for the REST layer, handling connections between the frontend, Go webserver, and underlying PostgreSQL layers.

### 6.1 Server-Sent Events (SSE) Chat Endpoint

`/api/chat` utilizes Server-Sent Events to keep the connection open while the LangChain Go agents stream tokens.

**Important:** All consumers of SSE endpoints (including the Dashboard's "Recommend Actions" feature) must use a streaming reader (not `await res.text()` which buffers the full response). The frontend uses `ReadableStream` + `TextDecoder` with an `AbortController` (45-second timeout) to parse `data:` lines incrementally.

```mermaid
sequenceDiagram
    participant Client as React Dashboard
    participant API as Go /api/chat
    participant RateLimiter
    participant Agent as Analyst/Strategist
    participant DB as Postgres Vector Store
    participant LLM as Amazon Bedrock

    Client->>API: POST /api/chat {"message":"Explain Electronics sales"}
    API->>RateLimiter: Check Token Bucket
    RateLimiter-->>API: 200 OK
    API->>DB: Fetch Chat History for session_id
    DB-->>API: List[Historic Messages]
    API->>API: Initialize Context + Gin ResponseWriter(SSE)
    
    API->>Agent: GenerateStream(Context)
    
    Note over Agent: ReAct Loop spins up internally
    
    loop Token Generation
        Agent->>LLM: Stream Inference Request
        LLM-->>Agent: Token chunk
        Agent-->>API: Intercept chunk
        API-->>Client: `data: {"text":"..."}`
    end
    
    Agent->>DB: Append human query & AI answer to Memory
    Agent-->>API: Stream Complete
    API-->>Client: `event: done`
```

### 6.2 Dashboard Aggregation Endpoints 

The dashboard UI makes several high-concurrency requests upon pageload.

```mermaid
sequenceDiagram
    participant Dashboard UI
    participant Gateway as Go /api/dashboard/*
    participant RateLimiter
    participant Auth as Optional API Key Middleware
    participant Controller as Dashboard Handlers
    participant DB as Postgres (pgxpool)

    par Parallel Data Fetch
        Dashboard UI->>Gateway: GET /kpis
        Dashboard UI->>Gateway: GET /sales-trend
        Dashboard UI->>Gateway: GET /top-products
    end
    
    Gateway->>RateLimiter: Permit requests
    Gateway->>Auth: Validate Optional Keys
    
    Gateway->>Controller: Route to specific fetcher
    
    Controller->>DB: Execute pre-compiled `sqlc` queries
    
    DB-->>Controller: JSON formatted database rows
    Controller-->>Dashboard UI: Aggregated JSON Metrics
```


## 7. Agent Data Flow & Context Pipeline

To prevent hallucination in SQL generation, AI-CM employs an in-memory `SchemaCache` overlaid with persistent `pgvector` memory embeddings.

### 7.1 Schema Caching & DDL Context

LLMs require specific schema maps to translate text to SQL accurately.

```mermaid
graph TD
    subgraph Startup Layer
        AppBuild[Go Binary Startup]
        DB_Meta[Information_Schema]
        Builder[SchemaCache Service]
        AppBuild -->|Init| Builder
        Builder -->|Queries DDL| DB_Meta
    end

    subgraph Memory Space
        Cache[(In-Memory RAM Cache)]
    end

    subgraph Execution Loop
        Agent_Prompt_Builder[Agent Prompt Builder]
        Analyst[Analyst LLM Interface]
    end

    Builder -->|Loads Clean DDL| Cache
    
    Agent_Prompt_Builder -->|Extracts relevant tables| Cache
    Agent_Prompt_Builder -->|Injects context| Analyst
```

### 7.2 Strategist Data Flow (RAG)

When users ask strategic questions, the system checks past actions and alerts using vector search.

```mermaid
graph LR
    User("User Query 'Why are sales down?'") --> Router("Supervisor Agent")
    Router -->|If requires reasoning| Strategist
    
    subgraph Data_Warehouse [Data Warehouse]
        Fact_Sales[fact_sales]
        Dim_Date[dim_date]
        Dim_Products[dim_products]
    end
    
    subgraph Vector_Data_Store [Vector Data Store]
        Memory[pgvector Chat History]
        Alerts[pgvector Recent Anomalies]
        Actions[pgvector Execution Logs]
    end
    
    Strategist -->|Sub-Queries via Analyst| Fact_Sales
    Strategist -->|RAG via Embeddings| Memory
    Strategist -->|RAG via Embeddings| Alerts
    
    Strategist -->|"Injects Matrix & RAG Docs"| PromptCompiler
    PromptCompiler --> Bedrock(LLM)
    Bedrock -->|Returns markdown| Supervisor
```

### 7.3 Table Definitions Loaded into SchemaCache

The backend specifically isolates these tables into the cache to define the semantic boundary the Analyst LLM can see:

1. **fact_sales:** Transactional metrics `margin, revenue, units, date_id, store_id, product_id`
2. **dim_products:** Taxonomies `category, brand, line, sku`
3. **dim_date:** Temporal metadata
4. **dim_stores:** Geographic metadata `region, city, manager`

This allows prompts to specifically block unauthorized access to other schema tables (like `users` or `system_logs`).


## 8. Subsystem: Deep Episodic Memory (PgVector)
Rather than blindly stuffing conversational arrays back into the LLM context limits, the platform relies on **Contextual Memory Retrieval** through Semantic Indexing.

- **Storage Hook**: Handlers fire an asynchronous storage event after an LLM successfully responds to the user.
- **Embedding Generation**: It leverages `llmClient.Embed(query + response)` to convert textual meaning into dense float vectors.
- **Retrieval Engine**: By querying `memory.GetRelevant()`, the system calculates vector offsets returning the top 3 most "historically similar" QA pairs.

## 9. Security & Rate Limiting Controls
- **API Key Auth**: Secured via custom Gin middleware leveraging `API_KEYS` env variable. Bearer Token required for programmatic API consumption.
- **Rate Limiting**: IP-based rate limiting via `x/time/rate`, restricting endpoint spam (`RATE_LIMIT_PER_MINUTE`).
- **Postgres RBAC**: The LLM queries data using a restricted logical user (`aicm`) to eliminate SQL injection threat risks for destructive operations.
