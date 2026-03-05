# GitHub CI/CD Pipeline

This document covers the two GitHub Actions workflows: **CI** (build, test, push images) and **Deploy** (EC2 deployment).

---

## Overview

```
Push to master
      │
      ▼
  │  PR Review  (pr_review.yml)        │
  │  1. Run Formatting & Linting       │
  │  2. Test Backend (skip e2e)        │
  │  3. Check Backend Coverage (>= 80%)│
  │  4. Lint & Build Frontend          │
  └──────────────┬─────────────────────┘
                 │ (if merged)
                 ▼
  ┌─────────────────────────────────────┐
  │  CI workflow  (ci.yml)             │
 │  1. Build & test Go backend         │
 │  2. Build Next.js frontend          │
 │  3. Push Docker images to DockerHub │
 └──────────────┬──────────────────────┘
                │ (on success)
                ▼
 ┌─────────────────────────────────────┐
 │  Deploy workflow  (deploy.yml)      │
 │  1. SSH into EC2                    │
 │  2. Pull images from DockerHub      │
 │  3. Restart containers              │
 └─────────────────────────────────────┘
```

---

## Workflow 1 — PR Review & Quality Gate (`pr_review.yml`)

### Triggers
| Event | When |
|---|---|
| `pull_request` to `master` | Automatic on every PR targeting master |

### What it does (Backend Gatekeeper)
- Validates code formatting and linting (`golangci-lint`)
- Runs unit and integration tests (explicitly **skipping `e2e_test.go`** as it requires a local DB)
- Checks backend test coverage (`go tool cover`) to ensure it is **$\ge$ 80%** overall, failing the action if the threshold is not met.

### What it does (Frontend Gatekeeper)
- Runs `npm run lint` and `npm run build`
- **Exclusion:** Explicitly skips frontend test coverage enforcement to avoid stalling UI iteration.

> A PR cannot be merged if it drops total backend coverage below 80% or breaks the frontend build.

---

## Workflow 2 — CI (`ci.yml`)

### Triggers
| Event | When |
|---|---|
| `push` to `master` | Automatic on every commit pushed or PR merged to master |
| `workflow_dispatch` | Manual — click **Run workflow** in GitHub Actions tab |

### Jobs

#### `build-backend`
- Sets up Go 1.22
- Downloads modules (`go mod download`)
- Generates Swagger docs (`swag init`)
- Runs `golangci-lint`
- **Runs tests excluding e2e**: `go test -skip 'E2E|e2e|EndToEnd' ./...`
- Builds the binary (`go build ./cmd/server`)

#### `build-frontend`
- Sets up Node 20
- Installs dependencies (`npm ci`)
- Builds Next.js (`npm run build`)

#### `push-images`
- Runs only after **both** `build-backend` and `build-frontend` succeed
- Skipped on pull requests (images are only pushed from `master`)
- Builds backend and frontend Docker images
- Pushes to DockerHub as:
  - `<username>/aicm-backend:latest` + `:<git-sha>`
  - `<username>/aicm-frontend:latest` + `:<git-sha>`
- Uses GitHub Actions layer cache for faster builds

---

## Workflow 3 — Deploy (`deploy.yml`)

### Triggers
| Event | When |
|---|---|
| `workflow_run` (CI completes) | Auto-deploys to EC2 when CI succeeds on master |
| `workflow_dispatch` | Manual trigger — use when you want to re-deploy without a code push (e.g., after updating `.env.prod` on EC2 manually) |

### What it does
1. Copies updated config files to EC2 (`docker-compose.prod.yml`, `config.prod.yaml`, `nginx.conf`, prompts)
2. Updates `DOCKER_REGISTRY` in `.env.prod` on EC2
3. Pulls the latest pre-built images from DockerHub (`docker compose pull`)
4. Restarts all containers (`docker compose up -d --remove-orphans`)

> No Docker build happens on EC2 — images are always pulled from DockerHub. This avoids OOM on the 1GB t3.micro instance.

---

## Required GitHub Secrets

Go to **GitHub → your repo → Settings → Secrets and variables → Actions → New repository secret**.

