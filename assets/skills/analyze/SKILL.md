---
name: analyze
version: "1.0.0"
description: "Run synthesized analysis on a topic by aggregating URLs, search results, and/or raw text, then generate a structured report with insights and recommendations. Use when the user wants a report, summary, comparison, or synthesis over known materials rather than a citation-first research workflow."
invocation: "blue analyze topic=\"Product feedback analysis\" --json"
examples:
  - "blue analyze topic=\"Product feedback analysis\" --json"
  - "blue analyze topic=\"Survey insights\" text=\"...\" lang=en-US --json"
capability_tags:
  - analysis
  - synthesis
  - report
interaction_mode: stateless
card_support: both
---

# Analyze Skill

## Setup

No external dependencies required. Uses built-in data collection + analysis + report generation pipeline.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Full deep-dive report on a topic | `blue analyze topic=...` with `urls` and/or `search_queries` |
| Analyze only provided text | `blue analyze topic=... text=...` |
| Need quick link discovery only | Use `web_query` instead of `analyze` |

---

## Command Usage

```bash
blue analyze topic="Product feedback analysis" urls='["https://example.com/reviews"]' search_queries='["product reviews 2026"]' --json
```

```bash
blue analyze topic="Survey insights" text="..." lang=en-US --json
```

Parameters:
- `topic` (required)
- `urls` (optional, max 5)
- `search_queries` (optional, max 3)
- `text` (optional)
- `lang` (optional)

---

## Error Handling

| Error | Resolution |
|-------|------------|
| `topic is required` | Provide a concrete topic |
| No data collected | Add `urls`, `search_queries`, or `text` |
| Analysis backend unavailable | Retry later or reduce scope |

---

## Notes

- Use `analyze` when the user expects synthesized insights, not just raw search output.
- Output is report-oriented and heavier than simple QA responses.
- Prefer `analyze` for provided text or a bounded set of URLs/search queries that need one synthesized report.
- For local workspace files or README inspection, use an explicit local-file route such as `exec`/file tools instead of `analyze`.
- If the user explicitly asks for citations, evidence, multi-source comparison, or timeline-oriented research, prefer `deep_research`.
