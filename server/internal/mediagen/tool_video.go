package mediagen

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// VideoGenerateTool implements tools.Tool for LLM-driven video generation.
// Unlike image generation, video gen returns a task ID immediately since it takes 30-60s.
type VideoGenerateTool struct {
	manager *Manager
}

// NewVideoGenerateTool creates a new video generation tool.
func NewVideoGenerateTool(manager *Manager) *VideoGenerateTool {
	return &VideoGenerateTool{manager: manager}
}

// Definition returns the tool definition for the LLM.
func (t *VideoGenerateTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "video_generate",
		Description: "Generate videos from text descriptions. Returns a task ID for tracking progress since video generation takes 30-60 seconds. The user can check status at /api/media/tasks/{task_id}.",
		Icon:        "video",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Text description of the video to generate",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model to use (e.g., 'mr-wan2.6-t2v' for text-to-video)",
				},
				"duration": map[string]interface{}{
					"type":        "number",
					"description": "Video duration in seconds (default: 5)",
				},
				"size": map[string]interface{}{
					"type":        "string",
					"description": "Video resolution (e.g., '1280x720')",
				},
			},
			"required": []string{"prompt"},
		},
	}
}

// Execute starts video generation and returns a task ID immediately.
func (t *VideoGenerateTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	req := &MediaRequest{Type: MediaTypeVideo}

	if v, ok := args["prompt"].(string); ok {
		req.Prompt = v
	} else {
		return nil, fmt.Errorf("prompt is required")
	}
	if v, ok := args["model"].(string); ok {
		req.Model = v
	}
	if v, ok := args["duration"].(float64); ok {
		req.Duration = int(v)
	}
	if v, ok := args["size"].(string); ok {
		req.Size = v
	}

	// Default model for video
	if req.Model == "" {
		req.Model = "mr-wan2.6-t2v"
	}

	task, err := t.manager.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task_id": task.ID,
		"status":  string(task.Status),
		"message": "Video generation started. Check progress at /api/media/tasks/" + task.ID,
	}, nil
}

var _ tools.Tool = (*VideoGenerateTool)(nil)
