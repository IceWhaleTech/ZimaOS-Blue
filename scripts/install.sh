#!/bin/bash
set -e

# ZimaOS-Echo One-Click Installation Script
# Usage: curl -fsSL https://echo.zimaos.com/install.sh | bash
# Or: curl -fsSL https://echo.zimaos.com/install.sh | bash -s -- --version v0.1.0

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/opt/zimaos-echo}"
SERVICE_USER="${SERVICE_USER:-zimaos-echo}"
GITHUB_REPO="zimaos/echo"
BASE_URL="https://github.com/${GITHUB_REPO}/releases"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        --dir|-d)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version, -v    Version to install (default: latest)"
            echo "  --dir, -d        Installation directory (default: /opt/zimaos-echo)"
            echo "  --help, -h       Show this help message"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

# Detect OS and architecture
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="arm" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_banner() {
    echo -e "${BLUE}"
    echo "╔═══════════════════════════════════════════╗"
    echo "║         ZimaOS-Echo Installer             ║"
    echo "║     NAS-Native Agent Runtime              ║"
    echo "╚═══════════════════════════════════════════╝"
    echo -e "${NC}"
}

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Please run as root or with sudo"
        echo "  sudo bash -c \"\$(curl -fsSL https://echo.zimaos.com/install.sh)\""
        exit 1
    fi
}

check_dependencies() {
    for cmd in curl tar; do
        if ! command -v $cmd &> /dev/null; then
            log_error "$cmd is required but not installed"
            exit 1
        fi
    done
}

get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            VERSION="v0.1.0"
            log_warn "Could not fetch latest version, using $VERSION"
        fi
    fi
    log_info "Installing version: $VERSION"
}

create_user() {
    if ! id "$SERVICE_USER" &>/dev/null; then
        log_info "Creating service user: $SERVICE_USER"
        useradd -r -s /bin/false -d "$INSTALL_DIR" "$SERVICE_USER" 2>/dev/null || true
    fi
}

download_and_install() {
    local BINARY_NAME="echo-${OS}-${ARCH}"
    local DOWNLOAD_URL="${BASE_URL}/download/${VERSION}/${BINARY_NAME}.tar.gz"

    log_info "Downloading from: $DOWNLOAD_URL"

    mkdir -p "$INSTALL_DIR"/{bin,config,data,logs}

    # Download and extract
    curl -fsSL "$DOWNLOAD_URL" | tar -xz -C "$INSTALL_DIR/bin"
    chmod +x "$INSTALL_DIR/bin/echo"

    # Create default config if not exists
    if [ ! -f "$INSTALL_DIR/config/config.yaml" ]; then
        cat > "$INSTALL_DIR/config/config.yaml" << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"

log:
  level: "info"
  format: "json"
  output: "stdout"

worker:
  pool_size: 10
  max_queue_len: 100
EOF
    fi

    chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
    log_info "Installed to $INSTALL_DIR"
}

setup_systemd() {
    if ! command -v systemctl &> /dev/null; then
        log_warn "systemd not found, skipping service setup"
        return
    fi

    log_info "Setting up systemd service..."

    cat > /etc/systemd/system/zimaos-echo.service << EOF
[Unit]
Description=ZimaOS Echo - NAS-Native Agent Runtime
Documentation=https://docs.zimaos-echo.dev
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bin/echo --config $INSTALL_DIR/config/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$INSTALL_DIR/data $INSTALL_DIR/logs
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable zimaos-echo
    systemctl start zimaos-echo

    sleep 2
    if systemctl is-active --quiet zimaos-echo; then
        log_info "Service started successfully!"
    else
        log_warn "Service may not have started. Check: journalctl -u zimaos-echo"
    fi
}

print_success() {
    local IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")

    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Installation Complete!              ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════╝${NC}"
    echo ""
    echo "  Dashboard: http://${IP}:8080"
    echo "  Health:    http://${IP}:8080/health"
    echo ""
    echo "  Commands:"
    echo "    systemctl status zimaos-echo   - Check status"
    echo "    systemctl restart zimaos-echo  - Restart"
    echo "    journalctl -u zimaos-echo -f   - View logs"
    echo ""
    echo "  Config: $INSTALL_DIR/config/config.yaml"
    echo ""
}

main() {
    print_banner
    check_root
    check_dependencies
    get_latest_version
    create_user
    download_and_install
    setup_systemd
    print_success
}

main "$@"
