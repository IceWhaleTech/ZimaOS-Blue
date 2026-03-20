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
| Create recurring timed task | `blue cron.create` |
| List existing schedules | `blue cron.list` |
| Run schedule immediately for testing | `blue cron.trigger` |
| Temporarily stop/resume schedule | `blue cron.disable` / `blue cron.enable` |
| Remove schedule permanently | `blue cron.delete` |

---

## Command Usage

```bash
blue cron.create name=uptime_check schedule="*/10 * * * *" command="uptime" description="Check system uptime every 10 minutes"
blue cron.list
blue cron.trigger id=cron_abc123
blue cron.disable id=cron_abc123
blue cron.enable id=cron_abc123
blue cron.delete id=cron_abc123
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
| Unknown schedule ID | Use `blue cron.list` to get valid IDs |

---

## Notes

- `command` uses the shell command handler and is validated at creation time.
- Currently allowed base commands are: `echo`, `date`, `uptime`, `df`, `free`, `ps`, `curl`, `wget`.
- Use scheduler for recurring execution; for one-time reminders use `reminder`.
