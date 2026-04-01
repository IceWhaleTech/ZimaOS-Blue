---
name: timer
version: "0.1.0"
description: "Disabled placeholder for session-local countdown timers. ZimaOS Blue does not currently register a live builtin timer skill at runtime."
invocation: "blue timer action=start duration=5m"
examples:
  - "blue timer action=start duration=5m"
capability_tags:
  - timer
  - countdown
interaction_mode: stateless
card_support: none
enabled: false
category: internal
tags:
  - timer
  - countdown
---

# Timer

This skill is currently disabled.

ZimaOS Blue does not currently register a live builtin `timer` skill at runtime, so this document is kept only as a placeholder.

## Use Instead

- Use `reminder` for user-facing alerts and notifications.
- Use `scheduler` for recurring or cron-style command execution.

## Planned Behavior

If a real timer implementation is restored later, it should be clearly documented as:

- Session-local or in-memory only
- Distinct from persistent reminders
- Distinct from recurring command automation
