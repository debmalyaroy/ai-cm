# =============================================================================
# AI-CM: Build Script (Windows PowerShell)
# Usage: .\scripts\build.ps1 [all|backend|frontend|docker|test]
# =============================================================================

param(
    [ValidateSet("all", "backend", "frontend", "docker", "test")]
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir   = Split-Path -Parent $ScriptDir

function Write-Info  { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn  { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Err   { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red }

function Build-Backend {
    Write-Info "Building backend..."
    Push-Location (Join-Path $RootDir "src\backend")
    try {
        Write-Info "Running unit tests..."
        go test ./internal/... -count=1
        if ($LASTEXITCODE -ne 0) { throw "Backend unit tests failed" }

        Write-Info "Building binary..."
        $env:CGO_ENABLED = "0"
        $binDir = Join-Path $RootDir "bin"
        if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }
        go build -o (Join-Path $binDir "aicm-server.exe") ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Backend build failed" }

        Write-Info "Backend built: bin\aicm-server.exe"
    }
    finally {
        Pop-Location
    }
}

function Build-Frontend {
    Write-Info "Building frontend..."
    Push-Location (Join-Path $RootDir "src\apps\web")
    try {
        if (-not (Test-Path "node_modules")) {
            Write-Info "Installing npm dependencies..."
            npm install
            if ($LASTEXITCODE -ne 0) { throw "npm install failed" }
        }

        Write-Info "Building Next.js app..."
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }

        Write-Info "Frontend built: src\apps\web\.next\"
    }
    finally {
        Pop-Location
    }
}

function Build-Docker {
    Write-Info "Building Docker images..."
    Push-Location $RootDir
    try {
        docker compose build
        if ($LASTEXITCODE -ne 0) { throw "Docker build failed" }
        Write-Info "Docker images built successfully"
    }
    finally {
        Pop-Location
    }
}

function Run-Tests {
    Write-Info "Running all backend tests..."
    Push-Location (Join-Path $RootDir "src\backend")
    try {
        go test ./... -v -count=1 -cover
        if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
    }
    finally {
        Pop-Location
    }
}

switch ($Target) {
    "backend"  { Build-Backend }
    "frontend" { Build-Frontend }
    "docker"   { Build-Docker }
    "test"     { Run-Tests }
    "all" {
        Build-Backend
        Build-Frontend
        Write-Info "All builds complete!"
    }
}
