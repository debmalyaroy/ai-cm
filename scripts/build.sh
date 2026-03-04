#!/bin/bash
# AI-CM Build Script (Unix/macOS/Linux)
# Usage: ./build.sh [all|backend|frontend|docker]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

build_backend() {
    info "Building backend..."
    cd "$ROOT_DIR/src/backend"
    
    info "Running tests..."
    go test ./... -count=1 || error "Backend tests failed"
    
    info "Building binary..."
    CGO_ENABLED=0 go build -o "$ROOT_DIR/bin/aicm-server" ./cmd/server
    
    info "Backend built: bin/aicm-server"
}

build_frontend() {
    info "Building frontend..."
    cd "$ROOT_DIR/src/apps/web"
    
    if [ ! -d "node_modules" ]; then
        info "Installing dependencies..."
        npm install
    fi
    
    info "Building Next.js app..."
    npm run build
    
    info "Frontend built: src/apps/web/.next/"
}

build_docker() {
    info "Building Docker images..."
    cd "$ROOT_DIR"
    
    docker compose build
    
    info "Docker images built successfully"
}

run_all_tests() {
    info "Running all backend tests..."
    cd "$ROOT_DIR/src/backend"
    go test ./... -v -count=1 -cover
}

TARGET="${1:-all}"

case "$TARGET" in
    backend)  build_backend ;;
    frontend) build_frontend ;;
    docker)   build_docker ;;
    test)     run_all_tests ;;
    all)
        build_backend
        build_frontend
        info "✅ All builds complete!"
        ;;
    *)
        echo "Usage: $0 [all|backend|frontend|docker|test]"
        exit 1
        ;;
esac
