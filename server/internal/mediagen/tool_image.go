package mediagen

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ImageGenerateTool implements tools.Tool for LLM-driven image generation.
type ImageGenerateTool struct {
	manager *Manager
}

// NewImageGenerateTool creates a new image generation tool.
func NewImageGenerateTool(manager *Manager) *ImageGenerateTool {
	return &ImageGenerateTool{manager: manager}
}

// Definition returns the tool definition for the LLM.
func (t *ImageGenerateTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "image_generate",
		Description: "Generate images from text descriptions. Supports multiple providers: Gemini, Qwen Image (DashScope), DALL-E (MuleRouter). Returns URLs of generated images.",
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
					"description": "Model to use. Options: gemini-2.0-flash-exp-image-generation, qwen-image-max, dall-e-3, wanx-v1",
				},
				"size": map[string]interface{}{
					"type":        "string",
					"description": "Image size (e.g., '1024x1024', '1024x1792', '1792x1024')",
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
		return nil, fmt.Errorf("prompt is required")
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

	task, err := t.manager.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	// Wait for completion (with timeout)
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	task, err = t.manager.WaitForTask(waitCtx, task.ID)
	if err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"task_id": task.ID,
			"status":  string(task.Status),
		}, nil
	}

	// Build response with image URLs
	var images []map[string]interface{}
	if task.Response != nil {
		for _, r := range task.Response.Data {
			img := map[string]interface{}{
				"url": r.URL,
			}
			if r.RevisedPrompt != "" {
				img["revised_prompt"] = r.RevisedPrompt
			}
			images = append(images, img)
		}
	}

	return map[string]interface{}{
		"images": images,
	}, nil
}

var _ tools.Tool = (*ImageGenerateTool)(nil)
