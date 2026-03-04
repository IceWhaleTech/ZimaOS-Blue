---
name: calendar
description: Manage calendar events and schedules using Google Calendar via gog CLI.
---

# Calendar

Manage calendar events and schedules.

## Google Calendar via `gog`

Install `gog` to access Google Calendar:

```bash
brew install steipete/tap/gogcli
```

### Setup (once)

```bash
gog auth credentials /path/to/client_secret.json
gog auth add you@gmail.com --services calendar
gog auth list
```

### Commands

- List events: `gog calendar events <calendarId> --from <iso> --to <iso>`
- Create event: `gog calendar create <calendarId> --summary "Title" --from <iso> --to <iso>`
- Create with color: `gog calendar create <calendarId> --summary "Title" --from <iso> --to <iso> --event-color 7`
- Update event: `gog calendar update <calendarId> <eventId> --summary "New Title"`
- Show colors: `gog calendar colors`

### Event Colors (IDs 1-11)

Use `--event-color <id>`: 1=#a4bdfc, 2=#7ae7bf, 3=#dbadff, 4=#ff887c, 5=#fbd75b, 6=#ffb878, 7=#46d6db, 8=#e1e1e1, 9=#5484ed, 10=#51b749, 11=#dc2127

Set `GOG_ACCOUNT=you@gmail.com` to avoid repeating `--account`.

## Fallback

Without `gog`, calendar events are stored in-memory (lost on restart). Use the built-in `calendar` tool with actions: `create`, `read`, `update`, `delete`, `list`, `today`, `upcoming`.
