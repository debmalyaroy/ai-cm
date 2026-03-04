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
    end

    subgraph Backend [Go Micro-Monolith API]
        R_Dash[Dashboard Handlers]
        R_Chat[Chat Handlers - SSE]
        R_Graph[GraphQL Handlers]
        R_Action[Actions Handlers]
        
        %% Core Business Logic
        subgraph Cognitive_Layer [Multi-Agent System]
            A_Supervisor((Supervisor Agent))
            A_Analyst((Analyst Agent))
            A_Strategist((Strategist Agent))
            A_Planner((Planner Agent))
            A_Liaison((Liaison Agent))
            A_Watchdog((Watchdog Agent))
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
    
    A_Supervisor -->|Delegates| A_Analyst
    A_Supervisor -->|Delegates| A_Strategist
    A_Supervisor -->|Delegates| A_Planner
    
    A_Analyst -->|Text-to-SQL| DB_Warehouse
    A_Strategist -->|RAG Queries| DB_Meta
    A_Planner -->|Writes Actions| DB_Meta
    
    Cognitive_Layer <--> LLM
```

### 1.2 Component Definitions

| Component | Responsibility | Tech Stack |
| :--- | :--- | :--- |
| **Next.js Frontend** | Manages UI state, renders Server Components for fast loading, handles SSE parsing. | React, TypeScript, Tailwind |
| **Go Handlers** | Serves REST endpoints, validates authentication API keys, executes Rate Limits. | Golang, Gin, pgx |
| **Supervisor Agent** | Classifies incoming chat intent and routes to specialized worker agents. | LangChain-patterns (Go) |
| **Analyst Agent** | Converts natural language definitions into structured Postgres SQL metrics. | ReAct pattern (Go) |
| **Strategist Agent** | Generates reasoning, explanations, and strategic context on data anomalies. | Chain-of-Thought (Go) |
| **Planner Agent** | Emits atomic "Action" objects requiring human approval. | Go Struct Parsing |
| **Vector Store** | Persists chat history, background context, and system metadata. | PostgreSQL `pgvector` |
| **Data Warehouse** | The Star-Schema database feeding real-time metric aggregates. | PostgreSQL |


## 2. Database Schema Design (Key Tables)
The system relies on PostgreSQL for analytical data, agent memory, and operational logs.

- **`chat_memory_embeddings`**: Stores pgvector embeddings of `[user_query, assistant_response]` for RAG (Retrieval-Augmented Generation) memory recall.
- **`action_log`**: Records actions suggested by the Planner and manually executed by the user from the dashboard drill downs (`id, title, description, action_type, category, metadata, confidence_score, status, executed_at`).
- **`alerts`**: Stores real-time anomalies discovered by the Watchdog agent (`id, title, severity, category, message, acknowledged, created_at`).
- **Fact / Dim Tables**: `fact_sales`, `fact_inventory`, `dim_products`, `dim_locations` hold the core retail data accessible by the Analyst agent.


## 3. Top-Level Agent Breakdown

### 3.1 Supervisor (Orchestrator)
The core controller located at `src/backend/internal/agent/supervisor.go`. 
- Relies on few-shot prompting to classify user queries into Intents (`IntentSQL`, `IntentPlan`, `IntentInsight`, `IntentChat`).
- Delegates the request and memory context to the specific worker agent.

### 3.2 Watchdog (Anomaly Detection)
- Operates independently or triggered via system health checks (`src/backend/internal/agent/watchdog.go`).
- Queries database thresholds for: **Price Drops**, **Stockout Risks**, **Sales Anomalies**, and **Excess Inventory**.
- Persists expected anomalies into the `alerts` database table, enabling the frontend Dashboard / Alerts page to display them.

### 3.3 Analyst (Data Retrieval & Text-to-SQL)
- High-reasoning agent utilizing the smartest configured model (e.g., `gpt-4o` or `claude-3-5-sonnet`).
- Executes a **ReAct loop (max 3 retries)**. If a generated SQL query fails, the database error and the exact failing SQL query are passed *back* into the LLM context to self-correct hallucinated schemas or syntax errors.
- Strict read-only output enforcement.

### 3.4 Planner (Action Engine)
- Receives complex strategies and breaks them down into discrete execution steps.
- Outputs JSON conforming to the `action_log` schema, which is parsed and saved to the DB as actionable recommendations.


## 4. Agent Sequence Diagrams

The Cognitive Layer is driven by a master Supervisor agent routing to highly specialized Sub-Agents.

### 4.1 Supervisor Orchestration Flow

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

### 4.2 ReAct Pattern: Analyst Agent Workflow

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


## 5. API Sequence Diagrams

These diagrams map out the system boundaries for the REST layer, handling connections between the frontend, Go webserver, and underlying PostgreSQL layers.

### 5.1 Server-Sent Events (SSE) Chat Endpoint

`/api/chat` utilizes Server-Sent Events to keep the connection open while the LangChain Go agents stream tokens.

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

### 5.2 Dashboard Aggregation Endpoints 

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


## 6. Agent Data Flow & Context Pipeline

To prevent hallucination in SQL generation, AI-CM employs an in-memory `SchemaCache` overlaid with persistent `pgvector` memory embeddings.

### 6.1 Schema Caching & DDL Context

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

### 6.2 Strategist Data Flow (RAG)

When users ask strategic questions, the system checks past actions and alerts using vector search.

```mermaid
graph LR
    User(User Query "Why are sales down?") --> Router(Supervisor Agent)
    Router -->|If requires reasoning| Strategist
    
    subgraph Data Warehouse
        Fact_Sales[fact_sales]
        Dim_Date[dim_date]
        Dim_Products[dim_products]
    end
    
    subgraph Vector Data Store
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

### 6.3 Table Definitions Loaded into SchemaCache

The backend specifically isolates these tables into the cache to define the semantic boundary the Analyst LLM can see:

1. **fact_sales:** Transactional metrics `margin, revenue, units, date_id, store_id, product_id`
2. **dim_products:** Taxonomies `category, brand, line, sku`
3. **dim_date:** Temporal metadata
4. **dim_stores:** Geographic metadata `region, city, manager`

This allows prompts to specifically block unauthorized access to other schema tables (like `users` or `system_logs`).


## 7. Subsystem: Deep Episodic Memory (PgVector)
Rather than blindly stuffing conversational arrays back into the LLM context limits, the platform relies on **Contextual Memory Retrieval** through Semantic Indexing.

- **Storage Hook**: Handlers fire an asynchronous storage event after an LLM successfully responds to the user.
- **Embedding Generation**: It leverages `llmClient.Embed(query + response)` to convert textual meaning into dense float vectors.
- **Retrieval Engine**: By querying `memory.GetRelevant()`, the system calculates vector offsets returning the top 3 most "historically similar" QA pairs.

## 8. Security & Rate Limiting Controls
- **API Key Auth**: Secured via custom Gin middleware leveraging `API_KEYS` env variable. Bearer Token required for programmatic API consumption.
- **Rate Limiting**: IP-based rate limiting via `x/time/rate`, restricting endpoint spam (`RATE_LIMIT_PER_MINUTE`).
- **Postgres RBAC**: The LLM queries data using a restricted logical user (`aicm`) to eliminate SQL injection threat risks for destructive operations.
