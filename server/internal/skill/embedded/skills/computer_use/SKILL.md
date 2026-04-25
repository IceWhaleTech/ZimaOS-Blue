---
name: computer_use
version: "1.0.0"
description: "Use when the task is to inspect or operate a live desktop application, native window, browser accessibility tree, menu, dialog, button, list, toggle, or text input through the built-in computer_use tool. Prefer this for host UI focus, snapshot, click, type, select, toggle, scroll, screenshot, or chat-style message flows. Prefer scenario actions `message|type|select|click|toggle` over generic `act` whenever the user goal is already clear."
invocation: "blue computer_use action=snapshot_interactive"
examples:
  - "blue computer_use action=snapshot_interactive"
  - "blue computer_use action=message app_name='Feishu,Lark' conversation='Orca' value='hello'"
capability_tags:
  - computer-use
  - desktop
  - ui
  - automation
interaction_mode: interactive
card_support: streaming
os:
  - darwin
  - windows
---

# Computer Use

Use the built-in `computer_use` tool for live UI work on desktop apps, native windows, and accessibility-backed browser flows.

## Setup

No external CLI is required.

For host-window actions:

- macOS: Accessibility permission may be required.
- Windows: UI Automation / WinEvent-backed inspection must be available.

If the user only wants to edit a workspace artifact such as `.docx`, `.xlsx`, `.pptx`, or `.pdf`, use the native document tools instead of driving the app UI.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Focus the right app or window | `blue computer_use action=focus ...` |
| Inspect interactive host UI before acting | `blue computer_use action=snapshot_interactive ...` |
| Read the full accessibility tree for debugging | `blue computer_use action=snapshot ...` |
| Send a chat message in a known conversation | `blue computer_use action=message ...` |
| Switch to a conversation/list row/item without typing | `blue computer_use action=select ...` |
| Type into the likely composer/input without exposing refs | `blue computer_use action=type ...` |
| Click a named control or toggle a named setting | `blue computer_use action=click ...` / `toggle ...` |
| Scroll, press keys, or capture evidence | `blue computer_use action=scroll ...`, `key ...`, `screenshot ...` |
| Run a multi-step CUA desktop task | `blue computer_use action=task goal="..."` |
| Use CUA action aliases | `open_app`, `input_text`, `Click`, `RightSingle`, `move_mouse`, `scroll_up/down`, `Hotkey`, `multi_Hotkey`, `record_info`, `done` |
| User goal is still ambiguous and you truly need low-level control | `blue computer_use action=act params...` |

---

## Command Usage

### Fast Path

```bash
blue computer_use action=message app_name="Feishu,Lark" conversation="Orca" value="hello"
blue computer_use action=type app_name="Feishu,Lark" conversation="Orca" value="draft only"
blue computer_use action=select app_name="Feishu,Lark" conversation="Orca"
blue computer_use action=click window_title="Settings" control="Open Network"
blue computer_use action=toggle window_title="Settings" setting="Enable notifications"
```

### CUA Task And Actions

```bash
blue computer_use action=task goal="Find the latest note in Feishu and summarize it"
blue computer_use action=record_info file_name="note.txt" text="Important info to reuse later"
blue computer_use action=Click window_id="win-1" position='[0.5,0.5]'
blue computer_use action=Click window_id="win-1" position='[500,500]'
blue computer_use action=multi_Hotkey window_id="win-1" key1="command" key2="shift" key3="2"
```

`task` uses the Go-native CUA planner/brain/actor/memory loop by default. It does not shell out to a Python runner. Coordinates are accepted as normalized `0-1000`; existing `0-1` normalized coordinates still work. Common CUA/LLM-fuzzy parameter aliases are accepted, including `name`/`type` for action, `coordinate`/`coords`/`point` for `position`, `body`/`message`/`content` for text, `shortcut`/`hotkey`/`key`/JSON-array strings for `keys`, `send_keys`/`send_key` for `submit_keys`, and `amount`/`scroll_direction` for scrolling.

### Window And Snapshot

```bash
blue computer_use action=focus app_name="Feishu,Lark"
blue computer_use action=snapshot_interactive window_title="Settings"
blue computer_use action=snapshot window_title="Settings"
blue computer_use action=screenshot
```

### Low-Level Act

```bash
blue computer_use action=act params.ref=@5 params.act_type=click
blue computer_use action=act params.ref=@8 params.act_type=type params.value="hello"
blue computer_use action=act app_name="Feishu,Lark" params.intent=message params.conversation="Orca" params.value="hello" params.submit=true
```

### Scroll And Key

```bash
blue computer_use action=scroll params.direction=down params.lines=6
blue computer_use action=key params.keys='["cmd","s"]'
```

---

## Fast Path Rules

- Prefer `message|type|select|click|toggle` when the user intent is already clear.
- Prefer `app_name` or `window_title` over listing every window first when you already know the target app.
- Prefer `snapshot_interactive` over full `snapshot` when you only need actionable controls.
- Prefer ref-less scenario actions first. Drop to `act + ref` only when you truly need exact low-level control.

## Completion Path Rules

The goal is not to hit any matching node. The goal is to finish the user-visible path.

- For chat/message flows, prefer the active composer over search boxes.
- For toggles/settings, prefer the control nearest the named label or anchor.
- For selection flows, prefer list rows/items over generic buttons with the same text.
- Treat ambiguity as a stop signal unless ranking yields one clear best candidate.

Think in this order:

`intent -> constrained query -> top-k candidates -> completion-aware ranking -> execute -> verify`

## Fallback Discipline

Use the cheapest safe path first:

1. Scenario action with app/window hint
2. Cached structured query against the current snapshot
3. `snapshot_interactive` refresh
4. Full `snapshot` only for debugging or last-resort label/proximity reasoning
5. OCR / visual fallback only when the structured path cannot safely finish

Do not ask for a full tree by default if the user already gave enough intent to run a scenario action.

## Verification Discipline

- For `message` and `type`, verify the text landed in the intended composer/input.
- For submit flows, verify the pending text cleared or the composer state changed as expected.
- For `toggle`, prefer state change confirmation over blind success.
- For `click` and `select`, prefer focused/selected state change when available.

## Timeout Discipline

Completion-path confirmation should not fail fast.

- Use long timeout windows in the `30-60s` range for conversation switching, submit confirmation, and cross-app activation.
- Use bounded second-level polling around `1s`, not millisecond-level retry loops.
- Exit immediately once the expected state is observed.

## When Not To Use

- Do not use `computer_use` to generate or edit structured workspace documents when `docx`, `xlsx`, `pptx`, or `pdf` can express the task directly.
- Do not use full snapshots as the default control path for simple click/type/toggle requests.
- Do not use `exec` or ad hoc scripts for native UI actions that `computer_use` can inspect and execute safely.
