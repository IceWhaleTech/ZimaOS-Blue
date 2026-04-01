#!/usr/bin/env python3
from __future__ import annotations

import argparse
import glob
import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence

import execution_gate_history_report as execution_history
import selector_gate_history_report as selector_history


@dataclass
class PipelineAttempt:
    candidate_id: str
    candidate_label: str
    generated_at: str
    generated_at_sort: datetime
    report_path: str
    ready: bool
    status: str
    selector_eval_run_id: str
    execution_eval_run_id: str
    evaluated_gates_ready: Optional[bool]
    selector_streak: Optional[int]
    execution_streak: Optional[int]
    budget_streak: Optional[int]
    required_consecutive_runs: Optional[int]
    blocking_reasons: List[str]
    selector_locale_breakdown: Dict[str, Dict[str, Any]]
    selector_primary_route_breakdown: Dict[str, Dict[str, Any]]
    execution_locale_breakdown: Dict[str, Dict[str, Any]]
    execution_primary_route_breakdown: Dict[str, Dict[str, Any]]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Summarize cutover candidate pipeline history and evaluate consecutive-green requirements.")
    parser.add_argument(
        "--reports",
        nargs="+",
        default=["docs/reports/cutover_candidate_pipeline/cutover_candidate_pipeline_report*.json"],
        help="One or more pipeline report files or glob patterns.",
    )
    parser.add_argument("--candidate-id", default="", help="Candidate id to evaluate. Takes precedence over --candidate-label.")
    parser.add_argument("--candidate-label", default="", help="Candidate label to evaluate. Defaults to the latest available candidate.")
    parser.add_argument(
        "--require-consecutive-green",
        type=int,
        default=0,
        help="If set, fail unless the focused candidate has at least this many consecutive ready attempts.",
    )
    parser.add_argument(
        "--output-md",
        default="docs/reports/cutover_candidate_pipeline_history_report.md",
        help="Markdown summary output path.",
    )
    parser.add_argument(
        "--output-json",
        default="docs/reports/cutover_candidate_pipeline_history_report.json",
        help="Structured JSON summary output path.",
    )
    parser.add_argument("--title", default="Cutover Candidate Pipeline History Summary", help="Markdown title.")
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


def as_bool(value: Any) -> Optional[bool]:
    if isinstance(value, bool):
        return value
    return None


def as_int(value: Any) -> Optional[int]:
    if isinstance(value, bool):
        return int(value)
    if isinstance(value, int):
        return value
    return None


def is_pipeline_report_payload(payload: Any) -> bool:
    if not isinstance(payload, dict):
        return False
    if not isinstance(payload.get("status"), str):
        return False
    if not isinstance(payload.get("ready"), bool):
        return False
    return isinstance(payload.get("steps"), list)


def extract_step_report(steps: Any, step_name: str) -> Dict[str, Any]:
    if not isinstance(steps, list):
        return {}
    for step in steps:
        if not isinstance(step, dict):
            continue
        if str(step.get("name") or "").strip() != step_name:
            continue
        report = step.get("report")
        if isinstance(report, dict):
            return report
    return {}


def extract_step_eval_run_id(step_report: Dict[str, Any]) -> str:
    eval_run = step_report.get("eval_run") if isinstance(step_report.get("eval_run"), dict) else {}
    return str(eval_run.get("id") or "").strip()


def extract_gate_metrics(step_report: Dict[str, Any], gate_key: str) -> Dict[str, Any]:
    gate = step_report.get(gate_key) if isinstance(step_report.get(gate_key), dict) else {}
    metrics = gate.get("metrics") if isinstance(gate.get("metrics"), dict) else {}
    return metrics if isinstance(metrics, dict) else {}


def selector_alert_count(raw_breakdown: Dict[str, Dict[str, Any]]) -> int:
    return len(selector_history.select_breakdown_alert_rows(raw_breakdown))


def execution_alert_count(raw_breakdown: Dict[str, Dict[str, Any]]) -> int:
    return len(execution_history.select_breakdown_alert_rows(raw_breakdown))


