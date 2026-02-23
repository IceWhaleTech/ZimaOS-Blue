---
name: reminders
description: Create, list, and manage reminders with time-based triggers. Persisted in SQLite, fires via cron. Native OS notifications — macOS (Apple Reminders + Notification Center), Linux (notify-send), Windows (toast).
metadata:
  {
    "openclaw":
      {
        "emoji": "⏰",
      },
  }
---

# Reminders

Create, list, and manage reminders with time-based triggers. Reminders are persisted in SQLite and fire via cron scheduling, surviving restarts.

## Parameters

- `action`: add, list, delete, clear (required)
- `message`: Reminder text (required for add)
- `time`: Relative (1h, 30m, 2h30m) or absolute (2026-01-04 09:00) or RFC3339
- `id`: Reminder ID (required for delete)
- `recurring`: daily, weekly, monthly (optional)
- `session_id`: Target conversation ID (optional)

## Time Formats

- Relative: `1h`, `30m`, `2h30m`
- Absolute: `2026-01-04 09:00`, `2026-01-04 15:04:05`
- RFC3339: `2026-01-04T09:00:00+08:00`

## Native OS Notifications

When a reminder fires, a native OS notification is delivered in addition to the web SSE event:

- **macOS**: Creates an entry in Apple Reminders.app + Notification Center banner (via AppleScript, no external CLI needed)
- **Linux**: Desktop notification via `notify-send` (silent no-op on headless servers)
- **Windows**: Toast notification via PowerShell WinRT (Windows 10+, no external deps)

macOS note: First use may prompt for Reminders permission in System Settings > Privacy & Security > Reminders.

## CLI

```bash
blue remind add "Buy groceries" --in 1h --token $BLUE_TOKEN
blue remind add "Team meeting" --at "2026-02-24 09:00" --token $BLUE_TOKEN
blue remind list --token $BLUE_TOKEN
blue remind delete <id> --token $BLUE_TOKEN
blue remind clear --token $BLUE_TOKEN
```

## Optional: remindctl (advanced macOS)

For advanced Apple Reminders management (lists, date views, editing), install `remindctl`:

- Install: `brew install steipete/tap/remindctl`
- View today: `remindctl today`
- Add with list: `remindctl add --title "Call mom" --list Personal --due tomorrow`
- JSON output: `remindctl today --json`
