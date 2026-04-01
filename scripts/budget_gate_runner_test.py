import sys
import unittest
from pathlib import Path
from types import SimpleNamespace


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import budget_gate_runner as runner


class StubHarnessClient:
    def __init__(self):
        self.ensure_calls = []
        self.created_runs = []
        self.eval_run_polls = 0
        self.list_eval_runs_calls = []
        self.list_baselines_calls = []
        self.created_baselines = []
        self.budget_gate_calls = []

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

    def evaluate_budget_gate(self, eval_run_id: str, payload):
        self.budget_gate_calls.append((eval_run_id, payload))
        return {
            "target_eval_run_id": eval_run_id,
            "base_eval_run_id": "base-run-1",
            "baseline_id": "baseline-1",
            "passed": True,
            "metrics": {
                "case_count": 120,
                "comparable_case_count": 120,
                "missing_surface_case_count": 0,
                "base_median_tool_count": 5.0,
                "target_median_tool_count": 1.0,
                "base_median_schema_bytes": 4000.0,
                "target_median_schema_bytes": 600.0,
                "median_schema_byte_reduction_rate": 0.85,
                "base_median_latency_ms": 1000.0,
                "target_median_latency_ms": 1050.0,
                "median_latency_increase_rate": 0.05,
                "allowed_final_native_tools": ["exec"],
                "allowed_final_native_tool_cases": 120,
                "allowed_final_native_tool_case_rate": 1.0,
                "non_allowed_native_tool_case_count": 0,
            },
            "thresholds": {
                "min_median_schema_byte_reduction_rate": 0.8,
                "max_median_latency_increase_rate": 0.1,
                "allowed_final_native_tools": ["exec"],
            },
        }


