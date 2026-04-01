import sys
import unittest
from pathlib import Path
from types import SimpleNamespace


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import selector_gate_runner as runner


class StubHarnessClient:
    def __init__(self):
        self.ensure_calls = []
        self.created_runs = []
        self.eval_run_polls = 0
        self.list_eval_runs_calls = []
        self.list_baselines_calls = []
        self.created_baselines = []
        self.selector_gate_calls = []
        self.comparison_fetches = []
        self.compare_calls = []

    def ensure_selector_curated_assets(self, owner_user_id: str):
        self.ensure_calls.append(owner_user_id)
        return {
            "dataset": {"id": "dataset-1"},
            "dataset_version": {"id": "version-1", "version": "v1"},
            "eval_spec": {"id": "eval-spec-1", "name": "Selector Curated Dry Run"},
        }

    def create_eval_run(self, eval_spec_id: str, owner_user_id: str, title: str, base_eval_run_id: str, metadata=None):
        self.created_runs.append(
            {
                "eval_spec_id": eval_spec_id,
                "owner_user_id": owner_user_id,
                "title": title,
                "base_eval_run_id": base_eval_run_id,
                "metadata": metadata or {},
            }
        )
        return {"id": "eval-run-1", "status": "queued", "eval_spec_id": eval_spec_id}

    def get_eval_run(self, eval_run_id: str):
        self.eval_run_polls += 1
        status = "queued"
        if self.eval_run_polls >= 2:
            status = "completed"
        return {
            "id": eval_run_id,
            "status": status,
            "eval_spec_id": "eval-spec-1",
            "group_id": "group-1",
        }

    def list_eval_runs(self, *, eval_spec_id="", limit=50, statuses=None):
        self.list_eval_runs_calls.append(
            {
                "eval_spec_id": eval_spec_id,
                "limit": limit,
                "statuses": list(statuses or []),
            }
        )
        return [
            {
                "id": "historical-run-1",
                "status": "completed",
                "eval_spec_id": eval_spec_id or "eval-spec-1",
                "metadata": {"candidate_id": "baseline-candidate"},
            }
        ]

    def list_baselines(self, *, eval_spec_id="", limit=50):
        self.list_baselines_calls.append({"eval_spec_id": eval_spec_id, "limit": limit})
        return [
            {
                "id": "baseline-1",
                "eval_run_id": "base-run-1",
                "eval_spec_id": eval_spec_id or "eval-spec-1",
                "is_default": True,
            }
        ]

    def create_baseline(self, *, name, eval_run_id, eval_spec_id="", metadata=None, is_default=False):
        baseline = {
            "id": "auto-baseline-1",
            "name": name,
            "eval_run_id": eval_run_id,
            "eval_spec_id": eval_spec_id or "eval-spec-1",
            "is_default": bool(is_default),
            "metadata": metadata or {},
        }
        self.created_baselines.append(baseline)
        return baseline

    def evaluate_selector_gate(self, eval_run_id: str, payload):
        self.selector_gate_calls.append((eval_run_id, payload))
        return {
            "target_eval_run_id": eval_run_id,
            "comparison_report_id": "comparison-1",
            "passed": True,
            "metrics": {
                "passed_count": 119,
                "case_count": 120,
                "pass_rate": 0.9916666667,
                "critical_passed_count": 10,
                "critical_case_count": 10,
                "critical_pass_rate": 1.0,
                "route_agreement_count": 118,
                "route_case_count": 120,
                "route_agreement_rate": 0.9833333333,
                "route_compatible_count": 119,
                "route_compatible_rate": 0.9916666667,
                "route_improvement_count": 1,
                "clarify_rate_delta": 0.0,
                "critical_regression_count": 0,
                "locale_breakdown": {
                    "zh-CN": {
                        "case_count": 5,
                        "route_agreement_count": 4,
                        "route_agreement_rate": 0.8,
                        "route_compatible_count": 5,
                        "route_compatible_rate": 1.0,
                        "route_improvement_count": 1,
                        "route_disagreement_count": 1,
                        "critical_regression_count": 0,
                        "clarify_rate_delta": 0.0,
                    }
                },
            },
            "thresholds": {
                "min_pass_rate": 0.98,
                "min_critical_pass_rate": 1.0,
            },
        }

    def get_comparison_report(self, comparison_report_id: str):
        self.comparison_fetches.append(comparison_report_id)
        return {
            "id": comparison_report_id,
            "baseline_id": "baseline-1",
            "base_eval_run_id": "base-run-1",
            "summary": {
                "regression_count": 0,
                "improvement_count": 2,
                "changed_case_count": 2,
                "route_agreement_rate": 0.9833333333,
                "route_compatible_rate": 0.9916666667,
                "route_improvement_count": 1,
                "clarify_rate_delta": 0.0,
            },
            "regressions": [],
            "improvements": [
                {"key": "selected-web_search-en-us", "base_verdict": "fail", "target_verdict": "pass"},
            ],
        }

    def compare_eval_run(self, eval_run_id: str, payload):
        self.compare_calls.append((eval_run_id, payload))
        return {
            "id": "comparison-fallback",
            "base_eval_run_id": payload.get("base_eval_run_id", ""),
            "target_eval_run_id": eval_run_id,
            "summary": {},
        }


