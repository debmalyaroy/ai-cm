<#
.SYNOPSIS
AI-CM: Run End-to-End Tests
Usage: .\scripts\test_e2e.ps1
#>

$ErrorActionPreference = "Stop"

Write-Host "=================================================="
Write-Host " Running AI-CM End-to-End Tests                   "
Write-Host "=================================================="

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$BackendDir = Join-Path $RootDir "src\backend"
$InfraDir = Join-Path $RootDir "infra"
$EnvFile = Join-Path $RootDir ".env"

if (-not (Test-Path $EnvFile)) {
    Write-Host "[ERROR] Root .env file not found. Copy .env.example to .env." -ForegroundColor Red
    exit 1
}

function Parse-Env {
    param([string]$FilePath, [string]$Section)
    $inSection = $false
    Get-Content $FilePath | ForEach-Object {
        $line = $_.Trim()
        if ($line -match "^\[(.*)\]$") {
            if ($Matches[1] -eq $Section) { $inSection = $true }
            else { $inSection = $false }
        }
        elseif ($inSection -and $line -match "^([^#=]+)=(.*)$") {
            $key = $Matches[1].Trim()
            $value = $Matches[2].Trim()
            $value = $value -replace "\s*#.*$",""
            $value = $value -replace '^"|"$', ''
            $value = $value -replace "^'|'$", ''
            [Environment]::SetEnvironmentVariable($key, $value, "Process")
        }
    }
}

Write-Host "Loading [local.local] secrets for E2E..." -ForegroundColor Cyan
Parse-Env -FilePath $EnvFile -Section "local.local"

Push-Location $InfraDir
try {
    Write-Host "Starting Postgres and Ollama locally..." -ForegroundColor Yellow
    docker compose -f docker-compose.local-llm.yml up -d postgres ollama
    
    Write-Host "Waiting 5 seconds for Ollama..." -ForegroundColor Yellow
    Start-Sleep -Seconds 5
    docker exec aicm-ollama ollama pull llama3.2
}
finally {
    Pop-Location
}

Push-Location $BackendDir
try {
    Write-Host "Executing Go E2E tests..." -ForegroundColor Cyan
    $env:LLM_PROVIDER = "local"
    go test ./tests/... -v -count=1 -timeout 120s
    Write-Host "✅ E2E Tests Passed!" -ForegroundColor Green
}
catch {
    Write-Host "❌ E2E Tests Failed: $_" -ForegroundColor Red
    exit 1
}
finally {
    Pop-Location
}
