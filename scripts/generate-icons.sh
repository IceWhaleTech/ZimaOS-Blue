#!/bin/bash
# Generate all app icon sizes from icon-source.svg
# Requires: rsvg-convert (librsvg) or falls back to sips
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ICONS_DIR="$SCRIPT_DIR/../tauri-app/src-tauri/icons"
NSIS_ICONS_DIR="$SCRIPT_DIR/../tauri-app/src-tauri/nsis/icons"
SOURCE="$ICONS_DIR/icon-source.svg"
ICONSET_DIR="/tmp/zimaos-blue.iconset"

if ! command -v rsvg-convert &>/dev/null; then
  echo "Installing librsvg via brew..."
  brew install librsvg
fi

echo "Generating PNGs from $SOURCE..."

# Generate main icon.png (1024x1024)
rsvg-convert -w 1024 -h 1024 "$SOURCE" -o "$ICONS_DIR/icon.png"

# Generate Tauri required sizes
rsvg-convert -w 32  -h 32  "$SOURCE" -o "$ICONS_DIR/32x32.png"
rsvg-convert -w 64  -h 64  "$SOURCE" -o "$ICONS_DIR/64x64.png"
rsvg-convert -w 48  -h 48  "$SOURCE" -o "$ICONS_DIR/48x48.png"
rsvg-convert -w 128 -h 128 "$SOURCE" -o "$ICONS_DIR/128x128.png"
rsvg-convert -w 256 -h 256 "$SOURCE" -o "$ICONS_DIR/128x128@2x.png"

# Generate tray icon from dedicated tray-source.svg (single-color template, 44x44 for Retina)
TRAY_SOURCE="$ICONS_DIR/tray-source.svg"
rsvg-convert -w 44 -h 44 "$TRAY_SOURCE" -o "$ICONS_DIR/tray.png"

# Generate macOS .icns via iconutil
rm -rf "$ICONSET_DIR"
mkdir -p "$ICONSET_DIR"
rsvg-convert -w 16   -h 16   "$SOURCE" -o "$ICONSET_DIR/icon_16x16.png"
rsvg-convert -w 32   -h 32   "$SOURCE" -o "$ICONSET_DIR/icon_16x16@2x.png"
rsvg-convert -w 32   -h 32   "$SOURCE" -o "$ICONSET_DIR/icon_32x32.png"
rsvg-convert -w 64   -h 64   "$SOURCE" -o "$ICONSET_DIR/icon_32x32@2x.png"
rsvg-convert -w 128  -h 128  "$SOURCE" -o "$ICONSET_DIR/icon_128x128.png"
rsvg-convert -w 256  -h 256  "$SOURCE" -o "$ICONSET_DIR/icon_128x128@2x.png"
rsvg-convert -w 256  -h 256  "$SOURCE" -o "$ICONSET_DIR/icon_256x256.png"
rsvg-convert -w 512  -h 512  "$SOURCE" -o "$ICONSET_DIR/icon_256x256@2x.png"
rsvg-convert -w 512  -h 512  "$SOURCE" -o "$ICONSET_DIR/icon_512x512.png"
rsvg-convert -w 1024 -h 1024 "$SOURCE" -o "$ICONSET_DIR/icon_512x512@2x.png"

iconutil -c icns "$ICONSET_DIR" -o "$ICONS_DIR/icon.icns"
rm -rf "$ICONSET_DIR"
echo "Generated icon.icns"

# Keep the macOS bundle icns aligned with the canonical Blue source icon.
mkdir -p "$NSIS_ICONS_DIR"
cp "$ICONS_DIR/icon.icns" "$NSIS_ICONS_DIR/icon.icns"
echo "Synced icon.icns to $NSIS_ICONS_DIR/icon.icns"

# Generate Windows .ico (multi-size)
# iconutil doesn't do .ico, use sips + png2ico or just provide the PNGs
# Tauri will handle .ico generation from PNGs if needed
# For now, create a simple .ico from the 256x256 PNG using sips
if command -v magick &>/dev/null; then
  magick "$ICONS_DIR/icon.png" -define icon:auto-resize=256,128,64,48,32,16 "$ICONS_DIR/icon.ico"
  echo "Generated icon.ico (ImageMagick)"
else
  echo "Skipping .ico generation (install ImageMagick for .ico support)"
fi

# Copy to web/public as well
cp "$ICONS_DIR/icon.png" "$SCRIPT_DIR/../web/public/logo.png"
echo "Copied to web/public/logo.png"

echo "Done! All icons generated."
ls -la "$ICONS_DIR"/*.png "$ICONS_DIR"/*.icns "$ICONS_DIR"/*.ico 2>/dev/null
