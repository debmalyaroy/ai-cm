# AI-CM: Phased Implementation Plan

This document describes what has been built in **Phase 1 (POC)** and what is deferred to **Phase 2 (Production)**. It maps every requirement from `requirements.md` to its current implementation status.

---

## Phase Overview

### Phase 1 — Proof of Concept (Current)

**Goal:** Validate the multi-agent architecture end-to-end on real data, deployed to AWS, usable by a single Category Manager.

**Scope:**
- Modular Go monolith (single binary): API server + all 6 agents + cron scheduler
- Static seed data (157K+ sales rows, 200 products, 6000 inventory, 1530 competitor prices)
- PostgreSQL + pgvector as the sole data store (no S3/MinIO ingestion pipeline)
- Amazon Bedrock (Meta Llama 3.1 70B) for LLM inference in production; Ollama locally
- Multi-tier caching: SchemaCache (30m), L1 SQLCache (15m), L2 VectorSQLCache (24h), ResultCache (5m/2m)
- 3-tier agent memory: STM (chat history), LTM Episodic (pgvector Q/A pairs), LTM Semantic (business_context)
- RAG pipeline: Bedrock Titan embeddings → cosine similarity search → top-3 context injection
- Deployed on AWS EC2 t4g.small + RDS db.t4g.micro (ARM64, both within free tier)
- Multi-arch Docker images (linux/amd64 + linux/arm64) built by CI and pushed to DockerHub
- IP-based rate limiting (20 req/min prod), Critic layer PII masking, read-only SQL enforcement
- Actions: create, approve, reject, revert, comment, edit — with full audit trail
- Alerts: Watchdog auto-detects 4 anomaly types; alert acknowledgement workflow
- Liaison: email/report drafting (LLM output only — no actual email delivery)
- Reports: CSV download, per-agent SSE streaming

**Not in Phase 1:**
- File/API/stream-based data ingestion pipeline
- Demand forecasting models (Prophet/ARIMA)
- Closed-loop action execution via external APIs (price changes applied in actual systems)
- Real email delivery (SMTP/SES)
- OIDC/JWT authentication (API key auth only)
- Horizontal auto-scaling

---

### Phase 2 — Production Hardening

**Goal:** Turn the validated POC into a production-grade, multi-user platform with real integrations.

**Scope:**
- File/API/CDC ingestion pipeline (S3/MinIO Raw Zone → PostgreSQL Serving Zone)
- Demand forecasting service (Python, Prophet/ARIMA models)
- Closed-loop execution: Planner calls real pricing/inventory APIs on approval
- Real email delivery via AWS SES (Liaison agent)
- OIDC/JWT authentication + RBAC
- Horizontal scaling: ECS Fargate or EKS (currently single EC2)
- CloudWatch alarms + auto-restart on degradation
- 30-day conversation history TTL + data retention policies
- Page-aware chat suggestions
- Customizable quick actions per user

---

## Requirements Status Table

### Requirement 1: Natural Language Query Processing

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 1.1 | Parse query and identify intent | 1 | **DONE** | Supervisor classifies intent via few-shot LLM prompt into `IntentSQL`, `IntentPlan`, `IntentInsight`, `IntentChat` |
| 1.2 | Route to appropriate Analytics Module | 1 | **DONE** | Supervisor delegates to Analyst, Strategist, Planner, or Liaison based on intent |
| 1.3 | Ask clarifying questions for ambiguous queries | 2 | PLANNED | Supervisor currently returns a best-effort response; explicit clarification dialogue loop not implemented |
| 1.4 | Provide helpful suggestions when query cannot be understood | 1 | **DONE** | Fallback intent (`IntentChat`) returns a helpful LLM-generated response with suggestions |
| 1.5 | Support sales, pricing, inventory, competitor, product queries | 1 | **DONE** | Text-to-SQL covers all fact/dim tables; schema-grounded to prevent hallucinated table access |

---

### Requirement 2: Contextual Conversation Management

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 2.1 | Initialize empty Conversation Context on session start | 1 | **DONE** | New `chat_sessions` row created on first message; `chat_messages` table stores history |
| 2.2 | Add query and response to Conversation Context | 1 | **DONE** | Both user and assistant messages inserted into `chat_messages` after each turn |
| 2.3 | Resolve follow-up references using Conversation Context | 1 | **DONE** | Last 10 messages injected into agent prompt as conversation history |
| 2.4 | Update context with current page context | 2 | PLANNED | Chat panel has no awareness of which dashboard page the user is viewing |
| 2.5 | Reset Conversation Context on session clear | 1 | **DONE** | New session starts a fresh `chat_sessions` row; history sidebar allows switching sessions |

