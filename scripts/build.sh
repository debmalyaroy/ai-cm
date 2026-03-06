#!/bin/bash
# AI-CM Build Script (Unix/macOS/Linux)
# Usage: ./build.sh [all|backend|frontend|docker] [-t prod]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

parse_env_prod() {
    local in_section=0
    while IFS= read -r line || [ -n "$line" ]; do
        [[ "$line" =~ ^#.* ]] && continue
        [[ -z "${line// }" ]] && continue
        if [[ "$line" =~ ^\[(.*)\]$ ]]; then
            if [ "${BASH_REMATCH[1]}" == "prod.aws" ]; then in_section=1; else in_section=0; fi
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

build_backend() {
    info "Building backend..."
    cd "$ROOT_DIR/src/backend"
    info "Running internal unit tests..."
    go test ./internal/... -count=1 || error "Backend internal tests failed"
    info "Building cross-platform Linux binary..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$ROOT_DIR/bin/aicm-server" ./cmd/server
    info "Backend built: bin/aicm-server (Linux)"
}

build_frontend() {
    info "Building frontend..."
    cd "$ROOT_DIR/src/apps/web"
    if [ ! -d "node_modules" ]; then
        info "Installing dependencies..."
        npm install
    fi
    info "Building Vite/React app..."
    npm run build
    info "Frontend built: src/apps/web/dist/"
}

build_docker() {
    info "Building Docker images..."
    cd "$ROOT_DIR"
    
    REGISTRY_USER="localdev"
    if [ "$TARGET_ENV" == "prod" ]; then
        if [ -f "$ENV_FILE" ]; then parse_env_prod; fi
        
        if [ -z "$DOCKER_USERNAME" ]; then
            error "DOCKER_USERNAME must be set in .env [prod.aws] or CI environment to push to production"
        fi
        REGISTRY_USER="$DOCKER_USERNAME"
        
        # Prefer PAT (Personal Access Token) over plain password
        if [ -n "$DOCKER_PAT" ]; then
            info "Logging into DockerHub using Personal Access Token..."
            echo "$DOCKER_PAT" | docker login -u "$DOCKER_USERNAME" --password-stdin
        elif [ -n "$DOCKER_PASSWORD" ]; then
            info "Logging into DockerHub using Password..."
            echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
        fi
    fi

    BACKEND_TAG="$REGISTRY_USER/aicm-backend:latest"
    FRONTEND_TAG="$REGISTRY_USER/aicm-frontend:latest"

    info "Building backend image: $BACKEND_TAG"
    docker build -t "$BACKEND_TAG" -f infra/Dockerfile.backend ./src/backend
    
    info "Building frontend image: $FRONTEND_TAG"
    docker build -t "$FRONTEND_TAG" -f infra/Dockerfile.frontend ./src/apps/web

    if [ "$TARGET_ENV" == "prod" ]; then
        info "Pushing images to DockerHub..."
        docker push "$BACKEND_TAG"
        docker push "$FRONTEND_TAG"
    fi
    if [ "$TARGET_ENV" != "prod" ]; then
        warn "Images built locally only (tag: localdev/...). To push to DockerHub:"
        warn "   ./scripts/build.sh all -t prod"
    fi
    info "Docker images built successfully"
}

ACTION="all"
TARGET_ENV="local"

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -t|--target) TARGET_ENV="$2"; shift ;;
        all|backend|frontend|docker) ACTION="$1" ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

case "$ACTION" in
    backend)  build_backend ;;
    frontend) build_frontend ;;
    docker)   build_docker ;;
    all)
        build_backend
        build_frontend
        build_docker
        info "✅ All builds complete!"
        ;;
esac
