# Scheduler

Create, list, delete, trigger, enable, and disable cron jobs. Jobs execute shell commands on a cron schedule.

## How to Send

Use the `blue` CLI:

```bash
blue cron.create name=health_check schedule="*/10 * * * *" command="curl -s http://localhost:8080/health"
```

Add `--json` for JSON output.

## Commands

### cron.create

Create a new cron job.

**Required:** `name`, `schedule`, `command`
**Optional:** `description`

**Cron expression format (5-field):**
- `*/5 * * * *` — every 5 minutes
- `0 9 * * *` — daily at 9:00 AM
- `0 0 * * 1` — every Monday at midnight
- `0 */2 * * *` — every 2 hours

```bash
blue cron.create name=health_check schedule="*/10 * * * *" command="curl -s http://localhost:8080/health" description="Check service health every 10 minutes"
```

### cron.list

List all cron jobs.

```bash
blue cron.list
```

### cron.delete

Delete a cron job by ID.

**Required:** `id`

```bash
blue cron.delete id=cron_abc123
```

### cron.trigger

Manually trigger a cron job immediately.

**Required:** `id`

```bash
blue cron.trigger id=cron_abc123
```

### cron.enable

Enable a disabled cron job.

**Required:** `id`

```bash
blue cron.enable id=cron_abc123
```

### cron.disable

Disable a cron job without deleting it.

**Required:** `id`

```bash
blue cron.disable id=cron_abc123
```

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `missing name` — no `name` key for create
- `missing schedule` — no `schedule` key for create
- `missing command` — no `command` key for create
- `missing id` — no `id` key for delete/trigger/enable/disable

## Allowed Commands

For security, only whitelisted shell commands are permitted: `echo`, `date`, `uptime`, `df`, `free`, `ps`, `curl`, `wget`.

## Example Triggers

- "Run a health check every 10 minutes"
- "每10分钟检查一次服务状态"
- "Schedule a daily backup at 2am"
- "每天凌晨2点执行备份"
