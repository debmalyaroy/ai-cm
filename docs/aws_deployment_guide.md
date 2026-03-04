# AWS Deployment Guide (Free Tier + Bedrock Optimization)

This document provides a step-by-step process for deploying the AI Category Manager to AWS using Free Tier services wherever possible and utilizing Amazon Bedrock for highly optimized, cost-effective LLM performance.

---

## 1. Architectural Overview & Cost Strategy
To keep costs minimal to zero, we leverage AWS Free Tier:
- **Compute:** EC2 `t3.micro` (or `t2.micro` depending on region) - 750 hours/month free for 12 months.
- **Database:** RDS for PostgreSQL `db.t3.micro` or `db.t4g.micro` - 750 hours/month free for 12 months (requires `pgvector` supported engine).
- **LLM (Amazon Bedrock):** Not free tier, but highly cost-controlled using **Agent-Specific Model Routing**:
  - **Analyst (Text-to-SQL + heavy reasoning):** `anthropic.claude-3-5-sonnet-20241022-v2:0` (highly capable, moderate cost).
  - **Planner / Strategist / Liaison:** `anthropic.claude-3-haiku-20240307-v1:0` (extremely fast, very low cost).
- **Container Registry (DockerHub vs AWS ECR):** 
  - AWS ECR Free Tier gives 500MB/month of storage. If your images exceed this, ECR will incur storage costs. 
  - **Cost Optimization:** Use a standard **DockerHub** free account. Build the images via GitHub Actions and push to DockerHub, then pull from DockerHub on your EC2 instance. This avoids ECR costs entirely.

---

## 2. Infrastructure Setup (RDS & EC2)

### Step 2.1: Setup RDS PostgreSQL (Free Tier)
1. Go to **AWS Console > RDS** and click **Create database**.
2. Select **Standard create**, Engine: **PostgreSQL** (version 15 or higher).
3. **Crucial:** Under Templates, select **Free tier**.
4. Set DB instance identifier, Master username (`postgres`), and a strong password.
5. In **Connectivity**, ensure it's in a VPC your EC2 can access, and set **Public access** to **No**.
6. **Security Group:** Create a new security group that allows inbound TCP 5432 from your EC2 instance's security group.
7. Click **Create database**. Once active, note the Endpoint URL.

### Step 2.2: Prepare EC2 Instance (Free Tier)
1. Go to **AWS Console > EC2** and click **Launch Instances**.
2. Select **Amazon Linux 2023 AMI** or **Ubuntu 24.04 LTS**.
3. Select Instance Type: `t3.micro` or `t2.micro` (marked "Free tier eligible").
4. **Key pair (SSH Access):** 
   - Click **Create new key pair**. Name it (e.g., `aicm-key`), choose RSA, format `.pem`.
5. **Network settings:** Allow SSH (port 22) and HTTP (port 80) from anywhere (0.0.0.0/0).
6. **IAM Role:** VERY IMPORTANT. Create an IAM Role attached to this EC2 instance that has `AmazonBedrockFullAccess` (or a restricted policy allowing `bedrock:InvokeModel`).
7. Click **Launch instance**.

---

## 3. Configuration Generation

On your EC2 machine, you will configure `.env.prod` to optimize the dual-LLM constraint strategy to minimize costs:

Create `config/.env.prod`:
```env
# Backend Logger Config
LOG_LEVEL=INFO

# LLM Selection
LLM_PROVIDER=aws

# Production DB Secrets
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_rds_password
POSTGRES_DB=ai_cm
DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@your-rds-endpoint.us-east-1.rds.amazonaws.com:5432/${POSTGRES_DB}?sslmode=require

# AWS Bedrock/ECR Details
AWS_REGION=us-east-1

# Registry
DOCKER_REGISTRY=yourdockerhubusername

# Endpoints
NEXT_PUBLIC_API_URL=http://your-ec2-elastic-ip
```

Ensure `config/config.prod.yaml` is also populated with your model routing routing configuration (Sonnet + Haiku) as detailed in the GitHub repo.

---

## 4. End-to-End Deployment Script

1. SSH into your `t3.micro` instance using the `.pem` key you generated earlier:
```bash
chmod 400 your-key.pem
ssh -i "your-key.pem" ubuntu@<your-ec2-public-ip>
```

2. Run the deployment sequence:
```bash
git clone https://github.com/debmalyaroy/ai-cm.git
cd ai-cm
# Copy the env template and fill in your details
cp config/.env.prod config/.env.prod.active
nano config/.env.prod.active # Add RDS details, DockerHub username

# Run the unified deployment wrapper
chmod +x scripts/deploy_e2e.sh
./scripts/deploy_e2e.sh prod
```

## 5. Exposing the Frontend publicly

When the docker containers are running, the frontend binds to Port `80` on the EC2 instance.

**Free & Secure Public Exposition Options:**
1. **AWS Elastic IP (EIP):**
   - Allocate a free Elastic IP address in the EC2 dashboard and associate it with your EC2 instance. This prevents your IP from changing if the instance reboots. Connect directly via `http://<your-elastic-ip>`.
2. **Cloudflare (Recommended for HTTPS + DDOS Protection):**
   - Cloudflare offers a free tier. Sign up, add your custom domain, and point an `A` record to your Elastic IP. 
   - Enable Cloudflare's "Flexible" or "Full" SSL. You get immediate HTTPS on your public domain with caching and basic security without any AWS ALB costs.
3. **Frontend decoupling (Vercel/Amplify):**
   - Deploy `src/apps/web` to Vercel (free tier) and set `NEXT_PUBLIC_API_URL` to your EC2 backend IP. This provides optimal frontend scaling.

---

## 6. Model Access in AWS Bedrock

By default, Amazon Bedrock models are NOT enabled. You must request access to them:
1. Open the **AWS Console** and search for **Amazon Bedrock**.
2. Click **Manage model access** (top right). Select **Claude 3 Haiku** and **Claude 3.5 Sonnet**. Request access.

### Permissions (Crucial)
Your EC2 instance **must** have an IAM role attached. Without it, the backend Go application will not be able to securely interact with the Bedrock APIs without hardcoded IAM keys in the `.env` file (which is an anti-pattern).
