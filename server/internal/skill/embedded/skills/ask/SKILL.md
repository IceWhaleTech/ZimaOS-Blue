# Ask

Ask the user one or more multiple-choice questions and wait for their response.

## How to Send

Use this skill when you need explicit user input before continuing.

Preferred single-question format:

```bash
ask q="Choose a deploy strategy" a='["Canary","Blue-Green"]'
```

Alternative single-question format (legacy):

```bash
ask question="Choose a deploy strategy" options='["Canary","Blue-Green"]'
```

Multi-question format (recommended):

```bash
ask questions='[
  {"question":"How should I address you?","type":"radio","options":["Alex","A."]},
  {"question":"Preferred language?","type":"radio","options":["zh","en"]},
  {"question":"Timezone?","type":"radio","options":["Asia/Shanghai","UTC-8"]},
  {"question":"Any other preferences?","type":"checkbox","options":[
    {"label":"Free-form answer (Recommended)","description":"Reply directly in plain text","value":"free"},
    {"label":"Template answer","value":"template"}
  ]}
]'
```

## Input Format

- Preferred: `questions`
- Type: array
- Item shape: `{ question, type, options }`
- `type`: `radio` or `checkbox`
- `options`: array of string or `{label,description?,value?}`
- Shorthand for one question: `q` or `mq` + `a`

Example:

```json
[
  {
    "question": "Which environment should we use?",
    "type": "radio",
    "options": ["Staging", "Production"]
  },
  {
    "question": "How do you want to respond?",
    "type": "checkbox",
    "options": [
      {"label": "Free-form answer (Recommended)", "description": "Directly answer in plain text", "value": "free"},
      {"label": "Template answer", "value": "template"}
    ]
  }
]
```

Compatibility note:
- Legacy aliases (`q/a` inside `questions`, `option`) may still exist in old prompts, but should not be used.

## Behavior

- Sends questions to frontend and blocks until user answers or timeout.
- In non-interactive/silent mode, it auto-selects the first option.
- Returns structured JSON output with selected answers.

## When To Use

- Missing critical requirement details.
- User decision gates the next technical step.
- Multiple valid approaches need explicit user preference.
