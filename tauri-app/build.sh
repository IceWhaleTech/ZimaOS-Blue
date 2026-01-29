#!/bin/bash
# ZimaOS Echo - Tauri Build Script
# This script builds the complete Tauri application package

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TAURI_DIR="$SCRIPT_DIR/src-tauri"

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

# Step 1: Build frontend
print_step "Building frontend..."
cd "$PROJECT_ROOT/web"
npm install
npm run build

# Step 2: Copy frontend to embed directory (excluding source maps)
print_step "Copying frontend to embed directory..."
EMBED_DIR="$PROJECT_ROOT/server/internal/web/dist"
mkdir -p "$EMBED_DIR"
rsync -av --delete --exclude='*.map' "$PROJECT_ROOT/web/dist/" "$EMBED_DIR/"

# Step 3: Build Go sidecar
print_step "Building Go sidecar..."
cd "$PROJECT_ROOT/server"

# Detect platform
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

case "$GOOS-$GOARCH" in
    darwin-arm64)
        TARGET="aarch64-apple-darwin"
        ;;
    darwin-amd64)
        TARGET="x86_64-apple-darwin"
        ;;
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

# Build with optimizations
CGO_ENABLED=0 go build -ldflags="-s -w" -o "$TAURI_DIR/binaries/$SIDECAR_NAME" ./cmd/echo/

# Also copy to bin directory for resources bundling
mkdir -p "$TAURI_DIR/bin"
cp "$TAURI_DIR/binaries/$SIDECAR_NAME" "$TAURI_DIR/bin/"

print_step "Sidecar built: $SIDECAR_NAME"

# Step 4: Clean up data directory before build
print_step "Cleaning up data directory..."
rm -rf "$TAURI_DIR/data"
mkdir -p "$TAURI_DIR/data"

# Step 5: Build Tauri app
print_step "Building Tauri application..."
cd "$SCRIPT_DIR"
npm install
npm run build

# Step 6: Set DMG file icon (macOS only)
if [ "$GOOS" = "darwin" ]; then
    print_step "Setting DMG file icon..."

    # Find the DMG file
    DMG_DIR="$TAURI_DIR/target/release/bundle/dmg"
    DMG_FILE=$(find "$DMG_DIR" -name "*.dmg" -type f 2>/dev/null | head -1)
    ICON_FILE="$TAURI_DIR/icons/icon.icns"

    if [ -n "$DMG_FILE" ] && [ -f "$DMG_FILE" ]; then
        if command -v fileicon &> /dev/null; then
            fileicon set "$DMG_FILE" "$ICON_FILE"
            print_step "DMG file icon set successfully"
        else
            print_warning "fileicon tool not found. Install with: brew install fileicon"
            print_warning "The DMG was built but the file icon was not set."
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
    echo "Output files:"
    ls -la "$TAURI_DIR/target/release/bundle/nsis/"*.exe 2>/dev/null || true
    ls -la "$TAURI_DIR/target/release/bundle/msi/"*.msi 2>/dev/null || true
elif [ "$GOOS" = "linux" ]; then
    echo ""
    echo "Output files:"
    ls -la "$TAURI_DIR/target/release/bundle/appimage/"*.AppImage 2>/dev/null || true
    ls -la "$TAURI_DIR/target/release/bundle/deb/"*.deb 2>/dev/null || true
fi

echo ""
echo "Done!"
