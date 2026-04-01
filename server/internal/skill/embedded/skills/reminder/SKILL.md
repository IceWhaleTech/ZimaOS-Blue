---
name: reminder
version: "1.0.0"
description: "Schedule and manage reminders (add/list/delete/clear) with multi-channel delivery. Use when the user asks to be reminded at a specific time, recurring cadence, or to manage existing reminder tasks."
invocation: "blue reminder.add message=\"Check the build\" time=30m"
examples:
  - "blue reminder.add message=\"Check the build\" time=30m"
  - "blue reminder.list"
capability_tags:
  - reminder
  - schedule
  - notify
interaction_mode: stateless
card_support: batch
---

# Reminder Skill

## Setup

No external dependencies required. Uses built-in reminder scheduler and delivery channels.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Create reminder for future time | `blue reminder.add` |
| View pending reminders | `blue reminder.list` |
| Delete one reminder | `blue reminder.delete` |
| Remove all reminders | `blue reminder.clear` |

---

## Command Usage

```bash
blue reminder.add message="Check the build" time=30m
blue reminder.add message="Team standup" time="2026-03-01 09:00" recurring=daily
blue reminder.list
blue reminder.delete id=push_abc123
blue reminder.clear
```

Time formats:
- Relative: `1h`, `30m`, `2h30m`
- Absolute: `2026-03-01 09:00`
- RFC3339: `2026-03-01T09:00:00+08:00`

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing `message` | Provide reminder message |
| Missing/invalid `time` | Use supported time format |
| Missing/unknown reminder `id` | Use `blue reminder.list` to find valid ID |

---

## Notes

- Delivery may include conversation injection, SSE, web push, and native OS alerts depending on availability.
- Prefer `blue reminder.*` command style to avoid model/tool pre-check mismatches in some providers.
- Use `reminder` for user-facing alerts and notifications that should reach the user later.
- Use `scheduler` for cron-style command automation; it runs commands, not reminder notifications.
- This skill is not a generic recurring shell automation surface.
