package mediagen

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ImageGenerateTool implements tools.Tool for LLM-driven image generation.
type ImageGenerateTool struct {
	manager *Manager
}

// availableImageModels returns a comma-separated list of image model IDs
// from currently registered providers. Falls back to a static list.
func (t *ImageGenerateTool) availableImageModels() string {
	models := t.manager.Models()
	var names []string
	for _, m := range models {
		if m.Type == MediaTypeImage {
			names = append(names, m.ID)
		}
	}
	if len(names) == 0 {
		return "dall-e-3, nano-banana-pro, qwen-image-max, midjourney, gemini-2.0-flash-exp-image-generation"
	}
	return strings.Join(names, ", ")
}

// NewImageGenerateTool creates a new image generation tool.
func NewImageGenerateTool(manager *Manager) *ImageGenerateTool {
	return &ImageGenerateTool{manager: manager}
}

// Definition returns the tool definition for the LLM.
func (t *ImageGenerateTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "image_generate",
		Description: "Generate images from text descriptions. Default model: nano-banana-pro (recommended). IMPORTANT: Image generation takes 15-60 seconds — do NOT call this tool multiple times for the same request. If the result shows status 'processing', tell the user to wait. Returns URLs of generated images saved to the media gallery.",
		Icon:        "image",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Text description of the image to generate",
				},
				"negative_prompt": map[string]interface{}{
					"type":        "string",
					"description": "What to avoid in the image (optional)",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model to use (default: nano-banana-pro). Available: " + t.availableImageModels(),
				},
				"size": map[string]interface{}{
					"type":        "string",
					"description": "Image size (e.g., '1024x1024', '1024x1792', '1792x1024'). For nano-banana-pro, this is auto-converted to aspect_ratio.",
				},
				"aspect_ratio": map[string]interface{}{
					"type":        "string",
					"description": "Aspect ratio for nano-banana-pro (e.g., '1:1', '16:9', '9:16', '4:3', '3:4', '5:4', '4:5'). Takes precedence over size.",
				},
				"resolution": map[string]interface{}{
					"type":        "string",
					"description": "Resolution for nano-banana-pro: '1K', '2K', or '4K' (default: '2K')",
				},
				"n": map[string]interface{}{
					"type":        "number",
					"description": "Number of images to generate (1-4, default 1)",
				},
				"quality": map[string]interface{}{
					"type":        "string",
					"description": "Quality level: 'standard' or 'hd'",
				},
				"style": map[string]interface{}{
					"type":        "string",
					"description": "Style: 'natural' or 'vivid'",
				},
			},
			"required": []string{"prompt"},
		},
	}
}

// Execute generates images and blocks until done.
func (t *ImageGenerateTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	req := &MediaRequest{Type: MediaTypeImage}

	if v, ok := args["prompt"].(string); ok {
		req.Prompt = v
	} else {
		return map[string]interface{}{
			"status":  "error",
			"message": "prompt is required",
		}, nil
	}
	if v, ok := args["negative_prompt"].(string); ok {
		req.NegativePrompt = v
	}
	if v, ok := args["model"].(string); ok {
		req.Model = v
	}
	if v, ok := args["size"].(string); ok {
		req.Size = v
	}
	if v, ok := args["n"].(float64); ok {
		req.N = int(v)
	}
	if v, ok := args["quality"].(string); ok {
		req.Quality = v
	}
	if v, ok := args["style"].(string); ok {
		req.Style = v
	}
	if v, ok := args["aspect_ratio"].(string); ok {
		if req.Extra == nil {
			req.Extra = make(map[string]any)
		}
		req.Extra["aspect_ratio"] = v
	}
	if v, ok := args["resolution"].(string); ok {
		if req.Extra == nil {
			req.Extra = make(map[string]any)
		}
		req.Extra["resolution"] = v
	}

	start := time.Now()
	task, err := t.manager.Generate(ctx, req)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to start image generation: %s. Please check that a media provider is configured and enabled.", err.Error()),
		}, nil
	}

	// Wait for completion — image generation typically takes 15-60s.
	// Use a generous timeout to avoid premature cancellation.
	waitCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	task, err = t.manager.WaitForTask(waitCtx, task.ID)
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		taskID := ""
		if task != nil {
			taskID = task.ID
		}
		return map[string]interface{}{
			"status":     "failed",
			"message":    fmt.Sprintf("Image generation failed after %s: %s. The task may still be running in the background.", elapsed, err.Error()),
			"task_id":    taskID,
			"elapsed_ms": elapsed.Milliseconds(),
		}, nil
	}

	// WaitForTask may return a still-processing task on context timeout (nil error).
	// Tell the LLM to inform the user and NOT retry.
	if task.Status != TaskStatusSucceeded {
		return map[string]interface{}{
			"status":     "processing",
			"message":    fmt.Sprintf("Image generation is still in progress (waited %s). This is normal — it can take 30-120 seconds. The image will appear automatically when ready. Do NOT call image_generate again for this request.", elapsed),
			"task_id":    task.ID,
			"elapsed_ms": elapsed.Milliseconds(),
		}, nil
	}

	// Build response with image URLs
	var images []map[string]interface{}
	if task.Response != nil {
		for _, r := range task.Response.Data {
			img := map[string]interface{}{
				"url": r.URL,
			}
			if r.ThumbnailURL != "" {
				img["thumbnail_url"] = r.ThumbnailURL
			}
			if r.RevisedPrompt != "" {
				img["revised_prompt"] = r.RevisedPrompt
			}
			images = append(images, img)
		}
	}

	return map[string]interface{}{
		"status":     "success",
		"images":     images,
		"elapsed_ms": elapsed.Milliseconds(),
	}, nil
}

var _ tools.Tool = (*ImageGenerateTool)(nil)
