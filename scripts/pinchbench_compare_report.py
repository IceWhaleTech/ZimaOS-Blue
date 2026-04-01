#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence, Tuple


@dataclass
class ReportAttempt:
    task_id: str
    task_name: str
    report_path: str
    generated_at: str
    judge_mode: str
    score: Optional[float]
    execution_status: str
    grade_error: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate a consolidated Blue vs OpenClaw PinchBench markdown report.")
    default_reference = Path(__file__).with_name("pinchbench_reference_scores.json")
    parser.add_argument(
        "--reference",
        default=str(default_reference),
        help="Reference JSON containing baseline Blue scores and OpenClaw scores.",
    )
    parser.add_argument(
        "--reports",
        nargs="+",
        default=["docs/reports/pinchbench_blue_*.json"],
        help="One or more report files or glob patterns to merge.",
    )
    parser.add_argument(
        "--output-md",
        default="docs/reports/pinchbench_comparison_report.md",
        help="Markdown report output path.",
    )
    parser.add_argument(
        "--output-json",
        default="docs/reports/pinchbench_comparison_report.json",
        help="Structured JSON summary output path.",
    )
    parser.add_argument(
        "--title",
        default="Blue vs OpenClaw PinchBench 汇总对比",
        help="Markdown title.",
    )
    parser.add_argument(
        "--model-contains",
        default="",
        help="Only include reports whose top-level model contains this substring.",
    )
    parser.add_argument(
        "--include-judge-off",
        action="store_true",
        help="Include judge-off reports when consolidating results.",
    )
    return parser.parse_args()


def parse_timestamp(raw: str, fallback_path: Path) -> datetime:
    text = str(raw or "").strip()
    if text:
        try:
            if text.endswith("Z"):
                return datetime.fromisoformat(text.replace("Z", "+00:00"))
            return datetime.fromisoformat(text)
        except ValueError:
            pass
    stat = fallback_path.stat()
    return datetime.fromtimestamp(stat.st_mtime, tz=timezone.utc)


def expand_report_paths(patterns: Sequence[str]) -> List[Path]:
    paths: List[Path] = []
    seen = set()
    for pattern in patterns:
        matches = sorted(Path().glob(pattern))
        if matches:
            for match in matches:
                resolved = match.resolve()
                key = str(resolved)
                if key in seen or not resolved.is_file():
                    continue
                seen.add(key)
                paths.append(resolved)
            continue
        candidate = Path(pattern).resolve()
        if candidate.is_file():
            key = str(candidate)
            if key not in seen:
                seen.add(key)
                paths.append(candidate)
    return paths


def load_reference(path: Path) -> List[Dict[str, Any]]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    tasks = payload.get("tasks")
    if not isinstance(tasks, list):
        raise ValueError(f"reference file {path} does not contain a tasks list")
    return tasks


def as_float(value: Any) -> Optional[float]:
    if isinstance(value, (int, float)):
        return float(value)
    return None


def load_attempts(
    report_paths: Sequence[Path],
    include_judge_off: bool,
    model_contains: str,
) -> Tuple[List[ReportAttempt], List[Path]]:
    attempts: List[ReportAttempt] = []
    used_reports: List[Path] = []
    model_contains = model_contains.strip().lower()
    for report_path in report_paths:
        payload = json.loads(report_path.read_text(encoding="utf-8"))
        model = str(payload.get("model") or "").strip()
        if model_contains and model_contains not in model.lower():
            continue
        judge_mode = str(payload.get("judge_mode") or "").strip()
        if judge_mode == "" and bool(payload.get("skip_judge")):
            judge_mode = "judge-off"
        if judge_mode == "":
            judge_mode = "judge-on"
        if judge_mode != "judge-on" and not include_judge_off:
            continue
        results = payload.get("results")
        if not isinstance(results, list):
            continue
        used_reports.append(report_path)
        generated_at = str(payload.get("generated_at") or "")
        for item in results:
            if not isinstance(item, dict):
                continue
            grade = item.get("grade") if isinstance(item.get("grade"), dict) else {}
            execution = item.get("execution") if isinstance(item.get("execution"), dict) else {}
            attempts.append(
                ReportAttempt(
                    task_id=str(item.get("task_id") or "").strip(),
                    task_name=str(item.get("task_name") or "").strip(),
                    report_path=report_path.name,
                    generated_at=generated_at,
                    judge_mode=judge_mode,
                    score=as_float(grade.get("score")),
                    execution_status=str(execution.get("status") or "").strip(),
                    grade_error=str(item.get("grade_error") or "").strip(),
                )
            )
    return attempts, used_reports


