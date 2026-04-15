#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Summarize A11y quick-path Harness report success and accuracy.")
    parser.add_argument("report", help="Path to a Harness eval report JSON file.")
    parser.add_argument(
        "--output-json",
        default="",
        help="Optional path for the structured JSON summary.",
    )
    parser.add_argument(
        "--output-md",
        default="",
        help="Optional path for the markdown summary.",
    )
    parser.add_argument(
        "--title",
        default="A11y Quick Paths Report",
        help="Markdown report title.",
    )
    return parser.parse_args()


def load_json(path: Path) -> Dict[str, Any]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    return payload if isinstance(payload, dict) else {}


def decode_json_map(raw: Any) -> Dict[str, Any]:
    if isinstance(raw, dict):
        return dict(raw)
    if not isinstance(raw, str):
        return {}
    text = raw.strip()
    if not text:
        return {}
    try:
        decoded = json.loads(text)
    except json.JSONDecodeError:
        return {}
    return decoded if isinstance(decoded, dict) else {}


def metadata_map(raw: Any) -> Dict[str, Any]:
    return dict(raw) if isinstance(raw, dict) else {}


def text_value(raw: Any) -> str:
    if isinstance(raw, str):
        return raw.strip()
    if raw is None:
        return ""
    return str(raw).strip()


def bool_value(raw: Any) -> Optional[bool]:
    if isinstance(raw, bool):
        return raw
    if isinstance(raw, (int, float)):
        return bool(raw)
    if isinstance(raw, str):
        lowered = raw.strip().lower()
        if lowered in {"true", "1", "yes", "y"}:
            return True
        if lowered in {"false", "0", "no", "n"}:
            return False
    return None


def number_value(raw: Any) -> Optional[float]:
    if isinstance(raw, bool):
        return None
    if isinstance(raw, (int, float)):
        return float(raw)
    if isinstance(raw, str):
        text = raw.strip()
        if not text:
            return None
        try:
            return float(text)
        except ValueError:
            return None
    return None


def first_bool(*sources: Dict[str, Any], keys: str) -> Optional[bool]:
    for source in sources:
        if not source:
            continue
        for key in keys:
            if key not in source:
                continue
            value = bool_value(source.get(key))
            if value is not None:
                return value
    return None


def first_text(*sources: Dict[str, Any], keys: str) -> str:
    for source in sources:
        if not source:
            continue
        for key in keys:
            value = text_value(source.get(key))
            if value:
                return value
    return ""


def first_number(*sources: Dict[str, Any], keys: str) -> Optional[float]:
    for source in sources:
        if not source:
            continue
        for key in keys:
            if key not in source:
                continue
            value = number_value(source.get(key))
            if value is not None:
                return value
    return None


def latest_by_key(rows: List[Dict[str, Any]], key_name: str) -> Dict[str, Dict[str, Any]]:
    latest: Dict[str, Dict[str, Any]] = {}
    for row in rows:
        key = text_value(row.get(key_name))
        if not key:
            continue
        latest[key] = row
    return latest


def normalize_os(item: Dict[str, Any]) -> str:
    metadata = metadata_map(item.get("metadata"))
    for key in ("os", "platform", "host_os"):
        value = text_value(metadata.get(key))
        if value:
            return value.lower()
    return "unknown"


def case_goal(item: Dict[str, Any]) -> str:
    input_map = metadata_map(item.get("input"))
    return first_text(input_map, keys=("goal", "prompt", "query"))


def iso_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def percentile(values: List[float], ratio: float) -> Optional[float]:
    if not values:
        return None
    ordered = sorted(values)
    index = max(0, min(len(ordered) - 1, math.ceil(ratio * len(ordered)) - 1))
    return ordered[index]


