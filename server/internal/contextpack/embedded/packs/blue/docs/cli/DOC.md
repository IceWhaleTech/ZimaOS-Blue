---
id: blue/cli
type: doc
description: Blue CLI quick reference for workspace, skills, sessions, and diagnostics.
source_trust: official
tags: [blue, cli, workspace, skills, sessions, diagnostics]
languages: [en]
revision: "1"
updated_on: "2026-03-11"
---

# Blue CLI Quick Reference

Use `blue` for local diagnostics and agent/runtime operations.

## Common commands

- `blue status` shows runtime summary.
- `blue health` checks core services.
- `blue sessions list|show|delete|clear` manages chat sessions.
- `blue skills list|info|check` inspects available skills.
- `blue models status` checks local and configured models.
- `blue gateway` runs the gateway service.

## Workspace-oriented commands

- Prefer workspace-driven customization through files under the data workspace.
- Use `SOUL.md`, `USER.md`, `IDENTITY.md`, `AGENTS.md`, and `MEMORY.md` for durable guidance.

## Context pack usage

- `blue context search <query>` searches local context packs.
- `blue context get <id>` loads the primary pack document.
- `blue context annotate <id> <note>` saves a local note for future prompt augmentation.

## Operational guidance

- Prefer built-in commands before external scripts.
- Keep destructive actions explicit and auditable.
- When debugging model/tool routing, inspect session and audit information first.
