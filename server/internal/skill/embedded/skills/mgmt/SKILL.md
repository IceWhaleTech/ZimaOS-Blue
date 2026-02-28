---
name: mgmt
description: "Perform administrative/system operations: provider management, settings, channel/skill/tool toggles, user controls, health/version, and proxy stats. Use for ops/admin tasks, diagnostics, and runtime configuration changes."
---

# Mgmt Skill

## Setup

No external dependencies required. Uses built-in management API bridge.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Manage LLM providers/models/connectivity | `mgmt.providers.*` |
| Read/update runtime settings | `mgmt.settings.*` |
| Check channels/skills/tools status and toggle | `mgmt.channels.*` / `mgmt.skills.*` / `mgmt.tools.*` |
| Get system health/version/proxy stats | `mgmt.system.*` / `mgmt.proxy.*` |
| Manage user lock/unlock operations | `mgmt.users.*` |

---

## Command Usage

```bash
mgmt.providers.list
mgmt.providers.test id=provider_abc
mgmt.settings.get
mgmt.settings.set key=agent_mode value=true
mgmt.channels.list
mgmt.skills.list
mgmt.tools.list
mgmt.system.health
mgmt.proxy.stats
mgmt.users.list
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Unknown domain/action | Use supported `mgmt.<domain>.<action>` form |
| Missing required parameter | Provide required `id/key/value/name` fields |
| Permission denied | Use authorized account/context |

---

## Notes

- `mgmt` is for system administration and diagnostics; avoid using it for normal user conversation tasks.
