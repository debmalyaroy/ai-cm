#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "=================================================="
echo " Starting AI Category Manager AWS Deployment...   "
echo "=================================================="

# 1. Update system and install dependencies
echo "[1/4] Installing system dependencies (Docker, Git)..."
sudo dnf update -y || sudo apt update -y
sudo dnf install docker git -y || sudo apt install docker.io git -y

# Enable and start docker
echo "Starting Docker service..."
sudo systemctl enable docker
sudo systemctl start docker

# Install docker-compose if not available natively
if ! docker compose version &> /dev/null; then
    echo "Installing Docker Compose plugin..."
    DOCKER_CONFIG=${DOCKER_CONFIG:-/usr/local/lib/docker}
    sudo mkdir -p $DOCKER_CONFIG/cli-plugins
    sudo curl -SL https://github.com/docker/compose/releases/download/v2.24.5/docker-compose-linux-x86_64 -o $DOCKER_CONFIG/cli-plugins/docker-compose
    sudo chmod +x $DOCKER_CONFIG/cli-plugins/docker-compose
fi

# Add current user to docker group
sudo usermod -aG docker $USER
echo "Docker installed successfully."

# 2. Setup production config
echo "[2/4] Setting up production configuration..."
if [ ! -f "config/config.prod.yaml" ]; then
    echo "Warning: config/config.prod.yaml not found. Please create it based on the deployment guide."
    exit 1
fi

# 3. Create docker-compose.prod.yml if it doesn't exist
echo "[3/4] Preparing docker-compose.prod.yml..."
cat << 'EOF' > docker-compose.prod.yml
version: '3.8'

services:
  # Using the pgvector image to support the memory vector DB natively
  db:
    image: pgvector/pgvector:15
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}
      POSTGRES_DB: ${POSTGRES_DB:-ai_cm}
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
    image: ${DOCKER_REGISTRY}/aicm-backend:latest
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
    image: ${DOCKER_REGISTRY}/aicm-frontend:latest
    ports:
      - "80:3000"
    environment:
      # Expose public IP dynamically or map appropriately
      - NEXT_PUBLIC_API_URL=http://localhost:8080
    restart: always

volumes:
  pgdata:
EOF

# 4. Spin up the containers
echo "[4/4] Pulling latest images and launching containers..."
sudo docker compose -f docker-compose.prod.yml pull
sudo docker compose -f docker-compose.prod.yml up -d

echo "=================================================="
echo " Deployment successful!                           "
echo " The application should now be accessible on Port 80."
echo "=================================================="
