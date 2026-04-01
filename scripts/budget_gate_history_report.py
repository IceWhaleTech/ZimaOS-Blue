#!/usr/bin/env python3
from __future__ import annotations

import argparse
import glob
import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence


@dataclass
class GateAttempt:
    candidate_id: str
    candidate_label: str
    generated_at: str
    generated_at_sort: datetime
    report_path: str
    passed: bool
    status: str
    eval_run_id: str
    case_count: Optional[int]
    comparable_case_count: Optional[int]
    missing_surface_case_count: Optional[int]
    median_schema_byte_reduction_rate: Optional[float]
    median_latency_increase_rate: Optional[float]
    allowed_final_native_tool_case_rate: Optional[float]
    non_allowed_native_tool_case_count: Optional[int]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Summarize budget gate history and evaluate consecutive-green requirements.")
    parser.add_argument(
        "--reports",
        nargs="+",
        default=["docs/reports/budget_gate_report*.json"],
        help="One or more budget gate report files or glob patterns.",
    )
    parser.add_argument(
        "--candidate-id",
        default="",
        help="Candidate id to evaluate. Takes precedence over --candidate-label.",
    )
    parser.add_argument(
        "--candidate-label",
        default="",
        help="Candidate label to evaluate. Defaults to the latest available candidate.",
    )
    parser.add_argument(
        "--require-consecutive-green",
        type=int,
        default=0,
        help="If set, fail unless the focused candidate has at least this many consecutive passing attempts.",
    )
    parser.add_argument(
        "--output-md",
        default="docs/reports/budget_gate_history_report.md",
        help="Markdown summary output path.",
    )
    parser.add_argument(
        "--output-json",
        default="docs/reports/budget_gate_history_report.json",
        help="Structured JSON summary output path.",
    )
    parser.add_argument(
        "--title",
        default="Budget Gate History Summary",
        help="Markdown title.",
    )
    return parser.parse_args()


def parse_timestamp(raw: str, fallback_path: Path) -> datetime:
    text = str(raw or "").strip()
    if text:
        try:
            if text.endswith("Z"):
                return datetime.fromisoformat(text.replace("Z", "+00:00"))
            return datetime.fromisoformat(text)
        except ValueError:
            pass
    stat = fallback_path.stat()
    return datetime.fromtimestamp(stat.st_mtime, tz=timezone.utc)


def expand_report_paths(patterns: Sequence[str]) -> List[Path]:
    paths: List[Path] = []
    seen = set()
    for pattern in patterns:
        matches = sorted(Path(match) for match in glob.glob(pattern, recursive=True))
        if matches:
            for match in matches:
                resolved = match.resolve()
                key = str(resolved)
                if key in seen or not resolved.is_file():
                    continue
                seen.add(key)
                paths.append(resolved)
            continue
        candidate = Path(pattern).resolve()
        if candidate.is_file():
            key = str(candidate)
            if key not in seen:
                seen.add(key)
                paths.append(candidate)
    return paths


def as_float(value: Any) -> Optional[float]:
    if isinstance(value, (int, float)):
        return float(value)
    return None


def as_int(value: Any) -> Optional[int]:
    if isinstance(value, bool):
        return int(value)
    if isinstance(value, int):
        return value
    return None


def extract_candidate_id(payload: Dict[str, Any], eval_run: Dict[str, Any]) -> str:
    candidate_id = str(payload.get("candidate_id") or "").strip()
    if candidate_id:
        return candidate_id
    metadata = eval_run.get("metadata") if isinstance(eval_run, dict) else {}
    if isinstance(metadata, dict):
        candidate_id = str(metadata.get("candidate_id") or "").strip()
        if candidate_id:
            return candidate_id
    return ""


def is_budget_gate_report_payload(payload: Any) -> bool:
    if not isinstance(payload, dict):
        return False
    if not isinstance(payload.get("status"), str):
        return False
    if not isinstance(payload.get("passed"), bool):
        return False
    budget_gate = payload.get("budget_gate")
    if not isinstance(budget_gate, dict):
        return False
    return payload.get("candidate_label") or isinstance(payload.get("eval_run"), dict)


