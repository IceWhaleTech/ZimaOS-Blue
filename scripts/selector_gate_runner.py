#!/usr/bin/env python3
"""
Selector curated release-gate runner for Blue Harness.

Workflow:
1. Ensure the built-in selector curated dataset assets exist.
2. Submit a candidate eval run.
3. Poll until the eval run reaches a terminal state.
4. Evaluate the selector gate.
5. Fetch the persisted comparison report and write JSON/Markdown artifacts.
"""

from __future__ import annotations

import argparse
import json
import logging
import os
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
from urllib import error, request


LOG = logging.getLogger("selector-gate-runner")

TERMINAL_EVAL_STATUSES = {"completed", "partial", "failed", "cancelled"}


class HarnessAPIError(RuntimeError):
    pass


class MissingComparisonBaseError(HarnessAPIError):
    def __init__(self, message: str, details: Optional[Dict[str, Any]] = None):
        super().__init__(message)
        self.details = details or {}


class HarnessClient:
    def __init__(self, blue_base_url: str, api_key: str = "", bearer_token: str = ""):
        self.base_url = normalize_harness_base_url(blue_base_url)
        self.api_key = api_key.strip()
        self.bearer_token = bearer_token.strip()

    def ensure_selector_curated_assets(self, owner_user_id: str) -> Dict[str, Any]:
        return self._json_request(
            "POST",
            "/selector-curated/ensure",
            {"owner_user_id": owner_user_id},
        )

    def create_eval_run(
        self,
        eval_spec_id: str,
        owner_user_id: str,
        title: str,
        base_eval_run_id: str,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        merged_metadata: Dict[str, Any] = {
            "source": "scripts/selector_gate_runner.py",
        }
        if isinstance(metadata, dict):
            merged_metadata.update(metadata)
        payload: Dict[str, Any] = {
            "eval_spec_id": eval_spec_id,
            "owner_user_id": owner_user_id,
            "trigger_kind": "selector_gate_runner",
            "metadata": merged_metadata,
        }
        if title:
            payload["title"] = title
        if base_eval_run_id:
            payload["baseline_eval_run_id"] = base_eval_run_id
        return self._json_request("POST", "/eval-runs", payload)

    def get_eval_run(self, eval_run_id: str) -> Dict[str, Any]:
        return self._json_request("GET", f"/eval-runs/{eval_run_id}", None)

    def list_eval_runs(
        self,
        *,
        eval_spec_id: str = "",
        limit: int = 50,
        statuses: Optional[List[str]] = None,
    ) -> List[Dict[str, Any]]:
        query_params: List[Tuple[str, str]] = []
        if eval_spec_id:
            query_params.append(("eval_spec_id", str(eval_spec_id).strip()))
        if limit > 0:
            query_params.append(("limit", str(limit)))
        for status in statuses or []:
            cleaned = str(status or "").strip()
            if cleaned:
                query_params.append(("status", cleaned))
        suffix = ""
        if query_params:
            suffix = "?" + "&".join(f"{key}={request.quote(value, safe='')}" for key, value in query_params)
        response = self._json_request("GET", f"/eval-runs{suffix}", None)
        if isinstance(response, list):
            return response
        return []

    def list_baselines(
        self,
        *,
        eval_spec_id: str = "",
        limit: int = 50,
    ) -> List[Dict[str, Any]]:
        query_params: List[Tuple[str, str]] = []
        if eval_spec_id:
            query_params.append(("eval_spec_id", str(eval_spec_id).strip()))
        if limit > 0:
            query_params.append(("limit", str(limit)))
        suffix = ""
        if query_params:
            suffix = "?" + "&".join(f"{key}={request.quote(value, safe='')}" for key, value in query_params)
        response = self._json_request("GET", f"/baselines{suffix}", None)
        if isinstance(response, list):
            return response
        return []

    def create_baseline(
        self,
        *,
        name: str,
        eval_run_id: str,
        eval_spec_id: str = "",
        metadata: Optional[Dict[str, Any]] = None,
        is_default: bool = False,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {
            "name": str(name or "").strip(),
            "eval_run_id": str(eval_run_id or "").strip(),
            "is_default": bool(is_default),
        }
        if eval_spec_id:
            payload["eval_spec_id"] = str(eval_spec_id).strip()
        if isinstance(metadata, dict) and metadata:
            payload["metadata"] = metadata
        return self._json_request("POST", "/baselines", payload)

    def evaluate_selector_gate(self, eval_run_id: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        return self._json_request("POST", f"/eval-runs/{eval_run_id}/selector-gate", payload)

    def get_comparison_report(self, comparison_report_id: str) -> Dict[str, Any]:
        return self._json_request("GET", f"/comparison-reports/{comparison_report_id}", None)

    def compare_eval_run(self, eval_run_id: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        return self._json_request("POST", f"/eval-runs/{eval_run_id}/compare", payload)

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
            with request.urlopen(req, timeout=30.0) as resp:
                raw = resp.read().decode("utf-8", errors="replace")
        except error.HTTPError as exc:
            raw = exc.read().decode("utf-8", errors="replace")
            raise HarnessAPIError(f"{method} {path} failed: HTTP {exc.code}: {raw}") from exc
        except error.URLError as exc:
            raise HarnessAPIError(f"{method} {path} failed: {exc}") from exc

        if not raw.strip():
            return {}
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            raise HarnessAPIError(f"{method} {path} returned non-JSON body: {raw[:400]}") from exc


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the Blue selector curated gate and emit report artifacts.")
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
        "--owner",
        default=os.environ.get("BLUE_USER_ID", "release-gate"),
        help="Owner user id for selector curated assets and eval runs.",
    )
    parser.add_argument(
        "--eval-spec-id",
        default="",
        help="Optional eval spec id. When omitted, the builtin selector curated spec is ensured and used.",
    )
    parser.add_argument(
        "--title",
        default="",
        help="Optional eval run title. Defaults to a UTC timestamped selector-gate title.",
    )
    parser.add_argument(
        "--candidate-label",
        default=os.environ.get("BLUE_SELECTOR_GATE_CANDIDATE_LABEL", ""),
        help="Optional stable candidate label used for historical gating reports.",
    )
    parser.add_argument(
        "--candidate-id",
        default=os.environ.get("BLUE_SELECTOR_GATE_CANDIDATE_ID", ""),
        help="Optional stable candidate id used for readiness-aligned historical gating reports.",
    )
    parser.add_argument(
        "--baseline-id",
        default="",
        help="Optional named baseline to compare against.",
    )
    parser.add_argument(
        "--base-eval-run-id",
        default="",
        help="Optional base eval run id to compare against and persist on the candidate run.",
    )
    parser.add_argument(
        "--poll-interval-seconds",
        type=float,
        default=1.0,
        help="Polling interval while waiting for the eval run to complete.",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=float,
        default=600.0,
        help="Maximum time to wait for the eval run to reach a terminal state.",
    )
    parser.add_argument(
        "--min-pass-rate",
        type=float,
        default=None,
        help="Optional minimum curated pass rate override.",
    )
    parser.add_argument(
        "--min-critical-pass-rate",
        type=float,
        default=None,
        help="Optional minimum critical curated pass rate override.",
    )
    parser.add_argument(
        "--min-route-agreement-rate",
        type=float,
        default=None,
        help="Optional minimum route agreement rate.",
    )
    parser.add_argument(
        "--min-route-compatible-rate",
        type=float,
        default=None,
        help="Optional minimum route compatibility rate where truth-improving disagreements still count.",
    )
    parser.add_argument(
        "--max-clarify-rate-delta",
        type=float,
        default=None,
        help="Optional maximum clarify-rate delta.",
    )
    parser.add_argument(
        "--max-critical-regressions",
        type=int,
        default=None,
        help="Optional maximum critical regression count.",
    )
    parser.add_argument(
        "--output-json",
        default="docs/reports/selector_gate_report.json",
        help="Where to write the structured JSON output.",
    )
    parser.add_argument(
        "--output-md",
        default="docs/reports/selector_gate_report.md",
        help="Where to write the Markdown summary output.",
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Enable verbose logging.",
    )
    return parser.parse_args()


def normalize_harness_base_url(raw: str) -> str:
    base = raw.strip().rstrip("/")
    if base.endswith("/api/v1/harness"):
        return base
    if base.endswith("/api/v1"):
        return base + "/harness"
    return base + "/api/v1/harness"


def resolve_eval_title(raw: str) -> str:
    title = str(raw or "").strip()
    if title:
        return title
    return "selector-gate-" + datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")


def resolve_candidate_label(raw: str, title: str) -> str:
    label = str(raw or "").strip()
    if label:
        return label
    return str(title or "").strip()


def resolve_candidate_id(raw: str, candidate_label: str) -> str:
    candidate_id = str(raw or "").strip()
    if candidate_id:
        return candidate_id
    return str(candidate_label or "").strip()


def build_selector_gate_request(args: argparse.Namespace) -> Dict[str, Any]:
    payload: Dict[str, Any] = {}
    if args.base_eval_run_id:
        payload["base_eval_run_id"] = args.base_eval_run_id
    if args.baseline_id:
        payload["baseline_id"] = args.baseline_id

    thresholds: Dict[str, Any] = {}
    if args.min_pass_rate is not None:
        thresholds["min_pass_rate"] = args.min_pass_rate
    if args.min_critical_pass_rate is not None:
        thresholds["min_critical_pass_rate"] = args.min_critical_pass_rate
    if args.min_route_agreement_rate is not None:
        thresholds["min_route_agreement_rate"] = args.min_route_agreement_rate
    if args.min_route_compatible_rate is not None:
        thresholds["min_route_compatible_rate"] = args.min_route_compatible_rate
    if args.max_clarify_rate_delta is not None:
        thresholds["max_clarify_rate_delta"] = args.max_clarify_rate_delta
    if args.max_critical_regressions is not None:
        thresholds["max_critical_regression_count"] = args.max_critical_regressions
    if thresholds:
        payload["thresholds"] = thresholds
    return payload


def build_selector_gate_request_from_base(
    args: argparse.Namespace,
    *,
    baseline_id: str = "",
    base_eval_run_id: str = "",
) -> Dict[str, Any]:
    payload = build_selector_gate_request(args)
    payload.pop("baseline_id", None)
    payload.pop("base_eval_run_id", None)
    if base_eval_run_id:
        payload["base_eval_run_id"] = str(base_eval_run_id).strip()
    if baseline_id:
        payload["baseline_id"] = str(baseline_id).strip()
    return payload


def wait_for_eval_run(
    client: HarnessClient,
    eval_run_id: str,
    timeout_seconds: float,
    poll_interval_seconds: float,
) -> Dict[str, Any]:
    deadline = time.time() + max(timeout_seconds, 1.0)
    poll_interval = max(poll_interval_seconds, 0.1)
    while True:
        eval_run = client.get_eval_run(eval_run_id)
        status = str(eval_run.get("status") or "").strip().lower()
        if status in TERMINAL_EVAL_STATUSES:
            return eval_run
        if time.time() >= deadline:
            raise TimeoutError(f"timed out waiting for eval run {eval_run_id}")
        time.sleep(poll_interval)


def fetch_comparison_report(
    client: HarnessClient,
    comparison_report_id: str,
    target_eval_run_id: str,
    compare_payload: Dict[str, Any],
) -> Dict[str, Any]:
    if comparison_report_id:
        try:
            return client.get_comparison_report(comparison_report_id)
        except HarnessAPIError as exc:
            if "HTTP 404" not in str(exc):
                raise
            LOG.warning("comparison report %s not fetchable yet; falling back to compare endpoint", comparison_report_id)
    return client.compare_eval_run(target_eval_run_id, compare_payload)


def eval_run_candidate_id(eval_run: Dict[str, Any]) -> str:
    if not isinstance(eval_run, dict):
        return ""
    metadata = eval_run.get("metadata")
    if not isinstance(metadata, dict):
        return ""
    return str(metadata.get("candidate_id") or "").strip()


def baseline_lookup_by_id(baselines: List[Dict[str, Any]]) -> Dict[str, Dict[str, Any]]:
    out: Dict[str, Dict[str, Any]] = {}
    for baseline in baselines:
        if not isinstance(baseline, dict):
            continue
        baseline_id = str(baseline.get("id") or "").strip()
        if baseline_id:
            out[baseline_id] = baseline
    return out


def choose_preferred_baseline(baselines: List[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    for baseline in baselines:
        if isinstance(baseline, dict) and bool(baseline.get("is_default")):
            return baseline
    for baseline in baselines:
        if isinstance(baseline, dict):
            return baseline
    return None


def choose_auto_baseline_eval_run(eval_runs: List[Dict[str, Any]], current_candidate_id: str) -> Optional[Dict[str, Any]]:
    preferred_statuses = ("completed", "partial")
    normalized_candidate_id = str(current_candidate_id or "").strip()
    for status in preferred_statuses:
        different_candidate: List[Dict[str, Any]] = []
        uncategorized: List[Dict[str, Any]] = []
        matching_candidate: List[Dict[str, Any]] = []
        for eval_run in eval_runs:
            if not isinstance(eval_run, dict):
                continue
            run_status = str(eval_run.get("status") or "").strip().lower()
            if run_status != status:
                continue
            run_candidate_id = eval_run_candidate_id(eval_run)
            if normalized_candidate_id and run_candidate_id == normalized_candidate_id:
                matching_candidate.append(eval_run)
                continue
            if run_candidate_id:
                different_candidate.append(eval_run)
                continue
            uncategorized.append(eval_run)
        for bucket in (different_candidate, uncategorized, matching_candidate):
            if bucket:
                return bucket[0]
    return None


def auto_baseline_name(eval_run: Dict[str, Any]) -> str:
    eval_run_id = str((eval_run or {}).get("id") or "").strip() or "unknown-run"
    return f"selector-auto-baseline-{eval_run_id}"


def resolve_selector_comparison_base(
    client: HarnessClient,
    *,
    owner_user_id: str,
    eval_spec_id: str,
    candidate_id: str,
    requested_baseline_id: str,
    requested_base_eval_run_id: str,
) -> Dict[str, Any]:
    resolved: Dict[str, Any] = {
        "owner_user_id": str(owner_user_id or "").strip(),
        "eval_spec_id": str(eval_spec_id or "").strip(),
        "requested_baseline_id": str(requested_baseline_id or "").strip(),
        "requested_base_eval_run_id": str(requested_base_eval_run_id or "").strip(),
        "resolution": "none",
        "baseline_id": "",
        "base_eval_run_id": "",
    }

    baselines: List[Dict[str, Any]] = []
    if eval_spec_id:
        baselines = client.list_baselines(eval_spec_id=eval_spec_id, limit=100)
    baseline_by_id = baseline_lookup_by_id(baselines)

    if resolved["requested_baseline_id"]:
        baseline = baseline_by_id.get(resolved["requested_baseline_id"])
        resolved["baseline_id"] = resolved["requested_baseline_id"]
        resolved["base_eval_run_id"] = str((baseline or {}).get("eval_run_id") or "").strip()
        resolved["resolution"] = "explicit_baseline"
        if baseline:
            resolved["baseline"] = baseline
        return resolved

    if resolved["requested_base_eval_run_id"]:
        resolved["base_eval_run_id"] = resolved["requested_base_eval_run_id"]
        resolved["resolution"] = "explicit_base_eval_run"
        return resolved

    baseline = choose_preferred_baseline(baselines)
    if baseline:
        resolved["baseline_id"] = str(baseline.get("id") or "").strip()
        resolved["base_eval_run_id"] = str(baseline.get("eval_run_id") or "").strip()
        resolved["baseline"] = baseline
        resolved["resolution"] = "existing_default_baseline" if bool(baseline.get("is_default")) else "existing_baseline"
        return resolved

    eval_runs = client.list_eval_runs(
        eval_spec_id=eval_spec_id,
        limit=100,
        statuses=["completed", "partial"],
    )
    candidate_eval_run = choose_auto_baseline_eval_run(eval_runs, candidate_id)
    if not candidate_eval_run:
        raise MissingComparisonBaseError(
            f"no baseline or reusable completed eval run available for eval spec {eval_spec_id or 'unknown'}",
            details={
                "eval_spec_id": str(eval_spec_id or "").strip(),
                "owner_user_id": str(owner_user_id or "").strip(),
                "baseline_count": len(baselines),
                "eligible_eval_run_count": len(eval_runs),
                "eligible_eval_run_ids": [
                    str((eval_run or {}).get("id") or "").strip()
                    for eval_run in eval_runs
                    if isinstance(eval_run, dict)
                ],
                "eligible_eval_run_statuses": [
                    str((eval_run or {}).get("status") or "").strip().lower()
                    for eval_run in eval_runs
                    if isinstance(eval_run, dict)
                ],
            },
        )

    created_baseline = client.create_baseline(
        name=auto_baseline_name(candidate_eval_run),
        eval_run_id=str(candidate_eval_run.get("id") or "").strip(),
        eval_spec_id=eval_spec_id,
        is_default=True,
        metadata={
            "source": "scripts/selector_gate_runner.py",
            "auto_created": True,
            "auto_created_for": "selector_gate",
            "candidate_id": str(candidate_id or "").strip(),
        },
    )
    resolved["baseline_id"] = str(created_baseline.get("id") or "").strip()
    resolved["base_eval_run_id"] = str(created_baseline.get("eval_run_id") or "").strip() or str(
        candidate_eval_run.get("id") or ""
    ).strip()
    resolved["baseline"] = created_baseline
    resolved["baseline_eval_run"] = candidate_eval_run
    resolved["resolution"] = "auto_created_baseline"
    return resolved


def run_selector_gate_workflow(client: HarnessClient, args: argparse.Namespace) -> Dict[str, Any]:
    owner_user_id = str(args.owner or "").strip() or "release-gate"
    assets = client.ensure_selector_curated_assets(owner_user_id)
    eval_spec = assets.get("eval_spec") if isinstance(assets, dict) else {}
    eval_spec_id = str(args.eval_spec_id or "") or str((eval_spec or {}).get("id") or "")
    if not eval_spec_id:
        raise HarnessAPIError("selector curated assets did not include an eval spec id")
    eval_title = resolve_eval_title(args.title)
    candidate_label = resolve_candidate_label(args.candidate_label, eval_title)
    candidate_id = resolve_candidate_id(args.candidate_id, candidate_label)
    resolved_comparison_base = resolve_selector_comparison_base(
        client,
        owner_user_id=owner_user_id,
        eval_spec_id=eval_spec_id,
        candidate_id=candidate_id,
        requested_baseline_id=str(args.baseline_id or ""),
        requested_base_eval_run_id=str(args.base_eval_run_id or ""),
    )

    eval_run = client.create_eval_run(
        eval_spec_id=eval_spec_id,
        owner_user_id=owner_user_id,
        title=eval_title,
        base_eval_run_id=str(resolved_comparison_base.get("base_eval_run_id") or ""),
        metadata={"candidate_id": candidate_id, "candidate_label": candidate_label},
    )
    final_eval_run = wait_for_eval_run(
        client=client,
        eval_run_id=str(eval_run.get("id") or ""),
        timeout_seconds=args.timeout_seconds,
        poll_interval_seconds=args.poll_interval_seconds,
    )

    report: Dict[str, Any] = {
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "owner_user_id": owner_user_id,
        "candidate_id": candidate_id,
        "candidate_label": candidate_label,
        "assets": assets,
        "eval_run": final_eval_run,
        "resolved_comparison_base": resolved_comparison_base,
        "requested_selector_gate": build_selector_gate_request_from_base(
            args,
            baseline_id=str(resolved_comparison_base.get("baseline_id") or ""),
            base_eval_run_id=str(resolved_comparison_base.get("base_eval_run_id") or ""),
        ),
        "status": "eval_incomplete",
        "passed": False,
    }

    if str(final_eval_run.get("status") or "").strip().lower() != "completed":
        report["status"] = "eval_failed"
        return report

    selector_gate = client.evaluate_selector_gate(str(final_eval_run.get("id") or ""), report["requested_selector_gate"])
    comparison_report = fetch_comparison_report(
        client=client,
        comparison_report_id=str(selector_gate.get("comparison_report_id") or ""),
        target_eval_run_id=str(final_eval_run.get("id") or ""),
        compare_payload=report["requested_selector_gate"],
    )

    report["selector_gate"] = selector_gate
    report["comparison_report"] = comparison_report
    report["passed"] = bool(selector_gate.get("passed"))
    report["status"] = "passed" if report["passed"] else "failed"
    return report


def build_markdown_report(report: Dict[str, Any]) -> str:
    eval_run = report.get("eval_run") or {}
    selector_gate = report.get("selector_gate") or {}
    metrics = selector_gate.get("metrics") if isinstance(selector_gate, dict) else {}
    thresholds = selector_gate.get("thresholds") if isinstance(selector_gate, dict) else {}
    comparison = report.get("comparison_report") or {}
    summary = comparison.get("summary") if isinstance(comparison, dict) else {}

    status = str(report.get("status") or "").upper()
    lines: List[str] = [
        "# Selector Gate Report",
        "",
        f"- Generated At: {report.get('generated_at', '-')}",
        f"- Status: {status}",
        f"- Passed: {'yes' if report.get('passed') else 'no'}",
        f"- Candidate ID: {report.get('candidate_id', '-')}",
        f"- Candidate: {report.get('candidate_label', '-')}",
        f"- Owner: {report.get('owner_user_id', '-')}",
        f"- Eval Run: {eval_run.get('id', '-')}",
        f"- Eval Spec: {eval_run.get('eval_spec_id', '-')}",
        f"- Group: {eval_run.get('group_id', '-')}",
        f"- Eval Status: {eval_run.get('status', '-')}",
    ]

    resolved_base = report.get("resolved_comparison_base") or {}
    if resolved_base:
        lines.extend(
            [
                f"- Comparison base resolution: {resolved_base.get('resolution', '-')}",
                f"- Resolved baseline: {resolved_base.get('baseline_id', '-')}",
                f"- Resolved base eval run: {resolved_base.get('base_eval_run_id', '-')}",
            ]
        )
    missing_base = report.get("missing_baseline") or {}
    if missing_base:
        lines.extend(
            [
                f"- Missing baseline eval spec: {missing_base.get('eval_spec_id', '-')}",
                f"- Existing baselines: {missing_base.get('baseline_count', '-')}",
                f"- Reusable completed/partial eval runs: {missing_base.get('eligible_eval_run_count', '-')}",
            ]
        )

    if selector_gate:
        lines.extend(
            [
                "",
                "## Gate Metrics",
                "",
                f"- Curated pass rate: {format_ratio(metrics.get('passed_count'), metrics.get('case_count'), metrics.get('pass_rate'))}",
                f"- Critical pass rate: {format_ratio(metrics.get('critical_passed_count'), metrics.get('critical_case_count'), metrics.get('critical_pass_rate'))}",
                f"- Route agreement: {format_ratio(metrics.get('route_agreement_count'), metrics.get('route_case_count'), metrics.get('route_agreement_rate'))}",
                f"- Route compatibility: {format_ratio(metrics.get('route_compatible_count'), metrics.get('route_case_count'), metrics.get('route_compatible_rate'))}",
                f"- Route truth improvements: {metrics.get('route_improvement_count', '-')}",
                f"- Clarify rate delta: {format_decimal(metrics.get('clarify_rate_delta'))}",
                f"- Critical regressions: {metrics.get('critical_regression_count', '-')}",
            ]
        )
        append_breakdown_section(lines, "Locale Drift Alerts", metrics.get("locale_breakdown"))
        append_breakdown_section(lines, "Primary Route Drift Alerts", metrics.get("primary_route_breakdown"))
        if thresholds:
            lines.extend(
                [
                    "",
                    "## Applied Thresholds",
                    "",
                ]
            )
            for key in sorted(thresholds):
                lines.append(f"- {key}: {thresholds[key]}")

        failed_checks = [
            str(check.get("name") or "unnamed_check")
            for check in selector_gate.get("checks", [])
            if isinstance(check, dict) and not bool(check.get("passed"))
        ]
        if failed_checks:
            lines.extend(
                [
                    "",
                    "## Failed Checks",
                    "",
                ]
            )
            lines.extend(f"- {name}" for name in failed_checks)

    if comparison:
        lines.extend(
            [
                "",
                "## Comparison Summary",
                "",
                f"- Comparison report: {comparison.get('id', '-')}",
                f"- Baseline: {comparison.get('baseline_id', '-')}",
                f"- Base eval run: {comparison.get('base_eval_run_id', '-')}",
                f"- Regression count: {summary.get('regression_count', '-')}",
                f"- Improvement count: {summary.get('improvement_count', '-')}",
                f"- Changed case count: {summary.get('changed_case_count', '-')}",
                f"- Route agreement rate: {format_decimal(summary.get('route_agreement_rate'))}",
                f"- Route compatibility rate: {format_decimal(summary.get('route_compatible_rate'))}",
                f"- Route truth improvement count: {summary.get('route_improvement_count', '-')}",
                f"- Clarify rate delta: {format_decimal(summary.get('clarify_rate_delta'))}",
            ]
        )
        append_case_section(lines, "Top Regressions", comparison.get("regressions") or [])
        append_case_section(lines, "Top Improvements", comparison.get("improvements") or [])

    return "\n".join(lines) + "\n"


def append_case_section(lines: List[str], title: str, cases: List[Dict[str, Any]]) -> None:
    if not cases:
        return
    lines.extend(["", f"## {title}", ""])
    for case in cases[:5]:
        key = str(case.get("key") or "unknown-case")
        base_verdict = str(case.get("base_verdict") or "-")
        target_verdict = str(case.get("target_verdict") or "-")
        lines.append(f"- {key}: {base_verdict} -> {target_verdict}")


def append_breakdown_section(lines: List[str], title: str, raw_breakdown: Any) -> None:
    rows = select_breakdown_alert_rows(raw_breakdown)
    if not rows:
        return
    lines.extend(["", f"## {title}", ""])
    for key, metrics in rows[:5]:
        lines.append(
            "- "
            + f"{key}: "
            + f"compatible={format_ratio(metrics.get('route_compatible_count'), metrics.get('case_count'), metrics.get('route_compatible_rate'))} | "
            + f"agreement={format_ratio(metrics.get('route_agreement_count'), metrics.get('case_count'), metrics.get('route_agreement_rate'))} | "
            + f"critical_regressions={metrics.get('critical_regression_count', 0)} | "
            + f"clarify_delta={format_decimal(metrics.get('clarify_rate_delta'))}"
        )


def select_breakdown_alert_rows(raw_breakdown: Any) -> List[tuple[str, Dict[str, Any]]]:
    if not isinstance(raw_breakdown, dict):
        return []

    rows: List[tuple[str, Dict[str, Any]]] = []
    for key, value in raw_breakdown.items():
        if not isinstance(value, dict):
            continue
        critical_regressions = value.get("critical_regression_count", 0)
        compatible_rate = value.get("route_compatible_rate")
        clarify_delta = value.get("clarify_rate_delta")
        needs_attention = (
            (isinstance(critical_regressions, (int, float)) and float(critical_regressions) > 0)
            or (isinstance(compatible_rate, (int, float)) and float(compatible_rate) < 1.0)
            or (isinstance(clarify_delta, (int, float)) and float(clarify_delta) != 0.0)
        )
        if needs_attention:
            rows.append((str(key), value))

    rows.sort(
        key=lambda item: (
            -float(item[1].get("critical_regression_count", 0) or 0),
            float(item[1].get("route_compatible_rate", 1.0) or 1.0),
            -float(item[1].get("route_disagreement_count", 0) or 0),
            item[0],
        )
    )
    return rows


def format_ratio(numerator: Any, denominator: Any, rate: Any) -> str:
    return f"{numerator or 0}/{denominator or 0} ({format_decimal(rate)})"


def format_decimal(value: Any) -> str:
    if isinstance(value, (int, float)):
        text = f"{float(value):.6f}".rstrip("0").rstrip(".")
        return text if text else "0"
    return "-"


def write_report_outputs(report: Dict[str, Any], output_json: Path, output_md: Path) -> None:
    output_json.parent.mkdir(parents=True, exist_ok=True)
    output_md.parent.mkdir(parents=True, exist_ok=True)
    output_json.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    output_md.write_text(build_markdown_report(report), encoding="utf-8")


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(levelname)s %(message)s",
    )

    client = HarnessClient(args.blue_base_url, api_key=args.api_key, bearer_token=args.bearer_token)
    try:
        report = run_selector_gate_workflow(client, args)
    except MissingComparisonBaseError as exc:
        LOG.error("selector gate workflow missing comparison base: %s", exc)
        report = {
            "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            "owner_user_id": str(args.owner or "").strip() or "release-gate",
            "candidate_id": resolve_candidate_id(
                getattr(args, "candidate_id", ""),
                resolve_candidate_label(getattr(args, "candidate_label", ""), resolve_eval_title(getattr(args, "title", ""))),
            ),
            "candidate_label": resolve_candidate_label(getattr(args, "candidate_label", ""), resolve_eval_title(getattr(args, "title", ""))),
            "status": "missing_baseline",
            "passed": False,
            "error": str(exc),
            "missing_baseline": exc.details,
        }
    except Exception as exc:
        LOG.error("selector gate workflow failed: %s", exc)
        report = {
            "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
            "owner_user_id": str(args.owner or "").strip() or "release-gate",
            "candidate_id": resolve_candidate_id(
                getattr(args, "candidate_id", ""),
                resolve_candidate_label(getattr(args, "candidate_label", ""), resolve_eval_title(getattr(args, "title", ""))),
            ),
            "candidate_label": resolve_candidate_label(getattr(args, "candidate_label", ""), resolve_eval_title(getattr(args, "title", ""))),
            "status": "error",
            "passed": False,
            "error": str(exc),
        }

    write_report_outputs(report, Path(args.output_json), Path(args.output_md))
    if report.get("passed"):
        LOG.info("selector gate passed")
        return 0
    LOG.error("selector gate did not pass; see %s and %s", args.output_json, args.output_md)
    return 1


if __name__ == "__main__":
    sys.exit(main())