def percentile_summary(values: List[float]) -> Dict[str, Any]:
    cleaned = [float(value) for value in values if value is not None]
    if not cleaned:
        return {"count": 0}
    return {
        "count": len(cleaned),
        "p50": int(percentile(cleaned, 0.50)),
        "p90": int(percentile(cleaned, 0.90)),
        "p99": int(percentile(cleaned, 0.99)),
        "min": int(min(cleaned)),
        "max": int(max(cleaned)),
    }


def extract_timestamp(raw: Any) -> Optional[datetime]:
    text = text_value(raw)
    if not text:
        return None
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        return datetime.fromisoformat(text)
    except ValueError:
        return None


def approx_llm_latency_ms(run: Dict[str, Any], result_map: Dict[str, Any]) -> Optional[float]:
    explicit = first_number(result_map, keys=("llm_latency_ms", "model_latency_ms", "think_time_ms"))
    if explicit is not None:
        return explicit

    audit = metadata_map(run.get("audit") or run.get("timing") or run.get("timestamps"))
    started = extract_timestamp(audit.get("started_at") or run.get("started_at") or run.get("created_at"))
    completed = extract_timestamp(audit.get("completed_at") or run.get("completed_at") or run.get("finished_at"))
    native_end_to_end = first_number(result_map, keys=("end_to_end_ms", "native_end_to_end_ms"))
    if started and completed and native_end_to_end is not None:
        total_ms = (completed - started).total_seconds() * 1000.0
        if total_ms >= native_end_to_end:
            return total_ms - native_end_to_end
    return None


