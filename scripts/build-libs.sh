#!/bin/bash
# Build whisper.cpp and opus as shared libraries for FFI loading

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
THIRD_PARTY="$PROJECT_ROOT/third_party"
OUTPUT_DIR="$PROJECT_ROOT/libs"

mkdir -p "$OUTPUT_DIR"

echo "=== Building Shared Libraries for FFI ==="

# Detect platform
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        PLATFORM="windows"
        LIB_EXT="dll"
        ;;
    Darwin*)
        PLATFORM="darwin"
        LIB_EXT="dylib"
        ;;
    Linux*)
        PLATFORM="linux"
        LIB_EXT="so"
        ;;
    *)
        echo "Unsupported platform"
        exit 1
        ;;
esac

echo "Platform: $PLATFORM"

# Build whisper.cpp as shared library
echo "[1/2] Building whisper.cpp..."
cd "$THIRD_PARTY/whisper.cpp"
mkdir -p build-shared
cd build-shared

cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=ON \
    -DWHISPER_BUILD_TESTS=OFF \
    -DWHISPER_BUILD_EXAMPLES=OFF

cmake --build . --config Release

# Copy whisper library
if [ "$PLATFORM" = "windows" ]; then
    cp src/whisper.dll "$OUTPUT_DIR/"
    cp ggml/src/ggml.dll "$OUTPUT_DIR/" 2>/dev/null || true
else
    cp src/libwhisper.$LIB_EXT "$OUTPUT_DIR/"
    cp ggml/src/libggml.$LIB_EXT "$OUTPUT_DIR/" 2>/dev/null || true
fi

# Build opus as shared library
echo "[2/2] Building opus..."
cd "$THIRD_PARTY/opus-src"
mkdir -p build-shared
cd build-shared

cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=ON

cmake --build . --config Release

# Copy opus library
if [ "$PLATFORM" = "windows" ]; then
    cp opus.dll "$OUTPUT_DIR/"
else
    cp libopus.$LIB_EXT "$OUTPUT_DIR/"
fi

echo "=== Build Complete ==="
echo "Libraries installed to: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"
