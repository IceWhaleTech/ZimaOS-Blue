---
name: deep_research
version: "1.0.0"
description: "Legacy entry for the unified Research family. Research covers three modes: deep_research for citation-first investigation, analyze for bounded synthesis/report generation, and ui_review for UI/UX or accessibility review."
invocation: "blue deep_research query=\"ZimaOS-Blue deep research architecture\" --json"
examples:
  - "blue deep_research query=\"ZimaOS-Blue deep research architecture\" --json"
  - "blue deep_research mode=analyze topic=\"Product feedback analysis\" urls='[\"https://example.com/reviews\"]' --json"
  - "blue deep_research mode=ui_review url=https://example.com/pricing review_action=check_accessibility --json"
  - "blue deep_research action=status job_id=job_123"
capability_tags:
  - research
  - citations
  - web
interaction_mode: stateless
card_support: both
---

# Research

This document describes the unified Research family.

The file and manifest name stay `deep_research` for compatibility, but the runtime concept is now one research surface with three execution modes:

- `mode=deep_research`: citation-first, multi-source investigation
- `mode=analyze`: bounded synthesis over provided text, URLs, and/or search queries
- `mode=ui_review`: UI/UX or accessibility review for a live URL or screenshot

## How To Send

Use the retained compatibility entry:

```bash
blue deep_research query="ZimaOS-Blue deep research architecture" --json
```

When the runtime exposes the canonical `research` entry directly, treat it as the same family and use the same arguments.

## Routing Guide

| Need | Recommended mode | What it is best at |
|------|------------------|--------------------|
| Multi-source research with evidence, citations, comparisons, or timeline work | `deep_research` | Investigation with planning, retrieval, evidence merging, and confidence-backed answers |
| One synthesized report from known materials | `analyze` | Bounded synthesis over provided text, URLs, and search queries |
| UI/UX, screenshot, or accessibility evaluation | `ui_review` | Visual and accessibility review for URLs or images |
| Local workspace files or README inspection | Not this family | Prefer `exec` / file tools instead |

## Common Parameters

- `action` (optional): `run` or `status`
- `query` (required for normal research runs unless another mode-specific input supplies the goal)
- `mode` (optional): `auto`, `deep_research`, `analyze`, or `ui_review`
- `lang` (optional): output language such as `zh-CN` or `en-US`
- `report_style` (optional): output style hint such as `summary`, `timeline`, or other mode-supported variants

Important:

- `mode` chooses the research family branch.
- `research_depth` chooses the depth only inside `mode=deep_research`.
- Do not overload `mode` with `fast|standard|deep`; use `research_depth` for that.

## Mode: `deep_research`

Use this for citation-first investigation.

Example:

```bash
blue deep_research query="Compare ZimaOS and CasaOS with citations and a timeline" mode=deep_research research_depth=deep --json
```

Key parameters:

- `query` (required)
- `research_depth` (optional): `fast`, `standard`, or `deep`
- `strict_entity` (optional): `true|false`
- `time_windows` (optional): array of timeline windows
- `report_style` (optional): typically `summary` or `timeline`

Behavior:

1. Plan sub-questions
2. Retrieve and compare evidence
3. Merge and deduplicate sources
4. Return answer, citations, confidence, and evidence count

## Mode: `analyze`

Use this for a bounded report when the materials are already known or can be collected with a small number of URLs/searches.

Examples:

```bash
blue deep_research mode=analyze topic="Product feedback analysis" urls='["https://example.com/reviews"]' search_queries='["product reviews 2026"]' --json
blue deep_research mode=analyze topic="Survey insights" text="..." lang=en-US --json
```

Key parameters:

- `topic` (usually required for bounded synthesis)
- `urls` (optional, max bounded set)
- `search_queries` (optional, bounded supplemental discovery)
- `text` (optional)
- `output_mode` (optional): `inline` or `report`

Use `analyze` when:

- the user wants one synthesized report
- the source set is already mostly known
- citations are not the primary requirement

## Mode: `ui_review`

Use this for visual quality or accessibility review of a webpage or screenshot.

Examples:

```bash
blue deep_research mode=ui_review url=https://example.com/pricing --json
blue deep_research mode=ui_review url=https://example.com/pricing review_action=check_accessibility --json
blue deep_research mode=ui_review image=<base64_png_data> review_action=review_image --json
```

Key parameters:

- `url` for live-page review
- `image` for screenshot review
- `review_action` (optional): `review_url`, `review_image`, or `check_accessibility`
- `device`, `channel`, `wait_ms`, `threshold`, `format`, `profile` (optional review controls)

Important:

- Top-level `action` is reserved for `run` / `status`.
- Use `review_action` for the UI-review sub-action.

## Status / Resume

Poll an existing research-family job with:

```bash
blue deep_research action=status job_id=job_123
```

## Output

Depending on mode, returns structured data such as:

- `answer`
- `citations`
- `confidence`
- `evidence_count`
- report metadata
- UI review findings and scores

## Notes

- Think of this as one research family, not three unrelated tools.
- Prefer `mode=deep_research` for citation-first work.
- Prefer `mode=analyze` for one bounded synthesized report.
- Prefer `mode=ui_review` for URL/screenshot quality review.
- If the task is only quick link discovery, prefer `web_query`.
- If the task is about local files already in the workspace, prefer `exec` / file tools.

## Example Triggers

- "帮我做一个带引用的调研结论"
- "Compare these claims with citations and a timeline"
- "Summarize these links into one report"
- "Review the UI of this page for accessibility and layout issues"
