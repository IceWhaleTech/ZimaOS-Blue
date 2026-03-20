#!/usr/bin/env python3
"""
Blue-compatible PinchBench runner.

This script reuses the official PinchBench task loader and grading logic, but
executes tasks through Blue's HTTP conversation API instead of the OpenClaw CLI.

Notes:
- Blue must already be running.
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
import shutil
import sys
import time
from dataclasses import asdict, is_dataclass
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence
from urllib import error, request


LOG = logging.getLogger("pinchbench-blue")


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
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")

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


def clear_workspace(workspace_dir: Path) -> None:
    if workspace_dir.exists():
        shutil.rmtree(workspace_dir)
    workspace_dir.mkdir(parents=True, exist_ok=True)


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


def extract_prompts(task: Any) -> List[str]:
    sessions = task.frontmatter.get("sessions", [])
    if not sessions:
        return [task.prompt]

    prompts: List[str] = []
    for session in sessions:
        if isinstance(session, str):
            prompts.append(session)
            continue
        if isinstance(session, dict):
            prompt = session.get("prompt") or session.get("message")
            if prompt:
                prompts.append(prompt)
    return prompts or [task.prompt]


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
                content_items.append(
                    {
                        "type": "toolCall",
                        "id": tool_call.get("id"),
                        "name": tool_call.get("name"),
                        "arguments": parsed_args,
                        "params": parsed_args,
                    }
                )
            if msg.get("content"):
                content_items.append({"type": "text", "text": msg.get("content", "")})
            body["content"] = content_items
            usage = usage_from_stats(msg.get("stats"))
            if usage:
                body["usage"] = usage
        elif role == "tool":
            body["role"] = "toolResult"
            body["toolCallId"] = msg.get("tool_call_id", "")
            body["toolName"] = msg.get("tool_name", "")
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


class BlueJudgeRunner:
    def __init__(self, client: BlueClient, provider: str, model: str):
        self.client = client
        self.provider = provider
        self.model = model

    def run_prompt(self, *, prompt: str, workspace: Path, timeout_seconds: float) -> Dict[str, Any]:
        workspace.mkdir(parents=True, exist_ok=True)
        conv = self.client.create_conversation("PinchBench judge", timeout=min(timeout_seconds, 30.0))
        conv_id = conv["id"]
        try:
            self.client.send_message(
                conv_id,
                prompt,
                self.provider,
                self.model,
                timeout=timeout_seconds,
                web_search_enabled=False,
                deep_research_enabled=False,
            )
            messages = self.client.get_messages(conv_id, timeout=min(timeout_seconds, 30.0))
            transcript = convert_blue_messages_to_transcript(messages)
            return {
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
                "stderr": "",
            }
        finally:
            try:
                self.client.delete_conversation(conv_id, timeout=10.0)
            except BlueAPIError:
                pass


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
) -> Dict[str, Any]:
    start_time = time.time()
    workspace = prepare_workspace(task, pinchbench_dir, workspace_dir)
    prompts = extract_prompts(task)
    timeout_seconds = float(task.timeout_seconds) * timeout_multiplier
    stdout_chunks: List[str] = []
    stderr_chunks: List[str] = []
    status = "success"
    timed_out = False
    exit_code = 0
    failed_prompt_index: Optional[int] = None

    conv = client.create_conversation(task.name, timeout=min(timeout_seconds, 30.0))
    conv_id = conv["id"]
    try:
        for idx, prompt in enumerate(prompts, start=1):
            elapsed = time.time() - start_time
            remaining = timeout_seconds - elapsed
            if remaining <= 0:
                status = "timeout"
                timed_out = True
                exit_code = 124
                stderr_chunks.append("Task timed out before all prompts were sent.")
                break

            LOG.info("   Prompt %d/%d", idx, len(prompts))
            response = client.send_message(
                conv_id,
                prompt,
                provider,
                model,
                timeout=remaining,
                web_search_enabled=True,
                deep_research_enabled=True,
            )
            stdout_chunks.append(str(response.get("content", "")))
            if not response.get("id"):
                status = "error"
                exit_code = 1
                stderr_chunks.append(f"Unexpected message response: {response}")
                break
    except BlueAPIError as exc:
        status = "error"
        exit_code = 1
        failed_prompt_index = idx if "idx" in locals() else None
        stderr_chunks.append(str(exc))

    messages: List[Dict[str, Any]] = []
    try:
        messages = client.get_messages(conv_id, timeout=min(timeout_seconds, 30.0))
    except BlueAPIError as exc:
        status = "error"
        exit_code = 1
        stderr_chunks.append(f"Failed to fetch conversation messages: {exc}")

    transcript = convert_blue_messages_to_transcript(messages)
    if (
        status == "error"
        and failed_prompt_index is not None
        and failed_prompt_index == len(prompts)
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


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
    )

    pinchbench_dir = Path(args.pinchbench_dir).resolve()
    workspace_dir = Path(args.workspace_dir).resolve()
    output_path = Path(args.output).resolve()

    lib_tasks, lib_grading = load_pinchbench_modules(pinchbench_dir)
    task_loader = lib_tasks.TaskLoader(pinchbench_dir / "tasks")
    tasks = task_loader.load_all_tasks()
    selected_tasks = select_tasks(tasks, args.suite)

    client = BlueClient(args.blue_base_url)
    judge_provider = args.judge_provider or args.provider
    judge_model = args.judge_model or args.model
    judge_runner = None if args.skip_judge else BlueJudgeRunner(client, judge_provider, judge_model)

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

    LOG.info("Wrote results to %s", output_path)
    LOG.info("Summary: %s", json.dumps(payload["summary"], ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
