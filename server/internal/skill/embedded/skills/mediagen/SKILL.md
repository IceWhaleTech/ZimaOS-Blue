---
name: mediagen
description: Generate images and videos through media pipeline categories including t2i t2v i2v and i2i.
---

# Media Generation

Generate images and videos using AI models via the IR pipeline.

## How It Works

User messages are classified by the IR (Intent Recognition) layer before reaching the LLM. If a media generation intent is detected, the system routes directly to the media pipeline — no LLM tool-use needed.

## Supported Categories

| Category | ID | Description |
|----------|----|-------------|
| Text-to-Image | `t2i` | Generate image from text prompt |
| Text-to-Video | `t2v` | Generate video from text prompt |
| Image-to-Video | `i2v` | Animate a reference image |
| Image Editing | `i2i` | Modify/transform an existing image |

## API Endpoints

- `POST /api/v1/media/classify` — Classify message intent
- `POST /api/v1/media/generate` — Create generation task (returns task ID)
- `GET /api/v1/media/tasks/:id` — Poll task status
- `GET /api/v1/media/tasks/by-message/:message_id` — Tasks by message
- `GET /api/v1/media/models?category=t2i` — List available models

## Example Triggers

- "Draw me a cat on a rainbow"
- "生成一张日落的图片"
- "Create a video of waves crashing"
- "把这张图片变成动画"
