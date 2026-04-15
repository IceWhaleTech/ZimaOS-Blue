---
name: summarize
version: "1.0.0"
description: "Run the external summarize.sh CLI through exec to summarize URLs, local files, and media transcripts. Use when the user explicitly wants summarize.sh or when this installed CLI is the best fit."
invocation: "blue exec command='summarize https://example.com --model google/gemini-3-flash-preview'"
examples:
  - "blue exec command='summarize https://example.com --model google/gemini-3-flash-preview'"
  - "blue exec command='summarize https://youtu.be/dQw4w9WgXcQ --youtube auto --extract-only'"
capability_tags:
  - summarize
  - external-cli
  - transcription
interaction_mode: stateless
card_support: none
category: external_cli
environment:
  - summarize
homepage: https://summarize.sh
metadata:
  zimaos-blue:
    emoji: "🧾"
    requires:
      bins:
        - summarize
    install:
      - id: brew
        kind: brew
        formula: steipete/tap/summarize
        bins:
          - summarize
        label: Install summarize (brew)
---

# Summarize

Run the external `summarize` binary via `blue exec`.

## Setup

Verify the CLI exists before relying on it:

```bash
blue exec command="summarize --help"
```

If needed, install it with Homebrew:

```bash
blue exec command="brew install steipete/tap/summarize"
```

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| User explicitly asks for summarize.sh | `blue exec command='summarize ...'` |
| Need best-effort transcript extraction for a YouTube URL | `blue exec command='summarize ... --youtube auto --extract-only'` |
| Need a plain webpage/article summary and summarize.sh is not required | Prefer built-in web tools plus a normal answer |
| Need JS/login/interactive browsing first | Use the built-in `browser` tool / unified web tools first, then summarize the retrieved content |

---

## When to use

Prefer this skill when the user asks any of:

- “use summarize.sh”
- “what’s this link/video about?”
- “summarize this URL/article”
- “transcribe this YouTube/video” (best-effort transcript extraction; no `yt-dlp` needed)

---

## Command Usage

Quick summary:

```bash
blue exec command='summarize https://example.com --model google/gemini-3-flash-preview'
blue exec command='summarize /path/to/file.pdf --model google/gemini-3-flash-preview'
blue exec command='summarize https://youtu.be/dQw4w9WgXcQ --youtube auto'
```

## YouTube: summary vs transcript

Best-effort transcript (URLs only):

```bash
blue exec command='summarize https://youtu.be/dQw4w9WgXcQ --youtube auto --extract-only'
```

If the user asked for a transcript but it’s huge, return a tight summary first, then ask which section/time range to expand.

## Model + keys

Set the API key for your chosen provider:

- OpenAI: `OPENAI_API_KEY`
- Anthropic: `ANTHROPIC_API_KEY`
- xAI: `XAI_API_KEY`
- Google: `GEMINI_API_KEY` (aliases: `GOOGLE_GENERATIVE_AI_API_KEY`, `GOOGLE_API_KEY`)

Default model is `google/gemini-3-flash-preview` if none is set.

## Useful flags

- `--length short|medium|long|xl|xxl|<chars>`
- `--max-output-tokens <count>`
- `--extract-only` (URLs only)
- `--json` (machine readable)
- `--firecrawl auto|off|always` (fallback extraction)
- `--youtube auto` (Apify fallback if `APIFY_API_TOKEN` set)

## Config

Optional config file: `~/.summarize/config.json`

```json
{ "model": "openai/gpt-5.2" }
```

Optional services:

- `FIRECRAWL_API_KEY` for blocked sites
- `APIFY_API_TOKEN` for YouTube fallback

## Notes

- This is an external CLI guide, not a native `blue summarize` subcommand.
- Prefer `blue exec command='summarize ...'` so invocation style stays consistent with the rest of the skill system.
- If the user only needs a normal summary and does not care about summarize.sh specifically, the built-in web/document tools are usually a better first choice.
