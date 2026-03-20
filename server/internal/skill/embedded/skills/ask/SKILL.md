---
name: ask
description: "Ask the user one or more questions and wait for their response. Supports both multiple-choice and free-text prompts. Use when the user explicitly asks to be questioned (e.g. 'ask me question(s)'), when intent is ambiguous and clarification is required, or as a confirmation gate in MCP internal Plan mode."
---

# Ask Skill

## Setup

No external dependencies required. This skill uses the built-in `ask` capability.

---

## Task Routing

Based on user intent, choose the corresponding `ask` action:

| User Intent | Action |
|-------------|--------|
| User explicitly asks for question flow (e.g. "ask me questions", "用 ask 技能问我问题") | Run one `blue ask` call immediately (prefer `questions='[...]'` for multi-question flows) |
| Requirement is ambiguous and you must clarify before choosing a strategy | Run `blue ask` to collect missing constraints/preferences |
| MCP internal Plan mode requires a confirmation/clarification gate | Run `blue ask` before continuing plan execution |
| Multi-step questionnaire/interview (Q1/Q2/Q3) | Use a single `blue ask questions='[...]'` call whenever possible |
| Multiple valid implementation approaches | Run `blue ask` to get explicit user choice before execution |

---

## Command Usage

### Preferred Single-Question Format

```bash
blue ask q="Choose a deploy strategy" a='["Canary","Blue-Green"]'
```

### Multi-Question Format (Recommended)

```bash
blue ask questions='[
  {"question":"How should I address you?","type":"radio","options":["Alex","A."]},
  {"question":"Preferred language?","type":"radio","options":["zh","en"]},
  {"question":"请假时长？","type":"text"},
  {"question":"邮件收件人是？","type":"radio","options":["直属上级","HR","两者都抄送"]}
]'
```

Do not continue staged questionnaires via plain assistant text (e.g. "continnue second question...").
Each follow-up must be another `blue ask` call.

---

## Input Format

| Field | Requirement | Notes |
|-------|-------------|-------|
| `questions` | Preferred | Array of question objects |
| `type` | Required per item | `radio`, `checkbox`, or `text` |
| `options` | Conditional | Required for `radio`/`checkbox`; omit for `text` |
| `q` + `a` | Shorthand | Single-question shorthand |
| `mq` + `a` | Shorthand | Multi-choice shorthand |

Example payload:

```json
[
  {
    "question": "Which environment should we use?",
    "type": "radio",
    "options": ["Staging", "Production"]
  },
  {
    "question": "How long is the leave?",
    "type": "text"
  }
]
```

Compatibility note:
- Legacy aliases (e.g. `option`, legacy `q/a` inside `questions`) may appear in old prompts, but should not be used in new calls.

---

## Behavior

- Sends question payload to frontend and blocks until user answers or timeout.
- Returns structured JSON output containing selected answers.
- In non-interactive/silent mode, it auto-selects the first option when options exist.
- For interviews/surveys, keep interaction in tool calls (`ask`) instead of plain assistant text questions.

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing options for a `radio`/`checkbox` question | Add valid `options` and retry |
| Invalid `type` value | Use only `radio`, `checkbox`, or `text` |
| Empty/invalid `questions` payload | Provide a non-empty array of valid question objects |
| User does not answer (timeout/dismiss) | Surface status to user and ask whether to retry or continue with defaults |

---

## Notes

- Prefer a single `blue ask questions='[...]'` call over fragmented multi-round questioning.
- Ask only what is necessary to unblock execution and keep options mutually exclusive where possible.
- In MCP internal Plan mode, use `ask` as a hard gate when user confirmation is required.
