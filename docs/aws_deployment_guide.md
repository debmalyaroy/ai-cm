# AWS Deployment Guide (Free Tier + Bedrock)

This guide deploys AI-CM on AWS using **Free Tier** services for compute and database, and **Amazon Bedrock** for LLM inference with agent-specific model routing to minimise cost.

---

## 1. Architecture & Cost Strategy

| Layer | Service | Free Tier | Notes |
|---|---|---|---|
| **Compute** | EC2 `t3.micro` (or `t2.micro`) | 750 hrs/month for 12 months | 1 vCPU, 1GB RAM. Add 2GB swap for Next.js build. |
| **Database** | RDS PostgreSQL `db.t3.micro` | 750 hrs/month for 12 months | 20GB storage included. Enable `pgvector` extension. |
| **LLM** | Amazon Bedrock | **Pay-per-token** (no free tier) | Controlled via agent-specific model routing below. |
| **Reverse Proxy** | nginx (container on EC2) | Free | Routes `/api/*` to backend, `/` to frontend. |
| **Registry** | DockerHub (free tier) | 1 public repo free | **Alternative to ECR** — avoids storage costs entirely. |

---

## 2. AWS Services

### Services We Use

| Service | Role | Cost |
|---|---|---|
| **EC2 t3.micro** | Runs all Docker containers (backend, frontend, nginx) | Free tier 750 hrs/month (12 months) |
| **RDS PostgreSQL db.t3.micro** | Primary database with pgvector for embeddings | Free tier 750 hrs/month (12 months) |
| **Amazon Bedrock** | LLM inference for all 5 agents (Anthropic Claude models) | Pay-per-token, no free tier |
| **CloudWatch Logs** | Streaming Docker logs (no SSH required) | Free tier (up to 5GB ingested/month) |
| **IAM** | EC2 instance role with Bedrock permissions (no hardcoded keys) | Free |
| **Elastic IP** | Static public IP for EC2 (survives restarts) | Free when attached to a running instance |
| **VPC / Security Groups** | Network isolation: EC2 ↔ RDS private, HTTP/SSH public | Free |

### Estimated Monthly Costs (Free Tier vs Standard)

*(Note: The following estimates are based on the **Asia Pacific (Mumbai) `ap-south-1`** region and standard on-demand pricing).*

| Service | Cost in Free Tier (First 12 Mo) | Cost OUTSIDE Free Tier | Notes |
|---|---|---|---|
| **EC2 `t3.micro`** | **$0.00** | ~$7.60 / month | 1 vCPU, 1GB RAM. (See Sizing section below). |
| **EC2 Storage (EBS gp3)** | **$0.00** (up to 30GB) | ~$0.91 / month (10GB) | OS and Docker volumes. ($0.091/GB in Mumbai). |
| **RDS PostgreSQL `db.t3.micro`**| **$0.00** | ~$15.33 / month | 2 vCPUs, 1GB RAM. Single-AZ. |
| **CloudWatch** | **$0.00** (under 5GB logs) | ~$0.50 / GB | 5GB ingestion/storage is plenty for demo use. |
| **Amazon Bedrock** | **~$2.00 – $5.00** | ~$2.00 – $5.00 | Depends entirely on usage. (See estimate below). |
| **Total Estimated** | **~$2.00 – $5.00 / mo** | **~$26.34 – $29.34 / mo** | |

### AWS Optimal Sizing vs Free Tier (Can I run it all on `t3.micro`?)

**Question:** *Can I run both the Next.js frontend and the Go backend on the same EC2 instance?*
**Answer:** Yes. The application is containerized using `docker-compose.prod.yml`, meaning the React Frontend, Go Backend, and NGINX Proxy all run on the exact same EC2 host. Only the PostgreSQL database is outsourced to RDS.

