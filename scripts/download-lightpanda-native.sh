#!/usr/bin/env bash

set -euo pipefail

url="${1:-https://github.com/lightpanda-io/browser/releases/download/nightly/lightpanda-aarch64-macos}"
base="${2:-$HOME/Library/Caches/zimaos-blue/browser/lightpanda/darwin-arm64}"

parts_dir="$base/parts"
out_tmp="$base/lightpanda.download"
out_bin="$base/lightpanda"
part_count=8
subparts=4

total_size="${LIGHTPANDA_TOTAL_SIZE:-}"
if [[ -z "$total_size" ]]; then
  total_size="$(
    curl \
      --silent \
      --show-error \
      --location \
      --head \
      --connect-timeout 10 \
      --max-time 30 \
      "$url" \
      | tr -d '\r' \
      | awk 'tolower($1) == "content-length:" { print $2 }' \
      | tail -n1
  )"
fi

if [[ -z "$total_size" || "$total_size" -le 0 ]]; then
  echo "failed to determine content-length for $url" >&2
  exit 1
fi

file_end=$((total_size - 1))
chunk_size=$(((total_size + part_count - 1) / part_count))
subchunk_size=$(((chunk_size + subparts - 1) / subparts))

mkdir -p "$parts_dir"
rm -f "$out_tmp" "$out_bin"

pending_parts=()
pids=()

for ((i = 0; i < part_count; i++)); do
  part_file="$parts_dir/part.$i"
  part_start=$((i * chunk_size))
  if [[ "$part_start" -gt "$file_end" ]]; then
    break
  fi
  part_end=$((((i + 1) * chunk_size) - 1))
  if [[ "$part_end" -gt "$file_end" ]]; then
    part_end="$file_end"
  fi
  expected_part_size=$((part_end - part_start + 1))
  size=0
  if [[ -f "$part_file" ]]; then
    size=$(stat -f "%z" "$part_file")
  fi
  if [[ "$size" -eq "$expected_part_size" ]]; then
    echo "part $i already complete"
    continue
  fi

  pending_parts+=("$i")
  frag_dir="$parts_dir/part.$i.frags"
  mkdir -p "$frag_dir"
  echo "queueing part $i"

  for ((j = 0; j < subparts; j++)); do
    start=$((i * chunk_size + j * subchunk_size))
    if [[ "$start" -gt "$part_end" ]]; then
      continue
    fi
    end=$((start + subchunk_size - 1))
    if [[ "$end" -gt "$part_end" ]]; then
      end="$part_end"
    fi

    frag_file="$frag_dir/frag.$j"
    expected_size=$((end - start + 1))
    current_size=0
    if [[ -f "$frag_file" ]]; then
      current_size=$(stat -f "%z" "$frag_file")
    fi
    if [[ "$current_size" -eq "$expected_size" ]]; then
      echo "part $i fragment $j already complete"
      continue
    fi

    (
      range_start="$start"
      tmp_file="$frag_file.tmp"
      if [[ "$current_size" -gt 0 ]]; then
        range_start=$((start + current_size))
        echo "part $i fragment $j resuming from byte $range_start"
      fi
      curl \
        --http1.1 \
        --silent \
        --show-error \
        --location \
        --fail \
        --connect-timeout 10 \
        --max-time 300 \
        --retry 5 \
        --retry-all-errors \
        --retry-delay 2 \
        --range "${range_start}-${end}" \
        --output "$tmp_file" \
        "$url"
      if [[ "$current_size" -gt 0 ]]; then
        cat "$tmp_file" >> "$frag_file"
        rm -f "$tmp_file"
      else
        mv "$tmp_file" "$frag_file"
      fi
      actual_size=$(stat -f "%z" "$frag_file")
      if [[ "$actual_size" -ne "$expected_size" ]]; then
        echo "part $i fragment $j size mismatch: $actual_size/$expected_size" >&2
        exit 1
      fi
      echo "part $i fragment $j done"
    ) &
    pids+=("$!")
  done
done

if ((${#pids[@]} > 0)); then
  for pid in "${pids[@]}"; do
    wait "$pid"
  done
fi

if ((${#pending_parts[@]} > 0)); then
  for part in "${pending_parts[@]}"; do
    part_file="$parts_dir/part.$part"
    frag_dir="$parts_dir/part.$part.frags"
    cat "$frag_dir"/frag.* > "$part_file.tmp"
    mv "$part_file.tmp" "$part_file"
    rm -rf "$frag_dir"
    echo "part $part complete: $(stat -f "%z" "$part_file") bytes"
  done
fi

cat "$parts_dir"/part.* > "$out_tmp"
mv "$out_tmp" "$out_bin"
chmod +x "$out_bin"
echo "binary ready: $out_bin"
ls -lh "$out_bin"
