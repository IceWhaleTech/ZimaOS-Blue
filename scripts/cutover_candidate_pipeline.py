#!/usr/bin/env python3
"""
Cutover candidate pipeline orchestrator.

Workflow:
1. Run selector gate for one candidate id.
2. Run execution gate for the same candidate id.
3. Reuse the selector eval run to run budget gate.
4. Run candidate-scoped cutover readiness.
5. Persist an aggregated JSON/Markdown summary.
"""

from __future__ import annotations

import argparse
import json
import logging
import os
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Sequence

import execution_gate_history_report as execution_history
import selector_gate_history_report as selector_history


LOG = logging.getLogger("cutover-candidate-pipeline")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run selector, execution, budget, and readiness checks for one cutover candidate.")
    parser.add_argument("--blue-base-url", default="http://127.0.0.1:18080/api/v1", help="Blue API base URL.")
    parser.add_argument("--api-key", default=os.environ.get("BLUE_API_KEY", ""), help="Optional Blue API key.")
    parser.add_argument(
        "--bearer-token",
        default=os.environ.get("BLUE_BEARER_TOKEN", ""),
        help="Optional Bearer token used for preview-mode or JWT-authenticated local API access.",
    )
    parser.add_argument("--owner", default=os.environ.get("BLUE_USER_ID", "release-gate"), help="Harness owner user id.")
    parser.add_argument("--candidate-id", default=os.environ.get("BLUE_CUTOVER_CANDIDATE_ID", ""), help="Candidate id shared across all lane runs.")
    parser.add_argument("--candidate-label", default=os.environ.get("BLUE_CUTOVER_CANDIDATE_LABEL", ""), help="Human-readable candidate label.")
    parser.add_argument("--selector-eval-spec-id", default="", help="Optional selector eval spec id override.")
    parser.add_argument("--execution-eval-spec-id", default="", help="Optional execution eval spec id override.")
    parser.add_argument("--selector-baseline-id", default="", help="Selector baseline id.")
    parser.add_argument("--selector-base-eval-run-id", default="", help="Selector base eval run id.")
    parser.add_argument("--execution-baseline-id", default="", help="Execution baseline id.")
    parser.add_argument("--execution-base-eval-run-id", default="", help="Execution base eval run id.")
    parser.add_argument("--budget-baseline-id", default="", help="Budget baseline id.")
    parser.add_argument("--budget-base-eval-run-id", default="", help="Budget base eval run id.")
    parser.add_argument("--required-consecutive-runs", type=int, default=2, help="Required consecutive green runs per lane for readiness.")
    parser.add_argument("--max-assessments", type=int, default=5, help="Maximum readiness assessments returned per lane.")
    parser.add_argument("--min-pass-rate", type=float, default=None, help="Selector minimum pass rate.")
    parser.add_argument("--min-critical-pass-rate", type=float, default=None, help="Selector minimum critical pass rate.")
    parser.add_argument("--min-route-agreement-rate", type=float, default=None, help="Selector minimum route agreement rate.")
    parser.add_argument("--min-route-compatible-rate", type=float, default=0.95, help="Selector minimum route compatibility rate.")
    parser.add_argument("--max-clarify-rate-delta", type=float, default=0.01, help="Selector maximum clarify-rate delta.")
    parser.add_argument("--selector-max-critical-regressions", type=int, default=None, help="Selector maximum critical regressions.")
    parser.add_argument("--max-pass-rate-drop", type=float, default=0.01, help="Execution maximum pass-rate drop.")
    parser.add_argument("--execution-max-critical-regressions", type=int, default=0, help="Execution maximum critical regressions.")
    parser.add_argument("--max-verification-pass-rate-drop", type=float, default=0.0, help="Execution maximum verification pass-rate drop.")
    parser.add_argument("--max-evidence-backed-pass-rate-drop", type=float, default=0.0, help="Execution maximum evidence-backed pass-rate drop.")
    parser.add_argument("--min-median-schema-byte-reduction-rate", type=float, default=0.80, help="Budget minimum median schema-byte reduction rate.")
    parser.add_argument("--max-median-latency-increase-rate", type=float, default=0.10, help="Budget maximum median latency increase rate.")
    parser.add_argument("--allowed-final-native-tools", default="exec", help="Comma-separated allowed final native tools.")
    parser.add_argument("--require-consecutive-green", type=int, default=0, help="Optional top-level candidate pipeline streak requirement. When set, history is evaluated after the current run.")
    parser.add_argument("--history-reports", nargs="+", default=[], help="Optional additional report paths or glob patterns used for local history evaluation.")
    parser.add_argument("--history-artifacts-dir", default="", help="Optional directory used to stage current and downloaded pipeline history artifacts.")
    parser.add_argument("--history-output-json", default="", help="Optional cutover candidate pipeline history JSON output path.")
    parser.add_argument("--history-output-md", default="", help="Optional cutover candidate pipeline history Markdown output path.")
    parser.add_argument("--github-repo", default=os.environ.get("GITHUB_REPOSITORY", ""), help="Optional owner/repo used to fetch prior cutover candidate pipeline workflow artifacts.")
    parser.add_argument("--github-token", default=os.environ.get("GITHUB_TOKEN", ""), help="Optional GitHub token used for pipeline history artifact downloads.")
    parser.add_argument("--github-api-base-url", default=os.environ.get("GITHUB_API_URL", "https://api.github.com"), help="GitHub API base URL used for optional history artifact downloads.")
    parser.add_argument("--history-workflow", default="cutover-candidate-pipeline.yml", help="Workflow file name or id used for optional pipeline history artifact downloads.")
    parser.add_argument("--history-branch", default=os.environ.get("GITHUB_REF_NAME", ""), help="Optional branch filter for pipeline history artifact downloads.")
    parser.add_argument("--history-artifact-name", default="cutover-candidate-pipeline-report", help="Artifact name used for optional pipeline history artifact downloads.")
    parser.add_argument("--history-limit-runs", type=int, default=10, help="Maximum number of prior workflow runs to inspect when downloading pipeline history artifacts.")
    parser.add_argument("--history-exclude-run-id", default=os.environ.get("GITHUB_RUN_ID", ""), help="Optional workflow run id excluded from downloaded pipeline history.")
    parser.add_argument("--output-dir", default="docs/reports/cutover_candidate_pipeline", help="Directory for all generated reports.")
    parser.add_argument("--output-json", default="", help="Optional aggregate JSON output path.")
    parser.add_argument("--output-md", default="", help="Optional aggregate Markdown output path.")
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable verbose logging.")
    return parser.parse_args()