def choose_latest_attempt(attempts: Sequence[ReportAttempt]) -> Optional[ReportAttempt]:
    if not attempts:
        return None
    return max(
        attempts,
        key=lambda attempt: (
            attempt.generated_at,
            attempt.report_path,
            attempt.score is not None,
        ),
    )


def choose_latest_scored_attempt(attempts: Sequence[ReportAttempt]) -> Optional[ReportAttempt]:
    scored = [attempt for attempt in attempts if attempt.score is not None]
    if not scored:
        return None
    return max(scored, key=lambda attempt: (attempt.generated_at, attempt.report_path))


def format_score(value: Optional[float]) -> str:
    if value is None:
        return "未复测"
    text = f"{value:.3f}"
    text = text.rstrip("0").rstrip(".")
    return text if text else "0"


def format_delta(value: Optional[float]) -> str:
    if value is None:
        return "-"
    sign = "+" if value > 1e-12 else ""
    text = f"{value:.3f}".rstrip("0").rstrip(".")
    if text == "-0":
        text = "0"
    return f"{sign}{text}"


def overtake_status(new_score: Optional[float], openclaw_score: Optional[float]) -> str:
    if new_score is None:
        return "未复测"
    if openclaw_score is None:
        return "无对照"
    if abs(new_score - openclaw_score) <= 1e-9:
        return "持平"
    if new_score > openclaw_score:
        return "是"
    return "否"


def average(values: Iterable[Optional[float]]) -> Optional[float]:
    nums = [value for value in values if value is not None]
    if not nums:
        return None
    return sum(nums) / len(nums)