def summarize_recurring_selector_alerts(
    attempts: Sequence[PipelineAttempt],
    breakdown_attr: str,
    eval_run_attr: str,
    limit: int = 10,
) -> List[Dict[str, Any]]:
    aggregates: Dict[str, Dict[str, Any]] = {}
    for attempt in attempts:
        breakdown = getattr(attempt, breakdown_attr, {})
        for key, metrics in selector_history.select_breakdown_alert_rows(breakdown):
            bucket = aggregates.setdefault(
                key,
                {
                    "key": key,
                    "attempts_with_drift": 0,
                    "critical_regression_attempts": 0,
                    "worst_route_compatible_rate": 1.0,
                    "max_abs_clarify_rate_delta": 0.0,
                    "latest_generated_at": "",
                    "latest_report_path": "",
                    "latest_eval_run_id": "",
                    "latest_route_compatible_rate": None,
                    "latest_critical_regression_count": 0,
                },
            )
            bucket["attempts_with_drift"] += 1
            critical_regressions = selector_history.as_int(metrics.get("critical_regression_count")) or 0
            if critical_regressions > 0:
                bucket["critical_regression_attempts"] += 1
            compatible_rate = selector_history.as_float(metrics.get("route_compatible_rate"))
            if compatible_rate is not None:
                bucket["worst_route_compatible_rate"] = min(float(bucket["worst_route_compatible_rate"]), compatible_rate)
                if bucket["latest_route_compatible_rate"] is None:
                    bucket["latest_route_compatible_rate"] = compatible_rate
            clarify_delta = selector_history.as_float(metrics.get("clarify_rate_delta"))
            if clarify_delta is not None:
                bucket["max_abs_clarify_rate_delta"] = max(float(bucket["max_abs_clarify_rate_delta"]), abs(clarify_delta))
            if not bucket["latest_generated_at"]:
                bucket["latest_generated_at"] = attempt.generated_at
                bucket["latest_report_path"] = attempt.report_path
                bucket["latest_eval_run_id"] = str(getattr(attempt, eval_run_attr, "") or "")
                bucket["latest_critical_regression_count"] = critical_regressions

    rows = list(aggregates.values())
    rows.sort(
        key=lambda row: (
            -int(row["critical_regression_attempts"]),
            -int(row["attempts_with_drift"]),
            float(row["worst_route_compatible_rate"]),
            row["key"],
        )
    )
    return rows[:limit]


def summarize_recurring_execution_alerts(
    attempts: Sequence[PipelineAttempt],
    breakdown_attr: str,
    eval_run_attr: str,
    limit: int = 10,
) -> List[Dict[str, Any]]:
    aggregates: Dict[str, Dict[str, Any]] = {}
    for attempt in attempts:
        breakdown = getattr(attempt, breakdown_attr, {})
        for key, metrics in execution_history.select_breakdown_alert_rows(breakdown):
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
            critical_regressions = execution_history.as_int(metrics.get("critical_regression_count")) or 0
            if critical_regressions > 0:
                bucket["critical_regression_attempts"] += 1
            new_failures = execution_history.as_int(metrics.get("new_failure_count")) or 0
            if new_failures > 0:
                bucket["new_failure_attempts"] += 1
            pass_rate = execution_history.as_float(metrics.get("pass_rate"))
            if pass_rate is not None:
                bucket["worst_pass_rate"] = min(float(bucket["worst_pass_rate"]), pass_rate)
                if bucket["latest_pass_rate"] is None:
                    bucket["latest_pass_rate"] = pass_rate
            if not bucket["latest_generated_at"]:
                bucket["latest_generated_at"] = attempt.generated_at
                bucket["latest_report_path"] = attempt.report_path
                bucket["latest_eval_run_id"] = str(getattr(attempt, eval_run_attr, "") or "")
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


