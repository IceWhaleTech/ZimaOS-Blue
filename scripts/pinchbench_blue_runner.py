#!/usr/bin/env python3
"""
Blue-compatible PinchBench runner.

This script reuses the official PinchBench task loader and grading logic, but
executes tasks through Blue's HTTP conversation API instead of the OpenClaw CLI.

Notes:
- Blue must already be running.
- For runs that should bypass chat prompt interception, start Blue with
  `blue --no-intercept`.
- Blue's configured workspace directory should match --workspace-dir.
- The execution provider/model must already be available in Blue.
- Judge grading can also be routed through Blue; disable it with --skip-judge
  when you only want automated checks or when the upstream judge model is down.
"""

from __future__ import annotations

import argparse
import http.client
import importlib
import json
import logging
import os
import shutil
import sqlite3
import sys
import time
import uuid
from dataclasses import asdict, is_dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence
from urllib import error, request


LOG = logging.getLogger("pinchbench-blue")

MIN_MESSAGE_TRANSPORT_TIMEOUT_SECONDS = 180.0
MAX_MESSAGE_TRANSPORT_TIMEOUT_SECONDS = 600.0
MESSAGE_TRANSPORT_TIMEOUT_GRACE_SECONDS = 120.0
EMPTY_JUDGE_RESPONSE_MAX_RETRIES = 1
JUDGE_MESSAGE_VISIBILITY_TIMEOUT_SECONDS = 5.0
JUDGE_MESSAGE_POLL_INTERVAL_SECONDS = 0.25
EXECUTION_MESSAGE_VISIBILITY_TIMEOUT_SECONDS = 120.0
PINCHBENCH_TOOL_NAME_ALIASES = {
    "web_query": "web_search",
}


def pinchbench_display_tool_name(name: Any) -> str:
    original = str(name or "")
    return PINCHBENCH_TOOL_NAME_ALIASES.get(original, original)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run PinchBench against Blue.")
    parser.add_argument(
        "--pinchbench-dir",
        required=True,
        help="Path to the official PinchBench checkout",
    )
    parser.add_argument(
        "--blue-base-url",
        default="http://127.0.0.1:18080/api/v1",
        help="Blue API base URL",
    )
    parser.add_argument(
        "--api-key",
        default=os.environ.get("BLUE_API_KEY", ""),
        help="Optional Blue API key used for authenticated local API access",
    )
    parser.add_argument(
        "--workspace-dir",
        default="/tmp/pinchbench-workspace",
        help="Workspace directory that Blue is configured to use",
    )
    parser.add_argument(
        "--provider",
        default="",
        help="Optional Blue provider id to pin for task execution. Leave empty to let Blue route across the provider pool.",
    )
    parser.add_argument(
        "--model",
        required=True,
        help="Blue model id to use for task execution",
    )
    parser.add_argument(
        "--suite",
        default="all",
        help='Task selection: "all", "automated-only", or comma-separated task IDs',
    )
    parser.add_argument(
        "--output",
        default="docs/reports/pinchbench_blue_results.json",
        help="Where to write the JSON results",
    )
    parser.add_argument(
        "--output-db",
        default="",
        help="Optional SQLite path for structured run/task result persistence",
    )
    parser.add_argument(
        "--blue-db-path",
        default="",
        help="Optional path to Blue's SQLite database. When available, tool audit logs are merged back into the transcript so graders can see tool calls.",
    )
    parser.add_argument(
        "--blue-audit-db-path",
        default="",
        help="Optional path to Blue's session audit SQLite database. Defaults to auto-detecting session_audit.db beside blue.db.",
    )
    parser.add_argument(
        "--timeout-multiplier",
        type=float,
        default=1.0,
        help="Scale PinchBench task timeouts",
    )
    parser.add_argument(
        "--judge-provider",
        default=None,
        help="Blue provider id for LLM judge runs (defaults to --provider)",
    )
    parser.add_argument(
        "--judge-model",
        default=None,
        help="Blue model id for LLM judge runs (defaults to --model)",
    )
    parser.add_argument(
        "--skip-judge",
        action="store_true",
        help="Skip LLM-judge grading and run automated checks only",
    )
    parser.add_argument(
        "--no-fail-fast",
        action="store_true",
        help="Continue after task execution errors",
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Enable verbose logging",
    )
    return parser.parse_args()


def load_pinchbench_modules(pinchbench_dir: Path):
    scripts_dir = pinchbench_dir / "scripts"
    if not scripts_dir.exists():
        raise FileNotFoundError(f"PinchBench scripts directory not found: {scripts_dir}")
    sys.path.insert(0, str(scripts_dir))
    lib_tasks = importlib.import_module("lib_tasks")
    lib_grading = importlib.import_module("lib_grading")
    return lib_tasks, lib_grading


class BlueAPIError(RuntimeError):
    pass


