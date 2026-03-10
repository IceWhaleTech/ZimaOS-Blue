#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  verify_autoresearch_wrapper_smoke.sh

Description:
  Runs docs/examples/autoresearch-wrapper.sh with a local mock autoresearch repo
  and validates the JSON output contract end-to-end.
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

assert_eq() {
  local actual="$1"
  local expected="$2"
  local label="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "ERROR: ${label}: got='${actual}' want='${expected}'" >&2
    exit 1
  fi
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd bash
require_cmd jq

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WRAPPER_PATH="${ROOT_DIR}/docs/examples/autoresearch-wrapper.sh"
if [[ ! -f "$WRAPPER_PATH" ]]; then
  echo "ERROR: wrapper not found: $WRAPPER_PATH" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

MOCK_REPO="${TMP_DIR}/mock-autoresearch"
ARTIFACT_DIR="${TMP_DIR}/artifacts"
mkdir -p "$MOCK_REPO" "$ARTIFACT_DIR"

cat >"${MOCK_REPO}/train.py" <<'EOF'
#!/usr/bin/env python3
print("mock train.py")
EOF

cat >"${MOCK_REPO}/mock_run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "query_env=${BLUE_RESEARCH_QUERY:-missing}"
echo "mode_env=${BLUE_RESEARCH_MODE:-missing}"
echo "route_env=${BLUE_RESEARCH_ROUTE_MODE:-missing}"
echo "val_bpb: 1.111"
echo "num_steps: 42"
echo "total_seconds: 7.5"
echo "peak_vram_mb: 1234"
echo "mfu_percent: 56.7"
EOF

chmod +x "${MOCK_REPO}/mock_run.sh"

PAYLOAD="$(jq -n \
  --arg job_id "job-smoke-1" \
  --arg query "smoke query" \
  --arg mode "deep" \
  --arg route_mode "experiment" \
  --arg lang "en-US" \
  --arg artifact_dir "$ARTIFACT_DIR" \
  '{
    job_id: $job_id,
    query: $query,
    mode: $mode,
    route_mode: $route_mode,
    lang: $lang,
    artifact_dir: $artifact_dir
  }')"

OUT_JSON="${TMP_DIR}/wrapper-output.json"
printf '%s\n' "$PAYLOAD" | \
  AUTORESEARCH_REPO="$MOCK_REPO" \
  AUTORESEARCH_RUN_CMD="bash ./mock_run.sh" \
  bash "$WRAPPER_PATH" >"$OUT_JSON"

backend="$(jq -r '.metadata.backend // empty' "$OUT_JSON")"
assert_eq "$backend" "autoresearch-wrapper" "metadata.backend"

query="$(jq -r '.metadata.query // empty' "$OUT_JSON")"
assert_eq "$query" "smoke query" "metadata.query"

route_mode="$(jq -r '.metadata.route_mode // empty' "$OUT_JSON")"
assert_eq "$route_mode" "experiment" "metadata.route_mode"

val_bpb="$(jq -r '.metadata.metrics.val_bpb // empty' "$OUT_JSON")"
assert_eq "$val_bpb" "1.111" "metadata.metrics.val_bpb"

num_steps="$(jq -r '.metadata.metrics.num_steps // empty' "$OUT_JSON")"
assert_eq "$num_steps" "42" "metadata.metrics.num_steps"

request_path="$(jq -r '.artifacts[] | select(.label=="request_payload") | .path // empty' "$OUT_JSON")"
if [[ -z "$request_path" || ! -f "$request_path" ]]; then
  echo "ERROR: request_payload artifact not found: $request_path" >&2
  exit 1
fi

log_path="$(jq -r '.artifacts[] | select(.label=="train_log") | .path // empty' "$OUT_JSON")"
if [[ -z "$log_path" || ! -f "$log_path" ]]; then
  echo "ERROR: train_log artifact not found: $log_path" >&2
  exit 1
fi

if ! grep -q "query_env=smoke query" "$log_path"; then
  echo "ERROR: train_log missing query_env marker (env not propagated?)" >&2
  exit 1
fi
if ! grep -q "mode_env=deep" "$log_path"; then
  echo "ERROR: train_log missing mode_env marker (env not propagated?)" >&2
  exit 1
fi
if ! grep -q "route_env=experiment" "$log_path"; then
  echo "ERROR: train_log missing route_env marker (env not propagated?)" >&2
  exit 1
fi

echo "OK: autoresearch wrapper smoke passed"
echo "output_json=${OUT_JSON}"
echo "train_log=${log_path}"
