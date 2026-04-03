#!/usr/bin/env python3
"""
Cutover readiness runner for Blue Harness.

Workflow:
1. Query the candidate-scoped cutover readiness API.
2. Persist JSON/Markdown artifacts for CI and release review.
"""

from __future__ import annotations

import argparse
import json
import logging
import os
import socket
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional
from urllib import error, request


LOG = logging.getLogger("cutover-readiness-runner")


class HarnessAPIError(RuntimeError):
    pass


class HarnessClient:
    def __init__(self, blue_base_url: str, api_key: str = "", bearer_token: str = "", request_timeout_seconds: float = 90.0):
        self.base_url = normalize_harness_base_url(blue_base_url)
        self.api_key = api_key.strip()
        self.bearer_token = bearer_token.strip()
        self.request_timeout_seconds = max(float(request_timeout_seconds), 1.0)

    def evaluate_cutover_readiness(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        return self._json_request("POST", "/cutover-readiness", payload)

    def _json_request(self, method: str, path: str, payload: Optional[Dict[str, Any]]) -> Dict[str, Any]:
        body = None
        headers = {"Accept": "application/json"}
        if self.bearer_token:
            headers["Authorization"] = f"Bearer {self.bearer_token}"
        elif self.api_key:
            headers["X-API-Key"] = self.api_key
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"

        req = request.Request(f"{self.base_url}{path}", data=body, method=method, headers=headers)
        try:
            with request.urlopen(req, timeout=self.request_timeout_seconds) as resp:
                raw = resp.read().decode("utf-8", errors="replace")
        except error.HTTPError as exc:
            raw = exc.read().decode("utf-8", errors="replace")
            raise HarnessAPIError(f"{method} {path} failed: HTTP {exc.code}: {raw}") from exc
        except (socket.timeout, TimeoutError) as exc:
            raise HarnessAPIError(f"{method} {path} failed: timed out after {self.request_timeout_seconds:g}s") from exc
        except error.URLError as exc:
            raise HarnessAPIError(f"{method} {path} failed: {exc}") from exc

        if not raw.strip():
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            raise HarnessAPIError(f"{method} {path} returned non-JSON body: {raw[:400]}") from exc


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Evaluate candidate-scoped tool -> skill cutover readiness.")
    parser.add_argument(
        "--blue-base-url",
        default="http://127.0.0.1:18080/api/v1",
        help="Blue API base URL. /harness is appended automatically when needed.",
    )
    parser.add_argument(
        "--api-key",
        default=os.environ.get("BLUE_API_KEY", ""),
        help="Optional Blue API key used for authenticated local API access.",
    )
    parser.add_argument(
        "--bearer-token",
        default=os.environ.get("BLUE_BEARER_TOKEN", ""),
        help="Optional Bearer token used for preview-mode or JWT-authenticated local API access.",
    )
    parser.add_argument(
        "--request-timeout-seconds",
        type=float,
        default=float(os.environ.get("BLUE_HARNESS_REQUEST_TIMEOUT_SECONDS", "90")),
        help="HTTP timeout for harness API requests.",
    )
    parser.add_argument(
        "--owner",
        default=os.environ.get("BLUE_USER_ID", "release-gate"),
        help="Owner user id for readiness checks.",
    )
    parser.add_argument(
        "--candidate-id",
        default=os.environ.get("BLUE_CUTOVER_CANDIDATE_ID", ""),
        help="Candidate id to evaluate. When omitted, the API picks the newest shared candidate id.",
    )
    parser.add_argument(
        "--required-consecutive-runs",
        type=int,
        default=2,
        help="Required consecutive green count per lane.",
    )
    parser.add_argument(
        "--max-assessments",
        type=int,
        default=5,
        help="Maximum lane assessments returned in the readiness report.",
    )
    parser.add_argument("--selector-baseline-id", default="", help="Selector baseline id used for readiness gating.")
    parser.add_argument("--selector-base-eval-run-id", default="", help="Selector base eval run id used for readiness gating.")
    parser.add_argument("--execution-baseline-id", default="", help="Execution baseline id used for readiness gating.")
    parser.add_argument("--execution-base-eval-run-id", default="", help="Execution base eval run id used for readiness gating.")
    parser.add_argument("--budget-baseline-id", default="", help="Budget baseline id used for readiness gating.")
    parser.add_argument("--budget-base-eval-run-id", default="", help="Budget base eval run id used for readiness gating.")
    parser.add_argument("--min-pass-rate", type=float, default=None, help="Optional selector minimum curated pass rate override.")
    parser.add_argument("--min-critical-pass-rate", type=float, default=None, help="Optional selector minimum critical curated pass rate override.")
    parser.add_argument("--min-route-agreement-rate", type=float, default=None, help="Optional selector minimum route agreement rate.")
    parser.add_argument("--min-route-compatible-rate", type=float, default=None, help="Optional selector minimum route compatibility rate.")
    parser.add_argument("--max-clarify-rate-delta", type=float, default=None, help="Optional selector maximum clarify-rate delta.")
    parser.add_argument("--selector-max-critical-regressions", type=int, default=None, help="Optional selector maximum critical regression count.")
    parser.add_argument("--max-pass-rate-drop", type=float, default=None, help="Optional execution maximum allowed pass-rate drop.")
    parser.add_argument("--execution-max-critical-regressions", type=int, default=None, help="Optional execution maximum critical regression count.")
    parser.add_argument("--max-verification-pass-rate-drop", type=float, default=None, help="Optional execution maximum verification pass-rate drop.")
    parser.add_argument("--max-evidence-backed-pass-rate-drop", type=float, default=None, help="Optional execution maximum evidence-backed pass-rate drop.")
    parser.add_argument("--min-median-schema-byte-reduction-rate", type=float, default=None, help="Optional budget minimum median schema-byte reduction rate.")
    parser.add_argument("--max-median-latency-increase-rate", type=float, default=None, help="Optional budget maximum median latency increase rate.")
    parser.add_argument("--allowed-final-native-tools", default="", help="Optional comma-separated allowed final native tools override.")
    parser.add_argument(
        "--output-json",
        default="docs/reports/cutover_readiness_report.json",
        help="Where to write the structured JSON output.",
    )
    parser.add_argument(
        "--output-md",
        default="docs/reports/cutover_readiness_report.md",
        help="Where to write the Markdown summary output.",
    )
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable verbose logging.")
    return parser.parse_args()


def normalize_harness_base_url(raw: str) -> str:
    base = raw.strip().rstrip("/")
    if base.endswith("/api/v1/harness"):
        return base
    if base.endswith("/api/v1"):
        return base + "/harness"
    return base + "/api/v1/harness"


def parse_comma_separated_values(raw: str) -> List[str]:
    raw = str(raw or "").strip()
    if not raw:
        return []
    values: List[str] = []
    seen = set()
    for part in raw.split(","):
        value = part.strip()
        if not value:
            continue
        key = value.lower()
        if key in seen:
            continue
        seen.add(key)
        values.append(value)
    return values


def build_cutover_readiness_request(args: argparse.Namespace) -> Dict[str, Any]:
    payload: Dict[str, Any] = {
        "owner_user_id": str(args.owner or "").strip() or "release-gate",
        "required_consecutive_runs": max(int(args.required_consecutive_runs), 1),
        "max_assessments": max(int(args.max_assessments), 1),
    }
    if args.candidate_id:
        payload["candidate_id"] = str(args.candidate_id).strip()

    selector_thresholds: Dict[str, Any] = {}
    if args.min_pass_rate is not None:
        selector_thresholds["min_pass_rate"] = args.min_pass_rate
    if args.min_critical_pass_rate is not None:
        selector_thresholds["min_critical_pass_rate"] = args.min_critical_pass_rate
    if args.min_route_agreement_rate is not None:
        selector_thresholds["min_route_agreement_rate"] = args.min_route_agreement_rate
    if args.min_route_compatible_rate is not None:
        selector_thresholds["min_route_compatible_rate"] = args.min_route_compatible_rate
    if args.max_clarify_rate_delta is not None:
        selector_thresholds["max_clarify_rate_delta"] = args.max_clarify_rate_delta
    if args.selector_max_critical_regressions is not None:
        selector_thresholds["max_critical_regression_count"] = args.selector_max_critical_regressions

    execution_thresholds: Dict[str, Any] = {}
    if args.max_pass_rate_drop is not None:
        execution_thresholds["max_pass_rate_drop"] = args.max_pass_rate_drop
    if args.execution_max_critical_regressions is not None:
        execution_thresholds["max_critical_regression_count"] = args.execution_max_critical_regressions
    if args.max_verification_pass_rate_drop is not None:
        execution_thresholds["max_verification_pass_rate_drop"] = args.max_verification_pass_rate_drop
    if args.max_evidence_backed_pass_rate_drop is not None:
        execution_thresholds["max_evidence_backed_pass_rate_drop"] = args.max_evidence_backed_pass_rate_drop

    budget_thresholds: Dict[str, Any] = {}
    if args.min_median_schema_byte_reduction_rate is not None:
        budget_thresholds["min_median_schema_byte_reduction_rate"] = args.min_median_schema_byte_reduction_rate
    if args.max_median_latency_increase_rate is not None:
        budget_thresholds["max_median_latency_increase_rate"] = args.max_median_latency_increase_rate
    allowed_final_native_tools = parse_comma_separated_values(args.allowed_final_native_tools)
    if allowed_final_native_tools:
        budget_thresholds["allowed_final_native_tools"] = allowed_final_native_tools

    selector: Dict[str, Any] = {}
    if args.selector_baseline_id:
        selector["baseline_id"] = str(args.selector_baseline_id).strip()
    if args.selector_base_eval_run_id:
        selector["base_eval_run_id"] = str(args.selector_base_eval_run_id).strip()
    if selector_thresholds:
        selector["thresholds"] = selector_thresholds
    if selector:
        payload["selector"] = selector

    execution: Dict[str, Any] = {}
    if args.execution_baseline_id:
        execution["baseline_id"] = str(args.execution_baseline_id).strip()
    if args.execution_base_eval_run_id:
        execution["base_eval_run_id"] = str(args.execution_base_eval_run_id).strip()
    if execution_thresholds:
        execution["thresholds"] = execution_thresholds
    if execution:
        payload["execution"] = execution

    budget: Dict[str, Any] = {}
    if args.budget_baseline_id:
        budget["baseline_id"] = str(args.budget_baseline_id).strip()
    if args.budget_base_eval_run_id:
        budget["base_eval_run_id"] = str(args.budget_base_eval_run_id).strip()
    if budget_thresholds:
        budget["thresholds"] = budget_thresholds
    if budget:
        payload["budget"] = budget

    return payload


def run_cutover_readiness_workflow(client: HarnessClient, args: argparse.Namespace) -> Dict[str, Any]:
    request_payload = build_cutover_readiness_request(args)
    report = client.evaluate_cutover_readiness(request_payload)
    return {
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "requested_cutover_readiness": request_payload,
        "cutover_readiness": report,
        "ready": bool(report.get("ready")),
        "status": "ready" if bool(report.get("ready")) else "not_ready",
    }


def build_markdown_report(wrapper: Dict[str, Any]) -> str:
    report = wrapper.get("cutover_readiness") or {}
    lines: List[str] = [
        "# Cutover Readiness Report",
        "",
        f"- Generated At: {wrapper.get('generated_at', '-')}",
        f"- Status: {str(wrapper.get('status') or '-').upper()}",
        f"- Ready: {'yes' if wrapper.get('ready') else 'no'}",
        f"- Candidate ID: {report.get('candidate_id', '-')}",
        f"- Evaluated Gates Ready: {report.get('evaluated_gates_ready', False)}",
        f"- Required Consecutive Runs: {report.get('required_consecutive_runs', '-')}",
    ]

    if report.get("blocking_reasons"):
        lines.extend(["", "## Blocking Reasons", ""])
        lines.extend(f"- {reason}" for reason in report.get("blocking_reasons", []))
    if report.get("unverified_requirements"):
        lines.extend(["", "## Unverified Requirements", ""])
        lines.extend(f"- {reason}" for reason in report.get("unverified_requirements", []))

    append_lane_summary(lines, "Selector Lane", report.get("selector") or {})
    append_lane_summary(lines, "Execution Lane", report.get("execution") or {})
    append_lane_summary(lines, "Budget Lane", report.get("budget") or {})

    return "\n".join(lines) + "\n"


def append_lane_summary(lines: List[str], title: str, lane: Dict[str, Any]) -> None:
    if not isinstance(lane, dict):
        return
    lines.extend(
        [
            "",
            f"## {title}",
            "",
            f"- Ready: {'yes' if lane.get('ready') else 'no'}",
            f"- Candidate ID: {lane.get('candidate_id', '-')}",
            f"- Consecutive Green Count: {lane.get('consecutive_pass_count', 0)}/{lane.get('required_consecutive_runs', 0)}",
            f"- Candidate Run Count: {lane.get('candidate_run_count', 0)}",
            f"- Baseline: {lane.get('baseline_id', '-')}",
            f"- Base Eval Run: {lane.get('base_eval_run_id', '-')}",
            f"- Error: {lane.get('error', '-')}",
        ]
    )
    assessments = lane.get("assessments") if isinstance(lane.get("assessments"), list) else []
    if assessments:
        lines.extend(["", "### Recent Assessments", ""])
        for assessment in assessments[:5]:
            if not isinstance(assessment, dict):
                continue
            failed_checks = assessment.get("failed_checks") if isinstance(assessment.get("failed_checks"), list) else []
            lines.append(
                "- "
                + f"{assessment.get('eval_run_id', '-')} | "
                + f"passed={'yes' if assessment.get('passed') else 'no'} | "
                + f"created_at={assessment.get('created_at', '-')} | "
                + f"failed_checks={','.join(str(item) for item in failed_checks) if failed_checks else '-'}"
            )


def write_report_outputs(report: Dict[str, Any], output_json: Path, output_md: Path) -> None:
    output_json.parent.mkdir(parents=True, exist_ok=True)
    output_md.parent.mkdir(parents=True, exist_ok=True)
    output_json.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    output_md.write_text(build_markdown_report(report), encoding="utf-8")


def main() -> int:
    args = parse_args()
    logging.basicConfig(level=logging.DEBUG if args.verbose else logging.INFO, format="%(levelname)s %(message)s")

    client = HarnessClient(
        args.blue_base_url,
        api_key=args.api_key,
        bearer_token=args.bearer_token,
        request_timeout_seconds=args.request_timeout_seconds,
    )
    try:
        report = run_cutover_readiness_workflow(client, args)
    except Exception as exc:
        LOG.error("cutover readiness workflow failed: %s", exc)
        report = {
            "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            "status": "error",
            "ready": False,
            "error": str(exc),
        }

    write_report_outputs(report, Path(args.output_json), Path(args.output_md))
    if report.get("ready"):
        LOG.info("cutover readiness is green")
        return 0
    LOG.error("cutover readiness is not green; see %s and %s", args.output_json, args.output_md)
    return 1


if __name__ == "__main__":
    sys.exit(main())
