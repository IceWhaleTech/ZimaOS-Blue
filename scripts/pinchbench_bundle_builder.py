#!/usr/bin/env python3
"""
Build a first-party Harness dataset bundle from a local PinchBench checkout.
"""

from __future__ import annotations

import argparse
import base64
import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml


DEFAULT_JUDGE_MODEL = "claude-sonnet-4-6"


@dataclass
class Task:
    task_id: str
    name: str
    category: str
    grading_type: str
    timeout_seconds: int
    workspace_files: list[dict[str, Any]]
    prompt: str
    expected_behavior: str
    grading_criteria: list[str]
    automated_checks: str | None
    llm_judge_rubric: str | None
    grading_weights: dict[str, float] | None
    source_file: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build the PinchBench Harness bundle.")
    parser.add_argument(
        "--pinchbench-dir",
        default="/Users/orca/Downloads/pinchbench-skill",
        help="Path to the official PinchBench checkout.",
    )
    parser.add_argument(
        "--bundle-dir",
        default=str(Path(__file__).resolve().parents[1] / "harness" / "datasets" / "pinchbench"),
        help="Output directory for the generated bundle.",
    )
    return parser.parse_args()


def parse_sections(body: str) -> dict[str, str]:
    sections: dict[str, str] = {}
    current: str | None = None
    lines: list[str] = []
    for line in body.splitlines():
        header = re.match(r"^##\s+(.+)$", line)
        if header:
            if current:
                sections[current] = "\n".join(lines).strip()
            current = header.group(1).strip()
            lines = []
            continue
        if current:
            lines.append(line)
    if current:
        sections[current] = "\n".join(lines).strip()
    return sections


def extract_grading_criteria(text: str) -> list[str]:
    criteria: list[str] = []
    for line in text.splitlines():
        match = re.match(r"^-\s+\[[ x]\]\s+(.+)$", line.strip())
        if match:
            criteria.append(match.group(1).strip())
    return criteria


def load_task(path: Path) -> Task:
    content = path.read_text(encoding="utf-8")
    match = re.match(r"^---\s*\n(.*?)\n---\s*\n(.*)$", content, re.DOTALL)
    if not match:
        raise ValueError(f"missing frontmatter: {path}")
    frontmatter = yaml.safe_load(match.group(1)) or {}
    sections = parse_sections(match.group(2))
    return Task(
        task_id=str(frontmatter.get("id", "")).strip(),
        name=str(frontmatter.get("name", "")).strip(),
        category=str(frontmatter.get("category", "")).strip(),
        grading_type=str(frontmatter.get("grading_type", "")).strip(),
        timeout_seconds=int(frontmatter.get("timeout_seconds", 0) or 0),
        workspace_files=list(frontmatter.get("workspace_files", []) or []),
        prompt=sections.get("Prompt", "").strip(),
        expected_behavior=sections.get("Expected Behavior", "").strip(),
        grading_criteria=extract_grading_criteria(sections.get("Grading Criteria", "")),
        automated_checks=sections.get("Automated Checks", None),
        llm_judge_rubric=sections.get("LLM Judge Rubric", None),
        grading_weights=frontmatter.get("grading_weights", None),
        source_file=path.name,
    )


def build_success_rubric(task: Task) -> str:
    parts: list[str] = []
    if task.expected_behavior:
        parts.append(task.expected_behavior)
    if task.grading_criteria:
        parts.append("Checklist: " + "; ".join(task.grading_criteria[:6]))
    if not parts:
        parts.append(task.prompt)
    rubric = "\n\n".join(part for part in parts if part).strip()
    return rubric[:4000]


def convert_workspace_files(task: Task, pinchbench_dir: Path) -> list[dict[str, Any]]:
    converted: list[dict[str, Any]] = []
    assets_dir = pinchbench_dir / "assets"
    for file_spec in task.workspace_files:
        if "path" in file_spec and "content" in file_spec:
            converted.append(
                {
                    "path": str(file_spec["path"]).strip(),
                    "content": str(file_spec["content"]),
                }
            )
            continue
        if "source" in file_spec and "dest" in file_spec:
            source = assets_dir / str(file_spec["source"]).strip()
            blob = source.read_bytes()
            converted.append(
                {
                    "path": str(file_spec["dest"]).strip(),
                    "content_base64": base64.b64encode(blob).decode("ascii"),
                }
            )
            continue
        raise ValueError(f"unsupported workspace file shape for {task.task_id}: {file_spec}")
    return converted


def extract_expected_artifacts(task: Task) -> list[str]:
    artifacts: list[str] = []
    texts = [task.prompt, task.expected_behavior]
    backtick_regex = re.compile(
        r"(?i)(?:save|write|create|output|named|called)[^`\n]{0,160}`([^`]+\.[A-Za-z0-9]+)`"
    )
    plain_regex = re.compile(r"(?i)(?:save|write|create|output|named|called)[^\n]{0,120}\b([A-Za-z0-9_./-]+\.[A-Za-z0-9]+)\b")
    for text in texts:
        for line in text.splitlines():
            lowered = line.lower()
            if not any(token in lowered for token in ("save", "write", "create", "output", "named", "called")):
                continue
            for candidate in backtick_regex.findall(line):
                if candidate not in artifacts:
                    artifacts.append(candidate)
            for candidate in plain_regex.findall(line):
                if candidate not in artifacts:
                    artifacts.append(candidate)
    return artifacts


