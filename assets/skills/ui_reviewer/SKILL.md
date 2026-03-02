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
| Audit a live website UI/UX | `blue ui.review_url` |
| Review a provided screenshot/image only | `blue ui.review_image` |
| Accessibility-focused check only | `blue ui.check_accessibility` |

---

## Command Usage

### Review URL (full audit)

```bash
blue ui.review_url url=https://example.com --json
blue ui.review_url url=https://example.com lang=zh-CN device=mobile --json
```

### Review image (visual only)

```bash
blue ui.review_image image=<base64_png_data> lang=en-US --json
```

### Accessibility check only

```bash
blue ui.check_accessibility url=https://example.com --json
```

---

## Error Handling

| Error | Resolution |
|-------|------------|
| Missing `url` | Provide valid URL for `blue ui.review_url` / `blue ui.check_accessibility` |
| Missing `image` | Provide base64 image for `blue ui.review_image` |
| Browser/VLM review failure | Retry, then reduce scope (accessibility-only or single viewport) |

---

## Notes

- Prefer `--json` for structured output.
- Use this skill for quality evaluation, not for generic browsing/search.
