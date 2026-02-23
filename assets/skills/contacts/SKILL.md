# Contacts

Manage contacts and address book.

## Google Contacts via `gog`

Install `gog` to access Google Contacts:

```bash
brew install steipete/tap/gogcli
```

### Setup (once)

```bash
gog auth credentials /path/to/client_secret.json
gog auth add you@gmail.com --services contacts
gog auth list
```

### Commands

- List contacts: `gog contacts list --max 20`
- Search: `gog contacts search "query"`

Set `GOG_ACCOUNT=you@gmail.com` to avoid repeating `--account`.

## Fallback

Without `gog`, contacts are stored in-memory (lost on restart). Use the built-in `contacts` tool with actions: `create`, `read`, `update`, `delete`, `list`, `search`.