def load_attempts(report_paths: Sequence[Path]) -> List[PipelineAttempt]:
    attempts: List[PipelineAttempt] = []
    for report_path in report_paths:
        payload = json.loads(report_path.read_text(encoding="utf-8"))
        if not is_pipeline_report_payload(payload):
            continue
        readiness = payload.get("readiness") if isinstance(payload.get("readiness"), dict) else {}
        steps = payload.get("steps") if isinstance(payload.get("steps"), list) else []
        selector_report = extract_step_report(steps, "selector")
        execution_report = extract_step_report(steps, "execution")
        selector_metrics = extract_gate_metrics(selector_report, "selector_gate")
        execution_metrics = extract_gate_metrics(execution_report, "execution_gate")
        selector_eval_run_id = str(payload.get("selector_eval_run_id") or "").strip() or extract_step_eval_run_id(selector_report)
        attempts.append(
            PipelineAttempt(
                candidate_id=str(payload.get("candidate_id") or "").strip() or str(payload.get("candidate_label") or "unknown").strip(),
                candidate_label=str(payload.get("candidate_label") or "").strip() or str(payload.get("candidate_id") or "unknown").strip(),
                generated_at=str(payload.get("generated_at") or ""),
                generated_at_sort=parse_timestamp(str(payload.get("generated_at") or ""), report_path),
                report_path=report_path.name,
                ready=bool(payload.get("ready")),
                status=str(payload.get("status") or "").strip(),
                selector_eval_run_id=selector_eval_run_id,
                execution_eval_run_id=extract_step_eval_run_id(execution_report),
                evaluated_gates_ready=as_bool(readiness.get("evaluated_gates_ready")),
                selector_streak=as_int((readiness.get("selector") or {}).get("consecutive_pass_count")) if isinstance(readiness.get("selector"), dict) else None,
                execution_streak=as_int((readiness.get("execution") or {}).get("consecutive_pass_count")) if isinstance(readiness.get("execution"), dict) else None,
                budget_streak=as_int((readiness.get("budget") or {}).get("consecutive_pass_count")) if isinstance(readiness.get("budget"), dict) else None,
                required_consecutive_runs=as_int(readiness.get("required_consecutive_runs")),
                blocking_reasons=[str(item) for item in (readiness.get("blocking_reasons") or []) if str(item).strip()] if isinstance(readiness.get("blocking_reasons"), list) else [],
                selector_locale_breakdown=selector_history.normalize_breakdown_map(selector_metrics.get("locale_breakdown")),
                selector_primary_route_breakdown=selector_history.normalize_breakdown_map(selector_metrics.get("primary_route_breakdown")),
                execution_locale_breakdown=execution_history.normalize_breakdown_map(execution_metrics.get("locale_breakdown")),
                execution_primary_route_breakdown=execution_history.normalize_breakdown_map(execution_metrics.get("primary_route_breakdown")),
            )
        )
    attempts.sort(key=lambda attempt: (attempt.generated_at_sort, attempt.report_path), reverse=True)
    return attempts


def group_attempts_by_candidate(attempts: Iterable[PipelineAttempt]) -> Dict[str, List[PipelineAttempt]]:
    grouped: Dict[str, List[PipelineAttempt]] = {}
    for attempt in attempts:
        grouped.setdefault(attempt.candidate_id, []).append(attempt)
    for candidate_attempts in grouped.values():
        candidate_attempts.sort(key=lambda attempt: (attempt.generated_at_sort, attempt.report_path), reverse=True)
    return grouped


def count_consecutive_green(attempts: Sequence[PipelineAttempt]) -> int:
    streak = 0
    for attempt in attempts:
        if attempt.ready:
            streak += 1
            continue
        break
    return streak


