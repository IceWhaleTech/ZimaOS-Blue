#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  verify_responses_provider.sh <base_url> <api_key> [model]

Example:
  verify_responses_provider.sh https://example.com sk-xxxx gpt-5.3-codex
EOF
}

if [ "${1:-}" = "" ] || [ "${2:-}" = "" ]; then
  usage
  exit 1
fi

BASE_URL="${1%/}"
API_KEY="$2"
MODEL="${3:-gpt-5.3-codex}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

has_jq() {
  command -v jq >/dev/null 2>&1
}

json_extract() {
  local file="$1"
  local expr="$2"
  if has_jq; then
    jq -r "$expr // empty" "$file" 2>/dev/null || true
  fi
}

post_json() {
  local url="$1"
  local body="$2"
  local out="$3"
  curl -sS -o "$out" -w "%{http_code}" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body" \
    "$url"
}

get_json() {
  local url="$1"
  local out="$2"
  curl -sS -o "$out" -w "%{http_code}" \
    -H "Authorization: Bearer ${API_KEY}" \
    "$url"
}

is_reachable_status() {
  local code="$1"
  case "$code" in
    2*|400|401|403|405)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

MODELS_FILE="${TMP_DIR}/models.json"
CHAT_FILE="${TMP_DIR}/chat.json"
RESP_V1_FILE="${TMP_DIR}/resp_v1.json"
RESP_PLAIN_FILE="${TMP_DIR}/resp_plain.json"

ROOT_FOR_V1="$BASE_URL"
RESP_V1_URL="${BASE_URL}/v1/responses"
RESP_PLAIN_URL="${BASE_URL}/responses"

if [[ "$BASE_URL" == */v1/responses ]]; then
  ROOT_FOR_V1="${BASE_URL%/v1/responses}"
  RESP_V1_URL="$BASE_URL"
  RESP_PLAIN_URL="${ROOT_FOR_V1}/responses"
elif [[ "$BASE_URL" == */responses ]]; then
  ROOT_FOR_V1="${BASE_URL%/responses}"
  RESP_PLAIN_URL="$BASE_URL"
  if [[ "$ROOT_FOR_V1" == */v1 ]]; then
    RESP_V1_URL="${ROOT_FOR_V1}/responses"
  else
    RESP_V1_URL="${ROOT_FOR_V1}/v1/responses"
  fi
elif [[ "$BASE_URL" == */v1 ]]; then
  ROOT_FOR_V1="${BASE_URL%/v1}"
  RESP_V1_URL="${BASE_URL}/responses"
  RESP_PLAIN_URL="${ROOT_FOR_V1}/responses"
fi

MODELS_URL="${ROOT_FOR_V1}/v1/models"
CHAT_URL="${ROOT_FOR_V1}/v1/chat/completions"

MODELS_CODE="$(get_json "$MODELS_URL" "$MODELS_FILE")"
CHAT_CODE="$(post_json "$CHAT_URL" "{\"model\":\"${MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"ping\"}],\"max_tokens\":8}" "$CHAT_FILE")"
RESP_V1_CODE="$(post_json "$RESP_V1_URL" "{\"model\":\"${MODEL}\",\"input\":\"ping\",\"max_output_tokens\":8}" "$RESP_V1_FILE")"
RESP_PLAIN_CODE="$(post_json "$RESP_PLAIN_URL" "{\"model\":\"${MODEL}\",\"input\":\"ping\",\"max_output_tokens\":8}" "$RESP_PLAIN_FILE")"

CHAT_ERROR="$(json_extract "$CHAT_FILE" '.error.message')"
RESP_STATUS="$(json_extract "$RESP_V1_FILE" '.status')"
RESP_TEXT="$(json_extract "$RESP_V1_FILE" '.output[0].content[0].text')"
MODELS_COUNT="$(json_extract "$MODELS_FILE" '.data | length')"
SAMPLE_MODELS="$(json_extract "$MODELS_FILE" '.data[0:8] | map(.id) | join(",")')"

RESPONSES_ONLY="no"
if printf '%s' "$CHAT_ERROR" | tr '[:upper:]' '[:lower:]' | grep -q "unsupported legacy protocol"; then
  RESPONSES_ONLY="yes"
fi

RECOMMENDED_BASE_URL="$BASE_URL"
if [[ "$BASE_URL" == */responses ]]; then
  RECOMMENDED_BASE_URL="$BASE_URL"
elif is_reachable_status "$RESP_V1_CODE"; then
  RECOMMENDED_BASE_URL="$RESP_V1_URL"
elif is_reachable_status "$RESP_PLAIN_CODE"; then
  RECOMMENDED_BASE_URL="$RESP_PLAIN_URL"
fi

printf 'base_url=%s\n' "$BASE_URL"
printf 'model=%s\n' "$MODEL"
printf 'models_code=%s\n' "$MODELS_CODE"
printf 'chat_completions_code=%s\n' "$CHAT_CODE"
printf 'responses_v1_code=%s\n' "$RESP_V1_CODE"
printf 'responses_plain_code=%s\n' "$RESP_PLAIN_CODE"
printf 'responses_only=%s\n' "$RESPONSES_ONLY"
printf 'recommended_api_format=responses\n'
printf 'recommended_base_url=%s\n' "$RECOMMENDED_BASE_URL"
printf 'chat_error=%s\n' "${CHAT_ERROR:-"(empty)"}"
printf 'responses_status=%s\n' "${RESP_STATUS:-"(empty)"}"
printf 'responses_text=%s\n' "${RESP_TEXT:-"(empty)"}"
printf 'models_count=%s\n' "${MODELS_COUNT:-"(unknown)"}"
printf 'sample_models=%s\n' "${SAMPLE_MODELS:-"(unknown)"}"
