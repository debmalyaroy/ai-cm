# How to Run Frontend and Backend Locally

This guide provides instructions on how to run the AI-CM frontend and backend services locally on your machine for development without relying entirely on Docker Compose.

All commands are written for **Windows PowerShell**. Run everything from the project root unless stated otherwise.

## Prerequisites

- **Node.js** (v18 or newer) and npm
- **Go** (v1.22+)
- **Docker Desktop** (with WSL2 backend for GPU support)
- **Git**
- **Ollama** installed natively — [https://ollama.com/download](https://ollama.com/download) — if running the backend without Docker

---

## Option A: One-Command Startup (Docker Compose)

### With a Cloud LLM (Gemini / OpenAI)

1. Ensure your local environment variables are set up:
   ```powershell
   Copy-Item config\.env.local config\.env.local
   notepad config\.env.local
   ```
2. Run the stack:
   ```powershell
   .\scripts\run.ps1
   ```

### With a Local LLM (Ollama on GPU)

Requires at least 16 GB RAM and a dedicated GPU (e.g. Nvidia RTX 4060 with 8 GB VRAM).

```powershell
.\scripts\run_local_llm.ps1
```

This script will:
1. Start Postgres, Backend, and Frontend containers.
2. Spin up an Ollama container with Nvidia GPU support.
3. Pull `llama3.2` (worker agents) and `tinyllama` (supervisor intent classifier).
4. Route all backend LLM calls to the local Ollama container.

> **Note:** Make sure Docker Desktop is using the WSL2 backend and has GPU access enabled.

---

## Option B: Native Development (Recommended for Hacking)

Run Postgres in Docker and the backend/frontend directly on your machine for fast iteration.

### 1. Start the Database

```powershell
docker compose -f infra/docker-compose.yml up postgres -d
```

The database will be accessible at `localhost:5432` (user: `aicm`, password: `aicm_secret`).

### 2. Pull LLM Models (if using Ollama natively)

Open a separate terminal and start Ollama, then pull the required models:

```powershell
# In a separate terminal — keep it running
ollama serve
```

```powershell
# In another terminal
ollama pull llama3.2
ollama pull tinyllama
```

### 3. Run the Backend (Go)

```powershell
Set-Location src\backend
go mod tidy
go run ./cmd/server
```

The backend reads `config.local.yaml` from `../../config/config.local.yaml` (relative to the `src/backend` working directory). The defaults map to `localhost:11434` (native Ollama) and `localhost:5432` (Postgres) — no extra env vars needed for a basic local run.

To override individual settings without editing the file:

```powershell
$env:DATABASE_URL  = "postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable"
$env:LLM_PROVIDER  = "local"
$env:LOCAL_LLM_URL = "http://localhost:11434/api/generate"
# or set OPENAI_API_KEY, GEMINI_API_KEY etc.
```

The API will be available at `http://localhost:8080`.

### 4. Run the Frontend (Next.js)

Open a new terminal:

```powershell
Set-Location src\apps\web
npm install
npm run dev
```

Access the app at `http://localhost:3000`.

---

## Running E2E Tests

Make sure Postgres and Ollama are running, then:

```powershell
.\scripts\run_e2e.ps1
```

Or run directly:

```powershell
Set-Location src\backend
go test ./tests/... -v -count=1 -timeout 120s
```

Tests skip automatically if Ollama is offline or Postgres is unreachable.

---

## Building

```powershell
# Build backend binary only
.\scripts\build.ps1 -Target backend

# Build frontend only
.\scripts\build.ps1 -Target frontend

# Build both
.\scripts\build.ps1

# Run all unit tests
.\scripts\build.ps1 -Target test

# Build Docker images
.\scripts\build.ps1 -Target docker
```

---

## Stopping Services

Press `Ctrl+C` in the terminals running the backend and frontend.

Stop the database container:

```powershell
docker compose -f infra/docker-compose.yml down
```

For the local LLM Docker stack:

```powershell
docker compose -f infra/docker-compose.yml -f infra/docker-compose.local-llm.yml down
```

Or use the shutdown script:

```powershell
.\scripts\shutdown.ps1
```
