# AI-CM: The AI Category Manager Copilot

[![CI](https://github.com/debmalyaroy/ai-cm/actions/workflows/ci.yml/badge.svg)](https://github.com/debmalyaroy/ai-cm/actions/workflows/ci.yml)

> **Transforming Retail Category Management from Reactive Analysis to Proactive Decision Intelligence.**

## 📋 Project Overview
**Category Managers (CMs)** are the "CEOs" of their categories, yet they spend 50-60% of their time stitching siloed data reports rather than making strategic decisions.
**AI-CM** is an **Agentic Decision Intelligence Platform** that acts as an autonomous team for the CM. It doesn't just display data; it actively investigates anomalies, explains "why" (Reasoning), and suggests "what to do" (Prescriptive Actions).

---

## 📚 Documentation Map

This repository contains the complete design and implementation roadmap for AI-CM.

| Artifact | Description |
| :--- | :--- |
| **[Product Requirements](docs/product_requirement.md)** | Detailed problem statement, user personas, use cases, and core capabilities. |
| **[System Design](docs/design.md)** | Technical architecture (Monolith), Ingestion flows, and Logical Lakehouse strategy. |
| **[Agentic Architecture](docs/design_agent.md)** | **The Cognitive Core.** Detailed design of the Multi-Agent System (Supervisor, Analyst, Strategist, Planner). |
| **[AWS Deployment Guide](docs/aws_deployment_guide.md)** | Full guide for deploying to AWS EC2 using Free Tier and Amazon Bedrock optimization. |
| **[Low Level Design](docs/low_level_design.md)** | Modular Monolith boundaries, Data Flows, Sequence Diagrams, and Memory RAG pipeline. |

---

## 🚀 Quick Start (E2E Wrapper)

**Prerequisites:** Docker, Docker Compose, and a Gemini or OpenAI API key.

```bash
# 1. Configure
cp config/.env.local config/.env.local.active
# Edit config/.env.local.active — set your LLM_PROVIDER and API Keys inside

# 2. Run Local Deployment (Linux/Mac)
./scripts/deploy_e2e.sh local

# 2. Run Local Deployment (Windows PowerShell)
.\scripts\deploy_e2e.ps1 -EnvTarget local

# 3. Stop running services gracefully
./scripts/shutdown.sh # Linux/Mac
.\scripts\shutdown.ps1 # Windows
```

**What starts:**
| Service | URL | Description |
|---------|-----|-------------|
| Frontend | http://localhost:3000 | Next.js Dashboard + Chat |
| Backend | http://localhost:8080 | Go API (Gin) |
| PostgreSQL | localhost:5432 | pgvector DB (~50K rows seeded) |

**Run tests:**
```bash
./scripts/run_e2e.sh # or .\scripts\run_e2e.ps1
```

---

## ⚙️ Configuration Definitions

The application utilizes centralized configurations stored in `config/config.local.yaml` (for local dev) and `config/config.prod.yaml` (for deployed production). Secrets and contextual overrides reside in `.env.local` / `.env.prod`.

### Core Configuration Parameters
- **`server.port`**: API binding port (default `8080`).
- **`database.url`**: Primary pgxpool connection string. Auto-injected via docker-compose.
- **`llm.provider`**: Target LLM router (`aws`, `gemini`, `openai`, `local`). Supports **Agent-Specific Routing** via `llm.agent_models` dict (e.g., routing Analyst queries to Claude 3.5 Sonnet and Strategist queries to Claude 3 Haiku to save costs).
- **`security.rate_limit_enabled`**: Backend IP-based rate limiting (Defaults to `true` at `30` requests per minute to prevent LLM credit abuse).
- **`cors.allow_origins`**: Explicit frontend origin allowlist to lock down the proxy API.

---

## 🧪 Prompt Testing Guidelines

Since AI-CM utilizes a Multi-Agent architecture relying on specialized prompts, **prompt regression testing** is crucial.
1. **Prompts Live Externally:** All core instructional prompts are stored in `src/prompts/` rather than hardcoded in Go binaries.
2. **Context Injection Isolation:** Test prompts by rendering their `.tmpl` structures strictly against isolated schema strings to verify injection works properly before testing the LLM.
3. **Prompt Validation:** If you change `analyst.prompt.tmpl`, verify that the system can still process the baseline tests: `cd src/backend && go test ./internal/llm/prompts/...`.
4. **Iterative Refinement:** For prompt updates affecting logic, run the specific E2E regression check to guarantee the output conforms to expected `tool_call` JSON constraints.

---

## 🏗️ Architecture Highlights

### 1. Multi-Agent System (Cognitive Layer)
We utilize a **Hub-and-Spoke** agent pattern where a `Supervisor Agent` orchestrates specialized workers:
*   **🤖 Analyst Agent (ReAct):** Converts text to SQL, queries the database, and self-corrects errors.
*   **🧠 Strategist Agent (CoT):** Uses Chain-of-Thought reasoning + RAG (Business Context) to explain *why* metrics changed.
*   **⚡ Planner Agent (Human-in-Loop):** Proposes Actions (Price Match, Restock) and manages User Approvals.
*   **📧 Liaison Agent:** Handles communication (Email/Reports) with Sellers.

### 2. Microservices (Backend Layer)
Built on a high-performance **Golang Monorepo**:
*   **Gateway:** Traffic entry and SSE Streaming.
*   **Auth Service:** OIDC/JWT Identity provider.
*   **Agent Core:** Hosts the LangChainGo loops.
*   **Action Service:** Manages ERP write-backs and Audit Logs.

### 3. Data Logical Lakehouse
*   **Raw Zone (MinIO/S3):** Immutable unstructured data.
*   **Serving Zone (PostgreSQL):** High-performance structured data for Dashboards & SQL Agents.
*   **Vector Store (pgvector):** Semantic memory for RAG and Long-term Agent memory.

---

## 🚀 Key Features
1.  **Conversational Data Analysis:** "Why did margin drop in East?" (No SQL needed).
2.  **Proactive Alerts:** "Stockout predicted in 3 days. Reorder now?"
3.  **Closed-Loop Execution:** Click "Approve" to update prices in the ERP.
4.  **Seller Communication:** Automated compliance emails and feedback reports.