---

### Requirement 3: Multi-Source Data Integration

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 3.1 | Retrieve dashboard metrics | 1 | **DONE** | Analyst generates SQL over `fact_sales`, `dim_products`, `dim_locations` |
| 3.2 | Retrieve pricing information | 1 | **DONE** | `fact_competitor_prices` queried via Text-to-SQL |
| 3.3 | Retrieve inventory data | 1 | **DONE** | `fact_inventory` queried via Text-to-SQL |
| 3.4 | Retrieve competitor information | 1 | **DONE** | Competitor price table included in schema context |
| 3.5 | Retrieve product analysis | 1 | **DONE** | `dim_products` joins supported by schema cache |
| 3.6 | Aggregate from multiple modules into unified response | 1 | **DONE** | Strategist sub-delegates to Analyst for data, then synthesises using CoT reasoning |

---

### Requirement 4: Proactive Insights and Recommendations

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 4.1 | Display relevant insights when copilot is opened | 1 | **DONE** | Chat suggestion chips generated on session load (LLM + fallbacks) |
| 4.2 | Generate and display insight when significant trends detected | 1 | **DONE** | Watchdog agent runs on cron (5 min interval + 08:00 daily), inserts alerts for price/stock/sales anomalies |
| 4.3 | Include data source and confidence level with insight | 1 | **PARTIAL** | Confidence score included on Action suggestions; Watchdog alerts include severity but no explicit confidence % |
| 4.4 | Provide supporting data when more details requested | 1 | **DONE** | Follow-up queries re-engage Analyst + Strategist with full context |
| 4.5 | Limit proactive insights to max 3 per session | 2 | PLANNED | Suggestion chip count is configurable but no per-session limit enforced |

---

### Requirement 5: Actionable Recommendations

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 5.1 | Generate relevant Action Suggestions after analysis | 1 | **DONE** | Planner generates JSON actions saved as `pending` in `action_log`; Recommender generates heuristic actions |
| 5.2 | Include expected impact and priority with suggestion | 1 | **DONE** | `confidence_score` and `action_type` stored; LLM-drafted actions include description of expected impact |
| 5.3 | Provide step-by-step guidance when suggestion selected | 1 | **PARTIAL** | LLM narrates the rationale; structured step-by-step playbook not generated |
| 5.4 | Provide direct link to relevant page for navigation | 2 | PLANNED | Chat response is text/markdown only; deep-link navigation not implemented |
| 5.5 | Track which suggestions have been acted upon in session | 2 | PLANNED | Actions are persisted with status; in-session tracking of clicked suggestions not implemented |

---

### Requirement 6: Response Formatting and Visualization

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 6.1 | Format numbers with units and precision | 1 | **DONE** | LLM formats numeric output; prompts instruct markdown table formatting |
| 6.2 | Structured format (bullet points, tables) for comparative data | 1 | **DONE** | Strategist and Analyst prompts instruct markdown; frontend renders markdown |
| 6.3 | Directional indicators (up/down arrows, percentages) | 1 | **DONE** | LLM output includes percentage changes; frontend renders markdown symbols |
| 6.4 | Inline charts/graphs for trend responses | 2 | PLANNED | Chat panel renders markdown only; no chart embedding in responses |
| 6.5 | Progressive disclosure with expandable sections for lengthy responses | 2 | PLANNED | Full response rendered at once; collapsible sections not implemented |

---

### Requirement 7: Error Handling and Graceful Degradation

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 7.1 | Inform user when Analytics Module is unavailable | 1 | **DONE** | DB errors surface as user-friendly messages via SSE; LLM unavailability returns graceful error |
| 7.2 | Provide partial response on timeout; offer to retry | 1 | **DONE** | SSE streams tokens as they arrive; 240s write timeout with `context.WithoutCancel` prevents mid-stream disconnect |
| 7.3 | Indicate missing data; provide available information | 1 | **DONE** | Analyst self-corrects SQL errors (up to 3 retries); reports partial results if retries exhausted |
| 7.4 | Log errors; display user-friendly message | 1 | **DONE** | Structured JSON logging to CloudWatch; error SSE event sent to frontend |
| 7.5 | Maintain Conversation Context even when errors occur | 1 | **DONE** | Session and message records committed before agent processing; error does not roll back history |

