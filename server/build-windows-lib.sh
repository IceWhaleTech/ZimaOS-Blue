#!/bin/bash
# Build Windows static library for Blue server

set -e

echo "Building Windows static library (libblue.lib)..."

if ! command -v zig >/dev/null 2>&1; then
    echo "zig not found on PATH"
    exit 1
fi

# Set CGO and build flags
export CGO_ENABLED=1
export GOOS=windows
export GOARCH=amd64
export CC="zig cc -target x86_64-windows-gnu"
export CXX="zig c++ -target x86_64-windows-gnu"

# Build as C archive (static library)
go build -buildmode=c-archive -tags 'fts5 espeak kokoro' -o ../tauri-app/src-tauri/lib/libblue.lib ./cmd/bluelib

echo "✓ Built libblue.lib"
ls -lh ../tauri-app/src-tauri/lib/libblue.lib
