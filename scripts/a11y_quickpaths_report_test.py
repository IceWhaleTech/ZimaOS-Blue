import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import a11y_quickpaths_report as report


class A11yQuickpathsReportTest(unittest.TestCase):
    def write_report(self, root: Path, payload: dict) -> Path:
        path = root / "a11y-report.json"
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return path

    def test_build_summary_computes_success_and_accuracy_rates(self):
        payload = {
            "generated_at": "2026-04-14T12:00:00Z",
            "eval_run": {
                "id": "eval-123",
                "title": "A11y Quick Paths",
                "model": "claude-sonnet-4.6",
                "provider_id": "114.demo",
            },
            "group_report": {
                "group": {
                    "id": "group-123",
                    "title": "A11y Quick Paths",
                },
                "items": [
                    {
                        "id": "item-macos-message",
                        "metadata": {"os": "macos"},
                        "input": {"goal": "Say hello to Orca in Feishu on macOS."},
                    },
                    {
                        "id": "item-windows-message",
                        "metadata": {"os": "windows"},
                        "input": {"goal": "Say hello to Orca in Feishu on Windows."},
                    },
                ],
                "linked_runs": [
                    {
                        "id": "run-macos",
                        "group_item_id": "item-macos-message",
                        "result": json.dumps(
                            {
                                "target_hit": True,
                                "verification_passed": True,
                                "verification_method": "ocr",
                                "input_method": "set_value",
                            }
                        ),
                    },
                    {
                        "id": "run-windows",
                        "group_item_id": "item-windows-message",
                        "result": json.dumps(
                            {
                                "target_hit": True,
                                "verification_passed": False,
                                "verification_method": "ocr",
                                "input_method": "clipboard",
                            }
                        ),
                    },
                ],
                "scorecards": [
                    {
                        "id": "card-macos",
                        "group_item_id": "item-macos-message",
                        "run_id": "run-macos",
                        "verdict": "pass",
                        "breakdown_json": json.dumps({"verification_passed": True, "target_hit": True}),
                    },
                    {
                        "id": "card-windows",
                        "group_item_id": "item-windows-message",
                        "run_id": "run-windows",
                        "verdict": "fail",
                        "breakdown_json": json.dumps({"verification_passed": False, "target_hit": True}),
                    },
                ],
            },
        }

        with tempfile.TemporaryDirectory() as tmp_dir:
            report_path = self.write_report(Path(tmp_dir), payload)
            summary = report.build_summary(report_path)

        self.assertEqual(summary["eval_run_id"], "eval-123")
        self.assertEqual(summary["model"], "claude-sonnet-4.6")
        self.assertEqual(summary["provider_id"], "114.demo")
        self.assertEqual(summary["case_count"], 2)
        self.assertEqual(summary["success_count"], 1)
        self.assertEqual(summary["accuracy_count"], 2)
        self.assertAlmostEqual(summary["success_rate"], 0.5)
        self.assertAlmostEqual(summary["accuracy_rate"], 1.0)
        self.assertEqual(summary["by_os"]["macos"]["success_count"], 1)
        self.assertEqual(summary["by_os"]["windows"]["success_count"], 0)
        self.assertEqual(summary["cases"][1]["verification_passed"], False)

    def test_build_summary_falls_back_to_scorecard_breakdown(self):
        payload = {
            "generated_at": "2026-04-14T12:00:00Z",
            "eval_run": {"id": "eval-456"},
            "group_report": {
                "items": [
                    {
                        "id": "item-settings-toggle",
                        "metadata": {"os": "macos"},
                        "input": {"goal": "Toggle a setting."},
                    }
                ],
                "linked_runs": [
                    {
                        "id": "run-settings-toggle",
                        "group_item_id": "item-settings-toggle",
                        "result": "not-json",
                    }
                ],
                "scorecards": [
                    {
                        "id": "card-settings-toggle",
                        "group_item_id": "item-settings-toggle",
                        "run_id": "run-settings-toggle",
                        "breakdown_json": json.dumps({"verification_passed": True, "target_hit": False}),
                    }
                ],
            },
        }

        with tempfile.TemporaryDirectory() as tmp_dir:
            report_path = self.write_report(Path(tmp_dir), payload)
            summary = report.build_summary(report_path)

        self.assertEqual(summary["case_count"], 1)
        self.assertEqual(summary["success_count"], 1)
        self.assertEqual(summary["accuracy_count"], 0)
        self.assertAlmostEqual(summary["success_rate"], 1.0)
        self.assertAlmostEqual(summary["accuracy_rate"], 0.0)
        self.assertEqual(summary["cases"][0]["target_hit"], False)

    def test_build_summary_computes_latency_percentiles_and_ratios(self):
        payload = {
            "generated_at": "2026-04-15T12:00:00Z",
            "eval_run": {"id": "eval-789", "model": "claude-sonnet-4.6"},
            "group_report": {
                "items": [
                    {"id": "macos-type", "metadata": {"os": "macos"}, "input": {"goal": "Type text on macOS."}},
                    {"id": "windows-type", "metadata": {"os": "windows"}, "input": {"goal": "Type text on Windows."}},
                    {"id": "macos-toggle", "metadata": {"os": "macos"}, "input": {"goal": "Toggle on macOS."}},
                ],
                "linked_runs": [
                    {
                        "id": "run-1",
                        "group_item_id": "macos-type",
                        "result": json.dumps(
                            {
                                "target_hit": True,
                                "verification_passed": True,
                                "cache_hit": True,
                                "fallbacks": [],
                                "query_ms": 5,
                                "action_ms": 18,
                                "verification_ms": 7,
                                "end_to_end_ms": 30,
                                "llm_latency_ms": 90,
                            }
                        ),
                    },
                    {
                        "id": "run-2",
                        "group_item_id": "windows-type",
                        "result": json.dumps(
                            {
                                "target_hit": True,
                                "verification_passed": True,
                                "cache_hit": False,
                                "fallbacks": ["full_snapshot_label_anchor"],
                                "query_ms": 8,
                                "action_ms": 22,
                                "verification_ms": 10,
                                "end_to_end_ms": 40,
                                "llm_latency_ms": 120,
                            }
                        ),
                    },
                    {
                        "id": "run-3",
                        "group_item_id": "macos-toggle",
                        "result": json.dumps(
                            {
                                "target_hit": True,
                                "verification_passed": False,
                                "cache_hit": True,
                                "fallbacks": ["watch_unavailable"],
                                "query_ms": 4,
                                "action_ms": 12,
                                "verification_ms": 0,
                                "end_to_end_ms": 20,
                                "llm_latency_ms": 80,
                            }
                        ),
                    },
                ],
                "scorecards": [],
            },
        }

        with tempfile.TemporaryDirectory() as tmp_dir:
            report_path = self.write_report(Path(tmp_dir), payload)
            summary = report.build_summary(report_path)

        self.assertEqual(summary["latency"]["native_end_to_end_ms"]["p50"], 30)
        self.assertEqual(summary["latency"]["native_end_to_end_ms"]["p90"], 40)
        self.assertEqual(summary["latency"]["llm_latency_ms"]["p50"], 90)
        self.assertAlmostEqual(summary["cache_hit_rate"], 2 / 3)
        self.assertAlmostEqual(summary["fallback_rate"], 2 / 3)


if __name__ == "__main__":
    unittest.main()
