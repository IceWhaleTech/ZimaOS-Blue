#!/bin/bash

# ZimaOS-Blue Development Script
# Usage: ./build.sh [command]
# Commands: start (default), server, web, build, clean, prd

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
COMMAND="${1:-prd}"

# Enable CGO by default for production-capable builds.
# Allow explicit override from environment when needed.
export CGO_ENABLED="${CGO_ENABLED:-1}"

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

# Run a binary. On macOS, TCC binds speech recognition authorization to the
# parent app's bundle ID. If not running inside a real terminal (e.g. launched
# from an IDE), re-launch in Terminal.app so TCC uses com.apple.Terminal.
run_binary() {
    local bin="$1"
    shift
    # Resolve to absolute path — Terminal.app opens in ~/ by default
    case "$bin" in
        /*) ;; # already absolute
        *)  bin="$(cd "$(dirname "$bin")" && pwd)/$(basename "$bin")" ;;
    esac
    if [ "$(uname -s)" = "Darwin" ] && ! is_real_terminal; then
        info "Not running in Terminal.app, re-launching via Terminal..."
        local cmd
        cmd="'$(echo "$bin" | sed "s/'/'\\\\''/g")'"
        for arg in "$@"; do
            cmd="$cmd '$(echo "$arg" | sed "s/'/'\\\\''/g")'"
        done
        osascript -e "tell application \"Terminal\"
            activate
            -- Reuse the frontmost window if one exists, otherwise do script creates a new one
            if (count of windows) > 0 then
                do script \"$cmd\" in front window
            else
                do script \"$cmd\"
            end if
        end tell"
        return
    fi
    exec "$bin" "$@"
}

# Check if we're running inside a real terminal (not an IDE integrated terminal).
is_real_terminal() {
    case "${TERM_PROGRAM:-}" in
        Apple_Terminal|iTerm.app|WarpTerminal|Alacritty|tmux) return 0 ;;
    esac
    case "${__CFBundleIdentifier:-}" in
        com.apple.Terminal|com.googlecode.iterm2|dev.warp.Warp-Stable) return 0 ;;
    esac
    return 1
}

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

# Trim web dist: remove pre-compressed files, stats, and samples not needed for local serving
trim_dist() {
    local dist_dir="$1"
    info "Trimming dist (removing .gz, .br, stats.html)..."
    find "$dist_dir" \( -name "*.gz" -o -name "*.br" -o -name "stats.html" \) -delete 2>/dev/null
    info "Dist trimmed to $(du -sh "$dist_dir" | cut -f1)"
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

# Start the Go server
start_server() {
    info "Starting Go server..."
    cd "$PROJECT_ROOT/server"

    local binary_path="./blue"
    if [ "$(uname -s)" = "Darwin" ]; then
        make build-bluecli
        binary_path="./bin/bluecli"
    else
        go build -tags 'fts5 espeak kokoro' -o blue ./cmd/blue
    fi
    success "Server built successfully"

    info "Starting server on http://localhost"
    run_binary "$binary_path"
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

    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  ZimaOS-Blue Development Environment${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
    echo -e "  Backend:  ${YELLOW}http://localhost${NC}"
    echo -e "  Frontend: ${YELLOW}http://localhost:3000${NC} (background)"
    echo ""
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

    # Start server in foreground
    info "Starting Go server (dev mode)..."
    cd "$PROJECT_ROOT/server"
    local binary_path="./blue"
    if [ "$(uname -s)" = "Darwin" ]; then
        make build-bluecli
        binary_path="./bin/bluecli"
    else
        go build -tags 'fts5 espeak kokoro' -o blue ./cmd/blue
    fi
    run_binary "$binary_path"
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

# Run the web security audit with optional allowlist support.
run_web_audit() {
    if [ -f "$PROJECT_ROOT/web/scripts/audit-ci.mjs" ]; then
        if [ -f "$PROJECT_ROOT/web/audit-allowlist.json" ]; then
            node scripts/audit-ci.mjs --omit=dev
        else
            node scripts/audit-ci.mjs --no-allowlist --omit=dev
        fi
    else
        npm audit --omit=dev
    fi
}

handle_web_audit_failure() {
    local audit_status="$1"

    case "$audit_status" in
        1)
            error "Production dependencies have vulnerabilities. Please fix them before building."
            ;;
        2)
            error "Unable to complete npm audit because the npm registry request failed. Check network access and retry."
            ;;
        *)
            error "npm audit failed unexpectedly (exit code: $audit_status)."
            ;;
    esac
}

# Build for production
build_all() {
    check_prereqs

    info "Building for production..."

    # Check production dependencies for vulnerabilities
    info "Checking production dependencies for vulnerabilities..."
    cd "$PROJECT_ROOT/web"
    if run_web_audit; then
        :
    else
        audit_status=$?
        handle_web_audit_failure "$audit_status"
        exit 1
    fi

    # Build third_party native libraries (FFI mode - shared libs)
    if [ ! -d "$PROJECT_ROOT/libs" ] || [ -z "$(ls -A $PROJECT_ROOT/libs 2>/dev/null)" ]; then
        info "Building shared libraries for FFI..."
        bash "$PROJECT_ROOT/scripts/build-libs.sh"
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
    trim_dist "$PROJECT_ROOT/web/dist"

    success "Production build complete!"
}

# Production run: build web, copy to server/internal/web/dist, start server (embedded frontend)
prd_run() {
    check_prereqs

    info "Production run: build web, copy to server/internal/web, start server..."

    # Check production dependencies for vulnerabilities
    info "Checking production dependencies for vulnerabilities..."
    cd "$PROJECT_ROOT/web"
    if run_web_audit; then
        :
    else
        audit_status=$?
        handle_web_audit_failure "$audit_status"
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
    trim_dist "$PROJECT_ROOT/web/dist"

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
    run_binary ./bin/blue
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

    # Clean embedded skills (build artifact)
    rm -rf "$PROJECT_ROOT/server/internal/skill/embedded/skills"

    success "Clean complete!"
}

# Main
case "$COMMAND" in
    start)
        start_all
        ;;
    server)
        check_prereqs
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