class BlueClient:
    def __init__(self, base_url: str, api_key: str = ""):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key.strip()

    def create_conversation(self, title: str, timeout: float = 30.0) -> Dict[str, Any]:
        return self._json_request("POST", "/conversations", {"title": title}, timeout=timeout)

    def send_message(
        self,
        conversation_id: str,
        message: str,
        provider: str,
        model: str,
        timeout: float,
        *,
        web_search_enabled: Optional[bool] = None,
        deep_research_enabled: Optional[bool] = None,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {
            "message": message,
            "model": model,
        }
        if provider:
            payload["provider"] = provider
        if web_search_enabled is not None:
            payload["web_search_enabled"] = web_search_enabled
        if deep_research_enabled is not None:
            payload["deep_research_enabled"] = deep_research_enabled
        return self._json_request(
            "POST",
            f"/conversations/{conversation_id}/messages",
            payload,
            timeout=timeout,
        )

    def get_messages(self, conversation_id: str, timeout: float = 30.0) -> List[Dict[str, Any]]:
        data = self._json_request(
            "GET",
            f"/conversations/{conversation_id}/messages?limit=1000",
            None,
            timeout=timeout,
        )
        if isinstance(data, list):
            return data
        raise BlueAPIError(f"Unexpected messages payload: {type(data).__name__}")

    def delete_conversation(self, conversation_id: str, timeout: float = 30.0) -> None:
        self._json_request("DELETE", f"/conversations/{conversation_id}", None, timeout=timeout)

    def _json_request(
        self,
        method: str,
        path: str,
        payload: Optional[Dict[str, Any]],
        *,
        timeout: float,
    ) -> Any:
        body = None
        headers = {
            "Accept": "application/json",
            # Avoid fragile keep-alive reuse during long multi-task bench runs.
            "Connection": "close",
        }
        if self.api_key:
            headers["X-API-Key"] = self.api_key
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        max_attempts = 2 if method.upper() in {"GET", "DELETE"} else 1
        for attempt in range(1, max_attempts + 1):
            req = request.Request(f"{self.base_url}{path}", data=body, method=method, headers=headers)
            try:
                with request.urlopen(req, timeout=timeout) as resp:
                    raw = resp.read().decode("utf-8", errors="replace")
            except error.HTTPError as exc:
                raw = exc.read().decode("utf-8", errors="replace")
                raise BlueAPIError(f"{method} {path} failed: HTTP {exc.code}: {raw}") from exc
            except error.URLError as exc:
                if attempt < max_attempts:
                    LOG.warning(
                        "Retrying %s %s after transport error (%s/%s): %s",
                        method,
                        path,
                        attempt,
                        max_attempts,
                        exc,
                    )
                    continue
                raise BlueAPIError(f"{method} {path} failed: {exc}") from exc
            except (
                http.client.BadStatusLine,
                http.client.RemoteDisconnected,
                ConnectionResetError,
                OSError,
                TimeoutError,
            ) as exc:
                if attempt < max_attempts:
                    LOG.warning(
                        "Retrying %s %s after connection drop (%s/%s): %s",
                        method,
                        path,
                        attempt,
                        max_attempts,
                        exc,
                    )
                    continue
                raise BlueAPIError(
                    f"{method} {path} failed: {exc.__class__.__name__}: {exc}"
                ) from exc

            if not raw.strip():
                return None
            try:
                return json.loads(raw)
            except json.JSONDecodeError as exc:
                raise BlueAPIError(f"{method} {path} returned non-JSON body: {raw[:400]}") from exc

        raise BlueAPIError(f"{method} {path} failed after {max_attempts} attempts")


def select_task_ids(tasks: Sequence[Any], suite: str) -> Optional[List[str]]:
    if suite == "all":
        return None
    if suite == "automated-only":
        return [task.task_id for task in tasks if task.grading_type == "automated"]
    return [part.strip() for part in suite.split(",") if part.strip()]


def select_tasks(tasks: Sequence[Any], suite: str) -> List[Any]:
    selected_ids = select_task_ids(tasks, suite)
    if not selected_ids:
        return list(tasks)
    selected = [task for task in tasks if task.task_id in selected_ids]
    missing = sorted(set(selected_ids) - {task.task_id for task in selected})
    if missing:
        raise ValueError(f"Unknown task ids: {', '.join(missing)}")
    return selected


def selected_tasks_require_judge(tasks: Sequence[Any]) -> bool:
    for task in tasks:
        grading_type = str(getattr(task, "grading_type", "") or "").strip().lower()
        if grading_type in {"llm_judge", "hybrid"}:
            return True
    return False


def ensure_judge_compatible_selection(tasks: Sequence[Any], skip_judge: bool) -> None:
    if not skip_judge:
        return
    if not selected_tasks_require_judge(tasks):
        return
    task_ids = [getattr(task, "task_id", "") for task in tasks if str(getattr(task, "grading_type", "") or "").strip().lower() in {"llm_judge", "hybrid"}]
    raise ValueError(
        "skip-judge cannot be used for benchmark-comparable runs that include llm_judge or hybrid tasks. "
        f"Offending tasks: {', '.join(task_ids)}"
    )


def judge_mode(skip_judge: bool) -> str:
    return "judge-off" if skip_judge else "judge-on"


def resolve_output_path_with_judge_mode(output_path: Path, skip_judge: bool) -> Path:
    mode = judge_mode(skip_judge)
    stem = output_path.stem
    if stem.endswith("-judge-on") or stem.endswith("-judge-off"):
        return output_path
    suffix = output_path.suffix
    if suffix:
        return output_path.with_name(f"{stem}-{mode}{suffix}")
    return output_path.with_name(f"{output_path.name}-{mode}")


def clear_workspace(workspace_dir: Path) -> None:
    if workspace_dir.exists():
        shutil.rmtree(workspace_dir)
    workspace_dir.mkdir(parents=True, exist_ok=True)


def resolve_message_transport_timeout(remaining_seconds: float) -> float:
    remaining = max(float(remaining_seconds or 0.0), 0.0)
    timeout = max(
        MIN_MESSAGE_TRANSPORT_TIMEOUT_SECONDS,
        remaining + MESSAGE_TRANSPORT_TIMEOUT_GRACE_SECONDS,
    )
    return min(timeout, MAX_MESSAGE_TRANSPORT_TIMEOUT_SECONDS)


def prepare_workspace(task: Any, pinchbench_dir: Path, workspace_dir: Path) -> Path:
    clear_workspace(workspace_dir)
    for file_spec in task.workspace_files:
        if "content" in file_spec:
            dest = workspace_dir / file_spec["path"]
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_text(file_spec["content"], encoding="utf-8")
            continue
        if "source" in file_spec:
            source = pinchbench_dir / "assets" / file_spec["source"]
            dest = workspace_dir / file_spec["dest"]
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, dest)
            continue
        raise ValueError(f"Unsupported workspace file spec: {file_spec}")

    for bootstrap_name in ("BOOTSTRAP.md", "SOUL.md", "USER.md", "IDENTITY.md"):
        bootstrap_path = workspace_dir / bootstrap_name
        if bootstrap_path.exists():
            bootstrap_path.unlink()
    return workspace_dir


def extract_session_specs(task: Any) -> List[Dict[str, Any]]:
    sessions = task.frontmatter.get("sessions", [])
    if not sessions:
        return [{"id": "session_1", "prompt": task.prompt, "new_session": False}]

    specs: List[Dict[str, Any]] = []
    for index, session in enumerate(sessions, start=1):
        if isinstance(session, str):
            specs.append(
                {
                    "id": f"session_{index}",
                    "prompt": session,
                    "new_session": False,
                }
            )
            continue
        if isinstance(session, dict):
            prompt = session.get("prompt") or session.get("message")
            if prompt:
                specs.append(
                    {
                        "id": str(session.get("id") or f"session_{index}"),
                        "prompt": prompt,
                        "new_session": bool(session.get("new_session")),
                    }
                )
    if specs:
        return specs
    return [{"id": "session_1", "prompt": task.prompt, "new_session": False}]


def extract_prompts(task: Any) -> List[str]:
    return [item["prompt"] for item in extract_session_specs(task)]


def parse_tool_arguments(arguments: Any) -> Any:
    if not isinstance(arguments, str):
        return arguments
    text = arguments.strip()
    if not text:
        return {}
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return {"raw": arguments}


def usage_from_stats(stats: Optional[Dict[str, Any]]) -> Dict[str, Any]:
    if not isinstance(stats, dict):
        return {}
    usage = {
        "input": int(stats.get("input_tokens", 0) or 0),
        "output": int(stats.get("output_tokens", 0) or 0),
        "cacheRead": 0,
        "cacheWrite": 0,
        "totalTokens": int(stats.get("total_tokens", 0) or 0),
        "cost": {"total": 0.0},
    }
    if (
        usage["input"] == 0
        and usage["output"] == 0
        and usage["cacheRead"] == 0
        and usage["cacheWrite"] == 0
        and usage["totalTokens"] == 0
        and usage["cost"]["total"] == 0.0
    ):
        return {}
    return usage


def transcript_has_tool_calls(transcript: Sequence[Dict[str, Any]]) -> bool:
    for entry in transcript:
        if entry.get("type") != "message":
            continue
        message = entry.get("message", {})
        if message.get("role") != "assistant":
            continue
        for item in message.get("content", []):
            if item.get("type") == "toolCall":
                return True
    return False


