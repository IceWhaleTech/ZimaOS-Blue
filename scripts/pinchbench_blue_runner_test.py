import json
import sqlite3
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace


SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import pinchbench_blue_runner as runner


class StubBlueClient:
    def __init__(self):
        self.created = []
        self.sent = []
        self.messages = {}

    def create_conversation(self, title: str, timeout: float = 30.0):
        conv_id = f"conv-{len(self.created) + 1}"
        self.created.append((conv_id, title, timeout))
        self.messages.setdefault(conv_id, [])
        return {"id": conv_id}

    def send_message(
        self,
        conversation_id: str,
        message: str,
        provider: str,
        model: str,
        timeout: float,
        *,
        web_search_enabled=None,
        deep_research_enabled=None,
    ):
        self.sent.append(
            (
                conversation_id,
                message,
                provider,
                model,
                timeout,
                web_search_enabled,
                deep_research_enabled,
            )
        )
        self.messages.setdefault(conversation_id, []).extend(
            [
                {"role": "user", "content": message},
                {"role": "assistant", "content": f"stored:{message}"},
            ]
        )
        return {"id": f"msg-{len(self.sent)}", "content": f"stored:{message}"}

    def get_messages(self, conversation_id: str, timeout: float = 30.0):
        return list(self.messages.get(conversation_id, []))

    def delete_conversation(self, conversation_id: str, timeout: float = 10.0):
        self.messages.pop(conversation_id, None)


class StubJudgeBlueClient(StubBlueClient):
    def __init__(self, judge_responses):
        super().__init__()
        self.judge_responses = list(judge_responses)
        self.deleted = []

    def send_message(
        self,
        conversation_id: str,
        message: str,
        provider: str,
        model: str,
        timeout: float,
        *,
        web_search_enabled=None,
        deep_research_enabled=None,
    ):
        payload = self.judge_responses[len(self.sent)]
        self.sent.append(
            (
                conversation_id,
                message,
                provider,
                model,
                timeout,
                web_search_enabled,
                deep_research_enabled,
            )
        )
        self.messages[conversation_id] = [
            {"role": "user", "content": message},
            {"role": "assistant", "content": payload},
        ]
        return {"id": f"msg-{len(self.sent)}", "content": payload}

    def delete_conversation(self, conversation_id: str, timeout: float = 10.0):
        self.deleted.append((conversation_id, timeout))
        super().delete_conversation(conversation_id, timeout)


class LaggyJudgeBlueClient(StubJudgeBlueClient):
    def __init__(self, judge_responses, hidden_reads: int = 1):
        super().__init__(judge_responses)
        self.hidden_reads = hidden_reads
        self.read_counts = {}

    def get_messages(self, conversation_id: str, timeout: float = 30.0):
        count = self.read_counts.get(conversation_id, 0) + 1
        self.read_counts[conversation_id] = count
        messages = list(self.messages.get(conversation_id, []))
        if count <= self.hidden_reads and len(messages) > 1:
            return messages[:1]
        return messages


