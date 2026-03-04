#!/bin/bash
# AI-CM Deploy Script (Unix/macOS/Linux)
# Usage: ./deploy.sh [dev|prod]
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

ENV="${1:-dev}"

deploy_dev() {
    info "Starting development environment..."
    cd "$ROOT_DIR"
    
    info "Starting Docker Compose stack..."
    docker compose up -d
    
    info "Waiting for PostgreSQL..."
    sleep 5
    
    info "Starting backend (dev mode)..."
    cd "$ROOT_DIR/src/backend"
    go run ./cmd/server &
    BACKEND_PID=$!
    
    info "Starting frontend (dev mode)..."
    cd "$ROOT_DIR/src/apps/web"
    npm run dev &
    FRONTEND_PID=$!
    
    info "✅ Development environment ready!"
    info "  Frontend: http://localhost:3000"
    info "  Backend:  http://localhost:8080"
    info "  Login:    http://localhost:3000/login (admin/admin)"
    info ""
    info "Press Ctrl+C to stop all services"
    
    trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null; docker compose stop" EXIT
    wait
}

deploy_prod() {
    info "Building production images..."
    cd "$ROOT_DIR"
    
    # Build first
    bash "$SCRIPT_DIR/build.sh" all
    
    info "Starting production stack with Docker Compose..."
    cd "$ROOT_DIR/infra"
    docker compose -f docker-compose.prod.yml up -d --build
    
    info "✅ Production deployment complete!"
    info "  Application: http://localhost"
    info "  API:         http://localhost:8080"
}

case "$ENV" in
    dev)  deploy_dev ;;
    prod) deploy_prod ;;
    *)
        echo "Usage: $0 [dev|prod]"
        exit 1
        ;;
esac