def resolve_blue_db_path(explicit_path: str) -> Optional[Path]:
    candidates: List[Path] = []
    if explicit_path.strip():
        candidates.append(Path(explicit_path).expanduser())
    env_path = os.environ.get("BLUE_DB_PATH", "").strip()
    if env_path:
        candidates.append(Path(env_path).expanduser())
    candidates.append(Path.home() / ".zimaos-blue" / "data" / "blue.db")
    candidates.append(Path("/tmp/pinchbench-home/.zimaos-blue/data/blue.db"))

    seen: set[str] = set()
    for candidate in candidates:
        resolved = candidate.resolve(strict=False)
        key = str(resolved)
        if key in seen:
            continue
        seen.add(key)
        if resolved.exists():
            return resolved
    return None


def resolve_blue_audit_db_paths(
    explicit_audit_path: str,
    blue_db_path: Optional[Path],
) -> List[Path]:
    candidates: List[Path] = []
    if explicit_audit_path.strip():
        candidates.append(Path(explicit_audit_path).expanduser())
    env_path = os.environ.get("BLUE_AUDIT_DB_PATH", "").strip()
    if env_path:
        candidates.append(Path(env_path).expanduser())
    if blue_db_path is not None:
        candidates.append(blue_db_path)
        candidates.append(blue_db_path.with_name("session_audit.db"))
        candidates.append(blue_db_path.parent / "session_audit_logs")
    candidates.append(Path.home() / ".zimaos-blue" / "data" / "session_audit.db")
    candidates.append(Path.home() / ".zimaos-blue" / "data" / "session_audit_logs")
    candidates.append(Path("/tmp/pinchbench-home/.zimaos-blue/data/session_audit.db"))
    candidates.append(Path("/tmp/pinchbench-home/.zimaos-blue/data/session_audit_logs"))

    resolved_paths: List[Path] = []
    seen: set[str] = set()
    for candidate in candidates:
        resolved = candidate.resolve(strict=False)
        key = str(resolved)
        if key in seen or not resolved.exists():
            continue
        seen.add(key)
        resolved_paths.append(resolved)
    return resolved_paths


def _normalize_workspace_candidate(raw: Any) -> Optional[Path]:
    if not isinstance(raw, str):
        return None
    text = raw.strip()
    if not text or text == ".":
        return None
    return Path(text).expanduser().resolve(strict=False)


def resolve_blue_runtime_workspace_dir(blue_db_path: Optional[Path]) -> Optional[Path]:
    if blue_db_path is None or not blue_db_path.exists():
        return None

    query = """
        SELECT key, value
        FROM kvstore
        WHERE key IN ('config:app:agentcore', 'config:app:claudecode', 'config:app:claude_code_cli')
    """
    try:
        conn = sqlite3.connect(f"file:{blue_db_path}?mode=ro", uri=True)
    except sqlite3.Error as exc:
        LOG.debug("Failed to open Blue DB for workspace discovery at %s: %s", blue_db_path, exc)
        return None

    try:
        rows = conn.execute(query).fetchall()
    except sqlite3.Error as exc:
        LOG.debug("Failed to query Blue runtime workspace from %s: %s", blue_db_path, exc)
        conn.close()
        return None
    finally:
        try:
            conn.close()
        except sqlite3.Error:
            pass

    config_map: Dict[str, Any] = {}
    for key, value in rows:
        if not isinstance(value, str) or not value.strip():
            continue
        try:
            config_map[key] = json.loads(value)
        except json.JSONDecodeError:
            LOG.debug("Failed to decode config payload for %s in %s", key, blue_db_path)

    agentcore_cfg = config_map.get("config:app:agentcore")
    if isinstance(agentcore_cfg, dict):
        candidate = (
            _normalize_workspace_candidate(agentcore_cfg.get("WorkspaceDir"))
            or _normalize_workspace_candidate(agentcore_cfg.get("workspace_dir"))
            or _normalize_workspace_candidate(agentcore_cfg.get("workspaceDir"))
        )
        if candidate is not None:
            return candidate

    legacy_runtime_cfg = config_map.get("config:app:claudecode")
    if isinstance(legacy_runtime_cfg, dict):
        candidate = (
            _normalize_workspace_candidate(legacy_runtime_cfg.get("WorkspaceDir"))
            or _normalize_workspace_candidate(legacy_runtime_cfg.get("workspace_dir"))
            or _normalize_workspace_candidate(legacy_runtime_cfg.get("workspaceDir"))
        )
        if candidate is not None:
            return candidate

    cli_cfg = config_map.get("config:app:claude_code_cli")
    if isinstance(cli_cfg, dict):
        backend = cli_cfg.get("backend")
        if not isinstance(backend, dict):
            backend = cli_cfg.get("Backend")
        if isinstance(backend, dict):
            candidate = (
                _normalize_workspace_candidate(backend.get("workspace_dir"))
                or _normalize_workspace_candidate(backend.get("WorkspaceDir"))
                or _normalize_workspace_candidate(backend.get("workspaceDir"))
            )
            if candidate is not None:
                return candidate

    return None


def load_tool_audit_rows(db_paths: Sequence[Path], conversation_id: str) -> List[Dict[str, Any]]:
    if not db_paths or not conversation_id:
        return []
    query = """
        SELECT created_at, event_type, role, tool_call_id, tool_name, payload
        FROM session_tool_audit_logs
        WHERE conversation_id = ?
          AND event_type IN ('assistant_tool_call', 'tool_result')
        ORDER BY created_at ASC, id ASC
    """
    merged: List[Dict[str, Any]] = []
    seen: set[tuple[str, str, str, str, str, str]] = set()
    for db_path in db_paths:
        rows: List[Dict[str, Any]] = []
        if db_path.is_dir():
            rows = load_tool_audit_rows_from_jsonl_dir(db_path, conversation_id)
        else:
            uri = f"file:{db_path}?mode=ro"
            try:
                with sqlite3.connect(uri, uri=True, timeout=5.0) as conn:
                    conn.row_factory = sqlite3.Row
                    conn.execute("PRAGMA busy_timeout=5000")
                    rows = [dict(row) for row in conn.execute(query, (conversation_id,)).fetchall()]
            except sqlite3.Error as exc:
                LOG.debug("Failed to read Blue audit rows from %s: %s", db_path, exc)
                continue
        for data in rows:
            fingerprint = (
                normalize_audit_created_at(data.get("created_at")),
                str(data.get("event_type", "") or "").strip().lower(),
                str(data.get("role", "") or "").strip().lower(),
                str(data.get("tool_call_id", "") or ""),
                str(data.get("tool_name", "") or ""),
                normalize_audit_payload(data.get("payload")),
            )
            if fingerprint in seen:
                continue
            seen.add(fingerprint)
            merged.append(data)
    merged.sort(
        key=lambda row: (
            str(row.get("created_at", "") or ""),
            0 if str(row.get("event_type", "") or "").strip().lower() == "assistant_tool_call" else 1,
            str(row.get("tool_call_id", "") or ""),
        )
    )
    return merged


