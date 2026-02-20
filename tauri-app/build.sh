#!/bin/bash
# ZimaOS Blue - Tauri Build Script
# This script builds the complete Tauri application package
#
# Build Strategies:
# - macOS: CGO library approach (Go static library linked into Rust)
# - Windows: Sidecar approach (Go binary as separate process)
#
# Compression Strategy:
# - DO NOT use UPX on binaries (causes slow startup)
# - Use LZMA compression on DMG for smaller download size

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TAURI_DIR="$SCRIPT_DIR/src-tauri"
LIB_DIR="$TAURI_DIR/lib"

# Version and build metadata
VERSION=$(cat "$PROJECT_ROOT/VERSION" 2>/dev/null || git -C "$PROJECT_ROOT" describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT=$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Trial license (Ed25519-signed, from environment)
TRIAL_LICENSE="${ZIMAOS_TRIAL_LICENSE:-}"

# Go ldflags — must match server/Makefile
GO_LDFLAGS="-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildTime=${BUILD_TIME}"
if [ -n "$TRIAL_LICENSE" ]; then
    GO_LDFLAGS="$GO_LDFLAGS -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=${TRIAL_LICENSE}"
fi

echo "=========================================="
echo "ZimaOS Blue - Tauri Build Script"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_step() {
    echo -e "${GREEN}[STEP]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect platform
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

echo "Platform: $GOOS-$GOARCH"

# Step 0: Install required dependencies (macOS only)
if [ "$GOOS" = "darwin" ]; then
    print_step "Checking and installing macOS dependencies..."

    # Check if Homebrew is installed
    if ! command -v brew &> /dev/null; then
        print_error "Homebrew is not installed. Please install it first: https://brew.sh"
        exit 1
    fi

    # Install Rust if not present
    if ! command -v cargo &> /dev/null; then
        print_step "Installing Rust..."
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
        source "$HOME/.cargo/env"
    fi

    # macOS uses native Speech framework — no espeak-ng/whisper.cpp needed

    # Install fileicon for setting DMG icon
    if ! command -v fileicon &> /dev/null; then
        print_step "Installing fileicon..."
        brew install fileicon
    fi

    # Install create-dmg for LZMA compression (optional)
    if ! command -v create-dmg &> /dev/null; then
        print_step "Installing create-dmg..."
        brew install create-dmg
    fi

    print_step "All macOS dependencies installed"
fi

# Step 1: Build frontend first (force fresh build)
print_step "Building frontend (fresh build)..."
cd "$PROJECT_ROOT/web"
# Clean previous build
rm -rf dist node_modules/.vite
npm install

# Check production dependencies for vulnerabilities
print_step "Checking production dependencies for vulnerabilities..."
if ! npm audit --omit=dev; then
    print_error "Production dependencies have vulnerabilities. Please fix them before building."
    exit 1
fi

npm run build

# Verify frontend build succeeded
if [ ! -d "$PROJECT_ROOT/web/dist" ] || [ -z "$(ls -A "$PROJECT_ROOT/web/dist")" ]; then
    print_error "Frontend build failed - dist directory is empty or missing"
    exit 1
fi

print_step "Frontend built successfully ($(ls -1 "$PROJECT_ROOT/web/dist" | wc -l | tr -d ' ') files)"

# Trim dist: remove pre-compressed files and stats (not needed for local serving)
print_step "Trimming frontend dist (removing .gz, .br, stats.html)..."
find "$PROJECT_ROOT/web/dist" \( -name "*.gz" -o -name "*.br" -o -name "stats.html" \) -delete 2>/dev/null
TRIMMED_SIZE=$(du -sh "$PROJECT_ROOT/web/dist" | cut -f1)
print_step "Frontend dist trimmed to $TRIMMED_SIZE"

# Step 2: Copy frontend to server/internal/web/dist for Go embedding
print_step "Copying frontend to server/internal/web/dist for Go embedding..."
EMBED_DIR="$PROJECT_ROOT/server/internal/web/dist"
rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR"
rsync -av --delete --exclude='*.map' "$PROJECT_ROOT/web/dist/" "$EMBED_DIR/"

# Verify copy succeeded
if [ ! -f "$EMBED_DIR/index.html" ]; then
    print_error "Failed to copy frontend to server/internal/web/dist"
    exit 1
fi

print_step "Frontend copied to server/internal/web/dist ($(ls -1 "$EMBED_DIR" | wc -l | tr -d ' ') files)"

# Step 4: Build backend (platform-specific)
if [ "$GOOS" = "darwin" ]; then
    # macOS: Build Go static library for CGO integration
    print_step "Building Go static library for macOS (CGO approach with embedded frontend)..."
    cd "$PROJECT_ROOT/server"

    mkdir -p "$LIB_DIR"

    # Verify embed directory exists before building
    if [ ! -f "$EMBED_DIR/index.html" ]; then
        print_error "Embed directory missing - frontend must be built first"
        exit 1
    fi

    # macOS uses native Speech framework — no espeak/kokoro/whisper needed
    CGO_ENABLED=1 go build -buildmode=c-archive \
        -tags 'fts5' \
        -ldflags="$GO_LDFLAGS" \
        -o "$LIB_DIR/libblue.a" \
        ./cmd/bluelib/

    print_step "Go library built: $LIB_DIR/libblue.a"
    ls -lh "$LIB_DIR/libblue.a"

    # Clean up embedded dist to avoid double-bundling (Tauri webview serves the frontend)
    rm -rf "$EMBED_DIR"
    print_step "Cleaned server/internal/web/dist (Tauri webview serves frontend)"

    print_step "macOS uses CGO library - no sidecar binary needed"
elif [ "$GOOS" = "windows" ]; then
    # Windows: Build Go static library for CGO integration (same as macOS)
    print_step "Building Go static library for Windows (CGO approach with embedded frontend)..."
    cd "$PROJECT_ROOT/server"

    mkdir -p "$LIB_DIR"

    if [ ! -f "$EMBED_DIR/index.html" ]; then
        print_error "Embed directory missing - frontend must be built first"
        exit 1
    fi

    # Build espeak-ng if not already built
    ESPEAK_DIR="$PROJECT_ROOT/third_party/espeak-ng"
    if [ ! -f "$ESPEAK_DIR/build/src/libespeak-ng/libespeak-ng.a" ]; then
        print_step "Building espeak-ng from source..."
        cd "$ESPEAK_DIR"
        mkdir -p build && cd build
        cmake .. -DBUILD_SHARED_LIBS=OFF -G "MinGW Makefiles"
        cmake --build . --config Release
        cd "$PROJECT_ROOT/server"
    fi

    # Windows needs espeak + kokoro for TTS
    CGO_ENABLED=1 go build -buildmode=c-archive \
        -tags 'fts5 espeak kokoro' \
        -ldflags="$GO_LDFLAGS" \
        -o "$LIB_DIR/libblue.a" \
        ./cmd/bluelib/

    print_step "Go library built: $LIB_DIR/libblue.a"
    ls -lh "$LIB_DIR/libblue.a"

    rm -rf "$EMBED_DIR"
    print_step "Cleaned server/internal/web/dist (Tauri webview serves frontend)"

    print_step "Windows uses CGO library - no sidecar binary needed"
else
    # Linux: Build Go sidecar binary
    print_step "Building Go sidecar (with embedded frontend)..."
    cd "$PROJECT_ROOT/server"

    case "$GOOS-$GOARCH" in
        linux-amd64)
            TARGET="x86_64-unknown-linux-gnu"
            ;;
        linux-arm64)
            TARGET="aarch64-unknown-linux-gnu"
            ;;
        *)
            print_error "Unsupported platform: $GOOS-$GOARCH"
            exit 1
            ;;
    esac

    SIDECAR_NAME="blue-server-$TARGET"

    # Build with optimizations (NO UPX compression!)
    CGO_ENABLED=0 go build -tags 'fts5 kokoro' -ldflags="$GO_LDFLAGS" -o "$TAURI_DIR/binaries/$SIDECAR_NAME" ./cmd/blue/

    # Also copy to bin directory for resources bundling
    mkdir -p "$TAURI_DIR/bin"
    cp "$TAURI_DIR/binaries/$SIDECAR_NAME" "$TAURI_DIR/bin/"

    print_step "Sidecar built: $SIDECAR_NAME (no UPX compression)"
