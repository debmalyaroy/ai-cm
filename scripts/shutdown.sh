#!/bin/bash
# =============================================================================
# AI-CM: Shutdown script (Linux/Mac Bash)
# Usage: ./scripts/shutdown.sh
# =============================================================================

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
INFRA_DIR="$ROOT_DIR/infra"

echo -e "\033[33m🛑 Stopping AI-CM services...\033[0m"

pushd "$INFRA_DIR" > /dev/null

# Stop services including Ollama if the local-llm override is present
if [ -f "docker-compose.local-llm.yml" ]; then
    docker compose -f docker-compose.yml -f docker-compose.local-llm.yml down --remove-orphans || echo -e "\033[33m⚠️  docker compose down failed\033[0m"
else
    docker compose down --remove-orphans || echo -e "\033[33m⚠️  docker compose down failed\033[0m"
fi

popd > /dev/null

# Kill any orphan backend/frontend processes
if pgrep -x "server" > /dev/null; then
    echo -e "\033[90m   Stopping orphan backend processes...\033[0m"
    pkill -x -9 "server" || true
fi

# Kill node processes associated with the project
if pgrep "node" > /dev/null; then
    echo -e "\033[90m   Stopping related node processes...\033[0m"
    pkill -f "next" || true
fi

echo ""
echo -e "\033[32m✅ AI-CM services stopped.\033[0m"
