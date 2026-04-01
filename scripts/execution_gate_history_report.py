#!/usr/bin/env python3
from __future__ import annotations

import argparse
import glob
import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence, Tuple


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
    pass_rate: Optional[float]
    critical_pass_rate: Optional[float]
    verification_pass_rate_delta: Optional[float]
    evidence_backed_pass_rate_delta: Optional[float]
    critical_regression_count: Optional[int]
    locale_breakdown: Dict[str, Dict[str, Any]]
    primary_route_breakdown: Dict[str, Dict[str, Any]]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Summarize batch-1 execution gate history and evaluate consecutive-green requirements.")
    parser.add_argument(
        "--reports",
        nargs="+",
        default=["docs/reports/execution_gate_report*.json"],
        help="One or more execution gate report files or glob patterns.",
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
        default="docs/reports/execution_gate_history_report.md",
        help="Markdown summary output path.",
    )
    parser.add_argument(
        "--output-json",
        default="docs/reports/execution_gate_history_report.json",
        help="Structured JSON summary output path.",
    )
    parser.add_argument(
        "--title",
        default="Execution Gate History Summary",
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


def normalize_breakdown_map(raw: Any) -> Dict[str, Dict[str, Any]]:
    if not isinstance(raw, dict):
        return {}
    out: Dict[str, Dict[str, Any]] = {}
    for key, value in raw.items():
        if isinstance(value, dict):
            out[str(key)] = dict(value)
    return out


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


def select_breakdown_alert_rows(raw_breakdown: Any) -> List[Tuple[str, Dict[str, Any]]]:
    if not isinstance(raw_breakdown, dict):
        return []

    rows: List[Tuple[str, Dict[str, Any]]] = []
    for key, value in raw_breakdown.items():
        if not isinstance(value, dict):
            continue
        critical_regressions = value.get("critical_regression_count", 0)
        pass_rate = value.get("pass_rate")
        new_failures = value.get("new_failure_count", 0)
        needs_attention = (
            (isinstance(critical_regressions, (int, float)) and float(critical_regressions) > 0)
            or (isinstance(pass_rate, (int, float)) and float(pass_rate) < 1.0)
            or (isinstance(new_failures, (int, float)) and float(new_failures) > 0)
        )
        if needs_attention:
            rows.append((str(key), value))

    rows.sort(
        key=lambda item: (
            -float(item[1].get("critical_regression_count", 0) or 0),
            float(item[1].get("pass_rate", 1.0) or 1.0),
            -float(item[1].get("new_failure_count", 0) or 0),
            item[0],
        )
    )
    return rows


def serialize_breakdown_alert_rows(rows: Sequence[Tuple[str, Dict[str, Any]]], limit: int = 5) -> List[Dict[str, Any]]:
    out: List[Dict[str, Any]] = []
    for key, metrics in rows[:limit]:
        out.append(
            {
                "key": key,
                "case_count": as_int(metrics.get("case_count")),
                "passed_count": as_int(metrics.get("passed_count")),
                "pass_rate": as_float(metrics.get("pass_rate")),
                "critical_case_count": as_int(metrics.get("critical_case_count")),
                "critical_passed_count": as_int(metrics.get("critical_passed_count")),
                "critical_pass_rate": as_float(metrics.get("critical_pass_rate")),
                "regression_count": as_int(metrics.get("regression_count")),
                "improvement_count": as_int(metrics.get("improvement_count")),
                "new_failure_count": as_int(metrics.get("new_failure_count")),
                "resolved_failure_count": as_int(metrics.get("resolved_failure_count")),
                "critical_regression_count": as_int(metrics.get("critical_regression_count")),
            }
        )
    return out


def summarize_recurring_breakdown_alerts(
    attempts: Sequence[GateAttempt], breakdown_attr: str, limit: int = 10
) -> List[Dict[str, Any]]:
    aggregates: Dict[str, Dict[str, Any]] = {}
    for attempt in attempts:
        breakdown = getattr(attempt, breakdown_attr, {})
        for key, metrics in select_breakdown_alert_rows(breakdown):
            bucket = aggregates.setdefault(
                key,
                {
                    "key": key,
                    "attempts_with_drift": 0,
                    "critical_regression_attempts": 0,
                    "new_failure_attempts": 0,
                    "worst_pass_rate": 1.0,
                    "latest_generated_at": "",
                    "latest_report_path": "",
                    "latest_eval_run_id": "",
                    "latest_pass_rate": None,
                    "latest_critical_regression_count": 0,
                },
            )
            bucket["attempts_with_drift"] += 1
            critical_regressions = as_int(metrics.get("critical_regression_count")) or 0
            if critical_regressions > 0:
                bucket["critical_regression_attempts"] += 1
            new_failures = as_int(metrics.get("new_failure_count")) or 0
            if new_failures > 0:
                bucket["new_failure_attempts"] += 1
            pass_rate = as_float(metrics.get("pass_rate"))
            if pass_rate is not None:
                bucket["worst_pass_rate"] = min(float(bucket["worst_pass_rate"]), pass_rate)
                if bucket["latest_pass_rate"] is None:
                    bucket["latest_pass_rate"] = pass_rate
            if not bucket["latest_generated_at"]:
                bucket["latest_generated_at"] = attempt.generated_at
                bucket["latest_report_path"] = attempt.report_path
                bucket["latest_eval_run_id"] = attempt.eval_run_id
                bucket["latest_critical_regression_count"] = critical_regressions

    rows = list(aggregates.values())
    rows.sort(
        key=lambda row: (
            -int(row["critical_regression_attempts"]),
            -int(row["attempts_with_drift"]),
            float(row["worst_pass_rate"]),
            row["key"],
        )
    )
    return rows[:limit]


def is_execution_gate_report_payload(payload: Any) -> bool:
    if not isinstance(payload, dict):
        return False
    if not isinstance(payload.get("status"), str):
        return False
    if not isinstance(payload.get("passed"), bool):
        return False
    if payload.get("candidate_label"):
        return True
    return isinstance(payload.get("eval_run"), dict) and isinstance(payload.get("execution_gate"), dict)


def load_attempts(report_paths: Sequence[Path]) -> List[GateAttempt]:
    attempts: List[GateAttempt] = []
    for report_path in report_paths:
        payload = json.loads(report_path.read_text(encoding="utf-8"))
        if not is_execution_gate_report_payload(payload):
            continue
        execution_gate = payload.get("execution_gate") if isinstance(payload.get("execution_gate"), dict) else {}
        metrics = execution_gate.get("metrics") if isinstance(execution_gate, dict) and isinstance(execution_gate.get("metrics"), dict) else {}
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
                pass_rate=as_float(metrics.get("pass_rate")),
                critical_pass_rate=as_float(metrics.get("critical_pass_rate")),
                verification_pass_rate_delta=as_float(metrics.get("verification_pass_rate_delta")),
                evidence_backed_pass_rate_delta=as_float(metrics.get("evidence_backed_pass_rate_delta")),
                critical_regression_count=as_int(metrics.get("critical_regression_count")),
                locale_breakdown=normalize_breakdown_map(metrics.get("locale_breakdown")),
                primary_route_breakdown=normalize_breakdown_map(metrics.get("primary_route_breakdown")),
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
                "latest_pass_rate": latest.pass_rate,
                "latest_critical_pass_rate": latest.critical_pass_rate,
                "latest_verification_pass_rate_delta": latest.verification_pass_rate_delta,
                "latest_evidence_backed_pass_rate_delta": latest.evidence_backed_pass_rate_delta,
                "latest_critical_regression_count": latest.critical_regression_count,
                "latest_locale_drift_count": len(select_breakdown_alert_rows(latest.locale_breakdown)),
                "latest_primary_route_drift_count": len(select_breakdown_alert_rows(latest.primary_route_breakdown)),
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
        raise ValueError("no execution gate reports matched the provided patterns")

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
            "latest_locale_drift_alerts": serialize_breakdown_alert_rows(select_breakdown_alert_rows(latest.locale_breakdown)),
            "latest_primary_route_drift_alerts": serialize_breakdown_alert_rows(select_breakdown_alert_rows(latest.primary_route_breakdown)),
            "recurring_locale_drift": summarize_recurring_breakdown_alerts(focused_attempts, "locale_breakdown"),
            "recurring_primary_route_drift": summarize_recurring_breakdown_alerts(focused_attempts, "primary_route_breakdown"),
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
        "pass_rate": attempt.pass_rate,
        "critical_pass_rate": attempt.critical_pass_rate,
        "verification_pass_rate_delta": attempt.verification_pass_rate_delta,
        "evidence_backed_pass_rate_delta": attempt.evidence_backed_pass_rate_delta,
        "critical_regression_count": attempt.critical_regression_count,
        "locale_drift_alerts": serialize_breakdown_alert_rows(select_breakdown_alert_rows(attempt.locale_breakdown)),
        "primary_route_drift_alerts": serialize_breakdown_alert_rows(select_breakdown_alert_rows(attempt.primary_route_breakdown)),
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
        f"- Latest Pass Rate: {format_decimal(latest.get('pass_rate'))}",
        f"- Latest Critical Pass Rate: {format_decimal(latest.get('critical_pass_rate'))}",
        f"- Latest Verification Pass-Rate Delta: {format_decimal(latest.get('verification_pass_rate_delta'))}",
        f"- Latest Evidence-Backed Pass-Rate Delta: {format_decimal(latest.get('evidence_backed_pass_rate_delta'))}",
        f"- Latest Critical Regression Count: {latest.get('critical_regression_count', '-')}",
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
            + f"pass_rate={format_decimal(attempt.get('pass_rate'))}"
        )

    append_breakdown_section(lines, "Latest Locale Drift Alerts", focused.get("latest_locale_drift_alerts") or [])
    append_breakdown_section(lines, "Latest Primary Route Drift Alerts", focused.get("latest_primary_route_drift_alerts") or [])
    append_recurring_breakdown_section(lines, "Recurring Locale Drift", focused.get("recurring_locale_drift") or [])
    append_recurring_breakdown_section(lines, "Recurring Primary Route Drift", focused.get("recurring_primary_route_drift") or [])

    lines.extend(["", "## Candidate Overview", ""])
    for row in summary.get("candidates", [])[:20]:
        lines.append(
            "- "
            + f"{row.get('candidate_id', '-')} | "
            + f"label={row.get('candidate_label', '-')} | "
            + f"latest={'pass' if row.get('latest_passed') else 'fail'} | "
            + f"streak={row.get('consecutive_green_count', 0)} | "
            + f"attempts={row.get('attempt_count', 0)} | "
            + f"locale_drift={row.get('latest_locale_drift_count', 0)} | "
            + f"route_drift={row.get('latest_primary_route_drift_count', 0)} | "
            + f"latest_report={row.get('latest_report_path', '-')}"
        )
    return "\n".join(lines) + "\n"


