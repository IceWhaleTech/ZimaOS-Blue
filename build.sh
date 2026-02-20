#!/bin/bash

# ZimaOS-Blue Development Script
# Usage: ./build.sh [command]
# Commands: start (default), server, web, build, clean, prd

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
COMMAND="${1:-prd}"

# Disable CGO for FFI mode
export CGO_ENABLED=0

# Add Python user bin to PATH for edge-tts
export PATH="$PATH:$HOME/Library/Python/3.9/bin:$HOME/.local/bin"

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
        go build -tags 'fts5 espeak kokoro' -o blue ./cmd/blue
        success "Server built successfully"

        info "Starting server on http://localhost"
        ./blue
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
    echo -e "${CYAN}  ZimaOS-Blue Development Environment${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    echo -e "  Backend:  ${YELLOW}http://localhost${NC}"
    echo -e "  Frontend: ${YELLOW}http://localhost:3000${NC} (background)"
    echo ""
    if command_exists air; then
        echo -e "  ${GREEN}Hot reload enabled for backend${NC}"
    fi
    echo -e "  Press ${YELLOW}Ctrl+C${NC} to stop backend server"
    echo ""

    # Trap to cleanup background processes
    trap cleanup EXIT INT TERM

    # Start web in background (Go server proxies to it)
    info "Starting Vite dev server in background..."
    cd "$PROJECT_ROOT/web"
    npm run dev &
    WEB_PID=$!

    # Give Vite time to start
    sleep 3

    # Start server in foreground with hot reload
    info "Starting Go server (dev mode)..."
    cd "$PROJECT_ROOT/server"
    if command_exists air; then
        air
    else
        go build -tags 'fts5 espeak kokoro dev' -o blue ./cmd/blue
        ./blue
    fi
}

# Cleanup function
cleanup() {
    info "Stopping services..."

    # Kill web dev server if running
    if [ -n "$WEB_PID" ] && kill -0 "$WEB_PID" 2>/dev/null; then
        kill "$WEB_PID" 2>/dev/null || true
    fi

    # Kill any remaining vite processes
    pkill -f "vite" 2>/dev/null || true

    # Kill air if running
    pkill -f "air" 2>/dev/null || true

    success "Services stopped"
}

# Build third_party native libraries (espeak-ng, whisper.cpp, opus)
build_third_party() {
    info "Building third_party native libraries..."

    # Build espeak-ng
    if [ ! -f "$PROJECT_ROOT/third_party/espeak-ng/build/src/libespeak-ng/libespeak-ng.a" ]; then
        info "Building espeak-ng..."
        cd "$PROJECT_ROOT/third_party/espeak-ng"
        cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF
        cmake --build build --config Release -j"$(sysctl -n hw.ncpu 2>/dev/null || nproc)"
        success "espeak-ng built"
    else
        info "espeak-ng already built, skipping..."
    fi

    # Build libsonic.a from espeak-ng's compiled object if missing
    if [ ! -f "$PROJECT_ROOT/third_party/espeak-ng/build/libsonic.a" ] && \
       [ -f "$PROJECT_ROOT/third_party/espeak-ng/build/CMakeFiles/sonic.dir/_deps/sonic-git-src/sonic.c.o" ]; then
        info "Creating libsonic.a..."
        ar rcs "$PROJECT_ROOT/third_party/espeak-ng/build/libsonic.a" \
            "$PROJECT_ROOT/third_party/espeak-ng/build/CMakeFiles/sonic.dir/_deps/sonic-git-src/sonic.c.o"
        success "libsonic.a created"
    fi

    # Build whisper.cpp
    if [ ! -f "$PROJECT_ROOT/third_party/whisper.cpp/build/src/libwhisper.a" ]; then
        info "Building whisper.cpp..."
        cd "$PROJECT_ROOT/third_party/whisper.cpp"
        cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF
        cmake --build build --config Release -j"$(sysctl -n hw.ncpu 2>/dev/null || nproc)"
        success "whisper.cpp built"
    else
        info "whisper.cpp already built, skipping..."
    fi

    # Build opus
    if [ ! -f "$PROJECT_ROOT/third_party/opus-src/build/libopus.a" ]; then
        info "Building opus..."
        cd "$PROJECT_ROOT/third_party/opus-src"
        cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF
        cmake --build build --config Release -j"$(sysctl -n hw.ncpu 2>/dev/null || nproc)"
        success "opus built"
    else
        info "opus already built, skipping..."
    fi

    success "Third_party libraries ready"
}

