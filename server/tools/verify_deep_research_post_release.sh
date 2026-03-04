#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  verify_deep_research_post_release.sh [options]

Options:
  --server-url <url>          API base URL (default: http://127.0.0.1:18080)
  --token <jwt>               Existing JWT token (fallback: $BLUE_TOKEN, then preview token)
  --query <text>              Research query (default: "OpenAI")
  --mode <fast|standard|deep> Job mode (default: fast)
  --job-timeout-sec <n>       Per-locale timeout seconds (default: 120)
  --poll-interval-sec <n>     Poll interval seconds (default: 2)
  --max-create-retries <n>    Create retries on 429/5xx (default: 4)
  --locales <csv>             Comma-separated locale list (default: built-in 27 locales)
  --output-json <path>        Write summary JSON to file
  --stop-on-fail              Stop immediately on first failed locale
  -h, --help                  Show this help

Examples:
  verify_deep_research_post_release.sh
  verify_deep_research_post_release.sh --server-url http://127.0.0.1:18080 --query "ZimaOS"
  verify_deep_research_post_release.sh --token eyJhbGci... --output-json /tmp/deep-research-selfcheck.json
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
  jq -r '.error // .message // .code // empty' "$file" 2>/dev/null || true
}

SERVER_URL="${SERVER_URL:-http://127.0.0.1:18080}"
TOKEN="${TOKEN:-${BLUE_TOKEN:-}}"
QUERY="${QUERY:-OpenAI}"
MODE="${MODE:-fast}"
JOB_TIMEOUT_SEC="${JOB_TIMEOUT_SEC:-120}"
POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-2}"
MAX_CREATE_RETRIES="${MAX_CREATE_RETRIES:-4}"
STOP_ON_FAIL=0
OUTPUT_JSON="${OUTPUT_JSON:-}"
LOCALES_CSV="${LOCALES:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-url)
      SERVER_URL="${2:-}"
      shift 2
      ;;
    --token)
      TOKEN="${2:-}"
      shift 2
      ;;
    --query)
      QUERY="${2:-}"
      shift 2
      ;;
    --mode)
      MODE="${2:-}"
      shift 2
      ;;
    --job-timeout-sec)
      JOB_TIMEOUT_SEC="${2:-}"
      shift 2
      ;;
    --poll-interval-sec)
      POLL_INTERVAL_SEC="${2:-}"
      shift 2
      ;;
    --max-create-retries)
      MAX_CREATE_RETRIES="${2:-}"
      shift 2
      ;;
    --locales)
      LOCALES_CSV="${2:-}"
      shift 2
      ;;
    --output-json)
      OUTPUT_JSON="${2:-}"
      shift 2
      ;;
    --stop-on-fail)
      STOP_ON_FAIL=1
      shift 1
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

case "$MODE" in
  fast|standard|deep) ;;
  *)
    echo "ERROR: --mode must be one of: fast, standard, deep" >&2
    exit 1
    ;;
esac

require_cmd curl
require_cmd jq

SERVER_URL="${SERVER_URL%/}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

declare -a DEFAULT_LOCALES=(
  "ca-ES"
  "cs-CZ"
  "da-DK"
  "de-DE"
  "el-GR"
  "en-GB"
  "en-US"
  "es-ES"
  "fr-FR"
  "ga-IE"
  "hr-HR"
  "hu-HU"
  "it-IT"
  "ja-JP"
  "ko-KR"
  "ml-IN"
  "nb-NO"
  "nl-NL"
  "pl-PL"
  "pt-BR"
  "pt-PT"
  "ro-RO"
  "ru-RU"
  "sk-SK"
  "sv-SE"
  "zh-CN"
  "zh-TW"
)