fi

# Step 4b: Clean up data directory before build
print_step "Cleaning up data directory..."
rm -rf "$TAURI_DIR/data"
mkdir -p "$TAURI_DIR/data"

# Step 5: Clean Cargo cache (ensures icon/config changes take effect)
#print_step "Cleaning Cargo cache..."
#cd "$TAURI_DIR"
#cargo clean

# Step 6: Build Tauri app
print_step "Building Tauri application..."
cd "$SCRIPT_DIR"
npm install

# Check production dependencies for vulnerabilities
print_step "Checking Tauri app production dependencies for vulnerabilities..."
if ! npm audit --omit=dev; then
    print_error "Tauri app production dependencies have vulnerabilities. Please fix them before building."
    exit 1
fi

npm run build

# Step 7: Post-build processing (macOS only)
if [ "$GOOS" = "darwin" ]; then
    print_step "Post-processing macOS build..."

    # Find the built app and DMG
    APP_DIR="$TAURI_DIR/target/release/bundle/macos"
    DMG_DIR="$TAURI_DIR/target/release/bundle/dmg"

    # Get app name from tauri.conf.json
    APP_NAME="ZimaOS Blue"

    # ── Copy web dist into .app Resources ──
    RESOURCES_DIR="$APP_DIR/$APP_NAME.app/Contents/Resources"
    RESOURCES_DIST="$RESOURCES_DIR/dist"
    if [ -d "$EMBED_DIR" ] && [ -f "$EMBED_DIR/index.html" ]; then
        print_step "Copying web dist into .app Resources..."
        rm -rf "$RESOURCES_DIST"
        mkdir -p "$RESOURCES_DIST"
        rsync -a "$EMBED_DIR/" "$RESOURCES_DIST/"
        print_step "Web dist copied to $RESOURCES_DIST ($(ls -1 "$RESOURCES_DIST" | wc -l | tr -d ' ') files)"
    else
        print_warning "Embed dist not found at $EMBED_DIR — .app will not have web UI"
    fi

    # ── macOS Code Signing ──
    # Requires: APPLE_SIGNING_IDENTITY env var (e.g. "Developer ID Application: Your Name (TEAMID)")
    if [ -n "$APPLE_SIGNING_IDENTITY" ]; then
        print_step "Code signing macOS app with identity: $APPLE_SIGNING_IDENTITY"

        # Sign all binaries inside the .app bundle (deep sign)
        codesign --force --options runtime --deep \
            --sign "$APPLE_SIGNING_IDENTITY" \
            --timestamp \
            "$APP_DIR/$APP_NAME.app"

        # Verify signature
        codesign --verify --verbose=2 "$APP_DIR/$APP_NAME.app"
        print_step "macOS app signed and verified"
    else
        print_warning "APPLE_SIGNING_IDENTITY not set — skipping code signing"
        print_warning "Set it to sign: export APPLE_SIGNING_IDENTITY='Developer ID Application: Your Name (TEAMID)'"
    fi

    # Find existing DMG
    EXISTING_DMG=$(find "$DMG_DIR" -name "*.dmg" -type f 2>/dev/null | head -1)

    if [ -n "$EXISTING_DMG" ] && [ -f "$EXISTING_DMG" ]; then
        # Check if create-dmg is available for LZMA compression
        if command -v create-dmg &> /dev/null; then
            print_step "Re-creating DMG with LZMA compression..."

            # Get version from existing DMG name
            DMG_BASENAME=$(basename "$EXISTING_DMG")

            # Create new DMG with LZMA compression
            TEMP_DMG="$DMG_DIR/temp_lzma.dmg"

            create-dmg \
                --volname "$APP_NAME" \
                --window-pos 200 120 \
                --window-size 660 400 \
                --icon-size 100 \
                --icon "$APP_NAME.app" 180 170 \
                --hide-extension "$APP_NAME.app" \
                --app-drop-link 480 170 \
                --format ULMO \
                "$TEMP_DMG" \
                "$APP_DIR/$APP_NAME.app" 2>/dev/null || true

            if [ -f "$TEMP_DMG" ]; then
                # Replace original DMG with LZMA compressed version
                mv "$TEMP_DMG" "$EXISTING_DMG"
                print_step "DMG re-compressed with LZMA"
            else
                print_warning "LZMA compression failed, keeping original DMG"
            fi
        else
            print_warning "create-dmg not found. Install with: brew install create-dmg"
            print_warning "DMG was built but not LZMA compressed"
        fi

        # ── Sign DMG ──
        if [ -n "$APPLE_SIGNING_IDENTITY" ]; then
            print_step "Signing DMG..."
            codesign --force --sign "$APPLE_SIGNING_IDENTITY" --timestamp "$EXISTING_DMG"
            codesign --verify --verbose "$EXISTING_DMG"
            print_step "DMG signed"
        fi

        # ── Notarize DMG ──
        if [ -n "$APPLE_SIGNING_IDENTITY" ] && [ -n "$APPLE_ID" ] && [ -n "$APPLE_TEAM_ID" ]; then
            print_step "Submitting DMG for Apple notarization..."
            xcrun notarytool submit "$EXISTING_DMG" \
                --apple-id "$APPLE_ID" \
                --team-id "$APPLE_TEAM_ID" \
                --password "$APPLE_APP_PASSWORD" \
                --wait

            # Staple the notarization ticket
            xcrun stapler staple "$EXISTING_DMG"
            print_step "DMG notarized and stapled"
        else
            print_warning "Skipping notarization — set APPLE_ID, APPLE_TEAM_ID, APPLE_APP_PASSWORD"
        fi

        # Set DMG file icon
        ICON_FILE="$TAURI_DIR/icons/icon.icns"
        if command -v fileicon &> /dev/null; then
            fileicon set "$EXISTING_DMG" "$ICON_FILE"
            print_step "DMG file icon set successfully"
        else
            print_warning "fileicon tool not found. Install with: brew install fileicon"
        fi
    else
        print_warning "DMG file not found in $DMG_DIR"
    fi
