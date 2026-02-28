---
name: web_search
description: "Run keyword web search and return ranked result listings (title, URL, snippet). Use when the user asks to find references, official docs, sources, or latest links before opening pages in detail."
---

# Web Search Skill

## Setup

No external dependencies required. Uses built-in web search capability.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Find relevant links/sources quickly | `web_search query=...` |
| Need official docs/reference pages | `web_search` with precise query terms |
| Need page interaction/content extraction | Search first, then switch to `browser` |

---

## Command Usage

```bash
web_search query="ZimaOS Blue release notes"
web_search query="OpenAI Responses API function calling" max_results=8
web_search query="container sandbox security best practices" max_results=10 region=us-en
```

Parameters:
- `query` (required)
- `max_results` (optional, default 10, max 20)
- `region` (optional, e.g. `us-en`, `wt-wt`)

---

## Error Handling

| Error | Resolution |
|-------|------------|
| `query is required` | Provide non-empty `query` |
| Search backend unavailable | Retry later or reduce query complexity |

---

## Notes

- `web_search` returns result listings only; it does not open pages.
- Use `browser` to read or interact with a chosen URL.