def default_candidate_label(candidate_id: str) -> str:
    candidate_id = str(candidate_id or "").strip()
    if candidate_id:
        return candidate_id
    return "cutover-candidate"


def build_step_paths(output_dir: Path, step: str) -> Dict[str, Path]:
    return {
        "json": output_dir / f"{step}.json",
        "md": output_dir / f"{step}.md",
    }


def append_arg(args: List[str], flag: str, value: Any) -> None:
    if value is None:
        return
    if isinstance(value, str):
        value = value.strip()
        if value == "":
            return
    args.extend([flag, str(value)])


def run_command(command: Sequence[str], env: Dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, env=env, capture_output=True, text=True, check=False)


def parse_json_report(path: Path) -> Dict[str, Any]:
    if not path.is_file():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def extract_step_report(steps: Sequence[Dict[str, Any]], step_name: str) -> Dict[str, Any]:
    for step in steps:
        if not isinstance(step, dict):
            continue
        if str(step.get("name") or "").strip() != step_name:
            continue
        report = step.get("report")
        if isinstance(report, dict):
            return report
    return {}


def extract_gate_metrics(step_report: Dict[str, Any], gate_key: str) -> Dict[str, Any]:
    gate = step_report.get(gate_key) if isinstance(step_report.get(gate_key), dict) else {}
    metrics = gate.get("metrics") if isinstance(gate.get("metrics"), dict) else {}
    return metrics if isinstance(metrics, dict) else {}


def build_selector_drift_snapshot(metrics: Dict[str, Any]) -> Dict[str, Any]:
    locale_breakdown = selector_history.normalize_breakdown_map(metrics.get("locale_breakdown"))
    primary_route_breakdown = selector_history.normalize_breakdown_map(metrics.get("primary_route_breakdown"))
    locale_alerts = selector_history.serialize_breakdown_alert_rows(selector_history.select_breakdown_alert_rows(locale_breakdown))
    primary_route_alerts = selector_history.serialize_breakdown_alert_rows(
        selector_history.select_breakdown_alert_rows(primary_route_breakdown)
    )
    return {
        "locale_alert_count": len(locale_alerts),
        "primary_route_alert_count": len(primary_route_alerts),
        "locale_alerts": locale_alerts,
        "primary_route_alerts": primary_route_alerts,
    }


def build_execution_drift_snapshot(metrics: Dict[str, Any]) -> Dict[str, Any]:
    locale_breakdown = execution_history.normalize_breakdown_map(metrics.get("locale_breakdown"))
    primary_route_breakdown = execution_history.normalize_breakdown_map(metrics.get("primary_route_breakdown"))
    locale_alerts = execution_history.serialize_breakdown_alert_rows(execution_history.select_breakdown_alert_rows(locale_breakdown))
    primary_route_alerts = execution_history.serialize_breakdown_alert_rows(
        execution_history.select_breakdown_alert_rows(primary_route_breakdown)
    )
    return {
        "locale_alert_count": len(locale_alerts),
        "primary_route_alert_count": len(primary_route_alerts),
        "locale_alerts": locale_alerts,
        "primary_route_alerts": primary_route_alerts,
    }


def build_pipeline_drift_summary(steps: Sequence[Dict[str, Any]]) -> Dict[str, Any]:
    selector_report = extract_step_report(steps, "selector")
    execution_report = extract_step_report(steps, "execution")
    selector_metrics = extract_gate_metrics(selector_report, "selector_gate")
    execution_metrics = extract_gate_metrics(execution_report, "execution_gate")
    return {
        "selector": build_selector_drift_snapshot(selector_metrics),
        "execution": build_execution_drift_snapshot(execution_metrics),
    }


def step_by_name(steps: Sequence[Dict[str, Any]], name: str) -> Dict[str, Any]:
    for step in steps:
        if not isinstance(step, dict):
            continue
        if str(step.get("name") or "").strip() == name:
            return step
    return {}