---

### Requirement 8: Quick Actions and Shortcuts

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 8.1 | Display suggested quick action buttons on open | 1 | **DONE** | Suggestion chips rendered on session load; generated via LLM or fallback list |
| 8.2 | Execute corresponding query immediately on click | 1 | **DONE** | Chip click submits pre-filled message to the chat handler |
| 8.3 | Provide quick actions for top/under performers, compare to last period, recommendations | 1 | **DONE** | Fallback suggestions include "top products", "margin drop", "stockout risk", "download report" |
| 8.4 | Prioritise quick actions relevant to current page | 2 | PLANNED | Suggestions are session-global; page context not passed to the suggestion generator |
| 8.5 | Allow customisation of preferred quick actions | 2 | PLANNED | No user preferences or pinned actions implemented |

---

### Requirement 9: Session Persistence and History

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 9.1 | Save Conversation Context at session end | 1 | **DONE** | All messages persisted to PostgreSQL (`chat_sessions`, `chat_messages`); more durable than local storage |
| 9.2 | Offer to restore previous session on return | 1 | **DONE** | Session history sidebar lists past sessions ordered by `updated_at DESC` |
| 9.3 | Display list of past sessions with timestamps | 1 | **DONE** | `GET /api/chat/sessions` returns sessions with `updated_at` timestamp |
| 9.4 | Restore full Conversation Context from past session | 1 | **DONE** | `GET /api/chat/sessions/:id/messages` loads messages ASC for correct replay |
| 9.5 | Retain history for maximum 30 days | 2 | PLANNED | No TTL or expiry policy implemented; records persist indefinitely |

---

### Requirement 10: Response Time and Performance

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 10.1 | Simple query responds within 2 seconds | 1 | **PARTIAL** | Simple chat intents resolve quickly; Text-to-SQL with Llama 3.1 70B typically takes 10–30s |
| 10.2 | Complex multi-source query responds within 5 seconds | 1 | **PARTIAL** | Complex queries (Analyst + Strategist + RAG) can take 30–75s on Bedrock; SSE streaming makes latency tolerable |
| 10.3 | Display loading indicator when processing > 2 seconds | 1 | **DONE** | SSE stream shows typing indicator from first token; frontend shows spinner until first chunk arrives |
| 10.4 | Show which Analytics Module is being queried | 1 | **PARTIAL** | SSE events include partial text; no explicit "Querying Analyst..." status event |
| 10.5 | Cache frequently requested data | 1 | **DONE** | 4-tier caching: SchemaCache (30m TTL), L1 SQLCache (bag-of-words, 15m), L2 VectorSQLCache (cosine ≥0.92, 24h), ResultCache (5m Strategist / 2m Watchdog) |

---

### Requirement 11: Accessibility and Usability

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 11.1 | Keyboard navigation for all interactive elements | 1 | **PARTIAL** | Basic keyboard focus supported; full tab-order audit not performed |
| 11.2 | Submit queries using Enter key | 1 | **DONE** | `onKeyDown` handler submits on Enter (Shift+Enter for newline) |
| 11.3 | Clear visual feedback for all interactions | 1 | **DONE** | Button states, loading spinner, SSE streaming tokens, fadeIn animations |
| 11.4 | Sufficient color contrast | 1 | **DONE** | Tailwind design system with accessible color palette |
| 11.5 | ARIA labels and announcements for screen readers | 2 | PLANNED | No ARIA audit performed; labels not systematically added |

---

### Requirement 12: Integration with Existing UI

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 12.1 | Accessible from all pages | 1 | **DONE** | Chat panel is a persistent side dock rendered at the app layout level |
| 12.2 | Appears as side panel without obscuring content | 1 | **DONE** | Resizable left dock; drag handle allows width adjustment; transitions disabled during resize |
| 12.3 | Preserve Chat Session state when panel closed | 1 | **DONE** | Session state in PostgreSQL; panel visibility toggle does not clear session |
| 12.4 | Use application's existing design system | 1 | **DONE** | Tailwind CSS + shared component classes (`components.css`) used throughout |
| 12.5 | Responsive layout on browser resize | 1 | **DONE** | `--chat-panel-width` CSS variable driven by drag handle; layout adapts at all widths |

---