def normalize_audit_created_at(raw: Any) -> str:
    text = str(raw or "").strip()
    if not text:
        return ""
    candidate = text.replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(candidate)
    except ValueError:
        parsed = None
        for fmt in (
            "%Y-%m-%dT%H:%M:%S.%f%z",
            "%Y-%m-%dT%H:%M:%S%z",
            "%Y-%m-%dT%H:%M:%S.%f",
            "%Y-%m-%dT%H:%M:%S",
        ):
            try:
                parsed = datetime.strptime(candidate, fmt)
                break
            except ValueError:
                continue
        if parsed is None:
            return text
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc).isoformat()


def normalize_audit_payload(raw: Any) -> str:
    text = str(raw or "").strip()
    if not text:
        return ""
    try:
        parsed = json.loads(text)
    except json.JSONDecodeError:
        return text
    return json.dumps(parsed, sort_keys=True, ensure_ascii=False)


def load_tool_audit_rows_from_jsonl_dir(
    audit_dir: Path,
    conversation_id: str,
) -> List[Dict[str, Any]]:
    rows: List[Dict[str, Any]] = []
    try:
        log_paths = sorted(audit_dir.glob(f"{conversation_id}-*.jsonl"))
    except OSError as exc:
        LOG.debug("Failed to list Blue JSONL audit logs from %s: %s", audit_dir, exc)
        return rows

    for log_path in log_paths:
        try:
            raw_lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError as exc:
            LOG.debug("Failed to read Blue JSONL audit log %s: %s", log_path, exc)
            continue
        for raw_line in raw_lines:
            line = raw_line.strip()
            if not line:
                continue
            try:
                payload = json.loads(line)
            except json.JSONDecodeError:
                LOG.debug("Failed to decode Blue JSONL audit line from %s", log_path)
                continue
            event_type = str(payload.get("event_type", "") or "").strip().lower()
            if event_type not in {"assistant_tool_call", "tool_result"}:
                continue
            rows.append(
                {
                    "created_at": str(payload.get("created_at", "") or ""),
                    "event_type": event_type,
                    "role": str(payload.get("role", "") or ""),
                    "tool_call_id": str(payload.get("tool_call_id", "") or ""),
                    "tool_name": str(payload.get("tool_name", "") or ""),
                    "payload": str(payload.get("payload", "") or ""),
                }
            )
    return rows


def load_blue_messages_from_db(db_paths: Sequence[Path], conversation_id: str) -> List[Dict[str, Any]]:
    if not db_paths or not conversation_id:
        return []
    query = """
        SELECT role, content, tool_calls, tool_call_id, tool_name, stats, created_at
        FROM messages
        WHERE conversation_id = ?
        ORDER BY created_at ASC, id ASC
    """
    best_rows: List[Dict[str, Any]] = []
    for db_path in db_paths:
        uri = f"file:{db_path}?mode=ro"
        try:
            with sqlite3.connect(uri, uri=True, timeout=5.0) as conn:
                conn.row_factory = sqlite3.Row
                conn.execute("PRAGMA busy_timeout=5000")
                rows = conn.execute(query, (conversation_id,)).fetchall()
        except sqlite3.Error as exc:
            LOG.debug("Failed to read Blue messages from %s: %s", db_path, exc)
            continue
        decoded: List[Dict[str, Any]] = []
        for row in rows:
            item = dict(row)
            for field in ("tool_calls", "stats"):
                raw = item.get(field)
                if not isinstance(raw, str) or not raw.strip():
                    item[field] = None
                    continue
                try:
                    item[field] = json.loads(raw)
                except json.JSONDecodeError:
                    LOG.debug(
                        "Failed to decode %s for conversation %s from %s",
                        field,
                        conversation_id,
                        db_path,
                    )
                    item[field] = None
            decoded.append(item)
        if len(decoded) > len(best_rows):
            best_rows = decoded
    return best_rows


def recover_blue_transcript_from_db(
    *,
    db_paths: Sequence[Path],
    conversation_id: str,
) -> List[Dict[str, Any]]:
    messages = load_blue_messages_from_db(db_paths, conversation_id)
    if not messages:
        return []
    transcript = convert_blue_messages_to_transcript(messages)
    return augment_transcript_with_audit(
        transcript,
        db_paths=db_paths,
        conversation_id=conversation_id,
    )


def build_audit_transcript_entries(audit_rows: Sequence[Dict[str, Any]]) -> List[Dict[str, Any]]:
    entries: List[Dict[str, Any]] = []
    for row in audit_rows:
        event_type = str(row.get("event_type", "") or "").strip().lower()
        tool_name = str(row.get("tool_name", "") or "").strip()
        display_tool_name = pinchbench_display_tool_name(tool_name)
        tool_call_id = str(row.get("tool_call_id", "") or "").strip()
        payload_text = str(row.get("payload", "") or "")
        if event_type == "assistant_tool_call" and tool_name:
            parsed_args = parse_tool_arguments(payload_text)
            entries.append(
                {
                    "type": "message",
                    "message": {
                        "role": "assistant",
                        "content": [
                            {
                                "type": "toolCall",
                                "id": tool_call_id or f"audit-{len(entries)+1}",
                                "name": display_tool_name,
                                "arguments": parsed_args,
                                "params": parsed_args,
                                **({"canonical_name": tool_name} if display_tool_name != tool_name else {}),
                            }
                        ],
                    },
                }
            )
            continue
        if event_type == "tool_result":
            entries.append(
                {
                    "type": "message",
                    "message": {
                        "role": "toolResult",
                        "toolCallId": tool_call_id,
                        "toolName": display_tool_name,
                        **({"canonicalToolName": tool_name} if display_tool_name != tool_name else {}),
                        "content": [payload_text],
                    },
                }
            )
    return entries


def augment_transcript_with_audit(
    transcript: List[Dict[str, Any]],
    *,
    db_paths: Sequence[Path],
    conversation_id: str,
) -> List[Dict[str, Any]]:
    if not transcript or transcript_has_tool_calls(transcript):
        return transcript
    audit_rows = load_tool_audit_rows(db_paths, conversation_id)
    if not audit_rows:
        return transcript
    audit_entries = build_audit_transcript_entries(audit_rows)
    if not audit_entries:
        return transcript

    insert_at = len(transcript)
    for idx in range(len(transcript) - 1, -1, -1):
        entry = transcript[idx]
        if entry.get("type") != "message":
            continue
        message = entry.get("message", {})
        if message.get("role") == "assistant":
            insert_at = idx
            break
    merged = transcript[:insert_at] + audit_entries + transcript[insert_at:]
    LOG.debug(
        "Injected %d audit transcript entries for conversation %s from %s",
        len(audit_entries),
        conversation_id,
        ", ".join(str(path) for path in db_paths),
    )
    return merged