fi

# Step 6b: Post-build processing (Windows only)
if [ "$GOOS" = "windows" ]; then
    # ── Windows Code Signing ──
    # Requires: WINDOWS_CERTIFICATE_THUMBPRINT env var
    if [ -n "$WINDOWS_CERTIFICATE_THUMBPRINT" ]; then
        print_step "Code signing Windows binaries..."

        NSIS_DIR="$TAURI_DIR/target/release/bundle/nsis"

        # Sign the main binary
        MAIN_EXE="$TAURI_DIR/target/release/zimaos-blue.exe"
        if [ -f "$MAIN_EXE" ]; then
            signtool sign /sha1 "$WINDOWS_CERTIFICATE_THUMBPRINT" /fd sha256 \
                /tr http://timestamp.digicert.com /td sha256 "$MAIN_EXE"
            print_step "Main binary signed"
        fi

        # Sign the NSIS installer
        NSIS_EXE=$(find "$NSIS_DIR" -name "*.exe" -type f 2>/dev/null | head -1)
        if [ -n "$NSIS_EXE" ] && [ -f "$NSIS_EXE" ]; then
            signtool sign /sha1 "$WINDOWS_CERTIFICATE_THUMBPRINT" /fd sha256 \
                /tr http://timestamp.digicert.com /td sha256 "$NSIS_EXE"
            print_step "NSIS installer signed"
        fi
    else
        print_warning "WINDOWS_CERTIFICATE_THUMBPRINT not set — skipping code signing"
    fi
