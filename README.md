# AI-CM: The AI Category Manager Copilot

[![CI (Master)](https://github.com/debmalyaroy/ai-cm/actions/workflows/ci.yml/badge.svg)](https://github.com/debmalyaroy/ai-cm/actions/workflows/ci.yml)
[![PR Review](https://github.com/debmalyaroy/ai-cm/actions/workflows/pr_review.yml/badge.svg)](https://github.com/debmalyaroy/ai-cm/actions/workflows/pr_review.yml)
[![E2E Tests](https://github.com/debmalyaroy/ai-cm/actions/workflows/e2e.yml/badge.svg)](https://github.com/debmalyaroy/ai-cm/actions/workflows/e2e.yml)
![Coverage](https://img.shields.io/badge/Coverage-90%25%2B-brightgreen.svg)

> **Transforming Retail Category Management from Reactive Analysis to Proactive Decision Intelligence.**

## Project Overview

**Category Managers (CMs)** are the "CEOs" of their categories, yet they spend 50-60% of their time stitching siloed data reports rather than making strategic decisions.
**AI-CM** is an **Agentic Decision Intelligence Platform** that acts as an autonomous team for the CM. It doesn't just display data; it actively investigates anomalies, explains *why* (Reasoning), and suggests *what to do* (Prescriptive Actions).

---

## Documentation Map

| Artifact | Description |
| :--- | :--- |
| **[Product Requirements](requirements.md)** | Detailed problem statement, user personas, use cases, and core capabilities. |
| **[Phased Implementation](docs/phased_implementation.md)** | What is built vs. planned. Phase 1 (POC) vs Phase 2 (Production) breakdown with per-requirement status (DONE / PLANNED). |
| **[Agentic Architecture](docs/design_agent.md)** | The Cognitive Core. Detailed design of the Multi-Agent System (Supervisor, Analyst, Strategist, Planner), RAG pipeline, and repo structure. |
| **[Low Level Design](docs/low_level_design.md)** | Modular Monolith boundaries, Data Flows, Sequence Diagrams, Memory RAG pipeline, and AWS production deployment architecture (EC2 + RDS + Bedrock, multi-arch Docker build). |
| **[AWS Deployment Guide](docs/aws_deployment_guide.md)** | Deploy to AWS Free Tier (EC2 + RDS + Bedrock). Includes Windows-specific PuTTY / PSCP instructions, model selection analysis, and cost estimates. |
| **[How to Run Locally](docs/HOW_TO_RUN_LOCALLY.md)** | Run the full stack locally with Docker (Ollama or Bedrock profiles). |
| **[GitHub CI](docs/GITHUB_CI.md)** | CI pipeline details: PR Review quality gate, build & push to DockerHub. Deployment is done locally via scripts. |
| **[Prompt Testing](docs/PROMPT_TESTING.md)** | Guidelines for testing and iterating on agent prompts without running the full stack. |

---

## Quick Start

**Prerequisites:** Docker, Docker Compose. For LLM: either a local GPU (for Ollama) or an AWS Bedrock IAM credential.

```bash
# 1. Configure — copy and fill in your keys
cp .env.example .env

# 2. Run (Linux/Mac)
./scripts/run.sh -p local_llm   # local Ollama LLM
./scripts/run.sh -p bedrock     # AWS Bedrock

# 2. Run (Windows PowerShell)
.\scripts\run.ps1 -Profile local_llm
.\scripts\run.ps1 -Profile bedrock

# 3. Stop
./scripts/shutdown.sh    # Linux/Mac
.\scripts\shutdown.ps1   # Windows
```

**What starts:**

| Service | URL | Description |
|---------|-----|-------------|
| **Nginx** (entry point) | http://localhost:8181 | Reverse proxy — use this for the full app |
| &nbsp;&nbsp;→ Frontend | http://localhost:8181/ | Vite/React Dashboard + Chat |
| &nbsp;&nbsp;→ Backend API | http://localhost:8181/api/ | All `/api/` calls proxied to Go backend (SSE-aware, 310s timeout) |
| &nbsp;&nbsp;→ Project summary | http://localhost:8181/project-summary.html | Static project overview page |
| &nbsp;&nbsp;→ Health check | http://localhost:8181/health | Returns `ok` |
| Frontend (direct) | http://localhost:3000 | Vite static app served by nginx inside container |
| Backend (direct) | http://localhost:8080 | Go API — direct access, bypasses proxy |
| PostgreSQL | localhost:5432 | pgvector DB (~157K+ rows seeded) |
| Ollama (local_llm only) | localhost:11434 | Local LLM Engine (requires NVIDIA GPU + CUDA) |

---

## Testing

AI-CM has three levels of testing that progressively increase in scope and infrastructure requirements.

### 1. Unit Tests (no external services)

Backend unit tests cover all `internal/` packages. Frontend tests run with Vitest.

```bash
./scripts/test_unit.sh    # Linux/Mac
.\scripts\test_unit.ps1   # Windows
```

Coverage requirement: **90%+** on backend packages (enforced in CI on pull requests).

### 2. E2E Tests (Docker + Mock LLM)

E2E tests exercise the full agent pipeline — intent classification, SQL generation, SSE streaming, action generation — without requiring a real LLM. A lightweight mock server (`infra/llm-mock/`) listens on port 11434 and returns canned, structurally-valid responses for each agent prompt type.

```bash
./scripts/test_e2e.sh    # Linux/Mac
.\scripts\test_e2e.ps1   # Windows
```

The mock LLM classifies prompts by keyword matching:

| Agent | Keyword trigger | Mock response |
|-------|----------------|---------------|
| Supervisor (intent) | "reply with only one word" | `query` / `insight` / `plan` (keyword-driven) |
| Analyst (SQL) | "sqlforge" / "read-only" | Valid `SELECT` SQL block |
| Strategist | "chain-of-thought" / "analyse" | Business insight paragraph |
| Planner | "actionforge" / "propose actions" | `ACTION:` formatted blocks |
| Liaison | "draft" + "email"/"report" | Professional email body |

To run E2E tests against a manually started stack:
```bash
cd infra
docker compose -f docker-compose.e2e.yml up -d postgres llm-mock

cd ../src/backend
DATABASE_URL="postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable" \
LLM_PROVIDER=local OLLAMA_BASE_URL=http://localhost:11434 \
go test ./tests/... -v -count=1 -timeout 180s
```

### 3. CI Pipeline

| Workflow | Trigger | What runs |
|----------|---------|-----------|
| **PR Review** (`pr_review.yml`) | Pull Request to `master` | Go lint + unit tests (90% coverage gate) + frontend lint/test/build |
| **CI** (`ci.yml`) | Push to `master` | Unit tests → Swagger gen → lint → Docker build & push to DockerHub |
| **E2E** (`e2e.yml`) | Push to `master` | Postgres service container + mock LLM (`go run`) + `go test ./tests/...` |

> Deployment to EC2 is **not automated from CI**. Run `./scripts/deploy.sh prod` locally after CI passes.

---

## Building & Deploying

**Build and push production Docker images to DockerHub:**
```bash
./scripts/build.sh all -t prod    # Linux/Mac
.\scripts\build.ps1 all -Target prod  # Windows
```

Requires `DOCKER_USERNAME` and `DOCKER_PAT` set in the `[prod.aws]` section of your root `.env`. See [HOW_TO_RUN_LOCALLY.md](docs/HOW_TO_RUN_LOCALLY.md) for full details.

**Deploy to EC2 (after images are pushed):**
```bash
./scripts/deploy.sh prod    # Linux/Mac
.\scripts\deploy.ps1 prod   # Windows
```

**View container logs:**
```bash
docker logs -f aicm-backend
docker logs -f aicm-frontend
```

---

## Configuration

Config files live in `config/`:

| File | Used for |
|------|----------|
| `config/config.local.llm.yaml` | Local dev with Ollama |
| `config/config.local.bedrock.yaml` | Local dev with AWS Bedrock |
| `config/config.prod.yaml` | Production (EC2) |

Secrets and runtime overrides live in the root `.env`, organized by profile sections: `[local.local]`, `[local.aws]`, and `[prod.aws]`.

### Key Parameters
- **`server.port`**: API binding port (default `8080`).
- **`database.url`**: Primary pgxpool connection string. Auto-injected via docker-compose.
- **`llm.provider`**: `aws` for Amazon Bedrock, `local` for Ollama. Production uses `aws` with Meta Llama 3.1 70B Instruct (`us.meta.llama3-1-70b-instruct-v1:0`) at ~$0.72/MTok via cross-region inference to us-east-1.
- **`security.rate_limit_enabled`**: IP-based rate limiting (default `true`, 30 req/min) to prevent LLM credit abuse.
- **`cors.allow_origins`**: Frontend origin allowlist for the proxy API.

---

## Key Features

| Feature | Description |
| :--- | :--- |
| **Conversational Data Analysis** | Ask "Why did margin drop in East?" in plain English — no SQL needed. Smart ILIKE matching for product and brand name queries. |
| **Proactive Alerts** | Watchdog agent auto-detects price anomalies, stockout risks, and sales drops. Alerts page shows real-time issues with an acknowledge workflow. Alerts can also be created directly from chat suggestions. |
| **Closed-Loop Actions** | AI-generates action recommendations with priority (high/medium/low) and expected impact estimates. Category manager approves, rejects, or reverts. Comments provide an audit trail. Pending actions can be edited before approval. |
| **AI Action Drafting** | Enter a heading + details → LLM drafts a formal proposal with title, description, type, and confidence score → review and confirm to create as pending. |
| **Action Center Views** | Switch between Grid, List (sortable table), and Details views. Sort by Latest Updated, Newest, Oldest, or Status. Priority badges and expected impact shown in all views. |
| **Report Download** | CSV export of key metrics from the Reports page or via chat ("Download report"). |
| **Seller Communication** | Liaison agent drafts compliance emails and performance reports for sellers. |
| **Distributed Safety** | Cron scheduler uses PostgreSQL row locking to prevent duplicate job execution across multiple backend nodes. |
| **Chat Session Management** | Sessions created on panel open; last 10 active sessions in history with restore and delete. Offers to resume the previous conversation on next visit. Session end stores an episodic memory for future RAG retrieval. |
| **Responsive Chat Panel** | Resize via drag handle; dock left, right, or bottom. Transitions disabled during resize for smooth performance. |
| **Response Transparency** | Every AI response shows a colour-coded confidence score (green ≥85%, yellow ≥70%, red <70%) and data source label. Failed queries show a Retry button. |
| **Inline Data Charts** | When a data query returns tabular results (≥2 rows with a category + numeric column), an inline bar chart is rendered automatically in the chat. |
| **Clarifying Questions** | For vague queries ("sales", "show me inventory"), the AI asks for specifics — time period, category, region, metric — before attempting SQL generation. |
| **Page-Aware Quick Actions** | Quick-start prompts on the welcome screen adapt to the current page (Dashboard / Actions / Alerts / Reports). |
| **Persistent User Configuration** | All settings — AI thresholds, watchdog alert levels, notification preferences, UI prefs — are saved per-user in PostgreSQL via the `/config` page and restored across sessions. |

---

## Architecture Highlights

### 1. Multi-Agent System (Cognitive Layer)

Hub-and-Spoke pattern where a `Supervisor Agent` orchestrates specialized workers:

| Agent | Pattern | Responsibility |
| :--- | :--- | :--- |
| **Analyst** | ReAct | Text-to-SQL with self-correction (up to 3 retries). Two-tier SQL cache: in-process (L1) + pgvector semantic cache (L2). |
| **Strategist** | Chain-of-Thought + RAG | Explains *why* metrics changed using business context from the vector store. |
| **Planner** | Human-in-the-Loop | Proposes actions (Price Match, Restock) and manages approval workflow. |
| **Liaison** | | Drafts compliance emails and performance reports for sellers. |
| **Watchdog** | Scheduled | Continuous monitoring for price anomalies, stockout risks, and sales drops. Persists alerts to DB. |
| **Recommender** | Heuristic | Rule engine that auto-generates action recommendations from live inventory and pricing data. |

### 2. Backend Layer

Go Monolith on Gin:
- **REST API:** SSE streaming chat, dashboard, actions, alerts, reports, config endpoints.
- **GraphQL:** Alternative query interface for chat suggestions.
- **Distributed Cron Scheduler:** DB-locked scheduler (`internal/cron`) triggers Watchdog checks safely across multi-instance deployments.
- **Graceful Shutdown:** OS signal handling (`SIGINT`/`SIGTERM`) triggers ordered shutdown of API server → scheduler → DB pool.

### 3. Data Layer

PostgreSQL + pgvector: star-schema fact/dim tables for analytics + vector store for agent memory and RAG (business context, episodic memory, SQL cache).