class PinchBenchBlueRunnerTest(unittest.TestCase):
    def make_task(self):
        return SimpleNamespace(
            name="Second Brain Knowledge Persistence",
            task_id="task_22_second_brain",
            timeout_seconds=30,
            workspace_files=[],
            prompt="unused",
            frontmatter={
                "sessions": [
                    {"id": "store_knowledge", "prompt": "Save this to `memory/MEMORY.md`."},
                    {"id": "conversation", "prompt": "What is my project called?"},
                    {
                        "id": "new_session_recall",
                        "new_session": True,
                        "prompt": "Read `memory/MEMORY.md` and answer all 5 questions.",
                    },
                ]
            },
        )

    def test_extract_session_specs_preserves_new_session_metadata(self):
        specs = runner.extract_session_specs(self.make_task())
        self.assertEqual(
            specs,
            [
                {"id": "store_knowledge", "prompt": "Save this to `memory/MEMORY.md`.", "new_session": False},
                {"id": "conversation", "prompt": "What is my project called?", "new_session": False},
                {
                    "id": "new_session_recall",
                    "prompt": "Read `memory/MEMORY.md` and answer all 5 questions.",
                    "new_session": True,
                },
            ],
        )

    def test_execute_task_starts_new_conversation_when_requested(self):
        client = StubBlueClient()
        task = self.make_task()
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            result = runner.execute_task(
                client=client,
                task=task,
                pinchbench_dir=tmp_path,
                workspace_dir=tmp_path / "workspace",
                provider="",
                model="test-model",
                timeout_multiplier=1.0,
                blue_audit_db_paths=[],
            )

        self.assertEqual(result["status"], "success")
        self.assertEqual(result["conversation_ids"], ["conv-1", "conv-2"])
        self.assertEqual([item[0] for item in client.sent], ["conv-1", "conv-1", "conv-2"])
        self.assertTrue(all(item[5] is True for item in client.sent))
        self.assertTrue(all(item[6] is False for item in client.sent))
        self.assertEqual(
            result["session_runs"],
            [
                {"conversation_id": "conv-1", "session_ids": ["store_knowledge", "conversation"]},
                {"conversation_id": "conv-2", "session_ids": ["new_session_recall"]},
            ],
        )
        self.assertTrue(
            all(item[4] >= runner.MIN_MESSAGE_TRANSPORT_TIMEOUT_SECONDS for item in client.sent)
        )

    def test_resolve_message_transport_timeout_adds_grace_beyond_remaining_budget(self):
        self.assertEqual(
            runner.resolve_message_transport_timeout(30.0),
            runner.MIN_MESSAGE_TRANSPORT_TIMEOUT_SECONDS,
        )
        self.assertEqual(
            runner.resolve_message_transport_timeout(120.0),
            240.0,
        )
        self.assertEqual(
            runner.resolve_message_transport_timeout(900.0),
            runner.MAX_MESSAGE_TRANSPORT_TIMEOUT_SECONDS,
        )

    def test_resolve_blue_runtime_workspace_dir_prefers_agentcore_config(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            db_path = Path(tmp_dir) / "blue.db"
            conn = sqlite3.connect(db_path)
            conn.execute("CREATE TABLE kvstore (key TEXT PRIMARY KEY, value TEXT)")
            conn.execute(
                "INSERT INTO kvstore(key, value) VALUES(?, ?)",
                (
                    "config:app:claudecode",
                    json.dumps({"workspace_dir": str(Path(tmp_dir) / "legacy-workspace")}),
                ),
            )
            conn.execute(
                "INSERT INTO kvstore(key, value) VALUES(?, ?)",
                (
                    "config:app:agentcore",
                    json.dumps({"workspace_dir": str(Path(tmp_dir) / "agentcore-workspace")}),
                ),
            )
            conn.commit()
            conn.close()

            resolved = runner.resolve_blue_runtime_workspace_dir(db_path)

        self.assertEqual(resolved, (Path(tmp_dir) / "agentcore-workspace").resolve(strict=False))

    def test_resolve_blue_runtime_workspace_dir_falls_back_to_legacy_keys(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            db_path = Path(tmp_dir) / "blue.db"
            conn = sqlite3.connect(db_path)
            conn.execute("CREATE TABLE kvstore (key TEXT PRIMARY KEY, value TEXT)")
            conn.execute(
                "INSERT INTO kvstore(key, value) VALUES(?, ?)",
                (
                    "config:app:claude_code_cli",
                    json.dumps(
                        {
                            "backend": {
                                "workspace_dir": str(Path(tmp_dir) / "legacy-cli-workspace")
                            }
                        }
                    ),
                ),
            )
            conn.commit()
            conn.close()

            resolved = runner.resolve_blue_runtime_workspace_dir(db_path)

        self.assertEqual(
            resolved,
            (Path(tmp_dir) / "legacy-cli-workspace").resolve(strict=False),
        )

    def test_resolve_blue_audit_db_paths_includes_jsonl_audit_dir_beside_blue_db(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            db_path = root / "blue.db"
            db_path.touch()
            audit_dir = root / "session_audit_logs"
            audit_dir.mkdir()

            resolved = runner.resolve_blue_audit_db_paths("", db_path)

        self.assertIn(audit_dir.resolve(strict=False), resolved)

    def test_load_tool_audit_rows_reads_jsonl_audit_logs(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            audit_dir = Path(tmp_dir) / "session_audit_logs"
            audit_dir.mkdir()
            conversation_id = "conv-123"
            log_path = audit_dir / f"{conversation_id}-part1.jsonl"
            log_path.write_text(
                "\n".join(
                    [
                        json.dumps(
                            {
                                "created_at": "2026-04-10T02:59:29.330038+08:00",
                                "event_type": "assistant_tool_call",
                                "role": "assistant",
                                "tool_call_id": "call_1",
                                "tool_name": "exec",
                                "payload": json.dumps({"command": "cat report.txt"}),
                            }
                        ),
                        json.dumps(
                            {
                                "created_at": "2026-04-10T02:59:29.430038+08:00",
                                "event_type": "tool_result",
                                "role": "tool",
                                "tool_call_id": "call_1",
                                "tool_name": "exec",
                                "payload": json.dumps({"stdout": "hello"}),
                            }
                        ),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            rows = runner.load_tool_audit_rows([audit_dir], conversation_id)

        self.assertEqual(
            rows,
            [
                {
                    "created_at": "2026-04-10T02:59:29.330038+08:00",
                    "event_type": "assistant_tool_call",
                    "role": "assistant",
                    "tool_call_id": "call_1",
                    "tool_name": "exec",
                    "payload": json.dumps({"command": "cat report.txt"}),
                },
                {
                    "created_at": "2026-04-10T02:59:29.430038+08:00",
                    "event_type": "tool_result",
                    "role": "tool",
                    "tool_call_id": "call_1",
                    "tool_name": "exec",
                    "payload": json.dumps({"stdout": "hello"}),
                },
            ],
        )

    def test_augment_transcript_with_audit_injects_jsonl_tool_calls(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            audit_dir = Path(tmp_dir) / "session_audit_logs"
            audit_dir.mkdir()
            conversation_id = "conv-456"
            log_path = audit_dir / f"{conversation_id}-part1.jsonl"
            log_path.write_text(
                "\n".join(
                    [
                        json.dumps(
                            {
                                "created_at": "2026-04-10T02:59:29.330038+08:00",
                                "event_type": "assistant_tool_call",
                                "role": "assistant",
                                "tool_call_id": "call_1",
                                "tool_name": "exec",
                                "payload": json.dumps({"command": "cat report.txt"}),
                            }
                        ),
                        json.dumps(
                            {
                                "created_at": "2026-04-10T02:59:29.430038+08:00",
                                "event_type": "tool_result",
                                "role": "tool",
                                "tool_call_id": "call_1",
                                "tool_name": "exec",
                                "payload": json.dumps({"stdout": "hello"}),
                            }
                        ),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            transcript = [
                {"type": "message", "message": {"role": "user", "content": ["Do the task"]}},
                {"type": "message", "message": {"role": "assistant", "content": []}},
            ]

            augmented = runner.augment_transcript_with_audit(
                transcript,
                db_paths=[audit_dir],
                conversation_id=conversation_id,
            )

        self.assertEqual(len(augmented), 4)
        self.assertEqual(
            augmented[1]["message"]["content"][0],
            {
                "type": "toolCall",
                "id": "call_1",
                "name": "exec",
                "arguments": {"command": "cat report.txt"},
                "params": {"command": "cat report.txt"},
            },
        )
        self.assertEqual(
            augmented[2]["message"],
            {
                "role": "toolResult",
                "toolCallId": "call_1",
                "toolName": "exec",
                "content": [json.dumps({"stdout": "hello"})],
            },
        )

    def test_load_tool_audit_rows_dedupes_sqlite_and_jsonl_variants_of_same_event(self):
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            audit_db = root / "session_audit.db"
            audit_dir = root / "session_audit_logs"
            audit_dir.mkdir()
            conversation_id = "conv-dedupe"

            conn = sqlite3.connect(audit_db)
            try:
                conn.execute(
                    """
                    CREATE TABLE session_tool_audit_logs (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        conversation_id TEXT,
                        created_at TEXT,
                        event_type TEXT,
                        role TEXT,
                        tool_call_id TEXT,
                        tool_name TEXT,
                        payload TEXT
                    )
                    """
                )
                conn.executemany(
                    """
                    INSERT INTO session_tool_audit_logs
                    (conversation_id, created_at, event_type, role, tool_call_id, tool_name, payload)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                    """,
                    [
                        (
                            conversation_id,
                            "2026-04-10T10:18:43.20036Z",
                            "assistant_tool_call",
                            "assistant",
                            "call_1",
                            "exec",
                            json.dumps({"command": "blue /install humanizer"}),
                        ),
                        (
                            conversation_id,
                            "2026-04-10T10:18:43.40036Z",
                            "tool_result",
                            "tool",
                            "call_1",
                            "exec",
                            json.dumps(
                                {
                                    "command": "blue /install humanizer",
                                    "exit_code": 1,
                                    "stdout": "Error: install failed: skill already installed\n",
                                }
                            ),
                        ),
                    ],
                )
                conn.commit()
            finally:
                conn.close()

            (audit_dir / f"{conversation_id}-part1.jsonl").write_text(
                "\n".join(
                    [
                        json.dumps(
                            {
                                "conversation_id": conversation_id,
                                "created_at": "2026-04-10T18:18:43.20036+08:00",
                                "event_type": "assistant_tool_call",
                                "role": "assistant",
                                "tool_call_id": "call_1",
                                "tool_name": "exec",
                                "payload": json.dumps({"command": "blue /install humanizer"}),
                            }
                        ),
                        json.dumps(
                            {
                                "conversation_id": conversation_id,
                                "created_at": "2026-04-10T18:18:43.40036+08:00",
                                "event_type": "tool_result",
                                "role": "tool",
                                "tool_call_id": "call_1",
                                "tool_name": "exec",
                                "payload": json.dumps(
                                    {
                                        "stdout": "Error: install failed: skill already installed\n",
                                        "exit_code": 1,
                                        "command": "blue /install humanizer",
                                    }
                                ),
                            }
                        ),
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            rows = runner.load_tool_audit_rows([audit_db, audit_dir], conversation_id)

        self.assertEqual(
            rows,
            [
                {
                    "created_at": "2026-04-10T10:18:43.20036Z",
                    "event_type": "assistant_tool_call",
                    "role": "assistant",
                    "tool_call_id": "call_1",
                    "tool_name": "exec",
                    "payload": json.dumps({"command": "blue /install humanizer"}),
                },
                {
                    "created_at": "2026-04-10T10:18:43.40036Z",
                    "event_type": "tool_result",
                    "role": "tool",
                    "tool_call_id": "call_1",
                    "tool_name": "exec",
                    "payload": json.dumps(
                        {
                            "command": "blue /install humanizer",
                            "exit_code": 1,
                            "stdout": "Error: install failed: skill already installed\n",
                        }
                    ),
                },
            ],
        )

    def test_convert_blue_messages_to_transcript_maps_web_query_to_web_search_alias(self):
        transcript = runner.convert_blue_messages_to_transcript(
            [
                {
                    "role": "assistant",
                    "content": "",
                    "tool_calls": [
                        {
                            "id": "call-1",
                            "name": "web_query",
                            "arguments": json.dumps({"input": "latest observability market leaders"}),
                        }
                    ],
                },
                {
                    "role": "tool",
                    "tool_call_id": "call-1",
                    "tool_name": "web_query",
                    "content": json.dumps({"status": "ok"}),
                },
            ]
        )

        self.assertEqual(
            transcript[0]["message"]["content"][0],
            {
                "type": "toolCall",
                "id": "call-1",
                "name": "web_search",
                "arguments": {"input": "latest observability market leaders"},
                "params": {"input": "latest observability market leaders"},
                "canonical_name": "web_query",
            },
        )
        self.assertEqual(
            transcript[1]["message"],
            {
                "role": "toolResult",
                "toolCallId": "call-1",
                "toolName": "web_search",
                "canonicalToolName": "web_query",
                "content": [json.dumps({"status": "ok"})],
            },
        )

    def test_build_audit_transcript_entries_maps_web_query_to_web_search_alias(self):
        entries = runner.build_audit_transcript_entries(
            [
                {
                    "event_type": "assistant_tool_call",
                    "tool_call_id": "call-1",
                    "tool_name": "web_query",
                    "payload": json.dumps({"input": "latest observability market leaders"}),
                },
                {
                    "event_type": "tool_result",
                    "tool_call_id": "call-1",
                    "tool_name": "web_query",
                    "payload": json.dumps({"status": "ok"}),
                },
            ]
        )

        self.assertEqual(
            entries[0]["message"]["content"][0],
            {
                "type": "toolCall",
                "id": "call-1",
                "name": "web_search",
                "arguments": {"input": "latest observability market leaders"},
                "params": {"input": "latest observability market leaders"},
                "canonical_name": "web_query",
            },
        )
        self.assertEqual(
            entries[1]["message"],
            {
                "role": "toolResult",
                "toolCallId": "call-1",
                "toolName": "web_search",
                "canonicalToolName": "web_query",
                "content": [json.dumps({"status": "ok"})],
            },
        )

    def test_blue_judge_runner_retries_empty_object_response(self):
        client = StubJudgeBlueClient(
            [
                "{}",
                json.dumps({"scores": {"completion": 1.0}, "total": 0.9, "notes": "solid"}),
            ]
        )
        judge = runner.BlueJudgeRunner(client, "pinchbench-fixed", "claude-sonnet-4.6")

        with tempfile.TemporaryDirectory() as tmp_dir:
            result = judge.run_prompt(
                prompt="Judge this task output.",
                workspace=Path(tmp_dir) / "judge-workspace",
                timeout_seconds=30.0,
            )

        self.assertEqual(len(client.created), 2)
        self.assertEqual(len(client.deleted), 2)
        self.assertEqual(result["status"], "success")
        self.assertEqual(
            result["transcript"][-1]["message"]["content"],
            [
                {
                    "type": "text",
                    "text": json.dumps(
                        {"scores": {"completion": 1.0}, "total": 0.9, "notes": "solid"}
                    ),
                }
            ],
        )

    def test_blue_judge_runner_waits_for_delayed_assistant_visibility(self):
        payload = json.dumps({"scores": {"completion": 1.0}, "total": 0.8, "notes": "stable"})
        client = LaggyJudgeBlueClient([payload], hidden_reads=1)
        judge = runner.BlueJudgeRunner(client, "pinchbench-fixed", "claude-sonnet-4.6")

        with tempfile.TemporaryDirectory() as tmp_dir:
            result = judge.run_prompt(
                prompt="Judge this task output.",
                workspace=Path(tmp_dir) / "judge-workspace",
                timeout_seconds=30.0,
            )

        self.assertEqual(len(client.created), 1)
        self.assertEqual(len(client.deleted), 1)
        self.assertGreaterEqual(client.read_counts.get("conv-1", 0), 2)
        self.assertEqual(result["status"], "success")
        self.assertEqual(runner.extract_latest_assistant_text(result["transcript"]), payload)

    def test_persist_results_payload_to_db_writes_run_task_and_breakdown_rows(self):
        payload = {
            "runner": "blue",
            "generated_at": "2026-04-10T07:55:57Z",
            "blue_base_url": "http://127.0.0.1:18080/api/v1",
            "workspace_dir": "/tmp/pinchbench-workspace",
            "judge_mode": "judge-on",
            "provider": "pinchbench-fixed",
            "model": "claude-sonnet-4.6",
            "judge_provider": "pinchbench-fixed",
            "judge_model": "claude-sonnet-4.6",
            "suite": "task_14_humanizer",
            "skip_judge": False,
            "summary": {
                "task_count": 1,
                "success_count": 1,
                "failure_count": 0,
                "average_score": 0.885,
                "graded_count": 1,
            },
            "results": [
                {
                    "task_id": "task_14_humanizer",
                    "task_name": "Humanize AI-Generated Blog",
                    "category": "writing",
                    "grading_type": "hybrid",
                    "execution": {
                        "status": "success",
                        "conversation_id": "conv-123",
                        "stdout": "",
                    },
                    "grade": {
                        "score": 0.885,
                        "breakdown": {
                            "Skill Usage or Manual Rewrite": 0.75,
                            "Task Completion": 1.0,
                        },
                        "notes": "Strong fallback rewrite after install attempt.",
                    },
                    "grade_error": None,
                }
            ],
        }

        with tempfile.TemporaryDirectory() as tmp_dir:
            db_path = Path(tmp_dir) / "pinchbench_results.db"
            run_id = runner.persist_results_payload_to_db(db_path, payload)

            self.assertTrue(run_id)
            conn = sqlite3.connect(db_path)
            try:
                run_row = conn.execute(
                    "SELECT suite, judge_mode, model, skip_judge FROM runs WHERE run_id = ?",
                    (run_id,),
                ).fetchone()
                self.assertEqual(run_row, ("task_14_humanizer", "judge-on", "claude-sonnet-4.6", 0))

                task_row = conn.execute(
                    "SELECT task_name, execution_status, execution_conversation_id, grade_score FROM task_results WHERE run_id = ? AND task_id = ?",
                    (run_id, "task_14_humanizer"),
                ).fetchone()
                self.assertEqual(
                    task_row,
                    ("Humanize AI-Generated Blog", "success", "conv-123", 0.885),
                )

                breakdown_rows = conn.execute(
                    "SELECT criterion_name, score FROM criterion_scores WHERE run_id = ? AND task_id = ? ORDER BY criterion_name",
                    (run_id, "task_14_humanizer"),
                ).fetchall()
                self.assertEqual(
                    breakdown_rows,
                    [
                        ("Skill Usage or Manual Rewrite", 0.75),
                        ("Task Completion", 1.0),
                    ],
                )
            finally:
                conn.close()


if __name__ == "__main__":
    unittest.main()
