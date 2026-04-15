---
name: pptx
version: "1.0.0"
description: "Use when the task centers on a workspace .pptx file and needs native slide creation, editing, slide mutation, or chart updates."
invocation: "blue pptx action=create path=decks/launch_plan.pptx title='Launch Plan' sections='[{\"heading\":\"Overview\",\"bullets\":[\"Status\",\"Risks\",\"Timeline\"]}]'"
examples:
  - "blue pptx action=create path=decks/launch_plan.pptx title='Launch Plan' sections='[{\"heading\":\"Overview\",\"bullets\":[\"Status\",\"Risks\",\"Timeline\"]}]'"
  - "blue pptx action=edit path=decks/launch_plan.pptx replacements='{\"{{owner}}\":\"Platform Team\"}'"
  - "blue pptx action=duplicate_slide path=decks/launch_plan.pptx slide=2"
  - "blue pptx action=update_chart_data path=decks/launch_plan.pptx slide=2 chart_index=1 chart='{\"type\":\"bar\",\"categories\":[\"Q1\",\"Q2\"],\"series\":[{\"name\":\"Revenue\",\"values\":[12,18]}]}'"
  - "blue pptx action=validate_template path=decks/launch_plan.pptx"
capability_tags:
  - documents
  - office
  - pptx
  - slides
interaction_mode: stateless
card_support: none
---

# PPTX

Use this skill when the `.pptx` deck is the source of truth and the task needs native slide layout, slide mutations, or chart-aware updates.

## Common Actions

```bash
blue pptx action=create path=decks/launch_plan.pptx title="Launch Plan" sections='[{"heading":"Overview","bullets":["Status","Risks","Timeline"]}]'
blue pptx action=edit path=decks/launch_plan.pptx replacements='{"{{owner}}":"Platform Team"}'
blue pptx action=duplicate_slide path=decks/launch_plan.pptx slide=2
blue pptx action=update_chart_data path=decks/launch_plan.pptx slide=2 chart_index=1 chart='{"type":"bar","categories":["Q1","Q2"],"series":[{"name":"Revenue","values":[12,18]}]}'
blue pptx action=validate_template path=decks/launch_plan.pptx
```

## Markdown Input

- `action=create` accepts direct Markdown through `content`, `markdown`, `body`, or `text`.
- Headings become slide titles, lists become bullets, and Markdown tables can render into slide tables.

## When To Use

- The final artifact must be `.pptx`.
- The task needs native slide ordering, duplication, deletion, or text replacement.
- The task needs structured chart updates inside the deck.

## Boundaries

- Use `generate_image` only for standalone slide visuals or image assets, not for editable `.pptx` files.
- Use `docx` for document-style reports and `xlsx` for spreadsheets.
- Use `computer_use` for host presentation-app UI work such as menus, toolbars, dialogs, or screenshots.