def build_summary(report_path: Path) -> Dict[str, Any]:
    payload = load_json(report_path)
    group_report = metadata_map(payload.get("group_report") or payload.get("groupReport"))
    eval_run = metadata_map(payload.get("eval_run") or payload.get("evalRun"))

    items = [item for item in group_report.get("items", []) if isinstance(item, dict)]
    runs = [run for run in group_report.get("linked_runs", []) if isinstance(run, dict)]
    scorecards = [card for card in group_report.get("scorecards", []) if isinstance(card, dict)]

    item_by_id = latest_by_key(items, "id")
    run_by_item = latest_by_key(runs, "group_item_id")
    run_by_id = latest_by_key(runs, "id")
    scorecard_by_item = latest_by_key(scorecards, "group_item_id")

    item_ids = set(item_by_id.keys()) | set(run_by_item.keys()) | set(scorecard_by_item.keys())
    cases: List[Dict[str, Any]] = []
    by_os: Dict[str, Dict[str, Any]] = {}
    latency_samples: Dict[str, List[float]] = {
        "native_end_to_end_ms": [],
        "tree_fetch_ms": [],
        "tree_serialize_ms": [],
        "query_ms": [],
        "action_ms": [],
        "verification_ms": [],
        "llm_latency_ms": [],
    }
    cache_hits = 0
    cache_known = 0
    fallback_cases = 0

    for item_id in sorted(item_ids):
        item = item_by_id.get(item_id, {"id": item_id})
        run = run_by_item.get(item_id, {})
        scorecard = scorecard_by_item.get(item_id, {})
        if not run and text_value(scorecard.get("run_id")):
            run = run_by_id.get(text_value(scorecard.get("run_id")), {})

        result_map = decode_json_map(run.get("result"))
        breakdown_map = decode_json_map(scorecard.get("breakdown_json") or scorecard.get("breakdown"))

        target_hit = first_bool(result_map, breakdown_map, keys=("target_hit",))
        verification_passed = first_bool(result_map, breakdown_map, keys=("verification_passed",))
        cache_hit = first_bool(result_map, breakdown_map, keys=("cache_hit",))
        fallback_list = result_map.get("fallbacks")
        if not isinstance(fallback_list, list):
            fallback_list = []
        fallback_values = [text_value(value) for value in fallback_list if text_value(value)]
        native_end_to_end = first_number(result_map, breakdown_map, keys=("end_to_end_ms", "native_end_to_end_ms"))
        tree_fetch_ms = first_number(result_map, breakdown_map, keys=("tree_fetch_ms",))
        tree_serialize_ms = first_number(result_map, breakdown_map, keys=("tree_serialize_ms",))
        query_ms = first_number(result_map, breakdown_map, keys=("query_ms",))
        action_ms = first_number(result_map, breakdown_map, keys=("action_ms",))
        verification_ms = first_number(result_map, breakdown_map, keys=("verification_ms",))
        llm_latency_ms = approx_llm_latency_ms(run, result_map)
        case = {
            "item_id": item_id,
            "os": normalize_os(item),
            "goal": case_goal(item),
            "run_id": text_value(run.get("id") or scorecard.get("run_id")),
            "scorecard_id": text_value(scorecard.get("id")),
            "verdict": text_value(scorecard.get("verdict")),
            "target_hit": target_hit,
            "verification_passed": verification_passed,
            "verification_method": first_text(result_map, breakdown_map, keys=("verification_method",)),
            "input_method": first_text(result_map, breakdown_map, keys=("input_method",)),
            "overlay_mode": first_text(result_map, breakdown_map, keys=("overlay_mode",)),
            "cache_hit": cache_hit,
            "fallbacks": fallback_values,
            "end_to_end_ms": native_end_to_end,
            "tree_fetch_ms": tree_fetch_ms,
            "tree_serialize_ms": tree_serialize_ms,
            "query_ms": query_ms,
            "action_ms": action_ms,
            "verification_ms": verification_ms,
            "llm_latency_ms": llm_latency_ms,
        }
        cases.append(case)

        if cache_hit is not None:
            cache_known += 1
            if cache_hit:
                cache_hits += 1
        if fallback_values:
            fallback_cases += 1
        if native_end_to_end is not None:
            latency_samples["native_end_to_end_ms"].append(native_end_to_end)
        if tree_fetch_ms is not None:
            latency_samples["tree_fetch_ms"].append(tree_fetch_ms)
        if tree_serialize_ms is not None:
            latency_samples["tree_serialize_ms"].append(tree_serialize_ms)
        if query_ms is not None:
            latency_samples["query_ms"].append(query_ms)
        if action_ms is not None:
            latency_samples["action_ms"].append(action_ms)
        if verification_ms is not None:
            latency_samples["verification_ms"].append(verification_ms)
        if llm_latency_ms is not None:
            latency_samples["llm_latency_ms"].append(llm_latency_ms)

        bucket = by_os.setdefault(
            case["os"],
            {
                "case_count": 0,
                "success_count": 0,
                "accuracy_count": 0,
                "success_rate": 0.0,
                "accuracy_rate": 0.0,
            },
        )
        bucket["case_count"] += 1
        if verification_passed is True:
            bucket["success_count"] += 1
        if target_hit is True:
            bucket["accuracy_count"] += 1

    case_count = len(cases)
    success_count = sum(1 for case in cases if case["verification_passed"] is True)
    accuracy_count = sum(1 for case in cases if case["target_hit"] is True)

    for bucket in by_os.values():
        if bucket["case_count"] > 0:
            bucket["success_rate"] = bucket["success_count"] / bucket["case_count"]
            bucket["accuracy_rate"] = bucket["accuracy_count"] / bucket["case_count"]

    return {
        "generated_at": iso_now(),
        "source_report": str(report_path.resolve()),
        "eval_run_id": text_value(eval_run.get("id")),
        "eval_run_title": text_value(eval_run.get("title")),
        "model": first_text(eval_run, payload, keys=("model",)),
        "provider_id": first_text(eval_run, payload, keys=("provider_id", "providerId")),
        "group_id": first_text(metadata_map(group_report.get("group")), keys=("id",)),
        "group_title": first_text(metadata_map(group_report.get("group")), keys=("title",)),
        "case_count": case_count,
        "success_count": success_count,
        "accuracy_count": accuracy_count,
        "success_rate": (success_count / case_count) if case_count else 0.0,
        "accuracy_rate": (accuracy_count / case_count) if case_count else 0.0,
        "cache_hit_rate": (cache_hits / cache_known) if cache_known else 0.0,
        "fallback_rate": (fallback_cases / case_count) if case_count else 0.0,
        "latency": {name: percentile_summary(values) for name, values in latency_samples.items()},
        "by_os": by_os,
        "cases": cases,
    }


