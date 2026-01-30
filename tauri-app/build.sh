#!/bin/bash
# ZimaOS Echo - Tauri Build Script
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

echo "=========================================="
echo "ZimaOS Echo - Tauri Build Script"
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

# Step 1: Clean and prepare embed directory
print_step "Cleaning embed directory..."
EMBED_DIR="$PROJECT_ROOT/server/internal/web/dist"
rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR"

# Step 2: Build frontend (force fresh build)
print_step "Building frontend (fresh build)..."
cd "$PROJECT_ROOT/web"
# Clean previous build
rm -rf dist node_modules/.vite
npm install
npm run build

# Verify frontend build succeeded
if [ ! -d "$PROJECT_ROOT/web/dist" ] || [ -z "$(ls -A "$PROJECT_ROOT/web/dist")" ]; then
    print_error "Frontend build failed - dist directory is empty or missing"
    exit 1
fi

# Step 3: Copy frontend to embed directory (excluding source maps)
print_step "Copying frontend to embed directory..."
rsync -av --delete --exclude='*.map' "$PROJECT_ROOT/web/dist/" "$EMBED_DIR/"

# Verify copy succeeded
if [ ! -f "$EMBED_DIR/index.html" ]; then
    print_error "Failed to copy frontend to embed directory"
    exit 1
fi

print_step "Frontend embedded successfully ($(ls -1 "$EMBED_DIR" | wc -l | tr -d ' ') files)"

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

    # Build for current architecture
    CGO_ENABLED=1 go build -buildmode=c-archive \
        -ldflags="-s -w" \
        -o "$LIB_DIR/libecho.a" \
        ./cmd/echolib/

    print_step "Go library built: $LIB_DIR/libecho.a"
    ls -lh "$LIB_DIR/libecho.a"

    # Note: No sidecar needed for macOS
    print_step "macOS uses CGO library - no sidecar binary needed"
else
    # Windows/Linux: Build Go sidecar binary
    print_step "Building Go sidecar (with embedded frontend)..."
    cd "$PROJECT_ROOT/server"

    case "$GOOS-$GOARCH" in
        linux-amd64)
            TARGET="x86_64-unknown-linux-gnu"
            ;;
        linux-arm64)
            TARGET="aarch64-unknown-linux-gnu"
            ;;
        windows-amd64)
            TARGET="x86_64-pc-windows-msvc"
            ;;
        *)
            print_error "Unsupported platform: $GOOS-$GOARCH"
            exit 1
            ;;
    esac

    SIDECAR_NAME="echo-server-$TARGET"
    if [ "$GOOS" = "windows" ]; then
        SIDECAR_NAME="$SIDECAR_NAME.exe"
    fi

    # Build with optimizations (NO UPX compression!)
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TAURI_DIR/binaries/$SIDECAR_NAME" ./cmd/echo/

    # Also copy to bin directory for resources bundling
    mkdir -p "$TAURI_DIR/bin"
    cp "$TAURI_DIR/binaries/$SIDECAR_NAME" "$TAURI_DIR/bin/"

    print_step "Sidecar built: $SIDECAR_NAME (no UPX compression)"
fi

# Step 4: Clean up data directory before build
print_step "Cleaning up data directory..."
rm -rf "$TAURI_DIR/data"
mkdir -p "$TAURI_DIR/data"

# Step 5: Build Tauri app
print_step "Building Tauri application..."
cd "$SCRIPT_DIR"
npm install
npm run build

# Step 6: Post-build processing (macOS only)
if [ "$GOOS" = "darwin" ]; then
    print_step "Post-processing macOS build..."

    # Find the built app and DMG
    APP_DIR="$TAURI_DIR/target/release/bundle/macos"
    DMG_DIR="$TAURI_DIR/target/release/bundle/dmg"

    # Get app name from tauri.conf.json
    APP_NAME="ZimaOS Echo"

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

# Step 7: Print build results
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
