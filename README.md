# AI-CM: The AI Category Manager Copilot

[![CI (Master)](https://github.com/debmalyaroy/ai-cm/actions/workflows/ci.yml/badge.svg)](https://github.com/debmalyaroy/ai-cm/actions/workflows/ci.yml)
[![PR Review](https://github.com/debmalyaroy/ai-cm/actions/workflows/pr_review.yml/badge.svg)](https://github.com/debmalyaroy/ai-cm/actions/workflows/pr_review.yml)
![Coverage](https://img.shields.io/badge/Coverage-80%25%2B-brightgreen.svg)

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

**Prerequisites:** Docker, Docker Compose. For LLM: either a local NVIDIA GPU (for Ollama) or an AWS Bedrock IAM credential.

```bash
# 1. Configure
# Copy .env.example to .env and fill in your keys (AWS Bedrock or Local Postgres)
cp .env.example .env

# 2. Run Local Deployment (Linux/Mac)
# To use Local LLM (Ollama)
./scripts/run.sh -p local_llm
# To use AWS Bedrock
./scripts/run.sh -p bedrock

# 2. Run Local Deployment (Windows PowerShell)
.\scripts\run.ps1 -Profile local_llm # or bedrock

# 3. Stop running services gracefully
./scripts/shutdown.sh # Linux/Mac
.\scripts\shutdown.ps1 # Windows
```

**What starts:**
| Service | URL | Description |
|---------|-----|-------------|
| Frontend | http://localhost:3000 | Vite/React Dashboard + Chat |
| Backend | http://localhost:8080 | Go API (Gin) |
| PostgreSQL | localhost:5432 | pgvector DB (~157K+ rows seeded) |
| Ollama (if local_llm) | localhost:11434 | Local LLM Engine |

**Run backend unit tests:**
```bash
# Linux/Mac
cd src/backend && go test ./internal/... -count=1

# Windows
.scriptsuild.ps1 backend
```

**Run E2E tests (local only):**
Ensure the `local_llm` stack is running, then:
```bash
# Windows
.\scripts\test_e2e.ps1

# Linux/Mac
./scripts/test_e2e.sh
```

> ⚠️ **E2E tests are NOT run in CI** because they require a live PostgreSQL instance and a local Ollama LLM server. Run them manually via the `test_e2e` scripts.

---

**Build and push production Docker images to DockerHub:**
```bash
# Linux/Mac
./scripts/build.sh all -t prod

# Windows PowerShell
.\scripts\build.ps1 all -Target prod
```

Requires `DOCKER_USERNAME` and `DOCKER_PAT` set in the `[prod.aws]` section of your root `.env`. See [HOW_TO_RUN_LOCALLY.md](docs/HOW_TO_RUN_LOCALLY.md) for full details.

**View container logs:**
```bash
# Tail logs for a specific service
docker logs -f aicm-backend
docker logs -f aicm-frontend

# All services (local_llm profile)
docker compose -f infra/docker-compose.local-llm.yml logs -f
```

---

## ⚙️ Configuration Definitions

The application uses centralized config files in `config/config.local.llm.yaml` (local Ollama dev), `config/config.local.bedrock.yaml` (local Bedrock dev), and `config/config.prod.yaml` (production). Secrets and runtime overrides reside in the root `.env` file, organized by profile sections: `[local.local]`, `[local.aws]`, and `[prod.aws]`.

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
*   **🤖 Analyst Agent (ReAct):** Converts text to SQL, queries the database, and self-corrects errors (up to 3 retries).
*   **🧠 Strategist Agent (CoT):** Uses Chain-of-Thought reasoning + RAG (Business Context) to explain *why* metrics changed.
*   **⚡ Planner Agent (Human-in-Loop):** Proposes Actions (Price Match, Restock) and manages User Approvals.
*   **📧 Liaison Agent:** Handles communication (Email/Reports) with Sellers.
*   **🔔 Watchdog Agent:** Continuous monitoring for price anomalies, stockout risks, and sales drops. Persists alerts to DB.
*   **📋 Recommender:** Heuristic rule engine that auto-generates action recommendations from live inventory & pricing data.

### 2. Backend Layer
Built on a high-performance **Golang Monolith**:
*   **REST API (Gin):** SSE streaming chat, dashboard, actions, alerts, reports endpoints.
*   **GraphQL:** Alternative query interface for chat suggestions.
*   **Distributed Cron Scheduler:** DB-locked scheduler (`internal/cron`) triggers Watchdog checks on an interval and at daily scheduled times. Safe for multi-instance deployments.
*   **Graceful Shutdown:** OS signal handling (`SIGINT`/`SIGTERM`) triggers ordered shutdown of API server → scheduler → DB pool.

### 3. Data Layer
*   **Serving Zone (PostgreSQL + pgvector):** Star-schema fact/dim tables for analytics + vector store for agent memory and RAG.

---

## 🚀 Key Features
1.  **Conversational Data Analysis:** "Why did margin drop in East?" (No SQL needed).
2.  **Proactive Alerts:** Watchdog auto-detects anomalies. Alerts page shows real-time issues with acknowledge workflow. Alerts can also be created directly from chat suggestions.
3.  **Closed-Loop Actions:** AI-generates action recommendations. Category manager approves, rejects, or reverts. Add comments for audit trail. Pending actions can be edited before approval.
4.  **Draft Actions with AI:** Enter a heading + details → LLM drafts a formal proposal with title, description, type, confidence score → review and confirm to create as pending.
5.  **Action Center Views:** Switch between Grid, List (sortable table), and Details views. Sort by Latest Updated, Newest, Oldest, or Status. Every action shows created and updated timestamps.
6.  **Report Download:** CSV export of key metrics directly from the Reports page or via chat ("Download report").
7.  **Seller Communication:** Liaison agent drafts compliance emails and performance reports.
8.  **Distributed Safety:** Cron scheduler uses PostgreSQL row locking to prevent duplicate job execution across multiple backend nodes.
9.  **Chat Session Persistence:** Full conversation history is stored per session in PostgreSQL and restored when navigating session history.
10. **Responsive Chat Panel:** Resize the chat panel via drag handle; transitions are disabled during resize for smooth performance.
