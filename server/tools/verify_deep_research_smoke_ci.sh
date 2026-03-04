#!/usr/bin/env bash
set -euo pipefail

# CI quick smoke for deep_research post-release verification.
# Defaults to 3 locales for fast signal; all options can be overridden.
#
# Env overrides:
#   SERVER_URL (default: http://127.0.0.1:18080)
#   TOKEN
#   QUERY (default: OpenAI)
#   MODE (default: fast)
#   LOCALES (default: en-US,zh-CN,ja-JP)
#   JOB_TIMEOUT_SEC (default: 90)
#   POLL_INTERVAL_SEC (default: 2)
#   MAX_CREATE_RETRIES (default: 3)
#   OUTPUT_JSON
#
# Positional args are passed through to verify_deep_research_post_release.sh.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SERVER_URL_VALUE="${SERVER_URL:-http://127.0.0.1:18080}"
TOKEN_VALUE="${TOKEN:-}"
QUERY_VALUE="${QUERY:-OpenAI}"
MODE_VALUE="${MODE:-fast}"
LOCALES_VALUE="${LOCALES:-en-US,zh-CN,ja-JP}"
JOB_TIMEOUT_SEC_VALUE="${JOB_TIMEOUT_SEC:-90}"
POLL_INTERVAL_SEC_VALUE="${POLL_INTERVAL_SEC:-2}"
MAX_CREATE_RETRIES_VALUE="${MAX_CREATE_RETRIES:-3}"
OUTPUT_JSON_VALUE="${OUTPUT_JSON:-}"

ARGS=(
  "--server-url" "${SERVER_URL_VALUE}"
  "--query" "${QUERY_VALUE}"
  "--mode" "${MODE_VALUE}"
  "--locales" "${LOCALES_VALUE}"
  "--job-timeout-sec" "${JOB_TIMEOUT_SEC_VALUE}"
  "--poll-interval-sec" "${POLL_INTERVAL_SEC_VALUE}"
  "--max-create-retries" "${MAX_CREATE_RETRIES_VALUE}"
  "--stop-on-fail"
)

if [[ -n "${TOKEN_VALUE}" ]]; then
  ARGS+=("--token" "${TOKEN_VALUE}")
fi

if [[ -n "${OUTPUT_JSON_VALUE}" ]]; then
  ARGS+=("--output-json" "${OUTPUT_JSON_VALUE}")
fi

if [[ $# -gt 0 ]]; then
  ARGS+=("$@")
fi

"${ROOT_DIR}/tools/verify_deep_research_post_release.sh" "${ARGS[@]}"
