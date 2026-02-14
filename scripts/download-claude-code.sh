#!/bin/bash
# Download Claude Code CLI binaries for bundling with ZimaOS-Blue
# This script downloads the native Claude Code CLI for supported platforms
#
# Usage:
#   ./download-claude-code.sh              # Download for current platform
#   ./download-claude-code.sh --all        # Download for all platforms
#   ./download-claude-code.sh --platform darwin-arm64  # Download specific platform

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CLAUDE_CODE_DIR="$PROJECT_ROOT/server/internal/claudecode/bin"

GCS_BUCKET="https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"

# All supported platforms
ALL_PLATFORMS=(
    "darwin-arm64"
    "darwin-x64"
    "linux-arm64"
    "linux-x64"
    "win32-x64"
)

# Function to detect current platform
get_current_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$os" in
        darwin)
            case "$arch" in
                x86_64) echo "darwin-x64" ;;
                arm64)  echo "darwin-arm64" ;;
                *)      echo "darwin-x64" ;;
            esac
            ;;
        linux)
            case "$arch" in
                x86_64)  echo "linux-x64" ;;
                aarch64) echo "linux-arm64" ;;
                arm64)   echo "linux-arm64" ;;
                *)       echo "linux-x64" ;;
            esac
            ;;
        mingw*|msys*|cygwin*)
            echo "win32-x64"
            ;;
        *)
            echo "linux-x64"
            ;;
    esac
}

# Parse arguments
SPECIFIC_PLATFORM=""
DOWNLOAD_ALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --platform)
            SPECIFIC_PLATFORM="$2"
            shift 2
            ;;
        --all)
            DOWNLOAD_ALL=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --platform PLATFORM  Download specific platform (e.g., darwin-arm64)"
            echo "  --all                Download all platforms"
            echo "  --help, -h           Show this help message"
            echo ""
            echo "Supported platforms:"
            for p in "${ALL_PLATFORMS[@]}"; do
                echo "  - $p"
            done
            echo ""
            echo "If no options specified, downloads for current platform only."
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Determine which platforms to download
if [ -n "$SPECIFIC_PLATFORM" ]; then
    PLATFORMS=("$SPECIFIC_PLATFORM")
elif [ "$DOWNLOAD_ALL" = true ]; then
    PLATFORMS=("${ALL_PLATFORMS[@]}")
else
    # Default: current platform only
    CURRENT_PLATFORM=$(get_current_platform)
    PLATFORMS=("$CURRENT_PLATFORM")
    echo "Downloading for current platform: $CURRENT_PLATFORM"
fi

# Get latest version
echo "Fetching latest Claude Code version..."
VERSION=$(curl -fsSL "$GCS_BUCKET/latest")
echo "Latest version: $VERSION"

# Download manifest
echo "Downloading manifest..."
MANIFEST=$(curl -fsSL "$GCS_BUCKET/$VERSION/manifest.json")

# Create output directory
mkdir -p "$CLAUDE_CODE_DIR"

# Track download count
DOWNLOADED=0
FAILED=0

# Download each platform
for platform in "${PLATFORMS[@]}"; do
    echo ""
    echo "Downloading Claude Code for $platform..."

    # Extract checksum from manifest
    checksum=$(echo "$MANIFEST" | jq -r ".platforms[\"$platform\"].checksum // empty")

    if [ -z "$checksum" ]; then
        echo "  Warning: Platform $platform not found in manifest, skipping..."
        ((FAILED++)) || true
        continue
    fi

    # Determine output filename
    if [[ "$platform" == "win32-x64" ]]; then
        output_file="$CLAUDE_CODE_DIR/claude-$platform.exe"
    else
        output_file="$CLAUDE_CODE_DIR/claude-$platform"
    fi

    # Download binary
    # Windows binaries have .exe extension in the URL
    if [[ "$platform" == "win32-x64" ]]; then
        echo "  Downloading from $GCS_BUCKET/$VERSION/$platform/claude.exe..."
        if ! curl -fsSL -o "$output_file" "$GCS_BUCKET/$VERSION/$platform/claude.exe"; then
            echo "  ERROR: Failed to download $platform"
            ((FAILED++)) || true
            continue
        fi
    else
        echo "  Downloading from $GCS_BUCKET/$VERSION/$platform/claude..."
        if ! curl -fsSL -o "$output_file" "$GCS_BUCKET/$VERSION/$platform/claude"; then
            echo "  ERROR: Failed to download $platform"
            ((FAILED++)) || true
            continue
        fi
    fi

    # Verify checksum
    if [[ "$(uname -s)" == "Darwin" ]]; then
        actual=$(shasum -a 256 "$output_file" | cut -d' ' -f1)
    else
        actual=$(sha256sum "$output_file" | cut -d' ' -f1)
    fi

    if [ "$actual" != "$checksum" ]; then
        echo "  ERROR: Checksum verification failed for $platform"
        echo "    Expected: $checksum"
        echo "    Actual:   $actual"
        rm -f "$output_file"
        ((FAILED++)) || true
        continue
    fi

    echo "  Checksum verified: $checksum"

    # Make executable (except Windows)
    if [[ "$platform" != "win32-x64" ]]; then
        chmod +x "$output_file"
    fi

    # Show file size
    size=$(du -h "$output_file" | cut -f1)
    echo "  Downloaded: $output_file ($size)"
    ((DOWNLOADED++)) || true
done

# Write version file
echo "$VERSION" > "$CLAUDE_CODE_DIR/VERSION"

echo ""
echo "========================================"
echo "Claude Code CLI download complete!"
echo "Version: $VERSION"
echo "Downloaded: $DOWNLOADED platform(s)"
if [ $FAILED -gt 0 ]; then
    echo "Failed: $FAILED platform(s)"
fi
echo "Location: $CLAUDE_CODE_DIR"
echo ""
ls -lh "$CLAUDE_CODE_DIR"

# Exit with error if any downloads failed
if [ $FAILED -gt 0 ] && [ $DOWNLOADED -eq 0 ]; then
    exit 1
fi
