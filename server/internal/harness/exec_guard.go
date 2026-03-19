package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type ExecPathGuard struct {
	manager *Controller
}

func NewExecPathGuard(manager *Controller) *ExecPathGuard {
	if manager == nil {
		return nil
	}
	return &ExecPathGuard{manager: manager}
}

func (g *ExecPathGuard) CheckExecWorkdir(ctx context.Context, absWorkdir string) error {
	return g.checkPath(ctx, absWorkdir, "workdir")
}

func (g *ExecPathGuard) CheckExecPath(ctx context.Context, absPath string) error {
	return g.checkPath(ctx, absPath, "path")
}

func (g *ExecPathGuard) checkPath(ctx context.Context, rawPath string, label string) error {
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

	clean, err := filepath.Abs(strings.TrimSpace(rawPath))
	if err != nil {
		return fmt.Errorf("exec denied: invalid %s", label)
	}
	if workspaceRoot := strings.TrimSpace(run.WorkspaceRoot); workspaceRoot != "" {
		workspaceAbs, absErr := filepath.Abs(workspaceRoot)
		if absErr == nil && !pathWithinRoot(workspaceAbs, clean) {
			return fmt.Errorf("exec denied: %s %q escapes run workspace_root", label, displayRunPath(run, clean))
		}
	}
	if g.manager.resolver != nil {
		if protected, reason := g.manager.resolver.protectedPathReason(clean, run.ArtifactRoot); protected {
			return fmt.Errorf("exec denied: %s", reason)
		}
	}
	return nil
}
