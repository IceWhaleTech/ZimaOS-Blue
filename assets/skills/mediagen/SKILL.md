---
name: mediagen
version: "1.0.0"
description: "Generate images or videos through the built-in media pipeline. Use explicit `blue media generate` / `blue media status` commands when the user wants media creation, task polling, or model/category control."
invocation: "blue media generate \"Draw a cat on a rainbow\" --category t2i"
examples:
  - "blue media generate \"Draw a cat on a rainbow\" --category t2i"
  - "blue media status media-task-1"
capability_tags:
  - media
  - image
  - video
interaction_mode: stateless
card_support: none
---

# Media Generation

Generate images and videos through the built-in media pipeline.

## Setup

No external dependencies are required for the CLI surface itself. Availability still depends on configured media providers or enabled fallback engines in the running system.

---

## Task Routing

| User Intent | Action |
|-------------|--------|
| Explicitly generate an image or video from a prompt | `blue media generate "..." --category ...` |
| Check whether a submitted media task has finished | `blue media status <task_id>` |
| User wants image creation in normal chat and no special control is needed | The chat runtime may auto-route some image-generation intents directly to the media pipeline |
| User wants screenshot critique, accessibility review, or visual QA | Use `research` with `mode=ui_review`, not `mediagen` |
| User wants generic web/image search rather than generation | Use `web_query` or the built-in `browser` tool, not `mediagen` |

---

## Command Usage

Generate media:

```bash
blue media generate "Draw a cat on a rainbow" --category t2i
blue media generate "Create a video of waves crashing at sunset" --category t2v
blue media generate "Animate this product photo" --category i2v --model <model>
blue media generate "Restyle this image as a watercolor painting" --category i2i --poll
```

Check status:

```bash
blue media status media-task-1
blue media status media-task-1 --json
```

Useful flags:

- `--category t2i|t2v|i2v|i2i`
- `--model <model>`
- `--size <width>x<height>`
- `--poll`
- `--poll-interval <seconds>`

---

## Supported Categories

| Category | Meaning |
|----------|---------|
| `t2i` | Text to image |
| `t2v` | Text to video |
| `i2v` | Image to video |
| `i2i` | Image editing or transformation |

---

## Error Handling

| Error | Resolution |
|-------|------------|
| No provider available for the requested category/model | Check provider configuration or retry with a different category/model |
| Task remains pending/processing | Use `blue media status <task_id>` or `--poll` until completion |
| Generation failed or returned no results | Retry with a simpler prompt, different model, or different category |
| User asks for image review instead of generation | Route to `research` with `mode=ui_review` |

---

## Notes

- Use `blue media generate`, not the older dotted form `blue media.generate`.
- The chat runtime can auto-intercept some image-generation intents before normal tool selection, but explicit CLI commands are the clearest path when the user wants deterministic control or task polling.
- Image review and accessibility checking are separate capabilities; `mediagen` is for creation, not critique.
- The underlying media service is asynchronous, so task IDs and follow-up status checks are part of the normal workflow.
