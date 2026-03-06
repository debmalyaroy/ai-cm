<#
.SYNOPSIS
AI-CM: Run Applications locally for active development.
Usage: .\scripts\run.ps1 -Profile [local_llm|bedrock]
#>

param (
    [Parameter(Mandatory = $true)]
    [ValidateSet("local_llm", "bedrock")]
    [string]$Profile
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$InfraDir = Join-Path $RootDir "infra"
$EnvFile = Join-Path $RootDir ".env"

if (-not (Test-Path $EnvFile)) {
    Write-Host "[ERROR] Root .env file not found. Copy .env.example to .env and configure it." -ForegroundColor Red
    exit 1
}

function Parse-Env {
    param([string]$FilePath, [string]$Section)
    
    $inSection = $false
    $envHash = @{}

    Get-Content $FilePath | ForEach-Object {
        $line = $_.Trim()
        if ($line -match "^\[(.*)\]$") {
            if ($Matches[1] -eq $Section) { $inSection = $true }
            else { $inSection = $false }
        }
        elseif ($inSection -and $line -match "^([^#=]+)=(.*)$") {
            $key = $Matches[1].Trim()
            $value = $Matches[2].Trim()
            # Remove inline comments
            $value = $value -replace "\s*#.*$",""
            # Strip quotes
            $value = $value -replace '^"|"$', ''
            $value = $value -replace "^'|'$", ''
            
            $envHash[$key] = $value
            [Environment]::SetEnvironmentVariable($key, $value, "Process")
        }
    }
    return $envHash
}

Write-Host "=================================================="
Write-Host " Starting AI-CM Local Environment                 "
Write-Host " Profile: $Profile                                "
Write-Host "=================================================="

if ($Profile -eq "local_llm") {
    Write-Host "Parsing [local.local] keys..." -ForegroundColor Cyan
    Parse-Env -FilePath $EnvFile -Section "local.local"
    $ComposeFile = "docker-compose.local-llm.yml"
}
elseif ($Profile -eq "bedrock") {
    Write-Host "Parsing [local.aws] keys..." -ForegroundColor Cyan
    Parse-Env -FilePath $EnvFile -Section "local.aws"
    $ComposeFile = "docker-compose.bedrock.yml"
}

Push-Location $InfraDir
try {
    Write-Host "Building Docker images (locally)..." -ForegroundColor Yellow
    docker compose -f $ComposeFile build
    
    Write-Host "Starting containers..." -ForegroundColor Yellow
    docker compose -f $ComposeFile up -d

    if ($Profile -eq "local_llm") {
        Write-Host "Pulling required local LLM models..." -ForegroundColor Yellow
        docker exec aicm-ollama ollama pull llama3.2
        docker exec aicm-ollama ollama pull tinyllama
    }

    Write-Host ""
    Write-Host "✅ System started successfully!" -ForegroundColor Green
    Write-Host "   Frontend: http://localhost:3000"
    Write-Host "   Backend:  http://localhost:8080"
}
catch {
    Write-Host "Startup failed: $_" -ForegroundColor Red
    exit 1
}
finally {
    Pop-Location
}
