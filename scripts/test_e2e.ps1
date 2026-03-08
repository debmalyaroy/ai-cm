<#
.SYNOPSIS
AI-CM: Run End-to-End Tests (uses Mock LLM — no GPU/Ollama required)
Usage: .\scripts\test_e2e.ps1
#>

$ErrorActionPreference = "Stop"

Write-Host "=================================================="
Write-Host " Running AI-CM End-to-End Tests                   "
Write-Host "=================================================="

$ScriptDir  = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir    = Split-Path -Parent $ScriptDir
$BackendDir = Join-Path $RootDir "src\backend"
$InfraDir   = Join-Path $RootDir "infra"

function Write-Info { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Err  { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red }

# ── Start services ──────────────────────────────────────────────────────────
Write-Info "Starting Postgres and Mock LLM for E2E tests..."

Push-Location $InfraDir
try {
    docker compose -f docker-compose.e2e.yml up -d --build postgres llm-mock
    if ($LASTEXITCODE -ne 0) { Write-Err "Failed to start E2E services"; exit 1 }
}
finally {
    Pop-Location
}

Write-Info "Waiting for services to become healthy (up to 30s)..."
$healthy = $false
for ($i = 1; $i -le 30; $i++) {
    $pgHealth  = (docker inspect --format='{{.State.Health.Status}}' aicm-e2e-postgres  2>$null) -join ""
    $llmHealth = (docker inspect --format='{{.State.Health.Status}}' aicm-e2e-llm-mock  2>$null) -join ""

    if ($pgHealth -eq "healthy" -and $llmHealth -eq "healthy") {
        Write-Info "All services are healthy."
        $healthy = $true
        break
    }

    if ($i -eq 30) {
        Write-Err "Services did not become healthy within 30 seconds. PG=$pgHealth, LLM=$llmHealth"
        exit 1
    }

    Start-Sleep -Seconds 1
}

# ── Run Go E2E tests ────────────────────────────────────────────────────────
Push-Location $BackendDir
try {
    Write-Info "Executing Go E2E tests..."

    $env:DATABASE_URL    = "postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable"
    $env:LLM_PROVIDER    = "local"
    $env:OLLAMA_BASE_URL = "http://localhost:11434"

    go test ./tests/... -v -count=1 -timeout 180s
    if ($LASTEXITCODE -ne 0) {
        Write-Err "E2E Tests Failed"
        exit 1
    }

    Write-Info "E2E Tests Passed!"
}
finally {
    Pop-Location
}