fi

# Step 8: Print build results
echo ""
echo "=========================================="
echo "Build Complete!"
echo "=========================================="

# List output files
if [ "$GOOS" = "darwin" ]; then
    APP_PATH="$TAURI_DIR/target/release/bundle/macos"
    DMG_PATH="$TAURI_DIR/target/release/bundle/dmg"

    echo ""
    echo "Build Strategy: CGO Library (Go linked into Rust)"
    echo "Compression: LZMA on DMG (no UPX on binary)"
    echo ""
    echo "Output files:"
    if [ -d "$APP_PATH" ]; then
        echo "  App: $APP_PATH"
        ls -la "$APP_PATH"/*.app 2>/dev/null || true
    fi
    if [ -d "$DMG_PATH" ]; then
        echo ""
        echo "  DMG: $DMG_PATH"
        ls -la "$DMG_PATH"/*.dmg 2>/dev/null || true
    fi
elif [ "$GOOS" = "windows" ]; then
    echo ""
    echo "Build Strategy: Sidecar Process"
    echo "Compression: None on binary (installer handles compression)"
    echo ""
    echo "Output files:"
    ls -la "$TAURI_DIR/target/release/bundle/nsis/"*.exe 2>/dev/null || true
    ls -la "$TAURI_DIR/target/release/bundle/msi/"*.msi 2>/dev/null || true
elif [ "$GOOS" = "linux" ]; then
    echo ""
    echo "Build Strategy: Sidecar Process"
    echo ""
    echo "Output files:"
    ls -la "$TAURI_DIR/target/release/bundle/appimage/"*.AppImage 2>/dev/null || true
    ls -la "$TAURI_DIR/target/release/bundle/deb/"*.deb 2>/dev/null || true
fi

echo ""
echo "Done!"