declare -A EXPECTED_SOURCE_LABEL=(
  ["ca-ES"]="Fonts"
  ["cs-CZ"]="Zdroje"
  ["da-DK"]="Kilder"
  ["de-DE"]="Quellen"
  ["el-GR"]="Πηγές"
  ["en-GB"]="Sources"
  ["en-US"]="Sources"
  ["es-ES"]="Fuentes"
  ["fr-FR"]="Sources"
  ["ga-IE"]="Foinsí"
  ["hr-HR"]="Izvori"
  ["hu-HU"]="Források"
  ["it-IT"]="Fonti"
  ["ja-JP"]="情報源"
  ["ko-KR"]="출처"
  ["ml-IN"]="ഉറവിടങ്ങൾ"
  ["nb-NO"]="Kilder"
  ["nl-NL"]="Bronnen"
  ["pl-PL"]="Źródła"
  ["pt-BR"]="Fontes"
  ["pt-PT"]="Fontes"
  ["ro-RO"]="Surse"
  ["ru-RU"]="Источники"
  ["sk-SK"]="Zdroje"
  ["sv-SE"]="Källor"
  ["zh-CN"]="来源"
  ["zh-TW"]="來源"
)

declare -a LOCALES=()
if [[ -n "$LOCALES_CSV" ]]; then
  IFS=',' read -r -a LOCALES <<<"$LOCALES_CSV"
else
  LOCALES=("${DEFAULT_LOCALES[@]}")
fi

if [[ "${#LOCALES[@]}" -eq 0 ]]; then
  echo "ERROR: no locales provided" >&2
  exit 1
fi

get_preview_token_if_needed() {
  if [[ -n "$TOKEN" ]]; then
    return 0
  fi
  local out="${TMP_DIR}/preview_token.json"
  local code
  code="$(curl -sS -o "$out" -w "%{http_code}" -X POST "${SERVER_URL}/api/v1/preview/token")"
  if [[ "$code" != "200" ]]; then
    echo "ERROR: failed to get preview token (http=${code}): $(json_error_message "$out")" >&2
    exit 1
  fi
  TOKEN="$(jq -r '.token // empty' "$out")"
  if [[ -z "$TOKEN" ]]; then
    echo "ERROR: preview token endpoint returned empty token" >&2
    exit 1
  fi
}

api_get() {
  local path="$1"
  local out="$2"
  curl -sS -o "$out" -w "%{http_code}" \
    -H "Authorization: Bearer ${TOKEN}" \
    "${SERVER_URL}${path}"
}

api_post_json() {
  local path="$1"
  local body="$2"
  local out="$3"
  curl -sS -o "$out" -w "%{http_code}" \
    -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$body" \
    "${SERVER_URL}${path}"
}

api_patch_json() {
  local path="$1"
  local body="$2"
  local out="$3"
  curl -sS -o "$out" -w "%{http_code}" \
    -X PATCH \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$body" \
    "${SERVER_URL}${path}"
}

verify_v2_default_enabled() {
  local out="${TMP_DIR}/settings_get.json"
  local code
  code="$(api_get "/api/v1/settings" "$out")"
  if [[ "$code" != "200" ]]; then
    echo "ERROR: failed to read settings (http=${code}): $(json_error_message "$out")" >&2
    exit 1
  fi

  local v2
  v2="$(jq -r '.deep_research_v2_enabled // "null"' "$out")"
  if [[ "$v2" == "false" ]]; then
    echo "ERROR: deep_research_v2_enabled is false (expected default true for full release)" >&2
    exit 1
  fi

  local patch_out="${TMP_DIR}/settings_patch.json"
  local patch_code
  patch_code="$(api_patch_json "/api/v1/settings" '{"deep_research_v2_enabled":true}' "$patch_out")"
  if [[ "$patch_code" != "200" ]]; then
    echo "ERROR: failed to enforce deep_research_v2_enabled=true (http=${patch_code}): $(json_error_message "$patch_out")" >&2
    exit 1
  fi
}

