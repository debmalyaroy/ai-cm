#!/bin/bash
# =============================================================================
# AI-CM: End-to-End Deployment Wrapper
# Usage: ./scripts/deploy_e2e.sh [local|prod]
# =============================================================================

set -e

ENV_TARGET=$1

if [ -z "$ENV_TARGET" ]; then
    echo -e "\033[31mUsage: $0 [local|prod]\033[0m"
    exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
INFRA_DIR="$ROOT_DIR/infra"
CONFIG_DIR="$ROOT_DIR/config"

if [ "$ENV_TARGET" == "local" ]; then
    echo -e "\033[36mDeploying Local Environment (DEBUG, Local LLM)...\033[0m"
    ENV_FILE="$CONFIG_DIR/.env.local"
    # Ensure env file exists
    if [ ! -f "$ENV_FILE" ]; then
        echo -e "\033[33mWarning: $ENV_FILE not found. Proceeding with defaults.\033[0m"
    fi

    pushd "$INFRA_DIR" > /dev/null
    docker compose --env-file "$ENV_FILE" -f docker-compose.yml -f docker-compose.local-llm.yml up --build -d
    popd > /dev/null
    
    echo -e "\033[32m✅ Local deployment complete. Check logs via: docker compose -f infra/docker-compose.yml -f infra/docker-compose.local-llm.yml logs -f\033[0m"

elif [ "$ENV_TARGET" == "prod" ]; then
    echo -e "\033[36mDeploying Production Environment (INFO, AWS Bedrock/OpenAI)...\033[0m"
    ENV_FILE="$CONFIG_DIR/.env.prod"
    # Ensure env file exists
    if [ ! -f "$ENV_FILE" ]; then
        echo -e "\033[31mError: $ENV_FILE is required for production. Copy .env.prod template and fill secrets.\033[0m"
        exit 1
    fi

    pushd "$INFRA_DIR" > /dev/null
    # Build images from source and start containers.
    # Note: Next.js build requires ~512MB RAM; ensure the host has swap configured.
    # See scripts/aws_deploy.sh for swap setup on EC2.
    docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml up --build -d
    popd > /dev/null

    echo -e "\033[32m✅ Production deployment complete. App accessible on port 80 (nginx).\033[0m"

else
    echo -e "\033[31mUnknown target: $ENV_TARGET. Use 'local' or 'prod'.\033[0m"
    exit 1
fi
