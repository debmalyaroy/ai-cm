# How to Run Frontend and Backend Locally

This guide provides instructions on how to run the AI-CM frontend and backend services locally on your machine.

All commands are written for **Windows PowerShell** but equivalent `.sh` files exist. Run everything from the project root unless stated otherwise.

## Prerequisites

- **Docker Desktop** (with WSL2 backend for GPU support on Windows)
- **Git**
- Optional: Native **Ollama** installed if skipping the dockerized LLM ([https://ollama.com/download](https://ollama.com/download))

---

## 1. Environment Profiles & The `.env` File

At the root of the project, copy the `.env.example` file to create a `.env` file:
```powershell
Copy-Item .env.example .env
notepad .env
```

The file contains three sections:
- `[local.local]` - Config for using Dockerized Ollama locally on your GPU.
- `[local.aws]` - Config for running the UI/DB locally but routing LLMs to AWS Bedrock (saves local RAM/GPU).
- `[prod.aws]` - Config for EC2 + RDS production deployment.

Fill in the required AWS credentials under `[local.aws]` and `[prod.aws]` if needed.

---

## Option A: Run via Docker Compose (Recommended)

### Using Local LLM (Ollama)
Requires at least 16 GB RAM and a dedicated GPU (e.g. Nvidia RTX 4060 with 8 GB VRAM).

```powershell
.\scripts\run.ps1 -Profile local_llm
```

This script will:
1. Parse `[local.local]` from the root `.env`.
2. Start Postgres, Backend, and Frontend containers.
3. Spin up an Ollama container with Nvidia GPU support.
4. Auto-pull `llama3.2` and `tinyllama`.
5. Route all backend LLM calls to the local config.

### Using AWS Bedrock LLM
If your machine lacks a powerful GPU, offload inference to AWS Bedrock.

```powershell
.\scripts\run.ps1 -Profile bedrock
```

This will run the DB and Apps locally but use `[local.aws]` credentials to converse with Claude 3.5 Sonnet on AWS securely.

---

## Option B: Native Native Ollama (Mac / Linux / Windows)

If you prefer to run Ollama natively on your host OS (to bypass Docker virtualization overhead or better leverage Apple Silicon unified memory / Linux native drivers):

1. **Install Ollama** directly on your OS from [ollama.com](https://ollama.com).
2. Start the native server: `ollama serve` (or via Mac Menu bar app).
3. Pull required models from your host terminal:
   ```bash
   ollama pull llama3.2
   ollama pull tinyllama
   ```
4. **Modify `.env`**: Under `[local.local]`, alter the `LOCAL_LLM_URL` to point to the host gateway instead of the `ollama` container name:
   `LOCAL_LLM_URL=http://host.docker.internal:11434/api/generate`
5. Since the `run.ps1 -Profile local_llm` script attempts to launch the Dockerized Ollama container, you will need to manually invoke the native fallback compose if you wish to bypass Docker entirely, or simply run the Backend/Frontend manually via Node/Go.

---

## Windows GPU Support (WSL2 Allocations)

When using `run.ps1 -Profile local_llm` on Windows, Docker must correctly utilize the GPU.

### .wslconfig (Memory + CPU)

A `.wslconfig` template is provided at the project root. Copy it to your Windows user directory and restart WSL:
```powershell
Copy-Item .wslconfig ~.wslconfig
wsl --shutdown
```
Recommended settings for running llama3.2 (8B):
```ini
[wsl2]
memory=16GB     # llama3.2 needs ~8GB RAM during load; 16GB gives headroom
processors=8    # adjust to your CPU core count (leave 2+ cores for Windows)
swap=8GB        # protects against OOM if VRAM spills to RAM

[experimental]
hostAddressLoopback=true
autoMemoryReclaim=gradual
```

### GPU Passthrough (NVIDIA)

GPU access in WSL2 Docker is **automatic** — no special `.wslconfig` GPU settings are needed. Requirements:

1. **NVIDIA Windows Driver >= 472** (Game Ready or Studio) installed on Windows
2. **Docker Desktop** → Settings → General → "Use the WSL 2 based engine" enabled
3. The `ollama` service in `docker-compose.local-llm.yml` already declares full GPU access via the `deploy.resources.reservations.devices` block
4. Verify GPU is visible after starting: `docker exec aicm-ollama nvidia-smi`

---

## Accessing AWS Bedrock from Local (Bedrock Profile)

If you want to run the app locally but use **AWS Bedrock** for LLM inference instead of a local GPU:

### One-time Setup

#### Step 1: Create IAM Policy for Bedrock Access

1. Sign in to the **AWS Console** and navigate to **IAM**.
2. In the left sidebar, click **Policies** → **Create policy**.
3. Select the **JSON** tab and replace the default content with:
   ```json
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Action": ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"],
          "Resource": [
            "arn:aws:bedrock:*::foundation-model/*",
            "arn:aws:bedrock:*:*:inference-profile/*"
          ]
        },
        {
          "Effect": "Allow",
          "Action": ["aws-marketplace:ViewSubscriptions", "aws-marketplace:Subscribe"],
          "Resource": "*"
        }
      ]
    }
   ```
4. Click **Next**, enter the policy name `aicm-bedrock-invoke`, and click **Create policy**.

#### Step 2: Create IAM User

1. In the IAM left sidebar, click **Users** → **Create user**.
2. Enter username: `aicm-local-dev`. Click **Next**.
3. Under "Set permissions", select **Attach policies directly**.
4. Search for `aicm-bedrock-invoke` and tick the checkbox next to it.
5. Click **Next** → **Create user**.

#### Step 3: Create Access Keys

1. Click on the newly created user `aicm-local-dev`.
2. Go to the **Security credentials** tab.
3. Scroll down to **Access keys** and click **Create access key**.
4. Select **Application running outside AWS** as the use case. Click **Next** → **Create access key**.
5. **Copy and save both values immediately** — the Secret Access Key is shown only once:
   - **Access Key ID**: `AKIAxxxxxxxxxxxxx`
   - **Secret Access Key**: `xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

#### Step 4: Request Bedrock Model Access (if required)

In some AWS accounts and regions, Anthropic models must be explicitly enabled before they can be invoked:

1. Go to **Amazon Bedrock** → in the left sidebar, click **Model access**.
2. Click **Modify model access**.
3. Tick the checkboxes for all **Anthropic Claude** models used by this app (Haiku 3, Haiku 3.5, Sonnet 3.5 v2).
4. Click **Save changes**. Access is typically granted immediately or within a few minutes.

### Configure Local `.env`

Edit the root `.env` file and fill in the `[local.aws]` section:
```ini
[local.aws]
POSTGRES_USER=aicm
POSTGRES_PASSWORD=aicm_secret
AWS_ACCESS_KEY_ID=AKIAxxxxxxxxxxxxx
AWS_SECRET_ACCESS_KEY=your_secret_key
```

### Run
```powershell
.\scripts\run.ps1 -Profile bedrock
```
or on Linux/Mac:
```bash
./scripts/run.sh -p bedrock
```

This starts Postgres, Backend (with Bedrock credentials), and Frontend locally. No GPU required.

### Verify Bedrock Connectivity

After startup, check the backend health endpoint — it reports the active LLM provider:
```bash
curl http://localhost:8080/api/health
# Expected: {"database":"ok","provider":"aws","status":"ok"}
```

If you see `bedrock invoke failed: no credentials`, verify that `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY` are correctly set in the `[local.aws]` section of your root `.env`.

---

## Testing & Building

### Running E2E Tests
Make sure Postgres and Ollama are running via `run.ps1 -Profile local_llm`, then:

```powershell
.\scripts\test_e2e.ps1
```

### Building Production Images & Pushing to DockerHub

**Build all images locally** (no push):
```powershell
# Windows
.scriptsuild.ps1 all

# Linux/Mac
./scripts/build.sh all
```

**Build and push to DockerHub** (requires `DOCKER_USERNAME`, `DOCKER_PAT` in `[prod.aws]` of `.env`):
```powershell
# Windows
.scriptsuild.ps1 all -Target prod

# Linux/Mac
./scripts/build.sh all -t prod
```

The `-Target prod` flag will:
1. Read `DOCKER_USERNAME` and `DOCKER_PAT` from the `[prod.aws]` section of your root `.env`.
2. Log in to DockerHub via `docker login` using the PAT.
3. Build backend and frontend Docker images.
4. Push both images tagged as `<username>/aicm-backend:latest` and `<username>/aicm-frontend:latest`.

You can also build individual components:
```powershell
.scriptsuild.ps1 backend   # Go binary only (cross-compiled for Linux)
.scriptsuild.ps1 frontend  # Vite/React assets only
.scriptsuild.ps1 docker    # Docker images only
```

---

## Viewing Logs in Local Docker

Use `docker logs` to inspect running containers. Common container names:

| Container | Name |
|-----------|------|
| Backend (Go API) | `aicm-backend` |
| Frontend (nginx) | `aicm-frontend` |
| PostgreSQL | `aicm-postgres` |
| Ollama (local_llm only) | `aicm-ollama` |

```bash
# Tail live logs (follow mode)
docker logs -f aicm-backend
docker logs -f aicm-frontend
docker logs -f aicm-postgres

# Show last 100 lines
docker logs --tail 100 aicm-backend

# Show logs since a timestamp
docker logs --since 10m aicm-backend
```

**View logs for all services at once** (via Docker Compose):
```bash
# local_llm profile
docker compose -f infra/docker-compose.local-llm.yml logs -f

# bedrock profile
docker compose -f infra/docker-compose.bedrock.yml logs -f
```

**Check container health status:**
```bash
docker ps --format "table {{.Names}}	{{.Status}}	{{.Ports}}"
```

---

## Stopping Services
```powershell
.scriptsshutdown.ps1
```

Or on Linux/Mac:
```bash
./scripts/shutdown.sh
```