def safe_float(value: Any) -> Optional[float]:
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def build_offline_value_report(report: Dict[str, Any]) -> Dict[str, Any]:
    steps = report.get("steps") if isinstance(report.get("steps"), list) else []
    selector_step = step_by_name(steps, "selector")
    execution_step = step_by_name(steps, "execution")
    budget_step = step_by_name(steps, "budget")
    readiness = report.get("readiness") if isinstance(report.get("readiness"), dict) else {}

    selector_report = selector_step.get("report") if isinstance(selector_step.get("report"), dict) else {}
    execution_report = execution_step.get("report") if isinstance(execution_step.get("report"), dict) else {}
    budget_report = budget_step.get("report") if isinstance(budget_step.get("report"), dict) else {}
    selector_metrics = extract_gate_metrics(selector_report, "selector_gate")
    execution_metrics = extract_gate_metrics(execution_report, "execution_gate")
    budget_metrics = extract_gate_metrics(budget_report, "budget_gate")

    top_improvements: List[str] = []
    top_tradeoffs: List[str] = []
    gates_passed = all(bool(step.get("passed")) for step in (selector_step, execution_step, budget_step) if step)

    selector_pass_rate = safe_float(selector_metrics.get("pass_rate"))
    selector_critical_pass_rate = safe_float(selector_metrics.get("critical_pass_rate"))
    if selector_pass_rate is not None and selector_critical_pass_rate is not None and selector_pass_rate >= 0.98 and selector_critical_pass_rate >= 1.0:
        top_improvements.append("Selector lane stayed green.")
    elif selector_step and not selector_step.get("passed"):
        top_tradeoffs.append("Selector gate did not pass.")

    execution_delta = safe_float(execution_metrics.get("pass_rate_delta"))
    verification_delta = safe_float(execution_metrics.get("verification_pass_rate_delta"))
    evidence_delta = safe_float(execution_metrics.get("evidence_backed_pass_rate_delta"))
    if execution_delta is not None:
        if execution_delta > 0:
            top_improvements.append(f"Execution pass rate improved by {execution_delta:.2f}")
        elif execution_delta < 0:
            top_tradeoffs.append(f"Execution pass rate regressed by {abs(execution_delta):.2f}")
    if verification_delta is not None:
        if verification_delta > 0:
            top_improvements.append(f"Verification pass rate improved by {verification_delta:.2f}")
        elif verification_delta < 0:
            top_tradeoffs.append(f"Verification pass rate regressed by {abs(verification_delta):.2f}")
    if evidence_delta is not None:
        if evidence_delta > 0:
            top_improvements.append(f"Evidence-backed pass rate improved by {evidence_delta:.2f}")
        elif evidence_delta < 0:
            top_tradeoffs.append(f"Evidence-backed pass rate regressed by {abs(evidence_delta):.2f}")
    if execution_step and not execution_step.get("passed"):
        top_tradeoffs.append("Execution gate did not pass.")

    schema_reduction = safe_float(budget_metrics.get("median_schema_byte_reduction_rate"))
    latency_increase = safe_float(budget_metrics.get("median_latency_increase_rate"))
    if schema_reduction is not None and schema_reduction > 0:
        top_improvements.append(f"Median schema bytes reduced by {schema_reduction:.0%}")
    if latency_increase is not None and latency_increase > 0.05:
        top_tradeoffs.append(f"Median latency increased by {latency_increase:.0%}")
    if budget_step and not budget_step.get("passed"):
        top_tradeoffs.append("Budget gate did not pass.")

    blocking_reasons = readiness.get("blocking_reasons") if isinstance(readiness.get("blocking_reasons"), list) else []
    for item in blocking_reasons:
        value = str(item or "").strip()
        if value:
            top_tradeoffs.append(value)

    evaluated_gates_ready = bool(readiness.get("evaluated_gates_ready"))
    cutover_ready = bool(report.get("ready"))

    if not gates_passed:
        recommendation = "reject"
        value_summary = "One or more offline gates failed, so this candidate should not be promoted."
        confidence = "high"
    elif blocking_reasons or not evaluated_gates_ready or not cutover_ready:
        recommendation = "reject"
        value_summary = "Execution quality regressed and cutover readiness is blocked." if top_tradeoffs else "Offline gates passed, but cutover readiness is still blocked."
        confidence = "medium"
    else:
        recommendation = "hold"
        value_summary = "Offline gates are green and the candidate looks valuable, but runtime confirmation is still pending."
        confidence = "high"

    return {
        "offline_recommendation": recommendation,
        "value_summary": value_summary,
        "top_improvements": top_improvements,
        "top_tradeoffs": top_tradeoffs,
        "confidence": confidence,
        "selector_pass_rate": selector_pass_rate,
        "selector_critical_pass_rate": selector_critical_pass_rate,
        "execution_pass_rate_delta": execution_delta,
        "verification_pass_rate_delta": verification_delta,
        "evidence_backed_pass_rate_delta": evidence_delta,
        "median_schema_byte_reduction_rate": schema_reduction,
        "median_latency_increase_rate": latency_increase,
        "cutover_ready": cutover_ready,
    }


def default_runtime_value_report() -> Dict[str, Any]:
    return {
        "status": "not_started",
        "value_summary": "Runtime validation begins after promotion.",
        "confidence": "low",
    }


def build_selector_command(args: argparse.Namespace, output_paths: Dict[str, Path]) -> List[str]:
    command = [
        sys.executable,
        "scripts/selector_gate_runner.py",
        "--blue-base-url",
        args.blue_base_url,
        "--owner",
        args.owner,
        "--candidate-id",
        args.candidate_id,
        "--candidate-label",
        args.candidate_label,
        "--output-json",
        str(output_paths["json"]),
        "--output-md",
        str(output_paths["md"]),
    ]
    append_arg(command, "--eval-spec-id", args.selector_eval_spec_id)
    append_arg(command, "--baseline-id", args.selector_baseline_id)
    append_arg(command, "--base-eval-run-id", args.selector_base_eval_run_id)
    append_arg(command, "--min-pass-rate", args.min_pass_rate)
    append_arg(command, "--min-critical-pass-rate", args.min_critical_pass_rate)
    append_arg(command, "--min-route-agreement-rate", args.min_route_agreement_rate)
    append_arg(command, "--min-route-compatible-rate", args.min_route_compatible_rate)
    append_arg(command, "--max-clarify-rate-delta", args.max_clarify_rate_delta)
    append_arg(command, "--max-critical-regressions", args.selector_max_critical_regressions)
    return command