def load_attempts(report_paths: Sequence[Path]) -> List[GateAttempt]:
    attempts: List[GateAttempt] = []
    for report_path in report_paths:
        payload = json.loads(report_path.read_text(encoding="utf-8"))
        if not is_budget_gate_report_payload(payload):
            continue
        budget_gate = payload.get("budget_gate") if isinstance(payload.get("budget_gate"), dict) else {}
        metrics = budget_gate.get("metrics") if isinstance(budget_gate.get("metrics"), dict) else {}
        eval_run = payload.get("eval_run") if isinstance(payload.get("eval_run"), dict) else {}
        candidate_id = extract_candidate_id(payload, eval_run)
        candidate_label = str(payload.get("candidate_label") or eval_run.get("title") or "").strip() or "unknown"
        attempts.append(
            GateAttempt(
                candidate_id=candidate_id or candidate_label,
                candidate_label=candidate_label,
                generated_at=str(payload.get("generated_at") or ""),
                generated_at_sort=parse_timestamp(str(payload.get("generated_at") or ""), report_path),
                report_path=report_path.name,
                passed=bool(payload.get("passed")),
                status=str(payload.get("status") or "").strip(),
                eval_run_id=str(eval_run.get("id") or "").strip(),
                case_count=as_int(metrics.get("case_count")),
                comparable_case_count=as_int(metrics.get("comparable_case_count")),
                missing_surface_case_count=as_int(metrics.get("missing_surface_case_count")),
                median_schema_byte_reduction_rate=as_float(metrics.get("median_schema_byte_reduction_rate")),
                median_latency_increase_rate=as_float(metrics.get("median_latency_increase_rate")),
                allowed_final_native_tool_case_rate=as_float(metrics.get("allowed_final_native_tool_case_rate")),
                non_allowed_native_tool_case_count=as_int(metrics.get("non_allowed_native_tool_case_count")),
            )
        )
    attempts.sort(
        key=lambda attempt: (
            attempt.generated_at_sort,
            attempt.report_path,
        ),
        reverse=True,
    )
    return attempts


def group_attempts_by_candidate(attempts: Iterable[GateAttempt]) -> Dict[str, List[GateAttempt]]:
    grouped: Dict[str, List[GateAttempt]] = {}
    for attempt in attempts:
        grouped.setdefault(attempt.candidate_id or attempt.candidate_label, []).append(attempt)
    for candidate_attempts in grouped.values():
        candidate_attempts.sort(
            key=lambda attempt: (
                attempt.generated_at_sort,
                attempt.report_path,
            ),
            reverse=True,
        )
    return grouped


def count_consecutive_green(attempts: Sequence[GateAttempt]) -> int:
    streak = 0
    for attempt in attempts:
        if attempt.passed:
            streak += 1
            continue
        break
    return streak


def build_candidate_rows(grouped_attempts: Dict[str, List[GateAttempt]]) -> List[Dict[str, Any]]:
    rows: List[Dict[str, Any]] = []
    for candidate_key, attempts in grouped_attempts.items():
        latest = attempts[0]
        rows.append(
            {
                "candidate_id": latest.candidate_id or candidate_key,
                "candidate_label": latest.candidate_label,
                "attempt_count": len(attempts),
                "pass_count": sum(1 for attempt in attempts if attempt.passed),
                "consecutive_green_count": count_consecutive_green(attempts),
                "latest_generated_at": latest.generated_at,
                "latest_report_path": latest.report_path,
                "latest_eval_run_id": latest.eval_run_id,
                "latest_status": latest.status,
                "latest_passed": latest.passed,
                "latest_case_count": latest.case_count,
                "latest_comparable_case_count": latest.comparable_case_count,
                "latest_missing_surface_case_count": latest.missing_surface_case_count,
                "latest_median_schema_byte_reduction_rate": latest.median_schema_byte_reduction_rate,
                "latest_median_latency_increase_rate": latest.median_latency_increase_rate,
                "latest_allowed_final_native_tool_case_rate": latest.allowed_final_native_tool_case_rate,
                "latest_non_allowed_native_tool_case_count": latest.non_allowed_native_tool_case_count,
            }
        )
    rows.sort(key=lambda row: (row["latest_generated_at"], row["candidate_id"], row["candidate_label"]), reverse=True)
    return rows


