#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Dict, List, Sequence
from urllib import request


PINCHBENCH_RUNS_URL = "https://pinchbench.com/runs"
PINCHBENCH_BASE_URL = "https://pinchbench.com"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Fetch PinchBench /runs and emit verified seed rows for provider catalog maintenance."
    )
    parser.add_argument(
        "--url",
        default=PINCHBENCH_RUNS_URL,
        help="PinchBench runs page URL.",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=20.0,
        help="HTTP timeout in seconds.",
    )
    parser.add_argument(
        "--catalog",
        default="docs/provider_catalog.json",
        help="Existing provider catalog JSON for comparison.",
    )
    parser.add_argument(
        "--format",
        choices=("json", "go"),
        default="json",
        help="Output format.",
    )
    parser.add_argument(
        "--output",
        default="",
        help="Optional output file path. Prints to stdout when omitted.",
    )
    return parser.parse_args()


def fetch_runs_html(url: str, timeout: float) -> str:
    with request.urlopen(url, timeout=timeout) as response:
        return response.read().decode("utf-8", errors="replace")


def parse_runs_html(html: str) -> List[Dict[str, Any]]:
    quote = r'\\?"'
    href_re = re.compile(rf'{quote}href{quote}:{quote}(?P<href>/submission/[0-9a-f-]+){quote}')
    token_chars = r"[A-Za-z0-9._:+/-]+"
    model_re = re.compile(rf'{quote}children{quote}:\[{quote}(?P<model>{token_chars}){quote},false\]')
    provider_re = re.compile(rf'{quote}children{quote}:{quote}(?P<provider>{token_chars}){quote}')
    score_re = re.compile(rf'{quote}children{quote}:\[{quote}(?P<score>\d+(?:\.\d+)?){quote},{quote}%{quote}\]')

    rows_by_id: Dict[str, Dict[str, Any]] = {}

    for href_match in href_re.finditer(html):
        segment = html[href_match.start() : href_match.start() + 1200]

        model_match = model_re.search(segment)
        if not model_match:
            continue
        provider_match = provider_re.search(segment, model_match.end())
        score_match = score_re.search(segment, model_match.end())
        if not model_match or not provider_match or not score_match:
            continue

        model_id = model_match.group("model")
        row = {
            "id": model_id,
            "provider": provider_match.group("provider"),
            "score": float(score_match.group("score")),
            "url": f"{PINCHBENCH_BASE_URL}{href_match.group('href')}",
        }

        existing = rows_by_id.get(model_id)
        if existing is None or row["score"] > existing["score"]:
            rows_by_id[model_id] = row

    return [rows_by_id[key] for key in sorted(rows_by_id)]


def compare_rows_to_catalog(rows: Sequence[Dict[str, Any]], catalog_path: Path) -> Dict[str, Any]:
    payload = json.loads(catalog_path.read_text(encoding="utf-8"))
    verified_catalog = {
        model["id"]: model["pinchbench_url"]
        for models in payload.get("pinchbench_models", {}).values()
        for model in models
        if model.get("pinchbench_url")
    }
    fetched_rows = {row["id"]: row["url"] for row in rows}

    matching_verified_ids: List[str] = []
    newly_verified_ids: List[str] = []
    changed_verified_ids: List[Dict[str, str]] = []

    for row in rows:
        model_id = row["id"]
        fetched_url = row["url"]
        catalog_url = verified_catalog.get(model_id)

        if not catalog_url:
            newly_verified_ids.append(model_id)
            continue
        if catalog_url == fetched_url:
            matching_verified_ids.append(model_id)
            continue

        changed_verified_ids.append(
            {
                "id": model_id,
                "catalog_url": catalog_url,
                "fetched_url": fetched_url,
            }
        )

    catalog_only_verified_ids = sorted(
        model_id for model_id in verified_catalog if model_id not in fetched_rows
    )

    return {
        "fetched_rows": len(rows),
        "catalog_verified_rows": len(verified_catalog),
        "matching_verified_ids": sorted(matching_verified_ids),
        "newly_verified_ids": sorted(newly_verified_ids),
        "catalog_only_verified_ids": catalog_only_verified_ids,
        "changed_verified_ids": sorted(changed_verified_ids, key=lambda item: item["id"]),
    }


def format_go_seed_entries(rows: Sequence[Dict[str, Any]]) -> str:
    lines = []
    for row in rows:
        lines.append(
            f'{{ID: "{row["id"]}", Score: {row["score"]:.1f}, URL: "{row["url"]}"}},'
        )
    return "\n".join(lines)


def build_output(rows: Sequence[Dict[str, Any]], summary: Dict[str, Any], fmt: str, source_url: str) -> str:
    if fmt == "go":
        return format_go_seed_entries(rows) + "\n"

    payload = {
        "source_url": source_url,
        "verified_rows": list(rows),
        "summary": summary,
    }
    return json.dumps(payload, indent=2, ensure_ascii=False) + "\n"


def main() -> int:
    args = parse_args()
    html = fetch_runs_html(args.url, args.timeout)
    rows = parse_runs_html(html)
    summary = compare_rows_to_catalog(rows, Path(args.catalog))
    output = build_output(rows, summary, args.format, args.url)

    if args.output:
        Path(args.output).write_text(output, encoding="utf-8")
    else:
        sys.stdout.write(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