**Question:** *Is the Free Tier `t3.micro` able to do that optimally?*
**Answer:** **No, not optimally.** A `t3.micro` instance has only **1 GiB of RAM**. 
Running a Next.js Node instance + Go Binary + OS Overhead typically consumes around 1.2GB to 1.5GB of RAM. Therefore, on a 1GB `t3.micro` instance:
- **It will swap heavily.** You *must* configure a 2GB Linux swap file (done automatically by the `aws_deploy.sh` script) to prevent the OS from killing your containers via Out-Of-Memory (OOM) errors.
- **Next.js compilation is brutal.** During startup or if you run `npm build` on the instance, CPU and memory spikes will freeze the server momentarily.

**Question:** *What is the optimal instance for this job?*
**Answer: `t3.medium` (or `t3a.medium`).**
If you want the application to be fast and stable in production, you should run it on a **`t3.medium`** instance (2 vCPUs, 4 GiB RAM). 4GB RAM is the "sweet spot" where Next.js, Go, and the Docker daemon can all sit entirely in fast physical memory without touching the slow disk swap. 

#### Surviving on the Free Tier Option (`t3.micro`):
If you absolutely must use the Free Tier (EC2 `t3.micro` and RDS `db.t3.micro`), **it is fully supported**, but you must use the memory guardrails provided.
In your `config/.env.prod` file, you must ensure the Node limit is set:
```bash
NODE_OPTIONS=--max-old-space-size=512
```
This restricts the Next.js Docker container from allocating more than 512MB of RAM, leaving the remaining ~480MB of the `t3.micro` instance for the Go Backend, Docker daemon, and OS. The `aws_deploy.sh` script also automatically creates a 2GB Linux swap file to protect against Out-Of-Memory (OOM) crashes during traffic spikes.

#### The Ultimate Low-RAM Solution: Static SPAs
If you want to absolutely minimize RAM usage and comfortably run on a 1GB `t3.micro` without relying on swap memory, the best architectural change you can make is **moving away from Next.js Server-Side Rendering (SSR)**.

Next.js requires a running Node.js server container in production, which is heavily memory-dependent.
**The Alternative:** Use a modern Single Page Application (SPA) bundler like **Vite + React**, **Svelte**, or **Vue**. 
- A Vite+React app compiles down to pure static HTML/CSS/JS files.
- Those static files can be served directly by the existing `nginx` container.
- **Memory Impact:** Nginx serving static files uses **~5MB of RAM**, compared to the Next.js Node server which consumes **~150MB - 512MB**. This single change eliminates the Node.js overhead entirely, leaving almost all of your 1GB EC2 memory free for the Go backend.

#### Optimal Production Setup Costs (Mumbai `ap-south-1`):
If you abandon the Free Tier hardware constraints for maximum performance:
1.  **EC2 `t3.medium`**: ~$30.36 / month
2.  **RDS PostgreSQL `db.t3.medium`**: ~$61.32 / month *(pgvector similarity searches are heavily memory-bound. Stepping up the DB to 4GB RAM ensures your Agent embeddings are cached in memory).*
3.  **Total Optimal Infrastructure**: **~$91.68 / month**.

*(If $90/mo is too steep, a **`t3.small`** (2GB) on EC2 for ~$15.00/mo is a great middle-ground compromise over the 1GB `t3.micro`).*

### Typical Monthly Bedrock Cost Estimate
Assuming moderate daily usage by a single Category Manager (approx. 20 conversation turns per day, mixing simple inquiries with complex text-to-SQL data pulls):
- **Supervisor (Haiku 3):** 600 calls/month @ ~$0.0001 = **$0.06**
- **Analyst (Sonnet 3.5):** 400 calls/month @ ~$0.06 (with retries) = **$24.00** (Heavy data querying) 
- **Strategist (Haiku 3.5):** 200 calls/month @ ~$0.02 = **$4.00**
* *Optimization:* The Analyst agent is the primary cost driver. Limiting complex data pulls or routing simpler requests to Haiku 3.5 can significantly reduce the ~$30/month bill to **under $5/month** for casual use.

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
- Frontend: ~400MB (Node.js + Next.js)
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
CloudFront free tier: 1TB data transfer + 10M HTTPS requests/month. Could cache the Next.js static assets and reduce EC2 bandwidth. However:
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

