#!/bin/bash
# AI-CM: Run End-to-End Tests (uses Mock LLM — no GPU/Ollama required)
# Usage: ./scripts/test_e2e.sh
set -e

echo "=================================================="
echo " Running AI-CM End-to-End Tests                   "
echo "=================================================="

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/src/backend"
INFRA_DIR="$ROOT_DIR/infra"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ── Start services ──────────────────────────────────────────────────────────
info "Starting Postgres and Mock LLM for E2E tests..."
cd "$INFRA_DIR"
docker compose -f docker-compose.e2e.yml up -d --build postgres llm-mock

info "Waiting for services to become healthy (up to 30s)..."
for i in $(seq 1 30); do
    PG_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' aicm-e2e-postgres 2>/dev/null || echo "missing")
    LLM_HEALTH=$(docker inspect --format='{{.State.Health.Status}}' aicm-e2e-llm-mock 2>/dev/null || echo "missing")

    if [ "$PG_HEALTH" = "healthy" ] && [ "$LLM_HEALTH" = "healthy" ]; then
        info "All services are healthy."
        break
    fi

    if [ "$i" -eq 30 ]; then
        error "Services did not become healthy within 30 seconds. PG=${PG_HEALTH}, LLM=${LLM_HEALTH}"
    fi

    sleep 1
done

# ── Run Go E2E tests ────────────────────────────────────────────────────────
cd "$BACKEND_DIR"
info "Executing Go E2E tests..."

DATABASE_URL="postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable" \
LLM_PROVIDER=local \
OLLAMA_BASE_URL=http://localhost:11434 \
go test ./tests/... -v -count=1 -timeout 180s

info "E2E Tests Passed!"