| Secret | Value | Notes |
|---|---|---|
| `DOCKER_USERNAME` | Your DockerHub username | e.g., `debmalyaroy` |
| `DOCKER_PASSWORD` | DockerHub **Access Token** | See below — do NOT use your login password |
| `NEXT_PUBLIC_API_URL` | Public URL of your app | e.g., `http://your-elastic-ip` or `https://yourdomain.com` |
| `AWS_EC2_HOST` | EC2 Elastic IP or hostname | e.g., `54.123.45.67` |
| `AWS_EC2_USER` | SSH username | `ec2-user` (Amazon Linux) or `ubuntu` (Ubuntu) |
| `AWS_SSH_PRIVATE_KEY` | Contents of your `.pem` key file | Paste the full PEM including `-----BEGIN RSA PRIVATE KEY-----` |

### How to create a DockerHub Access Token

Using an access token (instead of your password) is **more secure** — it can be scoped to read/write only and revoked independently without changing your password.

1. Log in to [hub.docker.com](https://hub.docker.com)
2. Click your avatar → **Account Settings**
3. Go to **Security → Access Tokens → New Access Token**
4. Name it `github-actions-aicm`, set permissions to **Read & Write**
5. Copy the token — it is shown **only once**
6. Paste it as the `DOCKER_PASSWORD` GitHub secret

### How to get your SSH private key for `AWS_SSH_PRIVATE_KEY`

```bash
# Print the contents of your .pem key — copy the entire output including header/footer
cat ~/.ssh/aicm-key.pem
```

Paste the entire output (including `-----BEGIN RSA PRIVATE KEY-----` and `-----END RSA PRIVATE KEY-----`) into the GitHub secret.

---

| Concern | PR Review | CI (Master) | Deploy (EC2) |
|---|---|---|---|
| Runs on PRs? | Yes | No | No |
| Enforces 80% Coverage? | Yes (Backend) | No (Builds only) | No |
| Builds Docker images? | No | Yes | No (pulls images) |
| Touches EC2? | No | No | Yes |
| Can run manually? | No | Yes | Yes |

Separating them means:
- PRs act as strict gatekeepers (linting, tests, coverage) without accidentally triggering image pushes.
- Merges to `master` generate artifacts efficiently without repeating heavy coverage processing.
- You can re-deploy to EC2 independently without pushing code.

---

## Running Manually

### Trigger CI manually
1. Go to **GitHub → Actions → CI**
2. Click **Run workflow** (top right)
3. Select branch `master` → **Run workflow**

### Trigger Deploy manually
1. Go to **GitHub → Actions → Deploy AI-CM to AWS**
2. Click **Run workflow** → **Run workflow**

---

## Troubleshooting CI Failures

| Symptom | Likely Cause | Fix |
|---|---|---|
| CI not running on push | Branch mismatch | Ensure you're pushing to `master`, not `main` |
| `DOCKER_PASSWORD` auth failure | Wrong secret or expired token | Regenerate DockerHub Access Token and update the secret |
| `swag: command not found` | PATH issue | swag is installed with `go install` — check Go bin in PATH |
| `golangci-lint` version mismatch | Old action version | The workflow pins `v1.56.2` — update if lint config changes |
| Frontend build OOM in CI | Large Next.js app | GitHub Actions runners have 7GB RAM — should not be an issue |
| `AWS_SSH_PRIVATE_KEY` permission denied | Key format wrong | Ensure the full PEM block is in the secret (no extra whitespace) |
| Deploy job skipped | CI failed | Check CI logs — deploy only runs when CI succeeds |

---

## Local Equivalent Commands

If you want to replicate CI steps locally:

```bash
# Backend tests (excluding e2e)
cd src/backend
go test -v -skip 'E2E|e2e|EndToEnd' ./...

# Backend build
export CONFIG_PATH=../../config/config.local.yaml
go build -v ./cmd/server

# Frontend build
cd src/apps/web
npm ci && npm run build

# Build Docker images locally
docker build -f infra/Dockerfile.backend -t aicm-backend:local ./src/backend
docker build -f infra/Dockerfile.frontend --build-arg NEXT_PUBLIC_API_URL=http://localhost -t aicm-frontend:local ./src/apps/web
```