create_job_with_retry() {
  local locale="$1"
  local out="$2"
  local payload
  payload="$(jq -nc \
    --arg q "$QUERY" \
    --arg m "$MODE" \
    --arg l "$locale" \
    '{
      query: $q,
      mode: $m,
      lang: $l,
      strict_entity: true,
      report_style: "timeline",
      time_windows: ["Early", "Middle", "Recent"],
      budget: {max_steps: 4, max_sources: 4, max_seconds: 30}
    }')"

  local attempt=1
  local code=""
  while (( attempt <= MAX_CREATE_RETRIES )); do
    code="$(api_post_json "/api/v1/deep-research/jobs" "$payload" "$out")"
    if [[ "$code" == "201" ]]; then
      printf '%s' "$code"
      return 0
    fi
    if [[ "$code" == "429" || "$code" =~ ^5 ]]; then
      sleep $((attempt * 2))
      ((attempt++))
      continue
    fi
    break
  done

  printf '%s' "$code"
  return 1
}

poll_job_terminal() {
  local job_id="$1"
  local out="$2"
  local deadline=$(( $(date +%s) + JOB_TIMEOUT_SEC ))

  while true; do
    local code
    code="$(api_get "/api/v1/deep-research/jobs/${job_id}" "$out")"
    if [[ "$code" != "200" ]]; then
      echo "http_${code}"
      return 1
    fi
    local status
    status="$(jq -r '.status // empty' "$out")"
    case "$status" in
      completed)
        echo "completed"
        return 0
        ;;
      failed|cancelled)
        echo "$status"
        return 1
        ;;
      pending|running|synthesizing)
        ;;
      *)
        echo "unknown_status_${status}"
        return 1
        ;;
    esac

    if (( $(date +%s) >= deadline )); then
      echo "timeout"
      return 1
    fi
    sleep "$POLL_INTERVAL_SEC"
  done
}

