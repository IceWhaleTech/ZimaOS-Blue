import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import cutover_candidate_pipeline_history_report as history


class CutoverCandidatePipelineHistoryReportTest(unittest.TestCase):
    def write_report(self, root: Path, name: str, payload: dict) -> Path:
        path = root / name
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return path

    def make_report(
        self,
        generated_at: str,
        candidate_id: str,
        candidate_label: str,
        ready: bool,
        status: str,
        selector_streak: int,
        execution_streak: int,
        budget_streak: int,
        blocking_reasons=None,
        selector_locale_breakdown=None,
        selector_primary_route_breakdown=None,
        execution_locale_breakdown=None,
        execution_primary_route_breakdown=None,
    ) -> dict:
        return {
            "generated_at": generated_at,
            "candidate_id": candidate_id,
            "candidate_label": candidate_label,
            "ready": ready,
            "status": status,
            "selector_eval_run_id": "selector-eval-1",
            "steps": [
                {
                    "name": "selector",
                    "report": {
                        "eval_run": {"id": "selector-eval-1"},
                        "selector_gate": {
                            "metrics": {
                                "locale_breakdown": selector_locale_breakdown or {},
                                "primary_route_breakdown": selector_primary_route_breakdown or {},
                            }
                        },
                    },
                },
                {
                    "name": "execution",
                    "report": {
                        "eval_run": {"id": "execution-eval-1"},
                        "execution_gate": {
                            "metrics": {
                                "locale_breakdown": execution_locale_breakdown or {},
                                "primary_route_breakdown": execution_primary_route_breakdown or {},
                            }
                        },
                    },
                },
            ],
            "readiness": {
                "evaluated_gates_ready": ready,
                "required_consecutive_runs": 2,
                "selector": {"consecutive_pass_count": selector_streak},
                "execution": {"consecutive_pass_count": execution_streak},
                "budget": {"consecutive_pass_count": budget_streak},
                "blocking_reasons": blocking_reasons or [],
            },
        }

    def test_build_summary_uses_latest_candidate_when_not_specified(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(root, "report-a1.json", self.make_report("2026-03-28T00:00:00Z", "candidate-a", "candidate-a", True, "ready", 2, 2, 2))
            self.write_report(root, "report-b1.json", self.make_report("2026-03-29T00:00:00Z", "candidate-b", "candidate-b", False, "not_ready", 2, 2, 1, ["budget lane red"]))

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="", candidate_label="", require_consecutive_green=0)

        self.assertEqual(summary["focus_candidate_id"], "candidate-b")
        self.assertEqual(summary["focused_candidate"]["latest_attempt"]["status"], "not_ready")

    def test_build_summary_checks_consecutive_green_requirement(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(root, "report-a1.json", self.make_report("2026-03-28T00:00:00Z", "candidate-a", "candidate-a-v1", True, "ready", 2, 2, 2))
            self.write_report(root, "report-a2.json", self.make_report("2026-03-29T00:00:00Z", "candidate-a", "candidate-a-v2", True, "ready", 2, 2, 2))

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="candidate-a", candidate_label="", require_consecutive_green=2)

        self.assertTrue(summary["requirement_passed"])
        self.assertEqual(summary["focused_candidate"]["consecutive_green_count"], 2)
        self.assertEqual(summary["focus_candidate_label"], "candidate-a-v2")

    def test_build_summary_fails_requirement_when_streak_is_too_short(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(root, "report-a1.json", self.make_report("2026-03-28T00:00:00Z", "candidate-a", "candidate-a", False, "not_ready", 2, 2, 1, ["budget lane red"]))
            self.write_report(root, "report-a2.json", self.make_report("2026-03-29T00:00:00Z", "candidate-a", "candidate-a", True, "ready", 2, 2, 2))

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="candidate-a", candidate_label="", require_consecutive_green=2)

        self.assertFalse(summary["requirement_passed"])
        self.assertEqual(summary["focused_candidate"]["consecutive_green_count"], 1)

    def test_build_summary_surfaces_selector_and_execution_drift(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report(
                    "2026-03-28T00:00:00Z",
                    "candidate-a",
                    "candidate-a",
                    True,
                    "ready",
                    2,
                    2,
                    2,
                    selector_locale_breakdown={
                        "zh-CN": {
                            "case_count": 4,
                            "route_agreement_count": 2,
                            "route_agreement_rate": 0.5,
                            "route_compatible_count": 2,
                            "route_compatible_rate": 0.5,
                            "route_disagreement_count": 2,
                            "critical_regression_count": 1,
                            "clarify_rate_delta": 0.0,
                        }
                    },
                    selector_primary_route_breakdown={
                        "web_search": {
                            "case_count": 3,
                            "route_agreement_count": 2,
                            "route_agreement_rate": 0.666667,
                            "route_compatible_count": 2,
                            "route_compatible_rate": 0.666667,
                            "route_disagreement_count": 1,
                            "critical_regression_count": 1,
                            "clarify_rate_delta": 0.0,
                        }
                    },
                    execution_locale_breakdown={
                        "ja-JP": {
                            "case_count": 2,
                            "passed_count": 1,
                            "pass_rate": 0.5,
                            "critical_case_count": 1,
                            "critical_passed_count": 0,
                            "critical_pass_rate": 0.0,
                            "regression_count": 1,
                            "improvement_count": 0,
                            "new_failure_count": 1,
                            "resolved_failure_count": 0,
                            "critical_regression_count": 1,
                        }
                    },
                    execution_primary_route_breakdown={
                        "browser": {
                            "case_count": 2,
                            "passed_count": 1,
                            "pass_rate": 0.5,
                            "critical_case_count": 1,
                            "critical_passed_count": 0,
                            "critical_pass_rate": 0.0,
                            "regression_count": 1,
                            "improvement_count": 0,
                            "new_failure_count": 1,
                            "resolved_failure_count": 0,
                            "critical_regression_count": 1,
                        }
                    },
                ),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report(
                    "2026-03-29T00:00:00Z",
                    "candidate-a",
                    "candidate-a",
                    True,
                    "ready",
                    2,
                    2,
                    2,
                    selector_locale_breakdown={
                        "zh-CN": {
                            "case_count": 5,
                            "route_agreement_count": 4,
                            "route_agreement_rate": 0.8,
                            "route_compatible_count": 4,
                            "route_compatible_rate": 0.8,
                            "route_disagreement_count": 1,
                            "critical_regression_count": 1,
                            "clarify_rate_delta": 0.2,
                        }
                    },
                    selector_primary_route_breakdown={
                        "web_search": {
                            "case_count": 4,
                            "route_agreement_count": 3,
                            "route_agreement_rate": 0.75,
                            "route_compatible_count": 3,
                            "route_compatible_rate": 0.75,
                            "route_disagreement_count": 1,
                            "critical_regression_count": 1,
                            "clarify_rate_delta": 0.1,
                        }
                    },
                    execution_locale_breakdown={
                        "ja-JP": {
                            "case_count": 3,
                            "passed_count": 2,
                            "pass_rate": 0.666667,
                            "critical_case_count": 1,
                            "critical_passed_count": 0,
                            "critical_pass_rate": 0.0,
                            "regression_count": 1,
                            "improvement_count": 0,
                            "new_failure_count": 1,
                            "resolved_failure_count": 0,
                            "critical_regression_count": 1,
                        }
                    },
                    execution_primary_route_breakdown={
                        "browser": {
                            "case_count": 3,
                            "passed_count": 2,
                            "pass_rate": 0.666667,
                            "critical_case_count": 1,
                            "critical_passed_count": 0,
                            "critical_pass_rate": 0.0,
                            "regression_count": 1,
                            "improvement_count": 0,
                            "new_failure_count": 1,
                            "resolved_failure_count": 0,
                            "critical_regression_count": 1,
                        }
                    },
                ),
            )

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="candidate-a", candidate_label="", require_consecutive_green=2)

        focused = summary["focused_candidate"]
        self.assertEqual(focused["latest_selector_locale_drift_alerts"][0]["key"], "zh-CN")
        self.assertEqual(focused["latest_selector_primary_route_drift_alerts"][0]["key"], "web_search")
        self.assertEqual(focused["latest_execution_locale_drift_alerts"][0]["key"], "ja-JP")
        self.assertEqual(focused["latest_execution_primary_route_drift_alerts"][0]["key"], "browser")
        self.assertEqual(focused["recurring_selector_locale_drift"][0]["key"], "zh-CN")
        self.assertEqual(focused["recurring_selector_locale_drift"][0]["attempts_with_drift"], 2)
        self.assertEqual(focused["recurring_execution_primary_route_drift"][0]["key"], "browser")
        self.assertEqual(focused["recurring_execution_primary_route_drift"][0]["attempts_with_drift"], 2)
        self.assertEqual(summary["candidates"][0]["latest_selector_locale_drift_count"], 1)
        self.assertEqual(summary["candidates"][0]["latest_execution_primary_route_drift_count"], 1)

    def test_build_markdown_report_includes_blocking_reasons(self):
        summary = {
            "generated_at": "2026-03-29T00:00:00Z",
            "focus_candidate_id": "candidate-a",
            "focus_candidate_label": "candidate-a-v2",
            "require_consecutive_green": 2,
            "requirement_passed": False,
            "total_report_count": 2,
            "candidate_count": 1,
            "focused_candidate": {
                "candidate_id": "candidate-a",
                "candidate_label": "candidate-a-v2",
                "attempt_count": 2,
                "ready_count": 1,
                "consecutive_green_count": 1,
                "latest_attempt": {
                    "report_path": "report-a2.json",
                    "ready": False,
                    "status": "not_ready",
                    "evaluated_gates_ready": False,
                    "selector_streak": 2,
                    "execution_streak": 2,
                    "budget_streak": 1,
                    "selector_locale_drift_count": 1,
                    "selector_primary_route_drift_count": 1,
                    "execution_locale_drift_count": 1,
                    "execution_primary_route_drift_count": 1,
                    "blocking_reasons": ["budget lane has only 1/2 consecutive passes"],
                },
                "recent_attempts": [
                    {
                        "generated_at": "2026-03-29T00:00:00Z",
                        "report_path": "report-a2.json",
                        "ready": False,
                        "status": "not_ready",
                        "selector_streak": 2,
                        "execution_streak": 2,
                        "budget_streak": 1,
                        "selector_locale_drift_count": 1,
                        "selector_primary_route_drift_count": 1,
                        "execution_locale_drift_count": 1,
                        "execution_primary_route_drift_count": 1,
                    }
                ],
                "latest_selector_locale_drift_alerts": [
                    {
                        "key": "zh-CN",
                        "case_count": 5,
                        "route_agreement_count": 4,
                        "route_agreement_rate": 0.8,
                        "route_compatible_count": 4,
                        "route_compatible_rate": 0.8,
                        "critical_regression_count": 1,
                        "clarify_rate_delta": 0.2,
                    }
                ],
                "latest_selector_primary_route_drift_alerts": [],
                "latest_execution_locale_drift_alerts": [],
                "latest_execution_primary_route_drift_alerts": [
                    {
                        "key": "browser",
                        "case_count": 3,
                        "passed_count": 2,
                        "pass_rate": 0.666667,
                        "critical_case_count": 1,
                        "critical_passed_count": 0,
                        "critical_pass_rate": 0.0,
                        "regression_count": 1,
                        "new_failure_count": 1,
                        "critical_regression_count": 1,
                    }
                ],
                "recurring_selector_locale_drift": [
                    {
                        "key": "zh-CN",
                        "attempts_with_drift": 2,
                        "critical_regression_attempts": 2,
                        "worst_route_compatible_rate": 0.5,
                        "max_abs_clarify_rate_delta": 0.2,
                        "latest_report_path": "report-a2.json",
                    }
                ],
                "recurring_selector_primary_route_drift": [],
                "recurring_execution_locale_drift": [],
                "recurring_execution_primary_route_drift": [
                    {
                        "key": "browser",
                        "attempts_with_drift": 2,
                        "critical_regression_attempts": 2,
                        "new_failure_attempts": 2,
                        "worst_pass_rate": 0.5,
                        "latest_report_path": "report-a2.json",
                    }
                ],
            },
            "candidates": [
                {
                    "candidate_id": "candidate-a",
                    "candidate_label": "candidate-a-v2",
                    "latest_ready": False,
                    "consecutive_green_count": 1,
                    "attempt_count": 2,
                    "latest_selector_locale_drift_count": 1,
                    "latest_selector_primary_route_drift_count": 1,
                    "latest_execution_locale_drift_count": 1,
                    "latest_execution_primary_route_drift_count": 1,
                    "latest_blocking_reason_count": 1,
                    "latest_report_path": "report-a2.json",
                }
            ],
        }

        markdown = history.build_markdown_report(summary, "Cutover Candidate Pipeline History Summary")
        self.assertIn("# Cutover Candidate Pipeline History Summary", markdown)
        self.assertIn("Focus Candidate ID: candidate-a", markdown)
        self.assertIn("Latest Selector Locale Drift Alerts", markdown)
        self.assertIn("Latest Execution Primary Route Drift Alerts", markdown)
        self.assertIn("Recurring Selector Locale Drift", markdown)
        self.assertIn("Recurring Execution Primary Route Drift", markdown)
        self.assertIn("Latest Blocking Reasons", markdown)
        self.assertIn("budget lane has only 1/2 consecutive passes", markdown)
        self.assertIn("zh-CN: compatible=4/5 (0.8)", markdown)
        self.assertIn("browser: pass=2/3 (0.666667)", markdown)
        self.assertIn("candidate-a | label=candidate-a-v2 | latest=not_ready | streak=1 | attempts=2 | selector_locale_drift=1", markdown)

    def test_expand_report_paths_supports_recursive_globs(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            nested = root / "downloaded" / "run-1" / "docs" / "reports"
            nested.mkdir(parents=True)
            report_path = nested / "cutover_candidate_pipeline_report.json"
            report_path.write_text("{}", encoding="utf-8")

            matches = history.expand_report_paths([str(root / "**" / "*.json")])

        self.assertEqual(matches, [report_path.resolve()])


if __name__ == "__main__":
    unittest.main()
