import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import pinchbench_catalog_seed as seed


SAMPLE_RUNS_HTML = """
<html><body>
["$","tr","row-1",{"className":"border-b","children":[
["$","td",null,{"className":"py-2 pr-4","children":["$","$L8",null,{"href":"/submission/1aff7c69-35c5-439e-ba52-4768586cc773","className":"inline-flex items-center gap-2 font-mono text-foreground hover:underline","children":["anthropic/claude-opus-4.6",false]}]}],
["$","td",null,{"className":"py-2 pr-4","children":["$","span",null,{"style":{"color":"#d97757"},"children":"anthropic"}]}],
["$","td",null,{"className":"py-2 pr-4 text-right font-mono","children":["93.3","%"]}]
]}]
["$","tr","row-2",{"className":"border-b","children":[
["$","td",null,{"className":"py-2 pr-4","children":["$","$L8",null,{"href":"/submission/5d73c775-fb81-4df1-ac2f-a08434541601","className":"inline-flex items-center gap-2 font-mono text-foreground hover:underline","children":["openai/gpt-5.4",false]}]}],
["$","td",null,{"className":"py-2 pr-4","children":["$","span",null,{"style":{"color":"#10a37f"},"children":"openai"}]}],
["$","td",null,{"className":"py-2 pr-4 text-right font-mono","children":["90.5","%"]}]
]}]
</body></html>
"""

SAMPLE_ESCAPED_RUNS_HTML = r"""
<html><body>
{\"href\":\"/submission/f29c3e4c-b73c-4fa9-9c8d-2eb0afb050c8\",\"className\":\"inline-flex items-center gap-2 font-mono text-foreground hover:underline\",\"children\":[\"anthropic/claude-haiku-4.5\",false]}
{\"style\":{\"color\":\"#d97757\"},\"children\":\"anthropic\"}
{\"className\":\"py-2 pr-4 text-right font-mono\",\"children\":[\"89.5\",\"%\"]}
</body></html>
"""


class PinchBenchCatalogSeedTest(unittest.TestCase):
    def test_parse_runs_html_extracts_verified_rows(self):
        rows = seed.parse_runs_html(SAMPLE_RUNS_HTML)

        self.assertEqual(
            rows,
            [
                {
                    "id": "anthropic/claude-opus-4.6",
                    "provider": "anthropic",
                    "score": 93.3,
                    "url": "https://pinchbench.com/submission/1aff7c69-35c5-439e-ba52-4768586cc773",
                },
                {
                    "id": "openai/gpt-5.4",
                    "provider": "openai",
                    "score": 90.5,
                    "url": "https://pinchbench.com/submission/5d73c775-fb81-4df1-ac2f-a08434541601",
                },
            ],
        )

    def test_parse_runs_html_handles_escaped_live_page_fragments(self):
        rows = seed.parse_runs_html(SAMPLE_ESCAPED_RUNS_HTML)

        self.assertEqual(
            rows,
            [
                {
                    "id": "anthropic/claude-haiku-4.5",
                    "provider": "anthropic",
                    "score": 89.5,
                    "url": "https://pinchbench.com/submission/f29c3e4c-b73c-4fa9-9c8d-2eb0afb050c8",
                }
            ],
        )

    def test_compare_against_catalog_summarizes_verified_url_changes(self):
        rows = seed.parse_runs_html(SAMPLE_RUNS_HTML)

        with tempfile.TemporaryDirectory() as tmp_dir:
            catalog_path = Path(tmp_dir) / "provider_catalog.json"
            catalog_path.write_text(
                json.dumps(
                    {
                        "pinchbench_models": {
                            "anthropic": [
                                {
                                    "id": "anthropic/claude-opus-4.6",
                                    "pinchbench_score": 93.3,
                                    "pinchbench_url": "https://pinchbench.com/submission/1aff7c69-35c5-439e-ba52-4768586cc773",
                                }
                            ],
                            "openai": [
                                {
                                    "id": "openai/gpt-5.4",
                                    "pinchbench_score": 90.5
                                }
                            ],
                            "qwen": [
                                {
                                    "id": "qwen/qwen3.5-27b",
                                    "pinchbench_score": 90.0,
                                    "pinchbench_url": "https://pinchbench.com/submission/old-qwen-url",
                                }
                            ],
                        }
                    }
                ),
                encoding="utf-8",
            )

            summary = seed.compare_rows_to_catalog(rows, catalog_path)

        self.assertEqual(summary["fetched_rows"], 2)
        self.assertEqual(summary["catalog_verified_rows"], 2)
        self.assertEqual(summary["matching_verified_ids"], ["anthropic/claude-opus-4.6"])
        self.assertEqual(summary["newly_verified_ids"], ["openai/gpt-5.4"])
        self.assertEqual(summary["catalog_only_verified_ids"], ["qwen/qwen3.5-27b"])
        self.assertEqual(summary["changed_verified_ids"], [])

    def test_compare_against_catalog_reports_url_changes_for_same_model(self):
        rows = [
            {
                "id": "minimax/minimax-m2.1",
                "provider": "minimax",
                "score": 84.3,
                "url": "https://pinchbench.com/submission/new-minimax-url",
            }
        ]

        with tempfile.TemporaryDirectory() as tmp_dir:
            catalog_path = Path(tmp_dir) / "provider_catalog.json"
            catalog_path.write_text(
                json.dumps(
                    {
                        "pinchbench_models": {
                            "minimax": [
                                {
                                    "id": "minimax/minimax-m2.1",
                                    "pinchbench_score": 88.4,
                                    "pinchbench_url": "https://pinchbench.com/submission/old-minimax-url",
                                }
                            ]
                        }
                    }
                ),
                encoding="utf-8",
            )

            summary = seed.compare_rows_to_catalog(rows, catalog_path)

        self.assertEqual(summary["matching_verified_ids"], [])
        self.assertEqual(summary["newly_verified_ids"], [])
        self.assertEqual(summary["catalog_only_verified_ids"], [])
        self.assertEqual(
            summary["changed_verified_ids"],
            [
                {
                    "id": "minimax/minimax-m2.1",
                    "catalog_url": "https://pinchbench.com/submission/old-minimax-url",
                    "fetched_url": "https://pinchbench.com/submission/new-minimax-url",
                }
            ],
        )

    def test_format_go_seed_entries_emits_copyable_seed_lines(self):
        rows = seed.parse_runs_html(SAMPLE_RUNS_HTML)

        rendered = seed.format_go_seed_entries(rows)

        self.assertIn('{ID: "anthropic/claude-opus-4.6", Score: 93.3, URL: "https://pinchbench.com/submission/1aff7c69-35c5-439e-ba52-4768586cc773"},', rendered)
        self.assertIn('{ID: "openai/gpt-5.4", Score: 90.5, URL: "https://pinchbench.com/submission/5d73c775-fb81-4df1-ac2f-a08434541601"},', rendered)


if __name__ == "__main__":
    unittest.main()
