---
name: datetime
version: "0.1.0"
description: "Disabled placeholder for date/time/timezone helpers. ZimaOS Blue does not currently register a dedicated builtin datetime skill in the live runtime."
enabled: false
category: internal
tags:
  - datetime
  - time
  - timezone
---

# DateTime

This skill is currently disabled.

Earlier drafts documented `action=now|format|convert` as if `datetime` were a live builtin contract. In the current ZimaOS Blue runtime, there is no dedicated builtin `datetime` skill registration backing that interface.

## Use Instead

- For exact current time/date values, use normal runtime-aware answers or `blue exec command='date ...'` when a shell-derived value is required.
- For timezone conversions, use `blue exec` with platform date tooling or another verified source.
- If a real `datetime` skill is restored later, its invocation contract should be documented explicitly and covered by tests.