def convert_blue_messages_to_transcript(messages: Sequence[Dict[str, Any]]) -> List[Dict[str, Any]]:
    transcript: List[Dict[str, Any]] = []
    for msg in messages:
        role = msg.get("role")
        entry: Dict[str, Any] = {"type": "message", "message": {}}
        body = entry["message"]

        if role == "user":
            body["role"] = "user"
            body["content"] = [msg.get("content", "")]
        elif role == "assistant":
            body["role"] = "assistant"
            content_items: List[Dict[str, Any]] = []
            for tool_call in msg.get("tool_calls") or []:
                parsed_args = parse_tool_arguments(tool_call.get("arguments", ""))
                original_name = str(tool_call.get("name") or "")
                display_name = pinchbench_display_tool_name(original_name)
                content_items.append(
                    {
                        "type": "toolCall",
                        "id": tool_call.get("id"),
                        "name": display_name,
                        "arguments": parsed_args,
                        "params": parsed_args,
                        **({"canonical_name": original_name} if display_name != original_name else {}),
                    }
                )
            if msg.get("content"):
                content_items.append({"type": "text", "text": msg.get("content", "")})
            body["content"] = content_items
            usage = usage_from_stats(msg.get("stats"))
            if usage:
                body["usage"] = usage
        elif role == "tool":
            original_tool_name = str(msg.get("tool_name") or "")
            display_tool_name = pinchbench_display_tool_name(original_tool_name)
            body["role"] = "toolResult"
            body["toolCallId"] = msg.get("tool_call_id", "")
            body["toolName"] = display_tool_name
            if display_tool_name != original_tool_name:
                body["canonicalToolName"] = original_tool_name
            body["content"] = [msg.get("content", "")]
        else:
            body["role"] = role or "unknown"
            body["content"] = [msg.get("content", "")]

        transcript.append(entry)
    return transcript


def extract_usage_from_transcript(transcript: Sequence[Dict[str, Any]]) -> Dict[str, Any]:
    totals = {
        "input_tokens": 0,
        "output_tokens": 0,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "total_tokens": 0,
        "cost_usd": 0.0,
        "request_count": 0,
    }
    for entry in transcript:
        if entry.get("type") != "message":
            continue
        message = entry.get("message", {})
        if message.get("role") != "assistant":
            continue
        usage = message.get("usage", {})
        totals["request_count"] += 1
        totals["input_tokens"] += int(usage.get("input", 0) or 0)
        totals["output_tokens"] += int(usage.get("output", 0) or 0)
        totals["cache_read_tokens"] += int(usage.get("cacheRead", 0) or 0)
        totals["cache_write_tokens"] += int(usage.get("cacheWrite", 0) or 0)
        totals["total_tokens"] += int(usage.get("totalTokens", 0) or 0)
        cost = usage.get("cost", {})
        if isinstance(cost, dict):
            totals["cost_usd"] += float(cost.get("total", 0.0) or 0.0)
    return totals


def extract_latest_assistant_text(transcript: Sequence[Dict[str, Any]]) -> str:
    for entry in reversed(transcript):
        if entry.get("type") != "message":
            continue
        message = entry.get("message", {})
        if message.get("role") != "assistant":
            continue
        content = message.get("content")
        if isinstance(content, str):
            return content
        if not isinstance(content, list):
            return ""
        parts: List[str] = []
        for block in content:
            if isinstance(block, str):
                parts.append(block)
                continue
            if isinstance(block, dict) and block.get("type") == "text":
                parts.append(str(block.get("text", "")))
        return "\n".join(part for part in parts if part)
    return ""


def has_empty_judge_response(transcript: Sequence[Dict[str, Any]]) -> bool:
    return extract_latest_assistant_text(transcript).strip() in {"", "{}"}


def wait_for_visible_assistant_transcript(
    client: "BlueClient",
    conversation_id: str,
    *,
    timeout_seconds: float,
    blue_audit_db_paths: Sequence[Path],
) -> List[Dict[str, Any]]:
    deadline = time.time() + max(0.0, timeout_seconds)
    latest_transcript: List[Dict[str, Any]] = []

    while True:
        remaining = max(0.0, deadline - time.time())
        request_timeout = min(max(remaining, 0.1), 30.0)
        messages = client.get_messages(conversation_id, timeout=request_timeout)
        latest_transcript = convert_blue_messages_to_transcript(messages)
        latest_transcript = augment_transcript_with_audit(
            latest_transcript,
            db_paths=blue_audit_db_paths,
            conversation_id=conversation_id,
        )
        if extract_latest_assistant_text(latest_transcript).strip():
            return latest_transcript
        if remaining <= 0:
            return latest_transcript
        time.sleep(min(JUDGE_MESSAGE_POLL_INTERVAL_SECONDS, remaining))


class BlueJudgeRunner:
    def __init__(
        self,
        client: BlueClient,
        provider: str,
        model: str,
        *,
        blue_audit_db_paths: Optional[Sequence[Path]] = None,
    ):
        self.client = client
        self.provider = provider
        self.model = model
        self.blue_audit_db_paths = list(blue_audit_db_paths or [])

    def run_prompt(self, *, prompt: str, workspace: Path, timeout_seconds: float) -> Dict[str, Any]:
        workspace.mkdir(parents=True, exist_ok=True)
        max_attempts = EMPTY_JUDGE_RESPONSE_MAX_RETRIES + 1
        last_result: Optional[Dict[str, Any]] = None
        for attempt in range(1, max_attempts + 1):
            conv = self.client.create_conversation("PinchBench judge", timeout=min(timeout_seconds, 30.0))
            conv_id = conv["id"]
            try:
                stderr_chunks: List[str] = []
                transcript: List[Dict[str, Any]] = []
                try:
                    transport_timeout = resolve_message_transport_timeout(timeout_seconds)
                    self.client.send_message(
                        conv_id,
                        prompt,
                        self.provider,
                        self.model,
                        timeout=transport_timeout,
                        web_search_enabled=False,
                        deep_research_enabled=False,
                    )
                    transcript = wait_for_visible_assistant_transcript(
                        self.client,
                        conv_id,
                        timeout_seconds=min(
                            max(timeout_seconds, 0.1),
                            JUDGE_MESSAGE_VISIBILITY_TIMEOUT_SECONDS,
                        ),
                        blue_audit_db_paths=self.blue_audit_db_paths,
                    )
                except BlueAPIError as exc:
                    transcript = recover_blue_transcript_from_db(
                        db_paths=self.blue_audit_db_paths,
                        conversation_id=conv_id,
                    )
                    if not transcript:
                        raise
                    LOG.warning(
                        "Recovered judge transcript for conversation %s after transport error: %s",
                        conv_id,
                        exc,
                    )
                    stderr_chunks.append(f"Recovered judge transcript after transport error: {exc}")
                last_result = {
                    "agent_id": f"blue-judge-{self.provider or 'auto'}",
                    "task_id": "judge",
                    "status": "success" if transcript else "error",
                    "transcript": transcript,
                    "usage": extract_usage_from_transcript(transcript),
                    "workspace": str(workspace),
                    "exit_code": 0 if transcript else 1,
                    "timed_out": False,
                    "execution_time": 0.0,
                    "stdout": "",
                    "stderr": "\n".join(stderr_chunks),
                    "conversation_id": conv_id,
                }
                if (
                    last_result["status"] == "success"
                    and has_empty_judge_response(transcript)
                    and attempt < max_attempts
                ):
                    LOG.warning(
                        "Judge returned an empty response on attempt %d/%d; retrying",
                        attempt,
                        max_attempts,
                    )
                    continue
                return last_result
            finally:
                try:
                    self.client.delete_conversation(conv_id, timeout=10.0)
                except BlueAPIError:
                    pass

        return last_result or {
            "agent_id": f"blue-judge-{self.provider or 'auto'}",
            "task_id": "judge",
            "status": "error",
            "transcript": [],
            "usage": extract_usage_from_transcript([]),
            "workspace": str(workspace),
            "exit_code": 1,
            "timed_out": False,
            "execution_time": 0.0,
            "stdout": "",
            "stderr": "judge returned no transcript",
            "conversation_id": "",
        }


