#!/bin/bash
# Unified build script with static/FFI mode support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Load build config
if [ -f .buildrc ]; then
    source .buildrc
fi

BUILD_MODE=${BUILD_MODE:-static}

echo "=== ZimaOS-Blue Build ==="
echo "Mode: $BUILD_MODE"

case "$BUILD_MODE" in
    static)
        echo "[Static Mode] Building with embedded libraries..."
        cd server
        go build -tags whisper -buildmode=c-archive \
            -ldflags "$GO_LDFLAGS" \
            -o ../tauri-app/src-tauri/lib/libblue.a \
            ./cmd/bluelib
        ;;
    ffi)
        echo "[FFI Mode] Building with dynamic loading..."
        # Build shared libraries if needed
        if [ ! -d "$LIB_DIR" ] || [ -z "$(ls -A $LIB_DIR)" ]; then
            echo "Building shared libraries..."
            ./scripts/build-libs.sh
        fi
        cd server
        go build -ldflags "$GO_LDFLAGS" -o blue ./cmd/blue
        ;;
    *)
        echo "Unknown BUILD_MODE: $BUILD_MODE"
        exit 1
        ;;
esac

echo "✓ Build complete"
