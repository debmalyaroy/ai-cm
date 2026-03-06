<#
.SYNOPSIS
AI-CM: AWS Production Deployment Script (Windows PowerShell)
Usage: .\scripts\deploy.ps1
#>

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"
$EnvFile = Join-Path $RootDir ".env"

if (-not (Test-Path $EnvFile)) {
    Write-Host "[ERROR] Root .env file not found. Production requires [prod.aws] profile." -ForegroundColor Red
    exit 1
}

function Parse-Env-Prod {
    param([string]$FilePath)
    
    $inSection = $false
    
    Get-Content $FilePath | ForEach-Object {
        $line = $_.Trim()
        if ($line -match "^\[(.*)\]$") {
            if ($Matches[1] -eq "prod.aws") { $inSection = $true }
            else { $inSection = $false }
        }
        elseif ($inSection -and $line -match "^([^#=]+)=(.*)$") {
            $key = $Matches[1].Trim()
            $value = $Matches[2].Trim()
            # Remove inline comments
            $value = $value -replace "\s*#.*$", ""
            # Strip quotes
            $value = $value -replace '^"|"$', ''
            $value = $value -replace "^'|'$", ''
            
            [Environment]::SetEnvironmentVariable($key, $value, "Process")
        }
    }
}

Write-Host "=================================================="
Write-Host " Deploying AI-CM to AWS Production                "
Write-Host "=================================================="

Write-Host "Parsing [prod.aws] keys..." -ForegroundColor Cyan
Parse-Env-Prod -FilePath $EnvFile

if ([string]::IsNullOrEmpty($env:DOCKER_REGISTRY)) {
    Write-Host "[ERROR] DOCKER_REGISTRY environment variable missing in [prod.aws]." -ForegroundColor Red
    exit 1
}

Push-Location $InfraDir
try {
    Write-Host "Pulling latest production images from $env:DOCKER_REGISTRY..." -ForegroundColor Yellow
    docker compose -f docker-compose.prod.yml pull
    
    Write-Host "Starting production containers..." -ForegroundColor Yellow
    docker compose -f docker-compose.prod.yml up -d

    Write-Host ""
    Write-Host "✅ Production deployment complete! Services are spinning up." -ForegroundColor Green
    Write-Host "   Frontend expecting traffic at: $($env:VITE_API_URL)"
}
catch {
    Write-Host "Deployment failed: $_" -ForegroundColor Red
    exit 1
}
finally {
    Pop-Location
}
