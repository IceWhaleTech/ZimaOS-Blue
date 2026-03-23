package bootstrap

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newImageGenerateAdapter(manager *mediagen.Manager, workspaceRoots []string) tools.ImageGenerateFunc {
	if manager == nil {
		return nil
	}
	fallbackRoots := normalizeImageWorkspaceRoots(workspaceRoots)
	return func(ctx context.Context, req tools.ImageGenerateRequest) (*tools.ImageTaskResult, error) {
		lookupUserID := imageTaskLookupUserID(ctx)
		mediaReq := &mediagen.MediaRequest{
			Type:           mediagen.MediaTypeImage,
			Prompt:         req.Prompt,
			NegativePrompt: req.NegativePrompt,
			Model:          req.Model,
			N:              req.Count,
			Size:           req.Size,
			Quality:        req.Quality,
			Style:          req.Style,
			Extra:          cloneAnyMap(req.Extra),
		}
		if req.OutputPath != "" {
			if mediaReq.Extra == nil {
				mediaReq.Extra = make(map[string]interface{})
			}
			mediaReq.Extra["path"] = req.OutputPath
		}
		if req.ReferenceImageURL != "" {
			mediaReq.ReferenceURL = req.ReferenceImageURL
			mediaReq.ReferenceURLs = []string{req.ReferenceImageURL}
		}
		if req.ReferenceImageBase64 != "" {
			decoded, err := decodeCompatImageBase64(req.ReferenceImageBase64)
			if err != nil {
				return nil, err
			}
			mediaReq.ReferenceImage = decoded
		}

		task, err := manager.Generate(ctx, mediaReq)
		if err != nil {
			return nil, err
		}
		if req.Wait {
			waitTimeout := req.WaitTimeout
			if waitTimeout <= 0 {
				waitTimeout = 120 * time.Second
			}
			waitBase := context.Background()
			if lookupUserID != "" {
				waitBase = tools.WithUserID(waitBase, lookupUserID)
			}
			waitCtx, cancel := context.WithTimeout(waitBase, waitTimeout)
			defer cancel()
			waitedTask, waitErr := waitForImageTask(waitCtx, manager, task.ID, lookupUserID)
			if waitedTask != nil {
				task = waitedTask
			}
			result := imageTaskFromMediaTask(task, req.Category)
			if result != nil && req.OutputPath != "" {
				if result.Request == nil {
					result.Request = make(map[string]interface{})
				}
				result.Request["path"] = req.OutputPath
			}
			if waitErr != nil {
				result.Message = waitErr.Error()
				if result.Status == "" {
					result.Status = "processing"
				}
				return result, nil
			}
			saveCtx, scopedOutputPath := withImageOutputWorkspaceScope(ctx, req.OutputPath, fallbackRoots)
			if savedPath, saveErr := saveGeneratedImageOutput(saveCtx, manager, task, scopedOutputPath, lookupUserID); saveErr == nil && savedPath != "" {
				if result.Request == nil {
					result.Request = make(map[string]interface{})
				}
				result.Request["path"] = savedPath
				result.Message = strings.TrimSpace(joinImageMessages(result.Message, fmt.Sprintf("saved generated image to %s", savedPath)))
			} else if saveErr != nil {
				result.Message = strings.TrimSpace(joinImageMessages(result.Message, fmt.Sprintf("generated image but failed to save %q: %v", req.OutputPath, saveErr)))
			}
			return result, nil
		}
		return imageTaskFromMediaTask(task, req.Category), nil
	}
}

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
		var (
			refreshed *mediagen.MediaTask
			err       error
		)
		if strings.TrimSpace(lookupUserID) != "" {
			refreshed, err = manager.GetTask(task.ID, lookupUserID)
			if err != nil && errors.Is(err, mediagen.ErrTaskNotFound) {
				refreshed, err = manager.GetTask(task.ID)
			}
		} else {
			refreshed, err = manager.GetTask(task.ID)
		}
		if err != nil {
			return "", err
		}
		task = refreshed
	}
	if task == nil || task.Response == nil || len(task.Response.Data) == 0 {
		return "", fmt.Errorf("generated image has no output data")
	}

	first := task.Response.Data[0]
	var (
		data []byte
		err  error
	)
	switch {
	case strings.TrimSpace(first.B64JSON) != "":
		data, err = decodeCompatImageBase64(first.B64JSON)
	case strings.TrimSpace(first.URL) != "":
		data, err = manager.ReadServedURL(first.URL)
	default:
		err = fmt.Errorf("generated image has no retrievable asset")
	}
	if err != nil {
		return "", err
	}
	return tools.WriteBinaryArtifact(ctx, outputPath, data)
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

