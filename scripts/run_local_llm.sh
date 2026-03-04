#!/bin/bash
# =============================================================================
# AI-CM: Start AI-CM with Local LLM (Ollama) on GPU
# Usage: ./scripts/run_local_llm.sh
# Requires: 16GB RAM + RTX 4060 (or compatible GPU)
# =============================================================================

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
INFRA_DIR="$ROOT_DIR/infra"

echo -e "\033[32mStarting AI-CM with Local LLM (Llama 3.2)...\033[0m"
echo -e "\033[36mThis configuration is optimized for deep reasoning and SQL proficiency on 8GB VRAM.\033[0m\n"

pushd "$INFRA_DIR" > /dev/null

echo -e "\033[33mPulling and starting containers...\033[0m"
docker compose -f docker-compose.yml -f docker-compose.local-llm.yml up --build -d

echo -e "\033[33mWaiting for Ollama service to start...\033[0m"
ollamaReady=false
for i in {1..30}; do
    if curl -s -f http://localhost:11434/ > /dev/null; then
        ollamaReady=true
        break
    fi
    sleep 2
done

if [ "$ollamaReady" = false ]; then
    echo -e "\033[31m[ERROR] Ollama didn't start in time. Check logs: docker compose logs ollama\033[0m"
    popd > /dev/null
    exit 1
fi

echo -e "\033[33mDownloading Local LLM (llama3.2)... This will take a moment.\033[0m"
echo -e "\033[36mLlama 3.2 provides superior reasoning and Text-to-SQL logic handling over previous models.\033[0m"
docker exec aicm-ollama ollama pull llama3.2

echo -e "\033[33mDownloading tinyllama for fast intent classification (Supervisor Agent)...\033[0m"
docker exec aicm-ollama ollama pull tinyllama

popd > /dev/null

echo ""
echo -e "\033[32m[OK] AI-CM is starting up with Local LLM!\033[0m"
echo "   Frontend: http://localhost:3000"
echo "   Backend:  http://localhost:8080"
echo "   LLM API:  http://localhost:11434"
echo ""
echo -e "\033[36mView logs: docker compose -f infra/docker-compose.yml -f infra/docker-compose.local-llm.yml logs -f\033[0m"
echo -e "\033[36mStop: docker compose -f infra/docker-compose.yml -f infra/docker-compose.local-llm.yml down\033[0m"
