---
name: reminder
description: "Schedule and manage reminders (add/list/delete/clear) with multi-channel delivery. Use when the user asks to be reminded at a specific time, recurring cadence, or to manage existing reminder tasks."
---

# Reminder Skill

## Setup

No external dependencies required. Uses built-in reminder scheduler and delivery channels.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Create reminder for future time | `reminder.add` |
| View pending reminders | `reminder.list` |
| Delete one reminder | `reminder.delete` |
| Remove all reminders | `reminder.clear` |

---

## Command Usage

```bash
reminder.add message="Check the build" time=30m
reminder.add message="Team standup" time="2026-03-01 09:00" recurring=daily
reminder.list
reminder.delete id=push_abc123
reminder.clear
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
| Missing/unknown reminder `id` | Use `reminder.list` to find valid ID |

---

## Notes

- Delivery may include conversation injection, SSE, web push, and native OS alerts depending on availability.
- Use `scheduler` for cron-style command automation; use `reminder` for user-facing reminder alerts.
