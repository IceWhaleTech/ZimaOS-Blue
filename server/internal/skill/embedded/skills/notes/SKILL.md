# Notes

Create, read, update, search, and delete notes with tags.

## macOS: Apple Notes via `memo`

On macOS, install `memo` to manage Apple Notes directly:

```bash
brew tap antoniorodr/memo && brew install antoniorodr/memo/memo
```

If prompted, grant Automation access to Notes.app in System Settings > Privacy & Security > Automation.

### Commands

- List all notes: `memo notes`
- Filter by folder: `memo notes -f "Folder Name"`
- Search notes: `memo notes -s "query"`
- Add a note: `memo notes -a "Note Title"`
- Edit a note: `memo notes -e`
- Delete a note: `memo notes -d`
- Move to folder: `memo notes -m`
- Export: `memo notes -ex`

### Limitations

- Cannot edit notes containing images or attachments
- Interactive prompts require terminal access
- macOS only

## Fallback

Without `memo`, notes are stored in-memory (lost on restart). Use the built-in `notes` tool with actions: `create`, `read`, `update`, `delete`, `list`, `search`.
