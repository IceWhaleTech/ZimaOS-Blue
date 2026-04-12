#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: build_zimaos_raw.sh --binary <path> --output <path> [options]

Options:
  --binary <path>        Linux Blue binary to stage into the raw module.
  --output <path>        Output squashfs raw file path.
  --dist-dir <path>      Optional frontend module dist directory to copy into /usr/share/casaos/modules/<name>/.
  --staging-dir <path>   Optional staging directory. Kept on disk when provided.
  --module-name <name>   CasaOS/ZimaOS module name. Default: zimaos-blue
  --command-name <name>  Installed command name under /usr/bin. Default: blue
  --title <text>         UI title. Default: ZimaOS Blue
  --description <text>   UI description. Default: ZimaOS Blue
  --version <value>      Module version. Default: dev
  -h, --help             Show this help text.
EOF
}

fail() {
  echo "[zimaos-raw] $*" >&2
  exit 1
}

require_safe_name() {
  local value="$1"
  local label="$2"
  if [[ ! "$value" =~ ^[A-Za-z0-9._-]+$ ]]; then
    fail "$label must match [A-Za-z0-9._-]+, got: $value"
  fi
}

binary_path=""
output_path=""
staging_dir=""
module_name="zimaos-blue"
command_name="blue"
cleanup_staging=0
image_name=""
dist_dir=""
module_title="ZimaOS Blue"
module_description="ZimaOS Blue"
module_version="dev"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      [[ $# -ge 2 ]] || fail "missing value for --binary"
      binary_path="$2"
      shift 2
      ;;
    --output)
      [[ $# -ge 2 ]] || fail "missing value for --output"
      output_path="$2"
      shift 2
      ;;
    --dist-dir)
      [[ $# -ge 2 ]] || fail "missing value for --dist-dir"
      dist_dir="$2"
      shift 2
      ;;
    --staging-dir)
      [[ $# -ge 2 ]] || fail "missing value for --staging-dir"
      staging_dir="$2"
      shift 2
      ;;
    --module-name)
      [[ $# -ge 2 ]] || fail "missing value for --module-name"
      module_name="$2"
      shift 2
      ;;
    --command-name)
      [[ $# -ge 2 ]] || fail "missing value for --command-name"
      command_name="$2"
      shift 2
      ;;
    --title)
      [[ $# -ge 2 ]] || fail "missing value for --title"
      module_title="$2"
      shift 2
      ;;
    --description)
      [[ $# -ge 2 ]] || fail "missing value for --description"
      module_description="$2"
      shift 2
      ;;
    --version)
      [[ $# -ge 2 ]] || fail "missing value for --version"
      module_version="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[[ -n "$binary_path" ]] || fail "--binary is required"
[[ -n "$output_path" ]] || fail "--output is required"
[[ -f "$binary_path" ]] || fail "binary not found: $binary_path"
if [[ -n "$dist_dir" ]]; then
  [[ -d "$dist_dir" ]] || fail "dist dir not found: $dist_dir"
fi

require_safe_name "$module_name" "module name"
require_safe_name "$command_name" "command name"

image_name="$(basename "$output_path")"
image_name="${image_name%.raw}"
require_safe_name "$image_name" "raw image name"

command -v mksquashfs >/dev/null 2>&1 || fail "mksquashfs is required in PATH"

if [[ -z "$staging_dir" ]]; then
  staging_dir="$(mktemp -d "${TMPDIR:-/tmp}/zimaos-blue-raw.XXXXXX")"
  cleanup_staging=1
fi

cleanup() {
  if [[ "$cleanup_staging" -eq 1 && -n "$staging_dir" ]]; then
    rm -rf "$staging_dir"
  fi
}
trap cleanup EXIT

raw_dir="$staging_dir/raw"
bin_dir="$raw_dir/usr/bin"
module_dir="$raw_dir/usr/share/casaos/modules"
module_ui_dir="$module_dir/$module_name"
extension_dir="$raw_dir/usr/lib/extension-release.d"

rm -rf "$raw_dir"
mkdir -p "$bin_dir" "$module_dir" "$module_ui_dir" "$extension_dir" "$(dirname "$output_path")"

cp "$binary_path" "$bin_dir/$command_name"
chmod 0755 "$bin_dir/$command_name"

if [[ -n "$dist_dir" ]]; then
  cp -R "$dist_dir"/. "$module_ui_dir"/
  rm -f "$module_ui_dir/stats.html"
  find "$module_ui_dir" -name '*.gz' -delete 2>/dev/null || true
  find "$module_ui_dir" -name '*.br' -delete 2>/dev/null || true
  find "$module_ui_dir" -name '.DS_Store' -delete 2>/dev/null || true
fi

printf 'ID=_any\n' > "$extension_dir/extension-release.$image_name"
cat > "$module_dir/$module_name.json" <<EOF
{
  "name": "$module_name",
  "ui": {
    "name": "$module_name",
    "title": {
      "en_us": "$module_title"
    },
    "prefetch": true,
    "show": true,
    "entry": "/modules/$module_name/index.html",
    "icon": "/modules/$module_name/logo.svg",
    "description": "$module_description",
    "formality": {
      "type": "newtab",
      "props": {
        "width": "100vh",
        "height": "100vh",
        "hasModalCard": true,
        "animation": "zoom-in"
      }
    }
  },
  "version": "$module_version"
}
EOF

rm -f "$output_path"
mksquashfs "$raw_dir" "$output_path" -noappend

echo "[zimaos-raw] staged $binary_path into $raw_dir"
echo "[zimaos-raw] wrote $output_path"
