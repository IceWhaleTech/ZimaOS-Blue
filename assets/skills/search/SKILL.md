---
name: search
version: "0.1.0"
description: "Disabled placeholder for the deprecated search skill name. ZimaOS Blue now uses web_query as the canonical public web skill."
enabled: false
category: internal
tags:
  - search
  - deprecated
  - web
---

# Search

This skill is currently disabled.

The old `search` name is deprecated and should not be treated as a live runtime
skill in ZimaOS Blue.

## Use Instead

- Use `web_query` for public web discovery and public-page reads.
- Use `browser` only when interaction, login, screenshots, or JS-heavy pages are required.

## Notes

- `web_search` may still appear as a compatibility alias in older prompts or traces, but `web_query` is the canonical route for new prompts, manifests, and harness cases.
- If a dedicated `search` placeholder remains in prompts, treat it as documentation-only rather than an executable builtin contract.
