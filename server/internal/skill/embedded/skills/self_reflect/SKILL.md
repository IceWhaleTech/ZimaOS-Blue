---
name: self_reflect
description: Extract grounded lessons learned from a completed task and write reusable experience entries into memory.
---

# Self Reflect

Summarize a finished task, extract a few reusable lessons, and persist them into memory when available.

## How to Send

Use direct skill call:

```bash
blue self_reflect goal="Fix parser regression" final_status=completed result_summary="Patched parser and passed focused verification" --json
```

## Parameters

- `goal` (required): Task goal
- `task_id` (optional): Task identifier for memory tagging
- `plan` (optional): Step list with `description`, `status`, `output`
- `step_outputs` (optional): Parallel step outputs to merge by index
- `verification_output` (optional): Verification stage output
- `final_status` (required): `completed`, `failed`, or `partial`
- `result_summary` (optional): Short task outcome summary
- `failure_reason` (optional): Failure reason when the task did not succeed

## Behavior

1. Read the task record
2. Extract grounded reusable lessons
3. Filter generic or duplicate advice
4. Optionally write lessons into memory

## Output

Returns structured JSON including:
- `summary`
- `lessons`
- `memory_written`
- `skipped_reason`

## Example Triggers

- "帮我复盘一下这次任务"
- "Extract lessons learned from this task"
- "总结一下这次失败里能复用的经验"
