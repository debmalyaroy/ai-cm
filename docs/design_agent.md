# AI-CM: Agentic System Design

This document details the **Cognitive Architecture** of AI-CM, treating the system as a collaborative team of autonomous agents.

## 1. Why Agentic AI? (vs. Standard GenAI)
Standard LLM implementations (e.g., a simple chatbot wrapper) suffer from hallucinations, inability to execute actions, and lack of rigorous logic. **Agentic AI** solves this by introducing:

1.  **Tool Use & Grounding:** Agents don't just "guess"; they execute SQL, browse documents, and run forecasts. If data is missing, they know they don't know.
2.  **Multi-Step Reasoning:** Complex queries ("Why did margin drop?") require a chain of actions (Get Data -> Check Competitors -> Check Inventory). Single-turn LLMs fail here; Agents persist state through this chain.
3.  **Self-Correction:** If an Agent writes bad SQL, it reads the error message and fixes it (ReAct loop). A standard LLM would just return the error to the user.
4.  **Active Execution:** Agents can *do* things (send emails, update prices) rather than just *say* things, bridging the gap between Insight and Action.

---

## 2. Core Agentic Design Patterns
We utilize specific cognitive patterns to enable autonomous behavior.

### 1.1 Patterns Used
1.  **Orchestrator-Workers (Supervisor Pattern):**
    *   **Usage:** The `SupervisorAgent` manages the session and delegates to `Analyst`, `Strategist`, etc.
    *   **Why:** Prevents "agent confusion" by centralizing state and intent classification.
2.  **ReAct (Reason + Act):**
    *   **Usage:** `AnalystAgent` (Data Retrieval).
    *   **Flow:** Thought -> Action (SQL) -> Observation (Error) -> Thought (Correction) -> Action.
    *   **Why:** Critical for robust SQL generation where first attempts often fail.
3.  **Chain-of-Thought (CoT):**
    *   **Usage:** `StrategistAgent` (Insight).
    *   **Flow:** Step-by-step reasoning ("Sales dropped -> Check Inventory -> Check Competitor -> Conclude").
    *   **Why:** Improves accuracy of "Why" explanations.
4.  **Reflection (Critic):**
    *   **Usage:** `CriticLayer` before Supervisor response.
    *   **Why:** Safety check (e.g., ensuring no PII leaks or hallucinated tables).

---

## 2. High-Level System Architecture
This diagram illustrates how the User, specific Agents, and Data layers interact.

```mermaid
graph TD
    User("Category Manager") -->|"Chat / Actions"| FE("Frontend (Next.js)")
    FE -->|"REST / SSE"| Gateway("API Gateway")
    
    subgraph "Agentic Control Plane"
        Gateway --> Supervisor("Supervisor Agent")
        
        Supervisor -->|"Delegate"| Analyst("Analyst Agent (SQL)")
        Supervisor -->|"Delegate"| Strategist("Strategist Agent (Reasoning)")
        Supervisor -->|"Delegate"| Planner("Planner Agent (Action)")
        Supervisor -->|"Delegate"| Liaison("Liaison Agent (Comm)")
        
        Analyst <--> Tools("Toolbox")
        Strategist <--> Tools
        Planner <--> Tools
    end
    
    subgraph "Data & Logic Layer"
        Tools <--> DB[("Postgres DB")]
        Tools <--> Vector[("Vector DB")]
        Tools <--> Forecast("Forecast Service")
        Tools <--> Ingest("Ingestion Service")
    end
```

---

## 3. Microservices Architecture

### 3.1 Microservices Breakdown
The backend is composed of high-cohesion, loosely coupled services.

```mermaid
graph TD
    FE["Frontend (Next.js)"]
    
    subgraph "Backend Cluster"
        Gateway["API Gateway (Go)"]
        AuthSvc["Auth Service (Go)"]
        AgentCore["Agent Core Service (Go)"]
        IngestSvc["Ingestion Service (Go)"]
        YieldSvc["Forecasting Service (Python)"]
        CommSvc["Communication Service (Go)"]
        ActionSvc["Action Service (Go)"]
    end
    
    FE <-->|"REST / SSE"| Gateway
    
    Gateway <-->|"gRPC"| AuthSvc
    Gateway <-->|"gRPC"| AgentCore
    Gateway <-->|"gRPC"| IngestSvc
    Gateway <-->|"gRPC"| CommSvc
    Gateway <-->|"gRPC"| ActionSvc
    
    AgentCore <-->|"gRPC"| IngestSvc
    AgentCore <-->|"gRPC"| YieldSvc
    AgentCore <-->|"gRPC"| CommSvc
    AgentCore <-->|"gRPC"| ActionSvc
```

