---
name: browser
version: "1.0.0"
description: "Interact with live web pages using the built-in browser tool (navigate, inspect, click/type, screenshot, tab management). Use when the user asks to open/read a URL, extract page content, fill forms, click elements, reproduce web behavior, or capture screenshots."
invocation: "blue browser navigate url=https://example.com"
examples:
  - "blue browser navigate url=https://example.com"
  - "blue browser screenshot url=https://example.com"
capability_tags:
  - browser
  - web
  - automation
interaction_mode: interactive
card_support: streaming
---

# Browser Skill

## Setup

No external dependencies required. Uses built-in browser commands.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Open a URL and inspect page content | `blue browser navigate` -> `blue browser snapshot` or `blue browser snapshot_interactive` |
| Click/type/select on page elements | `blue browser snapshot*` to get `@ref`, then `blue browser act` |
| Capture screenshot evidence | `blue browser screenshot` |
| Inspect or switch active browser tabs | `blue browser tabs` |

---

## Command Usage

The CLI accepts `blue browser <action> ...` as the primary form. Dotted aliases such as `blue browser.navigate ...` remain supported for compatibility.

Common compatibility shorthands also work:

```bash
blue browser https://example.com
blue browser open https://example.com
blue browser.navigate https://example.com
blue browser click @5
blue browser type @8 "hello"
blue browser close tab-1
```

### browser navigate

```bash
blue browser navigate url=https://example.com
```

### browser snapshot

```bash
blue browser snapshot target_id=ABCDEF123456
```

### browser snapshot_interactive

```bash
blue browser snapshot_interactive target_id=ABCDEF123456
```

### browser act

```bash
blue browser act ref=5 act_type=click
blue browser act ref=8 act_type=type value="hello"
blue browser act ref=12 act_type=select value="option_a"
```

### browser screenshot

```bash
blue browser screenshot url=https://example.com
blue browser screenshot target_id=ABCDEF123456
blue browser screenshot
```

### browser tabs

```bash
blue browser tabs
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| URL invalid/unreachable | Verify URL and retry with full `https://` form |
| Missing `@ref` for act | Run snapshot first and use returned `@ref` |
| Element not interactable | Refresh snapshot and retry with correct visible element |

---

## Notes

- Use `browser` for real page interaction, not keyword discovery.
- `blue browser screenshot` without args captures the active tab.
- Browser refs may be passed as either `5` or `@5`.
- Common aliases are normalized for compatibility, for example `open -> navigate`, `list -> tabs`, and direct verbs like `click/type/select -> act`.
- For simple keyword lookup or public-page reads, prefer `web_query` first.
- For known public URLs that only need content, prefer the lighter unified web read/fetch surface first. When exposed, this may appear as compatibility actions such as `web_fetch` or `web_read`.
- Treat `browser` as the final fallback when lighter web tools are insufficient.