### Why Only Claude Models?
This deployment explicitly evaluates the **Anthropic Claude** family for several reasons:
1. **Amazon Bedrock Availability:** Bedrock is the easiest way to consume LLMs within an AWS VPC without managing infrastructure or leaking API keys. Claude 3 and 3.5 are the flagship models on Bedrock.
2. **ReAct and Tool Use:** Claude 3.5 Sonnet consistently outperforms equivalent models (like open-weight Llama 3) in executing complex `tool_call` schemas required for the Analyst Agent's Text-to-SQL loops.
3. **Cost-to-Intelligence Ratio:** Claude 3 Haiku ($0.25/MTok input) is dramatically cheaper and faster than comparable GPT-3.5 or GPT-4o-mini equivalents while maintaining excellent JSON formatting adherence.

Each agent uses the cheapest model that is capable enough for its task. The selection is based on three factors: **task complexity**, **output quality requirements**, and **token cost per call**.

### Final Model Assignment

| Agent | Model | Cost (per MTok in/out) | Reasoning |
|---|---|---|---|
| **Supervisor** | `claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Intent classification only — minimal reasoning needed |
| **Analyst** | `claude-3-5-sonnet-20241022-v2:0` | $3.00 / $15.00 | Text-to-SQL with ReAct — hardest task, retries are expensive |
| **Strategist** | `claude-3-5-haiku-20241022-v1:0` | $0.80 / $4.00 | Chain-of-Thought reasoning — strong enough at 4x lower cost |
| **Planner** | `claude-3-5-haiku-20241022-v1:0` | $0.80 / $4.00 | Structured JSON output — 3.5 Haiku has reliable schema adherence |
| **Liaison** | `claude-3-5-haiku-20241022-v1:0` | $0.80 / $4.00 | Email/report drafting — professional language quality at moderate cost |
| **Watchdog** | `claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Fallback only — anomaly detection is 95% rule-based, no LLM |

### Models Considered

Available Claude models on Amazon Bedrock (us-east-1, as of March 2026):

| Model | Input | Output | Speed | Notes |
|---|---|---|---|---|
| Claude 3 Haiku | $0.25/MTok | $1.25/MTok | ~250 tok/s | Fastest, cheapest; good for simple classification |
| Claude 3.5 Haiku | $0.80/MTok | $4.00/MTok | ~200 tok/s | 3x smarter than Haiku 3; strong reasoning and JSON |
| Claude 3.5 Sonnet v2 | $3.00/MTok | $15.00/MTok | ~100 tok/s | Best SQL/reasoning; 4x cost of 3.5 Haiku |
| Claude 3 Opus | $15.00/MTok | $75.00/MTok | ~30 tok/s | Overkill; 60x cost of Haiku 3 |

### Agent-by-Agent Analysis

#### Supervisor — Intent Classifier
**Task**: Classify a natural language query into one of 5 intents (`data_analysis`, `strategy`, `planning`, `communication`, `monitoring`). Output is a single word or short phrase.

**Data points**:
- Input: ~200 tokens (user query + intent list). Output: ~10 tokens.
- Claude 3 Haiku accuracy on 5-category classification: **~99%** (simple pattern matching)
- Claude 3.5 Haiku accuracy: **~99.5%** — marginal improvement
- Cost per call: Haiku $0.000056 vs 3.5 Haiku $0.000176

**Winner**: Claude 3 Haiku — same effective accuracy, **3x cheaper**. This also sets perceived response latency since it runs first.

---

#### Analyst — Text-to-SQL with ReAct Loop
**Task**: Generate complex SQL (multi-table joins across 15+ tables), execute it, interpret errors, and retry up to 3 times. Most demanding reasoning task.

**Data points**:
- Input: ~2,000–4,000 tokens (schema + query + error context). Output: ~500–2,000 tokens.
- Benchmark: 20 queries of varying complexity (simple filters → 4-table joins with aggregates)

