# Deep Search

Run multi-step research with planning, evidence collection, and citation-based summary.

## How to Send

Use direct skill call (prefer `--json`):

```bash
deep_search query="ZimaOS-Blue deep search architecture" --json
```

## Parameters

- `query` (required): Research question or topic
- `mode` (optional): `fast`, `standard`, `deep` (default `standard`)
- `lang` (optional): Output language (e.g. `zh-CN`, `en-US`)

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
- "Do a deep search on this topic"
- "给我一个带引用的调研结论"
