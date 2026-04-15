---
name: pdf
version: "1.0.0"
description: "Use when the task centers on a workspace .pdf file and needs native PDF reading, creation, filling, or reformatting."
invocation: "blue pdf action=read path=reports/brief.pdf"
examples:
  - "blue pdf action=read path=reports/brief.pdf"
  - "blue pdf action=info path=reports/brief.pdf"
  - "blue pdf action=create path=reports/brief.pdf title='Launch Brief' content='# Summary'"
  - "blue pdf action=fill path=forms/intake.pdf output_path=forms/intake_filled.pdf fields='{\"name\":\"Orca\"}'"
  - "blue pdf action=reformat input_path=reports/source.pdf output_path=reports/source_clean.pdf"
capability_tags:
  - documents
  - office
  - pdf
  - forms
interaction_mode: stateless
card_support: none
---

# PDF

Use this skill when the `.pdf` file is the source of truth and the task needs PDF-native reading, form filling, or final-document output.

## Common Actions

```bash
blue pdf action=read path=reports/brief.pdf
blue pdf action=info path=reports/brief.pdf
blue pdf action=create path=reports/brief.pdf title="Launch Brief" content="# Summary"
blue pdf action=fill path=forms/intake.pdf output_path=forms/intake_filled.pdf fields='{"name":"Orca"}'
blue pdf action=reformat input_path=reports/source.pdf output_path=reports/source_clean.pdf
```

## Markdown Input

- `action=create` accepts direct Markdown through `content`, `markdown`, `body`, or `text`.
- Headings, lists, and tables are converted into the native PDF creation pipeline for printable output.

## When To Use

- The final artifact must be `.pdf`.
- The task needs metadata or text extraction from an existing PDF.
- The task needs native PDF form inspection/filling or layout-preserving reformatting.

## Boundaries

- Use `docx`, `xlsx`, or `pptx` when the editable source of truth is an OOXML workspace file.
- Use `computer_use` for live PDF viewer window interaction rather than file conversion or extraction.
