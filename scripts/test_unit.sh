#!/bin/bash
# AI-CM: Run Unit Tests Only (no E2E, no external services needed)
# Usage: ./scripts/test_unit.sh
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

echo "=================================================="
echo " Running AI-CM Unit Tests                         "
echo "=================================================="

# ── Backend unit tests ──────────────────────────────────────────────────────
echo ""
info "[BACKEND] Running Go unit tests (skipping E2E)..."
cd "$ROOT_DIR/src/backend"

go test \
    -v \
    -coverprofile=coverage.out \
    -covermode=atomic \
    -skip 'E2E|e2e|EndToEnd' \
    ./... \
    -count=1 \
    -timeout 120s

info "[BACKEND] Generating coverage report..."
go tool cover -func=coverage.out | tail -10

TOTAL=$(go tool cover -func=coverage.out | grep total: | grep -Eo '[0-9]+\.[0-9]+')
echo ""
info "[BACKEND] Total coverage: ${TOTAL}%"

if (( $(echo "$TOTAL < 80" | bc -l) )); then
    warn "[BACKEND] Coverage ${TOTAL}% is below the recommended 80% threshold."
else
    info "[BACKEND] Coverage ${TOTAL}% meets the 80% threshold."
fi

# ── Frontend unit tests ─────────────────────────────────────────────────────
echo ""
info "[FRONTEND] Running frontend tests..."
cd "$ROOT_DIR/src/apps/web"

if [ ! -d "node_modules" ]; then
    info "[FRONTEND] Installing npm dependencies..."
    npm ci
fi

npm test

echo ""
info "All unit tests passed!"
