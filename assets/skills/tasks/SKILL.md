---
name: tasks
version: "1.0.0"
description: "Manage a real task list through the external Things 3 CLI on macOS. Use when the user explicitly wants task-app operations rather than reminders, cron automation, or an in-chat checklist."
invocation: "blue exec command='things today'"
examples:
  - "blue exec command='things today'"
  - "blue exec command='things add \"Buy milk\" --when today'"
capability_tags:
  - tasks
  - productivity
  - external-cli
interaction_mode: stateless
card_support: none
category: external_cli
os:
  - darwin
environment:
  - things
homepage: https://github.com/ossianhempel/things3-cli
metadata:
  zimaos-blue:
    emoji: "✅"
    requires:
      bins:
        - things
    install:
      - id: go
        kind: go
        module: github.com/ossianhempel/things3-cli/cmd/things@latest
        bins:
          - things
        label: Install things3-cli (go)
---

# Tasks

Use the external `things` binary via `blue exec` to read and update a real Things 3 task list on macOS.

## Setup

Install the CLI if needed:

```bash
blue exec command='GOBIN=/opt/homebrew/bin go install github.com/ossianhempel/things3-cli/cmd/things@latest'
```

Sanity-check the install:

```bash
blue exec command='things --help'
```

If database reads fail, grant **Full Disk Access** to the calling app (for example Terminal or your IDE).

Optional environment:

- `THINGSDB` or `--db` to point at a specific `ThingsData-*` folder
- `THINGS_AUTH_TOKEN` to avoid passing `--auth-token` for update operations

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Read inbox/today/upcoming/search/projects in Things 3 | `blue exec command='things ...'` |
| Add or update real tasks in the user's Things database | `blue exec command='things add ...'` / `blue exec command='things update ...'` |
| User wants a reminder/alert notification | Use `reminder`, not `tasks` |
| User wants recurring command automation | Use `scheduler`, not `tasks` |
| User only needs an in-chat checklist or plan | Prefer normal chat output or the plan skills, not `tasks` |

---

## Command Usage

Read from Things:

```bash
blue exec command='things inbox --limit 50'
blue exec command='things today'
blue exec command='things upcoming'
blue exec command='things search "query"'
blue exec command='things projects'
```

Add tasks:

```bash
blue exec command='things --dry-run add "Title"'
blue exec command='things add "Title" --notes "..." --when today --deadline 2026-01-02'
blue exec command='things add "Book flights" --list "Travel"'
blue exec command='things add "Call dentist" --tags "health,phone"'
```

Update tasks (auth token required):

```bash
blue exec command='things search "milk" --limit 5'
blue exec command='things update --id <UUID> --auth-token <TOKEN> --completed'
blue exec command='things update --id <UUID> --auth-token <TOKEN> --notes "New notes"'
```

---

## Notes

- This is an external CLI guide, not a native persistent `blue tasks` builtin.
- Earlier docs that promised an in-memory built-in `tasks` fallback were outdated and have been removed.
- `things3-cli` does not currently expose a true delete or trash write command; use the Things UI or mark items completed/canceled instead.
- macOS only.
