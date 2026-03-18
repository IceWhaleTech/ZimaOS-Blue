package bootstrap

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newImageGenerateAdapter(manager *mediagen.Manager) tools.ImageGenerateFunc {
	if manager == nil {
		return nil
	}
	return func(ctx context.Context, req tools.ImageGenerateRequest) (*tools.ImageTaskResult, error) {
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
			waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
			defer cancel()
			waitedTask, waitErr := manager.WaitForTask(waitCtx, task.ID)
			if waitedTask != nil {
				task = waitedTask
			}
			result := imageTaskFromMediaTask(task, req.Category)
			if waitErr != nil {
				result.Message = waitErr.Error()
				if result.Status == "" {
					result.Status = "processing"
				}
				return result, nil
			}
			return result, nil
		}
		return imageTaskFromMediaTask(task, req.Category), nil
	}
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
