#!/bin/bash
set -e

# OpenClaw Manager Installer
# Usage: curl -sSL https://raw.githubusercontent.com/openclaw/manager/main/install.sh | bash

set -e

MANAGER_VERSION="${MANAGER_VERSION:-latest}"
INSTALL_DIR="/opt/openclaw-manager"
CONFIG_DIR="/etc/openclaw-manager"
DATA_DIR="/var/lib/openclaw-manager"
USER="openclaw-manager"
GROUP="openclaw-manager"
PORT="11436"

log() {
    echo "[*] $1"
}

error() {
    echo "[!] $1" >&2
    exit 1
}

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root"
    fi
}

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS="$ID"
    else
        OS="unknown"
    fi
}

check_docker() {
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker first: https://docs.docker.com/get-docker/"
    fi

    if ! docker info &> /dev/null; then
        error "Docker daemon is not running. Please start Docker."
    fi

    log "Docker is available"
}

create_user() {
    if ! id "$USER" &> /dev/null; then
        log "Creating user $USER"
        useradd --system --no-create-home --shell /usr/sbin/nologin "$USER" || true
    fi

    if ! getent group "$GROUP" &> /dev/null; then
        log "Creating group $GROUP"
        groupadd --system "$GROUP" || true
    fi

    usermod -aG docker "$USER" || true
}

create_dirs() {
    log "Creating directories"

    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR/backups"
    mkdir -p "$DATA_DIR/workspaces"
    mkdir -p "$DATA_DIR/agents"

    chown -R "$USER:$GROUP" "$DATA_DIR"
    chmod 755 "$INSTALL_DIR"
    chmod 755 "$CONFIG_DIR"
    chmod 755 "$DATA_DIR"
}

download_binary() {
    log "Downloading openclaw-manager ${MANAGER_VERSION}"

    if [ "$MANAGER_VERSION" = "latest" ]; then
        ASSET_URL=$(curl -sSL "https://api.github.com/repos/openclaw/manager/releases/latest" | grep -o '"browser_download_url": "[^"]*linux-amd64"' | cut -d'"' -f4)
    else
        ASSET_URL="https://github.com/openclaw/manager/releases/download/${MANAGER_VERSION}/openclaw-manager-linux-amd64"
    fi

    if [ -z "$ASSET_URL" ]; then
        error "Could not find release asset for ${MANAGER_VERSION}"
    fi

    curl -sSL "$ASSET_URL" -o "$INSTALL_DIR/openclaw-manager"
    chmod +x "$INSTALL_DIR/openclaw-manager"

    log "Downloaded to $INSTALL_DIR/openclaw-manager"
}

write_config() {
    log "Writing configuration"

    cat > "$CONFIG_DIR/config.yaml" << EOF
server:
  host: 0.0.0.0
  port: ${PORT}

database:
  path: ${DATA_DIR}/manager.db

security:
  require_auth: false
  secret_key_file: ${CONFIG_DIR}/secret.key

paths:
  data_dir: ${DATA_DIR}
  backup_dir: ${DATA_DIR}/backups

reconcile:
  interval_seconds: 30

agents:
  default_image_repo: ghcr.io/openclaw/openclaw
  default_restart_policy: unless-stopped
  default_workspace_container_path: /workspace
EOF

    # Generate secret key
    openssl rand -base64 32 > "$CONFIG_DIR/secret.key"
    chmod 600 "$CONFIG_DIR/secret.key"
    chown "$USER:$GROUP" "$CONFIG_DIR/secret.key"
}

write_systemd() {
    log "Installing systemd service"

    cat > /etc/systemd/system/openclaw-manager.service << EOF
[Unit]
Description=OpenClaw Manager
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
User=${USER}
Group=${GROUP}
ExecStart=${INSTALL_DIR}/openclaw-manager --config ${CONFIG_DIR}/config.yaml
WorkingDirectory=${INSTALL_DIR}
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=60
StartLimitBurst=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR}
ReadWritePaths=${CONFIG_DIR}

# Allow Docker socket access
Environment=DOCKER_HOST=unix:///var/run/docker.sock

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    log "Systemd service installed"
}

enable_service() {
    log "Enabling service"

    systemctl enable --now openclaw-manager.service

    # Wait for health check
    for i in {1..30}; do
        if curl -s http://0.0.0.0:${PORT}/healthz &> /dev/null; then
            log "Service is healthy"
            break
        fi
        sleep 1
    done
}

main() {
    log "Starting OpenClaw Manager installation"

    check_root
    detect_os
    check_docker
    create_user
    create_dirs
    download_binary
    write_config
    write_systemd
    enable_service

    log ""
    log "=========================================="
    log "OpenClaw Manager installed successfully!"
    log ""
    log "Access the UI at: http://127.0.0.1:${PORT}"
    log ""
    log "To check status: systemctl status openclaw-manager"
    log "To view logs: journalctl -u openclaw-manager -f"
    log "=========================================="
}

main "$@"
