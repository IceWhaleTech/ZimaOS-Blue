import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import cutover_readiness_runner as runner


class StubHarnessClient:
    def __init__(self):
        self.requests = []

    def evaluate_cutover_readiness(self, payload):
        self.requests.append(payload)
        return {
            "candidate_id": "candidate-1",
            "required_consecutive_runs": 2,
            "evaluated_gates_ready": False,
            "ready": False,
            "blocking_reasons": ["budget lane has only 1/2 consecutive passes"],
            "selector": {
                "candidate_id": "candidate-1",
                "required_consecutive_runs": 2,
                "candidate_run_count": 2,
                "consecutive_pass_count": 2,
                "ready": True,
                "baseline_id": "selector-baseline",
                "assessments": [
                    {"eval_run_id": "selector-2", "passed": True, "created_at": "2026-03-28T00:00:00Z", "failed_checks": []}
                ],
            },
            "execution": {
                "candidate_id": "candidate-1",
                "required_consecutive_runs": 2,
                "candidate_run_count": 2,
                "consecutive_pass_count": 2,
                "ready": True,
                "baseline_id": "execution-baseline",
                "assessments": [
                    {"eval_run_id": "execution-2", "passed": True, "created_at": "2026-03-28T00:01:00Z", "failed_checks": []}
                ],
            },
            "budget": {
                "candidate_id": "candidate-1",
                "required_consecutive_runs": 2,
                "candidate_run_count": 2,
                "consecutive_pass_count": 1,
                "ready": False,
                "baseline_id": "budget-baseline",
                "assessments": [
                    {"eval_run_id": "budget-2", "passed": False, "created_at": "2026-03-28T00:02:00Z", "failed_checks": ["final_native_tool_surface"]}
                ],
            },
        }


class CutoverReadinessRunnerTest(unittest.TestCase):
    def make_args(self, **overrides):
        base = {
            "owner": "release-gate",
            "candidate_id": "candidate-1",
            "required_consecutive_runs": 2,
            "max_assessments": 5,
            "selector_baseline_id": "selector-baseline",
            "selector_base_eval_run_id": "",
            "execution_baseline_id": "execution-baseline",
            "execution_base_eval_run_id": "",
            "budget_baseline_id": "budget-baseline",
            "budget_base_eval_run_id": "",
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

    def test_build_cutover_readiness_request_includes_thresholds(self):
        payload = runner.build_cutover_readiness_request(self.make_args())

        self.assertEqual(payload["candidate_id"], "candidate-1")
        self.assertEqual(payload["required_consecutive_runs"], 2)
        self.assertEqual(payload["selector"]["thresholds"]["min_route_compatible_rate"], 0.95)
        self.assertEqual(payload["execution"]["thresholds"]["max_verification_pass_rate_drop"], 0.0)
        self.assertEqual(payload["budget"]["thresholds"]["allowed_final_native_tools"], ["exec"])

    def test_run_cutover_readiness_workflow_returns_wrapped_report(self):
        client = StubHarnessClient()
        report = runner.run_cutover_readiness_workflow(client, self.make_args())

        self.assertEqual(report["status"], "not_ready")
        self.assertFalse(report["ready"])
        self.assertEqual(report["cutover_readiness"]["candidate_id"], "candidate-1")
        self.assertEqual(client.requests[0]["candidate_id"], "candidate-1")

    def test_harness_client_uses_configured_request_timeout(self):
        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def read(self):
                return b'{"ready": false}'

        captured = {}

        def fake_urlopen(req, timeout=0):
            captured["timeout"] = timeout
            return FakeResponse()

        client = runner.HarnessClient(
            "http://127.0.0.1:18080/api/v1",
            bearer_token="token",
            request_timeout_seconds=91.5,
        )

        with mock.patch.object(runner.request, "urlopen", side_effect=fake_urlopen):
            response = client.evaluate_cutover_readiness({"candidate_id": "candidate-1"})

        self.assertEqual(captured["timeout"], 91.5)
        self.assertFalse(response["ready"])

    def test_build_markdown_report_includes_lane_summaries(self):
        markdown = runner.build_markdown_report(
            {
                "generated_at": "2026-03-28T00:00:00Z",
                "status": "not_ready",
                "ready": False,
                "cutover_readiness": StubHarnessClient().evaluate_cutover_readiness({}),
            }
        )

        self.assertIn("# Cutover Readiness Report", markdown)
        self.assertIn("Blocking Reasons", markdown)
        self.assertIn("budget lane has only 1/2 consecutive passes", markdown)
        self.assertIn("## Selector Lane", markdown)
        self.assertIn("Consecutive Green Count: 2/2", markdown)
        self.assertIn("budget-2 | passed=no", markdown)


if __name__ == "__main__":
    unittest.main()
