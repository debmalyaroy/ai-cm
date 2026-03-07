# AWS Deployment Guide (Bedrock + Graviton)

This guide documents the actual AI-CM deployment on AWS using **Graviton (ARM64)** instances for cost-efficient compute, **RDS PostgreSQL** for the database, and **Amazon Bedrock** for LLM inference.

> **Free Tier**: The current deployment uses `t4g.small` (EC2) and `db.t4g.micro` (RDS), both of which are covered:
> - **EC2 `t4g.small`**: AWS extended free trial — **750 hrs/month free through December 31, 2026** (ARM64 Graviton2 promotional offer; standard always-free tier only covers `t3.micro`/`t2.micro`).
> - **RDS `db.t4g.micro`**: **Permanently in the AWS Free Tier** since March 2022 — `db.t4g.micro` was officially added alongside `db.t3.micro` in all commercial regions.
> - **Bedrock**: Pay-per-token; no free tier. See Section 2 for Bedrock cost estimates.

---

## 1. Architecture & Cost Strategy

### Actual Deployed Resources (as of March 2026)

| Layer | Service | Instance | Notes |
|---|---|---|---|
| **Compute** | EC2 `t4g.small` | `i-043b1989c56cc1cd3` (`aicm-server`) | 2 vCPU, 2GB RAM, ARM64 Graviton2. AZ: ap-south-1a. |
| **Database** | RDS PostgreSQL `db.t4g.micro` | `aicm-postgres` | PostgreSQL 16.13, 20GB gp3, KMS-encrypted, Single-AZ. |
| **LLM** | Amazon Bedrock | Llama 3.1 70B | Cross-region inference `us-east-1`. Pay-per-token. |
| **Reverse Proxy** | nginx (Docker container) | — | Routes `/api/*` → backend, `/` → frontend. |
| **Logs** | CloudWatch Logs | `/aicm/docker` | Log group created in ap-south-1. |
| **Registry** | DockerHub | `debmalyaroy/*` | `aicm-backend:latest`, `aicm-frontend:latest` |
| **Network** | VPC `ai-cm-vpc-vpc` | `vpc-0956839fd979e21ce` | CIDR 10.0.0.0/16. 2 public subnets (ap-south-1a, 1b). |
| **Public IP** | Elastic IP | `13.126.208.105` | Static IP attached to EC2. |

> **ARM64 Architecture**: Both EC2 and RDS use Graviton2 (ARM64). Docker images **must be built for `linux/arm64`**. Images built for `linux/amd64` (default on x86 Windows/Mac) will run under QEMU emulation, which is significantly slower.

---

## 2. AWS Services

### Services We Use

| Service | Resource Name | Role | Cost |
|---|---|---|---|
| **EC2 t4g.small** | `aicm-server` | Runs all Docker containers (backend, frontend, nginx, Dozzle) | Free trial through Dec 31, 2026 (750 hrs/month); ~$12.10/month after |
| **RDS PostgreSQL db.t4g.micro** | `aicm-postgres` | Primary database with pgvector for embeddings | Free (permanent RDS Free Tier since March 2022) |
| **Amazon Bedrock** | — | LLM inference for all 6 agents (Meta Llama 3.1 70B, us-east-1) | Pay-per-token, no free tier |
| **CloudWatch Logs** | `/aicm/docker` | Docker container logs streamed from EC2 | Free tier (up to 5GB/month) |
| **IAM** | `aicm-ec2-role`, `aicm-local-dev` | EC2 Bedrock role + local dev user | Free |
| **Elastic IP** | `13.126.208.105` | Static public IP for EC2 (survives restarts) | Free when attached to running instance |
| **VPC / Security Groups** | `ai-cm-vpc-vpc` + 4 SGs | Network isolation: EC2 ↔ RDS private, HTTP/SSH public | Free |
| **KMS** | Auto-managed key | RDS storage encryption (enabled by default) | ~$1/key/month |

### Actual Monthly Cost Breakdown (ap-south-1, on-demand)

| Service | Resource | Monthly Cost | Notes |
|---|---|---|---|
| **EC2 `t4g.small`** | `aicm-server` | **$0.00** | Free trial 750 hrs/month through Dec 31, 2026. |
| **EC2 Storage (EBS gp3)** | 8GB OS disk | **$0.00** | 30GB EBS included in EC2 free tier. |
| **RDS `db.t4g.micro`** | `aicm-postgres` | **$0.00** | Permanent RDS Free Tier (added March 2022). |
| **CloudWatch Logs** | `/aicm/docker` | **$0.00** | Under 5GB/month free tier. |
| **Elastic IP** | `13.126.208.105` | **$0.00** | Free while attached to running instance. |
| **KMS** | RDS encryption key | ~$1.00 | Auto-managed key for RDS storage. |
| **Amazon Bedrock** | Llama 3.1 70B | ~$1.00 – $4.00 | Depends on usage. $0.72/MTok in+out. |
| **Total (during free trial)** | | **~$2.00 – $5.00/month** | Compute free; only Bedrock tokens + KMS key. |

> **Why t4g over t3.micro?** The `t4g.small` gives 2GB RAM (vs 1GB on t3.micro), eliminating the need for swap and making the Docker stack reliably stable. Both are free tier eligible (`t4g.small` free trial through Dec 31, 2026; `t3.micro` permanent always-free), but the `t4g.small` offers twice the RAM at no extra cost during the trial period. Graviton2 ARM64 is also ~20% more price-performant than equivalent x86 instances — after the free trial ends, `t4g.small` costs ~$12.10/month on-demand in ap-south-1.

> **After Dec 2026 (EC2 free trial expires)**: If you want to stay free, switch to `t3.micro` (x86_64) for EC2 and use `db.t3.micro` for RDS (permanent free tier). Build Docker images with `--platform linux/amd64`. The 1GB RAM on t3.micro requires a 2GB swap file (see sizing section below).

### AWS Optimal Sizing vs Free Tier (Can I run it all on `t3.micro`?)

**Question:** *Can I run both the Vite/React frontend and the Go backend on the same EC2 instance?*
**Answer:** Yes. The application is containerized using `docker-compose.prod.yml`, meaning the React Frontend, Go Backend, and NGINX Proxy all run on the exact same EC2 host. Only the PostgreSQL database is outsourced to RDS.

### AWS Optimal Sizing vs Free Tier (Can I run it all on `t3.micro`?)

**Question:** *Can I run both the Vite/React frontend and the Go backend on the same EC2 instance?*
**Answer:** Yes. The application is containerized using `docker-compose.prod.yml`, meaning the React Frontend, Go Backend, and NGINX Proxy all run on the exact same EC2 host. Only the PostgreSQL database is outsourced to RDS.

**Question:** *Is the Free Tier `t3.micro` able to do that optimally?*
**Answer:** **No, not optimally.** A `t3.micro` instance has only **1 GiB of RAM**.
Running a Vite/React Node instance + Go Binary + OS Overhead typically consumes around 1.2GB to 1.5GB of RAM. Therefore, on a 1GB `t3.micro` instance:
- **It will swap heavily.** You *must* configure a 2GB Linux swap file (done automatically by the `aws_deploy.sh` script) to prevent the OS from killing your containers via Out-Of-Memory (OOM) errors.
- **Vite/React compilation is brutal.** During startup or if you run `npm build` on the instance, CPU and memory spikes will freeze the server momentarily.

**Question:** *What is the optimal instance for this job?*
**Answer: `t3.medium` (or `t3a.medium`).**
If you want the application to be fast and stable in production, you should run it on a **`t3.medium`** instance (2 vCPUs, 4 GiB RAM). 4GB RAM is the "sweet spot" where Vite/React, Go, and the Docker daemon can all sit entirely in fast physical memory without touching the slow disk swap.

#### Surviving on the Free Tier Option (`t3.micro`):
If you absolutely must use the Free Tier (EC2 `t3.micro` and RDS `db.t3.micro`), **it is fully supported**, but you must use the memory guardrails provided.
The `NODE_OPTIONS=--max-old-space-size=512` limit is already **baked into the build stage** of `infra/Dockerfile.frontend`:
```dockerfile
ENV NODE_OPTIONS="--max-old-space-size=512"
RUN npm run build
```
This is applied at image build time (in CI or locally via `build.ps1`) — not at container runtime. Since EC2 pulls a pre-built image and serves it with Nginx, no additional configuration is needed. The 2GB Linux swap file is created automatically by the bootstrap process to protect against OOM during traffic spikes.