class BudgetGateRunnerTest(unittest.TestCase):
    def make_args(self, **overrides):
        base = {
            "owner": "release-gate",
            "eval_spec_id": "",
            "eval_run_id": "",
            "title": "candidate-20260328",
            "candidate_label": "batch-1",
            "candidate_id": "candidate-1",
            "baseline_id": "baseline-1",
            "base_eval_run_id": "base-run-1",
            "poll_interval_seconds": 0.01,
            "timeout_seconds": 1.0,
            "min_median_schema_byte_reduction_rate": 0.8,
            "max_median_latency_increase_rate": 0.1,
            "allowed_final_native_tools": "exec",
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

    def test_build_budget_gate_request_keeps_only_explicit_thresholds(self):
        payload = runner.build_budget_gate_request(
            self.make_args(
                baseline_id="baseline-1",
                base_eval_run_id="",
                max_median_latency_increase_rate=None,
                allowed_final_native_tools="exec, exec, browser",
            )
        )

        self.assertEqual(payload["baseline_id"], "baseline-1")
        self.assertNotIn("base_eval_run_id", payload)
        self.assertEqual(
            payload["thresholds"],
            {
                "min_median_schema_byte_reduction_rate": 0.8,
                "allowed_final_native_tools": ["exec", "browser"],
            },
        )

    def test_resolve_candidate_label_prefers_explicit_value(self):
        self.assertEqual(
            runner.resolve_candidate_label("candidate-a", "budget-gate-20260328"),
            "candidate-a",
        )
        self.assertEqual(
            runner.resolve_candidate_label("", "budget-gate-20260328"),
            "budget-gate-20260328",
        )

    def test_resolve_candidate_id_prefers_explicit_value(self):
        self.assertEqual(runner.resolve_candidate_id("candidate-id-a", "candidate-a"), "candidate-id-a")
        self.assertEqual(runner.resolve_candidate_id("", "candidate-a"), "candidate-a")

    def test_run_budget_gate_workflow_ensures_polls_and_evaluates_gate(self):
        client = StubHarnessClient()
        report = runner.run_budget_gate_workflow(client, self.make_args())

        self.assertEqual(report["status"], "passed")
        self.assertTrue(report["passed"])
        self.assertEqual(report["candidate_id"], "candidate-1")
        self.assertEqual(report["candidate_label"], "batch-1")
        self.assertEqual(client.ensure_calls, ["release-gate"])
        self.assertEqual(len(client.created_runs), 1)
        self.assertGreaterEqual(client.eval_run_polls, 2)
        self.assertEqual(client.list_baselines_calls, [{"eval_spec_id": "eval-spec-1", "limit": 100}])
        self.assertEqual(client.list_eval_runs_calls, [])
        self.assertEqual(client.budget_gate_calls[0][0], "eval-run-1")
        self.assertEqual(client.created_runs[0]["metadata"]["candidate_id"], "candidate-1")
        self.assertEqual(client.created_runs[0]["metadata"]["candidate_label"], "batch-1")
        self.assertEqual(client.created_runs[0]["base_eval_run_id"], "base-run-1")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "explicit_baseline")
        self.assertEqual(
            client.budget_gate_calls[0][1]["thresholds"]["allowed_final_native_tools"],
            ["exec"],
        )

    def test_run_budget_gate_workflow_reuses_existing_eval_run(self):
        client = StubHarnessClient()
        report = runner.run_budget_gate_workflow(client, self.make_args(eval_run_id="eval-run-existing"))

        self.assertEqual(report["status"], "passed")
        self.assertTrue(report["reused_eval_run"])
        self.assertEqual(client.ensure_calls, [])
        self.assertEqual(client.created_runs, [])
        self.assertEqual(client.budget_gate_calls[0][0], "eval-run-existing")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "explicit_baseline")

    def test_run_budget_gate_workflow_auto_creates_baseline_when_missing(self):
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
        report = runner.run_budget_gate_workflow(client, self.make_args(baseline_id="", base_eval_run_id=""))

        self.assertEqual(report["status"], "passed")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "auto_created_baseline")
        self.assertEqual(report["resolved_comparison_base"]["baseline_id"], "auto-baseline-1")
        self.assertEqual(report["resolved_comparison_base"]["base_eval_run_id"], "historical-run-2")
        self.assertEqual(len(client.created_baselines), 1)
        self.assertEqual(client.created_baselines[0]["eval_run_id"], "historical-run-2")
        self.assertEqual(client.created_runs[0]["base_eval_run_id"], "historical-run-2")
        self.assertEqual(client.budget_gate_calls[0][1]["baseline_id"], "auto-baseline-1")

    def test_resolve_budget_comparison_base_raises_missing_baseline_with_details(self):
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
            runner.resolve_budget_comparison_base(
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
                "budget_gate": {
                    "metrics": {
                        "case_count": 120,
                        "comparable_case_count": 118,
                        "missing_surface_case_count": 2,
                        "base_median_tool_count": 5.0,
                        "target_median_tool_count": 2.0,
                        "base_median_schema_bytes": 4000.0,
                        "target_median_schema_bytes": 2400.0,
                        "median_schema_byte_reduction_rate": 0.4,
                        "base_median_latency_ms": 1000.0,
                        "target_median_latency_ms": 1200.0,
                        "median_latency_increase_rate": 0.2,
                        "allowed_final_native_tool_cases": 110,
                        "allowed_final_native_tool_case_rate": 0.9167,
                        "non_allowed_native_tool_case_count": 10,
                    },
                    "thresholds": {
                        "min_median_schema_byte_reduction_rate": 0.8,
                        "max_median_latency_increase_rate": 0.1,
                        "allowed_final_native_tools": ["exec"],
                    },
                    "checks": [
                        {"name": "surface_metrics_coverage", "passed": False},
                        {"name": "median_latency_increase_rate", "passed": False},
                        {"name": "final_native_tool_surface", "passed": False},
                    ],
                },
            }
        )

        self.assertIn("# Budget Gate Report", markdown)
        self.assertIn("Comparison base resolution: existing_default_baseline", markdown)
        self.assertIn("Comparable cases: 118/120", markdown)
        self.assertIn("Median schema-byte reduction: 0.4", markdown)
        self.assertIn("Median latency increase: 0.2", markdown)
        self.assertIn("Allowed final native tool cases: 110/120 (0.9167)", markdown)
        self.assertIn("## Failed Checks", markdown)
        self.assertIn("- surface_metrics_coverage", markdown)
        self.assertIn("- final_native_tool_surface", markdown)

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


if __name__ == "__main__":
    unittest.main()
