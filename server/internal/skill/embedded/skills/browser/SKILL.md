---
name: browser
version: "1.0.0"
description: "Live web interaction skill. Use Browser when the task needs an actual browser session for login, JS-heavy pages, clicking, typing, scrolling, screenshots, or challenge handling."
invocation: "blue browser navigate url=https://example.com"
examples:
  - "blue browser navigate url=https://example.com"
  - "blue browser screenshot url=https://example.com full_page=true"
  - "blue browser click browser_target_id=tab_123 selector=\"text=Sign in\""
capability_tags:
  - browser
  - web
  - interaction
interaction_mode: stateless
card_support: none
---

# Browser

Use `browser` when the job requires a live browser tab instead of ordinary fetch/read.

## Routing Guide

| Need | Action |
|------|--------|
| Public links, docs, or readable page content without interaction | Prefer `web_query` first |
| Logged-in readable page and existing session state is already available | Try authenticated read/fetch with `Cookie`, `Authorization`, or `browser_target_id` before full browser automation |
| Real interaction: click, type, scroll, login, form submit, screenshot, or challenge handling | Use `blue browser ...` |
| Existing Chrome session should be reused | Prefer relay/local Chrome or an existing browser target instead of a fresh anonymous tab |

## Command Usage

```bash
blue browser navigate url=https://example.com
blue browser screenshot url=https://example.com full_page=true
blue browser click browser_target_id=tab_123 selector="text=Sign in"
blue browser type browser_target_id=tab_123 selector="input[type=email]" text="user@example.com"
blue browser scroll browser_target_id=tab_123 y=1200
```

Common parameters vary by action, but typical fields include:

- `url` for opening a page
- `browser_target_id` for reusing an existing tab/session
- `selector` or target metadata for click/type operations
- `text` for typed input
- `full_page` for screenshots

## Notes

- `browser` is the canonical interactive web skill.
- Prefer `web_query` for discovery or straightforward readable content; escalate to `browser` only when a live session or interaction is truly required.
- If `web_query` reports `login_wall`, `challenge`, `browser_required`, or `retry_browser`, first retry with session/auth reuse when possible and then switch to `browser`.
