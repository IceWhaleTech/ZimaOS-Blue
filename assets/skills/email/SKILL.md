---
name: email
description: Send read and manage email via built in SMTP and IMAP actions with optional gog Gmail workflow.
---

# Email

Send, read, and manage emails via SMTP/IMAP.

## Usage

Use the built-in `email` tool with actions:

- `send`: Send an email. Params: `to`, `subject`, `body`, `cc`, `bcc`
- `list`: List emails. Params: `folder` (default: INBOX), `limit` (default: 10)
- `read`: Read an email. Params: `id`
- `delete`: Delete an email. Params: `id`
- `config`: Show current email configuration

## Configuration

Requires SMTP/IMAP settings:
- `smtp_host`, `smtp_port`, `smtp_user`, `smtp_password`
- `imap_host`, `imap_port`, `imap_user`, `imap_password`
- `from_address`, `use_tls`

## macOS: Gmail via `gog`

Alternatively, use `gog` for Gmail:

```bash
brew install steipete/tap/gogcli
gog auth add you@gmail.com --services gmail
```

- Send: `gog gmail send --to a@b.com --subject "Hi" --body "Hello"`
- Search: `gog gmail search 'newer_than:7d' --max 10`
- Draft: `gog gmail drafts create --to a@b.com --subject "Hi" --body-file ./msg.txt`
