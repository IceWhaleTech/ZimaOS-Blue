#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  verify_provider_pool_responses_e2e.sh \
    --provider-base-url <url> \
    --api-key <key> \
    [--server-url <url>] \
    [--provider-id <id>] \
    [--provider-name <name>] \
    [--model <model>] \
    [--token <jwt>] \
    [--prompt <text>]

Examples:
  verify_provider_pool_responses_e2e.sh \
    --provider-base-url https://example.com \
    --api-key sk-xxxx \
    --model gpt-5.3-codex-spark

  # If server is not in preview mode, pass an existing user JWT:
  verify_provider_pool_responses_e2e.sh \
    --provider-base-url https://example.com \
    --api-key sk-xxxx \
    --token eyJhbGciOiJI...
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

json_error_message() {
  local file="$1"
  jq -r '.error.message // .error // .message // empty' "$file" 2>/dev/null || true
}

SERVER_URL="http://127.0.0.1:18080"
PROVIDER_BASE_URL=""
API_KEY=""
PROVIDER_ID="codex-e2e-$(date +%s)"
PROVIDER_NAME="Codex E2E"
MODEL="gpt-5.3-codex-spark"
TOKEN="${BLUE_TOKEN:-}"
PROMPT="Reply exactly with: CODEX_CONFIG_OK"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-url)
      SERVER_URL="${2:-}"
      shift 2
      ;;
    --provider-base-url)
      PROVIDER_BASE_URL="${2:-}"
      shift 2
      ;;
    --api-key)
      API_KEY="${2:-}"
      shift 2
      ;;
    --provider-id)
      PROVIDER_ID="${2:-}"
      shift 2
      ;;
    --provider-name)
      PROVIDER_NAME="${2:-}"
      shift 2
      ;;
    --model)
      MODEL="${2:-}"
      shift 2
      ;;
    --token)
      TOKEN="${2:-}"
      shift 2
      ;;
    --prompt)
      PROMPT="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$PROVIDER_BASE_URL" || -z "$API_KEY" ]]; then
  usage
  exit 1
fi

require_cmd curl
require_cmd jq

SERVER_URL="${SERVER_URL%/}"
PROVIDER_BASE_URL="${PROVIDER_BASE_URL%/}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "== verify_provider_pool_responses_e2e =="
echo "server_url=${SERVER_URL}"
echo "provider_base_url=${PROVIDER_BASE_URL}"
echo "provider_id=${PROVIDER_ID}"
echo "model=${MODEL}"

if [[ -z "$TOKEN" ]]; then
  echo "-- requesting preview token"
  PREVIEW_TOKEN_FILE="${TMP_DIR}/preview_token.json"
  PREVIEW_CODE="$(curl -sS -o "$PREVIEW_TOKEN_FILE" -w "%{http_code}" \
    -X POST "${SERVER_URL}/api/v1/preview/token")"

  if [[ "$PREVIEW_CODE" == "200" ]]; then
    TOKEN="$(jq -r '.token // empty' "$PREVIEW_TOKEN_FILE")"
  fi

  if [[ -z "$TOKEN" ]]; then
    MSG="$(json_error_message "$PREVIEW_TOKEN_FILE")"
    echo "ERROR: failed to get preview token (http=${PREVIEW_CODE}). ${MSG:-Pass --token when not in preview mode.}" >&2
    exit 1
  fi
else
  echo "-- using provided token"
fi

echo "-- adding provider"
ADD_FILE="${TMP_DIR}/add_provider.json"
ADD_CODE="$(curl -sS -o "$ADD_FILE" -w "%{http_code}" \
  -X POST "${SERVER_URL}/api/v1/providers" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$(jq -nc \
    --arg id "$PROVIDER_ID" \
    --arg name "$PROVIDER_NAME" \
    --arg base_url "$PROVIDER_BASE_URL" \
    --arg api_key "$API_KEY" \
    '{id:$id,name:$name,base_url:$base_url,api_key:$api_key,location:"cloud"}')")"

