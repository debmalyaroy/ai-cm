# AI-CM: Implementation Plan

## Phase 1: MVP - Foundation & "Read" Capabilities (Weeks 1-2)
**Goal:** Establish the core platform where a CM can view pre-seeded data, use the Chatbot (Text-to-SQL + Insights), and see basic dashboards.

### 1.1 Infrastructure & Scaffolding
*   [ ] Initialize Git Repo & Go Module (`github.com/org/ai-cm`).
*   [ ] Setup Docker Compose:
    *   PostgreSQL (Serving Zone).
    *   MinIO (S3 - Raw Zone simulation).
    *   pgvector extension.
*   [ ] Scaffold Next.js Frontend (App Router, Tailwind, Shadcn/UI).

### 1.2 Data Layer (Serving Zone)
*   [ ] Define SQL Schema (`db/migrations`):
    *   `dim_products`, `dim_sellers`, `dim_locations`.
    *   `fact_sales` (Time-series).
    *   `users` & `roles`.
*   [ ] Seed Mock Data: Generate realistic retail data (Diapers, Cradles) for testing.

### 1.3 Backend - Core Modules (Go)
*   [ ] **API Gateway:** Setup Gin/Echo router.
*   [ ] **Text-to-SQL Engine (Basic):**
    *   Implement Prompt Engineering module.
    *   Connect to OpenAI/Gemini API.
    *   Build "Safe SQL Executor" (Read-only connection).
*   [ ] **Insight Module:**
    *   Simple rule-based analysis (e.g., "Margin < 10%").
    *   LLM summarization of SQL results.

### 1.4 Frontend - Core UI
*   [ ] **Dashboard:** KPI Cards (GMV, Margin).
*   [ ] **Chat Interface:** Component to send messages and render Markdown/Table responses.

---

## Phase 2: User Ingestion & Operational "Write" (Weeks 3-4)
**Goal:** Allow CMs to upload their own data and Action Execution.

### 2.1 Unified Ingestion Module
*   [ ] **S3 Uploader:** API to accept CSV/Excel and write to MinIO (Raw Zone).
*   [ ] **Ingestion Worker:**
    *   Go routine to listen for S3 events.
    *   Parse CSV -> Validate -> Bulk Insert into Postgres.
*   [ ] **CDC Connector (Mock):** Simulate a stream of "Competitor Price Changes".

### 2.2 Action Engine
*   [ ] **Recommendation Logic:** Basic heuristics (e.g., If Competitor Price Drop > 5% -> Recommend "Price Match").
*   [ ] **Execution API:** Endpoint to `APPLY` an action (writes to `action_log`).
*   [ ] **Frontend Action Feed:** "Review & Approve" UI.

---

## Phase 3: Advanced Intelligence & Feedback (Weeks 5-6)
**Goal:** Predictive analytics and learning loops.

### 3.1 Forecasting Service
*   [ ] **Python Service:** Simple Prophet/ARIMA model service.
*   [ ] **Go Integration:** gRPC client to fetch forecasts and store in `fact_forecasts`.

### 3.2 Feedback Loop
*   [ ] **Feedback API:** API for "Thumbs Up/Down" on chat responses.
*   [ ] **Refinement Strategy:** Use feedback examples in the LLM System Prompt (Few-shot learning).

### 3.3 Advanced Chat
*   [ ] **Context Retention:** Implement Chat History/Session storage.
*   [ ] **Insight Refinement:** Logic to handle follow-up questions ("Drill down into South region").
