---
name: office_docs
version: "1.0.0"
description: "Use when the task involves creating, reading, editing, validating, or reformatting .docx, .xlsx, .pptx, or .pdf workspace artifacts, especially when the user also needs host application or window interaction that should go through a11y."
invocation: "blue docx action=create path=reports/brief.docx title='Launch Brief' content='# Summary'"
examples:
  - "blue docx action=create path=reports/brief.docx title='Launch Brief' content='# Summary'"
  - "blue xlsx action=create path=reports/scorecard.xlsx sheets='[{\"name\":\"Scorecard\",\"rows\":[{\"Metric\":\"Launch\",\"Value\":\"Ready\"}]}]'"
  - "blue a11y action=snapshot"
capability_tags:
  - documents
  - office
  - docx
  - xlsx
  - pptx
  - pdf
  - a11y
interaction_mode: interactive
card_support: streaming
---

# Office Docs

Route native workspace document work through the dedicated document tools, and switch to `a11y` when the task crosses from file manipulation into live host-window interaction.

## Setup

No external CLI is required. Use the built-in `docx`, `xlsx`, `pptx`, `pdf`, and `a11y` surfaces.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Create or rewrite a Word-style report | `blue docx action=create ...` or `blue docx action=edit ...` |
| Apply placeholders to an existing template | `blue docx action=apply_template ...` |
| Create or update a spreadsheet | `blue xlsx action=create ...`, `edit`, `append_rows`, or `update_cells` |
| Create or update a slide deck | `blue pptx action=create ...`, `edit`, `replace_text`, or slide mutation actions |
| Read, inspect, create, fill, or reformat PDFs | `blue pdf action=read|info|create|fill|reformat ...` |
| Focus a desktop app window, inspect host UI, click menus/buttons, scroll, type, or capture a host-window screenshot | `blue a11y action=windows|focus|snapshot|snapshot_interactive|act|scroll|key|screenshot ...` |
| Work on both the file artifact and the live desktop app | Use `docx` / `xlsx` / `pptx` / `pdf` for the file, and use `a11y` for the host window steps |

---

## Command Usage

### DOCX

```bash
blue docx action=create path=reports/brief.docx title="Launch Brief" content="# Summary"
blue docx action=edit path=reports/brief.docx replacements='{"{{status}}":"Ready"}'
blue docx action=validate path=reports/brief.docx
```

### XLSX

```bash
blue xlsx action=create path=reports/scorecard.xlsx sheets='[{"name":"Scorecard","rows":[{"Metric":"Launch","Value":"Ready"}]}]'
blue xlsx action=append_rows path=reports/scorecard.xlsx sheet="Scorecard" rows='[{"Metric":"Risk","Value":"Low"}]'
blue xlsx action=update_cells path=reports/scorecard.xlsx sheet="Scorecard" cells='{"B2":"Green"}'
```

### PPTX

```bash
blue pptx action=create path=decks/launch_plan.pptx title="Launch Plan" sections='[{"heading":"Overview","bullets":["Status","Risks","Timeline"]}]'
blue pptx action=edit path=decks/launch_plan.pptx replacements='{"{{owner}}":"Platform Team"}'
blue pptx action=validate path=decks/launch_plan.pptx
```

### PDF

```bash
blue pdf action=read path=reports/brief.pdf
blue pdf action=create path=reports/brief.pdf title="Launch Brief" content="# Summary"
blue pdf action=reformat input_path=reports/source.pdf output_path=reports/source_clean.pdf
```

### A11y For Host Apps And Windows

```bash
blue a11y action=windows
blue a11y action=focus window_id=12345
blue a11y action=snapshot
blue a11y action=act params.ref=@5 params.act_type=click
blue a11y action=key params.keys='["cmd","s"]'
blue a11y action=screenshot
```

---

## Notes

- Prefer the native document tools when the source of truth is the workspace file itself.
- Prefer `a11y` when the user explicitly wants to operate a desktop application, native window, menu, button, dialog, or scrollable host UI.
- Do not try to use `docx`, `xlsx`, `pptx`, or `pdf` to click through native application chrome; that is `a11y` work.
- Do not use `a11y` as a substitute for structured file edits when the artifact can be produced directly with `docx`, `xlsx`, `pptx`, or `pdf`.
