package tools

import "context"

// PPTTool exposes PPT slide-asset image orchestration.
type PPTTool struct {
	service PPTGenerateService
}

func NewPPTTool(service PPTGenerateService) *PPTTool {
	return &PPTTool{service: service}
}

func (t *PPTTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "ppt",
		Description: "Generate PPT-ready slide visuals and background assets using the banana_slides preset with optional reference images and one-pass automatic review/retry.",
		Icon:        "image",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description":         map[string]interface{}{"type": "string", "description": "What this PPT slide visual should communicate."},
				"aspect_ratio":        map[string]interface{}{"type": "string", "description": "Target aspect ratio such as 16:9."},
				"reference_images":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional reference images for style matching."},
				"style_preset":        map[string]interface{}{"type": "string", "description": "Optional preset name. Default banana_slides."},
				"quality_profile":     map[string]interface{}{"type": "string", "description": "Optional review profile. Default ppt."},
				"review_threshold":    map[string]interface{}{"type": "number", "description": "Automatic review threshold. Default 80."},
				"review_retry_budget": map[string]interface{}{"type": "integer", "description": "Automatic repair retry budget. Default 1."},
				"style_theme":         map[string]interface{}{"type": "string", "description": "Optional theme or brand guidance."},
				"source":              map[string]interface{}{"type": "string", "description": "Optional caller/source marker such as ppt or slides."},
				"lang":                map[string]interface{}{"type": "string", "description": "Optional locale for review feedback."},
			},
			"required": []string{"description"},
		},
	}
}

func (t *PPTTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return executePPTService(ctx, args, t.service)
}

func RegisterPPTTool(registry *Registry, service PPTGenerateService) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewPPTTool(service))
}
