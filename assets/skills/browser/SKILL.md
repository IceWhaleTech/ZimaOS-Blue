# Browser

Interact with a headless browser. Navigate to URLs, read page content via accessibility tree DSL, interact with elements using @ref references, take screenshots.

## How to Send

Direct call:

```bash
browser.navigate url=https://example.com
```

Add `--json` for JSON output.

## Commands

### browser.navigate

Open a URL and return page info.

**Required:** `url`

**Optional:**
- `target_id` — reuse an existing tab
- `lang` — output language (default: `en-US`)

```bash
browser.navigate url=https://example.com
```

### browser.snapshot

Get the full CDP accessibility tree for a tab. Token-efficient alternative to raw HTML.

**Optional:**
- `target_id` — tab to snapshot (defaults to active)
- `max_depth` — tree depth limit (default: 10)

```bash
browser.snapshot target_id=ABCDEF123456
```

### browser.snapshot_interactive

Get only interactive elements (buttons, links, inputs) via JS extraction. Faster and smaller than full a11y tree.

**Optional:** `target_id`

```bash
browser.snapshot_interactive
```

### browser.act

Interact with a page element by its @ref number from a previous snapshot.

**Required:** `ref`, `act_type` (`click`, `type`, `focus`, `hover`, `scroll`, `select`)

**Optional:**
- `target_id` — tab containing the element
- `value` — text for `type` or option for `select`

```bash
browser.act ref=5 act_type=click
```

### browser.screenshot

Capture a page as base64 PNG.

**One of:** `url` (navigate and screenshot) or `target_id` (screenshot existing tab)

```bash
browser.screenshot url=https://example.com
```

### browser.tabs

List all open browser tabs.

```bash
browser.tabs
```

### browser.close

Close a browser tab.

**Required:** `target_id`

```bash
browser.close target_id=ABCDEF123456
```

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `missing url` — no `url` key for navigate/screenshot
- `missing ref` — no `ref` key for act
- `missing act_type` — no `act_type` key for act
- `navigate failed: ...` — page load error
- `snapshot failed: ...` — accessibility tree extraction error
- `act failed: ...` — element interaction error
- `screenshot failed: ...` — capture error

## Workflow

1. `browser.navigate` to open a URL (returns `target_id`)
2. `browser.snapshot` or `browser.snapshot_interactive` to read page content
3. `browser.act` to interact with elements using @ref from the snapshot
4. Repeat 2-3 as needed
5. `browser.screenshot` to capture visual state
6. `browser.close` when done

## Example Triggers

- "Open https://example.com and read the page"
- "Click the login button"
- "Take a screenshot of the page"
- "打开这个网页看看内容"
