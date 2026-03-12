---
id: anthropic/messages-api
type: doc
description: Notes for cache-friendly system prompts and structured tool guidance in Anthropic-style runtimes.
source_trust: official
tags: [anthropic, messages, prompt-cache, tools, system-prompt]
languages: [en]
revision: "1"
updated_on: "2026-03-11"
---

# Anthropic Messages API Notes

- Split static, config, and dynamic prompt blocks when possible.
- Keep cacheable sections stable across turns.
- Dynamic task context should be injected in an uncached turn-specific block.
- Tool guidance belongs in stable prompt/config layers; retrieved context belongs in dynamic layers.
