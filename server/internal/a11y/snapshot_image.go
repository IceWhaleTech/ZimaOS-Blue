package a11y

import (
	"context"
	"strings"
)

func attachSnapshotImage(ctx context.Context, result SnapshotResult, capture func(context.Context, string) (ScreenshotResult, error)) SnapshotResult {
	if capture == nil {
		return result
	}
	windowID := strings.TrimSpace(result.WindowID)
	if windowID == "" {
		return result
	}
	screenshot, err := capture(ctx, windowID)
	if err != nil {
		return result
	}
	if imagePath := strings.TrimSpace(screenshot.ImagePath); imagePath != "" {
		result.ImagePath = imagePath
	}
	return result
}
