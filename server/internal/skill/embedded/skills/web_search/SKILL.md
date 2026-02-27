# Web Search

Keyword web search that returns result listings (title, URL, snippet).

## How to Send

Direct call:

```bash
web_search query="ZimaOS Blue release notes"
```

Add `--json` for structured output.

## Parameters

- `query` (required): Search query text
- `max_results` (optional): Maximum results (default 10, max 20)
- `region` (optional): Region code, for example `us-en`, `wt-wt`

```bash
web_search query="container sandbox security best practices" max_results=8 region=us-en
```

## Notes

- This skill only returns search listings.
- It does not open pages or extract full content.
- Use `browser` when you need to read/interact with a specific page.

## Error Response

Returns `status=error` when search fails.

Common errors:
- `query is required`
- `query must be a non-empty string`
- `web search not configured`

## Example Triggers

- "Search latest docs for this API"
- "Find official references for this topic"
- "帮我网页搜索这个关键词"
