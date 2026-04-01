---
name: self_reflect
version: "1.0.0"
description: "Extract grounded lessons learned from a completed task and optionally write reusable experience entries into memory. Primarily used by the runtime or agent as a post-task reflection step."
invocation: "blue self_reflect goal=\"Fix parser regression\" final_status=completed result_summary=\"Patched parser and passed focused verification\" --json"
examples:
  - "blue self_reflect goal=\"Fix parser regression\" final_status=completed result_summary=\"Patched parser and passed focused verification\" --json"
  - "blue self_reflect goal=\"Investigate flaky test\" final_status=failed failure_reason=\"Timeout in CI\" --json"
capability_tags:
  - reflection
  - lessons
  - memory
interaction_mode: stateless
card_support: none
category: internal
tags:
  - reflection
  - memory
  - internal
---

# Self Reflect

Summarize a finished task, extract grounded reusable lessons, and persist them into memory when that backend is available.

This skill is mostly for runtime-driven post-task reflection. It is not usually the first choice for ordinary end-user requests.

## Setup

No external dependencies required. This skill needs the runtime self-reflection backend to be wired; otherwise execution returns a backend-unavailable error.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Agent/runtime wants an automatic post-task reflection pass | `blue self_reflect ...` |
| User explicitly asks to review lessons learned from a completed task | `blue self_reflect ...` |
| Need reusable memory-worthy lessons rather than a normal prose recap | `blue self_reflect ...` |
| User only wants a plain summary or retrospective in chat | Prefer a normal answer unless structured reflection output is needed |

---

## Command Usage

Basic successful reflection:

```bash
blue self_reflect goal="Fix parser regression" final_status=completed result_summary="Patched parser and passed focused verification" --json
```

Failure reflection:

```bash
blue self_reflect goal="Investigate flaky test" final_status=failed failure_reason="Timeout in CI" --json
```

With plan and verification context:

```bash
blue self_reflect goal="Refactor scheduler docs" final_status=completed result_summary="Updated docs and tests" verification_output="go test ./internal/skillmanifest passed" --json
```

Advanced proposal/review mode is also supported through additional fields such as `source_kind`, `source_id`, `evaluation_summary`, `proposal_candidates`, and `proposal_mode`.

---

## Parameters

- `goal`: Required for normal reflection flows.
- `task_id`: Optional task identifier for memory tagging.
- `plan`: Optional step list with `description`, `status`, and `output`.
- `step_outputs`: Optional step outputs merged into `plan` by index.
- `verification_output`: Optional verification stage output.
- `final_status`: Required for normal reflection flows; must be `completed`, `failed`, or `partial`.
- `result_summary`: Optional short outcome summary.
- `failure_reason`: Optional failure reason.
- `source_kind`, `source_id`, `evaluation_summary`, `proposal_candidates`, `proposal_mode`: Optional advanced proposal inputs used by internal review workflows.

---

## Output

Returns structured JSON including:

- `summary`
- `lessons`
- `memory_written`
- `skipped_reason`
- `proposal_count`
- `proposal_ids`
- `proposal_skipped_reason`

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing `goal` for a normal reflection flow | Provide `goal`, or use proposal inputs if running proposal-only mode |
| Missing or invalid `final_status` | Use one of `completed`, `failed`, or `partial` |
| Self-reflection backend not available | Run in a runtime where the reflection backend is wired |

---

## Notes

- "When available" means the reflection backend can write into the configured memory layer only when that memory service is present in the current runtime.
- In normal agent flows this skill is often invoked automatically after task completion rather than explicitly requested by the user.
- Use this skill for structured lessons extraction, not as the default way to write a normal summary for the user.

## Example Triggers

- "帮我复盘一下这次任务"
- "Extract lessons learned from this task"
- "总结一下这次失败里能复用的经验"
