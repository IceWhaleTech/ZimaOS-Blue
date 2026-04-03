---
name: web_query
version: "1.0.0"
description: "Unified web discovery and reading entry point. Use when the user asks to find references, official docs, latest links, or read a public page from either a query or URL."
invocation: "blue web_query input=\"OpenAI Responses API docs\""
examples:
  - "blue web_query input=\"OpenAI Responses API docs\""
  - "blue web_query input=\"ZimaOS Blue release notes\" max_results=8"
capability_tags:
  - search
  - web
  - docs
interaction_mode: stateless
card_support: none
---

# Web Search Skill

## Setup

No external dependencies required. Uses built-in web search capability.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Need relevant links/sources quickly and do not yet have the right URL | `blue web_query input=...` |
| Need official docs/reference pages | `blue web_query` with precise query terms |
| Already have a concrete public URL and only need page content | `blue web_query input="https://..."` |
| Need page interaction/login/JS rendering | Search first, then switch to `browser` |

---

## Command Usage

```bash
blue web_query input="ZimaOS Blue release notes"
blue web_query input="OpenAI Responses API function calling" max_results=8
blue web_query input="container sandbox security best practices" max_results=10
blue web_query input="https://platform.openai.com/docs/api-reference/responses"
```

Parameters:
- `input` (required; search query or public URL)
- `max_results` (optional, default 10, max 20)
- `max_chars` (optional, only for page reads)
- `depth` (optional, `quick`, `standard`, `deep`)

---

## Error Handling

| Error | Resolution |
|-------|------------|
| `input is required` | Provide a non-empty search query or URL |
| Public page requires login or interaction | Switch to `browser` |

---

## Notes

- `web_query` is the canonical public web skill.
- Legacy `web_search`, `web_fetch`, and `web_read` names are compatibility aliases and should not be used as the primary route in new prompts or harness cases.
- If `web_query` reports login wall, challenge, or browser-required warnings, switch to `browser`.
