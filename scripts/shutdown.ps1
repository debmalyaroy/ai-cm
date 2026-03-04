# =============================================================================
# AI-CM: Shutdown script (Windows PowerShell)
# Usage: .\scripts\shutdown.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"

Write-Host "🛑 Stopping AI-CM services..." -ForegroundColor Yellow

Push-Location $InfraDir
try {
    # Stop services including Ollama if the local-llm override is present
    $localLlmFile = Join-Path $InfraDir "docker-compose.local-llm.yml"
    if (Test-Path $localLlmFile) {
        docker compose -f docker-compose.yml -f docker-compose.local-llm.yml down --remove-orphans
    }
    else {
        docker compose down --remove-orphans
    }
}
catch {
    Write-Host "⚠️  docker compose down failed: $_" -ForegroundColor Yellow
}
finally {
    Pop-Location
}

# Kill any orphan backend/frontend processes
$backendProcs = Get-Process -Name "server" -ErrorAction SilentlyContinue
if ($backendProcs) {
    Write-Host "   Stopping orphan backend processes..." -ForegroundColor Gray
    $backendProcs | Stop-Process -Force
}

$nodeProcs = Get-Process -Name "node" -ErrorAction SilentlyContinue | Where-Object {
    $_.Path -and $_.Path -like "*ai-cm*"
}
if ($nodeProcs) {
    Write-Host "   Stopping orphan frontend processes..." -ForegroundColor Gray
    $nodeProcs | Stop-Process -Force
}

Write-Host ""
Write-Host "✅ AI-CM services stopped." -ForegroundColor Green