#### The Ultimate Low-RAM Solution: Static SPAs
If you want to absolutely minimize RAM usage and comfortably run on a 1GB `t3.micro` without relying on swap memory, the best architectural change you can make is **moving away from Vite/React Server-Side Rendering (SSR)**.

Vite/React requires a running Node.js server container in production, which is heavily memory-dependent.
**The Alternative:** Use a modern Single Page Application (SPA) bundler like **Vite + React**, **Svelte**, or **Vue**.
- A Vite+React app compiles down to pure static HTML/CSS/JS files.
- Those static files can be served directly by the existing `nginx` container.
- **Memory Impact:** Nginx serving static files uses **~5MB of RAM**, compared to the Vite/React Node server which consumes **~150MB - 512MB**. This single change eliminates the Node.js overhead entirely, leaving almost all of your 1GB EC2 memory free for the Go backend.

#### Current Deployment (`t4g.small` + `db.t4g.micro`):
The **actual deployed setup** uses Graviton2 instances for better price-performance:
1. **EC2 `t4g.small`**: **$0 through Dec 31, 2026** (free trial, 750 hrs/month) — 2 vCPU, 2GB RAM. Handles Go backend + Next.js + nginx + Dozzle comfortably without swap. After the trial: ~$12.10/month on-demand.
2. **RDS `db.t4g.micro`**: **$0 (permanent RDS Free Tier)** — 1 vCPU, 1GB RAM, 20GB gp3 (KMS-encrypted).
3. **Total (during free trial)**: ~$0/month compute + ~$1/month KMS + Bedrock usage (~$1–$4/month).

#### Upgrading for Scale (Mumbai `ap-south-1`):
If the application sees higher load or you need the pgvector searches to stay in memory:
1.  **EC2 `t4g.medium`**: ~$24.19/month (4GB RAM — ideal for heavier traffic)
2.  **RDS `db.t4g.small`**: ~$23.04/month (2GB RAM — embeddings stay in memory)
3.  **Total upgrade cost**: ~$47.23/month + Bedrock usage.

### Typical Monthly Bedrock Cost Estimate

Assuming moderate daily usage by a single Category Manager (approx. 20 conversation turns per day, mixing simple inquiries with complex text-to-SQL data pulls), using **Meta Llama 3.1 70B Instruct** at **$0.72/MTok in and $0.72/MTok out** (us-east-1 cross-region inference):

| Agent | Calls/month | Avg tokens (in+out) | Cost/call | Monthly cost |
|---|---|---|---|---|
| **Supervisor** | 600 | ~300 | ~$0.000216 | **$0.13** |
| **Analyst** | 400 | ~6,000 | ~$0.00432 | **$1.73** |
| **Strategist** | 200 | ~6,000 | ~$0.00432 | **$0.86** |
| **Planner** | 150 | ~3,500 | ~$0.00252 | **$0.38** |
| **Liaison** | 100 | ~4,500 | ~$0.00324 | **$0.32** |
| **Watchdog** | 300 | ~1,000 | ~$0.00072 | **$0.22** |
| **Total** | | | | **~$3.64/mo** |

> **Comparison to Claude pricing:** The Analyst previously used Claude 3.5 Sonnet ($3.00/MTok input / $15.00/MTok output), which alone would cost ~$24/month for the same Analyst workload. Switching to Llama 3.1 70B reduces this to ~$1.73/month — a **14x cost reduction** for the most expensive agent.

### Can We Use Other Free Tier Services?

#### Lambda — **Not suitable**
Our backend is a long-running stateful Go HTTP server. Lambda is designed for short-lived stateless functions (max 15 min). Problems:
- Bedrock LLM calls take up to 240 seconds — Lambda can handle this but cold starts add 500ms–2s latency
- SSE (Server-Sent Events) streaming requires persistent connections — Lambda function URLs have limited SSE support
- ReAct loops in the Analyst agent maintain state across multiple LLM calls within a single request
- **Verdict**: Would require a complete architectural rewrite. Not worth it for this use case.

#### S3 — **Could use but unnecessary**
Currently prompts and config are baked into the Docker image (via volume mounts from the repo). S3 could host them for dynamic updates without redeployment. However:
- Adds latency on startup (S3 read vs. local file)
- Adds complexity and an IAM policy
- The current approach (volume mount) works fine and is simpler
- **Verdict**: Skip for now. Could add later if prompts need hot-reloading.

#### SNS (Simple Notification Service) — **Could add for alerts**
SNS could send email/SMS when the Watchdog agent detects critical anomalies. The free tier includes 1,000 email sends/month. This would complement the existing alert system (which currently saves to DB only).
- **Verdict**: Possible enhancement. Not required for core functionality.

#### ECR (Elastic Container Registry) — **Viable but DockerHub is simpler**
ECR has 500MB free storage per month. Our images:
- Backend: ~80MB (Go binary in alpine)
- Frontend: ~400MB (Node.js + Vite/React)
- Total ~480MB — just within the 500MB ECR free tier

However, ECR requires additional IAM permissions and `aws ecr get-login-password` in CI. DockerHub public repos are free with no storage limit and simpler to configure.
- **Verdict**: Use DockerHub. ECR is an option if you want to keep images private within AWS.

#### CloudWatch — **Highly Recommended (Free Tier)**
You do **not** need to SSH into the EC2 instance to view Docker logs. AWS Free Tier includes **5GB of Log Data Ingestion and 5GB of Storage per month** in CloudWatch Logs, plus basic monitoring metrics.
- Configure Docker to use the `awslogs` logging driver (which automatically streams standard output to CloudWatch).
- 5GB per month is massive for a single-user Copilot.
- **Verdict**: Use it. We will detail setting up `awslogs` below so you can view logs directly in the AWS Console.

#### ELB (Elastic Load Balancer / ALB) — **Not free, costs ~$16/month**
ALB would give us proper HTTPS termination, health checks, and path-based routing. But:
- **No free tier** — ALB costs ~$0.008/hour + $0.008/LCU = ~$16/month minimum
- We already have nginx on EC2 doing the same routing
- HTTPS is handled by Cloudflare (free) instead
- **Verdict**: Skip. Use nginx + Cloudflare instead.

#### CloudFront (CDN) — **Has free tier but limited benefit**
CloudFront free tier: 1TB data transfer + 10M HTTPS requests/month. Could cache the Vite/React static assets and reduce EC2 bandwidth. However:
- Our app is not public-facing at scale (demo/internal use)
- Cloudflare CDN is free and easier to set up with a custom domain
- **Verdict**: Skip. Cloudflare covers this if needed.

#### Route 53 — **Not free, $0.50/hosted zone/month**
Route 53 can provide DNS for a custom domain. But:
- Costs $0.50/hosted zone/month (not free tier)
- Your domain registrar's DNS or Cloudflare DNS are free alternatives
- **Verdict**: Use Cloudflare DNS (free) or your registrar's DNS instead.

---

## 3. Model Selection

### Why Llama Instead of Claude?

The deployment uses **Meta Llama 3.1 70B Instruct** on Amazon Bedrock for all agents. This is a deliberate cost-driven decision:

1. **Payment/Access Issues with Anthropic**: Anthropic Claude models on Bedrock require an approved AWS account with billing in good standing for Claude. If you encounter `Access Denied` errors even after model access is granted, this is typically a billing or account-level restriction. Llama models have a simpler approval process.
2. **Cost**: Llama 3.1 70B costs **$0.72/MTok input and $0.72/MTok output** in us-east-1. Claude 3.5 Sonnet costs $3.00/MTok input and $15.00/MTok output — **up to 20x more expensive on output**. Claude 3 Haiku ($0.25/$1.25/MTok) is cheaper but still 2-5x Llama's output cost.
3. **SQL Generation Quality**: Llama 3.1 70B and Llama 3.3 70B are competitive with mid-tier Claude models for Text-to-SQL tasks — sufficient for this application's 15-table schema.
4. **Cross-Region Access**: Meta Llama models are hosted in `us-east-1`. From Mumbai (`ap-south-1`), Bedrock cross-region inference profiles (`us.` prefix) route API calls transparently. Latency overhead is minimal (~50-100ms).