def dataclass_to_dict(value: Any) -> Any:
    if is_dataclass(value):
        return asdict(value)
    return value


def execute_task(
    *,
    client: BlueClient,
    task: Any,
    pinchbench_dir: Path,
    workspace_dir: Path,
    provider: str,
    model: str,
    timeout_multiplier: float,
    blue_audit_db_paths: Sequence[Path],
) -> Dict[str, Any]:
    start_time = time.time()
    workspace = prepare_workspace(task, pinchbench_dir, workspace_dir)
    session_specs = extract_session_specs(task)
    timeout_seconds = float(task.timeout_seconds) * timeout_multiplier
    stdout_chunks: List[str] = []
    stderr_chunks: List[str] = []
    status = "success"
    timed_out = False
    exit_code = 0
    failed_prompt_index: Optional[int] = None
    conversation_runs: List[Dict[str, Any]] = []
    assistant_text_counts: Dict[str, int] = {}

    def fetch_augmented_conversation_transcript(conversation_id: str, timeout: float) -> List[Dict[str, Any]]:
        messages = client.get_messages(conversation_id, timeout=min(timeout, 30.0))
        conv_transcript = convert_blue_messages_to_transcript(messages)
        return augment_transcript_with_audit(
            conv_transcript,
            db_paths=blue_audit_db_paths,
            conversation_id=conversation_id,
        )

    def count_assistant_text_messages(items: Sequence[Dict[str, Any]]) -> int:
        count = 0
        for item in items:
            if item.get("type") != "message":
                continue
            message = item.get("message")
            if not isinstance(message, dict) or message.get("role") != "assistant":
                continue
            content = message.get("content")
            if isinstance(content, str):
                if content.strip():
                    count += 1
                continue
            if not isinstance(content, list):
                continue
            has_text = False
            for block in content:
                if isinstance(block, str) and block.strip():
                    has_text = True
                    break
                if isinstance(block, dict) and block.get("type") == "text" and str(block.get("text") or "").strip():
                    has_text = True
                    break
            if has_text:
                count += 1
        return count

    def wait_for_recovered_assistant_transcript(
        conversation_id: str,
        *,
        baseline_assistant_texts: int,
        timeout_seconds: float,
    ) -> List[Dict[str, Any]]:
        deadline = time.time() + max(0.0, timeout_seconds)
        latest_transcript: List[Dict[str, Any]] = []

        while True:
            remaining = max(0.0, deadline - time.time())
            request_timeout = min(max(remaining, 0.1), 30.0)
            latest_transcript = fetch_augmented_conversation_transcript(
                conversation_id,
                request_timeout,
            )
            if count_assistant_text_messages(latest_transcript) > baseline_assistant_texts:
                return latest_transcript
            if remaining <= 0:
                return latest_transcript
            time.sleep(min(JUDGE_MESSAGE_POLL_INTERVAL_SECONDS, remaining))

    def start_conversation(session_spec: Dict[str, Any], index: int, timeout: float) -> str:
        session_id = str(session_spec.get("id") or f"session_{index}").strip()
        title = task.name if index == 1 else f"{task.name} [{session_id}]"
        conv = client.create_conversation(title, timeout=min(timeout, 30.0))
        conv_id = conv["id"]
        assistant_text_counts[conv_id] = 0
        conversation_runs.append(
            {
                "conversation_id": conv_id,
                "session_ids": [],
            }
        )
        return conv_id

    conv_id = start_conversation(session_specs[0], 1, timeout_seconds)
    try:
        for idx, session_spec in enumerate(session_specs, start=1):
            elapsed = time.time() - start_time
            remaining = timeout_seconds - elapsed
            if remaining <= 0:
                status = "timeout"
                timed_out = True
                exit_code = 124
                stderr_chunks.append("Task timed out before all prompts were sent.")
                break

            if idx > 1 and session_spec.get("new_session"):
                conv_id = start_conversation(session_spec, idx, remaining)
            session_id = str(session_spec.get("id") or f"session_{idx}").strip()
            conversation_runs[-1]["session_ids"].append(session_id)
            prompt = str(session_spec.get("prompt") or "")
            LOG.info(
                "   Prompt %d/%d (%s)%s",
                idx,
                len(session_specs),
                session_id,
                " [new_session]" if session_spec.get("new_session") else "",
            )
            try:
                transport_timeout = resolve_message_transport_timeout(remaining)
                response = client.send_message(
                    conv_id,
                    prompt,
                    provider,
                    model,
                    timeout=transport_timeout,
                    web_search_enabled=True,
                    deep_research_enabled=False,
                )
            except BlueAPIError as exc:
                recovered_transcript: List[Dict[str, Any]] = []
                try:
                    recovered_transcript = wait_for_recovered_assistant_transcript(
                        conv_id,
                        baseline_assistant_texts=assistant_text_counts.get(conv_id, 0),
                        timeout_seconds=EXECUTION_MESSAGE_VISIBILITY_TIMEOUT_SECONDS,
                    )
                except BlueAPIError as fetch_exc:
                    stderr_chunks.append(
                        f"Failed to fetch conversation messages for {conv_id} after send failure: {fetch_exc}"
                    )
                baseline_assistant_texts = assistant_text_counts.get(conv_id, 0)
                recovered_assistant_texts = count_assistant_text_messages(recovered_transcript)
                if recovered_assistant_texts > baseline_assistant_texts:
                    LOG.warning(
                        "Recovered completed transcript for %s session %s after transport error",
                        task.task_id,
                        session_id,
                    )
                    stderr_chunks.append(
                        f"Recovered completed transcript for session {session_id} after transport error."
                    )
                    assistant_text_counts[conv_id] = recovered_assistant_texts
                    continue
                raise
            stdout_chunks.append(str(response.get("content", "")))
            if not response.get("id"):
                status = "error"
                exit_code = 1
                stderr_chunks.append(f"Unexpected message response: {response}")
                break
            try:
                prompt_transcript = fetch_augmented_conversation_transcript(conv_id, remaining)
            except BlueAPIError as exc:
                LOG.debug("Failed to refresh transcript after successful prompt %s: %s", session_id, exc)
            else:
                assistant_text_counts[conv_id] = count_assistant_text_messages(prompt_transcript)
    except BlueAPIError as exc:
        status = "error"
        exit_code = 1
        failed_prompt_index = idx if "idx" in locals() else None
        stderr_chunks.append(str(exc))

    transcript: List[Dict[str, Any]] = []
    session_runs: List[Dict[str, Any]] = []
    for run in conversation_runs:
        conversation_id = str(run.get("conversation_id") or "").strip()
        if not conversation_id:
            continue
        messages: List[Dict[str, Any]] = []
        try:
            messages = client.get_messages(conversation_id, timeout=min(timeout_seconds, 30.0))
        except BlueAPIError as exc:
            status = "error"
            exit_code = 1
            stderr_chunks.append(f"Failed to fetch conversation messages for {conversation_id}: {exc}")
            continue

        conv_transcript = convert_blue_messages_to_transcript(messages)
        conv_transcript = augment_transcript_with_audit(
            conv_transcript,
            db_paths=blue_audit_db_paths,
            conversation_id=conversation_id,
        )
        transcript.extend(conv_transcript)
        session_runs.append(
            {
                "conversation_id": conversation_id,
                "session_ids": list(run.get("session_ids") or []),
            }
        )
    if (
        status == "error"
        and failed_prompt_index is not None
        and failed_prompt_index == len(session_specs)
        and transcript
    ):
        assistant_messages = [
            item
            for item in transcript
            if item.get("type") == "message"
            and isinstance(item.get("message"), dict)
            and item["message"].get("role") == "assistant"
        ]
        if len(assistant_messages) >= failed_prompt_index:
            LOG.warning(
                "Recovered completed transcript for %s after final-prompt transport error",
                task.task_id,
            )
            status = "success"
            exit_code = 0
            stderr_chunks.append(
                "Recovered completed transcript after final-prompt transport error."
            )
    if not transcript and status == "success":
        status = "error"
        exit_code = 1
        stderr_chunks.append("Blue returned no transcript messages.")

    return {
        "agent_id": f"blue-{provider or 'auto'}",
        "task_id": task.task_id,
        "status": status,
        "transcript": transcript,
        "usage": extract_usage_from_transcript(transcript),
        "workspace": str(workspace),
        "exit_code": exit_code,
        "timed_out": timed_out,
        "execution_time": time.time() - start_time,
        "stdout": "\n".join(chunk for chunk in stdout_chunks if chunk),
        "stderr": "\n".join(chunk for chunk in stderr_chunks if chunk),
        "conversation_id": conv_id,
        "conversation_ids": [run["conversation_id"] for run in conversation_runs if run.get("conversation_id")],
        "session_runs": session_runs,
    }


