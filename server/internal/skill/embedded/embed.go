// Package embedded provides go:embed access to bundled SKILL.md files.
// Source of truth: assets/skills/ at project root.
// Build-time copy: `make copy-skills` syncs assets/skills/ → skills/ here.
package embedded

import "embed"

// SkillsFS embeds all skills/*/SKILL.md files for ReleaseSkills at startup.
//
//go:embed skills/*/SKILL.md
var SkillsFS embed.FS
