#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for autoresearch wrapper" >&2
  exit 1
fi

payload="$(cat)"
query="$(printf '%s' "$payload" | jq -r '.query // empty')"
mode="$(printf '%s' "$payload" | jq -r '.mode // "standard"')"
route_mode="$(printf '%s' "$payload" | jq -r '.route_mode // "experiment"')"
lang="$(printf '%s' "$payload" | jq -r '.lang // empty')"
artifact_dir="$(printf '%s' "$payload" | jq -r '.artifact_dir // empty')"
if [ -z "$artifact_dir" ] || [ "$artifact_dir" = "null" ]; then
  artifact_dir="./artifacts"
fi
mkdir -p "$artifact_dir"

timestamp="$(date -u '+%Y%m%dT%H%M%SZ')"
request_path="${artifact_dir}/request-${timestamp}.json"
log_path="${artifact_dir}/train-${timestamp}.log"
printf '%s\n' "$payload" >"$request_path"

repo_dir="${AUTORESEARCH_REPO:-${BLUE_AUTORESEARCH_REPO:-.}}"
repo_dir="${repo_dir%/}"
run_cmd="${AUTORESEARCH_RUN_CMD:-uv run train.py}"
prepare_cmd="${AUTORESEARCH_PREPARE_CMD:-}"

if [ ! -d "$repo_dir" ]; then
  echo "autoresearch repo directory does not exist: $repo_dir" >&2
  exit 1
fi
if [ ! -f "$repo_dir/train.py" ]; then
  echo "train.py not found under $repo_dir (set AUTORESEARCH_REPO to karpathy/autoresearch checkout)" >&2
  exit 1
fi

if [ -n "$prepare_cmd" ]; then
  (
    cd "$repo_dir"
    bash -lc "$prepare_cmd"
  ) >>"$log_path" 2>&1
fi

set +e
(
  cd "$repo_dir"
  export BLUE_RESEARCH_QUERY="$query"
  export BLUE_RESEARCH_MODE="$mode"
  export BLUE_RESEARCH_ROUTE_MODE="$route_mode"
  if [ -n "$lang" ] && [ "$lang" != "null" ]; then
    export BLUE_RESEARCH_LANG="$lang"
  fi
  printf 'BLUE_RESEARCH_QUERY=%s\n' "$BLUE_RESEARCH_QUERY"
  printf 'BLUE_RESEARCH_MODE=%s\n' "$BLUE_RESEARCH_MODE"
  printf 'BLUE_RESEARCH_ROUTE_MODE=%s\n' "$BLUE_RESEARCH_ROUTE_MODE"
  if [ -n "${BLUE_RESEARCH_LANG:-}" ]; then
    printf 'BLUE_RESEARCH_LANG=%s\n' "$BLUE_RESEARCH_LANG"
  fi
  bash -lc "$run_cmd"
) >>"$log_path" 2>&1
run_status=$?
set -e

if [ "$run_status" -ne 0 ]; then
  tail -n 200 "$log_path" >&2 || true
  exit "$run_status"
fi

extract_metric() {
  local key="$1"
  awk -v key="$key" '
    $0 ~ ("^" key ":") {
      line=$0
      sub("^" key ":[[:space:]]*", "", line)
      value=line
    }
    END {
      if (value != "") {
        print value
      }
    }
  ' "$log_path"
}

val_bpb="$(extract_metric "val_bpb")"
num_steps="$(extract_metric "num_steps")"
total_seconds="$(extract_metric "total_seconds")"
peak_vram_mb="$(extract_metric "peak_vram_mb")"
mfu_percent="$(extract_metric "mfu_percent")"

summary="autoresearch experiment finished for: ${query}"
if [ -n "$val_bpb" ]; then
  summary="${summary} (val_bpb=${val_bpb})"
fi

jq -n \
  --arg summary "$summary" \
  --arg query "$query" \
  --arg mode "$mode" \
  --arg route_mode "$route_mode" \
  --arg repo_dir "$repo_dir" \
  --arg run_cmd "$run_cmd" \
  --arg log_path "$log_path" \
  --arg request_path "$request_path" \
  --arg val_bpb "$val_bpb" \
  --arg num_steps "$num_steps" \
  --arg total_seconds "$total_seconds" \
  --arg peak_vram_mb "$peak_vram_mb" \
  --arg mfu_percent "$mfu_percent" \
  '
  def maybe_num:
    if . == "" then null else (tonumber? // null) end;
  {
    summary: $summary,
    confidence: (if $val_bpb == "" then 0.45 else 0.72 end),
    findings: (
      [
        "command: \($run_cmd)",
        (if $val_bpb == "" then empty else "val_bpb=\($val_bpb)" end),
        (if $num_steps == "" then empty else "num_steps=\($num_steps)" end),
        (if $total_seconds == "" then empty else "total_seconds=\($total_seconds)" end),
        (if $peak_vram_mb == "" then empty else "peak_vram_mb=\($peak_vram_mb)" end),
        (if $mfu_percent == "" then empty else "mfu_percent=\($mfu_percent)" end)
      ]
    ),
    artifacts: [
      {label: "request_payload", kind: "file", path: $request_path},
      {label: "train_log", kind: "file", path: $log_path}
    ],
    open_questions: (
      if $val_bpb == "" then
        ["No val_bpb metric parsed; adjust AUTORESEARCH_RUN_CMD or verify training log format."]
      else
        []
      end
    ),
    metadata: {
      backend: "autoresearch-wrapper",
      repo_dir: $repo_dir,
      command: $run_cmd,
      query: $query,
      mode: $mode,
      route_mode: $route_mode,
      metrics: {
        val_bpb: ($val_bpb | maybe_num),
        num_steps: ($num_steps | maybe_num),
        total_seconds: ($total_seconds | maybe_num),
        peak_vram_mb: ($peak_vram_mb | maybe_num),
        mfu_percent: ($mfu_percent | maybe_num)
      }
    }
  }'
