import sys
import unittest
from pathlib import Path
from types import SimpleNamespace


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import execution_gate_runner as runner


class StubHarnessClient:
    def __init__(self):
        self.ensure_calls = []
        self.created_runs = []
        self.eval_run_polls = 0
        self.list_eval_runs_calls = []
        self.list_baselines_calls = []
        self.created_baselines = []
        self.patch_settings_calls = []
        self.execution_gate_calls = []
        self.comparison_fetches = []
        self.compare_calls = []

    def ensure_batch1_execution_assets(self, owner_user_id: str):
        self.ensure_calls.append(owner_user_id)
        return {
            "dataset": {"id": "dataset-1"},
            "dataset_version": {"id": "version-1", "version": "v1"},
            "eval_spec": {"id": "eval-spec-1", "name": "Skill Execution Batch 1"},
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

    def patch_settings(self, payload):
        self.patch_settings_calls.append(payload)
        return payload

    def evaluate_execution_gate(self, eval_run_id: str, payload):
        self.execution_gate_calls.append((eval_run_id, payload))
        return {
            "target_eval_run_id": eval_run_id,
            "comparison_report_id": "comparison-1",
            "passed": True,
            "metrics": {
                "passed_count": 84,
                "case_count": 84,
                "pass_rate": 1.0,
                "critical_passed_count": 4,
                "critical_case_count": 4,
                "critical_pass_rate": 1.0,
                "base_verification_pass_rate": 1.0,
                "target_verification_pass_rate": 1.0,
                "verification_pass_rate_delta": 0.0,
                "base_evidence_backed_pass_rate": 0.95,
                "target_evidence_backed_pass_rate": 0.95,
                "evidence_backed_pass_rate_delta": 0.0,
                "critical_regression_count": 0,
                "new_failure_count": 0,
                "resolved_failure_count": 0,
                "locale_breakdown": {
                    "zh-CN": {
                        "case_count": 3,
                        "passed_count": 2,
                        "pass_rate": 0.6667,
                        "critical_case_count": 1,
                        "critical_passed_count": 1,
                        "critical_pass_rate": 1.0,
                        "regression_count": 1,
                        "new_failure_count": 1,
                        "critical_regression_count": 0,
                    }
                },
            },
            "thresholds": {
                "max_pass_rate_drop": 0.01,
                "max_critical_regression_count": 0,
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
                "improvement_count": 1,
                "changed_case_count": 1,
                "new_failure_count": 0,
                "resolved_failure_count": 1,
                "pass_rate_delta": 0.0,
                "verification_pass_rate_delta": 0.0,
                "evidence_backed_pass_rate_delta": 0.0,
            },
            "regressions": [],
            "improvements": [
                {
                    "key": "critical-web-search-latest-docs-zh-cn",
                    "base_verdict": "fail",
                    "target_verdict": "pass",
                    "target_verification": "passed",
                }
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


class ExecutionGateRunnerTest(unittest.TestCase):
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
            "approval_mode": "",
            "skip_hil": False,
            "max_pass_rate_drop": 0.01,
            "max_critical_regressions": 0,
            "max_verification_pass_rate_drop": 0.0,
            "max_evidence_backed_pass_rate_drop": 0.01,
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

    def test_normalize_api_base_url_accepts_blue_api_root_and_harness_root(self):
        self.assertEqual(
            runner.normalize_api_base_url("http://127.0.0.1:18080/api/v1"),
            "http://127.0.0.1:18080/api/v1",
        )
        self.assertEqual(
            runner.normalize_api_base_url("http://127.0.0.1:18080/api/v1/harness"),
            "http://127.0.0.1:18080/api/v1",
        )

    def test_build_execution_gate_request_keeps_only_explicit_thresholds(self):
        payload = runner.build_execution_gate_request(
            self.make_args(
                baseline_id="baseline-1",
                base_eval_run_id="",
                max_verification_pass_rate_drop=None,
                max_evidence_backed_pass_rate_drop=0.02,
            )
        )

        self.assertEqual(payload["baseline_id"], "baseline-1")
        self.assertNotIn("base_eval_run_id", payload)
        self.assertEqual(
            payload["thresholds"],
            {
                "max_pass_rate_drop": 0.01,
                "max_critical_regression_count": 0,
                "max_evidence_backed_pass_rate_drop": 0.02,
            },
        )

    def test_resolve_candidate_label_prefers_explicit_value(self):
        self.assertEqual(
            runner.resolve_candidate_label("candidate-a", "execution-gate-20260328"),
            "candidate-a",
        )
        self.assertEqual(
            runner.resolve_candidate_label("", "execution-gate-20260328"),
            "execution-gate-20260328",
        )

    def test_resolve_candidate_id_prefers_explicit_value(self):
        self.assertEqual(runner.resolve_candidate_id("candidate-id-a", "candidate-a"), "candidate-id-a")
        self.assertEqual(runner.resolve_candidate_id("", "candidate-a"), "candidate-a")

    def test_run_execution_gate_workflow_ensures_polls_and_fetches_report(self):
        client = StubHarnessClient()
        report = runner.run_execution_gate_workflow(client, self.make_args())

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
        self.assertEqual(client.execution_gate_calls[0][0], "eval-run-1")
        self.assertEqual(client.created_runs[0]["metadata"]["candidate_id"], "candidate-1")
        self.assertEqual(client.created_runs[0]["metadata"]["candidate_label"], "batch-1")
        self.assertEqual(client.created_runs[0]["base_eval_run_id"], "base-run-1")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "explicit_baseline")
        self.assertEqual(
            client.execution_gate_calls[0][1]["thresholds"]["max_verification_pass_rate_drop"],
            0.0,
        )
        self.assertEqual(client.patch_settings_calls, [])

    def test_run_execution_gate_workflow_falls_back_to_compare_endpoint(self):
        client = StubFallbackHarnessClient()
        report = runner.run_execution_gate_workflow(client, self.make_args())

        self.assertEqual(report["status"], "passed")
        self.assertEqual(len(client.compare_calls), 1)
        self.assertEqual(client.compare_calls[0][0], "eval-run-1")
        self.assertEqual(client.compare_calls[0][1]["baseline_id"], "baseline-1")

    def test_run_execution_gate_workflow_auto_creates_baseline_when_missing(self):
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
        report = runner.run_execution_gate_workflow(client, self.make_args(baseline_id="", base_eval_run_id=""))

        self.assertEqual(report["status"], "passed")
        self.assertEqual(report["resolved_comparison_base"]["resolution"], "auto_created_baseline")
        self.assertEqual(report["resolved_comparison_base"]["baseline_id"], "auto-baseline-1")
        self.assertEqual(report["resolved_comparison_base"]["base_eval_run_id"], "historical-run-2")
        self.assertEqual(len(client.created_baselines), 1)
        self.assertEqual(client.created_baselines[0]["eval_run_id"], "historical-run-2")
        self.assertEqual(client.created_runs[0]["base_eval_run_id"], "historical-run-2")
        self.assertEqual(client.execution_gate_calls[0][1]["baseline_id"], "auto-baseline-1")

    def test_resolve_execution_comparison_base_prefers_explicit_base_eval_run(self):
        client = StubHarnessClient()

        resolved = runner.resolve_execution_comparison_base(
            client,
            owner_user_id="release-gate",
            eval_spec_id="eval-spec-1",
            candidate_id="candidate-1",
            requested_baseline_id="",
            requested_base_eval_run_id="manual-base-run",
        )

        self.assertEqual(resolved["resolution"], "explicit_base_eval_run")
        self.assertEqual(resolved["base_eval_run_id"], "manual-base-run")
        self.assertEqual(client.list_baselines_calls, [{"eval_spec_id": "eval-spec-1", "limit": 100}])
        self.assertEqual(client.list_eval_runs_calls, [])

    def test_run_execution_gate_workflow_skip_hil_enables_unattended_mode(self):
        client = StubHarnessClient()
        report = runner.run_execution_gate_workflow(client, self.make_args(skip_hil=True))

        self.assertEqual(
            client.patch_settings_calls,
            [
                {
                    "agent_auto_confirm": True,
                    "agent_ask_timeout_seconds": 20,
                    "agent_ask_timeout_action": "default",
                }
            ],
        )
        self.assertEqual(client.created_runs[0]["metadata"]["approval_mode"], "allow")
        self.assertTrue(report["skip_hil"])
        self.assertEqual(report["skip_hil_settings"]["agent_auto_confirm"], True)

    def test_resolve_execution_comparison_base_raises_missing_baseline_with_details(self):
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
            runner.resolve_execution_comparison_base(
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
                "execution_gate": {
                    "metrics": {
                        "passed_count": 82,
                        "case_count": 84,
                        "pass_rate": 0.9762,
                        "critical_passed_count": 3,
                        "critical_case_count": 4,
                        "critical_pass_rate": 0.75,
                        "base_verification_pass_rate": 1.0,
                        "target_verification_pass_rate": 0.8,
                        "verification_pass_rate_delta": -0.2,
                        "base_evidence_backed_pass_rate": 0.95,
                        "target_evidence_backed_pass_rate": 0.8,
                        "evidence_backed_pass_rate_delta": -0.15,
                        "critical_regression_count": 1,
                        "new_failure_count": 2,
                        "resolved_failure_count": 0,
                        "locale_breakdown": {
                            "zh-CN": {
                                "case_count": 3,
                                "passed_count": 2,
                                "pass_rate": 0.6667,
                                "critical_case_count": 1,
                                "critical_passed_count": 1,
                                "critical_pass_rate": 1.0,
                                "regression_count": 1,
                                "new_failure_count": 1,
                                "critical_regression_count": 0,
                            }
                        },
                        "primary_route_breakdown": {
                            "analyze": {
                                "case_count": 4,
                                "passed_count": 2,
                                "pass_rate": 0.5,
                                "critical_case_count": 2,
                                "critical_passed_count": 1,
                                "critical_pass_rate": 0.5,
                                "regression_count": 2,
                                "new_failure_count": 1,
                                "critical_regression_count": 1,
                            }
                        },
                    },
                    "thresholds": {
                        "max_pass_rate_drop": 0.01,
                        "max_verification_pass_rate_drop": 0.0,
                    },
                    "checks": [
                        {"name": "pass_rate_drop", "passed": False},
                        {"name": "verification_pass_rate_drop", "passed": False},
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
                        "new_failure_count": 2,
                        "resolved_failure_count": 0,
                        "pass_rate_delta": -0.0238,
                        "verification_pass_rate_delta": -0.2,
                        "evidence_backed_pass_rate_delta": -0.15,
                    },
                    "regressions": [
                        {
                            "key": "critical-analyze-workspace-readme-zh-cn",
                            "base_verdict": "pass",
                            "target_verdict": "fail",
                            "target_failure_label": "tool_selection_error",
                        }
                    ],
                    "improvements": [],
                },
            }
        )

        self.assertIn("# Execution Gate Report", markdown)
        self.assertIn("Verification pass rate: 0.8 (base 1, delta -0.2)", markdown)
        self.assertIn("Comparison base resolution: existing_default_baseline", markdown)
        self.assertIn("## Locale Regression Alerts", markdown)
        self.assertIn("zh-CN: pass=2/3 (0.6667)", markdown)
        self.assertIn("## Failed Checks", markdown)
        self.assertIn("- verification_pass_rate_drop", markdown)
        self.assertIn("critical-analyze-workspace-readme-zh-cn: pass -> fail | target_failure=tool_selection_error", markdown)

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

        self.assertIn("Missing baseline eval spec: eval-spec-1", markdown)
        self.assertIn("Existing baselines: 0", markdown)



if __name__ == "__main__":
    unittest.main()
