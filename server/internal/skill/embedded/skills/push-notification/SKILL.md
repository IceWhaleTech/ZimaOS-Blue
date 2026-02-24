# Push Notification

Schedule and manage push notifications. When a notification fires, it delivers through multiple channels: conversation message injection (typeless alert card), SSE event, Web Push (for closed browser tabs), and native OS notifications (macOS Notification Center, Linux notify-send, Windows toast).

## How to Send

Use the `blue` CLI:

```bash
blue push.add message="Check the build" time=30m
```

Add `--json` for JSON output.

## Commands

### push.add

Schedule a push notification at a specific time.

**Required:** `message`, `time`

**Optional:**
- `recurring` — `daily`, `weekly`, `monthly`
- `session_id` — target conversation ID (defaults to most recent)

**Time formats:**
- Relative duration: `1h`, `30m`, `2h30m`
- Absolute: `2026-01-04 09:00`
- RFC3339: `2026-01-04T09:00:00+08:00`

```bash
blue push.add message="Team standup in 5 minutes" time=5m
```

### push.list

List all pending notifications.

```bash
blue push.list
```

### push.delete

Delete a notification by ID.

**Required:** `id`

**Optional:** `lang`

```bash
blue push.delete id=push_abc123
```

### push.clear

Delete all notifications.

```bash
blue push.clear
```

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `missing message` — no `message` key for add
- `missing time` — no `time` key for add
- `invalid time: ...` — unrecognized time format
- `missing id` — no `id` key for delete

## Delivery Channels

When a notification fires, it is delivered through all available channels:

| Channel | Description |
|---------|-------------|
| Conversation | Injected as a typeless alert card into the target conversation |
| SSE | Real-time `push` event to connected web clients |
| Web Push | Browser push notification for closed tabs |
| macOS | Notification Center banner via AppleScript |
| Linux | Desktop notification via `notify-send` |
| Windows | Toast notification via PowerShell WinRT |

## Example Triggers

- "Remind me to check the build in 30 minutes"
- "30分钟后提醒我检查构建"
- "Set a daily notification at 9am for standup"
- "每天早上9点提醒我站会"
