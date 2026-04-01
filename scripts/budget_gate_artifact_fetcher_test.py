import io
import json
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path
from types import SimpleNamespace


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import budget_gate_artifact_fetcher as fetcher


class StubGitHubActionsClient:
    def __init__(self, workflow_pages, run_artifacts, artifact_payloads):
        self.workflow_pages = workflow_pages
        self.run_artifacts = run_artifacts
        self.artifact_payloads = artifact_payloads
        self.list_workflow_runs_calls = []
        self.list_run_artifacts_calls = []
        self.download_artifact_zip_calls = []
        self.repo = "example/ZimaOS-Blue"

    def list_workflow_runs(self, workflow: str, branch: str, per_page: int, page: int):
        self.list_workflow_runs_calls.append(
            {
                "workflow": workflow,
                "branch": branch,
                "per_page": per_page,
                "page": page,
            }
        )
        return {"workflow_runs": self.workflow_pages.get(page, [])}

    def list_run_artifacts(self, run_id: str):
        self.list_run_artifacts_calls.append(run_id)
        return {"artifacts": self.run_artifacts.get(run_id, [])}

    def download_artifact_zip(self, artifact_id: str):
        self.download_artifact_zip_calls.append(artifact_id)
        return self.artifact_payloads[artifact_id]


def make_zip(members):
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for name, content in members.items():
            archive.writestr(name, content)
    return buffer.getvalue()


class BudgetGateArtifactFetcherTest(unittest.TestCase):
    def test_iter_workflow_runs_fetches_more_pages_when_current_run_is_excluded(self):
        client = StubGitHubActionsClient(
            workflow_pages={
                1: [{"id": 300}, {"id": 299}],
                2: [{"id": 298}],
            },
            run_artifacts={},
            artifact_payloads={},
        )

        runs = fetcher.iter_workflow_runs(
            client=client,
            workflow="budget-gate.yml",
            branch="main",
            exclude_run_id="300",
            limit_runs=2,
        )

        self.assertEqual([str(run["id"]) for run in runs], ["299", "298"])
        self.assertEqual([call["page"] for call in client.list_workflow_runs_calls], [1, 2])

    def test_fetch_budget_gate_history_downloads_matching_artifacts_and_writes_manifest(self):
        matching_report = json.dumps(
            {
                "candidate_label": "batch-1",
                "passed": True,
                "status": "passed",
                "eval_run": {"id": "run-211"},
                "budget_gate": {"passed": True, "metrics": {"median_schema_byte_reduction_rate": 0.85}},
            }
        )
        stale_report = json.dumps(
            {
                "candidate_label": "batch-0",
                "passed": False,
                "status": "failed",
                "eval_run": {"id": "run-210"},
                "budget_gate": {"passed": False, "metrics": {"median_schema_byte_reduction_rate": 0.4}},
            }
        )
        history_summary = json.dumps(
            {
                "focus_candidate_label": "batch-1",
                "requirement_passed": False,
                "candidates": [],
            }
        )
        client = StubGitHubActionsClient(
            workflow_pages={
                1: [
                    {"id": 211, "run_number": 9, "run_attempt": 1, "head_branch": "main", "display_title": "Budget Gate", "created_at": "2026-03-28T00:00:00Z", "updated_at": "2026-03-28T00:05:00Z"},
                    {"id": 210, "run_number": 8, "run_attempt": 1, "head_branch": "main", "display_title": "Budget Gate", "created_at": "2026-03-27T00:00:00Z", "updated_at": "2026-03-27T00:05:00Z"},
                ]
            },
            run_artifacts={
                "211": [
                    {"id": 501, "name": "budget-gate-report", "expired": False, "size_in_bytes": 1200},
                    {"id": 502, "name": "other-report", "expired": False, "size_in_bytes": 400},
                ],
                "210": [
                    {"id": 503, "name": "budget-gate-report", "expired": True, "size_in_bytes": 1000},
                    {"id": 504, "name": "budget-gate-report", "expired": False, "size_in_bytes": 1100},
                ],
            },
            artifact_payloads={
                "501": make_zip(
                    {
                        "docs/reports/budget_gate_report.json": matching_report,
                        "docs/reports/budget_gate_history_report.json": history_summary,
                        "docs/reports/budget_gate_report.md": "# current\n",
                    }
                ),
                "504": make_zip(
                    {
                        "docs/reports/budget_gate_report.json": stale_report,
                    }
                ),
            },
        )

        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            args = SimpleNamespace(
                workflow="budget-gate.yml",
                branch="main",
                exclude_run_id="",
                limit_runs=5,
                artifact_name="budget-gate-report",
                output_dir=str(root / "history"),
                output_json=str(root / "history" / "manifest.json"),
            )

            manifest = fetcher.fetch_budget_gate_history(client, args)
            fetcher.write_manifest(manifest, Path(args.output_json))

            self.assertEqual(manifest["considered_run_count"], 2)
            self.assertEqual(manifest["downloaded_artifact_count"], 2)
            self.assertEqual(manifest["downloaded_report_count"], 2)
            self.assertEqual(client.list_run_artifacts_calls, ["211", "210"])
            self.assertEqual(client.download_artifact_zip_calls, ["501", "504"])

            current_report = root / "history" / "run-211-artifact-501" / "docs" / "reports" / "budget_gate_report.json"
            previous_report = root / "history" / "run-210-artifact-504" / "docs" / "reports" / "budget_gate_report.json"
            staged_current = root / "history" / "budget_reports" / "run-211-artifact-501-budget_gate_report.json"
            staged_previous = root / "history" / "budget_reports" / "run-210-artifact-504-budget_gate_report.json"
            skipped_history_summary = root / "history" / "budget_reports" / "run-211-artifact-501-budget_gate_history_report.json"
            self.assertTrue(current_report.is_file())
            self.assertTrue(previous_report.is_file())
            self.assertTrue(staged_current.is_file())
            self.assertTrue(staged_previous.is_file())
            self.assertFalse(skipped_history_summary.exists())

            manifest_payload = json.loads((root / "history" / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(manifest_payload["downloaded_artifact_count"], 2)
            self.assertEqual(manifest_payload["downloaded_report_count"], 2)

    def test_extract_artifact_zip_skips_unsafe_members(self):
        raw_zip = make_zip(
            {
                "../../escape.json": "{}",
                "/absolute/path.json": "{}",
                "docs/reports/budget_gate_report.json": "{}",
            }
        )

        with tempfile.TemporaryDirectory() as tmp_dir:
            destination = Path(tmp_dir) / "artifact"
            extracted = fetcher.extract_artifact_zip(raw_zip, destination)

            self.assertEqual(extracted, ["docs/reports/budget_gate_report.json"])
            self.assertTrue((destination / "docs" / "reports" / "budget_gate_report.json").is_file())
            self.assertFalse((destination / "escape.json").exists())


if __name__ == "__main__":
    unittest.main()
