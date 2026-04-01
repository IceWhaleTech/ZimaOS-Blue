---
name: deep_research
version: "1.0.0"
description: "Run a multi-step cited research workflow with planning, evidence collection, and confidence-backed summaries. Use when the user explicitly wants citations, evidence, multiple sources, comparisons, or timeline-oriented research."
invocation: "blue deep_research query=\"ZimaOS-Blue deep research architecture\" --json"
examples:
  - "blue deep_research query=\"ZimaOS-Blue deep research architecture\" --json"
  - "blue deep_research action=status job_id=job_123"
capability_tags:
  - research
  - citations
  - web
interaction_mode: stateless
card_support: both
---

# Deep Research

Run multi-step research with planning, evidence collection, and citation-based summary.

## How to Send

Use direct skill call (prefer `--json`):

```bash
blue deep_research query="ZimaOS-Blue deep research architecture" --json
blue deep_research query="ZimaOS-Blue deep research architecture" format=xml
```

## Parameters

- `query` (required): Research question or topic
- `mode` (optional): `fast`, `standard`, `deep` (default `standard`)
- `lang` (optional): Output language (e.g. `zh-CN`, `en-US`)
- `strict_entity` (optional): `true|false`, enable strict same-entity filtering
- `time_windows` (optional): timeline windows labels (array)
- `report_style` (optional): `summary` or `timeline`
- `format` (optional): `json` or `xml` (default `json`)

## Behavior

1. Plan sub-questions
2. Run parallel web retrieval
3. Merge + deduplicate evidence
4. Return answer with citations and confidence

## Output

Returns structured JSON including:
- `answer`
- `citations`
- `confidence`
- `evidence_count`

## Notes

- Prefer `deep_research` when the user explicitly wants multi-source evidence, citations, comparisons, timelines, or a research workflow that can be resumed via job status.
- If the task is mainly to synthesize provided text or known URLs into a report, prefer `analyze`.
- If the task is about local workspace files or README inspection, prefer an explicit local-file route such as `exec`/file tools.

## Example Triggers

- "帮我深度搜索一下这个技术方案"
- "Do a deep research on this topic"
- "给我一个带引用的调研结论"
