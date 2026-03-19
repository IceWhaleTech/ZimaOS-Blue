package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type WritePathGuard struct {
	manager *Controller
}

func NewWritePathGuard(manager *Controller) *WritePathGuard {
	if manager == nil {
		return nil
	}
	return &WritePathGuard{manager: manager}
}

func (g *WritePathGuard) CheckWritePath(ctx context.Context, absPath string) error {
	if g == nil || g.manager == nil {
		return nil
	}
	runID := strings.TrimSpace(tools.GetRunID(ctx))
	if runID == "" {
		return nil
	}
	run, err := g.manager.GetStored(ctx, runID)
	if err != nil {
		return err
	}

	clean, err := filepath.Abs(strings.TrimSpace(absPath))
	if err != nil {
		return fmt.Errorf("direct write denied: invalid path")
	}

	if workspaceRoot := strings.TrimSpace(run.WorkspaceRoot); workspaceRoot != "" {
		workspaceAbs, absErr := filepath.Abs(workspaceRoot)
		if absErr == nil && !pathWithinRoot(workspaceAbs, clean) {
			return fmt.Errorf("direct write denied: %q escapes run workspace_root", displayRunPath(run, clean))
		}
	}

	if g.manager.resolver != nil {
		if protected, reason := g.manager.resolver.protectedPathReason(clean, run.ArtifactRoot); protected {
			return fmt.Errorf("direct write denied: %s", reason)
		}
	}
	return nil
}

func pathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func displayRunPath(run *Run, absPath string) string {
	if run == nil {
		return absPath
	}
	root := strings.TrimSpace(run.WorkspaceRoot)
	if root == "" {
		return absPath
	}
	workspaceAbs, err := filepath.Abs(root)
	if err != nil {
		return absPath
	}
	if rel, relErr := filepath.Rel(workspaceAbs, absPath); relErr == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return absPath
}