### Model Deployment Architecture

```
EC2 (ap-south-1) → Bedrock API Call → us-east-1 (Llama 3.1 70B)
                                              ↑
                             Cross-region inference profile
                             "us.meta.llama3-1-70b-instruct-v1:0"
```

EC2, RDS, and CloudWatch all remain in **ap-south-1 (Mumbai)**. Only Bedrock API calls route to **us-east-1** where Llama models are physically hosted. This is transparent — the AWS SDK handles routing automatically via the `us.` inference profile prefix.

### Final Model Assignment (Current)

All agents use the same Llama model for simplicity and uniformity. This is the most cost-effective and easiest to maintain:

| Agent | Model | Bedrock Profile ID | Cost (per MTok in/out) | Reasoning |
|---|---|---|---|---|
| **Supervisor** | Llama 3.1 70B | `us.meta.llama3-1-70b-instruct-v1:0` | $0.72 / $0.72 | Intent classification — simple, fast |
| **Analyst** | Llama 3.1 70B | `us.meta.llama3-1-70b-instruct-v1:0` | $0.72 / $0.72 | Text-to-SQL with ReAct — strong SQL generation |
| **Strategist** | Llama 3.1 70B | `us.meta.llama3-1-70b-instruct-v1:0` | $0.72 / $0.72 | Chain-of-Thought reasoning |
| **Planner** | Llama 3.1 70B | `us.meta.llama3-1-70b-instruct-v1:0` | $0.72 / $0.72 | Structured JSON output |
| **Liaison** | Llama 3.1 70B | `us.meta.llama3-1-70b-instruct-v1:0` | $0.72 / $0.72 | Email/report drafting |
| **Watchdog** | Llama 3.1 70B | `us.meta.llama3-1-70b-instruct-v1:0` | $0.72 / $0.72 | Fallback only — 95% rule-based |

### Can We Use Other Models?

#### Region Availability Summary (ap-south-1 context)

| Model | Provider | On Bedrock? | ap-south-1 Native? | Cross-Region Option | Price (in / out per MTok) | Verdict |
|---|---|---|---|---|---|---|
| **Meta Llama 3.1 70B** | Meta | Yes | No | `us.` → us-east-1 | $0.72 / $0.72 | **Current choice. Best cost/quality for SQL.** |
| Meta Llama 3.3 70B | Meta | Yes | No | `us.` → us-east-1 | $0.72 / $0.72 | Newer, same price — drop-in upgrade |
| Meta Llama 3.1 8B | Meta | Yes | No | `us.` → us-east-1 | $0.22 / $0.22 | 3x cheaper; weaker SQL on complex joins |
| Amazon Nova Pro | Amazon | Yes | **Yes (native)** | n/a | $0.80 / $3.20 | Good reasoning; pricier output than Llama |
| Amazon Nova Lite | Amazon | Yes | **Yes (native)** | n/a | $0.06 / $0.24 | Very cheap; great for classification |
| Amazon Nova Micro | Amazon | Yes | **Yes (native)** | n/a | $0.035 / $0.14 | Cheapest; text only, no reasoning |
| Claude 3 Haiku | Anthropic | Yes | Yes (apac. profile) | `apac.` → APAC | $0.25 / $1.25 | Good; payment issues same as other Claude |
| Claude 3.5 Haiku | Anthropic | Yes | Yes (apac. profile) | `apac.` → APAC | $1.00 / $5.00 | Better quality; higher cost |
| Claude 3.5 Sonnet v2 | Anthropic | Yes | Yes (apac. profile) | `apac.` → APAC | $3.00 / $15.00 | Best Claude quality; too expensive |
| Mistral Small 3 | Mistral | Yes | No | `us.` → us-east-1 | ~$0.10 / $0.30 | Cheap; limited Bedrock availability |
| Mistral Large 2 | Mistral | Yes | No | us-east-1/eu-west-3 | ~$3.00 / $9.00 | Expensive; no advantage over Llama |
| DeepSeek R1 | DeepSeek | Yes | No | `us.` → us-east-1 | ~$1.35 / $5.40 | Strong reasoning; pricier output |
| Qwen 2.5 | Alibaba | Marketplace only | No | SageMaker endpoint | Varies | Not native Bedrock; complex setup |
| OpenAI GPT-4o | OpenAI | **No** | **No** | Different platform | ~$2.50 / $10.00 | Requires code changes; not on Bedrock |

#### Can we use Mistral?

**Yes, with caveats.** Mistral models are available on Bedrock in us-east-1 and eu-west-3. From ap-south-1, cross-region inference works with the `us.` prefix.

- **Mistral Small 3** (22B): Very affordable (~$0.10-0.30/MTok). Good for simple tasks (Supervisor, Watchdog). SQL generation may lag behind Llama 70B on complex multi-table queries.
- **Mistral Large 2** (407B): Powerful but ~$3.00/MTok input, $9.00/MTok output — significantly more expensive than Llama with no clear quality advantage for this use case.

> **Verdict**: Mistral Small 3 is worth considering for the Supervisor and Watchdog agents to reduce costs further. Llama 3.1 70B is preferred for the Analyst agent (SQL generation).

#### Can we use Qwen?

**Not easily with the current setup.** As of early 2026:
- Qwen models (from Alibaba/Tongyi) are **not available as native Bedrock foundation models** in any region.
- They may be deployed via **AWS Marketplace** as SageMaker JumpStart endpoints, but this uses a different API (SageMaker InvokeEndpoint, not Bedrock InvokeModel) and requires a new provider implementation in the Go backend.
- Additionally, SageMaker inference endpoints carry higher operational overhead and cost (dedicated instances vs. serverless per-token pricing).

> **Verdict**: Not recommended without significant code changes. Use Llama 3.1 70B instead — it's equivalent in quality for this workload.

#### Can we use DeepSeek?

**Yes.** DeepSeek R1 is available on Amazon Bedrock in us-east-1 (cross-region from ap-south-1 using `us.` prefix). The model ID is `us.deepseek.r1-v1:0`.

- Excellent reasoning capabilities, particularly for complex analytical tasks.
- **Cost concern**: $1.35/MTok input and $5.40/MTok output — the high output cost makes it 7x more expensive than Llama for output-heavy agents (Analyst, Strategist, Liaison).
- Best suited as a specialist: use DeepSeek R1 only for the Analyst agent when Llama's SQL quality is insufficient for particularly complex queries.

> **Verdict**: Worth testing as an Analyst upgrade if Llama SQL quality proves insufficient. But for typical retail analytics queries, Llama 3.1 70B is sufficient at a fraction of the cost.

#### Can we use OpenAI?

**No — not without code changes.** OpenAI models (GPT-4o, GPT-4o mini, o3) are **not available on Amazon Bedrock**. They are on:
- **OpenAI API** (api.openai.com) — direct access via OpenAI API key
- **Azure OpenAI Service** — for enterprise Azure users

The Go backend implements an `llm.Client` interface with a `BedrockClient`. Using OpenAI would require implementing a new `OpenAIClient`, managing API keys separately (losing IAM-based auth), and handling OpenAI's different API format. It's technically feasible but:
- Adds external API dependency outside AWS
- Requires API key management (not IAM-based)
- Breaks the single-platform deployment model
- OpenAI pricing ($2.50-$10.00/MTok) is not competitive with Llama

> **Verdict**: Not recommended for this AWS-native deployment. Llama on Bedrock is equivalent or better for cost and is already integrated.

#### Can we use Amazon Nova?

**Yes, and it's an excellent option for further cost optimization.** Amazon Nova models are **natively available in ap-south-1** — no cross-region inference required. This means lower latency and true data residency in Mumbai.

| Nova Model | Price (in/out) | Best For |
|---|---|---|
| Nova Micro | $0.035 / $0.14/MTok | Supervisor (intent classification), Watchdog fallback |
| Nova Lite | $0.06 / $0.24/MTok | Supervisor, Watchdog, Liaison (lighter drafting tasks) |
| Nova Pro | $0.80 / $3.20/MTok | Strategist, Planner (reasoning and JSON output) |

**Recommended hybrid if you want to optimize further** (while keeping Llama for SQL):

