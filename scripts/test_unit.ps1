<#
.SYNOPSIS
AI-CM: Run Unit Tests Only (no E2E, no external services needed)
Usage: .\scripts\test_unit.ps1
#>

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir   = Split-Path -Parent $ScriptDir

function Write-Info { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Err  { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red }

Write-Host "=================================================="
Write-Host " Running AI-CM Unit Tests                         "
Write-Host "=================================================="

# ── Backend unit tests ──────────────────────────────────────────────────────
Write-Host ""
Write-Info "[BACKEND] Running Go unit tests (skipping E2E)..."

Push-Location (Join-Path $RootDir "src\backend")
try {
    go test `
        -v `
        -coverprofile=coverage.out `
        -covermode=atomic `
        -skip 'E2E|e2e|EndToEnd' `
        ./... `
        -count=1 `
        -timeout 120s

    if ($LASTEXITCODE -ne 0) {
        Write-Err "[BACKEND] Go unit tests failed"
        exit 1
    }

    Write-Info "[BACKEND] Generating coverage report..."
    $coverSummary = go tool cover -func=coverage.out
    $coverSummary | Select-Object -Last 10 | ForEach-Object { Write-Host $_ }

    $totalLine = $coverSummary | Where-Object { $_ -match "^total:" }
    if ($totalLine -match '(\d+\.\d+)%') {
        $totalCoverage = [double]$Matches[1]
        Write-Host ""
        Write-Info "[BACKEND] Total coverage: ${totalCoverage}%"

        if ($totalCoverage -lt 80) {
            Write-Warn "[BACKEND] Coverage ${totalCoverage}% is below the recommended 80% threshold."
        } else {
            Write-Info "[BACKEND] Coverage ${totalCoverage}% meets the 80% threshold."
        }
    }
}
finally {
    Pop-Location
}

# ── Frontend unit tests ─────────────────────────────────────────────────────
Write-Host ""
Write-Info "[FRONTEND] Running frontend tests..."

Push-Location (Join-Path $RootDir "src\apps\web")
try {
    if (-not (Test-Path "node_modules")) {
        Write-Info "[FRONTEND] Installing npm dependencies..."
        npm ci
        if ($LASTEXITCODE -ne 0) { Write-Err "npm ci failed"; exit 1 }
    }

    npm test
    if ($LASTEXITCODE -ne 0) {
        Write-Err "[FRONTEND] Frontend tests failed"
        exit 1
    }
}
finally {
    Pop-Location
}

Write-Host ""
Write-Info "All unit tests passed!"