def resolve_focus_candidate_key(
    grouped: Dict[str, List[GateAttempt]],
    attempts: Sequence[GateAttempt],
    candidate_id: str,
    candidate_label: str,
) -> str:
    candidate_id = candidate_id.strip()
    if candidate_id:
        if candidate_id not in grouped:
            raise ValueError(f"candidate_id {candidate_id!r} was not found in the matched reports")
        return candidate_id

    candidate_label = candidate_label.strip()
    if candidate_label:
        if candidate_label in grouped:
            return candidate_label
        for key, grouped_attempts in grouped.items():
            if any(attempt.candidate_label == candidate_label for attempt in grouped_attempts):
                return key
        raise ValueError(f"candidate_label {candidate_label!r} was not found in the matched reports")

    latest = attempts[0]
    return latest.candidate_id or latest.candidate_label


def build_summary(attempts: Sequence[GateAttempt], candidate_id: str, candidate_label: str, require_consecutive_green: int) -> Dict[str, Any]:
    if not attempts:
        raise ValueError("no budget gate reports matched the provided patterns")

    grouped = group_attempts_by_candidate(attempts)
    focus_candidate = resolve_focus_candidate_key(grouped, attempts, candidate_id, candidate_label)

    focused_attempts = grouped[focus_candidate]
    latest = focused_attempts[0]
    consecutive_green = count_consecutive_green(focused_attempts)
    requirement_passed = True
    if require_consecutive_green > 0:
        requirement_passed = consecutive_green >= require_consecutive_green and latest.passed

    return {
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "focus_candidate_id": latest.candidate_id or focus_candidate,
        "focus_candidate_label": latest.candidate_label,
        "require_consecutive_green": require_consecutive_green,
        "requirement_passed": requirement_passed,
        "total_report_count": len(attempts),
        "candidate_count": len(grouped),
        "focused_candidate": {
            "candidate_id": latest.candidate_id or focus_candidate,
            "candidate_label": latest.candidate_label,
            "attempt_count": len(focused_attempts),
            "pass_count": sum(1 for attempt in focused_attempts if attempt.passed),
            "consecutive_green_count": consecutive_green,
            "latest_attempt": attempt_to_json(latest),
            "recent_attempts": [attempt_to_json(attempt) for attempt in focused_attempts[:10]],
        },
        "candidates": build_candidate_rows(grouped),
    }


def attempt_to_json(attempt: GateAttempt) -> Dict[str, Any]:
    return {
        "candidate_id": attempt.candidate_id,
        "candidate_label": attempt.candidate_label,
        "generated_at": attempt.generated_at,
        "report_path": attempt.report_path,
        "passed": attempt.passed,
        "status": attempt.status,
        "eval_run_id": attempt.eval_run_id,
        "case_count": attempt.case_count,
        "comparable_case_count": attempt.comparable_case_count,
        "missing_surface_case_count": attempt.missing_surface_case_count,
        "median_schema_byte_reduction_rate": attempt.median_schema_byte_reduction_rate,
        "median_latency_increase_rate": attempt.median_latency_increase_rate,
        "allowed_final_native_tool_case_rate": attempt.allowed_final_native_tool_case_rate,
        "non_allowed_native_tool_case_count": attempt.non_allowed_native_tool_case_count,
    }


