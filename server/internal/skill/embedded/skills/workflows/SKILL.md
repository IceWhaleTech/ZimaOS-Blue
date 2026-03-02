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
| Create new automation workflow | `blue workflows.create` |
| Inspect existing workflow definitions | `blue workflows.list` / `blue workflows.get` |
| Execute workflow now | `blue workflows.execute` |
| Pause/resume workflow | `blue workflows.disable` / `blue workflows.enable` |
| Remove obsolete workflow | `blue workflows.delete` |

---

## Command Usage

```bash
blue workflows.create name="Daily Digest" description="Summarize important updates every morning"
blue workflows.list
blue workflows.get id=wf_abc123
blue workflows.execute id=wf_abc123
blue workflows.disable id=wf_abc123
blue workflows.enable id=wf_abc123
blue workflows.delete id=wf_abc123
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing required `name` or `id` | Provide required fields per action |
| Unknown workflow ID | Use `blue workflows.list` to find valid IDs |
| Execution failed | Inspect workflow definition and retry |

---

## Notes

- Use workflows for reusable automation logic, not ad-hoc one-off commands.
