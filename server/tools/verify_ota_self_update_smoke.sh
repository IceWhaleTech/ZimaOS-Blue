#!/usr/bin/env bash
set -euo pipefail

# Verifies OTA self-update flow end-to-end and measures downtime.
#
# Optional env:
#   OTA_SMOKE_PORT=18896
#   OTA_SMOKE_PACKAGE_PORT=18897
#   OTA_MAX_DOWNTIME_MS=2000
#   OTA_POLL_INTERVAL_MS=100

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${OTA_SMOKE_PORT:-18896}"
PKG_PORT="${OTA_SMOKE_PACKAGE_PORT:-18897}"
MAX_DOWNTIME_MS="${OTA_MAX_DOWNTIME_MS:-2000}"
POLL_INTERVAL_MS="${OTA_POLL_INTERVAL_MS:-100}"
POLL_SECONDS="$(awk "BEGIN{printf \"%.3f\", ${POLL_INTERVAL_MS}/1000}")"
TMP_DIR="${TMPDIR:-/tmp}/ota-self-update-smoke.$$"

PKG_PID=""
APP_PID=""

now_ms() {
  perl -MTime::HiRes=time -e 'printf("%.0f\n", time()*1000)'
}

cleanup() {
  if [[ -n "${APP_PID}" ]]; then
    kill "${APP_PID}" 2>/dev/null || true
    wait "${APP_PID}" 2>/dev/null || true
  fi
  if [[ -n "${PKG_PID}" ]]; then
    kill "${PKG_PID}" 2>/dev/null || true
    wait "${PKG_PID}" 2>/dev/null || true
  fi
  rm -rf "${TMP_DIR}" 2>/dev/null || true
}
trap cleanup EXIT

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing command: $1" >&2
    exit 1
  fi
}

require_cmd go
require_cmd curl
require_cmd jq
require_cmd perl
require_cmd python3
require_cmd awk

mkdir -p "${TMP_DIR}/data" "${TMP_DIR}/storage" "${TMP_DIR}/pkg"

pushd "${ROOT_DIR}" >/dev/null
go build -ldflags "-X main.version=0.1.0" -o "${TMP_DIR}/blue" ./cmd/ota-smoke-harness
go build -ldflags "-X main.version=0.2.0" -o "${TMP_DIR}/pkg/blue-v2" ./cmd/ota-smoke-harness
popd >/dev/null

cat > "${TMP_DIR}/data/ota_latest.json" <<JSON
{
  "version": "0.2.0",
  "packages": ["http://127.0.0.1:${PKG_PORT}/blue-v2"],
  "release_note_url": "https://example.com/blue/notes.md"
}
JSON
date -u +"%Y-%m-%dT%H:%M:%SZ" > "${TMP_DIR}/data/.last_update_check"

pushd "${TMP_DIR}/pkg" >/dev/null
python3 -m http.server "${PKG_PORT}" --bind 127.0.0.1 > "${TMP_DIR}/pkg-server.log" 2>&1 &
PKG_PID="$!"
popd >/dev/null

OTA_SMOKE_PORT="${PORT}" \
OTA_SMOKE_DATA_DIR="${TMP_DIR}/data" \
OTA_SMOKE_STORAGE_DIR="${TMP_DIR}/storage" \
"${TMP_DIR}/blue" > "${TMP_DIR}/app.log" 2>&1 &
APP_PID="$!"

started="false"
for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${PORT}/api/v1/meta" >/dev/null 2>&1; then
    started="true"
    break
  fi
  sleep 0.1
done

if [[ "${started}" != "true" ]]; then
  echo "failed to start ota smoke harness on port ${PORT}" >&2
  echo "app log tail:" >&2
  tail -n 50 "${TMP_DIR}/app.log" >&2 || true
  exit 1
fi

meta_before="$(curl -fsS "http://127.0.0.1:${PORT}/api/v1/meta")"
before_version="$(jq -r '.version' <<<"${meta_before}")"
before_pid="$(jq -r '.pid' <<<"${meta_before}")"
before_start_ts="$(jq -r '.blue_start_ts // ""' <<<"${meta_before}")"

