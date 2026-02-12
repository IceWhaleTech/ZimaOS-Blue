#!/bin/bash
# Generate icons for Tauri app from source logo
# Requires: ImageMagick (magick command)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
ICONS_DIR="$PROJECT_ROOT/tauri-app/src-tauri/icons"
SOURCE_LOGO="$PROJECT_ROOT/docs/public/logo.png"

echo "Generating icons for ZimaOS Blue..."

# Check if ImageMagick is available
if ! command -v magick &> /dev/null; then
    echo "ImageMagick not found. Installing via Homebrew..."
    if command -v brew &> /dev/null; then
        brew install imagemagick
    else
        echo "Error: ImageMagick is required. Please install it manually."
        echo "  macOS: brew install imagemagick"
        echo "  Ubuntu: sudo apt-get install imagemagick"
        exit 1
    fi
fi

mkdir -p "$ICONS_DIR"

# Check if source logo exists
if [ ! -f "$SOURCE_LOGO" ]; then
    echo "Source logo not found at $SOURCE_LOGO"
    echo "Generating placeholder icons..."
    PRIMARY_COLOR="#3B82F6"
    magick -size 1024x1024 xc:"$PRIMARY_COLOR" -alpha set -background none PNG32:"$ICONS_DIR/icon-1024.png"
else
    echo "Using source logo: $SOURCE_LOGO"
    # Resize source logo to 1024x1024 for consistent processing
    magick "$SOURCE_LOGO" -resize 1024x1024 -alpha set -background none PNG32:"$ICONS_DIR/icon-1024.png"
fi

# Generate RGBA PNG icons
echo "Creating 32x32.png (RGBA)..."
magick "$ICONS_DIR/icon-1024.png" -resize 32x32 PNG32:"$ICONS_DIR/32x32.png"

echo "Creating 128x128.png (RGBA)..."
magick "$ICONS_DIR/icon-1024.png" -resize 128x128 PNG32:"$ICONS_DIR/128x128.png"

echo "Creating 128x128@2x.png (256x256, RGBA)..."
magick "$ICONS_DIR/icon-1024.png" -resize 256x256 PNG32:"$ICONS_DIR/128x128@2x.png"

# Generate macOS .icns
echo "Creating icon.icns..."
mkdir -p "$ICONS_DIR/icon.iconset"
magick "$ICONS_DIR/icon-1024.png" -resize 16x16 PNG32:"$ICONS_DIR/icon.iconset/icon_16x16.png"
magick "$ICONS_DIR/icon-1024.png" -resize 32x32 PNG32:"$ICONS_DIR/icon.iconset/icon_16x16@2x.png"
magick "$ICONS_DIR/icon-1024.png" -resize 32x32 PNG32:"$ICONS_DIR/icon.iconset/icon_32x32.png"
magick "$ICONS_DIR/icon-1024.png" -resize 64x64 PNG32:"$ICONS_DIR/icon.iconset/icon_32x32@2x.png"
magick "$ICONS_DIR/icon-1024.png" -resize 128x128 PNG32:"$ICONS_DIR/icon.iconset/icon_128x128.png"
magick "$ICONS_DIR/icon-1024.png" -resize 256x256 PNG32:"$ICONS_DIR/icon.iconset/icon_128x128@2x.png"
magick "$ICONS_DIR/icon-1024.png" -resize 256x256 PNG32:"$ICONS_DIR/icon.iconset/icon_256x256.png"
magick "$ICONS_DIR/icon-1024.png" -resize 512x512 PNG32:"$ICONS_DIR/icon.iconset/icon_256x256@2x.png"
magick "$ICONS_DIR/icon-1024.png" -resize 512x512 PNG32:"$ICONS_DIR/icon.iconset/icon_512x512.png"
magick "$ICONS_DIR/icon-1024.png" -resize 1024x1024 PNG32:"$ICONS_DIR/icon.iconset/icon_512x512@2x.png"

if command -v iconutil &> /dev/null; then
    iconutil -c icns "$ICONS_DIR/icon.iconset" -o "$ICONS_DIR/icon.icns"
else
    echo "Warning: iconutil not available (macOS only). Skipping .icns generation."
    # Create a placeholder file
    cp "$ICONS_DIR/icon-1024.png" "$ICONS_DIR/icon.icns"
fi
rm -rf "$ICONS_DIR/icon.iconset"

# Generate Windows .ico
echo "Creating icon.ico..."
magick "$ICONS_DIR/icon-1024.png" -define icon:auto-resize=256,128,64,48,32,16 "$ICONS_DIR/icon.ico"

# Generate tray icon (22x22, RGBA)
echo "Creating tray.png (RGBA)..."
magick "$ICONS_DIR/icon-1024.png" -resize 22x22 PNG32:"$ICONS_DIR/tray.png"

# Cleanup
rm -f "$ICONS_DIR/icon-1024.png"

echo ""
echo "Icons generated successfully!"
echo "Location: $ICONS_DIR"
ls -la "$ICONS_DIR"