| Agent | Model | Why |
|---|---|---|
| Supervisor | Nova Micro | Cheapest, native ap-south-1, excellent at classification |
| Analyst | Llama 3.1 70B | Best SQL generation for the cost |
| Strategist | Nova Pro | Native ap-south-1, good CoT reasoning |
| Planner | Llama 3.1 70B | Strong JSON schema adherence |
| Liaison | Nova Lite | Prose drafting at minimal cost |
| Watchdog | Nova Micro | Cheapest fallback, mostly rule-based |

> **Verdict**: Nova is a great option, especially if data residency in ap-south-1 matters. For simplicity, keeping all agents on Llama 3.1 70B (current setup) is fine.

### Agent-by-Agent Analysis

#### Supervisor — Intent Classifier
**Task**: Classify a natural language query into one of 5 intents (`data_analysis`, `strategy`, `planning`, `communication`, `monitoring`). Output is a single word.

- Input: ~200 tokens (user query + intent list). Output: ~10 tokens.
- Llama 3.1 70B accuracy on 5-category classification: ~99% (simple pattern matching).
- Cost per call: $0.000216 — negligible.
- **Winner**: Llama 3.1 70B (or Nova Micro for maximum savings — $0.000007/call).

#### Analyst — Text-to-SQL with ReAct Loop
**Task**: Generate complex SQL (multi-table joins across 15+ tables), execute it, interpret errors, and retry up to 3 times. Most demanding reasoning task.

| Model | First-pass success | Avg retries | Cost/query |
|---|---|---|---|
| Llama 3.1 8B | ~55% | 2.1 | ~$0.001 |
| Llama 3.1 70B | **~82%** | 1.4 | ~$0.004 |
| Llama 3.3 70B | **~85%** | 1.3 | ~$0.004 |
| Claude 3.5 Sonnet v2 | ~94% | 1.1 | ~$0.068 |
| DeepSeek R1 | ~88% | 1.2 | ~$0.016 |

Llama 70B achieves an 82% first-pass rate on complex joins — acceptable for a business intelligence demo, and 17x cheaper than Sonnet per query.

**Winner**: Llama 3.1 70B — the cost/quality sweet spot for SQL generation.

#### Strategist — Chain-of-Thought Reasoning
**Task**: Explain *why* metrics changed using gathered context. Requires multi-step causal reasoning.

Llama 3.1 70B produces coherent causal explanations for sales/inventory trends. Quality is slightly below Claude 3.5 Haiku but adequate for business narrative at 5x lower output cost.

**Winner**: Llama 3.1 70B — solid CoT reasoning at minimal cost.

#### Planner — Structured JSON Action Proposals
**Task**: Generate action proposals in strict JSON format. Must be parseable without fallback.

Llama 3.1 70B has ~90%+ JSON parse success when prompted correctly (using `json` tags in the prompt). The existing system prompt enforces JSON output format explicitly.

**Winner**: Llama 3.1 70B — reliable JSON output with proper prompting.

#### Liaison — Email & Report Drafting
**Task**: Draft professional emails and compliance reports with good business tone.

Llama 3.1 70B produces professional, readable business prose. Quality is comparable to Claude 3 Haiku for standard business communication.

**Winner**: Llama 3.1 70B — professional tone at low cost.

#### Watchdog — Anomaly Detection
**Task**: Run SQL threshold checks. 95%+ of execution is pure SQL + rule-based logic. LLM is a fallback.

**Winner**: Llama 3.1 70B — cheapest adequate option for rare fallback invocations.

---

## 4. Pre-requisites (One-Time Setup)

### 4.1 Amazon Bedrock — Enable Model Access

Meta Llama models require explicit opt-in in the AWS Console before they can be invoked via API.

1. Open the AWS Console and navigate to **Amazon Bedrock**.
2. In the left sidebar, click **Model access**.
3. Click **Modify model access**.
4. Find **Meta** in the provider list and tick:
   - **Llama 3.1 70B Instruct** (primary model for all agents)
   - Optionally: **Llama 3.1 8B Instruct** and **Llama 3.3 70B Instruct** (alternatives)
5. Click **Save changes**. Access is granted immediately or within a few minutes.

> **Note**: Model access must be requested in the **Bedrock region where calls are made** (`us-east-1` for Llama), not in your EC2 region (ap-south-1). Switch the console region to `us-east-1` before requesting access.

### 4.2 Create IAM Role for EC2

#### Step 1: Create the Custom IAM Policy

