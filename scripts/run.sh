#!/bin/bash
# AI-CM: Run Applications locally for active development.
# Usage: ./scripts/run.sh -p [local_llm|bedrock]
set -e

PROFILE=""
while getopts p: flag
do
    case "${flag}" in
        p) PROFILE=${OPTARG};;
    esac
done

if [ "$PROFILE" != "local_llm" ] && [ "$PROFILE" != "bedrock" ]; then
    echo "[ERROR] Invalid profile. Usage: ./run.sh -p [local_llm|bedrock]"
    exit 1
fi

ROOT_DIR=$(dirname "$0")/..
ENV_FILE="$ROOT_DIR/.env"

if [ ! -f "$ENV_FILE" ]; then
    echo "[ERROR] Root .env file not found. Copy .env.example to .env."
    exit 1
fi

# Function to parse env variables from a specific INI-style section
export_section_vars() {
    local target_section="$1"
    local in_section=0

    while IFS= read -r line || [ -n "$line" ]; do
        # Ignore full line comments and empty lines
        [[ "$line" =~ ^#.* ]] && continue
        [[ -z "${line// }" ]] && continue

        # Check for section headers
        if [[ "$line" =~ ^\[(.*)\]$ ]]; then
            section_name="${BASH_REMATCH[1]}"
            if [ "$section_name" == "$target_section" ]; then
                in_section=1
            else
                in_section=0
            fi
            continue
        fi

        # If inside target section, parse and export variables
        if [ $in_section -eq 1 ]; then
            if [[ "$line" =~ ^([^=]+)=(.*)$ ]]; then
                key="${BASH_REMATCH[1]}"
                value="${BASH_REMATCH[2]}"
                # Remove inline trailing comments
                value="${value%%#*}"
                # Remove trailing whitespace
                value="$(echo -e "${value}" | sed -e 's/[[:space:]]*$//')"
                # Remove surrounding quotes
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
echo " Starting AI-CM Local Environment                 "
echo " Profile: $PROFILE                                "
echo "=================================================="

if [ "$PROFILE" == "local_llm" ]; then
    echo "Parsing [local.local] keys..."
    export_section_vars "local.local"
    COMPOSE_FILE="docker-compose.local-llm.yml"
elif [ "$PROFILE" == "bedrock" ]; then
    echo "Parsing [local.aws] keys..."
    export_section_vars "local.aws"
    COMPOSE_FILE="docker-compose.bedrock.yml"
fi

cd "$ROOT_DIR/infra"

echo "Building Docker images (locally)..."
docker compose -f $COMPOSE_FILE build

echo "Starting containers..."
docker compose -f $COMPOSE_FILE up -d

if [ "$PROFILE" == "local_llm" ]; then
    echo "Waiting for Ollama..."
    sleep 5
    echo "Pulling required local LLM models (this may take a while)..."
    docker exec aicm-ollama ollama pull llama3.2
    docker exec aicm-ollama ollama pull tinyllama
fi

echo ""
echo "✅ System started successfully!"
echo "   Frontend: http://localhost:3000"
echo "   Backend:  http://localhost:8080"
