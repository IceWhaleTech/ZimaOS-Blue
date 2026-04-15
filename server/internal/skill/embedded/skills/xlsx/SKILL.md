---
name: xlsx
version: "1.0.0"
description: "Use when the task centers on a workspace .xlsx file and needs native spreadsheet creation, mutation, analysis, or validation."
invocation: "blue xlsx action=create path=reports/scorecard.xlsx sheets='[{\"name\":\"Scorecard\",\"rows\":[{\"Metric\":\"Launch\",\"Value\":\"Ready\"}]}]'"
examples:
  - "blue xlsx action=create path=reports/scorecard.xlsx sheets='[{\"name\":\"Scorecard\",\"rows\":[{\"Metric\":\"Launch\",\"Value\":\"Ready\"}]}]'"
  - "blue xlsx action=append_rows path=reports/scorecard.xlsx sheet='Scorecard' rows='[{\"Metric\":\"Risk\",\"Value\":\"Low\"}]'"
  - "blue xlsx action=update_cells path=reports/scorecard.xlsx sheet='Scorecard' cells='{\"B2\":\"Green\"}'"
  - "blue xlsx action=sheet_compare path=reports/current.xlsx compare_path=reports/baseline.xlsx"
capability_tags:
  - documents
  - office
  - xlsx
  - spreadsheet
interaction_mode: stateless
card_support: none
---

# XLSX

Use this skill when the `.xlsx` workbook is the source of truth and the task needs native spreadsheet edits, structural mutations, or workbook analysis.

## Common Actions

```bash
blue xlsx action=create path=reports/scorecard.xlsx markdown=$'# Scorecard\n\n## Metrics\n| Metric | Value |\n| --- | --- |\n| Launch | Ready |'
blue xlsx action=create path=reports/scorecard.xlsx sheets='[{"name":"Scorecard","rows":[{"Metric":"Launch","Value":"Ready"}]}]'
blue xlsx action=append_rows path=reports/scorecard.xlsx sheet="Scorecard" rows='[{"Metric":"Risk","Value":"Low"}]'
blue xlsx action=update_cells path=reports/scorecard.xlsx sheet="Scorecard" cells='{"B2":"Green"}'
blue xlsx action=sheet_compare path=reports/current.xlsx compare_path=reports/baseline.xlsx
```

## Markdown Input

- `action=create` can take direct Markdown through `content`, `markdown`, `body`, or `text`.
- Markdown pipe tables become worksheet tabs.
- Markdown lists or paragraphs become single-column content sheets or Overview notes when no table is present.

## When To Use

- The final artifact must be `.xlsx`.
- The task needs sheet-level mutation such as row inserts, cell updates, filters, or freeze panes.
- The task needs workbook-native analysis such as `group_by`, `top_n`, `profile_sheet`, or `sheet_compare`.

## Boundaries

- Use `docx` for narrative reports.
- Use `pptx` for slide decks.
- Use `pdf` for final printable documents.
- Use `a11y` for host-window interaction instead of spreadsheet file edits.
