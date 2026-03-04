# =============================================================================
# AI-CM: Start AI-CM with Local LLM (Ollama) on GPU
# Usage: .\scripts\run_local_llm.ps1
# Requires: 16GB RAM + RTX 4060 (or compatible GPU)
# =============================================================================

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"

Write-Host "Starting AI-CM with Local LLM (Llama 3.2)..." -ForegroundColor Green
Write-Host "This configuration is optimized for deep reasoning and SQL proficiency on 8GB VRAM." -ForegroundColor Cyan
Write-Host ""

Push-Location $InfraDir
try {
    # Start all services including Ollama in detached mode
    Write-Host "Pulling and starting containers..." -ForegroundColor Yellow
    docker compose -f docker-compose.yml -f docker-compose.local-llm.yml up --build -d

    # Wait for Ollama to be responsive
    Write-Host "Waiting for Ollama service to start..." -ForegroundColor Yellow
    $ollamaReady = $false
    for ($i = 0; $i -lt 30; $i++) {
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:11434/" -UseBasicParsing -ErrorAction SilentlyContinue 
            if ($response.StatusCode -eq 200) {
                $ollamaReady = $true
                break
            }
        }
        catch {
            # Ignore connection refused while starting up
        }
        Start-Sleep -Seconds 2
    }

    if (-not $ollamaReady) {
        Write-Host "[ERROR] Ollama didn't start in time. Check logs: docker compose logs ollama" -ForegroundColor Red
        exit 1
    }

    Write-Host "Downloading Local LLM (llama3.2)... This will take a moment." -ForegroundColor Yellow
    Write-Host "Llama 3.2 provides superior reasoning and Text-to-SQL logic handling over previous models." -ForegroundColor Cyan
    docker exec aicm-ollama ollama pull llama3.2

    Write-Host "Downloading tinyllama for fast intent classification (Supervisor Agent)..." -ForegroundColor Yellow
    docker exec aicm-ollama ollama pull tinyllama

}
catch {
    Write-Host "An error occurred: $_" -ForegroundColor Red
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "[OK] AI-CM is starting up with Local LLM!" -ForegroundColor Green
Write-Host "   Frontend: http://localhost:3000"
Write-Host "   Backend:  http://localhost:8080"
Write-Host "   LLM API:  http://localhost:11434"
Write-Host ""
Write-Host "View logs: docker compose -f infra/docker-compose.yml -f infra/docker-compose.local-llm.yml logs -f" -ForegroundColor Cyan
Write-Host "Stop: docker compose -f infra/docker-compose.yml -f infra/docker-compose.local-llm.yml down" -ForegroundColor Cyan
