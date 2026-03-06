#!/bin/bash
# AI-CM: AWS Production Deployment Script
# Usage: ./scripts/deploy.sh
set -e

ROOT_DIR=$(dirname "$0")/..
ENV_FILE="$ROOT_DIR/.env"

if [ ! -f "$ENV_FILE" ]; then
    echo "[ERROR] Root .env file not found. Production requires [prod.aws] profile."
    exit 1
fi

parse_env_prod() {
    local in_section=0

    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^#.* ]] && continue
        [[ -z "${line// }" ]] && continue

        if [[ "$line" =~ ^\[(.*)\]$ ]]; then
            if [ "${BASH_REMATCH[1]}" == "prod.aws" ]; then
                in_section=1
            else
                in_section=0
            fi
            continue
        fi

        if [ $in_section -eq 1 ]; then
            if [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
                key="${BASH_REMATCH[1]}"
                value="${BASH_REMATCH[2]}"
                value="${value%%#*}"
                value="$(echo -e "${value}" | sed -e 's/[[:space:]]*$//')"
                value="${value%\"}"
                value="${value#\"}"
                value="${value%\'}"
                value="${value#\'}"
                export "$key=$value"
            fi
        fi
    done < "$ENV_FILE"
}

echo "=================================================="
echo " Deploying AI-CM to AWS Production                "
echo "=================================================="

echo "Parsing [prod.aws] keys..."
parse_env_prod

if [ -z "$DOCKER_REGISTRY" ]; then
    echo "[ERROR] DOCKER_REGISTRY environment variable missing in [prod.aws]."
    exit 1
fi

cd "$ROOT_DIR/infra"

echo "Pulling latest production images from $DOCKER_REGISTRY..."
docker compose -f docker-compose.prod.yml pull

echo "Starting production containers..."
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "✅ Production deployment complete! Services are spinning up."
echo "   Frontend expecting traffic at: $VITE_API_URL"
