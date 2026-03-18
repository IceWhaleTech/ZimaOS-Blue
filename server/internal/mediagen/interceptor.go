package mediagen

import (
	"context"
	"fmt"
)

// Interceptor implements the server.MediaInterceptor interface.
// It classifies media intent using the IR classifier and creates tasks via the Manager.
type Interceptor struct {
	manager *Manager
	watcher *ChannelTaskWatcher
}

// NewInterceptor creates a media interceptor backed by the given manager and watcher.
func NewInterceptor(manager *Manager, watcher *ChannelTaskWatcher) *Interceptor {
	return &Interceptor{manager: manager, watcher: watcher}
}

// ClassifyAndGenerate checks if the message is a media generation request.
// If so, it creates a task and starts watching it for channel notification.
// Returns (taskID, true, nil) if media intent detected, ("", false, nil) otherwise.
func (i *Interceptor) ClassifyAndGenerate(ctx context.Context, message string, hasImages bool, imageCount int, locale string, source string) (string, bool, error) {
	// Skip classification entirely when neither a real provider nor fallback models are available.
	if !i.manager.HasAvailableModels() {
		return "", false, nil
	}

	intent := ClassifyMediaIntent(message, hasImages, imageCount, locale)
	if intent == nil || intent.Category == CategoryNone || intent.Confidence < 0.7 {
		return "", false, nil
	}

	// Build a media request from the classified intent
	req := &MediaRequest{
		Prompt: intent.Prompt,
	}
	switch intent.Category {
	case CategoryT2I, CategoryI2I:
		req.Type = MediaTypeImage
	case CategoryT2V, CategoryI2V, CategoryKF2V:
		req.Type = MediaTypeVideo
	default:
		return "", false, fmt.Errorf("unknown category: %s", intent.Category)
	}

	// Create a persistent task
	task, err := i.manager.CreateTask(ctx, req, "", string(intent.Category), source)
	if err != nil {
		return "", true, fmt.Errorf("failed to create media task: %w", err)
	}

	// Start watching for channel notification
	if i.watcher != nil && isChannelSource(source) {
		channelName, chatID := parseChannelSource(source)
		i.watcher.Watch(task.ID, channelName, chatID, locale)
	}

	return task.ID, true, nil
}