def resolve_focus_candidate_key(
    grouped: Dict[str, List[PipelineAttempt]],
    attempts: Sequence[PipelineAttempt],
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
        for key, grouped_attempts in grouped.items():
            if any(attempt.candidate_label == candidate_label for attempt in grouped_attempts):
                return key
        raise ValueError(f"candidate_label {candidate_label!r} was not found in the matched reports")

    return attempts[0].candidate_id


def build_candidate_rows(grouped_attempts: Dict[str, List[PipelineAttempt]]) -> List[Dict[str, Any]]:
    rows: List[Dict[str, Any]] = []
    for candidate_id, attempts in grouped_attempts.items():
        latest = attempts[0]
        rows.append(
            {
                "candidate_id": candidate_id,
                "candidate_label": latest.candidate_label,
                "attempt_count": len(attempts),
                "ready_count": sum(1 for attempt in attempts if attempt.ready),
                "consecutive_green_count": count_consecutive_green(attempts),
                "latest_generated_at": latest.generated_at,
                "latest_report_path": latest.report_path,
                "latest_ready": latest.ready,
                "latest_status": latest.status,
                "latest_selector_eval_run_id": latest.selector_eval_run_id,
                "latest_execution_eval_run_id": latest.execution_eval_run_id,
                "latest_evaluated_gates_ready": latest.evaluated_gates_ready,
                "latest_selector_streak": latest.selector_streak,
                "latest_execution_streak": latest.execution_streak,
                "latest_budget_streak": latest.budget_streak,
                "latest_blocking_reason_count": len(latest.blocking_reasons),
                "latest_selector_locale_drift_count": selector_alert_count(latest.selector_locale_breakdown),
                "latest_selector_primary_route_drift_count": selector_alert_count(latest.selector_primary_route_breakdown),
                "latest_execution_locale_drift_count": execution_alert_count(latest.execution_locale_breakdown),
                "latest_execution_primary_route_drift_count": execution_alert_count(latest.execution_primary_route_breakdown),
            }
        )
    rows.sort(key=lambda row: (row["latest_generated_at"], row["candidate_id"], row["candidate_label"]), reverse=True)
    return rows


def attempt_to_json(attempt: PipelineAttempt) -> Dict[str, Any]:
    return {
        "candidate_id": attempt.candidate_id,
        "candidate_label": attempt.candidate_label,
        "generated_at": attempt.generated_at,
        "report_path": attempt.report_path,
        "ready": attempt.ready,
        "status": attempt.status,
        "selector_eval_run_id": attempt.selector_eval_run_id,
        "execution_eval_run_id": attempt.execution_eval_run_id,
        "evaluated_gates_ready": attempt.evaluated_gates_ready,
        "selector_streak": attempt.selector_streak,
        "execution_streak": attempt.execution_streak,
        "budget_streak": attempt.budget_streak,
        "required_consecutive_runs": attempt.required_consecutive_runs,
        "blocking_reasons": attempt.blocking_reasons,
        "selector_locale_drift_count": selector_alert_count(attempt.selector_locale_breakdown),
        "selector_primary_route_drift_count": selector_alert_count(attempt.selector_primary_route_breakdown),
        "execution_locale_drift_count": execution_alert_count(attempt.execution_locale_breakdown),
        "execution_primary_route_drift_count": execution_alert_count(attempt.execution_primary_route_breakdown),
    }


def build_summary(attempts: Sequence[PipelineAttempt], candidate_id: str, candidate_label: str, require_consecutive_green: int) -> Dict[str, Any]:
    if not attempts:
        raise ValueError("no cutover candidate pipeline reports matched the provided patterns")

    grouped = group_attempts_by_candidate(attempts)
    focus_candidate = resolve_focus_candidate_key(grouped, attempts, candidate_id, candidate_label)
    focused_attempts = grouped[focus_candidate]
    latest = focused_attempts[0]
    consecutive_green = count_consecutive_green(focused_attempts)
    requirement_passed = True
    if require_consecutive_green > 0:
        requirement_passed = consecutive_green >= require_consecutive_green and latest.ready

    return {
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "focus_candidate_id": latest.candidate_id,
        "focus_candidate_label": latest.candidate_label,
        "require_consecutive_green": require_consecutive_green,
        "requirement_passed": requirement_passed,
        "total_report_count": len(attempts),
        "candidate_count": len(grouped),
        "focused_candidate": {
            "candidate_id": latest.candidate_id,
            "candidate_label": latest.candidate_label,
            "attempt_count": len(focused_attempts),
            "ready_count": sum(1 for attempt in focused_attempts if attempt.ready),
            "consecutive_green_count": consecutive_green,
            "latest_attempt": attempt_to_json(latest),
            "recent_attempts": [attempt_to_json(attempt) for attempt in focused_attempts[:10]],
            "latest_selector_locale_drift_alerts": selector_history.serialize_breakdown_alert_rows(
                selector_history.select_breakdown_alert_rows(latest.selector_locale_breakdown)
            ),
            "latest_selector_primary_route_drift_alerts": selector_history.serialize_breakdown_alert_rows(
                selector_history.select_breakdown_alert_rows(latest.selector_primary_route_breakdown)
            ),
            "latest_execution_locale_drift_alerts": execution_history.serialize_breakdown_alert_rows(
                execution_history.select_breakdown_alert_rows(latest.execution_locale_breakdown)
            ),
            "latest_execution_primary_route_drift_alerts": execution_history.serialize_breakdown_alert_rows(
                execution_history.select_breakdown_alert_rows(latest.execution_primary_route_breakdown)
            ),
            "recurring_selector_locale_drift": summarize_recurring_selector_alerts(
                focused_attempts, "selector_locale_breakdown", "selector_eval_run_id"
            ),
            "recurring_selector_primary_route_drift": summarize_recurring_selector_alerts(
                focused_attempts, "selector_primary_route_breakdown", "selector_eval_run_id"
            ),
            "recurring_execution_locale_drift": summarize_recurring_execution_alerts(
                focused_attempts, "execution_locale_breakdown", "execution_eval_run_id"
            ),
            "recurring_execution_primary_route_drift": summarize_recurring_execution_alerts(
                focused_attempts, "execution_primary_route_breakdown", "execution_eval_run_id"
            ),
        },
        "candidates": build_candidate_rows(grouped),
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
        f"- Ready Count: {focused.get('ready_count', 0)}",
        f"- Consecutive Green Count: {focused.get('consecutive_green_count', 0)}",
        f"- Latest Report: {latest.get('report_path', '-')}",
        f"- Latest Status: {latest.get('status', '-')}",
        f"- Latest Ready: {'yes' if latest.get('ready') else 'no'}",
        f"- Latest Evaluated Gates Ready: {latest.get('evaluated_gates_ready', False)}",
        f"- Latest Selector Streak: {latest.get('selector_streak', '-')}",
        f"- Latest Execution Streak: {latest.get('execution_streak', '-')}",
        f"- Latest Budget Streak: {latest.get('budget_streak', '-')}",
        "",
        "## Recent Attempts",
        "",
    ]
    for attempt in focused.get("recent_attempts", []):
        lines.append(
            "- "
            + f"{attempt.get('generated_at', '-')} | "
            + f"{attempt.get('report_path', '-')} | "
            + f"ready={'yes' if attempt.get('ready') else 'no'} | "
            + f"status={attempt.get('status', '-')} | "
            + f"selector_locale_drift={attempt.get('selector_locale_drift_count', 0)} | "
            + f"selector_route_drift={attempt.get('selector_primary_route_drift_count', 0)} | "
            + f"execution_locale_drift={attempt.get('execution_locale_drift_count', 0)} | "
            + f"execution_route_drift={attempt.get('execution_primary_route_drift_count', 0)}"
        )

    selector_history.append_breakdown_section(
        lines, "Latest Selector Locale Drift Alerts", focused.get("latest_selector_locale_drift_alerts") or []
    )
    selector_history.append_breakdown_section(
        lines, "Latest Selector Primary Route Drift Alerts", focused.get("latest_selector_primary_route_drift_alerts") or []
    )
    selector_history.append_recurring_breakdown_section(
        lines, "Recurring Selector Locale Drift", focused.get("recurring_selector_locale_drift") or []
    )
    selector_history.append_recurring_breakdown_section(
        lines, "Recurring Selector Primary Route Drift", focused.get("recurring_selector_primary_route_drift") or []
    )
    execution_history.append_breakdown_section(
        lines, "Latest Execution Locale Drift Alerts", focused.get("latest_execution_locale_drift_alerts") or []
    )
    execution_history.append_breakdown_section(
        lines, "Latest Execution Primary Route Drift Alerts", focused.get("latest_execution_primary_route_drift_alerts") or []
    )
    execution_history.append_recurring_breakdown_section(
        lines, "Recurring Execution Locale Drift", focused.get("recurring_execution_locale_drift") or []
    )
    execution_history.append_recurring_breakdown_section(
        lines, "Recurring Execution Primary Route Drift", focused.get("recurring_execution_primary_route_drift") or []
    )

    if latest.get("blocking_reasons"):
        lines.extend(["", "## Latest Blocking Reasons", ""])
        lines.extend(f"- {item}" for item in latest.get("blocking_reasons", []))

    lines.extend(["", "## Candidate Overview", ""])
    for row in summary.get("candidates", [])[:20]:
        lines.append(
            "- "
            + f"{row.get('candidate_id', '-')} | "
            + f"label={row.get('candidate_label', '-')} | "
            + f"latest={'ready' if row.get('latest_ready') else 'not_ready'} | "
            + f"streak={row.get('consecutive_green_count', 0)} | "
            + f"attempts={row.get('attempt_count', 0)} | "
            + f"selector_locale_drift={row.get('latest_selector_locale_drift_count', 0)} | "
            + f"selector_route_drift={row.get('latest_selector_primary_route_drift_count', 0)} | "
            + f"execution_locale_drift={row.get('latest_execution_locale_drift_count', 0)} | "
            + f"execution_route_drift={row.get('latest_execution_primary_route_drift_count', 0)} | "
            + f"blocking_reasons={row.get('latest_blocking_reason_count', 0)} | "
            + f"latest_report={row.get('latest_report_path', '-')}"
        )
    return "\n".join(lines) + "\n"


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
