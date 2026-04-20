#!/bin/bash
set -e

# OpenClaw Manager Uninstaller
# Usage: ./uninstall.sh [--keep-data]

set -e

KEEP_DATA=false
if [ "$1" = "--keep-data" ]; then
    KEEP_DATA=true
fi

INSTALL_DIR="/opt/openclaw-manager"
CONFIG_DIR="/etc/openclaw-manager"
DATA_DIR="/var/lib/openclaw-manager"
USER="openclaw-manager"
GROUP="openclaw-manager"

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

stop_service() {
    log "Stopping service"
    systemctl stop openclaw-manager.service 2>/dev/null || true
    systemctl disable openclaw-manager.service 2>/dev/null || true
}

remove_systemd() {
    log "Removing systemd service"
    rm -f /etc/systemd/system/openclaw-manager.service
    systemctl daemon-reload
}

remove_binary() {
    log "Removing binary"
    rm -f "$INSTALL_DIR/openclaw-manager" 2>/dev/null || true
    rmdir "$INSTALL_DIR" 2>/dev/null || true
}

remove_config() {
    if [ "$KEEP_DATA" = false ]; then
        log "Removing configuration"
        rm -rf "$CONFIG_DIR" 2>/dev/null || true
    else
        log "Keeping configuration (--keep-data)"
    fi
}

remove_data() {
    if [ "$KEEP_DATA" = false ]; then
        log "Removing data directory"
        rm -rf "$DATA_DIR" 2>/dev/null || true
    else
        log "Keeping data directory (--keep-data)"
    fi
}

remove_user() {
    if [ "$KEEP_DATA" = false ]; then
        log "Removing user and group"
        userdel "$USER" 2>/dev/null || true
        groupdel "$GROUP" 2>/dev/null || true
    fi
}

main() {
    log "Starting OpenClaw Manager uninstallation"

    check_root
    stop_service
    remove_systemd
    remove_binary
    remove_config
    remove_data
    remove_user

    log ""
    log "=========================================="
    log "OpenClaw Manager uninstalled!"
    if [ "$KEEP_DATA" = true ]; then
        log "Data preserved at: $DATA_DIR"
        log "Config preserved at: $CONFIG_DIR"
    fi
    log "=========================================="
}

main "$@"