def serialize_grade(grade: Any) -> Any:
    if grade is None:
        return None
    if hasattr(grade, "to_dict"):
        return grade.to_dict()
    return dataclass_to_dict(grade)


def grade_task_result(
    *,
    lib_grading: Any,
    task: Any,
    execution_result: Dict[str, Any],
    pinchbench_dir: Path,
    judge_runner: Optional[BlueJudgeRunner],
    judge_model: Optional[str],
    skip_judge: bool,
    verbose: bool,
) -> Any:
    if skip_judge:
        if task.grading_type == "automated":
            return lib_grading.grade_task(
                task=task,
                execution_result=execution_result,
                skill_dir=pinchbench_dir,
                verbose=verbose,
            )
        automated_only = getattr(lib_grading, "_grade_automated", None)
        if callable(automated_only):
            return automated_only(task, execution_result, verbose=verbose)
        return None

    if judge_runner is None or not judge_model:
        raise RuntimeError("judge runner is required when --skip-judge is not set")

    original_run = getattr(lib_grading, "run_openclaw_prompt")
    original_ensure = getattr(lib_grading, "ensure_agent_exists")

    def blue_run_openclaw_prompt(*, agent_id: str, prompt: str, workspace: Path, timeout_seconds: float):
        return judge_runner.run_prompt(
            prompt=prompt,
            workspace=workspace,
            timeout_seconds=timeout_seconds,
        )

    def blue_ensure_agent_exists(agent_id: str, model_id: str, workspace_dir: Path) -> bool:
        LOG.debug("Judge agent bootstrap skipped for Blue runner: %s %s", agent_id, model_id)
        workspace_dir.mkdir(parents=True, exist_ok=True)
        return False

    setattr(lib_grading, "run_openclaw_prompt", blue_run_openclaw_prompt)
    setattr(lib_grading, "ensure_agent_exists", blue_ensure_agent_exists)
    try:
        return lib_grading.grade_task(
            task=task,
            execution_result=execution_result,
            skill_dir=pinchbench_dir,
            judge_model=judge_model,
            verbose=verbose,
        )
    finally:
        setattr(lib_grading, "run_openclaw_prompt", original_run)
        setattr(lib_grading, "ensure_agent_exists", original_ensure)


def summarize_results(results: Sequence[Dict[str, Any]]) -> Dict[str, Any]:
    task_count = len(results)
    graded = [item["grade"]["score"] for item in results if item.get("grade") and isinstance(item["grade"].get("score"), (int, float))]
    successes = sum(1 for item in results if item["execution"]["status"] == "success")
    return {
        "task_count": task_count,
        "success_count": successes,
        "failure_count": task_count - successes,
        "average_score": (sum(graded) / len(graded)) if graded else None,
        "graded_count": len(graded),
    }


