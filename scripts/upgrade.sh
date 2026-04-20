#!/bin/bash
set -e

# OpenClaw Manager Upgrader
# Usage: ./upgrade.sh [version]

set -e

MANAGER_VERSION="${1:-latest}"
INSTALL_DIR="/opt/openclaw-manager"
CONFIG_DIR="/etc/openclaw-manager"
PORT=$(grep "^  port:" "$CONFIG_DIR/config.yaml" 2>/dev/null | awk '{print $3}' || echo "11436")

log() {
    echo "[*] $1"
}

error() {
    echo "[!] $1" >&2
    exit 1
}

backup_config() {
    log "Backing up configuration"
    cp "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/config.yaml.bak"
    cp "$CONFIG_DIR/secret.key" "$CONFIG_DIR/secret.key.bak" 2>/dev/null || true
}

download_binary() {
    log "Downloading openclaw-manager ${MANAGER_VERSION}"

    BACKUP="$INSTALL_DIR/openclaw-manager.bak"
    cp "$INSTALL_DIR/openclaw-manager" "$BACKUP"

    if [ "$MANAGER_VERSION" = "latest" ]; then
        ASSET_URL=$(curl -sSL "https://api.github.com/repos/openclaw/manager/releases/latest" | grep -o '"browser_download_url": "[^"]*linux-amd64"' | cut -d'"' -f4)
    else
        ASSET_URL="https://github.com/openclaw/manager/releases/download/${MANAGER_VERSION}/openclaw-manager-linux-amd64"
    fi

    if [ -z "$ASSET_URL" ]; then
        error "Could not find release asset for ${MANAGER_VERSION}"
    fi

    curl -sSL "$ASSET_URL" -o "$INSTALL_DIR/openclaw-manager.new"
    chmod +x "$INSTALL_DIR/openclaw-manager.new"

    mv "$INSTALL_DIR/openclaw-manager.new" "$INSTALL_DIR/openclaw-manager"

    log "Downloaded to $INSTALL_DIR/openclaw-manager"
}

restart_service() {
    log "Restarting service"

    systemctl restart openclaw-manager.service

    for i in {1..30}; do
        if curl -s http://0.0.0.0:${PORT}/healthz &> /dev/null; then
            log "Service is healthy after upgrade"
            return 0
        fi
        sleep 1
    done

    error "Service failed to restart after upgrade"
}

rollback() {
    log "Rolling back to previous version"
    if [ -f "$INSTALL_DIR/openclaw-manager.bak" ]; then
        mv "$INSTALL_DIR/openclaw-manager.bak" "$INSTALL_DIR/openclaw-manager"
        systemctl restart openclaw-manager.service
    fi
}

main() {
    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        error "OpenClaw Manager does not appear to be installed"
    fi

    log "Starting OpenClaw Manager upgrade to ${MANAGER_VERSION}"

    trap 'rollback' ERR

    backup_config
    download_binary
    restart_service

    log ""
    log "=========================================="
    log "OpenClaw Manager upgraded successfully!"
    log "=========================================="
}

main "$@"
