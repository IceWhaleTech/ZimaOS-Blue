package mediagen

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ChannelNotifier is the callback invoked when a channel-sourced task reaches a terminal state.
// channelName and chatID identify the target channel conversation.
// message is the formatted result text (may include image/video URLs).
type ChannelNotifier func(ctx context.Context, channelName, chatID, message string) error

// ChannelTaskWatcher monitors channel-sourced media tasks and sends results
// back to the originating channel when tasks complete.
type ChannelTaskWatcher struct {
	manager  *Manager
	notifier ChannelNotifier
	interval time.Duration

	mu       sync.Mutex
	watching map[string]struct{} // task IDs currently being watched
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewChannelTaskWatcher creates a watcher that polls channel tasks and notifies on completion.
func NewChannelTaskWatcher(manager *Manager, notifier ChannelNotifier) *ChannelTaskWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ChannelTaskWatcher{
		manager:  manager,
		notifier: notifier,
		interval: 3 * time.Second,
		watching: make(map[string]struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Watch starts watching a channel task. The channelName and chatID are stored
// in the task's metadata for notification routing.
// This is safe to call multiple times for the same task ID (idempotent).
func (w *ChannelTaskWatcher) Watch(taskID, channelName, chatID string) {
	w.mu.Lock()
	if _, ok := w.watching[taskID]; ok {
		w.mu.Unlock()
		return
	}
	w.watching[taskID] = struct{}{}
	w.mu.Unlock()

	w.wg.Add(1)
	go w.pollUntilDone(taskID, channelName, chatID)
}

// SetNotifier sets the channel notification callback.
// Call this after the channel manager is available.
func (w *ChannelTaskWatcher) SetNotifier(notifier ChannelNotifier) {
	w.mu.Lock()
	w.notifier = notifier
	w.mu.Unlock()
}

// RecoverChannelTasks re-watches any non-terminal channel tasks after a restart.
// Call this after Manager.RecoverTasks() so the tasks are already in memory.
func (w *ChannelTaskWatcher) RecoverChannelTasks() {
	if w.manager.taskStore == nil {
		return
	}
	pending, err := w.manager.taskStore.ListPending()
	if err != nil {
		log.Printf("[channel-watcher] failed to list pending tasks: %v", err)
		return
	}
	var count int
	for _, pt := range pending {
		if !isChannelSource(pt.Source) {
			continue
		}
		channelName, chatID := parseChannelSource(pt.Source)
		if channelName == "" {
			continue
		}
		w.Watch(pt.ID, channelName, chatID)
		count++
	}
	if count > 0 {
		log.Printf("[channel-watcher] recovering %d channel tasks", count)
	}
}

// Close stops all watchers and waits for goroutines to finish.
func (w *ChannelTaskWatcher) Close() error {
	w.cancel()
	w.wg.Wait()
	return nil
}

// pollUntilDone polls a task until it reaches a terminal state, then notifies the channel.
func (w *ChannelTaskWatcher) pollUntilDone(taskID, channelName, chatID string) {
	defer w.wg.Done()
	defer func() {
		w.mu.Lock()
		delete(w.watching, taskID)
		w.mu.Unlock()
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Timeout after 10 minutes to avoid leaked goroutines
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-timeout:
			log.Printf("[channel-watcher] task %s timed out after 10m", taskID)
			w.mu.Lock()
			notifier := w.notifier
			w.mu.Unlock()
			if notifier != nil {
				_ = notifier(w.ctx, channelName, chatID, "⏰ Media generation timed out. Please try again.")
			}
			return
		case <-ticker.C:
			task, err := w.manager.GetTask(taskID)
			if err != nil || task == nil {
				continue
			}
			if !task.Status.IsTerminal() {
				continue
			}
			// Task is done — format and notify
			msg := FormatChannelResult(task)
			w.mu.Lock()
			notifier := w.notifier
			w.mu.Unlock()
			if notifier != nil {
				if err := notifier(w.ctx, channelName, chatID, msg); err != nil {
					log.Printf("[channel-watcher] failed to notify channel %s: %v", channelName, err)
				}
			}
			return
		}
	}
}

// FormatChannelResult formats a completed media task for channel display.
func FormatChannelResult(task *MediaTask) string {
	if task == nil {
		return ""
	}

	switch task.Status {
	case TaskStatusFailed:
		errMsg := task.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return fmt.Sprintf("❌ Media generation failed: %s", errMsg)

	case TaskStatusCancelled:
		return "🚫 Media generation was cancelled."

	case TaskStatusSucceeded:
		if task.Response == nil || len(task.Response.Data) == 0 {
			return "✅ Generation complete, but no output was returned."
		}
		var sb strings.Builder
		categoryLabel := "Media"
		switch MediaCategory(task.Category) {
		case CategoryT2I, CategoryI2I:
			categoryLabel = "Image"
		case CategoryT2V, CategoryI2V, CategoryKF2V:
			categoryLabel = "Video"
		}
		sb.WriteString(fmt.Sprintf("✅ %s generated (%s)\n", categoryLabel, task.Model))
		for _, r := range task.Response.Data {
			if r.URL != "" {
				sb.WriteString(r.URL)
				sb.WriteString("\n")
			}
		}
		return strings.TrimSpace(sb.String())

	default:
		return ""
	}
}

// ChannelSource encodes channel routing info into the task source field.
// Format: "channel:<channelName>:<chatID>"
func ChannelSource(channelName, chatID string) string {
	return "channel:" + channelName + ":" + chatID
}

// isChannelSource returns true if the source string indicates a channel task.
func isChannelSource(source string) bool {
	return strings.HasPrefix(source, "channel:")
}

// parseChannelSource extracts channelName and chatID from a channel source string.
func parseChannelSource(source string) (channelName, chatID string) {
	parts := strings.SplitN(source, ":", 3)
	if len(parts) < 2 {
		return "", ""
	}
	channelName = parts[1]
	if len(parts) >= 3 {
		chatID = parts[2]
	}
	return
}
