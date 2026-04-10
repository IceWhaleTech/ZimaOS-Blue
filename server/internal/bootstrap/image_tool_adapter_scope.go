package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func normalizeImageWorkspaceRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, raw := range roots {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		if _, ok := seen[cleaned]; !ok {
			seen[cleaned] = struct{}{}
			out = append(out, cleaned)
		}
		if resolved, ok := resolveImagePathAlias(cleaned); ok && resolved != cleaned {
			if _, exists := seen[resolved]; !exists {
				seen[resolved] = struct{}{}
				out = append(out, resolved)
			}
		}
	}
	return out
}

func firstSpecificImageWorkspaceRoot(roots []string) string {
	for _, root := range normalizeImageWorkspaceRoots(roots) {
		cleaned := filepath.Clean(strings.TrimSpace(root))
		if !isAbsoluteImageOutputPath(cleaned) || isGenericTempRoot(cleaned) {
			continue
		}
		return cleaned
	}
	return ""
}

func orderedImageWorkspaceRoots(primary string, roots []string) []string {
	normalized := normalizeImageWorkspaceRoots(roots)
	if strings.TrimSpace(primary) == "" {
		return normalized
	}
	primary = filepath.Clean(strings.TrimSpace(primary))
	out := make([]string, 0, len(normalized))
	if isAbsoluteImageOutputPath(primary) {
		out = append(out, primary)
	}
	for _, root := range normalized {
		if imagePathEquivalent(root, primary) {
			continue
		}
		out = append(out, root)
	}
	return normalizeImageWorkspaceRoots(out)
}

func scopeRootsForImageOutput(ctx context.Context, fallbackRoots []string) []string {
	roots, _ := tools.GetFSScope(ctx)
	if len(roots) == 0 {
		return fallbackRoots
	}
	combined := make([]string, 0, len(roots)+len(fallbackRoots))
	seen := make(map[string]struct{}, len(roots)+len(fallbackRoots))
	for _, root := range append(append([]string{}, roots...), fallbackRoots...) {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		combined = append(combined, trimmed)
	}
	return normalizeImageWorkspaceRoots(combined)
}

func relativizeImageOutputPath(outputPath string, roots []string) (string, bool) {
	cleanedOutput := filepath.Clean(strings.TrimSpace(outputPath))
	if !isAbsoluteImageOutputPath(cleanedOutput) {
		return "", false
	}
	outputCandidates := []string{cleanedOutput}
	if resolved, ok := resolveImagePathAlias(cleanedOutput); ok && resolved != cleanedOutput {
		outputCandidates = append(outputCandidates, resolved)
	}
	for _, root := range normalizeImageWorkspaceRoots(roots) {
		cleanedRoot := filepath.Clean(strings.TrimSpace(root))
		if !isAbsoluteImageOutputPath(cleanedRoot) || isGenericTempRoot(cleanedRoot) {
			continue
		}
		for _, candidate := range outputCandidates {
			rel, err := filepath.Rel(cleanedRoot, candidate)
			if err != nil {
				continue
			}
			rel = filepath.Clean(rel)
			if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			return rel, true
		}
	}
	return "", false
}

func currentImageScopeTargetsWorkspaceRoot(ctx context.Context, target string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	if !isAbsoluteImageOutputPath(target) {
		return false
	}
	roots, aliases := tools.GetFSScope(ctx)
	for _, root := range roots {
		if imagePathEquivalent(root, target) {
			return true
		}
	}
	for _, root := range aliases {
		if imagePathEquivalent(root, target) {
			return true
		}
	}
	return false
}

func imagePathEquivalent(left, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	if resolvedLeft, ok := resolveImagePathAlias(left); ok {
		left = filepath.Clean(resolvedLeft)
	}
	if resolvedRight, ok := resolveImagePathAlias(right); ok {
		right = filepath.Clean(resolvedRight)
	}
	return left == right
}

func isAbsoluteImageOutputPath(path string) bool {
	return strings.TrimSpace(path) != "" && filepath.IsAbs(filepath.Clean(path))
}

func isGenericTempRoot(root string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if !isAbsoluteImageOutputPath(cleaned) {
		return false
	}
	candidates := []string{"/tmp", "/private/tmp"}
	if tempRoot := strings.TrimSpace(os.TempDir()); tempRoot != "" {
		candidates = append(candidates, filepath.Clean(tempRoot))
		if resolved, err := filepath.EvalSymlinks(tempRoot); err == nil {
			candidates = append(candidates, filepath.Clean(resolved))
		}
	}
	for _, candidate := range candidates {
		if filepath.Clean(candidate) == cleaned {
			return true
		}
	}
	return false
}

func resolveImagePathAlias(path string) (string, bool) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.Clean(resolved), true
	}
	current := cleaned
	suffix := make([]string, 0, 4)
	for {
		info, err := os.Lstat(current)
		if err == nil && info != nil {
			resolvedCurrent, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return "", false
			}
			resolvedCurrent = filepath.Clean(resolvedCurrent)
			for idx := len(suffix) - 1; idx >= 0; idx-- {
				resolvedCurrent = filepath.Join(resolvedCurrent, suffix[idx])
			}
			return filepath.Clean(resolvedCurrent), true
		}
		parent := filepath.Dir(current)
		if parent == current || parent == "." || parent == "" {
			return "", false
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}
