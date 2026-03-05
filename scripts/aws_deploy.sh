#!/bin/bash
# =============================================================================
# AI-CM: EC2 Bootstrap + Deployment Script
# Run this ONCE on a fresh EC2 instance (Amazon Linux 2023 or Ubuntu 24.04).
# After bootstrap, use deploy_e2e.sh prod to redeploy on updates.
#
# Usage:
#   chmod +x aws_deploy.sh && ./aws_deploy.sh
# =============================================================================

set -e

echo "=================================================="
echo " AI-CM: EC2 Bootstrap & Deployment               "
echo "=================================================="

# ── Step 1: System update & dependencies ─────────────────────────────────────
echo "[1/5] Updating system and installing Docker + Git..."

if command -v dnf &> /dev/null; then
    # Amazon Linux 2023
    sudo dnf update -y
    sudo dnf install -y docker git
else
    # Ubuntu
    sudo apt-get update -y
    sudo apt-get install -y docker.io git
fi

sudo systemctl enable docker
sudo systemctl start docker
sudo usermod -aG docker "$USER"
echo "  ✓ Docker $(docker --version | awk '{print $3}' | tr -d ',')"

# Install docker compose plugin if not present
if ! docker compose version &> /dev/null; then
    echo "  Installing Docker Compose plugin..."
    DOCKER_CONFIG=${DOCKER_CONFIG:-/usr/local/lib/docker}
    sudo mkdir -p "$DOCKER_CONFIG/cli-plugins"
    sudo curl -fsSL \
        "https://github.com/docker/compose/releases/download/v2.27.0/docker-compose-linux-x86_64" \
        -o "$DOCKER_CONFIG/cli-plugins/docker-compose"
    sudo chmod +x "$DOCKER_CONFIG/cli-plugins/docker-compose"
fi
echo "  ✓ Docker Compose $(docker compose version --short)"

# ── Step 2: Swap space (required for Next.js build on t3.micro) ──────────────
echo "[2/5] Configuring 2GB swap space (needed for Next.js build)..."
if [ ! -f /swapfile ]; then
    sudo fallocate -l 2G /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
    echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
    echo "  ✓ 2GB swap created and enabled"
else
    echo "  ✓ Swap already configured"
fi

# ── Step 3: Clone or update repo ─────────────────────────────────────────────
echo "[3/5] Cloning / updating repository..."
REPO_URL="https://github.com/debmalyaroy/ai-cm.git"
APP_DIR="$HOME/ai-cm"

if [ -d "$APP_DIR/.git" ]; then
    echo "  Repo found. Pulling latest changes..."
    git -C "$APP_DIR" pull origin master
else
    git clone "$REPO_URL" "$APP_DIR"
fi
cd "$APP_DIR"
echo "  ✓ Repository ready at $APP_DIR"

# ── Step 4: Configure production environment ──────────────────────────────────
echo "[4/5] Configuring production environment..."
ENV_FILE="config/.env.prod"

if [ ! -f "$ENV_FILE" ]; then
    echo ""
    echo "  ⚠️  $ENV_FILE not found."
    echo "  Please fill in the following values and re-run this script:"
    echo ""
    echo "    DATABASE_URL=postgres://aicm:PASSWORD@RDS_ENDPOINT:5432/aicm?sslmode=require"
    echo "    AWS_REGION=us-east-1"
    echo "    NEXT_PUBLIC_API_URL=http://YOUR_ELASTIC_IP"
    echo ""
    echo "  You can create the file with:"
    echo "    cp config/.env.prod config/.env.prod  # edit the template"
    echo ""
    exit 1
fi

# Validate that required vars are set
check_var() {
    local val
    val=$(grep "^$1=" "$ENV_FILE" | cut -d= -f2-)
    if [ -z "$val" ] || [[ "$val" == *"your_"* ]] || [[ "$val" == *"CHANGE_ME"* ]]; then
        echo "  ❌ $ENV_FILE is missing a real value for: $1"
        echo "     Edit the file and replace placeholder values."
        exit 1
    fi
}
check_var "DATABASE_URL"
check_var "AWS_REGION"
check_var "NEXT_PUBLIC_API_URL"
echo "  ✓ Environment config validated"

# ── Step 5: Deploy ────────────────────────────────────────────────────────────
echo "[5/5] Building and starting containers (this may take 5-10 minutes)..."
echo "  Note: Re-login or run 'newgrp docker' if you get 'permission denied' errors."

# Apply docker group without needing to re-login
if ! groups | grep -q docker; then
    exec sg docker "bash -c 'cd $APP_DIR && ./scripts/deploy_e2e.sh prod'"
else
    ./scripts/deploy_e2e.sh prod
fi

echo ""
echo "=================================================="
echo " Bootstrap complete!                              "
echo " The app is now running on port 80.              "
echo "                                                  "
echo " Useful commands:                                 "
echo "   Logs:   docker compose -f infra/docker-compose.prod.yml logs -f"
echo "   Stop:   docker compose -f infra/docker-compose.prod.yml down"
echo "   Update: ./scripts/deploy_e2e.sh prod          "
echo "=================================================="