### Requirement 13: Data Ingestion and Processing

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 13.1 | Support ingestion from Files (CSV/XLSX), APIs, Streams, CDC | 2 | PLANNED | Phase 1 uses static SQL seed scripts (`infra/postgres/*.sql`). File/API/stream ingestion pipeline is Phase 2 |
| 13.2 | Watchdog validates data quality and detects schema drift on ingestion | 2 | PLANNED | Watchdog monitors live DB thresholds (price/stock/sales rules), not ingestion events |
| 13.3 | Alert Supervisor when anomalies detected | 1 | **DONE** | Watchdog persists alerts to `alerts` table; displayed on Alerts page with acknowledge workflow |
| 13.4 | Store raw data in Raw Zone (S3/MinIO); processed in Serving Zone (PostgreSQL) | 2 | PLANNED | Only PostgreSQL Serving Zone exists; no Raw Zone |
| 13.5 | Log ingestion errors and continue processing other sources | 2 | PLANNED | Not applicable in Phase 1 (no ingestion pipeline) |

---

### Requirement 14: Predictive Analytics and Forecasting

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 14.1 | Demand forecasting using time-series models (Prophet/ARIMA) | 2 | PLANNED | No forecasting service; not in Phase 1 scope |
| 14.2 | Auto-retrain models when forecast accuracy drops below 80% | 2 | PLANNED | No forecasting service |
| 14.3 | Predict stockouts 3–7 days in advance with confidence scores | 2 | PLANNED | Watchdog detects current stockouts (`days_of_supply < 7`); forward-looking prediction not implemented |
| 14.4 | Strategist explains significant forecast deviations | 2 | PLANNED | Strategist can explain historical drops; no forecast data to reason over |
| 14.5 | Scenario analysis ("What if price drops 5%?") | 2 | PLANNED | No scenario modelling capability |

---

### Requirement 15: Automated Action Execution

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 15.1 | Execute approved action through appropriate API integration | 2 | PLANNED | Approval persists `status='approved'` in `action_log`; no outbound API call to pricing/inventory systems |
| 15.2 | Support price updates, inventory adjustments, promotional changes | 2 | PLANNED | Action types defined (`price_match`, `restock`, `promotion`) but execution is not automated |
| 15.3 | Log action with timestamp and user approval | 1 | **DONE** | `action_log.updated_at` stamped on every state change; comments provide audit trail |
| 15.4 | Rollback executed actions within 24 hours | 1 | **PARTIAL** | "Revert" transitions status back to `pending`; no time-limited rollback window enforced; no external system rollback |
| 15.5 | Notify user when execution fails; suggest manual alternatives | 2 | PLANNED | No external execution means no execution failure path |

---

### Requirement 16: Communication and Reporting

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 16.1 | Liaison generates and **sends** compliance alerts to sellers via email | 2 | PLANNED | Liaison drafts email content via LLM but has no SMTP/SES integration; output is text only |
| 16.2 | Create performance feedback reports | 1 | **DONE** | Liaison drafts structured reports; CSV download available from Reports page and via chat |
| 16.3 | Send automated notifications when seller performance drops below threshold | 2 | PLANNED | Watchdog detects anomalies and creates alerts; no email notification delivery |
| 16.4 | Track communication history and seller response rates | 2 | PLANNED | No seller communication records or response tracking |
| 16.5 | Generate executive summary reports | 1 | **DONE** | Liaison can draft executive summaries; CSV export covers KPI metrics |

---

### Requirement 17: Multi-Agent Coordination

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 17.1 | Supervisor orchestrates all agents via Hub-and-Spoke pattern | 1 | **DONE** | `supervisor.go` classifies intent and delegates to workers; all results pass back through Supervisor |
| 17.2 | Coordinate multiple agents in correct sequence when needed | 1 | **DONE** | Strategic analysis: Supervisor → Strategist → (sub-delegates) → Analyst → Strategist CoT → Supervisor |
| 17.3 | Prevent agent conflicts via centralised state management | 1 | **DONE** | Single Supervisor instance per request; shared `context.Context` carries cancellation; no shared mutable state between agents |
| 17.4 | Handle agent failures gracefully; continue with available agents | 1 | **DONE** | Each agent returns typed error; Supervisor catches and returns user-friendly fallback without crashing |
| 17.5 | Monitor agent performance metrics; auto-restart failed agents | 2 | PLANNED | Structured logs to CloudWatch; no automatic agent health check or restart mechanism |

---

