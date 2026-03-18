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
| Need relevant links/sources quickly and do not yet have the right URL | `blue web_search query=...` |
| Need official docs/reference pages | `blue web_search` with precise query terms |
| Already have a concrete public URL and only need page content | Prefer `web_fetch` or `web_read`, not `web_search` |
| Need page interaction/login/JS rendering | Search first, then switch to `browser` |

---

## Command Usage

```bash
blue web_search query="ZimaOS Blue release notes"
blue web_search query="OpenAI Responses API function calling" max_results=8
blue web_search query="container sandbox security best practices" max_results=10 region=us-en
blue web_search query="OpenAI Responses API" format=xml
```

Parameters:
- `query` (required)
- `max_results` (optional, default 10, max 20)
- `region` (optional, e.g. `us-en`, `wt-wt`)
- `format` (optional, `json` or `xml`, default `xml`)

---

## Error Handling

| Error | Resolution |
|-------|------------|
| `query is required` | Provide non-empty `query` |
| Search backend unavailable | Retry later or reduce query complexity |

---

## Notes

- `web_search` is the discovery step when you do not yet have the right URL.
- After choosing a URL, prefer `web_fetch` for quick public reads, `web_read` for normalized content reads, and `browser` as the final fallback.
