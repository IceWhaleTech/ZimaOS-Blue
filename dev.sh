#!/bin/bash

# ZimaOS-Echo Development Script
# Usage: ./dev.sh [command]
# Commands: start (default), server, web, build, clean, prd

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
COMMAND="${1:-start}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info() { echo -e "${CYAN}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prereqs() {
    info "Checking prerequisites..."

    local missing=()

    if ! command_exists go; then
        missing+=("go (https://golang.org/dl/)")
    fi

    if ! command_exists node; then
        missing+=("node (https://nodejs.org/)")
    fi

    if ! command_exists npm; then
        missing+=("npm (comes with node)")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        error "Missing prerequisites:"
        for m in "${missing[@]}"; do
            echo "  - $m"
        done
        exit 1
    fi

    success "All prerequisites found"
}

# Install air for hot reload
install_air() {
    if ! command_exists air; then
        info "Installing air for hot reload..."
        go install github.com/air-verse/air@latest
        success "Air installed"
    fi
}

# Install dependencies
install_deps() {
    info "Installing dependencies..."

    # Go dependencies
    info "Installing Go dependencies..."
    cd "$PROJECT_ROOT/server"
    go mod download
    success "Go dependencies installed"

    # Node dependencies
    cd "$PROJECT_ROOT/web"
    if [ ! -d "node_modules" ]; then
        info "Installing Node dependencies..."
        npm install
    else
        info "Node modules already installed, skipping..."
    fi
    success "Node dependencies installed"
}

# Start the Go server with hot reload
start_server() {
    info "Starting Go server with hot reload..."
    cd "$PROJECT_ROOT/server"

    # Check if air is available
    if command_exists air; then
        info "Starting server with air (hot reload enabled)"
        info "Server will auto-restart when Go files change"
        air
    else
        warn "Air not installed, running without hot reload"
        warn "Run 'go install github.com/air-verse/air@latest' to enable hot reload"
        # Build first
        go build -o echo ./cmd/echo
        success "Server built successfully"

        info "Starting server on http://localhost:8080"
        ./echo
    fi
}

# Start the web dev server
start_web() {
    info "Starting web dev server..."
    cd "$PROJECT_ROOT/web"

    if [ ! -d "node_modules" ]; then
        info "Installing Node dependencies..."
        npm install
    fi

    info "Starting Vite dev server on http://localhost:3000"
    npm run dev
}

# Start both server and web
start_all() {
    check_prereqs
    install_deps
    install_air

    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  ZimaOS-Echo Development Environment${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    echo -e "  Backend:  ${YELLOW}http://localhost:8080${NC}"
    echo -e "  Frontend: ${YELLOW}http://localhost:3000${NC}"
    echo ""
    if command_exists air; then
        echo -e "  ${GREEN}Hot reload enabled for backend${NC}"
    fi
    echo -e "  Press ${YELLOW}Ctrl+C${NC} to stop all services"
    echo ""

    # Trap to cleanup background processes
    trap cleanup EXIT INT TERM

    # Start server in background with hot reload
    cd "$PROJECT_ROOT/server"
    if command_exists air; then
        air &
    else
        go build -o echo ./cmd/echo
        ./echo &
    fi
    SERVER_PID=$!

    # Give server time to start
    sleep 2

    # Start web in foreground
    cd "$PROJECT_ROOT/web"
    npm run dev
}

# Cleanup function
cleanup() {
    info "Stopping services..."

    # Kill server if running
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
    fi

    # Kill any remaining echo processes
    pkill -f "echo" 2>/dev/null || true

    # Kill air if running
    pkill -f "air" 2>/dev/null || true

    success "Services stopped"
}

# Build for production
build_all() {
    check_prereqs

    info "Building for production..."

    # Build server
    info "Building Go server..."
    cd "$PROJECT_ROOT/server"
    go build -ldflags="-s -w" -o echo ./cmd/echo
    success "Server built: server/echo"

    # Build web
    info "Building web frontend..."
    cd "$PROJECT_ROOT/web"
    if [ ! -d "node_modules" ]; then
        npm install
    fi
    npm run build
    success "Web built: web/dist/"

    success "Production build complete!"
}

# Production run: build web, copy to server/internal/web/dist, start server (embedded frontend)
prd_run() {
    check_prereqs

    info "Production run: build web, copy to server/internal/web, start server..."

    # Build web
    info "Building web frontend..."
    cd "$PROJECT_ROOT/web"
    if [ ! -d "node_modules" ]; then
        npm install
    fi
    npm run build
    success "Web built: web/dist/"

    # Copy web/dist to server/internal/web/dist
    info "Copying web build to server/internal/web/dist..."
    rm -rf "$PROJECT_ROOT/server/internal/web/dist"
    cp -r "$PROJECT_ROOT/web/dist" "$PROJECT_ROOT/server/internal/web/dist"
    success "Web assets copied to server/internal/web/dist"

    # Start server (production mode, no -tags dev, serves embedded frontend)
    info "Starting Go server (production mode, http://localhost:8080)..."
    cd "$PROJECT_ROOT/server"
    go run ./cmd/echo
}

# Clean build artifacts
clean_all() {
    info "Cleaning build artifacts..."

    # Clean server
    rm -f "$PROJECT_ROOT/server/echo"
    rm -rf "$PROJECT_ROOT/server/data"
    rm -rf "$PROJECT_ROOT/server/tmp"

    # Clean web
    rm -rf "$PROJECT_ROOT/web/dist"
    rm -rf "$PROJECT_ROOT/web/node_modules"
    rm -rf "$PROJECT_ROOT/server/internal/web/dist"

    success "Clean complete!"
}

# Main
case "$COMMAND" in
    start)
        start_all
        ;;
    server)
        check_prereqs
        install_air
        start_server
        ;;
    web)
        check_prereqs
        start_web
        ;;
    build)
        build_all
        ;;
    clean)
        clean_all
        ;;
    prd)
        prd_run
        ;;
    *)
        echo "Unknown command: $COMMAND"
        echo "Usage: $0 [start|server|web|build|clean|prd]"
        exit 1
        ;;
esac
