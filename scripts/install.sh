#!/bin/bash
set -e

# OpenClaw Manager Installer
# Usage: curl -sSL https://raw.githubusercontent.com/PetrouilFan/clawdock/refs/heads/main/scripts/install.sh | bash
# Supports: Debian/Ubuntu, Arch/Manjaro, Fedora/RHEL

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

warn() {
    echo "[!] $1" >&2
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
    else
        ID="unknown"
    fi
    case "$ID" in
        debian|ubuntu|linuxmint)
            PKG_MANAGER="apt"
            ;;
        arch|manjaro|endeavouros)
            PKG_MANAGER="pacman"
            ;;
        fedora|rhel|centos|rocky|almalinux)
            PKG_MANAGER="dnf"
            ;;
        opensuse*|sles)
            PKG_MANAGER="zypper"
            ;;
        *)
            PKG_MANAGER=""
            ;;
    esac
    OS="$ID"
    log "Detected OS: $OS (package manager: ${PKG_MANAGER:-none})"
}

check_curl() {
    if ! command -v curl &> /dev/null; then
        log "curl not found, installing..."
        case "$PKG_MANAGER" in
            apt) apt-get update && apt-get install -y curl ;;
            pacman) pacman -Sy --noconfirm curl ;;
            dnf) dnf install -y curl ;;
            zypper) zypper install -y curl ;;
            *) error "Cannot install curl - please install it manually" ;;
        esac
    fi
    log "curl available"
}

install_docker() {
    if command -v docker &> /dev/null; then
        log "Docker is already installed"
        return 0
    fi

    log "Installing Docker..."

    # Try get.docker.com first (works on most distros)
    if curl -fsSL https://get.docker.com -o /tmp/get-docker.sh 2>/dev/null; then
        sh /tmp/get-docker.sh 2>/dev/null || true
        rm -f /tmp/get-docker.sh
    fi

    # Fallback to OS package manager
    if ! command -v docker &> /dev/null; then
        case "$PKG_MANAGER" in
            apt)
                apt-get update
                apt-get install -y docker.io docker-compose-plugin
                ;;
            pacman)
                pacman -Sy --noconfirm docker docker-compose
                ;;
            dnf)
                dnf install -y docker docker-compose-plugin
                ;;
            zypper)
                zypper install -y docker docker-compose
                ;;
            *)
                error "Docker not found and cannot install automatically. Please install Docker first: https://docs.docker.com/get-docker/"
                ;;
        esac
    fi

    # Enable and start docker
    systemctl enable docker --now 2>/dev/null || true

    # Wait for docker to be ready
    for i in {1..30}; do
        if docker info &>/dev/null; then
            log "Docker is running"
            return 0
        fi
        sleep 1
    done

    error "Docker installed but daemon is not responding"
}

check_docker() {
    if ! command -v docker &> /dev/null; then
        install_docker
    fi

    if ! docker info &> /dev/null; then
        error "Docker daemon is not running. Please start Docker: sudo systemctl start docker"
    fi

    log "Docker is available and running"
}

