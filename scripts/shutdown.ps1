# =============================================================================
# AI-CM: Shutdown script (Windows PowerShell)
# Usage: .\scripts\shutdown.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"

Write-Host "🛑 Stopping AI-CM services..." -ForegroundColor Yellow

try {
    # Force stop and remove all AI-CM containers explicitly
    $containers = @("aicm-frontend", "aicm-backend", "aicm-ollama", "aicm-postgres")
    foreach ($c in $containers) {
        docker rm -f $c 2>$null
    }
} catch {
    # Containers may not be running; that is fine
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

$ollamaProcs = Get-Process -Name "ollama" -ErrorAction SilentlyContinue
if ($ollamaProcs) {
    Write-Host "   Stopping native Ollama processes..." -ForegroundColor Gray
    $ollamaProcs | Stop-Process -Force
}

Write-Host ""
Write-Host "✅ AI-CM services stopped." -ForegroundColor Green
