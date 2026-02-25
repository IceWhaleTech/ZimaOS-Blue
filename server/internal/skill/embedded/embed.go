// Package embedded previously provided go:embed access to bundled SKILL.md files.
// Skills are now loaded from the database / skill store at runtime.
package embedded

import "embed"

// SkillsFS is an empty filesystem kept for backward compatibility.
// The skills/*/SKILL.md files were removed in favor of runtime-loaded skills.
var SkillsFS embed.FS
