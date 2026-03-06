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

# Force stop and remove all AI-CM containers regardless of which compose file was used
for container in aicm-frontend aicm-backend aicm-ollama aicm-postgres; do
    docker rm -f "$container" >/dev/null 2>&1 || true
done

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
