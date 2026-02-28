---
name: workflows
description: "Create and operate reusable workflow automations (create/list/get/execute/enable/disable/delete). Use when the user asks for multi-step automation, repeatable orchestration, or workflow lifecycle management."
---

# Workflows Skill

## Setup

No external dependencies required. Uses built-in workflow engine.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Create new automation workflow | `workflows.create` |
| Inspect existing workflow definitions | `workflows.list` / `workflows.get` |
| Execute workflow now | `workflows.execute` |
| Pause/resume workflow | `workflows.disable` / `workflows.enable` |
| Remove obsolete workflow | `workflows.delete` |

---

## Command Usage

```bash
workflows.create name="Daily Digest" description="Summarize important updates every morning"
workflows.list
workflows.get id=wf_abc123
workflows.execute id=wf_abc123
workflows.disable id=wf_abc123
workflows.enable id=wf_abc123
workflows.delete id=wf_abc123
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing required `name` or `id` | Provide required fields per action |
| Unknown workflow ID | Use `workflows.list` to find valid IDs |
| Execution failed | Inspect workflow definition and retry |

---

## Notes

- Use workflows for reusable automation logic, not ad-hoc one-off commands.
