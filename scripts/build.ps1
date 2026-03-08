<#
.SYNOPSIS
AI-CM: Build Script (Windows PowerShell)
Usage: .\scripts\build.ps1 [all|backend|frontend|docker] [-Target prod]

  all      - Build backend binary, frontend assets, and Docker images
  backend  - Build Go binary only (cross-compiled for Linux)
  frontend - Build Vite/React assets only
  docker   - Build Docker images only

  -Target local  (default) Build images locally, do NOT push to DockerHub
  -Target prod   Build, login to DockerHub using DOCKER_PAT from .env [prod.aws], and push
#>

param(
    [ValidateSet("all", "backend", "frontend", "docker")]
    [string]$Action = "all",

    [string]$Target = "local"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$EnvFile = Join-Path $RootDir ".env"

function Write-Info { param($msg) Write-Host "[INFO]  $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "[WARN]  $msg" -ForegroundColor Yellow }
function Write-Err  { param($msg) Write-Host "[ERROR] $msg" -ForegroundColor Red }

# Read key=value pairs from a specific INI-style section in the .env file.
# If -Section is omitted, reads all key=value pairs from the entire file.
function Parse-Env {
    param([string]$FilePath, [string]$Section = "")

    $inSection = ($Section -eq "")

    Get-Content $FilePath | ForEach-Object {
        $line = $_.Trim()

        if ([string]::IsNullOrWhiteSpace($line) -or $line.StartsWith("#")) { return }

        # Section header: [section.name]
        if ($line -match "^\[(.+)\]$") {
            if ($Section -ne "") { $inSection = ($Matches[1] -eq $Section) }
            return
        }

        if ($inSection -and $line -match "^([^#=]+)=(.*)$") {
            $key   = $Matches[1].Trim()
            $value = $Matches[2].Trim()
            $value = $value -replace "\s*#.*$", ""   # strip inline comments
            $value = $value -replace '^"|"$', ''     # strip double quotes
            $value = $value -replace "^'|'$", ''     # strip single quotes
            [Environment]::SetEnvironmentVariable($key, $value, "Process")
        }
    }
}

function Build-Backend {
    Write-Info "Building backend..."
    Push-Location (Join-Path $RootDir "src\backend")
    try {
        Write-Info "Downloading Go modules..."
        go mod download
        if ($LASTEXITCODE -ne 0) { Write-Err "go mod download failed"; exit 1 }

        Write-Info "Running unit tests..."
        $env:GOOS = ""; $env:GOARCH = ""; $env:CGO_ENABLED = ""
        go test -skip 'E2E|e2e|EndToEnd' ./... -count=1 -timeout 120s
        if ($LASTEXITCODE -ne 0) { Write-Err "Backend unit tests failed"; exit 1 }

        Write-Info "Generating Swagger docs..."
        $swag = Get-Command swag -ErrorAction SilentlyContinue
        if ($swag) {
            swag init -g cmd/server/main.go -d . --parseDependency --parseInternal
            if ($LASTEXITCODE -ne 0) { Write-Err "Swagger generation failed"; exit 1 }
        } else {
            Write-Warn "swag not found — skipping Swagger generation (install: go install github.com/swaggo/swag/cmd/swag@latest)"
        }

        Write-Info "Running golangci-lint..."
        $lint = Get-Command golangci-lint -ErrorAction SilentlyContinue
        if ($lint) {
            golangci-lint run --timeout=5m
            if ($LASTEXITCODE -ne 0) { Write-Err "golangci-lint failed"; exit 1 }
        } else {
            Write-Warn "golangci-lint not found — skipping lint (install: https://golangci-lint.run/usage/install/)"
        }

        $binDir = Join-Path $RootDir "bin"
        if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }

        Write-Info "Building Linux binary (amd64)..."
        $env:CGO_ENABLED = "0"
        $env:GOOS        = "linux"
        $env:GOARCH      = "amd64"
        go build -o (Join-Path $binDir "aicm-server-amd64") ./cmd/server
        if ($LASTEXITCODE -ne 0) { Write-Err "Backend build failed (amd64)"; exit 1 }

        Write-Info "Building Linux binary (arm64)..."
        $env:GOARCH = "arm64"
        go build -o (Join-Path $binDir "aicm-server-arm64") ./cmd/server
        if ($LASTEXITCODE -ne 0) { Write-Err "Backend build failed (arm64)"; exit 1 }

        Write-Info "Backend built: bin\aicm-server-amd64, bin\aicm-server-arm64"
    }
    finally {
        # Reset cross-compilation vars so subsequent commands (Docker, etc.) run natively
        $env:GOOS = ""; $env:GOARCH = ""; $env:CGO_ENABLED = ""
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
            if ($LASTEXITCODE -ne 0) { Write-Err "npm install failed"; exit 1 }
        }

        Write-Info "Running frontend tests..."
        npm test
        if ($LASTEXITCODE -ne 0) { Write-Err "Frontend tests failed"; exit 1 }

        Write-Info "Building Vite/React app..."
        npm run build
        if ($LASTEXITCODE -ne 0) { Write-Err "Frontend build failed"; exit 1 }

        Write-Info "Frontend built: src\apps\web\dist\"
    }
    finally {
        Pop-Location
    }
}

