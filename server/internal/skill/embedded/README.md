DO NOT EDIT files under `skills/` directly.

The source of truth for all SKILL.md files is `assets/skills/` at the project root.
The `skills/` subdirectory here is a build-time copy created by `make copy-skills`.

To add or modify a skill:
1. Edit `assets/skills/<name>/SKILL.md`
2. Run `make copy-skills` (or the full build) to sync here

Any changes made directly under `skills/` will be overwritten on the next build.
