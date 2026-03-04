#!/bin/bash
# =============================================================================
# AI-CM: One-command startup script (Linux/Mac)
# Usage: ./scripts/run.sh
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
INFRA_DIR="$ROOT_DIR/infra"

# Check for .env file
if [ ! -f "$INFRA_DIR/.env" ]; then
    echo "⚠️  No .env file found. Copying from .env.example..."
    cp "$INFRA_DIR/.env.example" "$INFRA_DIR/.env"
    echo "📝 Please edit infra/.env with your API keys, then re-run this script."
    exit 1
fi

# Source the .env file to validate
source "$INFRA_DIR/.env"

# Validate required API key
if [ "$LLM_PROVIDER" = "gemini" ] && [ -z "$GEMINI_API_KEY" ] || [ "$GEMINI_API_KEY" = "your_gemini_api_key_here" ]; then
    echo "❌ GEMINI_API_KEY is not set. Please edit infra/.env"
    exit 1
elif [ "$LLM_PROVIDER" = "openai" ] && [ -z "$OPENAI_API_KEY" ] || [ "$OPENAI_API_KEY" = "your_openai_api_key_here" ]; then
    echo "❌ OPENAI_API_KEY is not set. Please edit infra/.env"
    exit 1
elif [ "$LLM_PROVIDER" = "aws" ]; then
    if [ -z "$AWS_ACCESS_KEY_ID" ] || [ "$AWS_ACCESS_KEY_ID" = "your_aws_access_key" ]; then
        echo "⚠️  AWS_ACCESS_KEY_ID is not set in infra/.env. Assuming IAM roles or ~/.aws/credentials are configured."
    fi
fi

echo "🚀 Starting AI-CM (LLM Provider: $LLM_PROVIDER)..."
cd "$INFRA_DIR"
docker compose --env-file .env up --build -d

echo ""
echo "✅ AI-CM is starting up!"
echo "   Frontend: http://localhost:${FRONTEND_PORT:-3000}"
echo "   Backend:  http://localhost:${BACKEND_PORT:-8080}"
echo ""
echo "📋 View logs: docker compose -f infra/docker-compose.yml logs -f"
echo "🛑 Stop:      docker compose -f infra/docker-compose.yml down"
