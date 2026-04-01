import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import budget_gate_history_report as history


class BudgetGateHistoryReportTest(unittest.TestCase):
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
        comparable_case_count: int,
        schema_reduction_rate: float,
        latency_increase_rate: float,
        non_allowed_native_tool_case_count: int,
        candidate_id: str = "",
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
            "budget_gate": {
                "metrics": {
                    "case_count": 120,
                    "comparable_case_count": comparable_case_count,
                    "missing_surface_case_count": 120 - comparable_case_count,
                    "median_schema_byte_reduction_rate": schema_reduction_rate,
                    "median_latency_increase_rate": latency_increase_rate,
                    "allowed_final_native_tool_case_rate": 1.0 if passed else 0.9,
                    "non_allowed_native_tool_case_count": non_allowed_native_tool_case_count,
                }
            },
        }

    def test_build_summary_uses_latest_candidate_when_not_specified(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            self.write_report(
                root,
                "report-a1.json",
                self.make_report("2026-03-28T00:00:00Z", "candidate-a", True, "passed", "run-a1", 120, 0.85, 0.05, 0),
            )
            self.write_report(
                root,
                "report-b1.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-b", False, "failed", "run-b1", 118, 0.4, 0.2, 8),
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
                self.make_report("2026-03-28T00:00:00Z", "candidate-a", True, "passed", "run-a1", 120, 0.85, 0.05, 0),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a", True, "passed", "run-a2", 120, 0.86, 0.04, 0),
            )
            self.write_report(
                root,
                "report-a3.json",
                self.make_report("2026-03-27T00:00:00Z", "candidate-a", False, "failed", "run-a0", 118, 0.5, 0.2, 4),
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
                self.make_report("2026-03-28T00:00:00Z", "candidate-a", False, "failed", "run-a1", 116, 0.5, 0.2, 6),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a", True, "passed", "run-a2", 120, 0.85, 0.05, 0),
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
                self.make_report("2026-03-28T00:00:00Z", "candidate-a-zh", True, "passed", "run-a1", 120, 0.85, 0.05, 0, candidate_id="candidate-a"),
            )
            self.write_report(
                root,
                "report-a2.json",
                self.make_report("2026-03-29T00:00:00Z", "candidate-a-en", True, "passed", "run-a2", 120, 0.86, 0.04, 0, candidate_id="candidate-a"),
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
                    "case_count": 120,
                    "comparable_case_count": 120,
                    "missing_surface_case_count": 0,
                    "median_schema_byte_reduction_rate": 0.85,
                    "median_latency_increase_rate": 0.05,
                    "allowed_final_native_tool_case_rate": 1.0,
                    "non_allowed_native_tool_case_count": 0,
                },
                "recent_attempts": [
                    {
                        "generated_at": "2026-03-29T00:00:00Z",
                        "report_path": "report-a2.json",
                        "passed": True,
                        "status": "passed",
                        "median_schema_byte_reduction_rate": 0.85,
                        "median_latency_increase_rate": 0.05,
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
                    "latest_median_schema_byte_reduction_rate": 0.85,
                    "latest_median_latency_increase_rate": 0.05,
                    "latest_non_allowed_native_tool_case_count": 0,
                }
            ],
        }

        markdown = history.build_markdown_report(summary, "Budget Gate History Summary")
        self.assertIn("# Budget Gate History Summary", markdown)
        self.assertIn("Focus Candidate ID: candidate-a", markdown)
        self.assertIn("Consecutive Green Count: 2", markdown)
        self.assertIn("Latest Median Schema-Byte Reduction: 0.85", markdown)
        self.assertIn("Latest Comparable Cases: 120/120", markdown)
        self.assertIn("schema_reduction=0.85", markdown)
        self.assertIn("candidate-a | label=candidate-a | latest=pass | streak=2 | attempts=2 | schema_reduction=0.85 | latency_increase=0.05 | non_allowed_tools=0", markdown)

    def test_expand_report_paths_supports_recursive_globs(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            nested = root / "downloaded" / "run-1" / "docs" / "reports"
            nested.mkdir(parents=True)
            report_path = nested / "budget_gate_report.json"
            report_path.write_text("{}", encoding="utf-8")

            matches = history.expand_report_paths([str(root / "**" / "*.json")])

        self.assertEqual(matches, [report_path.resolve()])


if __name__ == "__main__":
    unittest.main()