### 3.2 Microservices Inventory

| Service Name | Language | Role | Inbound Protocol | Dependencies |
| :--- | :--- | :--- | :--- | :--- |
| **API Gateway** | Go (Gin) | Traffic Entry, Rate Limiting, Routing, SSE Streaming. | HTTP/REST | All Services |
| **Auth Service** | Go | Identity Provider (OIDC), Token Issuance, RBAC. | gRPC | Postgres (Users) |
| **Agent Core** | Go (LangChain) | Hosting Agent Loops (Supervisor, Analyst, etc.), Tool Execution. | gRPC | Postgres, Vector DB |
| **Ingestion Svc** | Go | Data Parsing (CSV/XLSX), Validation, Bulk Load. | gRPC / S3 Events | Postgres, S3 |
| **Forecast Svc** | Python (FastAPI) | Running ML Models (Prophet, ARIMA) for demand prediction. | gRPC | - |
| **Comm Service** | Go | Sending Emails, Notifications, and PDF Report Generation. | gRPC | SMTP, Templates |
| **Action Svc** | Go | Executing Write-backs (ERP updates), Audit Logging, Approvals. | gRPC | Postgres (Audit) |

---

## 4. Ingestion Layer Architecture
**Goal:** Ingest data and alert on anomalies.

### 4.1 Ingestion Flow & Watchdog

```mermaid
graph LR
    Source["Data Source"] -->|"Upload"| S3[("S3 Raw")]
    S3 -->|"Event"| IngestSvc["Ingestion Service"]
    
    IngestSvc -->|"Clean Data"| DB[("Postgres")]
    IngestSvc -->|"Log Metadata"| MetaStore[("Metadata DB")]
    
    Watchdog["Watchdog Agent"] -.->|"Polls"| MetaStore
    Watchdog -->|"Alert: Schema Drift"| Supervisor["Supervisor Agent"]
```

---

## 5. Agentic Memory Design
**Goal:** Context Retention & Personalization.

### 5.1 Memory Architecture Diagram

```mermaid
graph LR
    Input["User Input"] --> Encoder["Embedding Encoder"]
    Encoder --> Vector["Vector Search"]
    
    subgraph "Memory Store"
        STM[("STM: Short-Term (Redis)\nSession Chat History")]
        LTM_E[("LTM: Episodic (pgvector)\nPast Experiences")]
        LTM_S[("LTM: Semantic (pgvector)\nBusiness Knowledge")]
    end
    
    Vector --> LTM_E
    Vector --> LTM_S
    
    LTM_E -->|"Retrieve Past Plans"| ContextBuilder
    LTM_S -->|"Retrieve Business Rules"| ContextBuilder
    STM -->|"Retrieve Chat Context"| ContextBuilder
    
    ContextBuilder -->|"Enriched Prompt"| LLM
```

---

## 6. Detailed RAG Architecture (The Brain)
**Goal:** Retrieve business context (PDFs, Wikis) for Reasoning.

### 6.1 RAG Flow

```mermaid
graph TD
    Doc["Document"] --> Chunking
    Chunking --> Embedding
    Embedding --> VectorDB[("pgvector")]
    
    Query["User Query"] --> EmbedQuery
    EmbedQuery --> Search
    VectorDB -->|"Matches"| Search
    
    Search --> ReRanking["Cross-Encoder Rerank"]
    ReRanking -->|"Top-K Context"| Agent
```

---

## 7. Inter-Agent Communication (Hub-and-Spoke)
**Pattern:** We use a **Supervisor-Worker** pattern. The Supervisor prevents direct Peer-to-Peer chaos.

