package workspace

import (
	"path/filepath"
	"strings"
)

const (
	// SharedScratchpadAlias is the filesystem alias exposed to coordinating agents.
	SharedScratchpadAlias = "scratchpad"
	sharedScratchpadRel   = ".blue/scratchpad/shared"
)

// SharedScratchpadRelPath returns the canonical workspace-relative scratchpad path.
func SharedScratchpadRelPath() string {
	return sharedScratchpadRel
}

// SharedScratchpadDir resolves the shared scratchpad directory inside a workspace.
func SharedScratchpadDir(workspaceRoot string) string {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return ""
	}
	if !filepath.IsAbs(root) {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}
	root = filepath.Clean(root)
	if root == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(sharedScratchpadRel)))
}