def format_rate(value: float) -> str:
    return f"{value * 100:.1f}%"


def build_markdown(summary: Dict[str, Any], title: str) -> str:
    lines = [f"# {title}", ""]
    lines.append(f"- Eval run: `{summary.get('eval_run_id', '')}`")
    lines.append(f"- Model: `{summary.get('model', '')}`")
    lines.append(f"- Provider: `{summary.get('provider_id', '')}`")
    lines.append(f"- Cases: `{summary.get('case_count', 0)}`")
    lines.append(f"- Success rate: `{format_rate(float(summary.get('success_rate', 0.0)))}`")
    lines.append(f"- Accuracy rate: `{format_rate(float(summary.get('accuracy_rate', 0.0)))}`")
    lines.append(f"- Cache hit rate: `{format_rate(float(summary.get('cache_hit_rate', 0.0)))}`")
    lines.append(f"- Fallback rate: `{format_rate(float(summary.get('fallback_rate', 0.0)))}`")
    lines.append("")
    lines.append("## By OS")
    lines.append("")
    lines.append("| OS | Cases | Success | Accuracy |")
    lines.append("| --- | ---: | ---: | ---: |")
    for os_name in sorted(summary.get("by_os", {}).keys()):
        bucket = summary["by_os"][os_name]
        lines.append(
            f"| {os_name} | {bucket['case_count']} | {format_rate(bucket['success_rate'])} | {format_rate(bucket['accuracy_rate'])} |"
        )
    lines.append("")
    lines.append("## Latency")
    lines.append("")
    lines.append("| Metric | P50 | P90 | P99 | Count |")
    lines.append("| --- | ---: | ---: | ---: | ---: |")
    for metric_name in sorted(summary.get("latency", {}).keys()):
        metric = summary["latency"][metric_name]
        lines.append(
            f"| {metric_name} | {metric.get('p50', '')} | {metric.get('p90', '')} | {metric.get('p99', '')} | {metric.get('count', 0)} |"
        )
    lines.append("")
    lines.append("## Cases")
    lines.append("")
    lines.append("| Case | OS | Target Hit | Verification Passed | Cache Hit | Fallbacks | Method | Input | Verdict |")
    lines.append("| --- | --- | --- | --- | --- | --- | --- | --- | --- |")
    for case in summary.get("cases", []):
        lines.append(
            "| {item_id} | {os} | {target_hit} | {verification_passed} | {cache_hit} | {fallbacks} | {verification_method} | {input_method} | {verdict} |".format(
                item_id=case.get("item_id", ""),
                os=case.get("os", ""),
                target_hit=case.get("target_hit"),
                verification_passed=case.get("verification_passed"),
                cache_hit=case.get("cache_hit"),
                fallbacks=", ".join(case.get("fallbacks", [])),
                verification_method=case.get("verification_method", ""),
                input_method=case.get("input_method", ""),
                verdict=case.get("verdict", ""),
            )
        )
    lines.append("")
    return "\n".join(lines)


def write_text(path: str, content: str) -> None:
    if not path:
        return
    output = Path(path)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(content, encoding="utf-8")


def main() -> int:
    args = parse_args()
    report_path = Path(args.report).resolve()
    summary = build_summary(report_path)
    summary_json = json.dumps(summary, ensure_ascii=False, indent=2) + "\n"
    summary_md = build_markdown(summary, args.title)

    if args.output_json:
        write_text(args.output_json, summary_json)
    if args.output_md:
        write_text(args.output_md, summary_md)

    if not args.output_json and not args.output_md:
        print(summary_json, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
