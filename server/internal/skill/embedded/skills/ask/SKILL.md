# Ask

Ask the user one or more multiple-choice questions and wait for their response.

## How to Send

Use this skill when you need explicit user input before continuing.

Preferred single-question format:

```bash
ask question="Choose a deploy strategy" a='["Canary","Blue-Green"]'
```

Alternative single-question format:

```bash
ask q="Choose a deploy strategy" a='["Canary","Blue-Green"]'
```

Multi-question format:

```bash
ask questions='[{"question":"Choose a deploy strategy","options":["Canary","Blue-Green"]}]'
```

## Input Format

- `question` + `a` (recommended for single question):
- `question`: prompt text
- `a`: JSON string array with at least 2 options
- `q`/`mq` + `a`:
- `q`: single-select question (default)
- `mq`: multi-select question
- `a`: JSON string array with at least 2 options
- `questions`:
- JSON array of question objects; each object can use `question/options` (preferred) or `q/a` aliases

Example:

```json
[
  {
    "question": "Which environment should we use?",
    "options": ["Staging", "Production"]
  }
]
```

Compatibility note:
- `option` and `options` are accepted aliases for `a`.
- Repeating `option=...` is supported, but array-style (`a='[...]'`) is the canonical format.

## Behavior

- Sends questions to frontend and blocks until user answers or timeout.
- In non-interactive/silent mode, it auto-selects the first option.
- Returns structured JSON output with selected answers.

## When To Use

- Missing critical requirement details.
- User decision gates the next technical step.
- Multiple valid approaches need explicit user preference.
