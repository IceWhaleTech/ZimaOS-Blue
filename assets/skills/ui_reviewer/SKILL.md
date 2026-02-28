---
name: ui_reviewer
description: "Review UI/UX quality and accessibility for a webpage or screenshot, with structured scoring and findings. Use when the user asks for UI review, design critique, UX audit, accessibility check, or quality scoring."
---

# UI Reviewer Skill

## Setup

No external dependencies required. Uses built-in browser + accessibility + VLM review pipeline.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Audit a live website UI/UX | `ui.review_url` |
| Review a provided screenshot/image only | `ui.review_image` |
| Accessibility-focused check only | `ui.check_accessibility` |

---

## Command Usage

### Review URL (full audit)

```bash
ui.review_url url=https://example.com --json
ui.review_url url=https://example.com lang=zh-CN device=mobile --json
```

### Review image (visual only)

```bash
ui.review_image image=<base64_png_data> lang=en-US --json
```

### Accessibility check only

```bash
ui.check_accessibility url=https://example.com --json
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing `url` | Provide valid URL for `ui.review_url` / `ui.check_accessibility` |
| Missing `image` | Provide base64 image for `ui.review_image` |
| Browser/VLM review failure | Retry, then reduce scope (accessibility-only or single viewport) |

---

## Notes

- Prefer `--json` for structured output.
- Use this skill for quality evaluation, not for generic browsing/search.
