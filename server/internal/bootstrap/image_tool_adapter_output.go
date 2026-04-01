package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
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
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func withImageOutputWorkspaceScope(ctx context.Context, outputPath string, fallbackRoots []string) (context.Context, string) {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return ctx, trimmed
	}
	if rel, ok := relativizeImageOutputPath(trimmed, scopeRootsForImageOutput(ctx, fallbackRoots)); ok {
		return withImageWorkspaceOverride(ctx, fallbackRoots), rel
	}
	if filepath.IsAbs(trimmed) {
		return ctx, trimmed
	}
	roots, aliases := tools.GetFSScope(ctx)
	if len(roots) > 0 || len(aliases) > 0 {
		return ctx, trimmed
	}
	if len(fallbackRoots) == 0 {
		return ctx, trimmed
	}
	return withImageWorkspaceOverride(ctx, fallbackRoots), trimmed
}

func withImageWorkspaceOverride(ctx context.Context, fallbackRoots []string) context.Context {
	if len(fallbackRoots) == 0 {
		return ctx
	}
	scopeAliases := map[string]string{"workspace": fallbackRoots[0]}
	return tools.WithFSRootOverride(ctx, fallbackRoots, scopeAliases)
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
	return combined
}

func relativizeImageOutputPath(outputPath string, roots []string) (string, bool) {
	cleanedOutput := filepath.Clean(strings.TrimSpace(outputPath))
	if cleanedOutput == "" || !filepath.IsAbs(cleanedOutput) {
		return "", false
	}
	for _, root := range roots {
		cleanedRoot := filepath.Clean(strings.TrimSpace(root))
		if cleanedRoot == "" || !filepath.IsAbs(cleanedRoot) {
			continue
		}
		rel, err := filepath.Rel(cleanedRoot, cleanedOutput)
		if err != nil {
			continue
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return "", false
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return rel, true
	}
	return "", false
}

func imageTaskLookupUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if claims, ok := ctx.Value(auth.UserContextKey).(*auth.UserClaims); ok && claims != nil {
		if userID := strings.TrimSpace(claims.UserID); userID != "" {
			return userID
		}
	}
	return strings.TrimSpace(tools.GetUserID(ctx))
}

func waitForImageTask(ctx context.Context, manager *mediagen.Manager, taskID, lookupUserID string) (*mediagen.MediaTask, error) {
	if manager == nil {
		return nil, fmt.Errorf("media manager unavailable")
	}
	if strings.TrimSpace(lookupUserID) != "" {
		task, err := manager.WaitForTask(ctx, taskID, lookupUserID)
		if err == nil || !errors.Is(err, mediagen.ErrTaskNotFound) {
			return task, err
		}
	}
	return manager.WaitForTask(ctx, taskID)
}

func saveGeneratedImageOutput(ctx context.Context, manager *mediagen.Manager, task *mediagen.MediaTask, outputPath, lookupUserID string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" || task == nil {
		return "", nil
	}
	if task.Response == nil || len(task.Response.Data) == 0 {
		refreshed, err := loadImageTaskForOutput(manager, task.ID, lookupUserID)
		if err != nil {
			return "", err
		}
		task = refreshed
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		return "", fmt.Errorf("generated image has no output data")
	}

	data, err := loadGeneratedImageData(manager, task.Response.Data[0])
	if err != nil {
		return "", err
	}
	return tools.WriteBinaryArtifact(ctx, outputPath, data)
}

func loadImageTaskForOutput(manager *mediagen.Manager, taskID, lookupUserID string) (*mediagen.MediaTask, error) {
	if strings.TrimSpace(lookupUserID) != "" {
		refreshed, err := manager.GetTask(taskID, lookupUserID)
		if err != nil && errors.Is(err, mediagen.ErrTaskNotFound) {
			return manager.GetTask(taskID)
		}
		return refreshed, err
	}
	return manager.GetTask(taskID)
}

func loadGeneratedImageData(manager *mediagen.Manager, item mediagen.MediaResult) ([]byte, error) {
	switch {
	case strings.TrimSpace(item.B64JSON) != "":
		return decodeCompatImageBase64(item.B64JSON)
	case strings.TrimSpace(item.URL) != "":
		return manager.ReadServedURL(item.URL)
	default:
		return nil, fmt.Errorf("generated image has no retrievable asset")
	}
}

func joinImageMessages(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, ". ")
}