def persist_results_payload_to_db(db_path: Path, payload: Dict[str, Any]) -> str:
    db_path.parent.mkdir(parents=True, exist_ok=True)
    run_id = uuid.uuid4().hex
    summary_json = json.dumps(payload.get("summary") or {}, ensure_ascii=False, sort_keys=True)
    payload_json = json.dumps(payload, ensure_ascii=False, sort_keys=True)

    with sqlite3.connect(db_path) as conn:
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS runs (
                run_id TEXT PRIMARY KEY,
                generated_at TEXT NOT NULL,
                runner TEXT NOT NULL,
                blue_base_url TEXT NOT NULL,
                workspace_dir TEXT NOT NULL,
                judge_mode TEXT NOT NULL,
                provider TEXT,
                model TEXT,
                judge_provider TEXT,
                judge_model TEXT,
                suite TEXT NOT NULL,
                skip_judge INTEGER NOT NULL,
                summary_json TEXT NOT NULL,
                payload_json TEXT NOT NULL
            )
            """
        )
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS task_results (
                run_id TEXT NOT NULL,
                task_id TEXT NOT NULL,
                task_name TEXT NOT NULL,
                category TEXT,
                grading_type TEXT,
                execution_status TEXT,
                execution_conversation_id TEXT,
                execution_json TEXT NOT NULL,
                grade_score REAL,
                grade_error TEXT,
                grade_json TEXT,
                result_json TEXT NOT NULL,
                PRIMARY KEY (run_id, task_id)
            )
            """
        )
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS criterion_scores (
                run_id TEXT NOT NULL,
                task_id TEXT NOT NULL,
                criterion_name TEXT NOT NULL,
                score REAL NOT NULL,
                PRIMARY KEY (run_id, task_id, criterion_name)
            )
            """
        )
        conn.execute(
            "CREATE INDEX IF NOT EXISTS idx_task_results_task_id ON task_results(task_id)"
        )
        conn.execute(
            "CREATE INDEX IF NOT EXISTS idx_criterion_scores_task_id ON criterion_scores(task_id)"
        )

        conn.execute(
            """
            INSERT INTO runs (
                run_id,
                generated_at,
                runner,
                blue_base_url,
                workspace_dir,
                judge_mode,
                provider,
                model,
                judge_provider,
                judge_model,
                suite,
                skip_judge,
                summary_json,
                payload_json
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                run_id,
                str(payload.get("generated_at") or ""),
                str(payload.get("runner") or ""),
                str(payload.get("blue_base_url") or ""),
                str(payload.get("workspace_dir") or ""),
                str(payload.get("judge_mode") or ""),
                str(payload.get("provider") or ""),
                str(payload.get("model") or ""),
                str(payload.get("judge_provider") or ""),
                str(payload.get("judge_model") or ""),
                str(payload.get("suite") or ""),
                1 if payload.get("skip_judge") else 0,
                summary_json,
                payload_json,
            ),
        )

        for result in payload.get("results") or []:
            execution = result.get("execution") or {}
            grade = result.get("grade") or {}
            result_json = json.dumps(result, ensure_ascii=False, sort_keys=True)
            execution_json = json.dumps(execution, ensure_ascii=False, sort_keys=True)
            grade_json = json.dumps(grade, ensure_ascii=False, sort_keys=True) if grade else None
            grade_score = grade.get("score") if isinstance(grade.get("score"), (int, float)) else None
            conn.execute(
                """
                INSERT OR REPLACE INTO task_results (
                    run_id,
                    task_id,
                    task_name,
                    category,
                    grading_type,
                    execution_status,
                    execution_conversation_id,
                    execution_json,
                    grade_score,
                    grade_error,
                    grade_json,
                    result_json
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    run_id,
                    str(result.get("task_id") or ""),
                    str(result.get("task_name") or ""),
                    str(result.get("category") or ""),
                    str(result.get("grading_type") or ""),
                    str(execution.get("status") or ""),
                    str(execution.get("conversation_id") or ""),
                    execution_json,
                    grade_score,
                    result.get("grade_error"),
                    grade_json,
                    result_json,
                ),
            )

            if isinstance(grade.get("breakdown"), dict):
                for criterion_name, score in grade["breakdown"].items():
                    if not isinstance(score, (int, float)):
                        continue
                    conn.execute(
                        """
                        INSERT OR REPLACE INTO criterion_scores (
                            run_id,
                            task_id,
                            criterion_name,
                            score
                        ) VALUES (?, ?, ?, ?)
                        """,
                        (
                            run_id,
                            str(result.get("task_id") or ""),
                            str(criterion_name),
                            float(score),
                        ),
                    )

        conn.commit()
    return run_id


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
    )

    pinchbench_dir = Path(args.pinchbench_dir).resolve()
    requested_workspace_dir = Path(args.workspace_dir).resolve()
    output_path = resolve_output_path_with_judge_mode(Path(args.output).resolve(), args.skip_judge)
    blue_db_path = resolve_blue_db_path(args.blue_db_path)
    runtime_workspace_dir = resolve_blue_runtime_workspace_dir(blue_db_path)
    workspace_dir = requested_workspace_dir
    if runtime_workspace_dir is not None and runtime_workspace_dir != requested_workspace_dir:
        LOG.warning(
            "Workspace dir %s does not match Blue runtime workspace %s; using runtime workspace",
            requested_workspace_dir,
            runtime_workspace_dir,
        )
        workspace_dir = runtime_workspace_dir
    blue_audit_db_paths = resolve_blue_audit_db_paths(args.blue_audit_db_path, blue_db_path)
    if blue_db_path is not None:
        LOG.info("Using Blue DB at %s for transcript audit recovery", blue_db_path)
    else:
        LOG.info("Blue DB not found; transcript recovery will rely on conversation messages only")
    LOG.info("Using workspace dir %s", workspace_dir)
    if blue_audit_db_paths:
        LOG.info("Using Blue audit DBs: %s", ", ".join(str(path) for path in blue_audit_db_paths))

    lib_tasks, lib_grading = load_pinchbench_modules(pinchbench_dir)
    task_loader = lib_tasks.TaskLoader(pinchbench_dir / "tasks")
    tasks = task_loader.load_all_tasks()
    selected_tasks = select_tasks(tasks, args.suite)
    ensure_judge_compatible_selection(selected_tasks, args.skip_judge)

    client = BlueClient(args.blue_base_url, args.api_key)
    judge_provider = args.judge_provider or args.provider
    judge_model = args.judge_model or args.model
    judge_runner = None if args.skip_judge else BlueJudgeRunner(
        client,
        judge_provider,
        judge_model,
        blue_audit_db_paths=blue_audit_db_paths,
    )

    LOG.info("Loaded %d tasks; running %d", len(tasks), len(selected_tasks))
    run_results: List[Dict[str, Any]] = []

    for index, task in enumerate(selected_tasks, start=1):
        LOG.info("[%d/%d] %s (%s)", index, len(selected_tasks), task.task_id, task.name)
        execution = execute_task(
            client=client,
            task=task,
            pinchbench_dir=pinchbench_dir,
            workspace_dir=workspace_dir,
            provider=args.provider,
            model=args.model,
            timeout_multiplier=args.timeout_multiplier,
            blue_audit_db_paths=blue_audit_db_paths,
        )

        grade = None
        grade_error = None
        if execution["status"] == "success":
            try:
                grade_obj = grade_task_result(
                    lib_grading=lib_grading,
                    task=task,
                    execution_result=execution,
                    pinchbench_dir=pinchbench_dir,
                    judge_runner=judge_runner,
                    judge_model=judge_model,
                    skip_judge=args.skip_judge,
                    verbose=args.verbose,
                )
                grade = serialize_grade(grade_obj)
            except Exception as exc:  # pragma: no cover - benchmark runner surface
                grade_error = str(exc)
        else:
            grade_error = "execution_failed"

        task_result = {
            "task_id": task.task_id,
            "task_name": task.name,
            "category": task.category,
            "grading_type": task.grading_type,
            "execution": execution,
            "grade": grade,
            "grade_error": grade_error,
        }
        run_results.append(task_result)

        if execution["status"] != "success" and not args.no_fail_fast:
            LOG.error("Stopping early after task failure: %s", execution["stderr"])
            break

    payload = {
        "runner": "blue",
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "blue_base_url": args.blue_base_url,
        "workspace_dir": str(workspace_dir),
        "judge_mode": judge_mode(args.skip_judge),
        "provider": args.provider,
        "model": args.model,
        "judge_provider": None if args.skip_judge else judge_provider,
        "judge_model": None if args.skip_judge else judge_model,
        "suite": args.suite,
        "skip_judge": args.skip_judge,
        "results": run_results,
        "summary": summarize_results(run_results),
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    if args.output_db:
        output_db_path = Path(args.output_db).resolve()
        run_id = persist_results_payload_to_db(output_db_path, payload)
        LOG.info("Wrote results DB to %s (run_id=%s)", output_db_path, run_id)

    LOG.info("Wrote results to %s", output_path)
    LOG.info("Summary: %s", json.dumps(payload["summary"], ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
