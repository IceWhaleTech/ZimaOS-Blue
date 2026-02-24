# UI Reviewer

Evaluate UI/UX quality of a website or screenshot. Captures screenshots, runs accessibility checks, and uses VLM (vision language model) for visual review. Returns structured scoring report.

## How to Send

Use the `blue` CLI (always use `--json` for structured output):

```bash
blue ui.review_url url=https://example.com --json
```

## Commands

### ui.review_url

Full UI review of a URL. Navigates to the page, captures multi-viewport screenshots, runs accessibility checks, and performs VLM visual analysis.

**Required:** `url`

**Optional:**
- `lang` — output language (`en-US`, `zh-CN`, default: `en-US`)
- `device` — viewport: `desktop` (1920x1080) or `mobile` (375x812)

```bash
blue ui.review_url url=https://example.com lang=zh-CN --json
```

### ui.review_image

VLM visual review of a base64-encoded screenshot. No browser needed.

**Required:** `image` (base64 PNG data)

**Optional:** `lang`

```bash
blue ui.review_image image=<base64_png_data>
```

### ui.check_accessibility

Accessibility check only (no VLM). Navigates to URL, builds accessibility tree, and checks for common a11y issues.

**Required:** `url`

**Optional:** `lang`

```bash
blue ui.check_accessibility url=https://example.com
```

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `missing url` — no `url` key for review_url/check_accessibility
- `missing image` — no `image` key for review_image
- `review failed: ...` — browser or VLM error during review
- `check failed: ...` — accessibility check error

## Scoring

The review returns scores (0-10) in three categories:

| Category | What it checks |
|----------|---------------|
| Visual | Layout, typography, color contrast, spacing, consistency |
| Functional | Links, forms, buttons, navigation, error states |
| Accessibility | ARIA labels, alt text, focus order, color contrast, heading hierarchy |

- `overall` = weighted average of all categories
- `pass` = `overall >= threshold` (default threshold: 6.0)

## Example Triggers

- "Review the UI of https://example.com"
- "Check accessibility of this page"
- "评审一下这个网站的UI质量"
- "帮我检查这个页面的无障碍性"
