#!/bin/bash
# AI-CM: Run End-to-End Tests
# Usage: ./scripts/test_e2e.sh
set -e

echo "=================================================="
echo " Running AI-CM End-to-End Tests                   "
echo "=================================================="

ROOT_DIR=$(dirname "$0")/..
ENV_FILE="$ROOT_DIR/.env"
BACKEND_DIR="$ROOT_DIR/src/backend"
INFRA_DIR="$ROOT_DIR/infra"

if [ ! -f "$ENV_FILE" ]; then
    echo "[ERROR] Root .env file not found. Copy .env.example to .env."
    exit 1
fi

export_section_vars() {
    local target_section="$1"
    local in_section=0
    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^#.* ]] && continue
        [[ -z "${line// }" ]] && continue
        if [[ "$line" =~ ^\[(.*)\]$ ]]; then
            if [ "${BASH_REMATCH[1]}" == "$target_section" ]; then in_section=1; else in_section=0; fi
            continue
        fi
        if [ $in_section -eq 1 ]; then
            if [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
                key="${BASH_REMATCH[1]}"
                value="${BASH_REMATCH[2]}"
                value="${value%%#*}"
                value="$(echo -e "${value}" | sed -e 's/[[:space:]]*$//')"
                value="${value%\"}"; value="${value#\"}"
                value="${value%\'}"; value="${value#\'}"
                export "$key=$value"
            fi
        fi
    done < "$ENV_FILE"
}

echo "Loading [local.local] secrets for E2E..."
export_section_vars "local.local"

echo "Starting Postgres and Ollama locally..."
cd "$INFRA_DIR"
docker compose -f docker-compose.local-llm.yml up -d postgres ollama

echo "Waiting 5 seconds for Ollama..."
sleep 5
docker exec aicm-ollama ollama pull llama3.2 || true

cd "$BACKEND_DIR"
echo "Executing Go E2E tests..."
LLM_PROVIDER=local go test ./tests/... -v -count=1 -timeout 120s

echo "✅ E2E Tests Passed!"
