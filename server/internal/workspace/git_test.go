package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsGitRepositoryDir(t *testing.T) {
	t.Run("git directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
		if !IsGitRepositoryDir(dir) {
			t.Fatal("expected .git directory to be detected")
		}
	})

	t.Run("git file worktree", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/example"), 0o644); err != nil {
			t.Fatalf("write .git file: %v", err)
		}
		if !IsGitRepositoryDir(dir) {
			t.Fatal("expected .git file to be detected")
		}
	})

	t.Run("missing git metadata", func(t *testing.T) {
		if IsGitRepositoryDir(t.TempDir()) {
			t.Fatal("did not expect git metadata to be detected")
		}
	})
}

func TestDetectGitCapabilityWithoutGitBinary(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	origLookPath := gitLookPath
	origRun := gitRun
	t.Cleanup(func() {
		gitLookPath = origLookPath
		gitRun = origRun
	})

	gitLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	gitRun = func(_ context.Context, _ string, _ ...string) (string, error) {
		t.Fatal("gitRun should not be called when git is missing")
		return "", nil
	}

	got := mgr.DetectGitCapability(context.Background())
	if got.Available {
		t.Fatal("expected git to be unavailable")
	}
	if got.CanInitialize || got.CanCommit {
		t.Fatalf("expected no git actions to be available, got %+v", got)
	}
	if !strings.Contains(got.Reason, "not available") {
		t.Fatalf("expected missing-git reason, got %q", got.Reason)
	}
}

func TestDetectGitCapabilityRepoRootedAndCommitReady(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	origLookPath := gitLookPath
	origRun := gitRun
	t.Cleanup(func() {
		gitLookPath = origLookPath
		gitRun = origRun
	})

	gitLookPath = func(file string) (string, error) {
		return "/usr/bin/git", nil
	}
	gitRun = func(_ context.Context, gotDir string, args ...string) (string, error) {
		if gotDir != dir {
			t.Fatalf("gitRun dir = %q, want %q", gotDir, dir)
		}
		switch strings.Join(args, " ") {
		case "config --get user.name":
			return "Blue", nil
		case "config --get user.email":
			return "blue@example.com", nil
		case "rev-parse --show-toplevel":
			return dir, nil
		default:
			t.Fatalf("unexpected git args: %v", args)
			return "", nil
		}
	}

	got := mgr.DetectGitCapability(context.Background())
	if !got.Available || !got.Repository {
		t.Fatalf("expected repo capability, got %+v", got)
	}
	if !got.WorkspaceRooted {
		t.Fatalf("expected workspace_rooted=true, got %+v", got)
	}
	if !got.CanCommit {
		t.Fatalf("expected can_commit=true, got %+v", got)
	}
	if got.CanInitialize {
		t.Fatalf("did not expect can_initialize for existing repo, got %+v", got)
	}
	if got.Reason != "" {
		t.Fatalf("expected empty reason when commit-ready, got %q", got.Reason)
	}
	if !samePath(got.RepoRoot, dir) {
		t.Fatalf("repo_root = %q, want %q", got.RepoRoot, dir)
	}
}