function Build-Docker {
    Write-Info "Building Docker images..."
    Push-Location $RootDir
    try {
        $registryUser = "localdev"
        $platforms    = "linux/amd64,linux/arm64"

        # Ensure a multi-arch buildx builder is available
        Write-Info "Setting up multi-arch buildx builder (aicm-builder)..."
        $prevEA = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $createOut = docker buildx create --name aicm-builder --driver docker-container --bootstrap 2>&1
        $createExit = $LASTEXITCODE
        $ErrorActionPreference = $prevEA
        if ($createExit -ne 0 -and ($createOut -notmatch "existing instance")) {
            Write-Err "Failed to create buildx builder: $createOut"; exit 1
        }
        docker buildx use aicm-builder

        if ($Target -eq "prod") {
            # Read only the [prod.aws] section for Docker credentials
            if (Test-Path $EnvFile) { Parse-Env -FilePath $EnvFile -Section "prod.aws" }

            if ([string]::IsNullOrEmpty($env:DOCKER_USERNAME)) {
                Write-Err "DOCKER_USERNAME must be set in [prod.aws] of .env to push to production"
                exit 1
            }

            # DOCKER_REGISTRY may differ from DOCKER_USERNAME (e.g., ECR URIs)
            $registryUser = if (-not [string]::IsNullOrEmpty($env:DOCKER_REGISTRY)) {
                $env:DOCKER_REGISTRY
            } else {
                $env:DOCKER_USERNAME
            }

            # Login: prefer PAT over password for better security
            if (-not [string]::IsNullOrEmpty($env:DOCKER_PAT)) {
                Write-Info "Logging into DockerHub using Personal Access Token..."
                $env:DOCKER_PAT | docker login -u $env:DOCKER_USERNAME --password-stdin
                if ($LASTEXITCODE -ne 0) { Write-Err "Docker login failed. Check DOCKER_PAT in .env [prod.aws]."; exit 1 }
            } elseif (-not [string]::IsNullOrEmpty($env:DOCKER_PASSWORD)) {
                Write-Info "Logging into DockerHub using Password..."
                $env:DOCKER_PASSWORD | docker login -u $env:DOCKER_USERNAME --password-stdin
                if ($LASTEXITCODE -ne 0) { Write-Err "Docker login failed."; exit 1 }
            } else {
                Write-Warn "DOCKER_PAT and DOCKER_PASSWORD are both unset. Push will likely fail."
            }
        }

        $backendTag  = "$registryUser/aicm-backend:latest"
        $frontendTag = "$registryUser/aicm-frontend:latest"

        if ($Target -eq "prod") {
            Write-Info "Building and pushing multi-arch backend image ($platforms): $backendTag"
            docker buildx build --platform $platforms -t $backendTag -f infra/Dockerfile.backend ./src/backend --push
            if ($LASTEXITCODE -ne 0) { Write-Err "Backend Docker build/push failed"; exit 1 }

            Write-Info "Building and pushing multi-arch frontend image ($platforms): $frontendTag"
            docker buildx build --platform $platforms -t $frontendTag -f infra/Dockerfile.frontend ./src/apps/web --push
            if ($LASTEXITCODE -ne 0) { Write-Err "Frontend Docker build/push failed"; exit 1 }

            Write-Info "Multi-arch images pushed: $backendTag and $frontendTag"
        } else {
            # Docker daemon cannot load a multi-arch image; detect native arch for local use
            $arch = if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
            $localPlatform = "linux/$arch"
            Write-Warn "Local build: loading native arch only ($localPlatform). Multi-arch requires a registry push."
            Write-Warn "To build and push multi-arch images: .\scripts\build.ps1 docker -Target prod"

            docker buildx build --platform $localPlatform -t $backendTag -f infra/Dockerfile.backend ./src/backend --load
            if ($LASTEXITCODE -ne 0) { Write-Err "Backend Docker build failed"; exit 1 }

            docker buildx build --platform $localPlatform -t $frontendTag -f infra/Dockerfile.frontend ./src/apps/web --load
            if ($LASTEXITCODE -ne 0) { Write-Err "Frontend Docker build failed"; exit 1 }

            Write-Warn "Images loaded locally (registry: $registryUser). NOT pushed to DockerHub."
            Write-Warn "To build AND push to DockerHub: .\scripts\build.ps1 all -Target prod"
        }

        Write-Info "Docker build complete."
    }
    finally {
        Pop-Location
    }
}

switch ($Action) {
    "backend"  { Build-Backend }
    "frontend" { Build-Frontend }
    "docker"   { Build-Docker }
    "all" {
        Build-Backend
        Build-Frontend
        Build-Docker
        Write-Info "All builds complete!"
    }
}
