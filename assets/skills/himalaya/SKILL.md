---
name: himalaya
description: Manage real email accounts from the terminal with the Himalaya CLI using IMAP, SMTP, Notmuch, or Sendmail backends.
tags: ["email", "mail", "imap", "smtp", "notmuch", "maildir", "cli", "himalaya"]
category: productivity
environment: ["himalaya"]
os: ["darwin", "linux"]
homepage: https://github.com/pimalaya/himalaya
metadata:
  {
    "openclaw":
      {
        "requires": { "bins": ["himalaya"] },
        "install":
          [
            {
              "id": "brew",
              "kind": "brew",
              "formula": "himalaya",
              "bins": ["himalaya"],
              "label": "Install Himalaya (brew)",
            },
          ],
      },
  }
---

# Himalaya Email CLI

Use `himalaya` when the user wants to work with a real mailbox from the terminal instead of benchmark fixtures or workspace email files.

Himalaya is a CLI email client that supports IMAP, SMTP, Notmuch, and Sendmail backends. It can list folders, search envelopes, read full messages, compose drafts, send replies, move/archive mail, manage flags, and download attachments.

## Prefer This Skill When

- The user explicitly mentions `himalaya`
- The task is about a real inbox, IMAP/SMTP account, or terminal email workflow
- The user wants a CLI path for listing, reading, replying to, archiving, or sending emails
- The user is using multiple mail accounts and wants account-aware commands

Do not prefer this skill for benchmark-local email fixtures already stored in the workspace like `inbox/` unless the user explicitly asks for Himalaya or a real mail backend.

## Prerequisites

1. Verify the CLI exists:

```bash
himalaya --version
```

2. Configure an account interactively:

```bash
himalaya account configure
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
| List folders/accounts | `himalaya folder list` / `himalaya account list` |
| Search inbox or unread mail | `himalaya envelope list --output json ...` |
| Read a message body | `himalaya message read <id>` |
| Export raw MIME | `himalaya message export <id> --full` |
| Reply, forward, or compose | `himalaya message reply <id>` / `himalaya message forward <id>` / `himalaya message write` |
| Send a prepared message | `himalaya template send` |
| Archive or move mail | `himalaya message move <id> "Archive"` |
| Download attachments | `himalaya attachment download <id>` |

## Common Commands

List inbox envelopes:

```bash
himalaya envelope list
```

List envelopes with machine-readable output:

```bash
himalaya envelope list --output json
```

Search by sender or subject:

```bash
himalaya envelope list from alice@example.com subject invoice --output json
```

Read a message:

```bash
himalaya message read 42
```

Read raw MIME:

```bash
himalaya message export 42 --full
```

Reply:

```bash
himalaya message reply 42
```

Reply-all:

```bash
himalaya message reply 42 --all
```

Compose and send from stdin:

```bash
cat <<'EOF' | himalaya template send
From: you@example.com
To: recipient@example.com
Subject: Quick update

Hello from Himalaya.
EOF
```

Move or archive:

```bash
himalaya message move 42 "Archive"
```

Add or remove flags:

```bash
himalaya flag add 42 --flag seen
himalaya flag remove 42 --flag seen
```

Download attachments:

```bash
himalaya attachment download 42 --dir ~/Downloads
```

Use a specific account:

```bash
himalaya --account work envelope list --output json
```

## Guidance

- Prefer `--output json` whenever structured parsing is useful.
- Prefer `message read` for user-facing summaries and `message export --full` only when MIME inspection is necessary.
- Use `--account <name>` when the default account is not the intended mailbox.
- For sending rich or multi-line email content, prefer `template send` or a body file over inline shell quoting.
