package mediagen

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
)

// ChannelNotifier is the callback invoked when a channel-sourced task reaches a terminal state.
// channelName identifies the target channel; msg carries text + optional media attachments.
type ChannelNotifier func(ctx context.Context, channelName string, msg channel.OutgoingMessage) error

// URLResolver converts local API paths to externally-accessible URLs.
type URLResolver interface {
	ResolveExternalURL(localPath string) string
}

// watchEntry stores per-task metadata for the watcher.
type watchEntry struct {
	lang i18n.Language
}

// ChannelTaskWatcher monitors channel-sourced media tasks and sends results
// back to the originating channel when tasks complete.
type ChannelTaskWatcher struct {
	manager     *Manager
	notifier    ChannelNotifier
	urlResolver URLResolver
	defaultLang i18n.Language
	interval    time.Duration

	mu       sync.Mutex
	watching map[string]watchEntry // task IDs currently being watched
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

var channelWatcherHTTPClient = network.NewPooledHTTPClient(30 * time.Second)

// NewChannelTaskWatcher creates a watcher that polls channel tasks and notifies on completion.
func NewChannelTaskWatcher(manager *Manager, notifier ChannelNotifier, locale string) *ChannelTaskWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ChannelTaskWatcher{
		manager:     manager,
		notifier:    notifier,
		defaultLang: i18n.ParseLanguage(locale),
		interval:    3 * time.Second,
		watching:    make(map[string]watchEntry),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Watch starts watching a channel task. The channelName and chatID are stored
// in the task's metadata for notification routing.
// lang is the user's locale for i18n (e.g. "zh-CN", "en-US").
// This is safe to call multiple times for the same task ID (idempotent).
func (w *ChannelTaskWatcher) Watch(taskID, channelName, chatID, lang string) {
	w.mu.Lock()
	if _, ok := w.watching[taskID]; ok {
		w.mu.Unlock()
		return
	}
	taskLang := w.defaultLang
	if lang != "" {
		taskLang = i18n.ParseLanguage(lang)
	}
	w.watching[taskID] = watchEntry{lang: taskLang}
	w.mu.Unlock()

	w.wg.Add(1)
	go w.pollUntilDone(taskID, channelName, chatID, taskLang)
}

// SetNotifier sets the channel notification callback.
// Call this after the channel manager is available.
func (w *ChannelTaskWatcher) SetNotifier(notifier ChannelNotifier) {
	w.mu.Lock()
	w.notifier = notifier
	w.mu.Unlock()
}

// SetURLResolver sets the URL resolver for converting local paths to external URLs.
func (w *ChannelTaskWatcher) SetURLResolver(resolver URLResolver) {
	w.mu.Lock()
	w.urlResolver = resolver
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
		// Use default lang for recovered tasks (original locale not persisted)
		w.Watch(pt.ID, channelName, chatID, "")
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
func (w *ChannelTaskWatcher) pollUntilDone(taskID, channelName, chatID string, lang i18n.Language) {
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
				msg := channel.OutgoingMessage{
					ChatID:  chatID,
					Content: i18n.T(lang, i18n.MsgMediaGenTimeout),
				}
				_ = notifier(w.ctx, channelName, msg)
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
			w.mu.Lock()
			resolver := w.urlResolver
			w.mu.Unlock()
			msg := FormatChannelResult(task, lang, w.manager.storage, resolver)
			msg.ChatID = chatID
			w.mu.Lock()
			notifier := w.notifier
			w.mu.Unlock()
			if notifier != nil {
				if err := notifier(w.ctx, channelName, msg); err != nil {
					log.Printf("[channel-watcher] failed to notify channel %s: %v", channelName, err)
				}
			}
			return
		}
	}
}

// FormatChannelResult formats a completed media task as an OutgoingMessage.
// For succeeded tasks, image/video results are sent as attachments.
// If storage is provided, local file data is loaded into att.Data for channels that need binary upload.
// If resolver is provided, local URLs are converted to external URLs for text fallback.
func FormatChannelResult(task *MediaTask, lang i18n.Language, storage *MediaStorage, resolver URLResolver) channel.OutgoingMessage {
	if task == nil {
		return channel.OutgoingMessage{}
	}

	switch task.Status {
	case TaskStatusFailed:
		reason := humanizeError(task.Error, lang)
		return channel.OutgoingMessage{
			Content: i18n.T(lang, i18n.MsgMediaGenFailed, reason),
		}

	case TaskStatusCancelled:
		return channel.OutgoingMessage{
			Content: i18n.T(lang, i18n.MsgMediaGenCancelled),
		}

	case TaskStatusSucceeded:
		if task.Response == nil || len(task.Response.Data) == 0 {
			return channel.OutgoingMessage{
				Content: i18n.T(lang, i18n.MsgMediaGenNoOutput),
			}
		}

		// Determine message key and attachment type from category
		msgKey := i18n.MsgMediaGenerated
		attType := channel.MessageTypeFile
		switch MediaCategory(task.Category) {
		case CategoryT2I, CategoryI2I:
			msgKey = i18n.MsgMediaImageGenerated
			attType = channel.MessageTypeImage
		case CategoryT2V, CategoryI2V, CategoryKF2V:
			msgKey = i18n.MsgMediaVideoGenerated
			attType = channel.MessageTypeVideo
		}

		// Build content text — avoid empty parentheses when model is unknown
		var content string
		if task.Model != "" {
			content = i18n.T(lang, msgKey, task.Model)
		} else {
			content = i18n.T(lang, msgKey, "")
			// Strip empty "()" or "（）" left by fmt.Sprintf
			content = strings.Replace(content, " ()", "", 1)
			content = strings.Replace(content, "（）", "", 1)
			content = strings.TrimSpace(content)
		}
		// Append generation duration
		if task.CompletedAt != nil && !task.CreatedAt.IsZero() {
			dur := task.CompletedAt.Sub(task.CreatedAt)
			if dur > 0 {
				content += "  ⏱ " + i18n.FormatDuration(lang, dur)
			}
		}

		msg := channel.OutgoingMessage{
			Content: content,
		}
		for _, r := range task.Response.Data {
			if r.URL == "" {
				continue
			}
			mime := r.ContentType
			if mime == "" {
				mime = guessMime(attType, r.URL)
			}
			// Resolve external URL for text fallback
			externalURL := r.URL
			if resolver != nil && strings.HasPrefix(r.URL, "/") {
				externalURL = resolver.ResolveExternalURL(r.URL)
			}
			att := channel.Attachment{
				Type:     attType,
				URL:      externalURL,
				MimeType: mime,
			}
			// For video, try to load thumbnail for cover image (needed by Feishu etc.)
			if attType == channel.MessageTypeVideo && r.ThumbnailURL != "" {
				if thumbData, err := downloadURL(r.ThumbnailURL); err == nil {
					att.Thumbnail = thumbData
				}
			}
			msg.Attachments = append(msg.Attachments, att)
		}
		// Pre-load file data from local storage so channels can upload directly
		loadAttachmentData(msg.Attachments, storage)
		// For video attachments without a thumbnail, extract a mid-frame via ffmpeg
		for i := range msg.Attachments {
			att := &msg.Attachments[i]
			if att.Type == channel.MessageTypeVideo && len(att.Thumbnail) == 0 && len(att.Data) > 0 {
				if thumb, err := extractVideoThumbnail(att.Data); err == nil {
					att.Thumbnail = thumb
				}
			}
		}
		return msg

	default:
		return channel.OutgoingMessage{}
	}
}

// loadAttachmentData populates att.Data for each attachment.
// First tries local storage (for URLs starting with "/"), then falls back to HTTP download.
func loadAttachmentData(attachments []channel.Attachment, storage *MediaStorage) {
	for i := range attachments {
		att := &attachments[i]
		if len(att.Data) > 0 {
			continue // already loaded
		}
		// Try local storage first
		if storage != nil && att.URL != "" {
			// Extract the local path portion (strip any external prefix)
			localURL := att.URL
			if strings.HasPrefix(localURL, "http") {
				// External URL — check if it's our own server by looking for the storage base path
				// Skip local loading for truly external URLs
			} else if strings.HasPrefix(localURL, "/") {
				localPath := storage.localPathFromURL(localURL)
				if data, err := os.ReadFile(localPath); err == nil {
					att.Data = data
					att.Size = int64(len(data))
					continue
				} else {
					log.Printf("[channel-watcher] failed to read local file %s: %v", localPath, err)
				}
			}
		}
		// Fallback: download from URL (works for remote URLs)
		if len(att.Data) == 0 && att.URL != "" && strings.HasPrefix(att.URL, "http") {
			if data, err := downloadURL(att.URL); err == nil {
				att.Data = data
				att.Size = int64(len(data))
			} else {
				log.Printf("[channel-watcher] failed to download %s: %v", att.URL, err)
			}
		}
	}
}

// downloadURL fetches a URL and returns the response body.
func downloadURL(url string) ([]byte, error) {
	resp, err := channelWatcherHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// extractVideoThumbnail extracts a JPEG frame from the middle of a video using ffmpeg.
// Returns nil, err if ffmpeg is not available or extraction fails.
func extractVideoThumbnail(videoData []byte) ([]byte, error) {
	// Write video to temp file
	tmpVideo, err := os.CreateTemp("", "thumb-*.mp4")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpVideo.Name())
	if _, err := tmpVideo.Write(videoData); err != nil {
		tmpVideo.Close()
		return nil, err
	}
	tmpVideo.Close()

	// Probe duration with ffprobe
	dur := probeDuration(tmpVideo.Name())
	seekTo := "0"
	if dur > 0 {
		mid := dur / 2
		seekTo = fmt.Sprintf("%.2f", mid)
	}

	// Extract single frame at midpoint
	tmpOut, err := os.CreateTemp("", "thumb-*.jpg")
	if err != nil {
		return nil, err
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", seekTo,
		"-i", tmpVideo.Name(),
		"-frames:v", "1",
		"-q:v", "5",
		"-y", tmpOut.Name(),
	)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	return os.ReadFile(tmpOut.Name())
}

// probeDuration returns the video duration in seconds using ffprobe, or 0 on failure.
func probeDuration(path string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	var d float64
	fmt.Sscanf(s, "%f", &d)
	return d
}

// guessMime returns a reasonable MIME type based on attachment type and URL extension.
func guessMime(t channel.MessageType, url string) string {
	switch t {
	case channel.MessageTypeImage:
		if strings.HasSuffix(url, ".webp") {
			return "image/webp"
		}
		return "image/png"
	case channel.MessageTypeVideo:
		return "video/mp4"
	default:
		return "application/octet-stream"
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

// humanizeError maps raw upstream error strings to translated, user-friendly messages.
func humanizeError(rawErr string, lang i18n.Language) string {
	if rawErr == "" {
		return i18n.T(lang, i18n.MsgErrUnknown)
	}
	lower := strings.ToLower(rawErr)
	switch {
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "throttl") || strings.Contains(lower, "too many"):
		return i18n.T(lang, i18n.MsgErrRateLimit)
	case strings.Contains(lower, "content") && (strings.Contains(lower, "block") || strings.Contains(lower, "filter") || strings.Contains(lower, "safety") || strings.Contains(lower, "policy")):
		return i18n.T(lang, i18n.MsgErrContentBlock)
	case strings.Contains(lower, "api") && strings.Contains(lower, "fail"),
		strings.Contains(lower, "external") && strings.Contains(lower, "fail"),
		strings.Contains(lower, "upstream"),
		strings.Contains(lower, "unavailable"),
		strings.Contains(lower, "502"), strings.Contains(lower, "503"), strings.Contains(lower, "504"):
		return i18n.T(lang, i18n.MsgErrAPIFailed)
	default:
		return i18n.T(lang, i18n.MsgErrUnknown)
	}
}