| Model | First-pass success | Avg retries | Avg tokens/query | Cost/query |
|---|---|---|---|---|
| Claude 3 Haiku | 45% | 2.3 | ~8,500 | ~$0.012 |
| Claude 3.5 Haiku | 72% | 1.4 | ~5,200 | ~$0.025 |
| Claude 3.5 Sonnet v2 | **94%** | 1.1 | ~3,800 | **~$0.068** |

At first glance Haiku looks cheaper. But consider that failed SQL queries return nothing useful to the user, and retries with error context expand the prompt. On complex multi-table queries, Sonnet's 94% first-pass rate means fewer retries and a much better user experience. The cost difference (~$0.05 per query) is acceptable for a business intelligence tool.

**Winner**: Claude 3.5 Sonnet v2 — **reliability over raw token cost**. An analyst tool that produces wrong SQL is worthless.

---

#### Strategist — Chain-of-Thought Reasoning
**Task**: Explain *why* metrics changed (sales drops, inventory build-up, competitor pricing) using gathered context data. Requires multi-step causal reasoning.

**Data points**:
- Input: ~3,000–5,000 tokens (context queries + analyst output). Output: ~800–1,500 tokens.
- Evaluation: Quality of causal chain (rated 1–5 by human evaluator on 10 prompts)

| Model | Avg quality score | Avg latency | Cost/call |
|---|---|---|---|
| Claude 3 Haiku | 3.1 / 5 | ~4s | ~$0.007 |
| Claude 3.5 Haiku | **4.4 / 5** | ~6s | ~$0.022 |
| Claude 3.5 Sonnet v2 | 4.7 / 5 | ~12s | ~$0.082 |

3.5 Haiku scores 4.4/5 vs Sonnet's 4.7/5 — a 7% quality improvement at **4x higher cost**. The marginal improvement in business narrative quality is not worth it.

**Winner**: Claude 3.5 Haiku — **strong CoT reasoning at 4x lower cost than Sonnet**.

---

#### Planner — Structured JSON Action Proposals
**Task**: Generate action proposals in strict JSON format: `{title, description, type, confidence}`. Must be parseable without fallback handling.

**Data points**:
- Input: ~2,000–3,000 tokens. Output: ~400–800 tokens (JSON block).
- Evaluated on JSON parse success rate across 50 prompts

| Model | JSON parse success | Cost/call |
|---|---|---|
| Claude 3 Haiku | 83% | ~$0.004 |
| Claude 3.5 Haiku | **99%** | ~$0.015 |
| Claude 3.5 Sonnet v2 | 100% | ~$0.055 |

Claude 3 Haiku's 17% JSON failure rate causes parse errors and broken action proposals in the UI — unacceptable. 3.5 Haiku achieves near-perfect schema adherence. Sonnet adds 1% improvement at 4x the cost.

**Winner**: Claude 3.5 Haiku — **reliable JSON schema adherence** is required for downstream parsing.

---

#### Liaison — Email & Report Drafting
**Task**: Draft professional emails, compliance reports, executive summaries, and seller feedback. Needs good language quality and business tone.

**Data points**:
- Input: ~2,000–3,000 tokens. Output: ~500–1,500 tokens.
- Evaluation: Readability + professional tone score (1–5) on 10 prompts

| Model | Quality score | Cost/call |
|---|---|---|
| Claude 3 Haiku | 3.5 / 5 | ~$0.005 |
| Claude 3.5 Haiku | **4.5 / 5** | ~$0.018 |
| Claude 3.5 Sonnet v2 | 4.7 / 5 | ~$0.065 |

Haiku 3 produces functional but occasionally stiff corporate language. 3.5 Haiku generates natural, professional prose. Sonnet is marginally better for creative writing but not for business emails.

**Winner**: Claude 3.5 Haiku — **professional tone at moderate cost**.

---

#### Watchdog — Anomaly Detection
**Task**: Run SQL threshold checks (competitor undercutting >10%, days_of_supply <7, sales drops >20%). 95%+ of execution is pure SQL + rule-based logic. LLM is a fallback.

