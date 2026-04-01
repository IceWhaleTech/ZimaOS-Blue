import json
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Optional


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import execution_gate_history_report as history


class ExecutionGateHistoryReportTest(unittest.TestCase):
    def write_report(self, root: Path, name: str, payload: dict) -> Path:
        path = root / name
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return path

    def make_report(
        self,
        generated_at: str,
        candidate_label: str,
        passed: bool,
        report_status: str,
        eval_run_id: str,
        pass_rate: float,
        candidate_id: str = "",
        locale_breakdown: Optional[dict] = None,
        primary_route_breakdown: Optional[dict] = None,
    ) -> dict:
        return {
            "generated_at": generated_at,
            "candidate_id": candidate_id or candidate_label,
            "candidate_label": candidate_label,
            "passed": passed,
            "status": report_status,
            "eval_run": {
                "id": eval_run_id,
                "title": candidate_label,
                "status": "completed",
            },
            "execution_gate": {
                "metrics": {
                    "pass_rate": pass_rate,
                    "critical_pass_rate": 1.0 if passed else 0.75,
                    "verification_pass_rate_delta": 0.0 if passed else -0.2,
                    "evidence_backed_pass_rate_delta": 0.0 if passed else -0.15,
                    "critical_regression_count": 0 if passed else 1,
                    "locale_breakdown": locale_breakdown or {},
                    "primary_route_breakdown": primary_route_breakdown or {},
                }
            },
        }

    def test_build_summary_uses_latest_candidate_when_not_specified(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report("2026-03-28T00:00:00Z", "candidate-a", True, "passed", "run-a1", 1.0),
            )
            self.write_report(
                root,
                "report-b1.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-b", False, "failed", "run-b1", 0.95),
            )

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="", candidate_label="", require_consecutive_green=0)

        self.assertEqual(summary["focus_candidate_label"], "candidate-b")
        self.assertEqual(summary["focus_candidate_id"], "candidate-b")
        self.assertEqual(summary["focused_candidate"]["latest_attempt"]["eval_run_id"], "run-b1")

    def test_build_summary_checks_consecutive_green_requirement(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report("2026-03-28T00:00:00Z", "candidate-a", True, "passed", "run-a1", 1.0),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a", True, "passed", "run-a2", 1.0),
            )
            self.write_report(
                root,
                "report-a3.json",
                self.make_report("2026-03-27T00:00:00Z", "candidate-a", False, "failed", "run-a0", 0.95),
            )

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="", candidate_label="candidate-a", require_consecutive_green=2)

        self.assertTrue(summary["requirement_passed"])
        self.assertEqual(summary["focused_candidate"]["consecutive_green_count"], 2)

    def test_build_summary_fails_requirement_when_streak_is_too_short(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report("2026-03-28T00:00:00Z", "candidate-a", False, "failed", "run-a1", 0.95),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a", True, "passed", "run-a2", 1.0),
            )

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="", candidate_label="candidate-a", require_consecutive_green=2)

        self.assertFalse(summary["requirement_passed"])
        self.assertEqual(summary["focused_candidate"]["consecutive_green_count"], 1)

    def test_build_summary_groups_attempts_by_candidate_id(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report("2026-03-28T00:00:00Z", "candidate-a-zh", True, "passed", "run-a1", 1.0, candidate_id="candidate-a"),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a-en", True, "passed", "run-a2", 1.0, candidate_id="candidate-a"),
            )

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="candidate-a", candidate_label="", require_consecutive_green=2)

        self.assertTrue(summary["requirement_passed"])
        self.assertEqual(summary["focus_candidate_id"], "candidate-a")
        self.assertEqual(summary["focused_candidate"]["candidate_label"], "candidate-a-en")
        self.assertEqual(summary["focused_candidate"]["consecutive_green_count"], 2)

    def test_build_markdown_report_includes_candidate_overview(self):
        summary = {
            "generated_at": "2026-03-29T00:00:00Z",
            "focus_candidate_id": "candidate-a",
            "focus_candidate_label": "candidate-a",
            "require_consecutive_green": 2,
            "requirement_passed": True,
            "total_report_count": 3,
            "candidate_count": 2,
            "focused_candidate": {
                "candidate_id": "candidate-a",
                "candidate_label": "candidate-a",
                "attempt_count": 2,
                "pass_count": 2,
                "consecutive_green_count": 2,
                "latest_attempt": {
                    "report_path": "report-a2.json",
                    "eval_run_id": "run-a2",
                    "status": "passed",
                    "passed": True,
                    "pass_rate": 1.0,
                    "critical_pass_rate": 1.0,
                    "verification_pass_rate_delta": 0.0,
                    "evidence_backed_pass_rate_delta": 0.0,
                    "critical_regression_count": 0,
                    "locale_drift_alerts": [
                        {
                            "key": "zh-CN",
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
                    ],
                    "primary_route_drift_alerts": [
                        {
                            "key": "analyze",
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
                    ],
                },
                "recent_attempts": [
                    {
                        "generated_at": "2026-03-29T00:00:00Z",
                        "report_path": "report-a2.json",
                        "passed": True,
                        "status": "passed",
                        "pass_rate": 1.0,
                        "locale_drift_alerts": [],
                        "primary_route_drift_alerts": [],
                    }
                ],
                "latest_locale_drift_alerts": [
                    {
                        "key": "zh-CN",
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
                ],
                "latest_primary_route_drift_alerts": [
                    {
                        "key": "analyze",
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
                ],
                "recurring_locale_drift": [
                    {
                        "key": "zh-CN",
                        "attempts_with_drift": 2,
                        "critical_regression_attempts": 1,
                        "new_failure_attempts": 1,
                        "worst_pass_rate": 0.6667,
                        "latest_report_path": "report-a2.json",
                    }
                ],
                "recurring_primary_route_drift": [
                    {
                        "key": "analyze",
                        "attempts_with_drift": 2,
                        "critical_regression_attempts": 1,
                        "new_failure_attempts": 1,
                        "worst_pass_rate": 0.5,
                        "latest_report_path": "report-a2.json",
                    }
                ],
            },
            "candidates": [
                {
                    "candidate_label": "candidate-a",
                    "candidate_id": "candidate-a",
                    "latest_passed": True,
                    "consecutive_green_count": 2,
                    "attempt_count": 2,
                    "latest_locale_drift_count": 1,
                    "latest_primary_route_drift_count": 1,
                    "latest_report_path": "report-a2.json",
                }
            ],
        }

        markdown = history.build_markdown_report(summary, "Execution Gate History Summary")
        self.assertIn("# Execution Gate History Summary", markdown)
        self.assertIn("Focus Candidate ID: candidate-a", markdown)
        self.assertIn("Consecutive Green Count: 2", markdown)
        self.assertIn("Latest Pass Rate: 1", markdown)
        self.assertIn("## Latest Locale Drift Alerts", markdown)
        self.assertIn("zh-CN: pass=2/3 (0.6667)", markdown)
        self.assertIn("## Recurring Primary Route Drift", markdown)
        self.assertIn("analyze: drift_attempts=2", markdown)
        self.assertIn("candidate-a | label=candidate-a | latest=pass | streak=2 | attempts=2 | locale_drift=1 | route_drift=1", markdown)

    def test_expand_report_paths_supports_recursive_globs(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            nested = root / "downloaded" / "run-1" / "docs" / "reports"
            nested.mkdir(parents=True)
            report_path = nested / "execution_gate_report.json"
            report_path.write_text("{}", encoding="utf-8")

            matches = history.expand_report_paths([str(root / "**" / "*.json")])

        self.assertEqual(matches, [report_path.resolve()])

    def test_load_attempts_skips_non_execution_gate_reports(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            execution_report = self.write_report(
                root,
                "execution.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a", True, "passed", "run-a1", 1.0),
            )
            self.write_report(
                root,
                "history-summary.json",
                {
                    "generated_at": "2026-03-29T01:00:00Z",
                    "focus_candidate_label": "candidate-a",
                    "requirement_passed": True,
                },
            )

            attempts = history.load_attempts([execution_report, root / "history-summary.json"])

        self.assertEqual(len(attempts), 1)
        self.assertEqual(attempts[0].candidate_label, "candidate-a")

    def test_build_summary_tracks_latest_and_recurring_drift_hotspots(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report(
                    "2026-03-29T00:00:00Z",
                    "candidate-a",
                    False,
                    "failed",
                    "run-a1",
                    0.95,
                    locale_breakdown={
                        "zh-CN": {
                            "case_count": 3,
                            "passed_count": 2,
                            "pass_rate": 0.6667,
                            "critical_case_count": 1,
                            "critical_passed_count": 0,
                            "critical_pass_rate": 0.0,
                            "regression_count": 1,
                            "new_failure_count": 1,
                            "critical_regression_count": 1,
                        }
                    },
                    primary_route_breakdown={
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
                ),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report(
                    "2026-03-30T00:00:00Z",
                    "candidate-a",
                    True,
                    "passed",
                    "run-a2",
                    1.0,
                    locale_breakdown={
                        "zh-CN": {
                            "case_count": 3,
                            "passed_count": 3,
                            "pass_rate": 1.0,
                            "critical_case_count": 1,
                            "critical_passed_count": 1,
                            "critical_pass_rate": 1.0,
                            "regression_count": 0,
                            "improvement_count": 1,
                            "resolved_failure_count": 1,
                            "critical_regression_count": 0,
                        }
                    },
                    primary_route_breakdown={
                        "analyze": {
                            "case_count": 4,
                            "passed_count": 4,
                            "pass_rate": 1.0,
                            "critical_case_count": 2,
                            "critical_passed_count": 2,
                            "critical_pass_rate": 1.0,
                            "improvement_count": 1,
                            "resolved_failure_count": 1,
                            "critical_regression_count": 0,
                        }
                    },
                ),
            )

            attempts = history.load_attempts(history.expand_report_paths([str(root / "*.json")]))
            summary = history.build_summary(attempts, candidate_id="", candidate_label="candidate-a", require_consecutive_green=0)

        focused = summary["focused_candidate"]
        self.assertEqual(focused["latest_attempt"]["eval_run_id"], "run-a2")
        self.assertEqual(focused["latest_locale_drift_alerts"], [])
        self.assertEqual(focused["latest_primary_route_drift_alerts"], [])
        self.assertEqual(focused["recurring_locale_drift"][0]["key"], "zh-CN")
        self.assertEqual(focused["recurring_locale_drift"][0]["attempts_with_drift"], 1)
        self.assertEqual(focused["recurring_locale_drift"][0]["critical_regression_attempts"], 1)
        self.assertEqual(focused["recurring_primary_route_drift"][0]["key"], "analyze")
        self.assertEqual(summary["candidates"][0]["latest_locale_drift_count"], 0)
        self.assertEqual(summary["candidates"][0]["latest_primary_route_drift_count"], 0)


if __name__ == "__main__":
    unittest.main()
