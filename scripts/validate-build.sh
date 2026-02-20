#!/bin/bash
# Quick validation of build artifacts

set -e

echo "=== Build Validation ==="

# Check Go library
if [ -f "tauri-app/src-tauri/lib/libblue.a" ]; then
    SIZE=$(du -h tauri-app/src-tauri/lib/libblue.a | cut -f1)
    echo "✓ libblue.a exists ($SIZE)"

    # Check for required symbols
    if nm tauri-app/src-tauri/lib/libblue.a | grep -q "BlueServerStart"; then
        echo "✓ Exports found"
    else
        echo "✗ Missing exports"
        exit 1
    fi
else
    echo "✗ libblue.a not found"
    exit 1
fi

# Check Rust build
cd tauri-app/src-tauri
if cargo build --lib 2>&1 | grep -q "Finished"; then
    echo "✓ Rust build successful"
else
    echo "✗ Rust build failed"
    exit 1
fi

echo "=== All checks passed ==="