def build_execution_command(args: argparse.Namespace, output_paths: Dict[str, Path]) -> List[str]:
    command = [
        sys.executable,
        "scripts/execution_gate_runner.py",
        "--blue-base-url",
        args.blue_base_url,
        "--owner",
        args.owner,
        "--candidate-id",
        args.candidate_id,
        "--candidate-label",
        args.candidate_label,
        "--output-json",
        str(output_paths["json"]),
        "--output-md",
        str(output_paths["md"]),
    ]
    append_arg(command, "--eval-spec-id", args.execution_eval_spec_id)
    append_arg(command, "--baseline-id", args.execution_baseline_id)
    append_arg(command, "--base-eval-run-id", args.execution_base_eval_run_id)
    append_arg(command, "--max-pass-rate-drop", args.max_pass_rate_drop)
    append_arg(command, "--max-critical-regressions", args.execution_max_critical_regressions)
    append_arg(command, "--max-verification-pass-rate-drop", args.max_verification_pass_rate_drop)
    append_arg(command, "--max-evidence-backed-pass-rate-drop", args.max_evidence_backed_pass_rate_drop)
    return command


def build_budget_command(args: argparse.Namespace, output_paths: Dict[str, Path], selector_eval_run_id: str) -> List[str]:
    command = [
        sys.executable,
        "scripts/budget_gate_runner.py",
        "--blue-base-url",
        args.blue_base_url,
        "--owner",
        args.owner,
        "--candidate-id",
        args.candidate_id,
        "--candidate-label",
        args.candidate_label,
        "--output-json",
        str(output_paths["json"]),
        "--output-md",
        str(output_paths["md"]),
    ]
    append_arg(command, "--eval-run-id", selector_eval_run_id)
    append_arg(command, "--baseline-id", args.budget_baseline_id)
    append_arg(command, "--base-eval-run-id", args.budget_base_eval_run_id)
    append_arg(command, "--min-median-schema-byte-reduction-rate", args.min_median_schema_byte_reduction_rate)
    append_arg(command, "--max-median-latency-increase-rate", args.max_median_latency_increase_rate)
    append_arg(command, "--allowed-final-native-tools", args.allowed_final_native_tools)
    return command


def build_readiness_command(args: argparse.Namespace, output_paths: Dict[str, Path]) -> List[str]:
    command = [
        sys.executable,
        "scripts/cutover_readiness_runner.py",
        "--blue-base-url",
        args.blue_base_url,
        "--owner",
        args.owner,
        "--required-consecutive-runs",
        str(args.required_consecutive_runs),
        "--max-assessments",
        str(args.max_assessments),
        "--output-json",
        str(output_paths["json"]),
        "--output-md",
        str(output_paths["md"]),
    ]
    append_arg(command, "--candidate-id", args.candidate_id)
    append_arg(command, "--selector-baseline-id", args.selector_baseline_id)
    append_arg(command, "--selector-base-eval-run-id", args.selector_base_eval_run_id)
    append_arg(command, "--execution-baseline-id", args.execution_baseline_id)
    append_arg(command, "--execution-base-eval-run-id", args.execution_base_eval_run_id)
    append_arg(command, "--budget-baseline-id", args.budget_baseline_id)
    append_arg(command, "--budget-base-eval-run-id", args.budget_base_eval_run_id)
    append_arg(command, "--min-pass-rate", args.min_pass_rate)
    append_arg(command, "--min-critical-pass-rate", args.min_critical_pass_rate)
    append_arg(command, "--min-route-agreement-rate", args.min_route_agreement_rate)
    append_arg(command, "--min-route-compatible-rate", args.min_route_compatible_rate)
    append_arg(command, "--max-clarify-rate-delta", args.max_clarify_rate_delta)
    append_arg(command, "--selector-max-critical-regressions", args.selector_max_critical_regressions)
    append_arg(command, "--max-pass-rate-drop", args.max_pass_rate_drop)
    append_arg(command, "--execution-max-critical-regressions", args.execution_max_critical_regressions)
    append_arg(command, "--max-verification-pass-rate-drop", args.max_verification_pass_rate_drop)
    append_arg(command, "--max-evidence-backed-pass-rate-drop", args.max_evidence_backed_pass_rate_drop)
    append_arg(command, "--min-median-schema-byte-reduction-rate", args.min_median_schema_byte_reduction_rate)
    append_arg(command, "--max-median-latency-increase-rate", args.max_median_latency_increase_rate)
    append_arg(command, "--allowed-final-native-tools", args.allowed_final_native_tools)
    return command