class StubFallbackHarnessClient(StubHarnessClient):
    def get_comparison_report(self, comparison_report_id: str):
        raise runner.HarnessAPIError(f"GET /comparison-reports/{comparison_report_id} failed: HTTP 404: not found")


class SelectorGateRunnerTest(unittest.TestCase):
    def make_args(self, **overrides):
        base = {
            "owner": "release-gate",
            "eval_spec_id": "",
            "title": "candidate-20260328",
            "candidate_label": "batch-1",
            "candidate_id": "candidate-1",
            "baseline_id": "baseline-1",
            "base_eval_run_id": "base-run-1",
            "poll_interval_seconds": 0.01,
            "timeout_seconds": 1.0,
            "min_pass_rate": 0.98,
            "min_critical_pass_rate": 1.0,
            "min_route_agreement_rate": 0.95,
            "min_route_compatible_rate": 0.95,
            "max_clarify_rate_delta": 0.01,
            "max_critical_regressions": 0,
        }
        base.update(overrides)
        return SimpleNamespace(**base)

    def test_normalize_harness_base_url_accepts_blue_api_root(self):
        self.assertEqual(
            runner.normalize_harness_base_url("http://127.0.0.1:18080/api/v1"),
            "http://127.0.0.1:18080/api/v1/harness",
        )
        self.assertEqual(
            runner.normalize_harness_base_url("http://127.0.0.1:18080/api/v1/harness"),
            "http://127.0.0.1:18080/api/v1/harness",
        )

    def test_build_selector_gate_request_keeps_only_explicit_thresholds(self):
        payload = runner.build_selector_gate_request(
            self.make_args(
                baseline_id="baseline-1",
                base_eval_run_id="",
                min_route_agreement_rate=None,
                min_route_compatible_rate=0.95,
                max_critical_regressions=None,
            )
        )

        self.assertEqual(payload["baseline_id"], "baseline-1")
        self.assertNotIn("base_eval_run_id", payload)
        self.assertEqual(
            payload["thresholds"],
            {
                "min_pass_rate": 0.98,
                "min_critical_pass_rate": 1.0,
                "min_route_compatible_rate": 0.95,
                "max_clarify_rate_delta": 0.01,
            },
        )

    def test_resolve_candidate_label_prefers_explicit_value(self):
        self.assertEqual(
            runner.resolve_candidate_label("candidate-a", "selector-gate-20260328"),
            "candidate-a",
        )
        self.assertEqual(
            runner.resolve_candidate_label("", "selector-gate-20260328"),
            "selector-gate-20260328",
        )

    def test_resolve_candidate_id_prefers_explicit_value(self):
        self.assertEqual(runner.resolve_candidate_id("candidate-id-a", "candidate-a"), "candidate-id-a")
        self.assertEqual(runner.resolve_candidate_id("", "candidate-a"), "candidate-a")

    def test_run_selector_gate_workflow_ensures_polls_and_fetches_report(self):
        client = StubHarnessClient()
        report = runner.run_selector_gate_workflow(client, self.make_args())

        self.assertEqual(report["status"], "passed")
        self.assertTrue(report["passed"])
        self.assertEqual(report["candidate_id"], "candidate-1")
        self.assertEqual(report["candidate_label"], "batch-1")
        self.assertEqual(client.ensure_calls, ["release-gate"])
        self.assertEqual(len(client.created_runs), 1)
        self.assertGreaterEqual(client.eval_run_polls, 2)
        self.assertEqual(client.comparison_fetches, ["comparison-1"])
        self.assertEqual(client.compare_calls, [])
        self.assertEqual(client.list_baselines_calls, [{"eval_spec_id": "eval-spec-1", "limit": 100}])
        self.assertEqual(client.list_eval_runs_calls, [])
        self.assertEqual(client.selector_gate_calls[0][0], "eval-run-1")
        self.assertEqual(client.created_runs[0]["metadata"]["candidate_id"], "candidate-1")
        self.assertEqual(client.created_runs[0]["metadata"]["candidate_label"], "batch-1")
        self.assertEqual(client.created_runs[0]["base_eval_run_id"], "base-run-1")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "explicit_baseline")
        self.assertEqual(
            client.selector_gate_calls[0][1]["thresholds"]["min_route_compatible_rate"],
            0.95,
        )

    def test_run_selector_gate_workflow_falls_back_to_compare_endpoint(self):
        client = StubFallbackHarnessClient()
        report = runner.run_selector_gate_workflow(client, self.make_args())

        self.assertEqual(report["status"], "passed")
        self.assertEqual(len(client.compare_calls), 1)
        self.assertEqual(client.compare_calls[0][0], "eval-run-1")
        self.assertEqual(client.compare_calls[0][1]["baseline_id"], "baseline-1")

    def test_run_selector_gate_workflow_auto_creates_baseline_when_missing(self):
        class StubAutoBaselineHarnessClient(StubHarnessClient):
            def list_baselines(self, *, eval_spec_id="", limit=50):
                self.list_baselines_calls.append({"eval_spec_id": eval_spec_id, "limit": limit})
                return []

            def list_eval_runs(self, *, eval_spec_id="", limit=50, statuses=None):
                self.list_eval_runs_calls.append(
                    {
                        "eval_spec_id": eval_spec_id,
                        "limit": limit,
                        "statuses": list(statuses or []),
                    }
                )
                return [
                    {
                        "id": "historical-run-same-candidate",
                        "status": "completed",
                        "eval_spec_id": eval_spec_id or "eval-spec-1",
                        "metadata": {"candidate_id": "candidate-1"},
                    },
                    {
                        "id": "historical-run-2",
                        "status": "completed",
                        "eval_spec_id": eval_spec_id or "eval-spec-1",
                        "metadata": {"candidate_id": "stable-main"},
                    },
                ]

        client = StubAutoBaselineHarnessClient()
        report = runner.run_selector_gate_workflow(client, self.make_args(baseline_id="", base_eval_run_id=""))

        self.assertEqual(report["status"], "passed")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "auto_created_baseline")
        self.assertEqual(report["resolved_comparison_base"]["baseline_id"], "auto-baseline-1")
        self.assertEqual(report["resolved_comparison_base"]["base_eval_run_id"], "historical-run-2")
        self.assertEqual(len(client.created_baselines), 1)
        self.assertEqual(client.created_baselines[0]["eval_run_id"], "historical-run-2")
        self.assertEqual(client.created_runs[0]["base_eval_run_id"], "historical-run-2")
        self.assertEqual(client.selector_gate_calls[0][1]["baseline_id"], "auto-baseline-1")

    def test_resolve_selector_comparison_base_raises_missing_baseline_with_details(self):
        class StubNoBaselineHarnessClient(StubHarnessClient):
            def list_baselines(self, *, eval_spec_id="", limit=50):
                self.list_baselines_calls.append({"eval_spec_id": eval_spec_id, "limit": limit})
                return []

            def list_eval_runs(self, *, eval_spec_id="", limit=50, statuses=None):
                self.list_eval_runs_calls.append(
                    {
                        "eval_spec_id": eval_spec_id,
                        "limit": limit,
                        "statuses": list(statuses or []),
                    }
                )
                return []

        client = StubNoBaselineHarnessClient()

        with self.assertRaises(runner.MissingComparisonBaseError) as exc:
            runner.resolve_selector_comparison_base(
                client,
                owner_user_id="release-gate",
                eval_spec_id="eval-spec-1",
                candidate_id="candidate-1",
                requested_baseline_id="",
                requested_base_eval_run_id="",
            )

        self.assertEqual(exc.exception.details["eval_spec_id"], "eval-spec-1")
        self.assertEqual(exc.exception.details["baseline_count"], 0)
        self.assertEqual(exc.exception.details["eligible_eval_run_count"], 0)

    def test_build_markdown_report_includes_failed_checks(self):
        markdown = runner.build_markdown_report(
            {
                "generated_at": "2026-03-28T00:00:00Z",
                "owner_user_id": "release-gate",
                "status": "failed",
                "passed": False,
                "eval_run": {"id": "eval-run-1", "status": "completed", "eval_spec_id": "eval-spec-1", "group_id": "group-1"},
                "resolved_comparison_base": {
                    "resolution": "existing_default_baseline",
                    "baseline_id": "baseline-1",
                    "base_eval_run_id": "base-run-1",
                },
                "selector_gate": {
                    "metrics": {
                        "passed_count": 117,
                        "case_count": 120,
                        "pass_rate": 0.975,
                        "critical_passed_count": 9,
                        "critical_case_count": 10,
                        "critical_pass_rate": 0.9,
                        "route_agreement_count": 116,
                        "route_case_count": 120,
                        "route_agreement_rate": 0.9667,
                        "route_compatible_count": 118,
                        "route_compatible_rate": 0.9833,
                        "route_improvement_count": 2,
                        "clarify_rate_delta": 0.02,
                        "critical_regression_count": 1,
                        "locale_breakdown": {
                            "zh-CN": {
                                "case_count": 3,
                                "route_agreement_count": 1,
                                "route_agreement_rate": 0.3333,
                                "route_compatible_count": 2,
                                "route_compatible_rate": 0.6667,
                                "route_disagreement_count": 2,
                                "critical_regression_count": 1,
                                "clarify_rate_delta": -0.3333,
                            }
                        },
                        "primary_route_breakdown": {
                            "analyze": {
                                "case_count": 4,
                                "route_agreement_count": 2,
                                "route_agreement_rate": 0.5,
                                "route_compatible_count": 2,
                                "route_compatible_rate": 0.5,
                                "route_disagreement_count": 2,
                                "critical_regression_count": 1,
                                "clarify_rate_delta": -0.25,
                            }
                        },
                    },
                    "thresholds": {"min_pass_rate": 0.98},
                    "checks": [
                        {"name": "pass_rate", "passed": False},
                        {"name": "critical_pass_rate", "passed": False},
                    ],
                },
                "comparison_report": {
                    "id": "comparison-1",
                    "baseline_id": "baseline-1",
                    "base_eval_run_id": "base-run-1",
                    "summary": {
                        "regression_count": 2,
                        "improvement_count": 0,
                        "changed_case_count": 2,
                        "route_compatible_rate": 0.95,
                        "route_improvement_count": 1,
                    },
                    "regressions": [
                        {"key": "clarify-mixed-local-web-zh-cn", "base_verdict": "pass", "target_verdict": "fail"},
                    ],
                    "improvements": [],
                },
            }
        )

        self.assertIn("## Failed Checks", markdown)
        self.assertIn("Comparison base resolution: existing_default_baseline", markdown)
        self.assertIn("- pass_rate", markdown)
        self.assertIn("clarify-mixed-local-web-zh-cn: pass -> fail", markdown)
        self.assertIn("Route compatibility", markdown)
        self.assertIn("## Locale Drift Alerts", markdown)
        self.assertIn("zh-CN: compatible=2/3 (0.6667)", markdown)
        self.assertIn("## Primary Route Drift Alerts", markdown)
        self.assertIn("analyze: compatible=2/4 (0.5)", markdown)

    def test_build_markdown_report_includes_missing_baseline_details(self):
        markdown = runner.build_markdown_report(
            {
                "generated_at": "2026-03-29T00:00:00Z",
                "owner_user_id": "release-gate",
                "candidate_id": "candidate-1",
                "candidate_label": "candidate-1",
                "status": "missing_baseline",
                "passed": False,
                "missing_baseline": {
                    "eval_spec_id": "eval-spec-1",
                    "baseline_count": 0,
                    "eligible_eval_run_count": 0,
                },
            }
        )

        self.assertIn("Status: MISSING_BASELINE", markdown)
        self.assertIn("Missing baseline eval spec: eval-spec-1", markdown)
        self.assertIn("Existing baselines: 0", markdown)
        self.assertIn("Reusable completed/partial eval runs: 0", markdown)

    def test_select_breakdown_alert_rows_filters_clean_entries(self):
        rows = runner.select_breakdown_alert_rows(
            {
                "en-US": {
                    "case_count": 10,
                    "route_compatible_count": 10,
                    "route_compatible_rate": 1.0,
                    "route_agreement_count": 10,
                    "route_agreement_rate": 1.0,
                    "critical_regression_count": 0,
                    "clarify_rate_delta": 0.0,
                },
                "zh-CN": {
                    "case_count": 5,
                    "route_compatible_count": 4,
                    "route_compatible_rate": 0.8,
                    "route_agreement_count": 3,
                    "route_agreement_rate": 0.6,
                    "critical_regression_count": 1,
                    "clarify_rate_delta": -0.2,
                },
            }
        )

        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0][0], "zh-CN")


if __name__ == "__main__":
    unittest.main()