def build_rows(reference_tasks: Sequence[Dict[str, Any]], attempts: Sequence[ReportAttempt]) -> Tuple[List[Dict[str, Any]], Dict[str, Any]]:
    attempts_by_task: Dict[str, List[ReportAttempt]] = {}
    for attempt in attempts:
        if attempt.task_id == "":
            continue
        attempts_by_task.setdefault(attempt.task_id, []).append(attempt)

    ordered_rows: List[Dict[str, Any]] = []
    reference_ids = set()
    for task in reference_tasks:
        task_id = str(task.get("task_id") or "").strip()
        if task_id == "":
            continue
        reference_ids.add(task_id)
        task_attempts = attempts_by_task.get(task_id, [])
        latest_attempt = choose_latest_attempt(task_attempts)
        latest_scored = choose_latest_scored_attempt(task_attempts)
        new_score = latest_scored.score if latest_scored else None
        old_score = as_float(task.get("blue_old_score"))
        openclaw_score = as_float(task.get("openclaw_score"))

        status = "未复测"
        source = "-"
        if latest_scored is not None:
            status = "已复测"
            source = latest_scored.report_path
            if latest_attempt is not None and (
                latest_attempt.report_path != latest_scored.report_path
                or latest_attempt.generated_at != latest_scored.generated_at
            ):
                status = "已复测，较新尝试未出分"
        elif latest_attempt is not None:
            status = "已跑未出分"
            source = latest_attempt.report_path

        ordered_rows.append(
            {
                "task_id": task_id,
                "task_name": task.get("task_name") or (latest_attempt.task_name if latest_attempt else ""),
                "grading_type": task.get("grading_type") or "",
                "blue_old_score": old_score,
                "blue_new_verified_score": new_score,
                "openclaw_score": openclaw_score,
                "blue_delta": (new_score - old_score) if new_score is not None and old_score is not None else None,
                "vs_openclaw_delta": (new_score - openclaw_score) if new_score is not None and openclaw_score is not None else None,
                "overtake": overtake_status(new_score, openclaw_score),
                "status": status,
                "source_report": source,
                "generated_at": latest_scored.generated_at if latest_scored else (latest_attempt.generated_at if latest_attempt else ""),
                "latest_execution_status": latest_attempt.execution_status if latest_attempt else "",
                "latest_grade_error": latest_attempt.grade_error if latest_attempt else "",
            }
        )

    extra_task_ids = sorted(set(attempts_by_task) - reference_ids)
    for task_id in extra_task_ids:
        task_attempts = attempts_by_task[task_id]
        latest_attempt = choose_latest_attempt(task_attempts)
        latest_scored = choose_latest_scored_attempt(task_attempts)
        ordered_rows.append(
            {
                "task_id": task_id,
                "task_name": latest_attempt.task_name if latest_attempt else "",
                "grading_type": "",
                "blue_old_score": None,
                "blue_new_verified_score": latest_scored.score if latest_scored else None,
                "openclaw_score": None,
                "blue_delta": None,
                "vs_openclaw_delta": None,
                "overtake": "无对照",
                "status": "报告中存在但参考表未收录",
                "source_report": latest_scored.report_path if latest_scored else (latest_attempt.report_path if latest_attempt else "-"),
                "generated_at": latest_scored.generated_at if latest_scored else (latest_attempt.generated_at if latest_attempt else ""),
                "latest_execution_status": latest_attempt.execution_status if latest_attempt else "",
                "latest_grade_error": latest_attempt.grade_error if latest_attempt else "",
            }
        )

    scored_rows = [row for row in ordered_rows if row["blue_new_verified_score"] is not None]
    summary = {
        "reference_task_count": len(reference_tasks),
        "row_count": len(ordered_rows),
        "scored_task_count": len(scored_rows),
        "overtaken_count": sum(1 for row in ordered_rows if row["overtake"] == "是"),
        "tied_count": sum(1 for row in ordered_rows if row["overtake"] == "持平"),
        "behind_count": sum(1 for row in ordered_rows if row["overtake"] == "否"),
        "unverified_count": sum(1 for row in ordered_rows if row["blue_new_verified_score"] is None),
        "avg_blue_old_on_scored_tasks": average(row["blue_old_score"] for row in scored_rows),
        "avg_blue_new_on_scored_tasks": average(row["blue_new_verified_score"] for row in scored_rows),
        "avg_openclaw_on_scored_tasks": average(row["openclaw_score"] for row in scored_rows),
        "avg_delta_vs_old": average(row["blue_delta"] for row in scored_rows),
        "avg_delta_vs_openclaw": average(row["vs_openclaw_delta"] for row in scored_rows),
    }
    return ordered_rows, summary


