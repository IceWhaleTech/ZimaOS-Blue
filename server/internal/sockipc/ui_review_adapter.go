package sockipc

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// UIReviewIPCAdapter adapts tools.UIReviewerTool to the sockipc.UIReviewBackend interface.
type UIReviewIPCAdapter struct {
	tool *tools.UIReviewerTool
}

// NewUIReviewIPCAdapter creates a new adapter.
func NewUIReviewIPCAdapter(tool *tools.UIReviewerTool) *UIReviewIPCAdapter {
	return &UIReviewIPCAdapter{tool: tool}
}

func (a *UIReviewIPCAdapter) ReviewURL(ctx context.Context, url, lang, device string) (string, error) {
	args := map[string]interface{}{
		"action": "review_url",
		"url":    url,
	}
	if lang != "" {
		args["lang"] = lang
	}
	if device != "" {
		args["device"] = device
	}
	result, err := a.tool.Execute(ctx, args)
	if err != nil {
		return "", err
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return "", nil
}

func (a *UIReviewIPCAdapter) ReviewImage(ctx context.Context, imageBase64, lang string) (string, error) {
	args := map[string]interface{}{
		"action": "review_image",
		"image":  imageBase64,
	}
	if lang != "" {
		args["lang"] = lang
	}
	result, err := a.tool.Execute(ctx, args)
	if err != nil {
		return "", err
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return "", nil
}

func (a *UIReviewIPCAdapter) CheckAccessibility(ctx context.Context, url, lang string) (string, error) {
	args := map[string]interface{}{
		"action": "check_accessibility",
		"url":    url,
	}
	if lang != "" {
		args["lang"] = lang
	}
	result, err := a.tool.Execute(ctx, args)
	if err != nil {
		return "", err
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return "", nil
}
