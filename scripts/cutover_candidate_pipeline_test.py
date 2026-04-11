import json
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import cutover_candidate_pipeline as pipeline


class CutoverCandidatePipelineTest(unittest.TestCase):
    def make_args(self, root: Path, **overrides):
        base = {
            "blue_base_url": "http://127.0.0.1:18080/api/v1",
            "api_key": "",
            "bearer_token": "",
            "owner": "release-gate",
            "candidate_id": "candidate-1",
            "candidate_label": "batch-1",
            "selector_eval_spec_id": "",
            "execution_eval_spec_id": "",
            "selector_baseline_id": "selector-baseline",
            "selector_base_eval_run_id": "",
            "execution_baseline_id": "execution-baseline",
            "execution_base_eval_run_id": "",
            "budget_baseline_id": "budget-baseline",
            "budget_base_eval_run_id": "",
            "required_consecutive_runs": 2,
            "max_assessments": 5,
            "min_pass_rate": 0.98,
            "min_critical_pass_rate": 1.0,
            "min_route_agreement_rate": None,
            "min_route_compatible_rate": 0.95,
            "max_clarify_rate_delta": 0.01,
            "selector_max_critical_regressions": 0,
            "max_pass_rate_drop": 0.01,
            "execution_max_critical_regressions": 0,
            "max_verification_pass_rate_drop": 0.0,
            "max_evidence_backed_pass_rate_drop": 0.0,
            "min_median_schema_byte_reduction_rate": 0.8,
            "max_median_latency_increase_rate": 0.1,
            "allowed_final_native_tools": "exec",
            "require_consecutive_green": 0,
            "history_reports": [],
            "history_artifacts_dir": "",
            "history_output_json": "",
            "history_output_md": "",
            "github_repo": "",
            "github_token": "",
            "github_api_base_url": "https://api.github.com",
            "history_workflow": "cutover-candidate-pipeline.yml",
            "history_branch": "",
            "history_artifact_name": "cutover-candidate-pipeline-report",
            "history_limit_runs": 10,
            "history_exclude_run_id": "",
            "output_dir": str(root),
            "output_json": "",
            "output_md": "",
            "verbose": False,
        }
        base.update(overrides)
        return SimpleNamespace(**base)

    def test_pipeline_reuses_selector_eval_run_for_budget(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            commands = []

            def fake_runner(command, env):
                commands.append(list(command))
                script = Path(command[1]).name
                output_json = Path(command[command.index("--output-json") + 1])
                output_md = Path(command[command.index("--output-md") + 1])

                if script == "selector_gate_runner.py":
                    payload = {"status": "passed", "passed": True, "eval_run": {"id": "selector-eval-1"}}
                    returncode = 0
                elif script == "execution_gate_runner.py":
                    payload = {"status": "passed", "passed": True, "eval_run": {"id": "execution-eval-1"}}
                    returncode = 0
                elif script == "budget_gate_runner.py":
                    payload = {"status": "passed", "passed": True, "eval_run": {"id": "selector-eval-1"}, "reused_eval_run": True}
                    returncode = 0
                elif script == "cutover_readiness_runner.py":
                    payload = {
                        "status": "ready",
                        "ready": True,
                        "cutover_readiness": {
                            "evaluated_gates_ready": True,
                            "required_consecutive_runs": 2,
                            "selector": {"consecutive_pass_count": 2},
                            "execution": {"consecutive_pass_count": 2},
                            "budget": {"consecutive_pass_count": 2},
                        },
                    }
                    returncode = 0
                else:
                    raise AssertionError(f"unexpected script {script}")

                output_json.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                output_md.write_text(f"# {script}\n", encoding="utf-8")
                return SimpleNamespace(returncode=returncode, stdout="", stderr="")

            report = pipeline.run_cutover_candidate_pipeline(self.make_args(root), runner=fake_runner)

            self.assertTrue(report["ready"])
            self.assertEqual(report["selector_eval_run_id"], "selector-eval-1")
            self.assertEqual(report["offline_value_report"]["offline_recommendation"], "hold")
            self.assertEqual(report["runtime_status"], "not_started")
            budget_command = next(command for command in commands if Path(command[1]).name == "budget_gate_runner.py")
            self.assertIn("--eval-run-id", budget_command)
            self.assertEqual(budget_command[budget_command.index("--eval-run-id") + 1], "selector-eval-1")

    def test_pipeline_forwards_bearer_token_via_environment(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            seen_env = {}

            def fake_runner(command, env):
                seen_env["BLUE_BEARER_TOKEN"] = env.get("BLUE_BEARER_TOKEN", "")
                script = Path(command[1]).name
                output_json = Path(command[command.index("--output-json") + 1])
                output_md = Path(command[command.index("--output-md") + 1])

                if script == "cutover_readiness_runner.py":
                    payload = {
                        "status": "ready",
                        "ready": True,
                        "cutover_readiness": {
                            "evaluated_gates_ready": True,
                            "required_consecutive_runs": 2,
                            "selector": {"consecutive_pass_count": 2},
                            "execution": {"consecutive_pass_count": 2},
                            "budget": {"consecutive_pass_count": 2},
                        },
                    }
                else:
                    payload = {"status": "passed", "passed": True, "eval_run": {"id": f"{script}-eval-1"}}

                output_json.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                output_md.write_text(f"# {script}\n", encoding="utf-8")
                return SimpleNamespace(returncode=0, stdout="", stderr="")

            pipeline.run_cutover_candidate_pipeline(
                self.make_args(root, bearer_token="preview-token-123"),
                runner=fake_runner,
            )

            self.assertEqual(seen_env["BLUE_BEARER_TOKEN"], "preview-token-123")

    def test_pipeline_continues_to_readiness_when_selector_fails(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            commands = []

            def fake_runner(command, env):
                commands.append(Path(command[1]).name)
                script = Path(command[1]).name
                output_json = Path(command[command.index("--output-json") + 1])
                output_md = Path(command[command.index("--output-md") + 1])

                payload = {"status": "failed", "passed": False}
                returncode = 1
                if script == "execution_gate_runner.py":
                    payload = {"status": "passed", "passed": True, "eval_run": {"id": "execution-eval-1"}}
                    returncode = 0
                elif script == "selector_gate_runner.py":
                    payload = {"status": "failed", "passed": False, "eval_run": {"id": "selector-eval-1"}}
                elif script == "budget_gate_runner.py":
                    payload = {"status": "failed", "passed": False, "eval_run": {"id": "selector-eval-1"}, "reused_eval_run": True}
                elif script == "cutover_readiness_runner.py":
                    payload = {
                        "status": "not_ready",
                        "ready": False,
                        "cutover_readiness": {
                            "evaluated_gates_ready": False,
                            "required_consecutive_runs": 2,
                            "blocking_reasons": ["selector lane has only 0/2 consecutive passes"],
                            "selector": {"consecutive_pass_count": 0},
                            "execution": {"consecutive_pass_count": 1},
                            "budget": {"consecutive_pass_count": 0},
                        },
                    }
                output_json.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                output_md.write_text(f"# {script}\n", encoding="utf-8")
                return SimpleNamespace(returncode=returncode, stdout="", stderr="")

            report = pipeline.run_cutover_candidate_pipeline(self.make_args(root), runner=fake_runner)

            self.assertFalse(report["ready"])
            self.assertEqual(commands[-1], "cutover_readiness_runner.py")
            self.assertEqual(len(report["steps"]), 4)
            self.assertFalse(report["steps"][0]["passed"])

    def test_pipeline_surfaces_selector_and_execution_drift_summary(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)

            def fake_runner(command, env):
                script = Path(command[1]).name
                output_json = Path(command[command.index("--output-json") + 1])
                output_md = Path(command[command.index("--output-md") + 1])

                if script == "selector_gate_runner.py":
                    payload = {
                        "status": "passed",
                        "passed": True,
                        "eval_run": {"id": "selector-eval-1"},
                        "selector_gate": {
                            "metrics": {
                                "locale_breakdown": {
                                    "zh-CN": {
                                        "case_count": 5,
                                        "route_agreement_count": 4,
                                        "route_agreement_rate": 0.8,
                                        "route_compatible_count": 4,
                                        "route_compatible_rate": 0.8,
                                        "route_disagreement_count": 1,
                                        "critical_regression_count": 1,
                                        "clarify_rate_delta": 0.0,
                                    }
                                },
                                "primary_route_breakdown": {
                                    "web_search": {
                                        "case_count": 3,
                                        "route_agreement_count": 2,
                                        "route_agreement_rate": 0.666667,
                                        "route_compatible_count": 2,
                                        "route_compatible_rate": 0.666667,
                                        "route_disagreement_count": 1,
                                        "critical_regression_count": 1,
                                        "clarify_rate_delta": 0.1,
                                    }
                                },
                            }
                        },
                    }
                    returncode = 0
                elif script == "execution_gate_runner.py":
                    payload = {
                        "status": "passed",
                        "passed": True,
                        "eval_run": {"id": "execution-eval-1"},
                        "execution_gate": {
                            "metrics": {
                                "locale_breakdown": {
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
                                "primary_route_breakdown": {
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
                            }
                        },
                    }
                    returncode = 0
                elif script == "budget_gate_runner.py":
                    payload = {"status": "passed", "passed": True, "eval_run": {"id": "selector-eval-1"}, "reused_eval_run": True}
                    returncode = 0
                elif script == "cutover_readiness_runner.py":
                    payload = {
                        "status": "ready",
                        "ready": True,
                        "cutover_readiness": {
                            "evaluated_gates_ready": True,
                            "required_consecutive_runs": 2,
                            "selector": {"consecutive_pass_count": 2},
                            "execution": {"consecutive_pass_count": 2},
                            "budget": {"consecutive_pass_count": 2},
                        },
                    }
                    returncode = 0
                else:
                    raise AssertionError(f"unexpected script {script}")

                output_json.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                output_md.write_text(f"# {script}\n", encoding="utf-8")
                return SimpleNamespace(returncode=returncode, stdout="", stderr="")

            report = pipeline.run_cutover_candidate_pipeline(self.make_args(root), runner=fake_runner)

            self.assertEqual(report["drift_summary"]["selector"]["locale_alert_count"], 1)
            self.assertEqual(report["drift_summary"]["selector"]["primary_route_alert_count"], 1)
            self.assertEqual(report["drift_summary"]["execution"]["locale_alert_count"], 1)
            self.assertEqual(report["drift_summary"]["execution"]["primary_route_alert_count"], 1)
            self.assertEqual(report["drift_summary"]["selector"]["locale_alerts"][0]["key"], "zh-CN")
            self.assertEqual(report["drift_summary"]["execution"]["primary_route_alerts"][0]["key"], "browser")

    def test_build_markdown_report_includes_readiness_snapshot(self):
        markdown = pipeline.build_markdown_report(
            {
                "generated_at": "2026-03-28T00:00:00Z",
                "candidate_id": "candidate-1",
                "candidate_label": "batch-1",
                "status": "not_ready",
                "ready": False,
                "attempt_status": "ready",
                "attempt_ready": True,
                "selector_eval_run_id": "selector-eval-1",
                "steps": [
                    {"name": "selector", "passed": True, "returncode": 0, "output_json": "selector.json"},
                    {"name": "execution", "passed": True, "returncode": 0, "output_json": "execution.json"},
                    {"name": "budget", "passed": False, "returncode": 1, "output_json": "budget.json"},
                ],
                "readiness": {
                    "evaluated_gates_ready": False,
                    "required_consecutive_runs": 2,
                    "selector": {"consecutive_pass_count": 2},
                    "execution": {"consecutive_pass_count": 2},
                    "budget": {"consecutive_pass_count": 1},
                    "blocking_reasons": ["budget lane has only 1/2 consecutive passes"],
                },
                "offline_value_report": {
                    "offline_recommendation": "reject",
                    "value_summary": "Execution quality regressed and cutover readiness is blocked.",
                    "top_improvements": ["Selector lane stayed green."],
                    "top_tradeoffs": ["Budget lane has only 1/2 consecutive passes."],
                    "confidence": "medium",
                },
                "runtime_status": "not_started",
                "drift_summary": {
                    "selector": {
                        "locale_alert_count": 1,
                        "primary_route_alert_count": 0,
                        "locale_alerts": [
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
                        "primary_route_alerts": [],
                    },
                    "execution": {
                        "locale_alert_count": 0,
                        "primary_route_alert_count": 1,
                        "locale_alerts": [],
                        "primary_route_alerts": [
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
                    },
                },
                "history_gate": {
                    "evaluated": True,
                    "requirement_passed": False,
                    "require_consecutive_green": 2,
                    "focus_candidate_id": "candidate-1",
                    "focus_candidate_label": "batch-1",
                    "attempt_count": 2,
                    "ready_count": 1,
                    "consecutive_green_count": 1,
                    "latest_status": "not_ready",
                    "latest_ready": False,
                    "output_json": "history.json",
                    "output_md": "history.md",
                },
                "final_gate": {
                    "cutover_ready": False,
                    "history_gate_evaluated": True,
                    "blocking_reasons": [
                        "candidate candidate-1 has only 1/2 consecutive green attempts",
                    ],
                },
            }
        )

        self.assertIn("# Cutover Candidate Pipeline Report", markdown)
        self.assertIn("Cutover Ready: no", markdown)
        self.assertIn("Attempt Ready: yes", markdown)
        self.assertIn("Selector Streak: 2/2", markdown)
        self.assertIn("Budget Streak: 1/2", markdown)
        self.assertIn("## Offline Value", markdown)
        self.assertIn("Recommendation: REJECT", markdown)
        self.assertIn("Execution quality regressed and cutover readiness is blocked.", markdown)
        self.assertIn("Runtime Status: NOT_STARTED", markdown)
        self.assertIn("## Drift Snapshot", markdown)
        self.assertIn("Selector locale drift alerts: 1", markdown)
        self.assertIn("Execution primary-route drift alerts: 1", markdown)
        self.assertIn("Selector Locale Drift Alerts", markdown)
        self.assertIn("Execution Primary Route Drift Alerts", markdown)
        self.assertIn("zh-CN: compatible=4/5 (0.8)", markdown)
        self.assertIn("browser: pass=2/3 (0.666667)", markdown)
        self.assertIn("## History Gate", markdown)
        self.assertIn("Requirement Passed: no", markdown)
        self.assertIn("Consecutive Green Requirement: 2", markdown)
        self.assertIn("History JSON: history.json", markdown)
        self.assertIn("## Final Gate", markdown)
        self.assertIn("History Gate Evaluated: yes", markdown)
        self.assertIn("## Final Blocking Reasons", markdown)
        self.assertIn("candidate candidate-1 has only 1/2 consecutive green attempts", markdown)
        self.assertIn("budget lane has only 1/2 consecutive passes", markdown)

    def test_pipeline_builds_offline_value_report_from_gate_metrics(self):
        report = {
            "status": "ready",
            "ready": True,
            "attempt_status": "ready",
            "attempt_ready": True,
            "steps": [
                {
                    "name": "selector",
                    "passed": True,
                    "report": {
                        "selector_gate": {
                            "metrics": {
                                "pass_rate": 1.0,
                                "critical_pass_rate": 1.0,
                            }
                        }
                    },
                },
                {
                    "name": "execution",
                    "passed": True,
                    "report": {
                        "execution_gate": {
                            "metrics": {
                                "pass_rate_delta": 0.08,
                                "verification_pass_rate_delta": 0.04,
                                "evidence_backed_pass_rate_delta": 0.03,
                            }
                        }
                    },
                },
                {
                    "name": "budget",
                    "passed": True,
                    "report": {
                        "budget_gate": {
                            "metrics": {
                                "median_schema_byte_reduction_rate": 0.84,
                                "median_latency_increase_rate": 0.02,
                            }
                        }
                    },
                },
            ],
            "readiness": {
                "evaluated_gates_ready": True,
                "blocking_reasons": [],
            },
        }

        offline = pipeline.build_offline_value_report(report)

        self.assertEqual(offline["offline_recommendation"], "hold")
        self.assertIn("Execution pass rate improved by 0.08", offline["top_improvements"])
        self.assertEqual(offline["confidence"], "high")

    def test_summarize_history_gate_extracts_focus_snapshot(self):
        summary = pipeline.summarize_history_gate(
            {
                "summary": {
                    "requirement_passed": True,
                    "require_consecutive_green": 2,
                    "focus_candidate_id": "candidate-1",
                    "focus_candidate_label": "batch-1",
                    "focused_candidate": {
                        "attempt_count": 2,
                        "ready_count": 2,
                        "consecutive_green_count": 2,
                        "latest_attempt": {
                            "status": "ready",
                            "ready": True,
                        },
                    },
                },
                "output_json": "history.json",
                "output_md": "history.md",
                "artifacts_dir": "artifacts",
                "download_manifest": "manifest.json",
            }
        )

        self.assertTrue(summary["evaluated"])
        self.assertTrue(summary["requirement_passed"])
        self.assertEqual(summary["focus_candidate_id"], "candidate-1")
        self.assertEqual(summary["consecutive_green_count"], 2)
        self.assertEqual(summary["output_json"], "history.json")

    def test_apply_final_gate_state_distinguishes_attempt_and_cutover_ready(self):
        report = pipeline.apply_final_gate_state(
            {
                "candidate_id": "candidate-1",
                "attempt_status": "ready",
                "attempt_ready": True,
                "status": "ready",
                "ready": True,
                "history_gate": {
                    "evaluated": True,
                    "requirement_passed": False,
                    "require_consecutive_green": 2,
                    "focus_candidate_id": "candidate-1",
                    "consecutive_green_count": 1,
                },
            }
        )

        self.assertTrue(report["attempt_ready"])
        self.assertEqual(report["attempt_status"], "ready")
        self.assertFalse(report["ready"])
        self.assertEqual(report["status"], "not_ready")
        self.assertFalse(report["final_gate"]["cutover_ready"])
        self.assertIn("candidate candidate-1 has only 1/2 consecutive green attempts", report["final_gate"]["blocking_reasons"])

    def test_evaluate_history_gate_fails_when_current_attempt_is_only_green_run(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            args = self.make_args(root, require_consecutive_green=2)
            current_report = {
                "generated_at": "2026-03-29T00:00:00Z",
                "candidate_id": "candidate-1",
                "candidate_label": "batch-1",
                "status": "ready",
                "ready": True,
                "selector_eval_run_id": "selector-eval-1",
                "steps": [
                    {
                        "name": "selector",
                        "report": {
                            "eval_run": {"id": "selector-eval-1"},
                            "selector_gate": {"metrics": {}},
                        },
                    },
                    {
                        "name": "execution",
                        "report": {
                            "eval_run": {"id": "execution-eval-1"},
                            "execution_gate": {"metrics": {}},
                        },
                    },
                ],
                "readiness": {
                    "evaluated_gates_ready": True,
                    "required_consecutive_runs": 2,
                    "selector": {"consecutive_pass_count": 2},
                    "execution": {"consecutive_pass_count": 2},
                    "budget": {"consecutive_pass_count": 2},
                },
            }
            current_report_path = root / "cutover_candidate_pipeline_report.json"
            current_report_path.write_text(json.dumps(current_report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

            result = pipeline.evaluate_history_gate(args, root, current_report_path)
            self.assertFalse(result["summary"]["requirement_passed"])
            self.assertEqual(result["summary"]["focused_candidate"]["consecutive_green_count"], 1)
            self.assertTrue(Path(result["output_json"]).is_file())
            self.assertTrue(Path(result["output_md"]).is_file())

    def test_evaluate_history_gate_passes_with_previous_green_attempt(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            previous_report_dir = root / "prior"
            previous_report_dir.mkdir()
            args = self.make_args(
                root,
                require_consecutive_green=2,
                history_reports=[str(previous_report_dir / "*.json")],
            )
            current_report = {
                "generated_at": "2026-03-29T00:00:00Z",
                "candidate_id": "candidate-1",
                "candidate_label": "batch-1",
                "status": "ready",
                "ready": True,
                "selector_eval_run_id": "selector-eval-2",
                "steps": [
                    {
                        "name": "selector",
                        "report": {
                            "eval_run": {"id": "selector-eval-2"},
                            "selector_gate": {"metrics": {}},
                        },
                    },
                    {
                        "name": "execution",
                        "report": {
                            "eval_run": {"id": "execution-eval-2"},
                            "execution_gate": {"metrics": {}},
                        },
                    },
                ],
                "readiness": {
                    "evaluated_gates_ready": True,
                    "required_consecutive_runs": 2,
                    "selector": {"consecutive_pass_count": 2},
                    "execution": {"consecutive_pass_count": 2},
                    "budget": {"consecutive_pass_count": 2},
                },
            }
            previous_report = {
                "generated_at": "2026-03-28T00:00:00Z",
                "candidate_id": "candidate-1",
                "candidate_label": "batch-1",
                "status": "ready",
                "ready": True,
                "selector_eval_run_id": "selector-eval-1",
                "steps": [
                    {
                        "name": "selector",
                        "report": {
                            "eval_run": {"id": "selector-eval-1"},
                            "selector_gate": {"metrics": {}},
                        },
                    },
                    {
                        "name": "execution",
                        "report": {
                            "eval_run": {"id": "execution-eval-1"},
                            "execution_gate": {"metrics": {}},
                        },
                    },
                ],
                "readiness": {
                    "evaluated_gates_ready": True,
                    "required_consecutive_runs": 2,
                    "selector": {"consecutive_pass_count": 2},
                    "execution": {"consecutive_pass_count": 2},
                    "budget": {"consecutive_pass_count": 2},
                },
            }
            current_report_path = root / "cutover_candidate_pipeline_report.json"
            current_report_path.write_text(json.dumps(current_report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            (previous_report_dir / "older.json").write_text(json.dumps(previous_report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

            result = pipeline.evaluate_history_gate(args, root, current_report_path)
            self.assertTrue(result["summary"]["requirement_passed"])
            self.assertEqual(result["summary"]["focused_candidate"]["consecutive_green_count"], 2)
            self.assertTrue(Path(result["output_json"]).is_file())
            self.assertTrue(Path(result["output_md"]).is_file())


if __name__ == "__main__":
    unittest.main()