### Requirement 18: Business Context and Memory

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 18.1 | Long-term memory via vector storage (pgvector) | 1 | **DONE** | `agent_memory` table with `vector(1536)` embeddings; Bedrock Titan for production embeddings |
| 18.2 | Strategist uses RAG to retrieve relevant business context | 1 | **DONE** | `BuildContext()` computes embedding, queries episodic memory and `business_context` facts in parallel goroutines |
| 18.3 | Remember past successful actions; suggest similar approaches | 1 | **PARTIAL** | Episodic memory stores Q/A pairs (retrieved by similarity); action outcome linking to future suggestions not implemented |
| 18.4 | Update knowledge base when business rules change | 1 | **PARTIAL** | `business_context` table is manually updatable via SQL; no admin UI or automated update pipeline |
| 18.5 | Explanations reference specific business policies and historical decisions | 1 | **DONE** | RAG injects top-3 relevant `business_context` facts into Strategist prompt; LLM cites them in reasoning |

---

### Requirement 19: Security and Compliance

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 19.1 | OIDC/JWT authentication with role-based access control | 2 | PLANNED | Optional API key auth (`API_KEYS` env var); no OIDC/JWT or per-user RBAC |
| 19.2 | Analyst executes read-only SQL only | 1 | **DONE** | Critic layer validates generated SQL; only `SELECT` statements are executed; `run_sql` tool enforces read-only |
| 19.3 | Audit all actions and maintain compliance logs | 1 | **DONE** | `action_log` + `action_comments` provide full audit trail; structured JSON logs to CloudWatch |
| 19.4 | Encrypt data in transit and at rest | 1 | **DONE** | RDS KMS encryption at rest; `sslmode=require` for all DB connections; HTTPS enforced at nginx |
| 19.5 | Mask/redact sensitive data detected in queries | 1 | **DONE** | Critic layer checks LLM output for PII patterns before returning to user |

---

### Requirement 20: Performance and Scalability

| AC | Acceptance Criterion | Phase | Status | Implementation Notes |
|---|---|---|---|---|
| 20.1 | Handle concurrent users with response times under 5 seconds | 1 | **PARTIAL** | Rate limiting (20 req/min) protects Bedrock spend; `pgxpool` with 20 max connections; LLM latency exceeds 5s for complex queries |
| 20.2 | Caching strategies to reduce database load | 1 | **DONE** | SchemaCache (30m), L1 in-process SQLCache (100 entries, 15m), L2 VectorSQLCache (pgvector, 24h), ResultCache (5m Strategist, 2m Watchdog) |
| 20.3 | Prioritise critical queries over background processing | 2 | PLANNED | Cron scheduler runs background Watchdog jobs; no priority queue for user requests vs background |
| 20.4 | Scale horizontally by adding service instances | 1 | **PARTIAL** | Distributed cron uses DB-level row locking (`cron_jobs` table) making multi-instance deployment safe; single EC2 node currently |
| 20.5 | Monitor performance metrics; alert administrators on degradation | 1 | **PARTIAL** | Docker logs stream to CloudWatch `/aicm/docker`; no CloudWatch alarms or automated degradation alerts |

---

## Summary

| | Phase 1 (POC) | Phase 2 (Production) |
|---|---|---|
| **Requirements fully DONE** | 1, 2, 3, 7, 9, 12, 17, 18 (core) | 13, 14, 15 (execution), 16 (delivery) |
| **Requirements PARTIAL** | 4, 5, 6, 8, 10, 11, 15, 16, 18, 19, 20 | → Completed |
| **Requirements PLANNED** | — | 13 (full), 14, 15.1–15.2, 19.1 |

### Phase 2 Priorities (by business impact)

| Priority | Feature | Requirement(s) |
|---|---|---|
| P1 | OIDC/JWT authentication + RBAC | 19.1 |
| P1 | File/API/CDC ingestion pipeline (S3/MinIO) | 13.1, 13.2, 13.4, 13.5 |
| P1 | Real email delivery via AWS SES | 16.1, 16.3 |
| P2 | Demand forecasting service (Python, Prophet/ARIMA) | 14.1–14.5 |
| P2 | Closed-loop action execution (external API calls on approval) | 15.1, 15.2, 15.5 |
| P2 | Page-aware chat suggestions | 2.4, 8.4 |
| P3 | Inline chart visualisation in chat | 6.4, 6.5 |
| P3 | 30-day session history TTL + data retention | 9.5 |
| P3 | CloudWatch alarms + auto-restart on degradation | 17.5, 20.3, 20.5 |
| P3 | Horizontal scaling (ECS Fargate / EKS) | 20.4 |
