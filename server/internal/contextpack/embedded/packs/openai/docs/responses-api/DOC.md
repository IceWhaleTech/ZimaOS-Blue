---
id: openai/responses-api
type: doc
description: Working notes for OpenAI Responses-style tool calling and continuation patterns.
source_trust: official
tags: [openai, responses, tool-calling, api, reasoning]
languages: [en]
revision: "1"
updated_on: "2026-03-11"
references:
  - references/tools.md
---

# OpenAI Responses API Notes

Use this pack when implementing or debugging Responses-style workflows.

## Key ideas

- Treat response state and continuation as first-class runtime concepts.
- Separate system instructions, user content, and tool results clearly.
- Preserve stable tool names and structured JSON arguments.
- Prefer compact tool outputs when multiple search or retrieval rounds occur.

## Blue-specific guidance

- Map tool results to audit-friendly JSON.
- Keep prompt-added context separate from durable memory.
- Use context packs for provider/API usage notes instead of storing them in `MEMORY.md`.
