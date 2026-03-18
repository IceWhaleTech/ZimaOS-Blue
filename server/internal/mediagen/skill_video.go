package mediagen

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// VideoGenerateSkill implements tools.Tool for LLM-driven video generation.
// Unlike image generation, video gen returns a task ID immediately since it takes 30-60s.
type VideoGenerateSkill struct {
	manager *Manager
}

// availableVideoModels returns a comma-separated list of video model IDs
// from currently registered providers.
func (t *VideoGenerateSkill) availableVideoModels() string {
	models := t.manager.Models()
	var names []string
	for _, m := range models {
		if m.Type == MediaTypeVideo {
			names = append(names, m.ID)
		}
	}
	if len(names) == 0 {
		return "wan2.6-t2v, wan2.6-i2v, wan2-spark-t2v, midjourney-video"
	}
	return strings.Join(names, ", ")
}

// NewVideoGenerateSkill creates a new video generation tool.
func NewVideoGenerateSkill(manager *Manager) *VideoGenerateSkill {
	return &VideoGenerateSkill{manager: manager}
}

// Definition returns the tool definition for the LLM.
func (t *VideoGenerateSkill) Definition() tools.ToolDefinition {
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
					"description": "Model to use. Available: " + t.availableVideoModels(),
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
func (t *VideoGenerateSkill) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	req := &MediaRequest{Type: MediaTypeVideo}

	if v := skillCompatString(args, "prompt"); v != "" {
		req.Prompt = v
	} else {
		return map[string]interface{}{
			"status":  "error",
			"message": "prompt is required",
		}, nil
	}
	if v := skillCompatString(args, "model"); v != "" {
		req.Model = v
	}
	if v := skillCompatInt(args, "duration"); v > 0 {
		req.Duration = v
	}
	if v := skillCompatString(args, "size"); v != "" {
		req.Size = v
	}

	// Default model for video
	if req.Model == "" {
		req.Model = "wan2.6-t2v"
	}

	task, err := t.manager.Generate(ctx, req)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to start video generation: %s. Please check that a media provider is configured and enabled.", err.Error()),
		}, nil
	}

	return map[string]interface{}{
		"task_id":       task.ID,
		"status":        string(task.Status),
		"message":       fallbackSkillTaskMessage(task.ID, task.FallbackInfo),
		"fallback_info": task.FallbackInfo,
	}, nil
}

var _ tools.Tool = (*VideoGenerateSkill)(nil)
