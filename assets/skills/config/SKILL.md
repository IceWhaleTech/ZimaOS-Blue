---
name: config
version: "1.0.0"
description: "Perform administrative/system operations: provider management, settings, channel/skill/tool toggles, user controls, health/version, and proxy stats. Use for ops/admin tasks, diagnostics, and runtime configuration changes."
model_invocable: false
invocation: "blue config.providers.list"
examples:
  - "blue config.providers.list"
  - "blue config.settings.get"
capability_tags:
  - admin
  - settings
  - diagnostics
interaction_mode: stateless
card_support: none
---

# Config Skill

## Setup

No external dependencies required. Uses built-in management API bridge.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Manage LLM providers/models | `blue config.providers.*` |
| Read/update runtime settings | `blue config.settings.*` |
| Check channels/skills/tools status and toggle | `blue config.channels.*` / `blue config.skills.*` / `blue config.tools.*` |
| Get system health/version/proxy stats | `blue config.system.*` / `blue config.proxy.*` |
| Manage user lock/unlock operations | `blue config.users.*` |

---

## Command Usage

```bash
blue config.providers.list
blue config.settings.get
blue config.settings.set key=agent_mode value=true
blue config.channels.list
blue config.skills.list
blue config.tools.list
blue config.system.health
blue config.proxy.stats
blue config.users.list
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Unknown domain/action | Use supported `blue config.<domain>.<action>` form |
| Missing required parameter | Provide required `id/key/value/name` fields |
| Permission denied | Use authorized account/context |

---

## Notes

- `config` is for system administration and diagnostics; avoid using it for normal user conversation tasks.
- `config` is intentionally a namespaced admin umbrella (`config.<domain>.<action>`) so low-frequency operational controls stay in one place instead of being split across many tiny operator-only skills.
- Legacy `mgmt` requests may still resolve here as a compatibility alias, but `config` is the canonical surface.
- Prefer domain skills like `browser`, `web_query`, `analyze`, `deep_research`, `reminder`, or `scheduler` for ordinary user-facing work; use `config` only when the task is explicitly about runtime state, configuration, or diagnostics.
