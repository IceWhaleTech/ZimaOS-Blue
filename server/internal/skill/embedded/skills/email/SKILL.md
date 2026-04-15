---
name: email
version: "1.0.0"
description: "Run the external email CLI through exec for real IMAP/SMTP/Notmuch mailboxes. Use when the user explicitly wants a real terminal email workflow instead of the built-in benchmark email surfaces."
invocation: "blue exec command='himalaya envelope list --output json'"
examples:
  - "blue exec command='himalaya envelope list --output json'"
  - "blue exec command='himalaya --account work message read 42'"
capability_tags:
  - email
  - external-cli
  - mail
interaction_mode: stateless
card_support: none
tags: ["email", "mail", "imap", "smtp", "notmuch", "maildir", "cli", "himalaya"]
category: productivity
environment: ["himalaya"]
os: ["darwin", "linux"]
homepage: https://github.com/pimalaya/himalaya
metadata:
  openclaw:
    requires:
      bins:
        - himalaya
    install:
      - id: brew
        kind: brew
        formula: himalaya
        bins:
          - himalaya
        label: Install Himalaya (brew)
---

# Email

Run the external `himalaya` binary via `blue exec` when the task is about a real mailbox.

Himalaya is a CLI email client that supports IMAP, SMTP, Notmuch, and Sendmail backends. It can list folders, search envelopes, read full messages, compose drafts, send replies, move/archive mail, manage flags, and download attachments.

## Prefer This Skill When

- The user explicitly mentions `himalaya`
- The user explicitly mentions `email`
- The task is about a real inbox, IMAP/SMTP account, or terminal email workflow
- The user wants a CLI path for listing, reading, replying to, archiving, or sending emails
- The user is using multiple mail accounts and wants account-aware commands

Do not prefer this skill for benchmark-local email fixtures already stored in the workspace like `inbox/` unless the user explicitly asks for Himalaya or a real mail backend.

## Prerequisites

1. Verify the CLI exists:

```bash
blue exec command="himalaya --version"
```

2. Configure an account interactively:

```bash
blue exec command="himalaya account configure"
```

3. Or create `~/.config/himalaya/config.toml` manually:

```toml
[accounts.personal]
email = "you@example.com"
display-name = "Your Name"
default = true

backend.type = "imap"
backend.host = "imap.example.com"
backend.port = 993
backend.encryption.type = "tls"
backend.login = "you@example.com"
backend.auth.type = "password"
backend.auth.cmd = "pass show email/imap"

message.send.backend.type = "smtp"
message.send.backend.host = "smtp.example.com"
message.send.backend.port = 587
message.send.backend.encryption.type = "start-tls"
message.send.backend.login = "you@example.com"
message.send.backend.auth.type = "password"
message.send.backend.auth.cmd = "pass show email/smtp"
```

## Task Routing

| User Intent | Action |
|-------------|--------|
| List folders/accounts | `blue exec command='himalaya folder list'` / `blue exec command='himalaya account list'` |
| Search inbox or unread mail | `blue exec command='himalaya envelope list --output json ...'` |
| Read a message body | `blue exec command='himalaya message read <id>'` |
| Export raw MIME | `blue exec command='himalaya message export <id> --full'` |
| Reply, forward, or compose | `blue exec command='himalaya message reply <id>'` / `blue exec command='himalaya message forward <id>'` / `blue exec command='himalaya message write'` |
| Send a prepared message | `blue exec command='himalaya template send'` |
| Archive or move mail | `blue exec command='himalaya message move <id> \"Archive\"'` |
| Download attachments | `blue exec command='himalaya attachment download <id>'` |

## Common Commands

List inbox envelopes:

```bash
blue exec command="himalaya envelope list"
```

List envelopes with machine-readable output:

```bash
blue exec command="himalaya envelope list --output json"
```

Search by sender or subject:

```bash
blue exec command="himalaya envelope list from alice@example.com subject invoice --output json"
```

Read a message:

```bash
blue exec command="himalaya message read 42"
```

Read raw MIME:

```bash
blue exec command="himalaya message export 42 --full"
```

Reply:

```bash
blue exec command="himalaya message reply 42"
```

Reply-all:

```bash
blue exec command="himalaya message reply 42 --all"
```

Compose and send from stdin:

```bash
blue exec command="printf 'From: you@example.com\nTo: recipient@example.com\nSubject: Quick update\n\nHello from Himalaya.\n' | himalaya template send"
```

Move or archive:

```bash
blue exec command="himalaya message move 42 Archive"
```

Add or remove flags:

```bash
blue exec command="himalaya flag add 42 --flag seen"
blue exec command="himalaya flag remove 42 --flag seen"
```

Download attachments:

```bash
blue exec command="himalaya attachment download 42 --dir ~/Downloads"
```

Use a specific account:

```bash
blue exec command="himalaya --account work envelope list --output json"
```

## Guidance

- Prefer `--output json` whenever structured parsing is useful.
- Prefer `message read` for user-facing summaries and `message export --full` only when MIME inspection is necessary.
- Use `--account <name>` when the default account is not the intended mailbox.
- For sending rich or multi-line email content, prefer `template send` or a body file over inline shell quoting.
- Older prompts may still refer to `himalaya`; treat that as a compatibility alias for Email.
- This is an external CLI guide, not a native `blue email` subcommand.
- Prefer the built-in email skill for local benchmark fixtures; prefer Himalaya only for real mailbox operations.
