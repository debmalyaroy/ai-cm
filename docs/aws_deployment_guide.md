# AWS Deployment Guide (Free Tier + Bedrock)

This guide deploys AI-CM on AWS using **Free Tier** services for compute and database, and **Amazon Bedrock** for LLM inference with agent-specific model routing to minimise cost.

---

## 1. Architecture & Cost Strategy

| Layer | Service | Free Tier | Notes |
|---|---|---|---|
| **Compute** | EC2 `t3.micro` (or `t2.micro`) | 750 hrs/month for 12 months | 1 vCPU, 1GB RAM. Add 2GB swap for Vite/React build. |
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

#### Optimal Production Setup Costs (Mumbai `ap-south-1`):
If you abandon the Free Tier hardware constraints for maximum performance:
1.  **EC2 `t3.medium`**: ~$30.36 / month
2.  **RDS PostgreSQL `db.t3.medium`**: ~$61.32 / month *(pgvector similarity searches are heavily memory-bound. Stepping up the DB to 4GB RAM ensures your Agent embeddings are cached in memory).*
3.  **Total Optimal Infrastructure**: **~$91.68 / month**.

*(If $90/mo is too steep, a **`t3.small`** (2GB) on EC2 for ~$15.00/mo is a great middle-ground compromise over the 1GB `t3.micro`).*

### Typical Monthly Bedrock Cost Estimate
Assuming moderate daily usage by a single Category Manager (approx. 20 conversation turns per day, mixing simple inquiries with complex text-to-SQL data pulls):
- **Supervisor (Haiku 3):** 600 calls/month @ ~$0.0001 = **$0.06**
- **Analyst (Llama 3.1 70B / Qwen):** 400 calls/month @ ~$0.003 = **$1.20** (Heavy data querying) 
- **Strategist (Haiku 3):** 200 calls/month @ ~$0.004 = **$0.80**
* *Optimization:* The Analyst agent was previously using Claude 3.5 Sonnet ($24.00/month), which is the primary cost driver. By swapping to Llama 3.1 70B Instruct or Qwen models, we reduce the total LLM cost to **under $3/month**.
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

### Why Only Claude Models?
This deployment exclusively uses the **Anthropic Claude** family on Amazon Bedrock for the following reasons:
1. **Amazon Bedrock Availability:** Bedrock provides IAM-authenticated LLM access within an AWS VPC — no API keys, no external egress. Claude is the flagship model family on Bedrock with the broadest APAC region support.
2. **ReAct and Tool Use:** Claude Sonnet 4 sets the standard for complex `tool_call` schemas and Text-to-SQL reasoning, outperforming open-weight alternatives on multi-table join tasks.
3. **Cost-to-Intelligence Ratio:** The Claude family spans from Claude 3 Haiku ($0.25/MTok) to Haiku 4.5 ($1.00/MTok) and Sonnet 4 ($3.00/MTok) — covering every cost-quality tradeoff needed across the 5 agents.
4. **APAC Inference Profiles:** Mumbai (ap-south-1) supports `apac.` cross-region routing profiles, keeping inference latency within the Asia Pacific region and satisfying data residency expectations.

Each agent uses the cheapest model that is capable enough for its task. The selection is based on three factors: **task complexity**, **output quality requirements**, and **token cost per call**.

### Final Model Assignment

