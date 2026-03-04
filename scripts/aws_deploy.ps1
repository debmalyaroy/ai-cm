<#
.SYNOPSIS
AI Category Manager AWS Deployment for Windows PowerShell.
#>

$ErrorActionPreference = "Stop"

Write-Host "=================================================="
Write-Host " Starting AI Category Manager AWS Deployment...   "
Write-Host "=================================================="

# Wait for manual installation of dependencies in a real PS script
Write-Host "[1/4] Dependency Check (Docker, Git)..."
if (-not (Get-Command "docker" -ErrorAction SilentlyContinue)) {
    Write-Host "Docker is not installed or not in PATH. Please install Docker Desktop." -ForegroundColor Red
    exit 1
}

if (-not (Get-Command "git" -ErrorAction SilentlyContinue)) {
    Write-Host "Git is not installed or not in PATH. Please install Git for Windows." -ForegroundColor Red
    exit 1
}

Write-Host "Dependencies verified."

# 2. Setup production config
Write-Host "`n[2/4] Setting up production configuration..."
if (-not (Test-Path "config\config.prod.yaml")) {
    Write-Host "Warning: config\config.prod.yaml not found. Please create it based on the deployment guide." -ForegroundColor Yellow
    exit 1
}

# 3. Create docker-compose.prod.yml if it doesn't exist
Write-Host "`n[3/4] Preparing docker-compose.prod.yml..."
$yamlContent = @"
version: '3.8'

services:
  db:
    image: pgvector/pgvector:15
    environment:
      POSTGRES_USER: `$`{POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: `$`{POSTGRES_PASSWORD:-postgres}
      POSTGRES_DB: `$`{POSTGRES_DB:-ai_cm}
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./infra/init-db.sh:/docker-entrypoint-initdb.d/init-db.sh
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  backend:
    image: `$`{DOCKER_REGISTRY}/aicm-backend:latest
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/app/config.prod.yaml
      - LLM_PROVIDER=aws
    volumes:
      - ./config/config.prod.yaml:/app/config.prod.yaml
      - ./prompts:/app/prompts
    depends_on:
      db:
        condition: service_healthy
    restart: always

  frontend:
    image: `$`{DOCKER_REGISTRY}/aicm-frontend:latest
    ports:
      - "80:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080
    restart: always

volumes:
  pgdata:
"@

$yamlContent | Out-File -FilePath "docker-compose.prod.yml" -Encoding UTF8

# 4. Spin up the containers
Write-Host "`n[4/4] Pulling latest images and launching containers..."
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d

Write-Host "==================================================" -ForegroundColor Green
Write-Host " Deployment successful!                           " -ForegroundColor Green
Write-Host " The application should now be accessible on Port 80." -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Green
