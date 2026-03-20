#!/usr/bin/env bash
set -euo pipefail

# Simple WoL Install Script
# Runs inside an LXC container to install Simple WoL via Docker.
# Idempotent: safe to re-run to pull the latest image and recreate the container.

APP_DIR="/opt/simple-wol"

echo "==> Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq

echo "==> Installing Docker..."
if ! command -v docker &>/dev/null; then
  curl -fsSL https://get.docker.com | sh
else
  echo "    Docker already installed, skipping."
fi

echo "==> Enabling Docker service..."
systemctl enable --now docker

echo "==> Setting up Simple WoL in ${APP_DIR}..."
mkdir -p "${APP_DIR}/data"

cat > "${APP_DIR}/docker-compose.yml" <<'COMPOSE'
services:
  simple-wol:
    image: ghcr.io/ajustinjames/simple-wol:latest
    container_name: simple-wol
    network_mode: host
    cap_add:
      - NET_RAW
    volumes:
      - ./data:/data
    environment:
      - PORT=8080
      - DATA_DIR=/data
    restart: unless-stopped
COMPOSE

echo "==> Pulling latest image and starting Simple WoL..."
cd "${APP_DIR}"
docker compose pull
docker compose up -d

echo ""
echo "============================================"
echo "  Simple WoL installation complete!"
echo "  Listening on port 8080"
echo "============================================"
