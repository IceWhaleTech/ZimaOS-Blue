package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestResolveBuiltinToolAllowedPathsPrefersConfiguredWorkspace(t *testing.T) {
	cfg := &config.Config{}
	cfg.AgentCore.WorkspaceDir = filepath.Join(t.TempDir(), "bench-workspace")

	paths := resolveBuiltinToolAllowedPaths(cfg, filepath.Join(t.TempDir(), "data"))
	if len(paths) == 0 {
		t.Fatal("expected non-empty allowed paths")
	}

	want, err := filepath.Abs(cfg.AgentCore.WorkspaceDir)
	if err != nil {
		t.Fatalf("abs workspace dir: %v", err)
	}
	if !containsCleanPath(paths, want) {
		t.Fatalf("expected configured workspace root in allowed paths, got=%v want=%q", paths, want)
	}
}

func TestResolveWorkspaceDirPrefersConfiguredWorkspace(t *testing.T) {
	cfg := &config.Config{}
	cfg.AgentCore.WorkspaceDir = filepath.Join(t.TempDir(), "bench-workspace")

	got := ResolveWorkspaceDir(filepath.Join(t.TempDir(), "data"), cfg)
	want, err := filepath.Abs(cfg.AgentCore.WorkspaceDir)
	if err != nil {
		t.Fatalf("abs workspace dir: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("expected configured workspace dir, got=%q want=%q", got, want)
	}
}

func TestResolveWorkspaceDirIgnoresDefaultDotWorkspace(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	cfg := &config.Config{}
	cfg.AgentCore.WorkspaceDir = "."

	got := ResolveWorkspaceDir(dataDir, cfg)
	want := filepath.Join(dataDir, "workspace")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("expected fallback workspace dir, got=%q want=%q", got, want)
	}
}

func TestResolveBuiltinToolAllowedPathsAddsTmpWhenWorkspaceIsTemporary(t *testing.T) {
	cfg := &config.Config{}
	cfg.AgentCore.WorkspaceDir = filepath.Join("/tmp", "pinchbench-workspace")

	paths := resolveBuiltinToolAllowedPaths(cfg, filepath.Join(t.TempDir(), "data"))
	if !containsCleanPath(paths, "/tmp") {
		t.Fatalf("expected /tmp in allowed paths for temp workspace, got=%v", paths)
	}
}

func TestResolveBuiltinToolAllowedPathsKeepsDefaultWorkspaceWhenUnset(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	paths := resolveBuiltinToolAllowedPaths(&config.Config{}, dataDir)

	want := filepath.Join(dataDir, "workspace")
	if !containsCleanPath(paths, want) {
		t.Fatalf("expected default data workspace root in allowed paths, got=%v want=%q", paths, want)
	}
}

func containsCleanPath(paths []string, target string) bool {
	cleanTarget := filepath.Clean(target)
	for _, path := range paths {
		if filepath.Clean(path) == cleanTarget {
			return true
		}
	}
	return false
}