def build_item(task: Task, pinchbench_dir: Path) -> dict[str, Any]:
    expected: dict[str, Any] = {"status": "completed"}
    artifacts = extract_expected_artifacts(task)
    if artifacts:
        expected["expected_artifacts"] = artifacts

    metadata: dict[str, Any] = {
        "source_benchmark": "pinchbench",
        "pinchbench_task_id": task.task_id,
        "pinchbench_source_file": task.source_file,
        "pinchbench_name": task.name,
        "pinchbench_category": task.category,
        "pinchbench_grading_type": task.grading_type,
        "pinchbench_timeout_seconds": task.timeout_seconds,
        "pinchbench_expected_behavior": task.expected_behavior,
        "pinchbench_grading_criteria": task.grading_criteria,
        "task_success_criteria": task.grading_criteria,
        "success_rubric": build_success_rubric(task),
        "locale": "en-US",
        "critical": False,
        "policy_model_hint": DEFAULT_JUDGE_MODEL,
    }
    if task.automated_checks:
        metadata["pinchbench_automated_checks"] = task.automated_checks
    if task.llm_judge_rubric:
        metadata["pinchbench_llm_judge_rubric"] = task.llm_judge_rubric
    if task.grading_weights:
        metadata["pinchbench_grading_weights"] = task.grading_weights
    if task.workspace_files:
        metadata["harness_workspace_files"] = convert_workspace_files(task, pinchbench_dir)

    return {
        "id": task.task_id,
        "run_kind": "agent_task",
        "profile": "pinchbench",
        "input": {
            "goal": task.prompt,
            "prompt": task.prompt,
            "locale": "en-US",
            "lang": "en-US",
            "sandbox_mode": "workspace",
            "max_duration_seconds": task.timeout_seconds,
        },
        "expected": expected,
        "metadata": metadata,
    }


def build_manifest(tasks: list[Task], pinchbench_dir: Path) -> dict[str, Any]:
    return {
        "dataset": {
            "name": "pinchbench",
            "subject": "pinchbench",
        },
        "defaults": {
            "run_kind": "agent_task",
            "profile": "pinchbench",
            "scheduler": {
                "max_concurrency": 2,
                "max_attempts": 2,
                "retry_backoff": 15_000_000_000,
            },
            "scoring": {
                "mode": "hybrid",
                "rule_profile": "pinchbench",
                "judge_model": DEFAULT_JUDGE_MODEL,
                "pass_threshold": 0.7,
            },
        },
        "items": [build_item(task, pinchbench_dir) for task in tasks],
    }


def dataset_yaml(task_count: int) -> str:
    return (
        "api_version: harness.blue/v1alpha1\n"
        "kind: dataset_bundle\n"
        "name: pinchbench\n"
        "description: Official PinchBench task bundle adapted for Blue Harness import, execution, and quality evaluation.\n"
        "subject: pinchbench\n"
        "default_run_kind: agent_task\n"
        "default_profile: pinchbench\n"
        "default_version: v1\n"
        "metadata:\n"
        "  source_benchmark: pinchbench\n"
        f"  task_count: {task_count}\n"
        "  workspace_fixture_mode: embedded_manifest\n"
        "  scoring_mode: hybrid\n"
        "versions:\n"
        "  v1:\n"
        "    eval_specs:\n"
        "      - default.yaml\n"
    )


def eval_spec_yaml(task_count: int) -> str:
    return (
        "name: PinchBench v1\n"
        "subject: pinchbench\n"
        "run_kind: agent_task\n"
        "profile: pinchbench\n"
        "scheduler:\n"
        "  maxconcurrency: 2\n"
        "  maxattempts: 2\n"
        "  retrybackoff: 15s\n"
        "scoring:\n"
        "  mode: hybrid\n"
        "  ruleprofile: pinchbench\n"
        f"  judgemodel: {DEFAULT_JUDGE_MODEL}\n"
        "  passthreshold: 0.7\n"
        "metadata:\n"
        "  lane: pinchbench\n"
        "  source_benchmark: pinchbench\n"
        f"  task_count: {task_count}\n"
        "  workspace_fixture_mode: embedded_manifest\n"
    )


def readme(task_count: int) -> str:
    return f"""# PinchBench

This bundle adapts the full local PinchBench task set into an importable Blue Harness dataset bundle.

## Coverage

- Task count: {task_count}
- Source benchmark: official PinchBench markdown tasks
- Run kind: `agent_task`
- Default profile: `pinchbench`
- Quality evaluation: Harness `hybrid` scoring with a configured judge model plus task metadata, workspace summaries, and expected-artifact verification

## Mapping Notes

- Each task prompt is mapped into `input.goal` and `input.prompt`.
- Upstream `workspace_files` are preserved as embedded `metadata.harness_workspace_files`.
- Large source assets such as PDFs and spreadsheets are embedded directly in the manifest so imported datasets remain runnable without the original PinchBench checkout.
- Upstream expected behavior, grading criteria, automated checks, judge rubrics, and grading weights are preserved in item metadata for evaluation context.

## Limitations

- Harness currently evaluates these cases through its native hybrid scorer rather than executing the original PinchBench Python grading code directly.
- The preserved upstream grading assets remain available in metadata for future higher-fidelity scorer work.
"""


def write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def main() -> None:
    args = parse_args()
    pinchbench_dir = Path(args.pinchbench_dir).resolve()
    bundle_dir = Path(args.bundle_dir).resolve()
    tasks_dir = pinchbench_dir / "tasks"
    task_paths = sorted(tasks_dir.glob("task_*.md"))
    tasks = [load_task(path) for path in task_paths]
    manifest = build_manifest(tasks, pinchbench_dir)

    write_text(bundle_dir / "dataset.yaml", dataset_yaml(len(tasks)))
    write_text(bundle_dir / "README.md", readme(len(tasks)))
    write_text(
        bundle_dir / "versions" / "v1" / "eval-specs" / "default.yaml",
        eval_spec_yaml(len(tasks)),
    )
    write_text(
        bundle_dir / "versions" / "v1" / "manifest.json",
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
    )


if __name__ == "__main__":
    main()
