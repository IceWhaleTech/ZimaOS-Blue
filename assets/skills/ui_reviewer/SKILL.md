---
name: ui_reviewer
version: "1.0.0"
description: "Legacy wrapper for the unified Research family. Prefer Research with `mode=ui_review` for UI/UX, screenshot, and accessibility review."
invocation: "blue deep_research mode=ui_review url=https://example.com/pricing --json"
examples:
  - "blue deep_research mode=ui_review url=https://example.com/pricing --json"
  - "blue deep_research mode=ui_review url=https://example.com/pricing review_action=check_accessibility --json"
capability_tags:
  - ui
  - ux
  - accessibility
interaction_mode: stateless
card_support: both
---

# UI Reviewer (Legacy Wrapper)

This file is kept for compatibility.

Prefer the unified Research family and use `ui_review` as a mode, not `ui_reviewer` as the primary long-term standalone entry.

## Preferred Usage

```bash
blue deep_research mode=ui_review url=https://example.com/pricing --json
blue deep_research mode=ui_review image=<base64_png_data> review_action=review_image --json
```

If the runtime exposes the canonical `research` entry directly, treat it as the same family and use the same `mode=ui_review` arguments there.

## What `mode=ui_review` Is For

Use `mode=ui_review` when the user wants:

- UI/UX critique of a live webpage
- screenshot-based visual review
- accessibility-only evaluation

This mode is best for quality evaluation, not for generic browsing or general web search.

## Recommended Arguments

- `url`: live page to review
- `image`: screenshot to review
- `review_action`: `review_url`, `review_image`, or `check_accessibility`
- `device`, `channel`, `wait_ms`, `threshold`, `format`, `profile`: optional review controls

Examples:

```bash
blue deep_research mode=ui_review url=https://example.com/pricing review_action=review_url --json
blue deep_research mode=ui_review url=https://example.com/pricing review_action=check_accessibility --json
blue deep_research mode=ui_review image=<base64_png_data> review_action=review_image --json
```

Important:

- Top-level `action` is reserved for Research family control such as `run` and `status`.
- Use `review_action` for the UI-review sub-action.

## Route Elsewhere When

- Need generic browsing, login flows, or live page interaction:
  use `browser`
- Need citation-first or comparison-heavy research:
  use `mode=deep_research`
- Need one bounded synthesized report:
  use `mode=analyze`

## Compatibility Notes

- Older prompts, wrappers, or internal routes may still refer to `ui_reviewer`.
- Treat that surface as compatibility-only and map it to Research with `mode=ui_review`.
- Prefer the unified Research family wording in new docs, prompts, and examples.