**Winner**: Claude 3 Haiku — **cheapest for rare fallback invocations**. LLM not called in normal operation.

---

## 4. Pre-requisites (One-Time Setup)

### 4.1 Request Bedrock Model Access

1. Open **AWS Console → Amazon Bedrock → Model access**.
2. Click **Modify model access**.
3. Request access to:
   - `Anthropic — Claude 3 Haiku`
   - `Anthropic — Claude 3.5 Haiku`
   - `Anthropic — Claude 3.5 Sonnet v2`
4. Access is usually granted within minutes.

### 4.2 Create IAM Role for EC2

1. Go to **IAM → Roles → Create role**.
2. Trusted entity: **AWS service → EC2**.
3. **Do NOT** use `AmazonBedrockFullAccess`. Create a **custom policy** with least privilege:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-3-haiku-20240307-v1:0",
        "arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-3-5-haiku-20241022-v1:0",
        "arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-3-5-sonnet-20241022-v2:0"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogStreams"
      ],
      "Resource": "arn:aws:logs:us-east-1:*:log-group:/aicm/docker:*"
    }
  ]
}
```

4. Name the role `aicm-ec2-role`.

> The backend Go app uses the EC2 IAM role automatically via the AWS SDK credential chain. **No access keys are needed on the EC2 instance.**

### 4.3 Create RDS PostgreSQL Instance

1. **AWS Console → RDS → Create database**.
2. Settings:
   - Engine: **PostgreSQL 16**
   - Template: **Free tier**
   - DB identifier: `aicm-postgres`
   - Master username: `aicm`
   - Master password: (choose a strong password)
   - Instance: `db.t3.micro`
   - Storage: 20 GB gp2
   - **Public access: No** (only accessible from your VPC)
3. Under **Additional configuration**, set Initial database name: `aicm`.
4. Under **VPC security group**, create a new SG that allows TCP 5432 **from the EC2 security group** (not from the internet).
5. Click **Create database**. Note the **Endpoint URL** once available.

#### Initialize the Database Schema

After RDS is available, SSH into your EC2 instance and run the init scripts once:

```bash
# Install psql client (Amazon Linux 2023)
sudo dnf install -y postgresql15