| Agent | Model | Inference Profile ID | Cost (per MTok in/out) | Reasoning |
|---|---|---|---|---|
| **Supervisor** | Claude 3 Haiku | `apac.anthropic.claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Intent classification only — 99% accuracy |
| **Analyst** | **Llama 3.1 70B / Qwen** | `meta.llama3-1-70b-instruct-v1:0` / `qwen` | ~$0.72 / ~$0.72 | Text-to-SQL with ReAct — extremely cost-effective |
| **Strategist** | **Claude 3 Haiku** | `apac.anthropic.claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Chain-of-Thought at low cost |
| **Planner** | **Claude 3 Haiku** | `apac.anthropic.claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Structured JSON output — strong schema adherence |
| **Liaison** | **Claude 3 Haiku** | `apac.anthropic.claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Email/report drafting at low cost |
| **Watchdog** | Claude 3 Haiku | `apac.anthropic.claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 | Fallback only — 95% rule-based |

### Models Considered

Claude models evaluated for on-demand inference via **ap-south-1 (Mumbai)** APAC and global inference profiles (March 2026):

| Model | Profile / ID | Input | Output | Speed | Notes |
|---|---|---|---|---|---|
| Claude 3 Haiku | `apac.` profile | $0.25/MTok | $1.25/MTok | ~250 tok/s | Cheapest Anthropic model; great for high volume |
| Llama 3.1 70B | `meta.llama3-1-` | ~$0.72/MTok | ~$0.72/MTok | ~120 tok/s | Top tier SQL generator for a fraction of Claude's cost |
| Qwen models | `qwen` / custom | ~$0.40/MTok | ~$1.20/MTok | ~150 tok/s | Qwen3 and Qwen2.5-Coder are highly capable and available in ap-south-1 |
| Claude 3.5 Sonnet | `apac.` profile | $3.00/MTok | $15.00/MTok | ~80 tok/s | Too costly for free-tier/low-budget deployments |

> **Why not Claude 3.5 Haiku or 3.5 Sonnet v2?** Claude 3.7 Sonnet is deprecated and its successor (Sonnet 4) is available at the same price. Claude 3.5 Haiku is superseded by Haiku 4.5 (better quality, marginal cost increase). Neither 3.5 Haiku nor 3.5 Haiku has an `apac.` inference profile for ap-south-2 (Hyderabad) — another reason Mumbai with its broader APAC catalogue is preferred.
>
> **Why Mumbai over Hyderabad (ap-south-2)?** Hyderabad lacks an `apac.` Claude 3 Haiku inference profile, forcing Supervisor and Watchdog onto the more expensive `global.claude-haiku-4-5` ($1/MTok vs $0.25/MTok) — a 4x cost increase for the two simplest agents. Mumbai supports all APAC Claude profiles including Haiku 3.

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

**Winner**: Meta Llama 3.1 70B or Qwen Coder models — **cost-effectiveness while retaining excellent SQL generation capabilities**. Claude 3.5 Sonnet is arguably better, but at 5-10x the cost of Llama/Qwen.

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

### 4.1 Amazon Bedrock Access via IAM (No explicit opt-in required)

Amazon Bedrock models (such as Anthropic Claude) are now accessible directly through IAM policy authorization. You no longer need to explicitly "Enable specific models" in the AWS Console. 

Provide your EC2 instance an IAM Role with the appropriate Bedrock invocation permissions, and the Go Backend will automatically assume that role to execute LLM API calls.

### 4.2 Create IAM Role for EC2

#### Step 1: Create the Custom IAM Policy

1. Go to **IAM** → in the left sidebar, click **Policies** → **Create policy**.
2. Select the **JSON** tab and replace the default content with:

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
        "arn:aws:bedrock:*::foundation-model/*",
        "arn:aws:bedrock:*:*:inference-profile/*"
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
      "Resource": "arn:aws:logs:us-east-1:*:log-group:/aicm/docker:*"
    }
  ]
}
```

> **Do NOT** use `AmazonBedrockFullAccess` — it grants access to every model and every Bedrock action. The policy above restricts access to only the 3 model ARNs this app uses.

3. Click **Next**, enter the policy name `aicm-ec2-policy`, and click **Create policy**.

#### Step 2: Create the IAM Role

1. In the IAM left sidebar, click **Roles** → **Create role**.
2. Under "Trusted entity type", select **AWS service**.
3. Under "Use case", select **EC2**. Click **Next**.
4. In the permissions search box, search for `aicm-ec2-policy` and tick the checkbox next to it.
5. Click **Next**, enter the role name `aicm-ec2-role`, and click **Create role**.

> The backend Go app uses the EC2 IAM role automatically via the AWS SDK credential chain. **No access keys are needed on the EC2 instance.**

### 4.3 Create RDS PostgreSQL Instance

#### Step 1: Create the Database

1. In the AWS Console, go to **RDS** → **Create database**.
2. Select **Standard Create**.
3. Under "Engine options", select **PostgreSQL** and choose version **16**.
4. Under "Templates", select **Free tier**.
5. Under "Settings":
   - **DB instance identifier**: `aicm-postgres`
   - **Master username**: `aicm`
   - **Master password**: choose a strong password and note it down
6. Under "Instance configuration", confirm **db.t3.micro**.
7. Under "Storage", confirm **20 GiB gp2**.
8. Under "Connectivity":
   - **Public access**: **No** (only reachable from inside your VPC)
   - **VPC security group**: Create new, name it `aicm-rds-sg`
9. Under "Additional configuration":
   - **Initial database name**: `aicm`
10. Click **Create database**.

#### Step 2: Configure the RDS Security Group

Once the database is being created, update the security group to allow only EC2 access:

1. Go to **EC2 → Security Groups** and open `aicm-rds-sg`.
2. Click **Edit inbound rules** → **Add rule**:
   - Type: **PostgreSQL** (port 5432)
   - Source: select your EC2 instance's security group (type `aicm-ec2` in the source search box)
3. Click **Save rules**.

> Once RDS status shows **Available**, click on the instance and copy the **Endpoint** URL — you will need it for the `DATABASE_URL` in your config.

