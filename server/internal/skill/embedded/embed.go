// Package embedded provides go:embed access to bundled SKILL.md files.
package embedded

import "embed"

//go:embed skills/*/SKILL.md
var SkillsFS embed.FS