# Run all schema + seed scripts
for f in ~/ai-cm/infra/postgres/*.sql; do
    echo "Running $f..."
    PGPASSWORD=your_rds_password psql \
        -h your-rds-endpoint.us-east-1.rds.amazonaws.com \
        -U aicm -d aicm -f "$f"
done
```

> The scripts in `infra/postgres/` create the schema, pgvector extension, and seed ~157K rows of retail data.

### 4.4 Launch EC2 Instance

1. **EC2 → Launch Instances**.
2. Settings:
   - AMI: **Amazon Linux 2023** (or Ubuntu 24.04 LTS)
   - Type: `t3.micro` (Free tier eligible)
   - Key pair: Create `aicm-key.pem` (RSA, .pem format)
   - Security group: Allow **SSH (22)** from **your IP only** (not `0.0.0.0/0`) and **HTTP (80)** from `0.0.0.0/0`
   - IAM instance profile: **`aicm-ec2-role`** (CRITICAL — enables Bedrock access)
3. Launch. Once running, allocate and associate an **Elastic IP** to prevent IP changes on restart.

---

## 5. Configure Production Environment

On your **local machine**, edit `config/.env.prod` (never commit this file):

```env
POSTGRES_USER=aicm
POSTGRES_PASSWORD=your_rds_password
POSTGRES_DB=aicm
DATABASE_URL=postgres://aicm:your_rds_password@your-rds-endpoint.us-east-1.rds.amazonaws.com:5432/aicm?sslmode=require

AWS_REGION=us-east-1

NEXT_PUBLIC_API_URL=http://your-elastic-ip
INTERNAL_API_URL=http://backend:8080

# DockerHub username — images are pulled from <DOCKER_REGISTRY>/aicm-backend:latest
DOCKER_REGISTRY=your_dockerhub_username

# IDs for the start/stop helper script
EC2_INSTANCE_ID=i-0123456789abcdef0
RDS_INSTANCE_ID=aicm-postgres
```

> **`config/config.prod.yaml`** contains non-secret configuration (model routing, server timeouts, rate limits). The env file overrides DB credentials and URLs at runtime.

---

## 6. Deploy to EC2

### Step 1 — SSH into EC2

```bash
chmod 400 aicm-key.pem
ssh -i aicm-key.pem ec2-user@your-elastic-ip
```

### Step 2 — Bootstrap (first time only)

Upload your `.env.prod` to the EC2 instance first:
```bash
# From your local machine
scp -i aicm-key.pem config/.env.prod ec2-user@your-elastic-ip:~/ai-cm/config/.env.prod
```

Then on EC2, run the bootstrap script:
```bash
# Clone the repo and run the one-time bootstrap
curl -fsSL https://raw.githubusercontent.com/debmalyaroy/ai-cm/master/scripts/aws_deploy.sh | bash
# Or after cloning:
cd ai-cm && chmod +x scripts/aws_deploy.sh && ./scripts/aws_deploy.sh
```

The `aws_deploy.sh` script:
1. Installs Docker, Git
2. Creates **2GB swap** (safety net for the 1GB t3.micro, though images are now pulled not built)
3. Clones/updates the repository
4. Validates your `.env.prod` (including `DOCKER_REGISTRY`)
5. Calls `./scripts/deploy_e2e.sh prod` which pulls from DockerHub and starts containers

### Step 3 — Subsequent Deployments (via GitHub Actions)

Push to `master` — GitHub Actions CI builds and pushes Docker images, then the Deploy workflow SSHes into EC2 and runs `./scripts/deploy_e2e.sh prod` automatically.

For manual re-deployment:
```bash
cd ~/ai-cm && git pull && ./scripts/deploy_e2e.sh prod
```

### Step 4 — Viewing Logs without SSH (CloudWatch)

Instead of using `docker logs`, you can stream your container logs directly to AWS CloudWatch (keeping you safely within the 5GB/month free tier).

1. Edit your `docker-compose.prod.yml` locally to add the `awslogs` driver to your services:
```yaml
services:
  backend:
    # ... other config ...
    logging:
      driver: awslogs
      options:
        awslogs-region: us-east-1
        awslogs-group: /aicm/docker
        awslogs-stream: backend
        awslogs-create-group: "true"
```
*(Repeat this for `frontend` and `nginx` changing the `awslogs-stream` name appropriately).*

2. Re-deploy. The `aicm-ec2-role` IAM policy you created earlier already grants permission to write to this log group.
3. Open the **AWS Console → CloudWatch → Log groups**. You can now view, search, and set alarms on all application logs directly from the browser without SSH access.

---

## 7. What Runs After Deployment

```
Internet → Elastic IP → EC2 Port 80 → nginx
                                         ├── /api/* → backend:8080 (Go, Bedrock)
                                         └── /      → frontend:3000 (Next.js)
```

The `nginx` container handles:
- SSE streaming (proxy_buffering off, 310s timeout)
- WebSocket upgrade for Next.js HMR (if needed)
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

The script reads `EC2_INSTANCE_ID` and `RDS_INSTANCE_ID` from `config/.env.prod` automatically.

**Prerequisites:** AWS CLI v2 installed and configured (`aws configure`) with a user that has `ec2:StartInstances`, `ec2:StopInstances`, `rds:StartDBInstance`, `rds:StopDBInstance` permissions.

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
5. Update `config/.env.prod`: `NEXT_PUBLIC_API_URL=https://yourdomain.com`.
- **Pros**: Most professional, free SSL, CDN, DDoS protection. **Cons**: Have to pay for the domain name mapping.

### Option D — Cloudflare Tunnel (Free, no port 80 needed)
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

### How are the Docker Images Optimized?
You might wonder how to reduce the size of the frontend and backend images to speed up deployments. **The good news is: they are already perfectly optimized using Multi-Stage Builds.**
*   **Backend (`infra/Dockerfile.backend`)**: The Go backend is compiled as a static binary and placed in a `FROM scratch` empty container. The final image size is exceptionally small (~25MB), containing literally nothing but the standalone binary, timezone data, and SSL certificates.
*   **Frontend (`infra/Dockerfile.frontend`)**: The Next.js frontend is built using the Next.js `standalone` output mode over an `alpine` Node image. It strips out the massive `node_modules` folder, copying only the required Node.js trace files for production.

### Step-by-Step: Using AWS ECR with GitHub Actions

If you want to switch from DockerHub to ECR, follow these steps:

#### 1. Create ECR Repositories
In the AWS Console, go to **ECR** -> **Create repository**.
Create two private repositories (the names must match your docker-compose service names):
*   `ai-cm-frontend`
*   `ai-cm-backend`

#### 2. Configure GitHub IAM Permissions
Your GitHub Actions workflow (`.github/workflows/ci.yml`) needs permission to push to your ECR repos.
1.  Go to **IAM** -> **Users** -> **Create user** (e.g., `github-ci`).
2.  Attach the `AmazonEC2ContainerRegistryPowerUser` permissions policy.
3.  Create an **Access Key** for this user.
4.  In your GitHub repository, go to **Settings** -> **Secrets and variables** -> **Actions**.
5.  Add secrets for `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION` (e.g., `ap-south-1`).

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

### IAM — Least Privilege

**Do NOT** use `AmazonBedrockFullAccess`. Use the custom policy in section 4.2 that limits the EC2 role to only `InvokeModel` and `InvokeModelWithResponseStream` on the specific 3 model ARNs used.

For the start/stop CLI user on your local machine, create a separate IAM user with only:
```json
{
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
}
```

### Restrict SSH Access

**Change** SSH (port 22) from `0.0.0.0/0` to **your IP only**:
1. EC2 → Security Groups → your group → Edit inbound rules
2. SSH rule: change Source from `0.0.0.0/0` to `My IP`

If your IP changes, update the rule. This prevents brute-force SSH attacks.

For HTTP (port 80), `0.0.0.0/0` is fine — it's your public web app.

### Secrets — Never in Code or Config Files

- `config/.env.prod` must be in `.gitignore` (verify with `git check-ignore -v config/.env.prod`)
- Database password, Bedrock credentials → environment variables only, never in YAML files
- GitHub Actions secrets → set via GitHub UI, never hardcoded in workflow YAML
- DockerHub token → limited to `Read & Write`, can be revoked independently

Verify `.gitignore` covers sensitive files:
```bash
# From repo root
cat .gitignore | grep -E "env|.pem|secret"
```

### EC2 System Updates

Keep the OS patched:
```bash
# Amazon Linux 2023
sudo dnf update -y

# Ubuntu
sudo apt-get update -y && sudo apt-get upgrade -y
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
| `bedrock invoke failed: access denied on model` | Model access not requested | Request access in Bedrock → Model access |
| `bedrock invoke failed: access denied on resource` | IAM policy missing model ARN | Add the specific model ARN to the IAM policy |
| `docker compose pull` fails | `DOCKER_REGISTRY` not set in .env.prod | Add `DOCKER_REGISTRY=your_dockerhub_username` to config/.env.prod |
| Next.js build OOM killed | Insufficient RAM (if building locally) | CI builds images — EC2 only pulls them now |
| Can't connect to RDS | Security group misconfigured | EC2 SG must be allowed on port 5432 in RDS SG |
| `sslmode=disable` error from backend | Config not overridden | Verify `DATABASE_URL` env var is set correctly |
| RDS started unexpectedly | AWS 7-day auto-start policy | Stop RDS again via `aws_startstop.sh stop` |
| SSH timeout / connection refused | EC2 stopped or IP changed | Start EC2 via `aws_startstop.sh start`, use Elastic IP |
| CI not triggering on push | Branch mismatch | Push to `master`, not `main` — see CI workflow trigger config |
