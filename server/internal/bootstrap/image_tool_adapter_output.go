package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func withImageOutputWorkspaceScope(ctx context.Context, outputPath string, fallbackRoots []string) (context.Context, string) {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return ctx, trimmed
	}
	preferredFallbackRoots := normalizeImageWorkspaceRoots(fallbackRoots)
	preferredWorkspaceRoot := firstSpecificImageWorkspaceRoot(preferredFallbackRoots)
	if rel, ok := relativizeImageOutputPath(trimmed, scopeRootsForImageOutput(ctx, preferredFallbackRoots)); ok {
		overrideRoots := preferredFallbackRoots
		if preferredWorkspaceRoot != "" {
			overrideRoots = orderedImageWorkspaceRoots(preferredWorkspaceRoot, preferredFallbackRoots)
		}
		return withImageWorkspaceOverride(ctx, overrideRoots), rel
	}
	if isAbsoluteImageOutputPath(trimmed) {
		return ctx, trimmed
	}
	roots, aliases := tools.GetFSScope(ctx)
	if preferredWorkspaceRoot != "" && !currentImageScopeTargetsWorkspaceRoot(ctx, preferredWorkspaceRoot) {
		return withImageWorkspaceOverride(ctx, orderedImageWorkspaceRoots(preferredWorkspaceRoot, preferredFallbackRoots)), trimmed
	}
	if len(roots) > 0 || len(aliases) > 0 {
		return ctx, trimmed
	}
	if len(preferredFallbackRoots) == 0 {
		return ctx, trimmed
	}
	return withImageWorkspaceOverride(ctx, preferredFallbackRoots), trimmed
}

func withImageWorkspaceOverride(ctx context.Context, fallbackRoots []string) context.Context {
	if len(fallbackRoots) == 0 {
		return ctx
	}
	scopeAliases := map[string]string{"workspace": fallbackRoots[0]}
	return tools.WithFSRootOverride(ctx, fallbackRoots, scopeAliases)
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