### 7.1 Protocol & Flow
*   **Protocol:** Structured JSON over Go Channels (if in-process) or gRPC (if distributed).

```mermaid
graph TD
    User -->|"Why did sales drop?"| Supervisor
    
    subgraph "Flow Level 1: Data Gathering"
        Supervisor -->|"1. Get Sales Data"| Analyst
        Analyst -->|"run_sql(...)"| DB[("Database")]
        Analyst -->|"Data: Sales -10%"| Supervisor
    end
    
    subgraph "Flow Level 2: Reasoning"
        Supervisor -->|"2. Explain Why"| Strategist
        Strategist -->|"Insight: Pricing Pressure"| Supervisor
    end
    
    Supervisor -->|"Final Response"| User
```

---

## 8. Detailed Agent Flows

### 8.1 Data Analyst Agent (ReAct Pattern)
```mermaid
graph TD
    Input["Input: 'Show sales in East'"] --> Thought1["Thought: Need to query sales table"]
    Thought1 --> Action1["Action: get_table_schema('sales')"]
    Action1 --> Obs1["Observation: Table 'fact_sales' exists"]
    
    Obs1 --> Thought2["Thought: Now query by region"]
    Thought2 --> Action2["Action: run_sql('SELECT * FROM fact_sales...')"]
    Action2 --> Obs2["Observation: 10 rows returned"]
    
    Obs2 --> Final["Final Answer: Here is the data..."]
```

### 8.2 Strategist Agent (Chain-of-Thought)
```mermaid
graph TD
    Data["Input Data: Sales -10%"] --> CoT1["Thinking: Drop is significant."]
    CoT1 --> CoT2["Thinking: Check Inventory levels?"]
    CoT2 --> Tool["Tool: check_inventory()"]
    Tool --> Res1["Result: Inventory is High"]
    
    Res1 --> CoT3["Thinking: High Inventory + Low Sales = Demand Issue."]
    CoT3 --> CoT4["Thinking: Check Competitor Pricing?"]
    CoT4 --> Tool2["Tool: check_competitor()"]
    Tool2 --> Res2["Result: Competitor Price -15%"]
    
    Res2 --> Conclusion["Conclusion: Pricing Pressure"]
```

### 8.3 Action Planner Agent (Human-in-the-Loop)
```mermaid
graph TD
    Insight["Input: Competitor Undercut"] --> Retrieval["Retrieve Past Successful Actions"]
    Retrieval --> Plan["Propose Plan: 1. Match Price"]
    Plan --> UI["Wait for User Approval (UI)"]
    
    UI -->|"Approve"| Exec["Execute Tool: update_price()"]
    UI -->|"Reject"| Feedback["Store Feedback"]
    
    Exec --> Log["Log Action"]
```

---

## 9. Code Repository Structure (Monorepo)
```text
ai-cm/
├── apps/
│   ├── web/                    # Next.js Frontend
│   │   ├── src/app/            # App Router
│   │   ├── src/components/     # Shadcn UI
│   │   └── package.json
│
├── backend/                    # Go Backend (Go Workspace)
│   ├── go.work                 # Go Workspace file
│   ├── cmd/                    # Entry points
│   │   ├── gateway/            # main.go for Gateway
│   │   ├── agent-core/         # main.go for Agent Core
│   │   └── ingestion/          # main.go for Ingestion
│   │
│   ├── internal/               # Private shared code
│   │   ├── common/             # Loggers, Errors
│   │   └── proto/              # Generated gRPC stubs (.pb.go)
│   │
│   ├── services/
│   │   ├── auth/               # Auth Service Logic
│   │   ├── agent/              # Agent Logic (Chains, Tools)
│   │   └── ingestion/          # Parsing Logic
│   │
│   └── pkg/                    # Public libraries (if any)
│
├── ml/                         # Python ML Services
│   ├── forecasting/
│   │   ├── app/                # FastAPI app
│   │   └── models/             # Pickle files
│   └── requirements.txt
│
├── infra/                      # Terraform / Docker Compose
│   ├── docker-compose.yml
│   └── postgres/               # Init scripts
│
└── protos/                     # Raw Protocol Buffers
    ├── agent.proto
    └── auth.proto
```