func newImageOCRAdapter(service *ocrruntime.TesseractService) tools.ImageOCRService {
	if service == nil {
		return nil
	}
	return imageOCRAdapter{service: service}
}

type imageOCRAdapter struct {
	service *ocrruntime.TesseractService
}

func (a imageOCRAdapter) Extract(ctx context.Context, imagePNG []byte) (tools.ImageOCRResult, error) {
	result, err := a.service.Extract(ctx, imagePNG)
	return tools.ImageOCRResult{
		Text:     result.Text,
		Engine:   result.Engine,
		Model:    result.Model,
		Warnings: append([]string(nil), result.Warnings...),
	}, err
}

func newImageTaskLookupAdapter(manager *mediagen.Manager) tools.ImageTaskLookupFunc {
	if manager == nil {
		return nil
	}
	return func(ctx context.Context, taskID string) (*tools.ImageTaskResult, error) {
		lookupUserID := strings.TrimSpace(tools.GetUserID(ctx))
		var (
			task *mediagen.MediaTask
			err  error
		)
		if lookupUserID != "" {
			task, err = manager.GetTask(taskID, lookupUserID)
		} else {
			task, err = manager.GetTask(taskID)
		}
		if err != nil {
			return nil, err
		}
		return imageTaskFromMediaTask(task, ""), nil
	}
}

func imageTaskFromMediaTask(task *mediagen.MediaTask, category string) *tools.ImageTaskResult {
	if task == nil {
		return nil
	}
	request := map[string]interface{}{}
	if task.Request != nil {
		request["prompt"] = task.Request.Prompt
		request["negative_prompt"] = task.Request.NegativePrompt
		request["size"] = task.Request.Size
		request["quality"] = task.Request.Quality
		request["style"] = task.Request.Style
		request["n"] = task.Request.N
		if task.Request.Model != "" {
			request["model"] = task.Request.Model
		}
		if task.Request.Extra != nil {
			for key, value := range task.Request.Extra {
				request[key] = value
			}
		}
		if category == "" {
			if len(task.Request.ReferenceURLs) > 0 || len(task.Request.ReferenceImage) > 0 || task.Request.ReferenceURL != "" {
				category = "i2i"
			} else {
				category = "t2i"
			}
		}
	}
	outputs := make([]tools.ImageAsset, 0)
	if task.Response != nil {
		outputs = make([]tools.ImageAsset, 0, len(task.Response.Data))
		for _, item := range task.Response.Data {
			outputs = append(outputs, tools.ImageAsset{
				URL:           item.URL,
				ThumbnailURL:  item.ThumbnailURL,
				RevisedPrompt: item.RevisedPrompt,
				ContentType:   item.ContentType,
				Width:         item.Width,
				Height:        item.Height,
				DurationSec:   item.DurationSec,
			})
		}
	}
	return &tools.ImageTaskResult{
		ID:        task.ID,
		Status:    string(task.Status),
		Type:      string(task.Type),
		Category:  category,
		Provider:  task.Provider,
		Model:     task.Model,
		Progress:  task.Progress,
		Error:     task.Error,
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
		Request:   request,
		Outputs:   outputs,
		Fallback:  imageFallbackFromMediaTask(task),
	}
}

func imageFallbackFromMediaTask(task *mediagen.MediaTask) *tools.MediaFallbackInfo {
	if task == nil || task.FallbackInfo == nil {
		return nil
	}
	return &tools.MediaFallbackInfo{
		Used:        task.FallbackInfo.Used,
		Strategy:    task.FallbackInfo.Strategy,
		DisplayName: task.FallbackInfo.DisplayName,
		SourceURLs:  append([]string(nil), task.FallbackInfo.SourceURLs...),
		SpaceURL:    task.FallbackInfo.SpaceURL,
		Disclosure:  task.FallbackInfo.Disclosure,
		RenderMode:  task.FallbackInfo.RenderMode,
		TemplateID:  task.FallbackInfo.TemplateID,
		StylePreset: task.FallbackInfo.StylePreset,
	}
}

func decodeCompatImageBase64(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if idx := strings.Index(trimmed, ","); idx >= 0 && strings.Contains(trimmed[:idx], ";base64") {
		trimmed = trimmed[idx+1:]
	}
	return base64.StdEncoding.DecodeString(trimmed)
}

func cloneAnyMap(src map[string]interface{}) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
