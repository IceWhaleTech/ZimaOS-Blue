package a11y

import (
	"context"
	"strings"
)

func enrichSnapshotErrorWithImage(ctx context.Context, err error, windowID string, capture func(context.Context, string) (ScreenshotResult, error)) error {
	if err == nil {
		return nil
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok || runtimeErr == nil {
		runtimeErr = NewError("backend_unavailable", err.Error(), nil)
	}
	details := make(map[string]interface{}, len(runtimeErr.Details)+2)
	for key, value := range runtimeErr.Details {
		details[key] = value
	}
	windowID = strings.TrimSpace(windowID)
	if windowID != "" {
		details["window_id"] = windowID
	}
	if capture != nil && windowID != "" {
		if screenshot, captureErr := capture(ctx, windowID); captureErr == nil {
			if imagePath := strings.TrimSpace(screenshot.ImagePath); imagePath != "" {
				details["image_path"] = imagePath
			}
		}
	}
	return NewError(runtimeErr.Code, runtimeErr.Message, details)
}