def record_step(
    name: str,
    command: Sequence[str],
    completed: subprocess.CompletedProcess[str],
    output_paths: Dict[str, Path],
    parsed_report: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    return {
        "name": name,
        "command": list(command),
        "returncode": int(completed.returncode),
        "passed": completed.returncode == 0,
        "stdout": completed.stdout,
        "stderr": completed.stderr,
        "output_json": str(output_paths["json"]),
        "output_md": str(output_paths["md"]),
        "report": parsed_report or {},
    }


def run_cutover_candidate_pipeline(
    args: argparse.Namespace,
    *,
    runner=run_command,
) -> Dict[str, Any]:
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    api_key = str(args.api_key or "").strip()
    bearer_token = str(args.bearer_token or "").strip()
    env = dict(os.environ)
    if api_key:
        env["BLUE_API_KEY"] = api_key
    if bearer_token:
        env["BLUE_BEARER_TOKEN"] = bearer_token

    raw_candidate_id = str(args.candidate_id or "").strip()
    candidate_label = str(args.candidate_label or "").strip() or default_candidate_label(raw_candidate_id)
    candidate_id = raw_candidate_id or candidate_label
    args.candidate_id = candidate_id
    args.candidate_label = candidate_label

    steps: List[Dict[str, Any]] = []

    selector_paths = build_step_paths(output_dir, "selector_gate_report")
    selector_command = build_selector_command(args, selector_paths)
    selector_completed = runner(selector_command, env)
    selector_report = parse_json_report(selector_paths["json"])
    steps.append(record_step("selector", selector_command, selector_completed, selector_paths, selector_report))
    selector_eval_run_id = str(((selector_report.get("eval_run") or {}) if isinstance(selector_report, dict) else {}).get("id") or "").strip()

    execution_paths = build_step_paths(output_dir, "execution_gate_report")
    execution_command = build_execution_command(args, execution_paths)
    execution_completed = runner(execution_command, env)
    execution_report = parse_json_report(execution_paths["json"])
    steps.append(record_step("execution", execution_command, execution_completed, execution_paths, execution_report))

    budget_paths = build_step_paths(output_dir, "budget_gate_report")
    if selector_eval_run_id:
        budget_command = build_budget_command(args, budget_paths, selector_eval_run_id)
        budget_completed = runner(budget_command, env)
        budget_report = parse_json_report(budget_paths["json"])
        steps.append(record_step("budget", budget_command, budget_completed, budget_paths, budget_report))
    else:
        skipped = {
            "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            "status": "skipped",
            "passed": False,
            "error": "selector eval run id unavailable; budget gate was not executed",
        }
        budget_paths["json"].write_text(json.dumps(skipped, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        budget_paths["md"].write_text("# Budget Gate Report\n\n- Status: SKIPPED\n- Reason: selector eval run id unavailable\n", encoding="utf-8")
        steps.append(
            {
                "name": "budget",
                "command": [],
                "returncode": 1,
                "passed": False,
                "stdout": "",
                "stderr": "selector eval run id unavailable",
                "output_json": str(budget_paths["json"]),
                "output_md": str(budget_paths["md"]),
                "report": skipped,
            }
        )

    readiness_paths = build_step_paths(output_dir, "cutover_readiness_report")
    readiness_command = build_readiness_command(args, readiness_paths)
    readiness_completed = runner(readiness_command, env)
    readiness_report = parse_json_report(readiness_paths["json"])
    steps.append(record_step("readiness", readiness_command, readiness_completed, readiness_paths, readiness_report))

    readiness_wrapper = readiness_report if isinstance(readiness_report, dict) else {}
    readiness_data = readiness_wrapper.get("cutover_readiness") if isinstance(readiness_wrapper.get("cutover_readiness"), dict) else {}
    overall_ready = bool(readiness_wrapper.get("ready"))

    report = {
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "candidate_id": candidate_id,
        "candidate_label": candidate_label,
        "output_dir": str(output_dir),
        "selector_eval_run_id": selector_eval_run_id,
        "attempt_status": "ready" if overall_ready else "not_ready",
        "attempt_ready": overall_ready,
        "status": "ready" if overall_ready else "not_ready",
        "ready": overall_ready,
        "steps": steps,
        "readiness": readiness_data,
        "drift_summary": build_pipeline_drift_summary(steps),
    }
    offline_value_report = build_offline_value_report(report)
    runtime_value_report = default_runtime_value_report()
    report["offline_value_report"] = offline_value_report
    report["offline_recommendation"] = offline_value_report.get("offline_recommendation", "")
    report["runtime_value_report"] = runtime_value_report
    report["runtime_status"] = runtime_value_report.get("status", "")
    report["value_summary"] = offline_value_report.get("value_summary", "")
    return report


def build_markdown_report(report: Dict[str, Any]) -> str:
    lines: List[str] = [
        "# Cutover Candidate Pipeline Report",
        "",
        f"- Generated At: {report.get('generated_at', '-')}",
        f"- Candidate ID: {report.get('candidate_id', '-')}",
        f"- Candidate Label: {report.get('candidate_label', '-')}",
        f"- Status: {str(report.get('status') or '-').upper()}",
        f"- Cutover Ready: {'yes' if report.get('ready') else 'no'}",
        f"- Attempt Status: {str(report.get('attempt_status') or report.get('status') or '-').upper()}",
        f"- Attempt Ready: {'yes' if report.get('attempt_ready') else 'no'}",
        f"- Selector Eval Run ID: {report.get('selector_eval_run_id', '-')}",
    ]

    lines.extend(["", "## Step Summary", ""])
    for step in report.get("steps", []):
        lines.append(
            "- "
            + f"{step.get('name', '-')}: "
            + f"{'pass' if step.get('passed') else 'fail'} | "
            + f"returncode={step.get('returncode', '-')} | "
            + f"json={step.get('output_json', '-')}"
        )

    readiness = report.get("readiness") or {}
    if readiness:
        lines.extend(
            [
                "",
                "## Readiness Snapshot",
                "",
                f"- Evaluated Gates Ready: {readiness.get('evaluated_gates_ready', False)}",
                f"- Selector Streak: {((readiness.get('selector') or {}).get('consecutive_pass_count', 0))}/{readiness.get('required_consecutive_runs', 0)}",
                f"- Execution Streak: {((readiness.get('execution') or {}).get('consecutive_pass_count', 0))}/{readiness.get('required_consecutive_runs', 0)}",
                f"- Budget Streak: {((readiness.get('budget') or {}).get('consecutive_pass_count', 0))}/{readiness.get('required_consecutive_runs', 0)}",
            ]
        )
        blocking = readiness.get("blocking_reasons") if isinstance(readiness.get("blocking_reasons"), list) else []
        if blocking:
            lines.extend(["", "## Blocking Reasons", ""])
            lines.extend(f"- {item}" for item in blocking)

    offline_value = report.get("offline_value_report") if isinstance(report.get("offline_value_report"), dict) else {}
    runtime_value = report.get("runtime_value_report") if isinstance(report.get("runtime_value_report"), dict) else {}
    if offline_value or runtime_value:
        lines.extend(
            [
                "",
                "## Offline Value",
                "",
                f"- Recommendation: {str(offline_value.get('offline_recommendation') or '-').upper()}",
                f"- Confidence: {offline_value.get('confidence', '-')}",
                f"- Summary: {offline_value.get('value_summary', '-')}",
                f"- Runtime Status: {str(report.get('runtime_status') or runtime_value.get('status') or '-').upper()}",
            ]
        )
        improvements = offline_value.get("top_improvements") if isinstance(offline_value.get("top_improvements"), list) else []
        tradeoffs = offline_value.get("top_tradeoffs") if isinstance(offline_value.get("top_tradeoffs"), list) else []
        if improvements:
            lines.extend(["", "### Top Improvements", ""])
            lines.extend(f"- {item}" for item in improvements)
        if tradeoffs:
            lines.extend(["", "### Top Tradeoffs", ""])
            lines.extend(f"- {item}" for item in tradeoffs)

    drift_summary = report.get("drift_summary") or {}
    selector_drift = drift_summary.get("selector") if isinstance(drift_summary.get("selector"), dict) else {}
    execution_drift = drift_summary.get("execution") if isinstance(drift_summary.get("execution"), dict) else {}
    if selector_drift or execution_drift:
        lines.extend(
            [
                "",
                "## Drift Snapshot",
                "",
                f"- Selector locale drift alerts: {selector_drift.get('locale_alert_count', 0)}",
                f"- Selector primary-route drift alerts: {selector_drift.get('primary_route_alert_count', 0)}",
                f"- Execution locale drift alerts: {execution_drift.get('locale_alert_count', 0)}",
                f"- Execution primary-route drift alerts: {execution_drift.get('primary_route_alert_count', 0)}",
            ]
        )
        selector_history.append_breakdown_section(lines, "Selector Locale Drift Alerts", selector_drift.get("locale_alerts") or [])
        selector_history.append_breakdown_section(
            lines, "Selector Primary Route Drift Alerts", selector_drift.get("primary_route_alerts") or []
        )
        execution_history.append_breakdown_section(lines, "Execution Locale Drift Alerts", execution_drift.get("locale_alerts") or [])
        execution_history.append_breakdown_section(
            lines, "Execution Primary Route Drift Alerts", execution_drift.get("primary_route_alerts") or []
        )

    history_gate = report.get("history_gate") if isinstance(report.get("history_gate"), dict) else {}
    if history_gate.get("evaluated"):
        lines.extend(
            [
                "",
                "## History Gate",
                "",
                f"- Requirement Passed: {'yes' if history_gate.get('requirement_passed') else 'no'}",
                f"- Consecutive Green Requirement: {history_gate.get('require_consecutive_green', '-')}",
                f"- Focus Candidate ID: {history_gate.get('focus_candidate_id', '-')}",
                f"- Focus Candidate: {history_gate.get('focus_candidate_label', '-')}",
                f"- Attempt Count: {history_gate.get('attempt_count', '-')}",
                f"- Ready Count: {history_gate.get('ready_count', '-')}",
                f"- Consecutive Green Count: {history_gate.get('consecutive_green_count', '-')}",
                f"- Latest Status: {history_gate.get('latest_status', '-')}",
                f"- Latest Ready: {'yes' if history_gate.get('latest_ready') else 'no'}",
                f"- History JSON: {history_gate.get('output_json', '-')}",
                f"- History Markdown: {history_gate.get('output_md', '-')}",
            ]
        )
    elif history_gate.get("error"):
        lines.extend(
            [
                "",
                "## History Gate",
                "",
                "- Status: ERROR",
                f"- Error: {history_gate.get('error', '-')}",
            ]
        )

    final_gate = report.get("final_gate") if isinstance(report.get("final_gate"), dict) else {}
    if final_gate:
        lines.extend(
            [
                "",
                "## Final Gate",
                "",
                f"- Cutover Ready: {'yes' if final_gate.get('cutover_ready') else 'no'}",
                f"- History Gate Evaluated: {'yes' if final_gate.get('history_gate_evaluated') else 'no'}",
                f"- Blocking Reason Count: {len(final_gate.get('blocking_reasons') or []) if isinstance(final_gate.get('blocking_reasons'), list) else 0}",
            ]
        )
        blocking_reasons = final_gate.get("blocking_reasons") if isinstance(final_gate.get("blocking_reasons"), list) else []
        if blocking_reasons:
            lines.extend(["", "## Final Blocking Reasons", ""])
            lines.extend(f"- {item}" for item in blocking_reasons)

    return "\n".join(lines) + "\n"


def write_report_outputs(report: Dict[str, Any], output_json: Path, output_md: Path) -> None:
    output_json.parent.mkdir(parents=True, exist_ok=True)
    output_md.parent.mkdir(parents=True, exist_ok=True)
    output_json.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    output_md.write_text(build_markdown_report(report), encoding="utf-8")


def summarize_history_gate(history_result: Dict[str, Any]) -> Dict[str, Any]:
    summary = history_result.get("summary") if isinstance(history_result.get("summary"), dict) else {}
    focused = summary.get("focused_candidate") if isinstance(summary.get("focused_candidate"), dict) else {}
    latest_attempt = focused.get("latest_attempt") if isinstance(focused.get("latest_attempt"), dict) else {}
    return {
        "evaluated": True,
        "requirement_passed": bool(summary.get("requirement_passed")),
        "require_consecutive_green": summary.get("require_consecutive_green"),
        "focus_candidate_id": summary.get("focus_candidate_id"),
        "focus_candidate_label": summary.get("focus_candidate_label"),
        "attempt_count": focused.get("attempt_count"),
        "ready_count": focused.get("ready_count"),
        "consecutive_green_count": focused.get("consecutive_green_count"),
        "latest_status": latest_attempt.get("status"),
        "latest_ready": latest_attempt.get("ready"),
        "output_json": history_result.get("output_json"),
        "output_md": history_result.get("output_md"),
        "artifacts_dir": history_result.get("artifacts_dir"),
        "download_manifest": history_result.get("download_manifest"),
    }


def apply_final_gate_state(report: Dict[str, Any]) -> Dict[str, Any]:
    attempt_status = str(report.get("attempt_status") or report.get("status") or "").strip() or "not_ready"
    attempt_ready = bool(report.get("attempt_ready")) if "attempt_ready" in report else bool(report.get("ready"))

    blocking_reasons: List[str] = []
    if not attempt_ready:
        readiness = report.get("readiness") if isinstance(report.get("readiness"), dict) else {}
        readiness_reasons = readiness.get("blocking_reasons") if isinstance(readiness.get("blocking_reasons"), list) else []
        blocking_reasons.extend(str(item) for item in readiness_reasons if str(item).strip())
        if not blocking_reasons:
            if attempt_status == "error":
                blocking_reasons.append(str(report.get("error") or "current attempt failed"))
            else:
                blocking_reasons.append(f"current candidate attempt is {attempt_status}")

    history_gate = report.get("history_gate") if isinstance(report.get("history_gate"), dict) else {}
    if history_gate.get("error"):
        blocking_reasons.append(f"history gate evaluation failed: {history_gate.get('error')}")
    elif history_gate.get("evaluated") and not bool(history_gate.get("requirement_passed")):
        current = history_gate.get("consecutive_green_count")
        required = history_gate.get("require_consecutive_green")
        focus_candidate_id = history_gate.get("focus_candidate_id") or report.get("candidate_id") or "candidate"
        blocking_reasons.append(f"candidate {focus_candidate_id} has only {current}/{required} consecutive green attempts")

    deduped: List[str] = []
    seen = set()
    for reason in blocking_reasons:
        key = str(reason).strip()
        if not key or key in seen:
            continue
        seen.add(key)
        deduped.append(key)

    cutover_ready = not deduped
    final_status = "ready" if cutover_ready else ("error" if attempt_status == "error" else "not_ready")
    report["attempt_status"] = attempt_status
    report["attempt_ready"] = attempt_ready
    report["final_gate"] = {
        "cutover_ready": cutover_ready,
        "history_gate_evaluated": bool(history_gate.get("evaluated")) or bool(history_gate.get("error")),
        "blocking_reasons": deduped,
    }
    report["status"] = final_status
    report["ready"] = cutover_ready
    return report


def history_requested(args: argparse.Namespace) -> bool:
    if int(getattr(args, "require_consecutive_green", 0) or 0) > 0:
        return True
    if any(str(pattern or "").strip() for pattern in getattr(args, "history_reports", []) or []):
        return True
    if str(getattr(args, "github_repo", "") or "").strip():
        return True
    if str(getattr(args, "history_output_json", "") or "").strip():
        return True
    if str(getattr(args, "history_output_md", "") or "").strip():
        return True
    return False


def resolve_history_artifacts_dir(args: argparse.Namespace, output_dir: Path) -> Path:
    value = str(getattr(args, "history_artifacts_dir", "") or "").strip()
    if value:
        return Path(value)
    return output_dir / "history_artifacts"


def resolve_history_output_paths(args: argparse.Namespace, output_dir: Path) -> Dict[str, Path]:
    output_json = str(getattr(args, "history_output_json", "") or "").strip()
    output_md = str(getattr(args, "history_output_md", "") or "").strip()
    return {
        "json": Path(output_json) if output_json else output_dir / "cutover_candidate_pipeline_history_report.json",
        "md": Path(output_md) if output_md else output_dir / "cutover_candidate_pipeline_history_report.md",
    }


def normalize_history_patterns(raw_patterns: Sequence[str]) -> List[str]:
    patterns: List[str] = []
    seen = set()
    for raw in raw_patterns:
        value = str(raw or "").strip()
        if not value or value in seen:
            continue
        seen.add(value)
        patterns.append(value)
    return patterns


def evaluate_history_gate(args: argparse.Namespace, output_dir: Path, current_report_json: Path) -> Dict[str, Any]:
    import cutover_candidate_pipeline_artifact_fetcher as history_fetcher
    import cutover_candidate_pipeline_history_report as history_report

    artifacts_dir = resolve_history_artifacts_dir(args, output_dir)
    outputs = resolve_history_output_paths(args, output_dir)
    shutil.rmtree(artifacts_dir, ignore_errors=True)
    current_dir = artifacts_dir / "current"
    current_dir.mkdir(parents=True, exist_ok=True)
    staged_current = current_dir / f"{current_report_json.stem}-current.json"
    shutil.copy2(current_report_json, staged_current)

    report_patterns = [str(current_dir / "*.json")]
    report_patterns.extend(normalize_history_patterns(getattr(args, "history_reports", []) or []))

    download_manifest_path: Optional[Path] = None
    repo = str(getattr(args, "github_repo", "") or "").strip()
    if repo:
        download_dir = artifacts_dir / "downloaded"
        download_manifest_path = artifacts_dir / "download-manifest.json"
        client = history_fetcher.GitHubActionsClient(
            repo=repo,
            api_base_url=str(getattr(args, "github_api_base_url", "") or "").strip(),
            token=str(getattr(args, "github_token", "") or "").strip(),
        )
        fetch_args = argparse.Namespace(
            workflow=str(getattr(args, "history_workflow", "") or "").strip(),
            branch=str(getattr(args, "history_branch", "") or "").strip(),
            exclude_run_id=str(getattr(args, "history_exclude_run_id", "") or "").strip(),
            limit_runs=max(int(getattr(args, "history_limit_runs", 10) or 10), 0),
            artifact_name=str(getattr(args, "history_artifact_name", "") or "").strip(),
            output_dir=str(download_dir),
            output_json=str(download_manifest_path),
        )
        manifest = history_fetcher.fetch_cutover_candidate_pipeline_history(client, fetch_args)
        history_fetcher.write_manifest(manifest, download_manifest_path)
        report_patterns.append(str(download_dir / "pipeline_reports" / "*.json"))

    attempts = history_report.load_attempts(history_report.expand_report_paths(report_patterns))
    summary = history_report.build_summary(
        attempts=attempts,
        candidate_id=str(getattr(args, "candidate_id", "") or "").strip(),
        candidate_label=str(getattr(args, "candidate_label", "") or "").strip(),
        require_consecutive_green=max(int(getattr(args, "require_consecutive_green", 0) or 0), 0),
    )
    history_report.write_outputs(summary, outputs["json"], outputs["md"], "Cutover Candidate Pipeline History Summary")
    return {
        "summary": summary,
        "output_json": str(outputs["json"]),
        "output_md": str(outputs["md"]),
        "artifacts_dir": str(artifacts_dir),
        "download_manifest": str(download_manifest_path) if download_manifest_path is not None else "",
        "report_patterns": report_patterns,
    }


def main() -> int:
    args = parse_args()
    logging.basicConfig(level=logging.DEBUG if args.verbose else logging.INFO, format="%(levelname)s %(message)s")
    try:
        report = run_cutover_candidate_pipeline(args)
    except Exception as exc:
        LOG.error("cutover candidate pipeline failed: %s", exc)
        report = {
            "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            "candidate_id": str(args.candidate_id or "").strip(),
            "candidate_label": str(args.candidate_label or "").strip() or default_candidate_label(getattr(args, "candidate_id", "")),
            "attempt_status": "error",
            "attempt_ready": False,
            "status": "error",
            "ready": False,
            "error": str(exc),
            "steps": [],
        }

    output_dir = Path(args.output_dir)
    output_json = Path(args.output_json) if str(args.output_json or "").strip() else output_dir / "cutover_candidate_pipeline_report.json"
    output_md = Path(args.output_md) if str(args.output_md or "").strip() else output_dir / "cutover_candidate_pipeline_report.md"
    write_report_outputs(report, output_json, output_md)

    history_result: Optional[Dict[str, Any]] = None
    history_failed = False
    if history_requested(args):
        try:
            history_result = evaluate_history_gate(args, output_dir, output_json)
        except Exception as exc:
            history_failed = True
            LOG.error("cutover candidate pipeline history evaluation failed: %s", exc)
            report["history_gate"] = {
                "evaluated": False,
                "requirement_passed": False,
                "error": str(exc),
            }
        else:
            report["history_gate"] = summarize_history_gate(history_result)
            summary = history_result.get("summary") if isinstance(history_result, dict) else {}
            if not bool((summary or {}).get("requirement_passed", True)):
                history_failed = True
                LOG.error(
                    "cutover candidate pipeline history gate is not green; see %s and %s",
                    history_result.get("output_json", "-"),
                    history_result.get("output_md", "-"),
                )
            else:
                LOG.info(
                    "cutover candidate pipeline history gate passed; see %s and %s",
                    history_result.get("output_json", "-"),
                    history_result.get("output_md", "-"),
                )

    report = apply_final_gate_state(report)
    write_report_outputs(report, output_json, output_md)

    if report.get("ready") and not history_failed:
        LOG.info("cutover candidate pipeline is green")
        return 0
    LOG.error("cutover candidate pipeline is not green; see %s and %s", output_json, output_md)
    return 1


if __name__ == "__main__":
    sys.exit(main())
