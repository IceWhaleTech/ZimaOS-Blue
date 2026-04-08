#!/usr/bin/env bash

set -euo pipefail

if [[ -z "${CERTIFICATE_BASE64:-}" ]]; then
  echo "::error::MACOS_CERTIFICATE secret is empty or unavailable."
  exit 1
fi

if [[ -z "${CERTIFICATE_PASSWORD:-}" ]]; then
  echo "::error::MACOS_CERTIFICATE_PASSWORD secret is empty or unavailable."
  exit 1
fi

tmp_root="${RUNNER_TEMP:-/tmp}"
cert_base="$(mktemp "${tmp_root%/}/macos-cert.XXXXXX")"
cert_file="${cert_base}.p12"
keychain_path="${tmp_root%/}/build.keychain"

mv "$cert_base" "$cert_file"
trap 'rm -f "$cert_file"' EXIT

printf '%s' "$CERTIFICATE_BASE64" | base64 --decode > "$cert_file"

if [[ ! -s "$cert_file" ]]; then
  echo "::error::Decoded MACOS_CERTIFICATE is empty."
  exit 1
fi

if ! openssl pkcs12 -in "$cert_file" -passin "pass:${CERTIFICATE_PASSWORD}" -info -noout >/dev/null 2>&1; then
  echo "::error::MACOS_CERTIFICATE must be a base64-encoded PKCS#12 (.p12/.pfx) bundle and MACOS_CERTIFICATE_PASSWORD must match it."
  exit 1
fi

security delete-keychain "$keychain_path" 2>/dev/null || true
security create-keychain -p actions "$keychain_path"
security default-keychain -s "$keychain_path"
security unlock-keychain -p actions "$keychain_path"
security set-keychain-settings -lut 21600 "$keychain_path"
security import "$cert_file" -k "$keychain_path" -f pkcs12 -t agg -P "$CERTIFICATE_PASSWORD" \
  -T /usr/bin/codesign -T /usr/bin/security
security set-key-partition-list -S apple-tool:,apple: -s -k actions "$keychain_path"
