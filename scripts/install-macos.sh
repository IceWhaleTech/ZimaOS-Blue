#!/bin/bash
set -e

# ZimaOS-Echo macOS Installation Script
# Usage: curl -fsSL https://echo.zimaos.com/install-macos.sh | bash
# Or: curl -fsSL https://echo.zimaos.com/install-macos.sh | bash -s -- --version v0.1.0

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/zimaos-echo}"
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
            echo "Usage: install-macos.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version, -v    Version to install (default: latest)"
            echo "  --dir, -d        Installation directory (default: /usr/local/zimaos-echo)"
            echo "  --help, -h       Show this help message"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

# Detect architecture
ARCH=$(uname -m)
OS="darwin"

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
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
    echo "║     ZimaOS-Echo macOS Installer           ║"
    echo "║     NAS-Native Agent Runtime              ║"
    echo "╚═══════════════════════════════════════════╝"
    echo -e "${NC}"
}

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_macos() {
    if [[ "$(uname -s)" != "Darwin" ]]; then
        log_error "This script is for macOS only"
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

check_homebrew() {
    if command -v brew &> /dev/null; then
        log_info "Homebrew detected"
        return 0
    fi
    return 1
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

download_and_install() {
    local BINARY_NAME="echo-${OS}-${ARCH}"
    local DOWNLOAD_URL="${BASE_URL}/download/${VERSION}/${BINARY_NAME}.tar.gz"

    log_info "Downloading from: $DOWNLOAD_URL"

    # Create directories
    sudo mkdir -p "$INSTALL_DIR"/{bin,config,data,logs}

    # Download and extract
    curl -fsSL "$DOWNLOAD_URL" | sudo tar -xz -C "$INSTALL_DIR/bin"
    sudo chmod +x "$INSTALL_DIR/bin/echo"

    # Create default config if not exists
    if [ ! -f "$INSTALL_DIR/config/config.yaml" ]; then
        sudo tee "$INSTALL_DIR/config/config.yaml" > /dev/null << 'EOF'
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

    # Set ownership to current user
    sudo chown -R "$(whoami)" "$INSTALL_DIR"

    log_info "Installed to $INSTALL_DIR"
}

setup_launchd() {
    log_info "Setting up launchd service..."

    local PLIST_PATH="$HOME/Library/LaunchAgents/com.zimaos.echo.plist"
    mkdir -p "$HOME/Library/LaunchAgents"

    cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.zimaos.echo</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/bin/echo</string>
        <string>--config</string>
        <string>$INSTALL_DIR/config/config.yaml</string>
    </array>
    <key>WorkingDirectory</key>
    <string>$INSTALL_DIR</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>$INSTALL_DIR/logs/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>$INSTALL_DIR/logs/stderr.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
EOF

    # Load the service
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
    launchctl load "$PLIST_PATH"

    sleep 2
    if launchctl list | grep -q "com.zimaos.echo"; then
        log_info "Service started successfully!"
    else
        log_warn "Service may not have started. Check logs in $INSTALL_DIR/logs/"
    fi
}

create_symlink() {
    log_info "Creating symlink in /usr/local/bin..."
    sudo ln -sf "$INSTALL_DIR/bin/echo" /usr/local/bin/zimaos-echo
}

print_success() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Installation Complete!              ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════╝${NC}"
    echo ""
    echo "  Dashboard: http://localhost:8080"
    echo "  Health:    http://localhost:8080/health"
    echo ""
    echo "  Commands:"
    echo "    launchctl list | grep echo           - Check status"
    echo "    launchctl stop com.zimaos.echo       - Stop service"
    echo "    launchctl start com.zimaos.echo      - Start service"
    echo "    tail -f $INSTALL_DIR/logs/stdout.log - View logs"
    echo ""
    echo "  CLI: zimaos-echo --help"
    echo "  Config: $INSTALL_DIR/config/config.yaml"
    echo ""
}

uninstall() {
    log_info "Uninstalling ZimaOS-Echo..."

    # Stop and unload service
    launchctl unload "$HOME/Library/LaunchAgents/com.zimaos.echo.plist" 2>/dev/null || true
    rm -f "$HOME/Library/LaunchAgents/com.zimaos.echo.plist"

    # Remove symlink
    sudo rm -f /usr/local/bin/zimaos-echo

    # Remove installation directory
    if [ -d "$INSTALL_DIR" ]; then
        read -p "Remove $INSTALL_DIR and all data? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            sudo rm -rf "$INSTALL_DIR"
            log_info "Removed $INSTALL_DIR"
        else
            log_info "Kept $INSTALL_DIR"
        fi
    fi

    log_info "Uninstallation complete"
}

upgrade() {
    log_info "Upgrading ZimaOS-Echo..."

    # Stop service
    launchctl stop com.zimaos.echo 2>/dev/null || true

    # Download new version
    download_and_install

    # Restart service
    launchctl start com.zimaos.echo

    log_info "Upgrade complete"
}

main() {
    print_banner
    check_macos
    check_dependencies

    # Check for special commands
    case "${1:-}" in
        uninstall)
            uninstall
            exit 0
            ;;
        upgrade)
            get_latest_version
            upgrade
            exit 0
            ;;
    esac

    get_latest_version
    download_and_install
    setup_launchd
    create_symlink
    print_success
}

main "$@"
