package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	gitLookPath = exec.LookPath
	gitRun      = runGitCommand
)

// GitCapability describes whether the workspace can be managed safely with Git.
type GitCapability struct {
	WorkspaceDir        string `json:"workspace_dir"`
	Available           bool   `json:"available"`
	BinaryPath          string `json:"binary_path,omitempty"`
	Repository          bool   `json:"repository"`
	RepoRoot            string `json:"repo_root,omitempty"`
	WorkspaceRooted     bool   `json:"workspace_rooted"`
	CanInitialize       bool   `json:"can_initialize"`
	CanCommit           bool   `json:"can_commit"`
	UserNameConfigured  bool   `json:"user_name_configured"`
	UserEmailConfigured bool   `json:"user_email_configured"`
	Reason              string `json:"reason,omitempty"`
}

// IsGitRepositoryDir reports whether dir contains Git metadata.
// It supports both traditional repositories (.git directory) and worktrees
// where .git is a regular file pointing to the real gitdir.
func IsGitRepositoryDir(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}

// DetectGitCapability inspects whether the workspace can use Git safely.
func (m *Manager) DetectGitCapability(ctx context.Context) GitCapability {
	if ctx == nil {
		ctx = context.Background()
	}

	workspaceDir := strings.TrimSpace(m.dir)
	cap := GitCapability{WorkspaceDir: workspaceDir}
	if workspaceDir == "" {
		cap.Reason = "workspace directory is not configured"
		return cap
	}

	absWorkspace, err := filepath.Abs(workspaceDir)
	if err == nil {
		cap.WorkspaceDir = filepath.Clean(absWorkspace)
	}

	if _, err := os.Stat(workspaceDir); err != nil {
		if os.IsNotExist(err) {
			cap.Reason = "workspace directory does not exist"
		} else {
			cap.Reason = fmt.Sprintf("failed to inspect workspace directory: %v", err)
		}
		return cap
	}

	gitPath, err := gitLookPath("git")
	if err != nil {
		cap.Repository = IsGitRepositoryDir(workspaceDir)
		if cap.Repository {
			cap.Reason = "git metadata exists but git executable is not available in PATH"
		} else {
			cap.Reason = "git executable is not available in PATH"
		}
		return cap
	}

	cap.Available = true
	cap.BinaryPath = gitPath
	cap.UserNameConfigured = gitConfigPresent(ctx, workspaceDir, "user.name")
	cap.UserEmailConfigured = gitConfigPresent(ctx, workspaceDir, "user.email")

	repoRoot, err := gitRun(ctx, workspaceDir, "rev-parse", "--show-toplevel")
	if err == nil && strings.TrimSpace(repoRoot) != "" {
		cap.Repository = true
		cap.RepoRoot = cleanPathOrOriginal(repoRoot)
		cap.WorkspaceRooted = samePath(cap.WorkspaceDir, cap.RepoRoot)
	} else if IsGitRepositoryDir(workspaceDir) {
		cap.Repository = true
		cap.RepoRoot = cap.WorkspaceDir
		cap.WorkspaceRooted = true
	}

	cap.CanInitialize = cap.Available && !cap.Repository
	cap.CanCommit = cap.Available && cap.Repository && cap.WorkspaceRooted && cap.UserNameConfigured && cap.UserEmailConfigured

	switch {
	case cap.CanCommit:
		return cap
	case cap.CanInitialize:
		cap.Reason = "git is available; workspace is not initialized yet"
	case cap.Repository && !cap.WorkspaceRooted && cap.RepoRoot != "":
		cap.Reason = fmt.Sprintf("workspace is inside a git repository rooted at %s", cap.RepoRoot)
	case cap.Repository && !cap.WorkspaceRooted:
		cap.Reason = "workspace is inside a git repository rooted outside the workspace"
	case cap.Repository && !cap.UserNameConfigured && !cap.UserEmailConfigured:
		cap.Reason = "git user.name and user.email are not configured"
	case cap.Repository && !cap.UserNameConfigured:
		cap.Reason = "git user.name is not configured"
	case cap.Repository && !cap.UserEmailConfigured:
		cap.Reason = "git user.email is not configured"
	default:
		cap.Reason = "git capability unavailable"
	}

	return cap
}

func gitConfigPresent(ctx context.Context, dir string, key string) bool {
	value, err := gitRun(ctx, dir, "config", "--get", key)
	if err != nil {
		return false
	}
	return strings.TrimSpace(value) != ""
}

func runGitCommand(parent context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func cleanPathOrOriginal(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(absPath)
}

func samePath(left string, right string) bool {
	left = cleanPathOrOriginal(left)
	right = cleanPathOrOriginal(right)
	if left == "" || right == "" {
		return false
	}
	return left == right
}