def render_markdown(
    title: str,
    rows: Sequence[Dict[str, Any]],
    summary: Dict[str, Any],
    reference_path: Path,
    used_reports: Sequence[Path],
    model_contains: str,
) -> str:
    generated_at = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    lines: List[str] = [
        f"# {title}",
        "",
        f"- 生成时间: `{generated_at}`",
        f"- 参考分文件: `{reference_path}`",
        f"- 合并报告数: `{len(used_reports)}`",
    ]
    if model_contains.strip():
        lines.append(f"- 模型过滤: `model` 包含 `{model_contains.strip()}`")
    lines.extend(
        [
        "- 口径说明: `Blue 新 verified 分` 取 `judge-on` 报告里的最新有分结果；如果较新的尝试未出分，会在状态列标注。",
        "",
        "## 汇总",
        "",
        "| 指标 | 数值 |",
        "|---|---:|",
        f"| 参考任务总数 | {summary['reference_task_count']} |",
        f"| 已有 Blue 新 verified 分的任务数 | {summary['scored_task_count']} |",
        f"| 反超 OpenClaw | {summary['overtaken_count']} |",
        f"| 持平 OpenClaw | {summary['tied_count']} |",
        f"| 落后 OpenClaw | {summary['behind_count']} |",
        f"| 未复测 / 未出分 | {summary['unverified_count']} |",
        f"| 已出分任务平均 Blue 旧分 | {format_score(summary['avg_blue_old_on_scored_tasks']) if summary['avg_blue_old_on_scored_tasks'] is not None else '-'} |",
        f"| 已出分任务平均 Blue 新 verified 分 | {format_score(summary['avg_blue_new_on_scored_tasks']) if summary['avg_blue_new_on_scored_tasks'] is not None else '-'} |",
        f"| 已出分任务平均 OpenClaw 分 | {format_score(summary['avg_openclaw_on_scored_tasks']) if summary['avg_openclaw_on_scored_tasks'] is not None else '-'} |",
        f"| 已出分任务平均提升（新分-旧分） | {format_delta(summary['avg_delta_vs_old']) if summary['avg_delta_vs_old'] is not None else '-'} |",
        f"| 已出分任务平均差距（新分-OpenClaw） | {format_delta(summary['avg_delta_vs_openclaw']) if summary['avg_delta_vs_openclaw'] is not None else '-'} |",
        "",
        "## 对比表",
        "",
        "| Task ID | 任务 | Blue 旧分 | Blue 新 verified 分 | OpenClaw 分 | 提升 | 距 OpenClaw | 是否反超 | 状态 | 来源报告 |",
        "|---|---|---:|---:|---:|---:|---:|---|---|---|",
        ]
    )
    for row in rows:
        lines.append(
            "| {task_id} | {task_name} | {old} | {new} | {openclaw} | {delta_old} | {delta_oc} | {overtake} | {status} | {source} |".format(
                task_id=row["task_id"],
                task_name=str(row["task_name"] or "").replace("|", "\\|"),
                old=format_score(row["blue_old_score"]) if row["blue_old_score"] is not None else "-",
                new=format_score(row["blue_new_verified_score"]) if row["blue_new_verified_score"] is not None else "未复测",
                openclaw=format_score(row["openclaw_score"]) if row["openclaw_score"] is not None else "-",
                delta_old=format_delta(row["blue_delta"]),
                delta_oc=format_delta(row["vs_openclaw_delta"]),
                overtake=row["overtake"],
                status=str(row["status"]).replace("|", "\\|"),
                source=str(row["source_report"]).replace("|", "\\|"),
            )
        )

    if used_reports:
        lines.extend(
            [
                "",
                "## 使用报告",
                "",
            ]
        )
        for report in sorted({path.name for path in used_reports}):
            lines.append(f"- `{report}`")

    return "\n".join(lines) + "\n"


def main() -> int:
    args = parse_args()
    reference_path = Path(args.reference).resolve()
    report_paths = expand_report_paths(args.reports)
    if not report_paths:
        raise SystemExit("no report files matched --reports")

    reference_tasks = load_reference(reference_path)
    attempts, used_reports = load_attempts(
        report_paths,
        include_judge_off=args.include_judge_off,
        model_contains=args.model_contains,
    )
    rows, summary = build_rows(reference_tasks, attempts)

    md = render_markdown(args.title, rows, summary, reference_path, used_reports, args.model_contains)
    output_md = Path(args.output_md).resolve()
    output_json = Path(args.output_json).resolve()
    output_md.parent.mkdir(parents=True, exist_ok=True)
    output_json.parent.mkdir(parents=True, exist_ok=True)

    output_md.write_text(md, encoding="utf-8")
    output_json.write_text(
        json.dumps(
            {
                "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                "reference_path": str(reference_path),
                "used_reports": [str(path) for path in used_reports],
                "model_contains": args.model_contains,
                "summary": summary,
                "rows": rows,
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    print(f"Wrote markdown report to {output_md}")
    print(f"Wrote JSON report to {output_json}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
