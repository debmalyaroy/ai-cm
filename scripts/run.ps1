# =============================================================================
# AI-CM: One-command startup script (Windows PowerShell)
# Usage: .\scripts\run.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"

# Check for .env file
$EnvFile = Join-Path $InfraDir ".env"
$EnvExample = Join-Path $InfraDir ".env.example"

if (-not (Test-Path $EnvFile)) {
    Write-Host "⚠️  No .env file found. Copying from .env.example..." -ForegroundColor Yellow
    Copy-Item $EnvExample $EnvFile
    Write-Host "📝 Please edit infra\.env with your API keys, then re-run this script." -ForegroundColor Cyan
    exit 1
}

# Read .env file
$envVars = @{}
Get-Content $EnvFile | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
        $envVars[$matches[1].Trim()] = $matches[2].Trim()
    }
}

$provider = $envVars["LLM_PROVIDER"]

# Validate required API key
if ($provider -eq "gemini" -and ($envVars["GEMINI_API_KEY"] -eq "" -or $envVars["GEMINI_API_KEY"] -eq "your_gemini_api_key_here")) {
    Write-Host "❌ GEMINI_API_KEY is not set. Please edit infra\.env" -ForegroundColor Red
    exit 1
}
elseif ($provider -eq "openai" -and ($envVars["OPENAI_API_KEY"] -eq "" -or $envVars["OPENAI_API_KEY"] -eq "your_openai_api_key_here")) {
    Write-Host "❌ OPENAI_API_KEY is not set. Please edit infra\.env" -ForegroundColor Red
    exit 1
}
elseif ($provider -eq "aws") {
    if ($envVars["AWS_ACCESS_KEY_ID"] -eq "" -or $envVars["AWS_ACCESS_KEY_ID"] -eq "your_aws_access_key") {
        Write-Host "⚠️  AWS_ACCESS_KEY_ID is not set in infra\.env. Assuming IAM roles or ~/.aws/credentials are configured." -ForegroundColor Yellow
    }
}

Write-Host "🚀 Starting AI-CM (LLM Provider: $provider)..." -ForegroundColor Green

# Stop any existing services first
Write-Host "   Stopping existing services..." -ForegroundColor Gray
Push-Location $InfraDir
try {
    docker compose down --remove-orphans 2>$null
}
catch {
    # Ignore if nothing was running
}
finally {
    Pop-Location
}

Push-Location $InfraDir
try {
    docker compose --env-file .env up --build -d
}
finally {
    Pop-Location
}

$frontendPort = if ($envVars["FRONTEND_PORT"]) { $envVars["FRONTEND_PORT"] } else { "3000" }
$backendPort = if ($envVars["BACKEND_PORT"]) { $envVars["BACKEND_PORT"] } else { "8080" }

Write-Host ""
Write-Host "✅ AI-CM is starting up!" -ForegroundColor Green
Write-Host "   Frontend: http://localhost:$frontendPort"
Write-Host "   Backend:  http://localhost:$backendPort"
Write-Host ""
Write-Host "📋 View logs: docker compose -f infra/docker-compose.yml logs -f" -ForegroundColor Cyan
Write-Host "🛑 Stop:      docker compose -f infra/docker-compose.yml down" -ForegroundColor Cyan
