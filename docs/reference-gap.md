# ZimaOS Blue Gap Baseline

## Baseline
- Source repo: reference implementation repository
- Branch: `main`
- Submodule path: local third-party reference checkout
- Baseline commit: `ed8e0a814609a070f46a0b6942dadc8e06ebf9d6`
- Baseline date: `2026-03-04`

## Scope
- End-to-end chat chain comparison baseline against the reference implementation.
- Web chat API + streaming.
- IM channel request path.
- Gateway REST + WebSocket contract.
- Prompt policy and agent loop policy.
- Tool/skill selector control plane.
- OpenClaw-style high-frequency factory tools used by agent runtimes.

## Initial Gap Summary
- Session control plane had handler implementation but no route wiring in bootstrap.
- Gateway runtime existed but was not enabled in main route chain.
- Frontend gateway methods were defined without backend method registration.
- Smart skill selector backend settings existed but frontend store exposure was incomplete.
- Prompt/loop logic existed but lacked a unified policy observability surface.
- Several OpenClaw-compatible factory tools were exposed to the model but lacked native Blue-backed implementations.

## Current Core Parity Status
- Native session tools are in place: `sessions_list`, `sessions_history`, `session_status`, `sessions_spawn`, `sessions_send`.
- Native agent policy tools are in place: `agents_list`, `subagents`.
- Native runtime/control tools are in place: `gateway`, `cron`, `message`, `tts`.
- Native workflow/UI tools are in place: `browser`, `canvas`, `nodes`, and restored `/api/v1/a2ui/*` canvas routes.
- Native document tool is in place: `pdf` with `pdfium/webassembly` extraction, OCR fallback, LLM vision fallback, deduped multi-PDF input, and local/remote PDF URL support.
- Native media tool is in place: `image` now supports VLM-based image understanding with local OCR fallback, multi-image input with aggregate summary output, explicit compare mode plus structured shared/different keywords, local image path/file:// input, inline image normalization (PNG/JPEG/WebP-friendly), direct remote-image URL download/recognition including signed or extension-less image URLs, generation/edit, and task status lookup; dedicated UI audit remains in `ui_reviewer`.

## Pain Points Addressed
- Removed the prior `tool exposed but not really wired` problem for core session/agent/gateway/browser/workflow/speech modules.
- Restored the early Blue `canvas`/`agent2ui` capability that had been cut mainly for package-size reasons.
- Upgraded PDF handling for scanned/image-only files instead of only supporting text-layer PDFs.
- Unified image usage into a native VLM-first tool surface with local OCR fallback and direct remote-image URL recognition, including signed or extension-less image URLs so the agent no longer has to rely mainly on fragile exec/skill alias hops for image understanding vs generation vs status.
- Added OCR and vision fallbacks to reduce the common `empty extraction` failure mode reported for complex PDFs.
- Fixed native `memory_*` round-tripping so `write -> search -> get -> forget` stays consistent even when the secondary SQLite layer has no FTS5 support; legacy aliases remain on the same path.
- Fixed sqlite-vec `int8` contract issues in the secondary memory layer so live writes no longer fail with `expected int8, got float32`, and KNN queries now use the required `k = ?` path with query-side quantization.
- Made `skillstore` degrade cleanly without FTS5 so startup no longer emits noisy setup warnings and stale FTS triggers no longer break later writes/searches.
- Fixed `skillstore` catalog search/browse against partial or sparse rows so nullable text fields no longer cause runtime 500s during result scanning.
- Fixed media generation's unconfigured-provider path so `/api/v1/media/images/generations` and `/api/v1/media/generate` now return an actionable `503` with setup guidance instead of a raw internal error.
- Fixed media provider config-ID vs runtime-name drift so enable/disable now actually unregisters the intended backend, and model conflict resolution once again follows configured provider priority.
- Added a dev-only fake media provider behind `ZIMA_ENABLE_FAKE_MEDIA_PROVIDER=1`, so Blue can now smoke-test `/api/v1/media/providers`, `/api/v1/media/images/generations`, `/api/v1/media/generate`, and `/api/v1/media/tasks/:id` end to end without real vendor API keys.

## Deliverables
- Reference submodule pinned in repository.
- Gap checker JSON report: `docs/reports/chat-chain-gap.json`.
- Gap checker Markdown report: `docs/reports/chat-chain-gap.md`.
- Contract alignment for gateway methods:
- `chat.send`
- `chat.abort`
- `sessions.list`
- `sessions.reset`
- `browser.request`
- `hooks.wake`

## Remaining Non-Core Follow-ups
- Consider exposing PDF OCR/vision tuning knobs in config once runtime defaults settle.
- `memory_search` / `memory_get` / `memory_write` / `memory_forget` now route through native Blue-backed wrappers instead of compat HTTP fallback; legacy aliases like `memory_read` / `memory_store` / `memory_delete` stay on the same native path too.
- `tts` now routes to native Blue speech services for status/config, voice listing, local speak/stop, and provider-aware audio synthesis instead of compat HTTP fallback.
- Re-check whether any lower-frequency compatibility aliases should still be converted from exec fallback to native tools.

## Notes
- This document tracks baseline and current parity status only.
- Detailed per-check output is generated by the gap checker tool.
- Core OpenClaw-style tool parity is now focused on native or directly wired Blue implementations for the highest-frequency modules.
