# =============================================================================
# AI-CM: Deploy Script (Windows PowerShell)
# Usage: .\scripts\deploy.ps1 [dev|prod]
#   dev  - Start Postgres via Docker, then run backend and frontend natively
#   prod - Build everything and launch the full Docker Compose stack
# =============================================================================

param(
    [ValidateSet("dev", "prod")]
    [string]$Env = "dev"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir   = Split-Path -Parent $ScriptDir
$InfraDir  = Join-Path $RootDir "infra"

function Write-Info { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }

if ($Env -eq "dev") {
    Write-Info "Starting development environment..."

    # Start only the database container
    Write-Info "Starting PostgreSQL via Docker Compose..."
    Push-Location $InfraDir
    try {
        docker compose up postgres -d
        if ($LASTEXITCODE -ne 0) { throw "Failed to start postgres container" }
    }
    finally {
        Pop-Location
    }

    # Wait for postgres to accept connections
    Write-Info "Waiting for PostgreSQL to be ready..."
    $ready = $false
    for ($i = 0; $i -lt 20; $i++) {
        $check = Test-NetConnection -ComputerName localhost -Port 5432 -InformationLevel Quiet -WarningAction SilentlyContinue
        if ($check) { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        Write-Warn "PostgreSQL did not respond on port 5432 in time. Proceeding anyway."
    }

    # Launch backend in a new PowerShell window
    Write-Info "Starting backend (Go)..."
    $backendDir = Join-Path $RootDir "src\backend"
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$backendDir'; go run ./cmd/server" -WindowStyle Normal

    # Launch frontend in a new PowerShell window
    Write-Info "Starting frontend (Next.js)..."
    $frontendDir = Join-Path $RootDir "src\apps\web"
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$frontendDir'; npm run dev" -WindowStyle Normal

    Write-Host ""
    Write-Info "Development environment ready!"
    Write-Host "   Frontend: http://localhost:3000"
    Write-Host "   Backend:  http://localhost:8080"
    Write-Host ""
    Write-Host "Close the spawned terminal windows to stop the services." -ForegroundColor Cyan
    Write-Host "Stop database: docker compose -f infra/docker-compose.yml down" -ForegroundColor Cyan
}
elseif ($Env -eq "prod") {
    Write-Info "Building production images..."

    & (Join-Path $ScriptDir "build.ps1") -Target all

    Write-Info "Starting production stack via Docker Compose..."
    Push-Location $InfraDir
    try {
        docker compose -f docker-compose.prod.yml up -d --build
        if ($LASTEXITCODE -ne 0) { throw "Docker Compose up failed" }
    }
    finally {
        Pop-Location
    }

    Write-Host ""
    Write-Info "Production deployment complete!"
    Write-Host "   Application: http://localhost"
    Write-Host "   API:         http://localhost:8080"
}