validate_completed_job() {
  local locale="$1"
  local job_file="$2"

  local answer
  answer="$(jq -r '.report.answer // empty' "$job_file")"
  if [[ -z "$answer" ]]; then
    echo "missing_report_answer"
    return 1
  fi

  local coverage
  coverage="$(jq -r '.report.citation_coverage // 0' "$job_file")"
  if ! [[ "$coverage" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
    echo "invalid_citation_coverage_${coverage}"
    return 1
  fi
  if ! awk "BEGIN {exit !(${coverage} >= 0 && ${coverage} <= 1)}"; then
    echo "citation_coverage_out_of_range_${coverage}"
    return 1
  fi

  local citations_len
  citations_len="$(jq -r '.report.citations | length // 0' "$job_file")"
  if [[ "$citations_len" =~ ^[0-9]+$ ]] && (( citations_len > 0 )); then
    local expected="${EXPECTED_SOURCE_LABEL[$locale]:-Sources}"
    if ! jq -e --arg label "$expected" '.report.answer | contains("[" + $label + "#1]")' "$job_file" >/dev/null; then
      echo "missing_localized_source_label_${expected}"
      return 1
    fi
  fi

  local report_has_timeline
  report_has_timeline="$(jq -r '.report | has("timeline_sections")' "$job_file")"
  local report_has_stage_errors
  report_has_stage_errors="$(jq -r '.report | has("stage_errors")' "$job_file")"

  if [[ "$report_has_timeline" != "true" && "$report_has_stage_errors" != "true" ]]; then
    echo "missing_v2_optional_sections"
    return 1
  fi

  echo "ok"
  return 0
}

get_preview_token_if_needed
verify_v2_default_enabled

echo "== verify_deep_research_post_release =="
echo "server_url=${SERVER_URL}"
echo "mode=${MODE}"
echo "query=${QUERY}"
echo "locales=${#LOCALES[@]}"
echo "deep_research_v2_enabled=expected_true"

PASS_COUNT=0
FAIL_COUNT=0
RESULTS_JSON="${TMP_DIR}/results.jsonl"
: > "${RESULTS_JSON}"

for locale in "${LOCALES[@]}"; do
  locale="$(echo "$locale" | xargs)"
  if [[ -z "$locale" ]]; then
    continue
  fi

  create_out="${TMP_DIR}/create_${locale}.json"
  create_code="$(create_job_with_retry "$locale" "$create_out" || true)"
  if [[ "$create_code" != "201" ]]; then
    reason="create_failed_http_${create_code}"
    msg="$(json_error_message "$create_out")"
    if [[ -n "$msg" ]]; then
      reason="${reason}:${msg}"
    fi
    echo "FAIL locale=${locale} reason=${reason}"
    jq -nc --arg locale "$locale" --arg status "failed" --arg reason "$reason" \
      '{locale:$locale,status:$status,reason:$reason}' >> "${RESULTS_JSON}"
    ((FAIL_COUNT++))
    if (( STOP_ON_FAIL == 1 )); then
      break
    fi
    continue
  fi

  job_id="$(jq -r '.id // empty' "$create_out")"
  if [[ -z "$job_id" ]]; then
    reason="missing_job_id"
    echo "FAIL locale=${locale} reason=${reason}"
    jq -nc --arg locale "$locale" --arg status "failed" --arg reason "$reason" \
      '{locale:$locale,status:$status,reason:$reason}' >> "${RESULTS_JSON}"
    ((FAIL_COUNT++))
    if (( STOP_ON_FAIL == 1 )); then
      break
    fi
    continue
  fi

  job_out="${TMP_DIR}/job_${locale}.json"
  terminal_status="$(poll_job_terminal "$job_id" "$job_out" || true)"
  if [[ "$terminal_status" != "completed" ]]; then
    reason="terminal_${terminal_status}"
    if [[ -f "$job_out" ]]; then
      err="$(jq -r '.error // empty' "$job_out")"
      if [[ -n "$err" ]]; then
        reason="${reason}:${err}"
      fi
    fi
    echo "FAIL locale=${locale} job=${job_id} reason=${reason}"
    jq -nc --arg locale "$locale" --arg status "failed" --arg reason "$reason" --arg job_id "$job_id" \
      '{locale:$locale,status:$status,job_id:$job_id,reason:$reason}' >> "${RESULTS_JSON}"
    ((FAIL_COUNT++))
    if (( STOP_ON_FAIL == 1 )); then
      break
    fi
    continue
  fi

  validate_reason="$(validate_completed_job "$locale" "$job_out" || true)"
  if [[ "$validate_reason" != "ok" ]]; then
    echo "FAIL locale=${locale} job=${job_id} reason=${validate_reason}"
    jq -nc --arg locale "$locale" --arg status "failed" --arg reason "$validate_reason" --arg job_id "$job_id" \
      '{locale:$locale,status:$status,job_id:$job_id,reason:$reason}' >> "${RESULTS_JSON}"
    ((FAIL_COUNT++))
    if (( STOP_ON_FAIL == 1 )); then
      break
    fi
    continue
  fi

  coverage="$(jq -r '.report.citation_coverage // 0' "$job_out")"
  citations_len="$(jq -r '.report.citations | length // 0' "$job_out")"
  echo "PASS locale=${locale} job=${job_id} citations=${citations_len} citation_coverage=${coverage}"
  jq -nc --arg locale "$locale" --arg status "passed" --arg job_id "$job_id" \
    --argjson citations "${citations_len}" --argjson citation_coverage "${coverage}" \
    '{locale:$locale,status:$status,job_id:$job_id,citations:$citations,citation_coverage:$citation_coverage}' >> "${RESULTS_JSON}"
  ((PASS_COUNT++))
done

SUMMARY_JSON="$(jq -s --arg server_url "$SERVER_URL" --arg query "$QUERY" --arg mode "$MODE" \
  --argjson total "${#LOCALES[@]}" --argjson passed "$PASS_COUNT" --argjson failed "$FAIL_COUNT" \
  '{
    server_url: $server_url,
    query: $query,
    mode: $mode,
    total_locales: $total,
    passed: $passed,
    failed: $failed,
    results: .
  }' "${RESULTS_JSON}")"

echo "== summary =="
echo "$SUMMARY_JSON" | jq .

if [[ -n "$OUTPUT_JSON" ]]; then
  printf '%s\n' "$SUMMARY_JSON" > "$OUTPUT_JSON"
  echo "wrote_summary=${OUTPUT_JSON}"
fi

if (( FAIL_COUNT > 0 )); then
  echo "result=FAIL"
  exit 1
fi

echo "result=PASS"
