---
name: deep_research
description: Run multi step deep research with planning evidence collection and citation based summaries.
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

## Example Triggers

- "帮我深度搜索一下这个技术方案"
- "Do a deep research on this topic"
- "给我一个带引用的调研结论"
