# GitHub CI/CD Pipeline

This document covers the two GitHub Actions workflows: **PR Review** (quality gate on PRs) and **CI** (build, test, push images on merge to master).

> **Deployment is intentionally not automated from CI.** To deploy, run `scripts/deploy.sh` (or `scripts/deploy.ps1`) locally after CI passes.

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
 │  2. Build Vite frontend             │
 │  3. Push Docker images to DockerHub │
 └─────────────────────────────────────┘
                 │
                 ▼
         Deploy manually
         (scripts/deploy.sh)
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

#### `test`
- Sets up Go 1.22 and Node 20
- **Runs backend unit tests**: `go test -v -coverprofile=coverage.out -skip 'E2E|e2e|EndToEnd' ./...`
- Installs frontend dependencies and runs frontend tests (`npm test`)

#### `build-and-push`
- Runs only after `test` succeeds
- Sets up Go 1.22 and Node 20
- Generates Swagger docs (`swag init`)
- Runs `golangci-lint`
- Logs into DockerHub
- Builds backend and frontend Docker images via `./scripts/build.sh all -t prod`
- Pushes to DockerHub as:
  - `<username>/aicm-backend:latest` + `:<git-sha>`
  - `<username>/aicm-frontend:latest` + `:<git-sha>`

---

## Required GitHub Secrets

Go to **GitHub → your repo → Settings → Secrets and variables → Actions → New repository secret**.

| Secret | Value | Notes |
|---|---|---|
| `DOCKER_USERNAME` | Your DockerHub username | e.g., `debmalyaroy` |
| `DOCKER_PAT` | DockerHub **Personal Access Token** | See below — do NOT use your login password |

> No AWS credentials or API URL secrets are needed in GitHub. The frontend uses relative URLs (`/api/...`) so `VITE_API_URL` is not needed at build time — nginx on EC2 handles routing. Deployment is done locally via `scripts/deploy.sh`.

### How to create a DockerHub Access Token

Using an access token (instead of your password) is **more secure** — it can be scoped to read/write only and revoked independently without changing your password.

1. Log in to [hub.docker.com](https://hub.docker.com)
2. Click your avatar → **Account Settings**
3. Go to **Security → Access Tokens → New Access Token**
4. Name it `github-actions-aicm`, set permissions to **Read & Write**
5. Copy the token — it is shown **only once**
6. Paste it as the `DOCKER_PAT` GitHub secret (note: the secret name is `DOCKER_PAT`, not `DOCKER_PASSWORD`)

---

| Concern | PR Review | CI (Master) |
|---|---|---|
| Runs on PRs? | Yes | No |
| Enforces 80% Coverage? | Yes (Backend) | No (Builds only) |
| Builds Docker images? | No | Yes |
| Pushes to DockerHub? | No | Yes |
| Touches EC2? | No | No |
| Can run manually? | No | Yes |

Separating them means:
- PRs act as strict gatekeepers (linting, tests, coverage) without accidentally triggering image pushes.
- Merges to `master` generate artifacts efficiently without repeating heavy coverage processing.

---

## Docker Login — How It Works End-to-End

### 1. GitHub Actions (CI workflow — `.github/workflows/ci.yml`)
The workflow uses the official `docker/login-action` before building images:
```yaml
- name: Log in to Docker Hub
  uses: docker/login-action@v3
  with:
    username: ${{ secrets.DOCKER_USERNAME }}
    password: ${{ secrets.DOCKER_PAT }}
```

### 2. Local build with `-Target prod`
When you run `.\scripts\build.ps1 all -Target prod` locally, the script reads `DOCKER_PAT`
(or `DOCKER_PASSWORD` as fallback) from the root `.env` `[prod.aws]` section and calls
`docker login` automatically before pushing.

---

## AWS IAM Policies

Two IAM identities are needed to operate AI-CM on AWS.

### 1. EC2 Instance Role (for Bedrock access at runtime)

The EC2 instance that runs the containers needs this **IAM Role** attached so the backend can call Bedrock without storing credentials on the server:

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
    }
  ]
}
```

Create in IAM → Roles → Create role → AWS service → EC2, then attach this policy and assign the role to your EC2 instance.

### 2. EC2 Start/Stop User (optional, for the `aws_startstop` script)

If you use `scripts/aws_startstop.sh` or `.ps1` locally to start/stop the instance to save costs, that user needs:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:StartInstances",
        "ec2:StopInstances",
        "ec2:DescribeInstanceStatus",
        "rds:StartDBInstance",
        "rds:StopDBInstance",
        "rds:DescribeDBInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

Store `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for this user in the `[prod.aws]` section of your root `.env`.

## Running Manually

### Trigger CI manually
1. Go to **GitHub → Actions → CI**
2. Click **Run workflow** (top right)
3. Select branch `master` → **Run workflow**

### Deploy to EC2
Run locally after CI has pushed images to DockerHub:
```bash
./scripts/deploy.sh prod
# or on Windows:
.\scripts\deploy.ps1 prod
```

---

## Troubleshooting CI Failures

| Symptom | Likely Cause | Fix |
|---|---|---|
| CI not running on push | Branch mismatch | Ensure you're pushing to `master`, not `main` |
| `DOCKER_PAT` auth failure | Wrong secret or expired token | Regenerate DockerHub Access Token, update the `DOCKER_PAT` GitHub secret |
| `swag: command not found` | PATH issue | swag is installed with `go install` — check Go bin in PATH |
| `golangci-lint` version mismatch | Old action version | The workflow pins `v1.56.2` — update if lint config changes |
| Frontend build OOM in CI | Large Vite app | GitHub Actions runners have 7GB RAM — should not be an issue |

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
docker build -f infra/Dockerfile.frontend -t aicm-frontend:local ./src/apps/web
```