def append_breakdown_section(lines: List[str], title: str, rows: Sequence[Dict[str, Any]]) -> None:
    if not rows:
        return
    lines.extend(["", f"## {title}", ""])
    for row in rows[:5]:
        lines.append(
            "- "
            + f"{row.get('key', '-')}: "
            + f"pass={format_ratio(row.get('passed_count'), row.get('case_count'), row.get('pass_rate'))} | "
            + f"critical={format_ratio(row.get('critical_passed_count'), row.get('critical_case_count'), row.get('critical_pass_rate'))} | "
            + f"regressions={row.get('regression_count', 0)} | "
            + f"new_failures={row.get('new_failure_count', 0)} | "
            + f"critical_regressions={row.get('critical_regression_count', 0)}"
        )


def append_recurring_breakdown_section(lines: List[str], title: str, rows: Sequence[Dict[str, Any]]) -> None:
    if not rows:
        return
    lines.extend(["", f"## {title}", ""])
    for row in rows[:5]:
        lines.append(
            "- "
            + f"{row.get('key', '-')}: "
            + f"drift_attempts={row.get('attempts_with_drift', 0)} | "
            + f"critical_regression_attempts={row.get('critical_regression_attempts', 0)} | "
            + f"new_failure_attempts={row.get('new_failure_attempts', 0)} | "
            + f"worst_pass_rate={format_decimal(row.get('worst_pass_rate'))} | "
            + f"latest_report={row.get('latest_report_path', '-')}"
        )


def format_decimal(value: Any) -> str:
    if isinstance(value, (int, float)):
        text = f"{float(value):.6f}".rstrip("0").rstrip(".")
        return text if text else "0"
    return "-"


def format_ratio(numerator: Any, denominator: Any, rate: Any) -> str:
    return f"{numerator or 0}/{denominator or 0} ({format_decimal(rate)})"


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
    raise SystemExit(main())