def build_markdown_report(summary: Dict[str, Any], title: str) -> str:
    focused = summary["focused_candidate"]
    latest = focused["latest_attempt"]
    lines = [
        f"# {title}",
        "",
        f"- Generated At: {summary.get('generated_at', '-')}",
        f"- Focus Candidate ID: {summary.get('focus_candidate_id', '-')}",
        f"- Focus Candidate: {summary.get('focus_candidate_label', '-')}",
        f"- Consecutive Green Requirement: {summary.get('require_consecutive_green', 0)}",
        f"- Requirement Passed: {'yes' if summary.get('requirement_passed') else 'no'}",
        f"- Total Reports: {summary.get('total_report_count', 0)}",
        f"- Candidate Count: {summary.get('candidate_count', 0)}",
        "",
        "## Focus Candidate",
        "",
        f"- Candidate ID: {focused.get('candidate_id', '-')}",
        f"- Candidate: {focused.get('candidate_label', '-')}",
        f"- Attempts: {focused.get('attempt_count', 0)}",
        f"- Pass Count: {focused.get('pass_count', 0)}",
        f"- Consecutive Green Count: {focused.get('consecutive_green_count', 0)}",
        f"- Latest Report: {latest.get('report_path', '-')}",
        f"- Latest Eval Run: {latest.get('eval_run_id', '-')}",
        f"- Latest Status: {latest.get('status', '-')}",
        f"- Latest Passed: {'yes' if latest.get('passed') else 'no'}",
        f"- Latest Comparable Cases: {latest.get('comparable_case_count', 0)}/{latest.get('case_count', 0)}",
        f"- Latest Missing Surface Cases: {latest.get('missing_surface_case_count', '-')}",
        f"- Latest Median Schema-Byte Reduction: {format_decimal(latest.get('median_schema_byte_reduction_rate'))}",
        f"- Latest Median Latency Increase: {format_decimal(latest.get('median_latency_increase_rate'))}",
        f"- Latest Allowed Final Native Tool Case Rate: {format_decimal(latest.get('allowed_final_native_tool_case_rate'))}",
        f"- Latest Non-Allowed Native Tool Case Count: {latest.get('non_allowed_native_tool_case_count', '-')}",
        "",
        "## Recent Attempts",
        "",
    ]
    for attempt in focused.get("recent_attempts", []):
        lines.append(
            "- "
            + f"{attempt.get('generated_at', '-')} | "
            + f"{attempt.get('report_path', '-')} | "
            + f"passed={'yes' if attempt.get('passed') else 'no'} | "
            + f"status={attempt.get('status', '-')} | "
            + f"schema_reduction={format_decimal(attempt.get('median_schema_byte_reduction_rate'))} | "
            + f"latency_increase={format_decimal(attempt.get('median_latency_increase_rate'))}"
        )

    lines.extend(["", "## Candidate Overview", ""])
    for row in summary.get("candidates", [])[:20]:
        lines.append(
            "- "
            + f"{row.get('candidate_id', '-')} | "
            + f"label={row.get('candidate_label', '-')} | "
            + f"latest={'pass' if row.get('latest_passed') else 'fail'} | "
            + f"streak={row.get('consecutive_green_count', 0)} | "
            + f"attempts={row.get('attempt_count', 0)} | "
            + f"schema_reduction={format_decimal(row.get('latest_median_schema_byte_reduction_rate'))} | "
            + f"latency_increase={format_decimal(row.get('latest_median_latency_increase_rate'))} | "
            + f"non_allowed_tools={row.get('latest_non_allowed_native_tool_case_count', '-')}"
        )
    return "\n".join(lines) + "\n"


def format_decimal(value: Any) -> str:
    if isinstance(value, (int, float)):
        text = f"{float(value):.6f}".rstrip("0").rstrip(".")
        return text if text else "0"
    return "-"


def write_outputs(summary: Dict[str, Any], output_json: Path, output_md: Path, title: str) -> None:
    output_json.parent.mkdir(parents=True, exist_ok=True)
    output_md.parent.mkdir(parents=True, exist_ok=True)
    output_json.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    output_md.write_text(build_markdown_report(summary, title), encoding="utf-8")


def main() -> int:
    args = parse_args()
    report_paths = expand_report_paths(args.reports)
    summary = build_summary(
        attempts=load_attempts(report_paths),
        candidate_id=args.candidate_id,
        candidate_label=args.candidate_label,
        require_consecutive_green=args.require_consecutive_green,
    )
    write_outputs(summary, Path(args.output_json), Path(args.output_md), args.title)
    return 0 if summary.get("requirement_passed", True) else 1


if __name__ == "__main__":
    sys.exit(main())
