---
name: analyze
version: "1.0.0"
description: "Legacy wrapper for the unified Research family. Prefer Research with `mode=analyze` for bounded synthesis and report generation over known materials."
invocation: "blue deep_research mode=analyze topic=\"Product feedback analysis\" --json"
examples:
  - "blue deep_research mode=analyze topic=\"Product feedback analysis\" --json"
  - "blue deep_research mode=analyze topic=\"Survey insights\" text=\"...\" lang=en-US --json"
capability_tags:
  - analysis
  - synthesis
  - report
interaction_mode: stateless
card_support: both
---

# Analyze (Legacy Wrapper)

This file is kept for compatibility.

Prefer the unified Research family and use `analyze` as a mode, not as the primary long-term standalone entry.

## Preferred Usage

```bash
blue deep_research mode=analyze topic="Product feedback analysis" --json
blue deep_research mode=analyze topic="Survey insights" text="..." lang=en-US --json
```

If the runtime exposes the canonical `research` entry directly, treat it as the same family and use the same `mode=analyze` arguments there.

## What `mode=analyze` Is For

Use `mode=analyze` when the user wants one synthesized result over a bounded source set, for example:

- a report over a few known URLs
- a summary over provided text
- one synthesized answer that may optionally produce a report artifact

This mode is best when:

- citations are not the primary requirement
- the source set is already known or tightly bounded
- the task is report-oriented rather than investigation-oriented

## Recommended Arguments

- `topic`: report title or analysis focus
- `urls`: bounded source URLs
- `search_queries`: supplemental discovery queries
- `text`: direct source text
- `output_mode`: `inline` or `report`
- `lang`: output language

Example:

```bash
blue deep_research mode=analyze topic="Product feedback analysis" urls='["https://example.com/reviews"]' search_queries='["product reviews 2026"]' output_mode=report --json
```

## Route Elsewhere When

- Need citations, evidence, comparisons, or timeline research:
  use `mode=deep_research`
- Need UI/UX, screenshot, or accessibility review:
  use `mode=ui_review`
- Need local workspace file inspection:
  use `exec` / file tools instead of the research family
- Need quick link discovery only:
  use `web_query`

## Compatibility Notes

- Older prompts, wrappers, or internal routes may still refer to `analyze`.
- Treat that surface as compatibility-only and map it to Research with `mode=analyze`.
- Prefer the unified Research family wording in new docs, prompts, and examples.