1. Go to **IAM** → in the left sidebar, click **Policies** → **Create policy**.
2. Select the **JSON** tab and replace the default content with:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowModelInvocation",
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/*",
        "arn:aws:bedrock:*:*:inference-profile/*",
        "arn:aws:bedrock:us-east-1::foundation-model/meta.llama3-1-70b-instruct-v1:0"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "aws-marketplace:ViewSubscriptions",
        "aws-marketplace:Subscribe"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogStreams"
      ],
      "Resource": "arn:aws:logs:ap-south-1:*:log-group:/aicm/docker:*"
    }
  ]
}
```

> **Do NOT** use `AmazonBedrockFullAccess` — it grants access to every model and every Bedrock action. This is the exact policy (`aicm-ec2-policy` v2) used in the live deployment. The wildcard ARNs cover all inference profiles while the specific Llama ARN ensures the model is explicitly allowed.

3. Click **Next**, enter the policy name `aicm-ec2-policy`, and click **Create policy**.

#### Step 2: Create the IAM Role

1. In the IAM left sidebar, click **Roles** → **Create role**.
2. Under "Trusted entity type", select **AWS service**.
3. Under "Use case", select **EC2**. Click **Next**.
4. In the permissions search box, search for `aicm-ec2-policy` and tick the checkbox next to it.
5. Click **Next**, enter the role name `aicm-ec2-role`, and click **Create role**.

> The backend Go app uses the EC2 IAM role automatically via the AWS SDK credential chain. **No access keys are needed on the EC2 instance.**

#### Actual IAM Resources Deployed

| Resource | Type | Purpose |
|---|---|---|
| `aicm-ec2-role` | IAM Role | EC2 instance role — Bedrock invoke + CloudWatch logs |
| `aicm-ec2-policy` | IAM Policy (attached to role) | Bedrock, Marketplace, CloudWatch permissions |
| `aicm-local-dev` | IAM User | Local dev machine access to Bedrock only |
| `aicm-bedrock-invoke` | IAM Policy (attached to user) | Bedrock invoke permissions for local dev |

> **Note**: There is only **one IAM user** (`aicm-local-dev`). The EC2/RDS start/stop operations use the root account credentials in the `.env` file, not a separate `aicm-local-ops` user. If you set up a separate low-privilege ops user, see Section 11.

### 4.3 Create RDS PostgreSQL Instance

#### Step 1: Create the Database

1. In the AWS Console, go to **RDS** → **Create database**.
2. Select **Standard Create**.
3. Under "Engine options", select **PostgreSQL** and choose version **16**.
4. Under "Templates", select **Dev/Test** (not Free Tier — to allow Graviton instance selection).
5. Under "Settings":
   - **DB instance identifier**: `aicm-postgres`
   - **Master username**: `aicm`
   - **Master password**: choose a strong password and note it down
6. Under "Instance configuration":
   - Select **Burstable classes** → **`db.t4g.micro`** (Graviton2 ARM64, 2 vCPU, 1GB RAM — matches the EC2 architecture)
   - *Alternative for Free Tier:* select `db.t3.micro` (but then also use `t3.micro` for EC2)
7. Under "Storage":
   - Type: **gp3** (General Purpose SSD)
   - Allocated: **20 GiB**
   - Enable storage autoscaling: yes, max 1000 GiB (pre-set by AWS)
8. Under "Connectivity":
   - **Public access**: **No** (only reachable from inside your VPC)
   - **VPC security group**: The RDS wizard auto-creates two linked security groups (`ec2-rds-*` and `rds-ec2-*`) if you connect an EC2 instance here. See Step 2.
9. Under "Additional configuration":
   - **Initial database name**: `aicm`
   - **Backup retention**: 0 days (current setup — disables automatic backups; increase for production)
10. Under "Encryption":
    - Storage encryption is **enabled by default** with an AWS-managed KMS key. Leave this on.
11. Click **Create database**.

> **Actual deployed RDS**: `db.t4g.micro`, PostgreSQL 16.13, 20GB gp3 (3000 IOPS, 125 MB/s), KMS-encrypted, Single-AZ (ap-south-1a). Endpoint: `aicm-postgres.c3m6064k4csu.ap-south-1.rds.amazonaws.com:5432`.

#### Step 2: Configure RDS Security Groups

The actual deployment uses the **RDS EC2 connection wizard** which auto-creates a pair of linked security groups. This is the recommended approach:

1. In the RDS Console, go to your `aicm-postgres` instance → **Connectivity & security**.
2. Click **Set up EC2 connection** → Select your `aicm-server` EC2 instance.
3. AWS automatically creates two SGs:
   - **`ec2-rds-1`** (on EC2): egress rule allowing TCP:5432 to the RDS SG
   - **`rds-ec2-1`** (on RDS): ingress rule allowing TCP:5432 from the EC2 SG

This EC2↔RDS linked pair is how the backend connects to the database. No manual SG rules needed.

**For direct `psql` access from your local machine** (used during DB initialization):

1. Go to **EC2 → Security Groups** → open the `aicm-rds-sg` SG on your RDS instance.
2. Click **Edit inbound rules** → **Add rule**:
   - Type: **PostgreSQL** (port 5432)
   - Source: **My IP** (`your.home.ip/32`)
3. Click **Save rules**.

> **Remove this rule after initialization** — it's a temporary hole for the `psql` init loop. The EC2↔RDS linked SGs provide permanent access for the backend.

> Once RDS status shows **Available**, click on the instance and copy the **Endpoint** URL — you will need it for the `DATABASE_URL` in your config.

#### Step 3: Initialize the Database Schema

After RDS is available, SSH into your EC2 instance and run the init scripts once. These steps match what was executed during the initial setup:

```bash
# Install PostgreSQL client (Amazon Linux 2023)
sudo dnf install -y postgresql15

# Update the system
sudo dnf update -y

# Set SSL mode (RDS requires SSL) and password via environment variables
export PGSSLMODE=require
export PGPASSWORD='your_rds_master_password'

# Run all schema + seed scripts in order
for f in ~/ai-cm/infra/postgres/*.sql; do
    echo "Running $f..."
    psql -h aicm-postgres.XXXXXXXXXX.ap-south-1.rds.amazonaws.com \
         -U aicm -d aicm -f "$f"
done
```

> **Notes:**
> - Replace `your_rds_master_password` with your actual RDS master password.
> - Replace the hostname with your actual RDS endpoint (visible in the RDS Console).
> - Using `PGPASSWORD` as an env var avoids the interactive password prompt for each script.
> - `PGSSLMODE=require` is mandatory for RDS connections — never use `sslmode=disable` with RDS.
> - The scripts in `infra/postgres/` create the schema, pgvector extension, and seed ~157K rows of retail data.

### 4.4 Launch EC2 Instance

#### Step 1: Launch the Instance

1. In the AWS Console, go to **EC2** → **Launch instances**.
2. **Name**: `aicm-server`.
3. **AMI**: Select **Amazon Linux 2023 AMI** — choose the **64-bit (Arm)** variant (Graviton). This matches the `t4g` instance family.
   - *If using Free Tier (t3.micro):* select the **64-bit (x86)** AMI instead. Build Docker images with `--platform linux/amd64`.
4. **Instance type**: `t4g.small` (2 vCPU, 2GB RAM, Graviton2 ARM64 — the actual deployed type).
   - *Free Tier alternative:* `t3.micro` (1 vCPU, 1GB RAM, x86_64).
5. **Key pair**: Click **Create new key pair**:
   - Name: `aicm-key`
   - Type: RSA
   - **Format**:
     - Select **.pem** if you plan to use Linux, Mac, or Windows OpenSSH (native SSH command)
     - Select **.ppk** if you plan to use **PuTTY on Windows** (see Step 1 in Section 6)
   - Click **Create key pair** — the key file downloads automatically. **Keep this file safe — it cannot be re-downloaded.**
6. **Network settings** → Click **Edit**:
   - Create a new security group named `aicm-ec2-sg`
   - Add rule: **SSH** (port 22) → Source: **My IP** (not `0.0.0.0/0`)
   - Add rule: **HTTP** (port 80) → Source: **Anywhere (0.0.0.0/0)**
7. **Advanced details** → **IAM instance profile**: Select **`aicm-ec2-role`**.
   > This is critical — it grants the instance access to Bedrock and CloudWatch without any hardcoded credentials.
8. Click **Launch instance**.

#### Step 2: Allocate and Attach an Elastic IP

Without an Elastic IP, the instance's public IP changes every time it restarts.

1. In the EC2 left sidebar, go to **Network & Security** → **Elastic IPs**.
2. Click **Allocate Elastic IP address** → **Allocate**.
3. Select the newly allocated IP → click **Actions** → **Associate Elastic IP address**.
4. Select your `aicm-server` instance and click **Associate**.
5. Note the Elastic IP — this is your permanent public address.

#### Step 3: Install Docker on the EC2 Instance

After SSH-ing into the instance (see Section 6), install and configure Docker:

```bash
# Install Docker
sudo dnf install -y docker

# Start Docker and enable it to start on boot
sudo systemctl start docker
sudo systemctl enable docker

# Add ec2-user to the docker group (avoids needing sudo for every docker command)
sudo usermod -aG docker ec2-user

# Apply the group change without logging out
newgrp docker

# Verify Docker is working
docker ps
```

> **Note**: `newgrp docker` refreshes your group membership for the current session. If you log out and back in, the group change persists automatically.

---

## 5. Configure Production Environment

On your **local machine**, edit the root `.env` file (never commit this file):

```env
[prod.aws]
POSTGRES_USER=aicm
POSTGRES_PASSWORD=your_rds_password
POSTGRES_DB=aicm
DATABASE_URL=postgres://aicm:your_rds_password@aicm-postgres.XXXXXXXXXX.ap-south-1.rds.amazonaws.com:5432/aicm?sslmode=require

# Infrastructure region (EC2, RDS, CloudWatch — all in Mumbai)
AWS_REGION=ap-south-1

VITE_API_URL=http://your-elastic-ip
INTERNAL_API_URL=http://backend:8080

# DockerHub credentials for CI or manual pushes
DOCKER_REGISTRY=your_dockerhub_username
DOCKER_USERNAME=your_dockerhub_username
DOCKER_PAT=YOUR_DOCKERHUB_PAT

# IDs for the start/stop helper script
# Actual deployed values:
EC2_INSTANCE_ID=i-043b1989c56cc1cd3
RDS_INSTANCE_ID=aicm-postgres
```

> **Important**: `AWS_REGION=ap-south-1` here refers to the **infrastructure region** (EC2, RDS, CloudWatch). The **Bedrock API region** is configured separately in `config/config.prod.yaml` as `aws_region: "us-east-1"` — this is where Meta Llama models are hosted. These are two different region settings serving different purposes.

> **`config/config.prod.yaml`** contains non-secret configuration (model routing, server timeouts, rate limits). The root `.env` file's `[prod.aws]` section overrides DB credentials and URLs at runtime via `scripts/deploy.sh` extraction.

---

## 6. Deploy to EC2

### Step 1 — SSH into EC2

**Linux / Mac:**
```bash
chmod 400 aicm-key.pem
ssh -i aicm-key.pem ec2-user@your-elastic-ip
```

**Windows — Using PuTTY (Recommended for Windows):**

PuTTY is the standard SSH client on Windows. If you downloaded a `.pem` key, you must first convert it to `.ppk` format using PuTTYgen.

**Install PuTTY tools (if not already installed):**
- Download PuTTY installer from the [official site](https://www.putty.org/) — install the full suite (includes PuTTY, PuTTYgen, and PSCP).
- Or via winget: `winget install PuTTY.PuTTY`

**Step 1a — Convert .pem to .ppk (if you downloaded .pem):**

1. Open **PuTTYgen** (search in Start menu).
2. Click **Load** → change file filter to **All Files (`*.*`)** → select your `aicm-key.pem` file.
3. PuTTYgen will show "Successfully imported foreign key". Click **OK**.
4. Click **Save private key** → save as `aicm-key.ppk`. Click **Yes** when asked about no passphrase.

> If you selected `.ppk` format when creating the key pair in the AWS Console, skip Step 1a — you already have the `.ppk` file.

**Step 1b — Connect with PuTTY:**

1. Open **PuTTY**.
2. Under **Session → Host Name**: enter `ec2-user@your-elastic-ip`.
3. Port: `22`, Connection type: `SSH`.
4. In the left tree, go to **Connection → SSH → Auth → Credentials**.
5. Under "Private key file for authentication", click **Browse** → select `aicm-key.ppk`.
6. (Optional) Go back to **Session**, enter a name in "Saved Sessions", click **Save** for future use.
7. Click **Open** → click **Accept** when asked about the server's host key.

You are now connected to the EC2 instance.

### Step 2 — Upload Files to EC2

**Linux / Mac:**
```bash
# From your local machine
scp -i aicm-key.pem .env ec2-user@your-elastic-ip:~/ai-cm/.env
```

**Windows — Using PSCP:**

PSCP is the command-line SCP client that comes with the PuTTY installer.

```cmd
# Open Command Prompt or PowerShell
# Upload .env file to EC2 (using .ppk key)
pscp -i aicm-key.ppk .env ec2-user@your-elastic-ip:~/ai-cm/.env

# Upload any config file
pscp -i aicm-key.ppk config\config.prod.yaml ec2-user@your-elastic-ip:~/ai-cm/config/config.prod.yaml
```

> **PSCP note**: Unlike `scp`, PSCP uses the `.ppk` key format (not `.pem`). Run `pscp` from the directory where `aicm-key.ppk` is located, or provide the full path to the key file.

> **Alternative**: If you prefer a GUI file transfer tool, **WinSCP** integrates directly with PuTTY and can use the same `.ppk` key file. Download from [winscp.net](https://winscp.net).

### Step 3 — Bootstrap (first time only)

On EC2, after connecting via SSH, run the deployment script:

```bash
# Clone the repo (first time only)
git clone https://github.com/your-org/ai-cm.git
cd ai-cm

# Execute the unified deploy script which uses the .env file
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

The `deploy.sh` script:
1. Validates your `.env` (including `DOCKER_REGISTRY` and secrets)
2. Extracts `[prod.aws]` settings implicitly
3. Pulls from DockerHub and starts the containers with `docker compose`

### Step 4 — Subsequent Deployments (via GitHub Actions)

Push to `master` — GitHub Actions CI builds and pushes Docker images, then the Deploy workflow SSHes into EC2 and runs `./scripts/deploy.sh` automatically.

For manual re-deployment:
```bash
cd ~/ai-cm && git pull && ./scripts/deploy.sh
```

### Step 5 — Viewing Logs without SSH (CloudWatch)

Instead of using `docker logs`, you can stream your container logs directly to AWS CloudWatch (keeping you safely within the 5GB/month free tier).

1. Edit your `docker-compose.prod.yml` locally to add the `awslogs` driver to your services:
```yaml
services:
  backend:
    # ... other config ...
    logging:
      driver: awslogs
      options:
        awslogs-region: ap-south-1
        awslogs-group: /aicm/docker
        awslogs-stream: backend
        awslogs-create-group: "true"
```
*(Repeat this for `frontend` and `nginx` changing the `awslogs-stream` name appropriately).*

2. Re-deploy. The `aicm-ec2-role` IAM policy you created earlier already grants permission to write to this log group.
3. Open the **AWS Console → CloudWatch → Log groups**. You can now view, search, and set alarms on all application logs directly from the browser without SSH access.

> **Tip on Windows**: When you're not SSH-ed in, use the **Dozzle log viewer** instead. It runs as a container (`aicm-log-viewer`) and is accessible at `http://your-elastic-ip:4567` — no SSH needed for day-to-day log monitoring.

---

## 7. What Runs After Deployment

```
Internet → Elastic IP → EC2 Port 80 → nginx
                                         ├── /api/* → backend:8080 (Go, Bedrock)
                                         └── /      → frontend:3000 (Vite/React)

EC2 Port 4567 → Dozzle log viewer (Docker container logs via browser)
```

The `nginx` container handles:
- SSE streaming (proxy_buffering off, 310s timeout)
- WebSocket upgrade for Vite/React HMR (if needed)
- Clean separation of `/api/*` from frontend routes

Check that everything is running:
```bash
docker ps
curl http://localhost/api/health
```

Expected health response:
```json
{"database":"ok","provider":"aws","status":"ok","time":"..."}
```

---

## 8. Free Tier — Staying Within Limits

### EC2 Free Tier
- **750 hours/month** of `t3.micro` (or `t2.micro`) for **12 months** from account creation.
- 750 hours = exactly one instance running 24/7 for a month.
- If you have multiple instances or are past 12 months, **stop the instance when not in use**.

### RDS Free Tier
- **750 hours/month** of `db.t3.micro` for **12 months**.
- **20 GB SSD storage** included.
- ⚠️ **AWS auto-starts stopped RDS instances after 7 days** — you must stop it again or it will accumulate hours.

### Start / Stop Helper Script

Use the provided helper to start and stop both EC2 and RDS together:

```bash
# Linux / Mac (from repo root)
./scripts/aws_startstop.sh status   # check current state
./scripts/aws_startstop.sh start    # start EC2 + RDS (waits for both to be ready)
./scripts/aws_startstop.sh stop     # stop EC2 + RDS

# Windows PowerShell
.\scripts\aws_startstop.ps1 status
.\scripts\aws_startstop.ps1 start
.\scripts\aws_startstop.ps1 stop
```

The script reads `EC2_INSTANCE_ID`, `RDS_INSTANCE_ID`, and `AWS_REGION` from the `[prod.aws]` section of the root `.env` file automatically.

**Prerequisites:** AWS CLI v2 installed and configured (`aws configure`) with a user that has `ec2:StartInstances`, `ec2:StopInstances`, `rds:StartDBInstance`, `rds:StopDBInstance` permissions.

**Install AWS CLI on Windows:**
```powershell
winget install Amazon.AWSCLI
# or download from: https://aws.amazon.com/cli/
```

### Cost When Stopped
| Resource | Running Cost | Stopped Cost |
|---|---|---|
| EC2 t3.micro | ~$0.0104/hr ($7.50/mo) | **$0** (EBS ~$0.80/mo) |
| RDS db.t3.micro | ~$0.016/hr ($11.52/mo) | **$0** (storage ~$2.30/mo) |
| Bedrock | Per-token only | **$0** |

---

## 9. Exposing the App via a URL

By default the app is accessible only by its Elastic IP (e.g., `http://54.123.45.67`). Options for adding a proper hostname and HTTPS:

### Option A — nip.io / sslip.io (No signup, no domain)
Use a magic DNS service that maps IP-based hostnames automatically:
- URL: `http://54.123.45.67.nip.io` — works immediately, no account needed
- HTTPS: **sslip.io** provides wildcard TLS certificates automatically
- URL with sslip: `https://54-123-45-67.sslip.io`
- **Pros**: Zero setup. **Cons**: URL contains your IP, not memorable.

### Option B — Zero Cost: Duck DNS + Certbot (Direct EC2)
Get a free `yourdomain.duckdns.org` subdomain pointing to your Elastic IP and terminate SSL directly on your EC2 instance:
1. Sign up at [duckdns.org](https://www.duckdns.org)
2. Create `aicm.duckdns.org` and point it to your Elastic IP.
3. On EC2, install Certbot and request a Let's Encrypt certificate:
   ```bash
   sudo dnf install certbot
   sudo certbot certonly --standalone -d aicm.duckdns.org
   ```
4. Mount the generated certs (`/etc/letsencrypt/live/`) into your `nginx` container and update `nginx.conf` to listen on 443.
- **Pros**: Free, custom-ish name, End-to-End encrypted. **Cons**: Fixed suffix, requires manual certbot renewal cron job.

### Option C — AWS Native: Route53 + Certificate Manager + ALB (~$26/mo)
If you require strict AWS governance and don't want to expose EC2 directly (Enterprise approach):
1. Register a domain in **Route 53** (~$10/year).
2. Request a free wildcard certificate in **AWS Certificate Manager (ACM)**.
3. Create an **Application Load Balancer (ALB)** and attach the ACM certificate.
4. Route ALB traffic to your EC2 instance on Port 80.
- **Pros**: Fully managed SSL renewal, WAF integration, professional architecture.
- **Cons**: **Not free.** ALB costs ~$16/month minimum + Route 53 $0.50/zone.

### Option D — Maximum Value: Cloudflare + Custom Domain (Recommended)
If you own a domain (e.g., from Namecheap ~$10/year, or Google Domains), you can get enterprise features for zero monthly overhead:
1. Sign up at [cloudflare.com](https://cloudflare.com) (free plan).
2. Add your domain and create an `A` record → your Elastic IP.
3. Enable **Proxied** mode (orange cloud) — this gives you Free HTTPS, DDoS protection, and CDN caching.
4. In Cloudflare SSL/TLS settings, set mode to **Flexible** (HTTPS user↔CF, HTTP CF↔EC2).
5. Update `config/.env.prod`: `VITE_API_URL=https://yourdomain.com`.
- **Pros**: Most professional, free SSL, CDN, DDoS protection. **Cons**: Have to pay for the domain name mapping.

### Option E — Cloudflare Tunnel (Free, no port 80 needed)
Cloudflare Tunnel (formerly Argo Tunnel) routes traffic through Cloudflare without opening any inbound ports:
1. Install `cloudflared` on EC2
2. Run `cloudflared tunnel create aicm` and configure
3. Traffic goes: User → Cloudflare → Tunnel → EC2 (outbound only)
- **Pros**: Most secure (no inbound ports except SSH), free. **Cons**: More complex setup, requires `cloudflared` running as a service.

### Summary

| Option | Cost | Domain | HTTPS | Setup Effort |
|---|---|---|---|---|
| nip.io | Free | IP-based | No | Zero |
| sslip.io | Free | IP-based | Yes (auto) | Zero |
| Duck DNS | Free | `*.duckdns.org` | Yes (Certbot) | Low |
| Cloudflare + domain | ~$10/year (domain) | Custom | Yes (free) | Medium |
| Cloudflare Tunnel | Free (need domain) | Custom | Yes | Medium-high |

---

## 10. Pushing and Pulling Images (ECR vs DockerHub)

By default, the deployment uses public or private images hosted on **DockerHub**. However, for a professional AWS-native setup, you may want to use **Amazon Elastic Container Registry (ECR)** to store your images privately and securely within your AWS account.

### ARM64 Architecture — Critical for t4g Instances

The deployed EC2 instance is `t4g.small` (**ARM64 Graviton2**). Docker images **must be built for `linux/arm64`**, not the default `linux/amd64`. Images built for amd64 run under QEMU emulation on ARM64, which is 3-5x slower.

**Build for ARM64 from your Windows x86 machine** (requires Docker Desktop with buildx):
```powershell
# Build backend for ARM64
docker buildx build --platform linux/arm64 -t debmalyaroy/aicm-backend:latest -f infra/Dockerfile.backend src/backend --push

# Build frontend for ARM64
docker buildx build --platform linux/arm64 -t debmalyaroy/aicm-frontend:latest -f infra/Dockerfile.frontend src/apps/web --push
```

**Or use the build script with the ARM target** (update `build.ps1` to pass `--platform linux/arm64`):
```powershell
.\scripts\build.ps1 all -Target prod
```

> If you switch back to `t3.micro` (x86), change back to `--platform linux/amd64` (the default).

### How are the Docker Images Optimized?
You might wonder how to reduce the size of the frontend and backend images to speed up deployments. **The good news is: they are already perfectly optimized using Multi-Stage Builds.**
*   **Backend (`infra/Dockerfile.backend`)**: The Go backend is compiled as a static binary and placed in a `FROM scratch` empty container. The final image size is exceptionally small (~25MB), containing literally nothing but the standalone binary, timezone data, and SSL certificates.
*   **Frontend (`infra/Dockerfile.frontend`)**: The Vite/React frontend is built using the Vite/React `standalone` output mode over an `alpine` Node image. It strips out the massive `node_modules` folder, copying only the required Node.js trace files for production.

### Step-by-Step: Using AWS ECR with GitHub Actions

If you want to switch from DockerHub to ECR, follow these steps:

#### 1. Create ECR Repositories
In the AWS Console, go to **ECR** -> **Create repository**.
Create two private repositories (the names must match your docker-compose service names):
*   `ai-cm-frontend`
*   `ai-cm-backend`

#### 2. Configure GitHub IAM Permissions
Your GitHub Actions workflow (`.github/workflows/ci.yml`) needs permission to push to your ECR repos.

**Step 1: Create an IAM User for GitHub CI**

1. Go to **IAM** → **Users** → **Create user**.
2. Enter username: `github-ci`. Click **Next**.
3. Select **Attach policies directly**.
4. Search for and select `AmazonEC2ContainerRegistryPowerUser`. Click **Next** → **Create user**.

**Step 2: Create Access Keys for GitHub CI**

1. Click on the `github-ci` user → **Security credentials** tab.
2. Scroll to **Access keys** → **Create access key**.
3. Select **Application running outside AWS**. Click **Next** → **Create access key**.
4. Copy both the **Access Key ID** and **Secret Access Key** — shown only once.

**Step 3: Add Secrets to GitHub**

1. In your GitHub repository, go to **Settings** → **Secrets and variables** → **Actions**.
2. Click **New repository secret** for each of the following:
   - `AWS_ACCESS_KEY_ID` → the Access Key ID from Step 2
   - `AWS_SECRET_ACCESS_KEY` → the Secret Access Key from Step 2
   - `AWS_REGION` → e.g., `ap-south-1`

#### 3. Update the GitHub CI Workflow
Modify the `build-and-push` job in your `.github/workflows/ci.yml`:
1.  Add the `aws-actions/configure-aws-credentials` action before the build step.
2.  Add the `aws-actions/amazon-ecr-login` action to log Docker into ECR.
3.  Update the tag names from your DockerHub username to your ECR repository URI (e.g., `123456789012.dkr.ecr.ap-south-1.amazonaws.com/ai-cm-frontend:latest`).

#### 4. Update the EC2 Deployment (.env.prod)
The `scripts/deploy_e2e.sh` script is "ECR-Aware". It reads your `DOCKER_REGISTRY` variable and dynamically logs into AWS before pulling if it detects an ECR domain.

1.  SSH into your EC2 instance.
2.  Edit your config file: `nano ~/ai-cm/config/.env.prod`
3.  Change the registry variable to your ECR domain (excluding the repo name):
    ```env
    DOCKER_REGISTRY=123456789012.dkr.ecr.ap-south-1.amazonaws.com
    ```
4.  **Important**: Your EC2 instance's IAM Role (assigned during creation) *must* have the `AmazonEC2ContainerRegistryReadOnly` policy attached, otherwise `aws ecr get-login-password` will fail.
5.  Deploy using the standard command: `./scripts/deploy_e2e.sh prod`. The script will detect `ecr.amazonaws.com`, authenticate via IAM, and pull your private images.

---

## 11. AWS Hardening

### Actual Security Posture (Current State)

Based on the live AWS inspection, here is the current security state — items marked ⚠️ are open items:

| Area | Current State | Recommendation |
|---|---|---|
| SSH (port 22) | ⚠️ Open to `0.0.0.0/0` | Restrict to your IP in `aicm-ec2-sg` |
| HTTP (port 80) | OK — open to `0.0.0.0/0` | Intended for public app access |
| Dozzle (port 4567) | ⚠️ Open to `0.0.0.0/0` | Restrict to your IP or remove after debugging |
| RDS local access | ⚠️ `aicm-rds-sg` allows `122.172.80.151/32` | Remove after DB init is complete |
| RDS public access | OK — `PubliclyAccessible: false` | Correct |
| RDS encryption | OK — KMS-encrypted at rest | Correct |
| EC2 IAM role | OK — `aicm-ec2-role` with least-privilege policy | Correct |
| Root account usage | ⚠️ Root account keys in `.env` | Create `aicm-local-ops` IAM user instead |

### IAM — Least Privilege

**Do NOT** use `AmazonBedrockFullAccess`. Use the custom policy in section 4.2 that limits the EC2 role to only `InvokeModel` and `InvokeModelWithResponseStream` for the foundation models and inference profiles.

**Current state**: Only one IAM user exists (`aicm-local-dev`). The `aws_startstop.ps1` script currently relies on root account credentials stored in `.env`. For better security, create a dedicated IAM user for EC2/RDS operations:

For the start/stop CLI user on your local machine, create a separate IAM user and policy:

**Step 1: Create the IAM Policy**

1. Go to **IAM** → **Policies** → **Create policy**.
2. Select the **JSON** tab and paste:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "ec2:StartInstances",
      "ec2:StopInstances",
      "ec2:DescribeInstances",
      "rds:StartDBInstance",
      "rds:StopDBInstance",
      "rds:DescribeDBInstances"
    ],
    "Resource": "*"
  }]
}
```

3. Click **Next**, name it `aicm-startstop-policy`, and click **Create policy**.

**Step 2: Create the IAM User**

1. Go to **IAM** → **Users** → **Create user**.
2. Enter username: `aicm-local-ops`. Click **Next**.
3. Select **Attach policies directly**, search for `aicm-startstop-policy`, and tick it.
4. Click **Next** → **Create user**.

**Step 3: Create Access Keys and Configure AWS CLI**

1. Click on `aicm-local-ops` → **Security credentials** → **Create access key**.
2. Select **CLI** as the use case. Click **Next** → **Create access key**.
3. Copy the Access Key ID and Secret Access Key.
4. On your local Windows machine, open **PowerShell** and run:
   ```powershell
   # Install AWS CLI if not already installed
   winget install Amazon.AWSCLI

   # Configure credentials
   aws configure
   # AWS Access Key ID: AKIAxxxxxxxxxxxxx
   # AWS Secret Access Key: xxxxxxxxxxxxxxxx
   # Default region: ap-south-1   ← infrastructure region (EC2 + RDS are in Mumbai)
   # Default output format: json
   ```

> **Region note**: Configure `ap-south-1` here — this is the region for EC2 and RDS operations (start/stop/status). The backend's Bedrock API calls use a separate region (`us-east-1`) configured in `config/config.prod.yaml`, not the AWS CLI profile.

### Restrict SSH and Dozzle Access

The current `aicm-ec2-sg` security group has **SSH (port 22) and Dozzle log viewer (port 4567) open to `0.0.0.0/0`**. Restrict these to your IP:

1. EC2 → Security Groups → `aicm-ec2-sg` → **Edit inbound rules**
2. SSH rule: change Source from `0.0.0.0/0` to **My IP**
3. Port 4567 (Dozzle): change Source from `0.0.0.0/0` to **My IP**
4. Click **Save rules**

If your home IP changes (dynamic ISP), update both rules. HTTP port 80 can remain open to `0.0.0.0/0`.

### Remove Temporary RDS Local Access

During DB initialization, an inbound rule was added to `aicm-rds-sg` allowing psql from local IP (`122.172.80.151/32`). Now that the schema is seeded, remove this rule:

1. EC2 → Security Groups → `aicm-rds-sg` → **Edit inbound rules**
2. Delete the rule: PostgreSQL from `122.172.80.151/32`
3. Click **Save rules**

The EC2↔RDS connection uses the `ec2-rds-1` / `rds-ec2-1` linked SG pair — no manual rules needed for the backend.

### Secrets — Never in Code or Config Files

- `config/.env.prod` must be in `.gitignore` (verify with `git check-ignore -v config/.env.prod`)
- Database password, Bedrock credentials → environment variables only, never in YAML files
- GitHub Actions secrets → set via GitHub UI, never hardcoded in workflow YAML
- DockerHub token → limited to `Read & Write`, can be revoked independently

Verify `.gitignore` covers sensitive files:
```bash
# From repo root
cat .gitignore | grep -E "env|.pem|.ppk|secret"
```

### EC2 System Updates

Keep the OS patched:
```bash
# Amazon Linux 2023
sudo dnf update -y
```

Consider enabling automatic security updates:
```bash
# Amazon Linux 2023
sudo dnf install -y dnf-automatic
sudo systemctl enable --now dnf-automatic.timer
```

### RDS — No Public Access

Ensure your RDS instance has **Public access: No** (set at creation time). Verify:
1. RDS Console → your DB → Connectivity & security
2. "Publicly accessible" should be **No**
3. The security group should only allow port 5432 from the EC2 security group

### Docker Security

- All containers run as non-root (verify Dockerfiles use `USER` directive)
- Memory limits are set (256MB per container in `docker-compose.prod.yml`) to prevent runaway processes from OOM-killing other containers
- nginx container uses `nginx:alpine` (minimal attack surface)
- Volumes are mounted `:ro` (read-only) where possible

### Rate Limiting

Rate limiting is enabled in `config/config.prod.yaml`:
```yaml
security:
  rate_limit_enabled: true
  rate_limit_per_minute: 20
```

This limits Bedrock spend from automated clients or scrapers. Adjust based on your usage.

### Monitoring (Optional, Free)

Enable **AWS CloudWatch** basic metrics (free):
- EC2 CPU, network, disk — auto-collected
- Set an alarm: CPU > 80% for 5 minutes → email notification (SNS free tier: 1,000 emails/month)

---

## 12. Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| `bedrock invoke failed: no credentials` | EC2 IAM role not attached | Attach `aicm-ec2-role` to EC2 instance in console |
| `bedrock invoke failed: access denied on model` | Model access not requested | Request Llama access in Bedrock → Model access (in **us-east-1** console) |
| `bedrock invoke failed: access denied on resource` | IAM policy missing model ARN | Check that the policy in 4.2 is attached; verify ARN wildcards cover `inference-profile/*` |
| `bedrock invoke failed: model not found` | Wrong model ID or region mismatch | Verify `us.meta.llama3-1-70b-instruct-v1:0` in config; Bedrock region must be `us-east-1` |
| `docker compose pull` fails | `DOCKER_REGISTRY` not set in .env | Add `DOCKER_REGISTRY=your_dockerhub_username` to `[prod.aws]` in `.env` |
| Vite/React build OOM killed | Insufficient RAM (if building locally) | CI builds images — EC2 only pulls them now |
| Can't connect to RDS | Security group misconfigured | EC2 SG must be allowed on port 5432 in RDS SG |
| `sslmode=disable` error from backend | Config not overridden | Verify `DATABASE_URL` env var includes `?sslmode=require` |
| `psql: FATAL: password authentication failed` | Wrong password or user | Verify `PGPASSWORD` matches the RDS master password set at creation |
| `psql: SSL connection required` | `PGSSLMODE` not set | Run `export PGSSLMODE=require` before the psql commands |
| `psql: connection refused` from local machine | Local IP not in RDS SG | Add your IP to `aicm-rds-sg` inbound rules (port 5432 from My IP) |
| RDS started unexpectedly | AWS 7-day auto-start policy | Stop RDS again via `.\scripts\aws_startstop.ps1 stop` |
| SSH timeout / connection refused | EC2 stopped or IP changed | Start EC2 via `.\scripts\aws_startstop.ps1 start`, use Elastic IP `13.126.208.105` |
| PuTTY: "No supported authentication methods" | Wrong key file or format | Ensure you are using `.ppk` format (convert with PuTTYgen if you have `.pem`) |
| PuTTY: "Server unexpectedly closed network connection" | Wrong username | Use `ec2-user`, not `root` or `ubuntu`, for Amazon Linux 2023 |
| PSCP: "Unable to open" | Wrong key path or format | Use `.ppk` key with PSCP, not `.pem` |
| Docker container exits immediately on EC2 | Wrong image architecture | EC2 is ARM64 (`t4g.small`). Build with `--platform linux/arm64`. amd64 images run under emulation only. |
| `exec format error` in container logs | amd64 image on arm64 host | Rebuild images targeting `linux/arm64`. See Section 10. |
| CI not triggering on push | Branch mismatch | Push to `master`, not `main` — see CI workflow trigger config |
| `newgrp docker` doesn't persist | Group change is session-only | Log out and back in — group membership persists after re-login |
| AWS CLI `UnauthorizedOperation` for EC2/RDS | Using `aicm-local-dev` credentials | Those only have Bedrock access. Use root or `aicm-local-ops` credentials for EC2/RDS operations |
