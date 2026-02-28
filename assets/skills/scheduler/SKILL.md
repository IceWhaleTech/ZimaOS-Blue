---
name: scheduler
description: "Manage recurring cron schedules for commands (create/list/trigger/enable/disable/delete). Use when the user asks to run tasks automatically on a time schedule, periodic checks, or recurring maintenance jobs."
---

# Scheduler Skill

## Setup

No external dependencies required. Uses built-in cron scheduler.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Create recurring timed task | `cron.create` |
| List existing schedules | `cron.list` |
| Run schedule immediately for testing | `cron.trigger` |
| Temporarily stop/resume schedule | `cron.disable` / `cron.enable` |
| Remove schedule permanently | `cron.delete` |

---

## Command Usage

```bash
cron.create name=health_check schedule="*/10 * * * *" command="mgmt system.health" description="Check service health every 10 minutes"
cron.list
cron.trigger id=cron_abc123
cron.disable id=cron_abc123
cron.enable id=cron_abc123
cron.delete id=cron_abc123
```

Cron format examples:
- `*/5 * * * *` every 5 minutes
- `0 9 * * *` daily at 09:00
- `0 0 * * 1` every Monday

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Invalid cron expression | Validate 5-field cron syntax and retry |
| Missing required field (`name/schedule/command`) | Provide all required fields |
| Unknown schedule ID | Use `cron.list` to get valid IDs |

---

## Notes

- Use scheduler for recurring execution; for one-time reminders use `reminder`.