if [[ "${before_version}" != "0.1.0" ]]; then
  echo "unexpected initial version: ${before_version}" >&2
  exit 1
fi

curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v1/system/update/download-ota" >/dev/null

download_ok="false"
for _ in $(seq 1 200); do
  info="$(curl -fsS "http://127.0.0.1:${PORT}/api/v1/system/update/info")"
  state="$(jq -r '.status.state' <<<"${info}")"
  downloaded_path="$(jq -r '.status.downloaded_path // ""' <<<"${info}")"
  err_msg="$(jq -r '.status.error // ""' <<<"${info}")"
  if [[ "${state}" == "failed" ]]; then
    echo "download failed: ${err_msg}" >&2
    exit 1
  fi
  if [[ "${downloaded_path}" != "" ]]; then
    download_ok="true"
    break
  fi
  sleep 0.05
done

if [[ "${download_ok}" != "true" ]]; then
  echo "download timed out" >&2
  exit 1
fi

apply_start_ms="$(now_ms)"
curl -sS -X POST "http://127.0.0.1:${PORT}/api/v1/system/update/apply" >/dev/null || true

downtime_start_ms=""
downtime_end_ms=""
new_version_ms=""
meta_after=""

for _ in $(seq 1 400); do
  ts="$(now_ms)"
  resp="$(curl -s -m 0.3 "http://127.0.0.1:${PORT}/api/v1/meta" || true)"
  if [[ -z "${resp}" ]]; then
    if [[ -z "${downtime_start_ms}" ]]; then
      downtime_start_ms="${ts}"
    fi
  else
    ver="$(jq -r '.version // ""' <<<"${resp}")"
    if [[ "${ver}" == "0.2.0" ]]; then
      new_version_ms="${ts}"
      if [[ -n "${downtime_start_ms}" && -z "${downtime_end_ms}" ]]; then
        downtime_end_ms="${ts}"
      fi
      meta_after="${resp}"
      break
    fi
  fi
  sleep "${POLL_SECONDS}"
done

if [[ -z "${new_version_ms}" ]]; then
  echo "new version did not become healthy in time" >&2
  echo "app log tail:" >&2
  tail -n 50 "${TMP_DIR}/app.log" >&2 || true
  exit 1
fi

if [[ -z "${downtime_start_ms}" ]]; then
  downtime_ms=0
else
  if [[ -z "${downtime_end_ms}" ]]; then
    downtime_end_ms="${new_version_ms}"
  fi
  downtime_ms=$((downtime_end_ms - downtime_start_ms))
fi

switch_latency_ms=$((new_version_ms - apply_start_ms))

after_pid="$(jq -r '.pid' <<<"${meta_after}")"
after_start_ts="$(jq -r '.blue_start_ts // ""' <<<"${meta_after}")"

echo "OTA self-update smoke passed"
echo "from_version=0.1.0 to_version=0.2.0"
echo "pid_before=${before_pid} pid_after=${after_pid}"
echo "blue_start_ts_before=${before_start_ts} blue_start_ts_after=${after_start_ts}"
echo "switch_latency_ms=${switch_latency_ms}"
echo "downtime_ms=${downtime_ms} (threshold=${MAX_DOWNTIME_MS})"

if [[ "${before_start_ts}" != "" && "${after_start_ts}" != "" && "${before_start_ts}" != "${after_start_ts}" ]]; then
  echo "start timestamp not preserved across restart" >&2
  exit 1
fi

if [[ "${after_start_ts}" == "" ]]; then
  echo "BLUE_START_TIME missing after restart" >&2
  exit 1
fi

if (( downtime_ms > MAX_DOWNTIME_MS )); then
  echo "downtime too high: ${downtime_ms}ms > ${MAX_DOWNTIME_MS}ms" >&2
  exit 1
fi
