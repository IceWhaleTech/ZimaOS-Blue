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
| Focus a desktop app window, inspect host UI, click menus/buttons, scroll, type, or capture a host-window screenshot | `blue a11y action=focus|snapshot_interactive|act|scroll|key|screenshot ...` |
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
blue a11y action=message app_name="Feishu、飞书、Lark" conversation="Orca" value="你好，Orca"
blue a11y action=type app_name="Feishu、飞书、Lark" conversation="Orca" value="你好，Orca"
blue a11y action=select app_name="Feishu、飞书、Lark" conversation="Orca"
blue a11y action=click window_title="Settings" control="Open Network"
blue a11y action=toggle window_title="Settings" setting="Enable notifications"
blue a11y action=act params.value="你好，Orca" params.submit=true
blue a11y action=act app_name="Feishu、飞书、Lark" params.value="你好，Orca" params.submit=true
blue a11y action=act app_name="Feishu、飞书、Lark" params.conversation="Orca" params.value="你好，Orca"
blue a11y action=act app_name="Feishu、飞书、Lark" params.intent=message params.conversation="Orca" params.value="你好，Orca" params.submit=true
blue a11y action=act window_title="Settings" params.control="Open Network"
blue a11y action=act window_title="Settings" params.setting="Enable notifications"
blue a11y action=act window_title="Settings" params.intent=click params.control="Open Network"
blue a11y action=act window_title="Settings" params.intent=toggle params.setting="Enable notifications"
blue a11y action=focus window_title="Feishu、飞书、Lark"
blue a11y action=snapshot_interactive
blue a11y action=act params.ref=@5 params.act_type=click
blue a11y action=act window_title="Feishu、飞书、Lark" params.value="你好，Orca"
blue a11y action=act window_title="Feishu、飞书、Lark" params.value="你好，Orca" params.submit=true
blue a11y action=act params.ref=@8 params.act_type=type params.value="你好，Orca"
blue a11y action=key params.keys='["cmd","s"]'
blue a11y action=screenshot
```

---

## Notes

- Prefer the native document tools when the source of truth is the workspace file itself.
- Prefer `a11y` when the user explicitly wants to operate a desktop application, native window, menu, button, dialog, or scrollable host UI.
- If the target app is already frontmost, you can omit `window_title`, `app_name`, and `window_id` entirely; host actions will use the current focused window.
- If you already know the target host app, prefer `window_title` or `app_name` instead of listing windows first; the tool will try unique exact matching first and then unique fuzzy matching, and one string can include aliases like `Feishu、飞书、Lark`. If multiple candidates remain, a unique focused window is selected automatically.
- Prefer `app_name` when you are matching app-brand aliases such as `Feishu / 飞书 / Lark`; reserve `window_title` for cases where you really know the visible window or chat title.
- For chat-style `act` typing without `ref`, the tool will first prefer the most likely composer/editor input instead of generic search bars, and only fail when it still cannot pick a single safe target.
- For fast chat-style entry, prefer providing just `value`; when `ref` is omitted the tool defaults to text input and tries to resolve the unique visible input/editor automatically.
- When the user goal is already clear, prefer the scenario-style shortcuts instead of exposing more knobs:
  - `action=message app_name=... conversation=... value=...` is now the shortest "find chat, type, and ensure send" path
  - `action=type app_name=... conversation=... value=...` is now the shortest "find chat, switch to its input, and only type" path
  - `action=select app_name=... conversation=...` is now the shortest "find a chat/list item/config item and switch/select it" path
  - `action=click window_title=... control=...` is now the shortest "find a control/button and click it" path
  - `action=toggle window_title=... setting=...` is now the shortest "find a setting and switch it" path
  - `conversation=...` plus `value=...` for "find a chat, type, and send"; this now implies message flow and sends by default unless `submit=false`
  - `control=...` for "find a control/button and click it"
  - `setting=...` for "find a setting and switch it"
- `action=message|click|toggle` is the preferred explicit façade now; `params.intent=message|click|toggle` is still supported for compatibility but is no longer required for the common cases above.
- Those scenario target names are no longer exact-only:
  - exact match wins first
  - otherwise a unique fuzzy match on the same role is accepted
  - if same-name matches still remain inside that role family, the tool prefers the most likely role for the scenario such as conversation row/list item over a generic button, or switch over a menu item
  - if fuzzy matching still ties, the tool fails closed with ambiguity instead of guessing
- If a compact interactive snapshot still cannot see the visible label because the real control is an unlabeled nearby switch/button, the tool can do one internal full-snapshot proximity pass for common `setting` / `control` / `select` scenarios before giving up.
- Only reach for `params.target_name=...` or `params.target_role=...` when the default input resolution is ambiguous and you need to pin a specific control.
- If the flow is "type then send", prefer a single `act` call with `params.submit=true`; the tool will internally try the most likely send button first, prefer the one nearest the chosen input when multiple send-like controls exist, then downgrade to platform submit keys if needed, and keep retrying only when the post-submit UI still clearly shows the typed text sitting in the composer.
- Prefer `snapshot_interactive` over the full snapshot when the goal is to act quickly on visible controls.
- Prefer `act_type=type` for text entry because the runtime can use native set-value or clipboard-backed paste; reserve `key` for shortcuts such as save, submit, or navigation.
- Do not try to use `docx`, `xlsx`, `pptx`, or `pdf` to click through native application chrome; that is `a11y` work.
- Do not use `a11y` as a substitute for structured file edits when the artifact can be produced directly with `docx`, `xlsx`, `pptx`, or `pdf`.

---

## Quick Paths (High Success)

### Feishu Send Message (Deterministic Path)

Goal: open or focus Feishu, switch to a conversation, type, and send once.

Conversation search rule for Feishu / Lark:

- Prefer the app's search shortcut before exploratory snapshots.
- On macOS, search the target conversation with `Cmd+K` first; if that search surface does not appear, retry with `Cmd+F` twice.
- On Windows, use the same rule with `Ctrl+K` first and `Ctrl+F` twice as fallback.

Preferred one-shot:

```bash
blue a11y action=message app_name="Feishu、飞书、Lark" conversation="Orca" value="你好，Orca"
```

If you want the steps spelled out (same logic, more explicit):

1. Focus or match the app
```
blue a11y action=focus app_name="Feishu、飞书、Lark"
```
1. Switch to the conversation
```
blue a11y action=select app_name="Feishu、飞书、Lark" conversation="Orca"
```
1. Type and send
```
blue a11y action=message app_name="Feishu、飞书、Lark" conversation="Orca" value="你好，Orca"
```

### Feishu Draft Only (Type Without Send)

```bash
blue a11y action=type app_name="Feishu、飞书、Lark" conversation="Orca" value="你好，Orca"
```

### Settings Toggle (Label + Switch)

```bash
blue a11y action=toggle window_title="Settings" setting="Enable notifications"
```

---

## Common App Quick Paths (Mac/Windows)

Use these when the user goal is singular and obvious. Prefer one decisive path over exploratory snapshots.

### IM / Chat Apps

Targets: Feishu / Lark, WeCom / Enterprise WeChat, DingTalk, Slack, Teams.

- Deterministic path: app -> conversation -> composer -> send
- For Feishu / Lark specifically, searching the conversation should prefer shortcut search over scanning the sidebar:
  - macOS: `Cmd+K` first, then `Cmd+F` twice if needed
  - Windows: `Ctrl+K` first, then `Ctrl+F` twice if needed
- Prefer one shot when the user clearly wants a message delivered:

```bash
blue a11y action=message app_name="Feishu,飞书,Lark" conversation="Orca" value="你好，Orca"
blue a11y action=message app_name="Slack" conversation="Orca" value="hello"
blue a11y action=message app_name="Microsoft Teams,Teams" conversation="Orca" value="hello"
```

- Prefer draft-only when the user does not want to send yet:

```bash
blue a11y action=type app_name="Feishu,飞书,Lark" conversation="Orca" value="你好，Orca"
```

- Do not list windows first unless the first one-shot attempt is ambiguous.
- Let `app_name` carry aliases such as `Feishu,飞书,Lark`; fuzzy matching is the default.

### System Settings

Targets: macOS Settings / System Settings, Windows Settings / Control Panel surfaces.

- Deterministic path: settings window -> labeled control/setting -> click or toggle

```bash
blue a11y action=toggle window_title="Settings,System Settings,设置" setting="Enable notifications"
blue a11y action=click window_title="Settings,System Settings,设置" control="Open Network"
```

- Prefer `setting=...` for labeled switches.
- Prefer `control=...` for buttons, rows, or navigation entries.

### Browser

Targets: Chrome, Edge, Safari, Firefox.

- Deterministic path: browser app/window -> visible control -> click or select

```bash
blue a11y action=click app_name="Google Chrome,Chrome,Microsoft Edge,Edge,Safari" control="Address Bar"
blue a11y action=select app_name="Google Chrome,Chrome,Microsoft Edge,Edge,Safari" item="Downloads"
```

- Use browser-surface tools only when the task is clearly about web DOM automation; stay on host `a11y` for native browser chrome.

### File Manager

Targets: Finder / Files / Explorer.

- Deterministic path: app -> sidebar/list item -> select or click

```bash
blue a11y action=select app_name="Finder,访达,Explorer,文件资源管理器" item="Downloads"
blue a11y action=click app_name="Finder,访达,Explorer,文件资源管理器" control="Desktop"
```

- Prefer `select` for folders, rows, and sidebar entries.
- Prefer `click` when the user explicitly wants to press a named button or toolbar control.

### Mail

Targets: Apple Mail, Outlook, Mail app.

- Deterministic path: app -> target conversation or compose surface -> type or message

```bash
blue a11y action=type app_name="Mail,Apple Mail,Outlook" value="你好，Orca"
blue a11y action=click app_name="Mail,Apple Mail,Outlook" control="New Message"
```

- Prefer `type` when the compose input is already frontmost.
- Prefer `click` on `New Message` or a clearly labeled thread first when compose is not visible.

---

## 10-Second Feishu Flow

Prompt: `帮我在飞书上和 Orca 打一个招呼`

Preferred path:

```bash
blue a11y action=message app_name="Feishu,飞书,Lark" conversation="Orca" value="你好，Orca"
```

Why this is the default path:

- It skips window listing and goes straight to fuzzy app resolution.
- If the app match is unique and exact enough, the tool activates it immediately.
- For Feishu / Lark, the conversation lookup should first try the app's search shortcut surface:
  - macOS: `Cmd+K` first, then `Cmd+F` twice
  - Windows: `Ctrl+K` first, then `Ctrl+F` twice
- After entering the search surface, the conversation lookup remains scenario-aware and prefers chat rows over unrelated controls.
- Text entry prefers `set_value`, then verifies via AX/OCR, then degrades to clipboard paste, then unicode typing only if needed.
- The send step is bundled into the message flow, so the model does not need to decide whether to click Send or press Enter.

If the one-shot path cannot complete directly, use this fallback sequence and stop as soon as one step succeeds:

1. `blue a11y action=focus app_name="Feishu,飞书,Lark"`
2. Open conversation search first:
   - macOS: `blue a11y action=key app_name="Feishu,飞书,Lark" keys='["cmd","k"]'`
   - fallback: `blue a11y action=key app_name="Feishu,飞书,Lark" keys='["cmd","f"]'` twice
3. `blue a11y action=select app_name="Feishu,飞书,Lark" conversation="Orca"`
4. `blue a11y action=message app_name="Feishu,飞书,Lark" conversation="Orca" value="你好，Orca"`

Expected success signal in eval mode:

- Final output should contain raw a11y JSON with `intent=message`, `target_hit=true`, and `verification_passed=true`.
