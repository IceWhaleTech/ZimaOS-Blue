package bootstrap

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

func resolveBuiltinToolAllowedPaths(appCfg *config.Config, dataDir string) []string {
	workspaceRoot := ResolveWorkspaceDir(dataDir, appCfg)

	paths := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	addPath := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		clean := normalizeWorkspacePath(trimmed)
		if clean == "" {
			return
		}
		if _, ok := seen[clean]; !ok {
			seen[clean] = struct{}{}
			paths = append(paths, clean)
		}
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			resolved = filepath.Clean(resolved)
			if _, ok := seen[resolved]; !ok {
				seen[resolved] = struct{}{}
				paths = append(paths, resolved)
			}
		}
	}

	addPath(workspaceRoot)
	for _, skillRoot := range skillmanifest.ResolveRoots(workspaceRoot) {
		addPath(skillRoot)
		if filepath.Base(filepath.Clean(skillRoot)) == "skills" {
			addPath(filepath.Dir(skillRoot))
		}
	}
	if shouldAllowTmpForWorkspace(paths) {
		addPath("/tmp")
		addPath("/private/tmp")
		if tmpDir := strings.TrimSpace(os.TempDir()); tmpDir != "" {
			addPath(tmpDir)
		}
	}
	return paths
}

func shouldAllowTmpForWorkspace(paths []string) bool {
	tempRoots := []string{"/tmp", "/private/tmp"}
	if tmpDir := strings.TrimSpace(os.TempDir()); tmpDir != "" {
		tempRoots = append(tempRoots, tmpDir)
		if resolved, err := filepath.EvalSymlinks(tmpDir); err == nil {
			tempRoots = append(tempRoots, resolved)
		}
	}
	for _, candidate := range paths {
		for _, root := range tempRoots {
			if pathWithinRoot(root, candidate) {
				return true
			}
		}
	}
	return false
}

func pathWithinRoot(root, target string) bool {
	root = filepath.Clean(strings.TrimSpace(root))
	target = filepath.Clean(strings.TrimSpace(target))
	if root == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