# Code-sign macOS binary or .app bundle (skipped if APPLE_SIGNING_IDENTITY is not set)
codesign_binary() {
    local target="$1"
    if [ "$(uname -s)" != "Darwin" ]; then
        return
    fi
    if [ -z "$APPLE_SIGNING_IDENTITY" ]; then
        warn "APPLE_SIGNING_IDENTITY not set, skipping code signing"
        warn "macOS native speech recognition requires a signed binary"
        return
    fi
    info "Code-signing $target ..."
    local entitlements="$PROJECT_ROOT/tauri-app/src-tauri/entitlements.plist"
    local deep_flag=""
    if [[ "$target" == *.app ]]; then
        deep_flag="--deep"
    fi
    codesign --force $deep_flag --options runtime --sign "$APPLE_SIGNING_IDENTITY" \
        --entitlements "$entitlements" "$target"
    success "Binary signed: $(codesign -dv "$target" 2>&1 | head -1)"
}

# Build for production
build_all() {
    check_prereqs

    info "Building for production..."

    # Check production dependencies for vulnerabilities
    info "Checking production dependencies for vulnerabilities..."
    cd "$PROJECT_ROOT/web"
    if ! npm audit --omit=dev; then
        error "Production dependencies have vulnerabilities. Please fix them before building."
        exit 1
    fi

    # Build third_party native libraries (FFI mode - shared libs)
    if [ ! -d "$PROJECT_ROOT/libs" ] || [ -z "$(ls -A $PROJECT_ROOT/libs 2>/dev/null)" ]; then
        info "Building shared libraries for FFI..."
        "$PROJECT_ROOT/scripts/build-libs.sh"
    fi

    # Build server
    info "Building Go server..."
    cd "$PROJECT_ROOT/server"
    EXTRA_LDFLAGS=""
    if [ "$(uname -s)" = "Darwin" ]; then
        EXTRA_LDFLAGS="-extldflags '-sectcreate __TEXT __info_plist Info.plist'"
    fi
    go build -tags 'fts5 espeak kokoro' -ldflags="-s -w $EXTRA_LDFLAGS" -o blue ./cmd/blue

    # macOS: create .app bundle + codesign (TCC needs proper bundle for speech recognition)
    if [ "$(uname -s)" = "Darwin" ]; then
        info "Creating .app bundle..."
        APP_BUNDLE="$PROJECT_ROOT/server/Blue.app"
        rm -rf "$APP_BUNDLE"
        mkdir -p "$APP_BUNDLE/Contents/MacOS"
        mkdir -p "$APP_BUNDLE/Contents/Resources"
        cp "$PROJECT_ROOT/server/blue" "$APP_BUNDLE/Contents/MacOS/blue"
        cp "$PROJECT_ROOT/server/Info.plist" "$APP_BUNDLE/Contents/Info.plist"
        if [ -d "$PROJECT_ROOT/server/internal/web/dist" ]; then
            cp -r "$PROJECT_ROOT/server/internal/web/dist" "$APP_BUNDLE/Contents/Resources/dist"
        fi
        codesign_binary "$APP_BUNDLE"
    fi
    success "Server built: server/blue"

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

    # Check production dependencies for vulnerabilities
    info "Checking production dependencies for vulnerabilities..."
    cd "$PROJECT_ROOT/web"
    if ! npm audit --omit=dev; then
        error "Production dependencies have vulnerabilities. Please fix them before building."
        exit 1
    fi

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

    # Build server with pack-dist (signs before packing, appends web assets to binary)
    info "Building Go server (production mode)..."
    cd "$PROJECT_ROOT/server"
    make build
    success "Server built: server/bin/blue"

    # Run the built binary
    info "Starting server (production mode, http://localhost)..."
    ./bin/blue
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
