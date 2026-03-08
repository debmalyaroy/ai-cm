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
        line="${line//$'\r'/}"
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

    info "Downloading Go modules..."
    go mod download

    info "Running unit tests..."
    GOOS="" GOARCH="" CGO_ENABLED="" go test -skip 'E2E|e2e|EndToEnd' ./... -count=1 -timeout 120s || error "Backend unit tests failed"

    info "Generating Swagger docs..."
    if command -v swag &>/dev/null; then
        swag init -g cmd/server/main.go -d . --parseDependency --parseInternal
    else
        warn "swag not found — skipping Swagger generation (install: go install github.com/swaggo/swag/cmd/swag@latest)"
    fi

    info "Running golangci-lint..."
    if command -v golangci-lint &>/dev/null; then
        golangci-lint run --timeout=5m
    else
        warn "golangci-lint not found — skipping lint (install: https://golangci-lint.run/usage/install/)"
    fi

    mkdir -p "$ROOT_DIR/bin"
    info "Building Linux binary (amd64)..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$ROOT_DIR/bin/aicm-server-amd64" ./cmd/server
    info "Building Linux binary (arm64)..."
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$ROOT_DIR/bin/aicm-server-arm64" ./cmd/server
    info "Backend built: bin/aicm-server-amd64, bin/aicm-server-arm64"
}

build_frontend() {
    info "Building frontend..."
    cd "$ROOT_DIR/src/apps/web"
    if [ ! -d "node_modules" ]; then
        info "Installing dependencies..."
        npm install
    fi
    info "Running frontend tests..."
    npm test || error "Frontend tests failed"
    info "Building Vite/React app..."
    npm run build
    info "Frontend built: src/apps/web/dist/"
}

build_docker() {
    info "Building Docker images..."
    cd "$ROOT_DIR"

    REGISTRY_USER="localdev"
    PLATFORMS="linux/amd64,linux/arm64"

    # Ensure a multi-arch buildx builder is available
    info "Setting up multi-arch buildx builder (aicm-builder)..."
    create_out=$(docker buildx create --name aicm-builder --driver docker-container --bootstrap 2>&1) || \
        { echo "$create_out" | grep -q "existing instance" || error "Failed to create buildx builder: $create_out"; }
    docker buildx use aicm-builder

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

    if [ "$TARGET_ENV" == "prod" ]; then
        info "Building and pushing multi-arch backend image ($PLATFORMS): $BACKEND_TAG"
        docker buildx build --platform "$PLATFORMS" -t "$BACKEND_TAG" \
            -f infra/Dockerfile.backend ./src/backend --push

        info "Building and pushing multi-arch frontend image ($PLATFORMS): $FRONTEND_TAG"
        docker buildx build --platform "$PLATFORMS" -t "$FRONTEND_TAG" \
            -f infra/Dockerfile.frontend ./src/apps/web --push

        info "Multi-arch images pushed: $BACKEND_TAG, $FRONTEND_TAG"
    else
        # Docker daemon cannot load a multi-arch image; build native arch only for local use
        LOCAL_PLATFORM="linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
        warn "Local build: loading native arch only ($LOCAL_PLATFORM). Multi-arch requires a registry push."
        warn "   To build and push multi-arch images: ./scripts/build.sh docker -t prod"

        docker buildx build --platform "$LOCAL_PLATFORM" -t "$BACKEND_TAG" \
            -f infra/Dockerfile.backend ./src/backend --load

        docker buildx build --platform "$LOCAL_PLATFORM" -t "$FRONTEND_TAG" \
            -f infra/Dockerfile.frontend ./src/apps/web --load

        warn "Images loaded locally (tag: localdev/...). To push to DockerHub:"
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
