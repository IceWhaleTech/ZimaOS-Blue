#!/usr/bin/env bash
set -euo pipefail

artifact="${1:?Usage: notarize-apple-artifact.sh <zip-or-dmg>}"
[[ -f "$artifact" ]] || { echo "::error::Notarization artifact is missing: $artifact"; exit 1; }
for name in APPLE_ID APPLE_APP_PASSWORD APPLE_TEAM_ID; do
  if [[ -z "${!name:-}" ]]; then
    echo "::error::$name is empty or unavailable."
    exit 1
  fi
done

receipt="$(mktemp "${RUNNER_TEMP:-/tmp}/blue-notary.XXXXXX")"
trap 'rm -f "$receipt"' EXIT
xcrun notarytool submit "$artifact" \
  --apple-id "$APPLE_ID" \
  --password "$APPLE_APP_PASSWORD" \
  --team-id "$APPLE_TEAM_ID" \
  --wait --output-format json > "$receipt"
python3 - "$receipt" <<'PY'
import json
import sys
with open(sys.argv[1]) as source:
    result = json.load(source)
status = result.get('status')
print(f"Apple notarization: {status} (submission {result.get('id', 'unknown')})")
if status != 'Accepted':
    sys.exit('::error::Apple notarization was not Accepted.')
PY
