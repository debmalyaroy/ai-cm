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
        line="${line//$'\r'/}"
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

if [ -z "$DOCKER_USERNAME" ]; then
    echo "[ERROR] DOCKER_USERNAME environment variable missing in [prod.aws]."
    exit 1
fi

echo "Logging into DockerHub as $DOCKER_USERNAME..."
if [ -n "$DOCKER_PAT" ]; then
    echo "$DOCKER_PAT" | docker login -u "$DOCKER_USERNAME" --password-stdin || { echo "[ERROR] Docker login failed. Check DOCKER_PAT in .env [prod.aws]."; exit 1; }
elif [ -n "$DOCKER_PASSWORD" ]; then
    echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin || { echo "[ERROR] Docker login failed."; exit 1; }
else
    echo "[ERROR] Neither DOCKER_PAT nor DOCKER_PASSWORD is set in [prod.aws]."
    exit 1
fi

cd "$ROOT_DIR/infra"

# Support both Docker Compose v2 plugin (docker compose) and v1 standalone (docker-compose)
if docker compose version &>/dev/null 2>&1; then
    COMPOSE="docker compose"
elif command -v docker-compose &>/dev/null; then
    COMPOSE="docker-compose"
else
    echo "[ERROR] Neither 'docker compose' nor 'docker-compose' found. Please install Docker Compose."
    exit 1
fi
echo "Using compose command: $COMPOSE"

echo "Pulling latest production images from $DOCKER_REGISTRY..."
$COMPOSE -f docker-compose.prod.yml pull

echo "Starting production containers..."
$COMPOSE -f docker-compose.prod.yml up -d

echo ""
echo "✅ Production deployment complete! Services are spinning up."
