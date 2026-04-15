---
name: docx
version: "1.0.0"
description: "Use when the task centers on a workspace .docx file and needs native Word-style creation, editing, templating, or validation."
invocation: "blue docx action=create path=reports/brief.docx title='Launch Brief' content='# Summary'"
examples:
  - "blue docx action=create path=reports/brief.docx title='Launch Brief' content='# Summary'"
  - "blue docx action=edit path=reports/brief.docx replacements='{\"{{status}}\":\"Ready\"}'"
  - "blue docx action=apply_template path=reports/brief_filled.docx template_path=templates/brief.docx variables='{\"{{owner}}\":\"Platform Team\"}'"
  - "blue docx action=validate path=reports/brief.docx"
capability_tags:
  - documents
  - office
  - docx
  - templating
interaction_mode: stateless
card_support: none
---

# DOCX

Use this skill when the `.docx` file is the source of truth and the task needs native Word-style structure, templating, or validation.

## Common Actions

```bash
blue docx action=create path=reports/brief.docx title="Launch Brief" content="# Summary"
blue docx action=edit path=reports/brief.docx replacements='{"{{status}}":"Ready"}'
blue docx action=apply_template path=reports/brief_filled.docx template_path=templates/brief.docx variables='{"{{owner}}":"Platform Team"}'
blue docx action=validate path=reports/brief.docx
```

## Markdown Input

- `action=create` accepts direct Markdown through `content`, `markdown`, `body`, or `text`.
- Headings, lists, quotes, code fences, and pipe tables are converted into native `.docx` structure.

## When To Use

- The final artifact must be `.docx`.
- The task needs placeholder replacement or template filling.
- The task needs a structured native report instead of a plain text file.

## Boundaries

- Use `pdf` when the final deliverable must be `.pdf`.
- Use `xlsx` for workbook or table-first tasks.
- Use `pptx` for slide decks.
- Use `a11y` for host-window menus, dialogs, clicks, scrolling, typing, or screenshots.
