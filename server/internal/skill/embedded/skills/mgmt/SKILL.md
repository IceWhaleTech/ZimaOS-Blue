---
name: mgmt
version: "1.0.0"
description: "Perform administrative/system operations: provider management, settings, channel/skill/tool toggles, user controls, health/version, and proxy stats. Use for ops/admin tasks, diagnostics, and runtime configuration changes."
invocation: "blue mgmt.providers.list"
examples:
  - "blue mgmt.providers.list"
  - "blue mgmt.settings.get"
capability_tags:
  - admin
  - settings
  - diagnostics
interaction_mode: stateless
card_support: none
---

# Mgmt Skill

## Setup

No external dependencies required. Uses built-in management API bridge.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Manage LLM providers/models/connectivity | `blue mgmt.providers.*` |
| Read/update runtime settings | `blue mgmt.settings.*` |
| Check channels/skills/tools status and toggle | `blue mgmt.channels.*` / `blue mgmt.skills.*` / `blue mgmt.tools.*` |
| Get system health/version/proxy stats | `blue mgmt.system.*` / `blue mgmt.proxy.*` |
| Manage user lock/unlock operations | `blue mgmt.users.*` |

---

## Command Usage

```bash
blue mgmt.providers.list
blue mgmt.providers.test id=provider_abc
blue mgmt.settings.get
blue mgmt.settings.set key=agent_mode value=true
blue mgmt.channels.list
blue mgmt.skills.list
blue mgmt.tools.list
blue mgmt.system.health
blue mgmt.proxy.stats
blue mgmt.users.list
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Unknown domain/action | Use supported `blue mgmt.<domain>.<action>` form |
| Missing required parameter | Provide required `id/key/value/name` fields |
| Permission denied | Use authorized account/context |

---

## Notes

- `mgmt` is for system administration and diagnostics; avoid using it for normal user conversation tasks.
- `mgmt` is intentionally a namespaced admin umbrella (`mgmt.<domain>.<action>`) so low-frequency operational controls stay in one place instead of being split across many tiny operator-only skills.
- Prefer domain skills like `browser`, `web_query`, `analyze`, `deep_research`, `reminder`, or `scheduler` for ordinary user-facing work; use `mgmt` only when the task is explicitly about runtime state, configuration, or diagnostics.
