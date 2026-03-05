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

### Bedrock Model Routing (Optimised for Cost)

Each agent uses the cheapest model that is capable of its task:

| Agent | Task | Model | Cost (per MTok in/out) |
|---|---|---|---|
| **Supervisor** | Intent classification (one word) | `claude-3-haiku-20240307-v1:0` | $0.25 / $1.25 |
| **Analyst** | Text-to-SQL + ReAct loop (3 retries) | `claude-3-5-sonnet-20241022-v2:0` | $3.00 / $15.00 |
| **Strategist** | Chain-of-Thought "why" explanations | `claude-3-5-haiku-20241022-v1:0` | $0.80 / $4.00 |
| **Planner** | Structured JSON action proposals | `claude-3-5-haiku-20241022-v1:0` | $0.80 / $4.00 |
| **Liaison** | Email / compliance report drafting | `claude-3-5-haiku-20241022-v1:0` | $0.80 / $4.00 |
| **Watchdog** | Rule-based (no LLM calls) | `claude-3-haiku-20240307-v1:0` (fallback) | N/A |

**Analyst uses Sonnet** because Text-to-SQL is the hardest task (requires understanding a multi-table retail schema and self-correcting SQL errors). All other agents are on Haiku variants.

---

## 2. Pre-requisites (One-Time Setup)

### 2.1 Request Bedrock Model Access

1. Open **AWS Console → Amazon Bedrock → Model access**.
2. Click **Modify model access**.
3. Request access to:
   - `Anthropic — Claude 3 Haiku`
   - `Anthropic — Claude 3.5 Haiku`
   - `Anthropic — Claude 3.5 Sonnet v2`
4. Access is usually granted within minutes.

### 2.2 Create IAM Role for EC2

1. Go to **IAM → Roles → Create role**.
2. Trusted entity: **AWS service → EC2**.
3. Attach policy: **AmazonBedrockFullAccess** (or a custom policy with `bedrock:InvokeModel` + `bedrock:InvokeModelWithResponseStream` only).
4. Name the role `aicm-ec2-role`.

> The backend Go app uses the EC2 IAM role automatically via the AWS SDK credential chain. **No access keys are needed on the EC2 instance.**

### 2.3 Create RDS PostgreSQL Instance

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

### 2.4 Launch EC2 Instance

1. **EC2 → Launch Instances**.
2. Settings:
   - AMI: **Amazon Linux 2023** (or Ubuntu 24.04 LTS)
   - Type: `t3.micro` (Free tier eligible)
   - Key pair: Create `aicm-key.pem` (RSA, .pem format)
   - Security group: Allow **SSH (22)** and **HTTP (80)** from `0.0.0.0/0`
   - IAM instance profile: **`aicm-ec2-role`** (CRITICAL — enables Bedrock access)
3. Launch. Once running, allocate and associate an **Elastic IP** to prevent IP changes on restart.

---

## 3. Configure Production Environment

On your **local machine**, edit `config/.env.prod` (never commit this file):

```env
POSTGRES_USER=aicm
POSTGRES_PASSWORD=your_rds_password
POSTGRES_DB=aicm
DATABASE_URL=postgres://aicm:your_rds_password@your-rds-endpoint.us-east-1.rds.amazonaws.com:5432/aicm?sslmode=require

AWS_REGION=us-east-1

NEXT_PUBLIC_API_URL=http://your-elastic-ip
INTERNAL_API_URL=http://backend:8080

# IDs for the start/stop helper script
EC2_INSTANCE_ID=i-0123456789abcdef0
RDS_INSTANCE_ID=aicm-postgres
```

> **`config/config.prod.yaml`** contains non-secret configuration (model routing, server timeouts, rate limits). The env file overrides DB credentials and URLs at runtime.

---

## 4. Deploy to EC2

### Step 1 — SSH into EC2

```bash
chmod 400 aicm-key.pem
ssh -i aicm-key.pem ec2-user@your-elastic-ip
```

### Step 2 — Bootstrap (first time only)

Upload your `.env.prod` to the EC2 instance first:
```bash
# From your local machine
scp -i aicm-key.pem config/.env.prod ec2-user@your-elastic-ip:~/ai-cm-env.prod
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
2. Creates **2GB swap** (required for Next.js build on 1GB RAM t3.micro)
3. Clones/updates the repository
4. Validates your `.env.prod`
5. Calls `./scripts/deploy_e2e.sh prod` to build and start all containers

### Step 3 — Subsequent Deployments

For code updates, just re-run the deployment wrapper:
```bash
cd ~/ai-cm && git pull && ./scripts/deploy_e2e.sh prod
```

---

## 5. What Runs After Deployment

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

## 6. Free Tier — Staying Within Limits

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

## 7. HTTPS / Custom Domain (Optional)

For HTTPS without an AWS ALB (which is not free tier), use **Cloudflare**:

1. Register a free Cloudflare account at cloudflare.com.
2. Add your domain, create an `A` record pointing to your **Elastic IP**.
3. Enable **Flexible SSL** in Cloudflare (traffic between user ↔ Cloudflare is HTTPS; Cloudflare ↔ EC2 is HTTP).
4. Update `config/.env.prod`: set `NEXT_PUBLIC_API_URL=https://yourdomain.com`
5. Update `config/config.prod.yaml` CORS: add `https://yourdomain.com` to `allow_origins`.
6. Redeploy: `./scripts/deploy_e2e.sh prod`

> For true end-to-end HTTPS, use Cloudflare **Full (strict)** mode with a Let's Encrypt certificate installed in nginx.

---

## 8. Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| `bedrock invoke failed: no credentials` | EC2 IAM role not attached | Attach `aicm-ec2-role` to EC2 instance in console |
| `bedrock invoke failed: access denied on model` | Model access not requested | Request access in Bedrock → Model access |
| Next.js build OOM killed | Insufficient RAM | Verify 2GB swap exists: `free -h` |
| Can't connect to RDS | Security group misconfigured | EC2 SG must be allowed on port 5432 in RDS SG |
| `sslmode=disable` error from backend | Config not overridden | Verify `DATABASE_URL` env var is set correctly |
| RDS started unexpectedly | AWS 7-day auto-start policy | Stop RDS again via `aws_startstop.sh stop` |
