---
name: sandbox
description: "Execute commands in an isolated sandbox with limits and lifecycle controls. Use when running untrusted/high-risk commands, validating scripts safely, or when the user explicitly asks for sandboxed execution."
---

# Sandbox Skill

## Setup

No external dependencies required. Uses built-in sandbox runtime.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Run command safely in isolation | `sandbox.execute` |
| Check running/completed sandbox job status | `sandbox.status` |
| Stop runaway or unwanted sandbox job | `sandbox.kill` |
| Check sandbox availability/capabilities | `sandbox.info` |

---

## Command Usage

```bash
sandbox.execute command=sh args='["-lc","date"]' timeout=20
sandbox.status id=exec_abc123
sandbox.kill id=exec_abc123
sandbox.info
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing `command` for execute | Provide executable command |
| Missing `id` for status/kill | Pass valid execution ID |
| Sandbox service unavailable | Fall back to approved host execution path if allowed |

---

## Notes

- Prefer sandbox for medium/high-risk execution paths.
- Keep commands minimal and non-interactive.