#### Step 3: Initialize the Database Schema

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

#### Step 1: Launch the Instance

1. In the AWS Console, go to **EC2** → **Launch instances**.
2. **Name**: `aicm-server`.
3. **AMI**: Select **Amazon Linux 2023 AMI** (Free tier eligible, 64-bit x86).
4. **Instance type**: `t3.micro` (Free tier eligible).
5. **Key pair**: Click **Create new key pair**:
   - Name: `aicm-key`
   - Type: RSA
   - Format: **.pem** (for Linux/Mac) or **.ppk** for PuTTY on Windows
   - Click **Create key pair** — the `aicm-key.pem` file downloads automatically. **Keep this file safe.**
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

---

## 5. Configure Production Environment

On your **local machine**, edit the root `.env` file (never commit this file):

```env
[prod.aws]
POSTGRES_USER=aicm
POSTGRES_PASSWORD=your_rds_password
POSTGRES_DB=aicm
DATABASE_URL=postgres://aicm:your_rds_password@your-rds-endpoint.us-east-1.rds.amazonaws.com:5432/aicm?sslmode=require

AWS_REGION=us-east-1

VITE_API_URL=http://your-elastic-ip
INTERNAL_API_URL=http://backend:8080

# DockerHub credentials for CI or manual pushes
DOCKER_REGISTRY=your_dockerhub_username
DOCKER_USERNAME=your_dockerhub_username
DOCKER_PAT=YOUR_DOCKERHUB_PAT

# IDs for the start/stop helper script
EC2_INSTANCE_ID=i-0123456789abcdef0
RDS_INSTANCE_ID=aicm-postgres
```

> **`config/config.prod.yaml`** contains non-secret configuration (model routing, server timeouts, rate limits). The root `.env` file's `[prod.aws]` section overrides DB credentials and URLs at runtime via `scripts/deploy.sh` extraction.

---

## 6. Deploy to EC2

### Step 1 — SSH into EC2

```bash
chmod 400 aicm-key.pem
ssh -i aicm-key.pem ec2-user@your-elastic-ip
```

### Step 2 — Bootstrap (first time only)

Upload your `.env` to the EC2 instance first:
```bash
# From your local machine
scp -i aicm-key.pem .env ec2-user@your-elastic-ip:~/ai-cm/.env
```

Then on EC2, run the deployment script:
```bash
# Clone the repo
git clone https://github.com/debmalyaroy/ai-cm.git
cd ai-cm

# Execute the unified deploy script which leverages the local .env file
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

The `deploy.sh` script:
1. Validates your `.env` (including `DOCKER_REGISTRY` and secrets)
2. Extracts `[prod.aws]` settings implicitly
3. Pulls from DockerHub and starts the containers with `docker compose`

### Step 3 — Subsequent Deployments (via GitHub Actions)

Push to `master` — GitHub Actions CI builds and pushes Docker images, then the Deploy workflow SSHes into EC2 and runs `./scripts/deploy.sh` automatically.

For manual re-deployment:
```bash
cd ~/ai-cm && git pull && ./scripts/deploy.sh
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
                                         └── /      → frontend:3000 (Vite/React)
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

The script reads `EC2_INSTANCE_ID` and `RDS_INSTANCE_ID` from the `[prod.aws]` section of the root `.env` file automatically.

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
5. Update `config/.env.prod`: `VITE_API_URL=https://yourdomain.com`.
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

### IAM — Least Privilege

**Do NOT** use `AmazonBedrockFullAccess`. Use the custom policy in section 4.2 that limits the EC2 role to only `InvokeModel` and `InvokeModelWithResponseStream` for the foundation models and inference profiles.

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
4. On your local machine, run `aws configure` and enter the copied values:
   ```bash
   aws configure
   # AWS Access Key ID: AKIAxxxxxxxxxxxxx
   # AWS Secret Access Key: xxxxxxxxxxxxxxxx
   # Default region: us-east-1
   # Default output format: json
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
| Vite/React build OOM killed | Insufficient RAM (if building locally) | CI builds images — EC2 only pulls them now |
| Can't connect to RDS | Security group misconfigured | EC2 SG must be allowed on port 5432 in RDS SG |
| `sslmode=disable` error from backend | Config not overridden | Verify `DATABASE_URL` env var is set correctly |
| RDS started unexpectedly | AWS 7-day auto-start policy | Stop RDS again via `aws_startstop.sh stop` |
| SSH timeout / connection refused | EC2 stopped or IP changed | Start EC2 via `aws_startstop.sh start`, use Elastic IP |
| CI not triggering on push | Branch mismatch | Push to `master`, not `main` — see CI workflow trigger config |
