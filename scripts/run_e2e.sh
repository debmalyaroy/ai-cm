#!/bin/bash
# =============================================================================
# AI-CM: End-to-end integration test runner for ai-cm on Linux/Mac.
# Usage: ./scripts/run_e2e.sh
# =============================================================================

set -e

echo "======================================"
echo " Starting Integration Test Suite...   "
echo "======================================"

# Determine if a local database needs to be spun up via docker
if ! nc -z localhost 5432 > /dev/null 2>&1; then
    echo -e "\033[33mWarning: No local postgres instance detected on port 5432.\033[0m"
    echo "Please ensure you have a valid DATABASE_URL exported, or run 'docker compose up -d db' to start the test database."
else
    echo "Local postgres instance detected."
fi

# Run the integration suite locally
echo -e "\nRunning E2E verification test suite..."

cd src/backend

# By explicitly calling out tests/e2e_test.go or a specific package,
# we isolate the E2E verification from unit tests
if go test ./tests/... -v -count=1 -timeout 120s; then
    echo -e "\033[32m======================================\033[0m"
    echo -e "\033[32m E2E Integration Suite Passed!        \033[0m"
    echo -e "\033[32m======================================\033[0m"
else
    echo -e "\033[31m======================================\033[0m"
    echo -e "\033[31m E2E Integration Suite Failed.        \033[0m"
    echo -e "\033[31m======================================\033[0m"
    exit 1
fi
