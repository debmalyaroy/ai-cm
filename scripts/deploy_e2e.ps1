<#
.SYNOPSIS
AI-CM: End-to-End Deployment Wrapper for Windows PowerShell
Usage: .\scripts\deploy_e2e.ps1 -EnvTarget [local|prod]
#>

param (
    [Parameter(Mandatory = $true)]
    [ValidateSet("local", "prod")]
    [string]$EnvTarget
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"
$ConfigDir = Join-Path $RootDir "config"

if ($EnvTarget -eq "local") {
    Write-Host "Deploying Local Environment (DEBUG, Local LLM)..." -ForegroundColor Cyan
    $EnvFile = Join-Path $ConfigDir ".env.local"
    if (-not (Test-Path $EnvFile)) {
        Write-Host "Warning: $EnvFile not found. Proceeding with defaults." -ForegroundColor Yellow
    }

    Push-Location $InfraDir
    try {
        docker compose --env-file $EnvFile -f docker-compose.yml -f docker-compose.local-llm.yml up --build -d
        Write-Host "✅ Local deployment complete." -ForegroundColor Green
    }
    catch {
        Write-Host "Deployment failed: $_" -ForegroundColor Red
        exit 1
    }
    finally {
        Pop-Location
    }
}
elseif ($EnvTarget -eq "prod") {
    Write-Host "Deploying Production Environment (INFO, AWS Bedrock/OpenAI)..." -ForegroundColor Cyan
    $EnvFile = Join-Path $ConfigDir ".env.prod"
    if (-not (Test-Path $EnvFile)) {
        Write-Host "Error: $EnvFile is required for production. Copy .env.prod template and fill secrets." -ForegroundColor Red
        exit 1
    }

    Push-Location $InfraDir
    try {
        docker compose --env-file $EnvFile -f docker-compose.prod.yml pull
        docker compose --env-file $EnvFile -f docker-compose.prod.yml up --build -d
        Write-Host "✅ Production deployment complete. Backend listens on 8080." -ForegroundColor Green
    }
    catch {
        Write-Host "Deployment failed: $_" -ForegroundColor Red
        exit 1
    }
    finally {
        Pop-Location
    }
}
