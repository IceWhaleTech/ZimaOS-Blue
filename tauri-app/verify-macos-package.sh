#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TAURI_DIR="$SCRIPT_DIR/src-tauri"
DEFAULT_APP_DIR="$TAURI_DIR/target/release/bundle/macos"
DEFAULT_DMG_DIR="$TAURI_DIR/target/release/bundle/dmg"

APP_PATH="${MACOS_APP_PATH:-}"
DMG_PATH="${MACOS_DMG_PATH:-}"

print_step() {
    echo "[STEP] $1"
}

print_error() {
    echo "[ERROR] $1" >&2
}

if [ -z "$APP_PATH" ]; then
    APP_PATH=$(find "$DEFAULT_APP_DIR" -name "*.app" -type d 2>/dev/null | head -1 || true)
fi

if [ -z "$DMG_PATH" ]; then
    DMG_PATH=$(find "$DEFAULT_DMG_DIR" -name "*.dmg" -type f 2>/dev/null | head -1 || true)
fi

if [ -z "$APP_PATH" ] && [ -z "$DMG_PATH" ]; then
    print_error "No macOS .app or .dmg bundle was found to verify."
    exit 1
fi

if [ -n "$APP_PATH" ]; then
    if [ ! -d "$APP_PATH" ]; then
        print_error "App bundle not found: $APP_PATH"
        exit 1
    fi

    print_step "Verifying app signature: $APP_PATH"
    codesign --verify --deep --strict --verbose=2 "$APP_PATH"
fi

if [ -n "$DMG_PATH" ]; then
    if [ ! -f "$DMG_PATH" ]; then
        print_error "DMG not found: $DMG_PATH"
        exit 1
    fi

    print_step "Verifying DMG signature: $DMG_PATH"
    codesign --verify --verbose=2 "$DMG_PATH"

    print_step "Validating stapled notarization ticket: $DMG_PATH"
    xcrun stapler validate "$DMG_PATH"
    spctl -a -vv -t open --context context:primary-signature "$DMG_PATH"
fi

print_step "macOS package verification completed successfully"
