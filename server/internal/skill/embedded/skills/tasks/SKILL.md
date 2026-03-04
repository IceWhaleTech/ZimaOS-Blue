---
name: tasks
description: Create manage and track tasks with optional Things 3 integration on macOS.
---

# Tasks

Create, manage, and track tasks with priorities and status.

## macOS: Things 3 via `things`

On macOS with Things 3 installed, use the `things` CLI:

```bash
GOBIN=/opt/homebrew/bin go install github.com/ossianhempel/things3-cli/cmd/things@latest
```

If DB reads fail, grant **Full Disk Access** to the calling app (Terminal / your IDE).

### Read (DB)

- `things inbox --limit 50`
- `things today`
- `things upcoming`
- `things search "query"`
- `things projects` / `things areas` / `things tags`

### Write (URL scheme)

- Add: `things add "Title" --notes "..." --when today --deadline 2026-01-02`
- Into project: `things add "Book flights" --list "Travel"`
- With tags: `things add "Call dentist" --tags "health,phone"`
- Checklist: `things add "Trip prep" --checklist-item "Passport" --checklist-item "Tickets"`
- Preview: `things --dry-run add "Title"`

### Modify (needs auth token)

- Get ID: `things search "milk" --limit 5`
- Set `THINGS_AUTH_TOKEN` or pass `--auth-token <TOKEN>`
- Complete: `things update --id <UUID> --auth-token <TOKEN> --completed`

### Notes

- macOS only
- `--dry-run` prints the URL without opening Things

## Fallback

Without `things`, tasks are stored in-memory (lost on restart). Use the built-in `tasks` tool with actions: `create`, `read`, `update`, `delete`, `list`, `complete`, `reopen`.