check_dependencies() {
    log "Checking dependencies..."

    # Check for systemctl (systemd)
    if ! command -v systemctl &> /dev/null; then
        error "systemd is required but not found"
    fi

    # Check for openssl (for secret key generation)
    if ! command -v openssl &> /dev/null; then
        log "openssl not found, installing..."
        case "$PKG_MANAGER" in
            apt) apt-get update && apt-get install -y openssl ;;
            pacman) pacman -Sy --noconfirm openssl ;;
            dnf) dnf install -y openssl ;;
            zypper) zypper install -y openssl ;;
        esac
    fi

    # Check for useradd
    if ! command -v useradd &> /dev/null; then
        case "$PKG_MANAGER" in
            apt) apt-get update && apt-get install -y passwd ;;
            pacman) pacman -Sy --noconfirm shadow ;;
            dnf) dnf install -y shadow-utils ;;
        esac
    fi

    # Check for groupadd
    if ! command -v groupadd &> /dev/null; then
        case "$PKG_MANAGER" in
            apt) apt-get update && apt-get install -y passwd ;;
            pacman) pacman -Sy --noconfirm shadow ;;
            dnf) dnf install -y shadow-utils ;;
        esac
    fi

    log "All dependencies satisfied"
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
        ASSET_URL=$(curl -sSL "https://api.github.com/repos/PetrouilFan/clawdock/releases/latest" | grep -o '"browser_download_url": "[^"]*linux-amd64"' | cut -d'"' -f4)
    else
        ASSET_URL="https://github.com/PetrouilFan/clawdock/releases/download/${MANAGER_VERSION}/openclaw-manager-linux-amd64"
    fi

    if [ -z "$ASSET_URL" ]; then
        log "No release asset found, building from source..."

        # Always install Go 1.23 from official site to guarantee compatibility
        GO_VERSION=$(/usr/local/go/bin/go version 2>/dev/null | grep -oP 'go1\.\d+' || echo "none")
        if [[ "$GO_VERSION" != "go1.23"* && "$GO_VERSION" != "go1.24"* && "$GO_VERSION" != "go1.25"* && "$GO_VERSION" != "go1.26"* ]]; then
            log "Installing Go 1.23 (current: $GO_VERSION)..."
            curl -fsSL https://go.dev/dl/go1.23.6.linux-amd64.tar.gz -o /tmp/go.tar.gz || error "Failed to download Go"
            rm -rf /usr/local/go
            tar -C /usr/local -xzf /tmp/go.tar.gz || error "Failed to extract Go"
            rm /tmp/go.tar.gz
        fi

        export PATH=/usr/local/go/bin:$PATH
        log "Using Go: $(/usr/local/go/bin/go version)"

        if ! command -v make &> /dev/null; then
            log "make not found, installing..."
            case "$PKG_MANAGER" in
                apt) apt-get update && apt-get install -y make ;;
                pacman) pacman -Sy --noconfirm make ;;
                dnf) dnf install -y make ;;
                zypper) zypper install -y make ;;
            esac
        fi

        # Check for git
        if ! command -v git &> /dev/null; then
            log "git not found, installing..."
            case "$PKG_MANAGER" in
                apt) apt-get update && apt-get install -y git ;;
                pacman) pacman -Sy --noconfirm git ;;
                dnf) dnf install -y git ;;
                zypper) zypper install -y git ;;
            esac
        fi

        # Clone and build
        TMPDIR=$(mktemp -d)
        log "Cloning repository to $TMPDIR..."
        if ! git clone --depth 1 https://github.com/PetrouilFan/clawdock.git "$TMPDIR"; then
            cd / 2>/dev/null
            rm -rf "$TMPDIR" 2>/dev/null || true
            error "Failed to clone repository"
        fi
        if [ ! -d "$TMPDIR" ] || [ ! -f "$TMPDIR/Makefile" ]; then
            cd / 2>/dev/null
            rm -rf "$TMPDIR" 2>/dev/null || true
            error "Clone failed or Makefile not found"
        fi
        log "Building..."
        cd "$TMPDIR"
        export PATH=/usr/local/go/bin:$PATH
        log "Go version: $(/usr/local/go/bin/go version)"
        log "Building binary directly (not via make)..."
        log "Go version: $(/usr/local/go/bin/go version)"
        if ! /usr/local/go/bin/go build -mod=mod -o openclaw-manager ./cmd/server 2>&1; then
            cd / 2>/dev/null
            rm -rf "$TMPDIR" 2>/dev/null || true
            error "Build failed"
        fi
        ls -la openclaw-manager 2>&1 || error "Binary not found after build"
        if [ -f "$TMPDIR/openclaw-manager" ]; then
            mv "$TMPDIR/openclaw-manager" "$INSTALL_DIR/openclaw-manager"
        elif [ -f "$TMPDIR/openclaw-manager-linux-amd64" ]; then
            mv "$TMPDIR/openclaw-manager-linux-amd64" "$INSTALL_DIR/openclaw-manager"
        else
            cd / 2>/dev/null
            rm -rf "$TMPDIR" 2>/dev/null || true
            error "Build succeeded but binary not found"
        fi
        chmod +x "$INSTALL_DIR/openclaw-manager"
        cd /
        rm -rf "$TMPDIR"
        log "Built openclaw-manager from source"
    else
        curl -sSL "$ASSET_URL" -o "$INSTALL_DIR/openclaw-manager"
        chmod +x "$INSTALL_DIR/openclaw-manager"
        log "Downloaded to $INSTALL_DIR/openclaw-manager"
    fi
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
    check_curl
    check_dependencies
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
    log "Access the UI at: http://<your-ip>:${PORT}"
    log "Or on Tailscale: http://<tailscale-ip>:${PORT}"
    log ""
    log "To check status: systemctl status openclaw-manager"
    log "To view logs: journalctl -u openclaw-manager -f"
    log "=========================================="
}

main "$@"