if [[ "$ADD_CODE" == "409" ]]; then
  echo "-- provider exists, updating base_url + api_format"
  UPDATE_FILE="${TMP_DIR}/update_provider.json"
  UPDATE_CODE="$(curl -sS -o "$UPDATE_FILE" -w "%{http_code}" \
    -X PUT "${SERVER_URL}/api/v1/providers/${PROVIDER_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$(jq -nc \
      --arg name "$PROVIDER_NAME" \
      --arg base_url "$PROVIDER_BASE_URL" \
      '{name:$name,base_url:$base_url,api_format:"responses"}')")"
  if [[ "$UPDATE_CODE" != "200" ]]; then
    MSG="$(json_error_message "$UPDATE_FILE")"
    echo "ERROR: failed to update provider (http=${UPDATE_CODE}): ${MSG}" >&2
    exit 1
  fi
elif [[ "$ADD_CODE" != "201" ]]; then
  MSG="$(json_error_message "$ADD_FILE")"
  echo "ERROR: failed to add provider (http=${ADD_CODE}): ${MSG}" >&2
  exit 1
fi

echo "-- getting provider state"
GET_FILE="${TMP_DIR}/get_provider.json"
GET_CODE="$(curl -sS -o "$GET_FILE" -w "%{http_code}" \
  -H "Authorization: Bearer ${TOKEN}" \
  "${SERVER_URL}/api/v1/providers/${PROVIDER_ID}")"
if [[ "$GET_CODE" != "200" ]]; then
  MSG="$(json_error_message "$GET_FILE")"
  echo "ERROR: failed to get provider (http=${GET_CODE}): ${MSG}" >&2
  exit 1
fi

STATE_BASE_URL="$(jq -r '.provider.base_url // empty' "$GET_FILE")"
STATE_API_FORMAT="$(jq -r '.provider.api_format // empty' "$GET_FILE")"
STATE_DETECTED_FORMAT="$(jq -r '.provider.detected_format // empty' "$GET_FILE")"

echo "-- fetching models"
FETCH_FILE="${TMP_DIR}/fetch_models.json"
FETCH_CODE="$(curl -sS -o "$FETCH_FILE" -w "%{http_code}" \
  -X POST "${SERVER_URL}/api/v1/providers/${PROVIDER_ID}/models/fetch" \
  -H "Authorization: Bearer ${TOKEN}")"
if [[ "$FETCH_CODE" != "200" ]]; then
  MSG="$(json_error_message "$FETCH_FILE")"
  echo "ERROR: failed to fetch models (http=${FETCH_CODE}): ${MSG}" >&2
  exit 1
fi

MODELS_TOTAL="$(jq -r '.total // 0' "$FETCH_FILE")"
MODELS_SAMPLE="$(jq -r '.models[0:8] | map(.id) | join(",")' "$FETCH_FILE")"

echo "-- testing provider health"
TEST_FILE="${TMP_DIR}/provider_test.json"
TEST_CODE="$(curl -sS -o "$TEST_FILE" -w "%{http_code}" \
  -X POST "${SERVER_URL}/api/v1/providers/${PROVIDER_ID}/test" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{}')"
if [[ "$TEST_CODE" != "200" ]]; then
  MSG="$(json_error_message "$TEST_FILE")"
  echo "ERROR: provider test failed (http=${TEST_CODE}): ${MSG}" >&2
  exit 1
fi
HEALTHY="$(jq -r '.healthy // false' "$TEST_FILE")"

echo "-- direct responses inference"
RESP_FILE="${TMP_DIR}/responses_call.json"
RESP_CODE="$(curl -sS -o "$RESP_FILE" -w "%{http_code}" \
  -X POST "${PROVIDER_BASE_URL}/v1/responses" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d "$(jq -nc --arg model "$MODEL" --arg input "$PROMPT" \
    '{model:$model,input:$input}')")"
if [[ "$RESP_CODE" != "200" ]]; then
  MSG="$(json_error_message "$RESP_FILE")"
  echo "ERROR: direct /v1/responses call failed (http=${RESP_CODE}): ${MSG}" >&2
  exit 1
fi
RESP_STATUS="$(jq -r '.status // empty' "$RESP_FILE")"
RESP_TEXT="$(jq -r '.output[0].content[0].text // empty' "$RESP_FILE")"

echo "== summary =="
echo "provider_state.base_url=${STATE_BASE_URL}"
echo "provider_state.api_format=${STATE_API_FORMAT}"
echo "provider_state.detected_format=${STATE_DETECTED_FORMAT}"
echo "models.total=${MODELS_TOTAL}"
echo "models.sample=${MODELS_SAMPLE:-"(empty)"}"
echo "provider_test.healthy=${HEALTHY}"
echo "responses.status=${RESP_STATUS:-"(empty)"}"
echo "responses.text=${RESP_TEXT:-"(empty)"}"

if [[ "$HEALTHY" == "true" && "$MODELS_TOTAL" -gt 0 ]]; then
  echo "result=PASS"
else
  echo "result=FAIL"
  exit 1
fi
