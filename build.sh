#!/bin/bash
# Build script for ZimaOS-Echo
# This script builds the frontend and embeds it into the Go binary

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

# Version info
VERSION="${VERSION:-0.9.0}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo -e "${GREEN}Building ZimaOS-Echo v${VERSION}${NC}"
echo "Build time: $BUILD_TIME"
echo "Git commit: $GIT_COMMIT"
echo ""

# Step 1: Build frontend
echo -e "${YELLOW}Step 1: Building frontend...${NC}"
cd "$PROJECT_ROOT/web"

if [ ! -d "node_modules" ]; then
    echo "Installing npm dependencies..."
    npm install
fi

echo "Building production frontend..."
npm run build

# Step 2: Copy frontend build to server embed directory
echo -e "${YELLOW}Step 2: Copying frontend build to server...${NC}"
EMBED_DIR="$PROJECT_ROOT/server/internal/web/dist"
rm -rf "$EMBED_DIR"
mkdir -p "$EMBED_DIR"
cp -r dist/* "$EMBED_DIR/"
echo "Frontend copied to $EMBED_DIR"

# Step 3: Build Go binary
echo -e "${YELLOW}Step 3: Building Go binary...${NC}"
cd "$PROJECT_ROOT/server"

# Build flags
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X main.version=$VERSION"
LDFLAGS="$LDFLAGS -X main.buildTime=$BUILD_TIME"
LDFLAGS="$LDFLAGS -X main.gitCommit=$GIT_COMMIT"

# Output directory
OUTPUT_DIR="$PROJECT_ROOT/dist"
mkdir -p "$OUTPUT_DIR"

# Detect OS and architecture
OS="${GOOS:-$(go env GOOS)}"
ARCH="${GOARCH:-$(go env GOARCH)}"

# Binary name
BINARY_NAME="zimaos-echo"
if [ "$OS" = "windows" ]; then
    BINARY_NAME="zimaos-echo.exe"
fi

OUTPUT_PATH="$OUTPUT_DIR/${BINARY_NAME}"

echo "Building for $OS/$ARCH..."
CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build \
    -ldflags "$LDFLAGS" \
    -o "$OUTPUT_PATH" \
    ./cmd/echo

# Get file size
if [ "$OS" = "darwin" ]; then
    SIZE=$(stat -f%z "$OUTPUT_PATH" 2>/dev/null || echo "unknown")
else
    SIZE=$(stat -c%s "$OUTPUT_PATH" 2>/dev/null || echo "unknown")
fi

if [ "$SIZE" != "unknown" ]; then
    SIZE_MB=$(echo "scale=2; $SIZE / 1048576" | bc)
    echo -e "${GREEN}Binary size: ${SIZE_MB} MB${NC}"
fi

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo "Output: $OUTPUT_PATH"
echo ""
echo "To run the server:"
echo "  $OUTPUT_PATH"
echo ""
echo "To run with custom config:"
echo "  $OUTPUT_PATH -config /path/to/config.yaml"
