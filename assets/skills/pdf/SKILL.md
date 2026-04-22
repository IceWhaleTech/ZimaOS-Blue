---
name: pdf
version: "1.1.0"
description: "Use when the task centers on a workspace .pdf file and needs native PDF reading, creation, filling, or reformatting. Supports read_mode presets for speed vs coverage."
invocation: "blue pdf action=read path=reports/brief.pdf"
examples:
  - "blue pdf action=read path=reports/brief.pdf"
  - "blue pdf action=read path=reports/brief.pdf read_mode=fast"
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
blue pdf action=read path=reports/brief.pdf read_mode=fast
blue pdf action=info path=reports/brief.pdf
blue pdf action=create path=reports/brief.pdf title="Launch Brief" content="# Summary"
blue pdf action=fill path=forms/intake.pdf output_path=forms/intake_filled.pdf fields='{"name":"Orca"}'
blue pdf action=reformat input_path=reports/source.pdf output_path=reports/source_clean.pdf
```

## Parameters (Practical Reference)

Common read parameters (action=`read`):

- `path` / `pdf`: Path or URL to a single PDF (workspace path preferred).
- `pdfs` / `paths`: Multiple PDFs (deduped, capped).
- `read_mode`: Preset for speed vs coverage (`full` / `auto` / `fast`).
- `purpose` (Default: `summarize`): High-level intent hint that selects sensible defaults when you did not explicitly set `read_mode` / `max_pages` / `max_chars` / include flags. Values: `summarize` / `facts` / `skim`.
- `pages`: Page selection like `"1,3-5"` or `[1,3,4,5]`. Use this when you know where the answer is.
- `max_pages`: Hard cap on number of pages returned.
- `max_chars`: Hard cap on characters returned.
- `include_markdown`: Include a layout-aware Markdown view when possible.
- `include_outline`: Include bookmarks/outline when available (useful for navigation).
- `include_pages`: Include per-page outputs (helps citations and page-specific extraction).
- `include_layout`: Include semantic blocks/tables when available (more expensive).
- `fallback_mode`: How to handle scanned pages:
  - `auto` (default): try text extraction; fall back to OCR/vision when needed
  - `text_only`: disable OCR + vision (fastest; fails on scans)
  - `ocr_only`: force OCR (better for scans; slower)
  - `vision_only`: force vision OCR (slowest; last resort)

Create/reformat/fill parameters:

- `action=create`: `path` (output), plus `title`/`subtitle`/`content` (Markdown-like).
- `action=reformat`: `input_path` (source PDF), `output_path` (destination PDF).
- `action=fill`: `path` (source PDF), `fields` (map), and `output_path` (destination PDF).

## Read Mode (Speed vs Coverage)

When reading long PDFs, there is a tradeoff between speed and coverage. The PDF tool exposes a `read_mode` parameter so the LLM can choose the right preset per task.

- `read_mode=full` (Default): Coverage-first. Reads more pages and returns more text. Best for long papers, reports, and anything where key sections may appear late (e.g., “Limitations”, “Safety”, appendices).
- `read_mode=fast`: Speed-first. Reads fewer pages and returns less text. Best when you only need a quick skim, a single fact, or a table/number likely near the start.
- `read_mode=auto`: Balanced preset between fast and full.

Notes:
- Preset defaults (when you don't set `max_pages` / `max_chars`):
  - `fast`: ~12 pages, ~60k chars
  - `auto`: ~20 pages, ~120k chars
  - `full`: uses the server defaults (currently coverage-first; typically up to ~35 pages / ~180k chars)
- `max_pages` / `max_chars` always take priority over `read_mode` (explicit limits win).
- If you need a specific section, prefer providing `pages` explicitly (e.g., `pages="40-60"`).

## Purpose (Intent Hint)

`purpose` is a higher-level, more semantic alternative to manually choosing `read_mode` and include flags. It only takes effect when you did not explicitly set those lower-level knobs.

- `purpose=summarize` (Default): Coverage-first; best for long reports/papers and end-to-end summaries.
- `purpose=facts`: Balanced; best for extracting specific answers (numbers, definitions, short Q&A).
- `purpose=skim`: Speed-first; best for “give me a quick idea what this is”.

Precedence rules:
- Explicit `read_mode` / `max_pages` / `max_chars` always win over `purpose`.
- Explicit `include_markdown` / `include_outline` / `include_layout` always win over `purpose`.

## Output (What To Look For)

`action=read` returns structured JSON. The most useful fields:

- `text`: normalized combined text (best for summarization / QA)
- `raw_text`: closer to the raw extraction with more layout hints
- `markdown`: layout-aware Markdown (when enabled)
- `outline`: headings/bookmarks with page numbers (when enabled)
- `pages`: per-page text/markdown (when `include_pages=true`)
- `selected_pages`: which pages were actually extracted
- `warnings`: extraction warnings (e.g., truncation, OCR fallback)
- `truncated`: whether output hit limits (`max_pages` / `max_chars`)
- `ocr_used` / `vision_used`: whether fallbacks were used

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
