package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type ConvertTool struct {
	service   *convertpkg.Service
	approvals *ApprovalManager
}

func NewConvertTool(service *convertpkg.Service, approvals *ApprovalManager) *ConvertTool {
	return &ConvertTool{service: service, approvals: approvals}
}

func RegisterConvertTool(registry *Registry, service *convertpkg.Service, approvals *ApprovalManager) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewConvertTool(service, approvals))
}

func (t *ConvertTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "convert",
		Description: "Native conversion tool with host capability detection for documents, images, PDF, audio, video, TTS, and file-level ASR. Returns task-based results that can be polled and reused with att:/out: references.",
		Icon:        "wand-sparkles",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []string{"convert", "merge", "split", "trim", "extract_audio", "extract_frames", "tts", "asr", "status", "list", "cancel", "capabilities"},
				},
				"sources": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Only att:<id>, out:<task_id>:<output_id>, or absolute local paths are allowed.",
				},
				"target_format": map[string]interface{}{"type": "string"},
				"text":          map[string]interface{}{"type": "string"},
				"task_id":       map[string]interface{}{"type": "string"},
				"wait_ms":       map[string]interface{}{"type": "integer"},
				"options": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t *ConvertTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("convert service not available")
	}
	action := strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))
	userID := strings.TrimSpace(GetUserID(ctx))
	conversationID := strings.TrimSpace(GetSessionID(ctx))
	switch action {
	case "capabilities":
		return t.service.Capabilities(ctx), nil
	case "list":
		limit := compatInt(args, "limit", "max_results", "n")
		return t.service.ListTasks(ctx, userID, conversationID, limit)
	case "status":
		taskID := strings.TrimSpace(firstCompatString(args, "task_id", "taskId", "id"))
		if taskID == "" {
			return nil, errors.New("task_id is required for status")
		}
		return t.service.GetTask(ctx, userID, conversationID, taskID)
	case "cancel":
		taskID := strings.TrimSpace(firstCompatString(args, "task_id", "taskId", "id"))
		if taskID == "" {
			return nil, errors.New("task_id is required for cancel")
		}
		task, err := t.service.CancelTask(ctx, userID, conversationID, taskID)
		if err == nil {
			EmitCard(ctx, convertpkg.CardData(task))
		}
		return task, err
	}

	req, err := parseConvertTaskRequest(args)
	if err != nil {
		return nil, err
	}
	if err := t.requestLocalPathApproval(ctx, req.Sources); err != nil {
		return nil, err
	}
	task, err := t.service.Submit(ctx, userID, conversationID, req)
	if err != nil {
		return nil, err
	}
	EmitCard(ctx, convertpkg.CardData(task))
	if req.WaitMS > 0 {
		wait := time.Duration(req.WaitMS) * time.Millisecond
		if wait > 30*time.Second {
			wait = 30 * time.Second
		}
		if waited, waitErr := t.service.WaitForTask(ctx, userID, conversationID, task.ID, wait); waitErr == nil && waited != nil {
			task = waited
		}
	}
	return task, nil
}

func (t *ConvertTool) requestLocalPathApproval(ctx context.Context, sources []string) error {
	if len(sources) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(sources))
	for _, source := range sources {
		trimmed := strings.TrimSpace(source)
		if trimmed == "" || strings.HasPrefix(trimmed, "att:") || strings.HasPrefix(trimmed, "out:") {
			continue
		}
		if !filepath.IsAbs(trimmed) {
			return fmt.Errorf("source must be att:<id>, out:<task_id>:<output_id>, or absolute path")
		}
		cleanPath, err := filepath.Abs(trimmed)
		if err != nil {
			return err
		}
		if t.service.IsManagedPath(cleanPath) {
			continue
		}
		info, err := os.Stat(cleanPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && info.IsDir() {
			dirs = append(dirs, cleanPath)
		} else {
			dirs = append(dirs, filepath.Dir(cleanPath))
		}
	}
	dirs = sortedUniqueStrings(dirs)
	if len(dirs) == 0 {
		return nil
	}
	if t.approvals == nil {
		return errors.New("local path approval is unavailable")
	}
	userID := strings.TrimSpace(GetUserID(ctx))
	for _, dir := range dirs {
		decision, err := t.approvals.RequestApproval(ctx, ApprovalRequest{
			Type:      "directory",
			Directory: dir,
			Security:  "The convert tool wants to read a local file outside the managed conversation workspace.",
			UserID:    userID,
		})
		if err != nil {
			return err
		}
		if decision != ApprovalAllowOnce && decision != ApprovalAllowAlways {
			return fmt.Errorf("access denied for local path: %s", dir)
		}
	}
	return nil
}

func parseConvertTaskRequest(args map[string]interface{}) (convertpkg.TaskRequest, error) {
	var req convertpkg.TaskRequest
	req.Action = strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))
	req.TargetFormat = strings.TrimSpace(firstCompatString(args, "target_format", "targetFormat", "format"))
	req.Text = strings.TrimSpace(firstCompatString(args, "text", "input", "content", "message"))
	req.TaskID = strings.TrimSpace(firstCompatString(args, "task_id", "taskId", "id"))
	req.WaitMS = compatInt(args, "wait_ms", "waitMs")
	if rawSources, ok := compatArgValue(args, "sources"); ok {
		sources, err := toStringSlice(rawSources)
		if err != nil {
			return req, err
		}
		req.Sources = sources
	}
	if rawOptions, ok := compatArgValue(args, "options"); ok {
		if optMap, ok := coerceCompatMap(rawOptions); ok {
			req.Options = parseOptions(optMap)
		}
	}
	if req.Action == convertpkg.ActionTTS {
		req.TargetFormat = strings.TrimSpace(nonEmpty(req.Options.Speech.TTS.Format, req.TargetFormat))
	}
	return req, nil
}

func parseOptions(raw map[string]interface{}) convertpkg.TaskOptions {
	var out convertpkg.TaskOptions
	if doc, ok := raw["document"].(map[string]interface{}); ok {
		if pages, ok := doc["pages"]; ok {
			out.Document.Pages = toIntSlice(pages)
		}
	}
	if image, ok := raw["image"].(map[string]interface{}); ok {
		out.Image.Quality = compatInt(image, "quality")
		out.Image.Width = compatInt(image, "width")
		out.Image.Height = compatInt(image, "height")
	}
	if audio, ok := raw["audio"].(map[string]interface{}); ok {
		out.Audio.BitRate = compatInt(audio, "bit_rate", "bitrate")
		out.Audio.SampleRate = compatInt(audio, "sample_rate")
		out.Audio.Channels = compatInt(audio, "channels")
		out.Audio.Codec = strings.TrimSpace(fmt.Sprintf("%v", audio["codec"]))
	}
	if video, ok := raw["video"].(map[string]interface{}); ok {
		out.Video.Codec = strings.TrimSpace(fmt.Sprintf("%v", video["codec"]))
		out.Video.Width = compatInt(video, "width")
		out.Video.Height = compatInt(video, "height")
		out.Video.FrameRate = compatInt(video, "frame_rate", "fps")
		out.Video.BitRate = compatInt(video, "bit_rate", "bitrate")
		out.Video.StartMS = int64(compatInt(video, "start_ms", "startMs"))
		out.Video.EndMS = int64(compatInt(video, "end_ms", "endMs"))
		out.Video.FrameIntervalMS = int64(compatInt(video, "frame_interval_ms", "frameIntervalMs", "interval_ms", "intervalMs"))
		out.Video.Segments = parseSegments(video["segments"])
	}
	if speech, ok := raw["speech"].(map[string]interface{}); ok {
		if tts, ok := speech["tts"].(map[string]interface{}); ok {
			out.Speech.TTS.Voice = strings.TrimSpace(fmt.Sprintf("%v", tts["voice"]))
			out.Speech.TTS.Rate = compatInt(tts, "rate")
			out.Speech.TTS.Format = strings.TrimSpace(fmt.Sprintf("%v", tts["format"]))
		}
		if asr, ok := speech["asr"].(map[string]interface{}); ok {
			out.Speech.ASR.Language = strings.TrimSpace(fmt.Sprintf("%v", asr["language"]))
			if raw, ok := compatArgValue(asr, "on_device_only", "onDeviceOnly"); ok {
				out.Speech.ASR.OnDeviceOnly = compatBool(raw)
			}
		}
	}
	return out
}

func parseSegments(raw interface{}) []convertpkg.TimeSegment {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	segments := make([]convertpkg.TimeSegment, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		segments = append(segments, convertpkg.TimeSegment{
			StartMS: int64(compatInt(m, "start_ms", "startMs")),
			EndMS:   int64(compatInt(m, "end_ms", "endMs")),
		})
	}
	return segments
}

func toStringSlice(raw interface{}) ([]string, error) {
	switch typed := raw.(type) {
	case []string:
		return typed, nil
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			value := strings.TrimSpace(fmt.Sprintf("%v", item))
			if value != "" {
				out = append(out, value)
			}
		}
		return out, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("sources must be an array of strings")
	}
}

func toIntSlice(raw interface{}) []int {
	switch typed := raw.(type) {
	case []interface{}:
		out := make([]int, 0, len(typed))
		for _, item := range typed {
			if n, ok := coerceCompatInt(item); ok {
				out = append(out, n)
			}
		}
		return out
	case []int:
		return typed
	default:
		return nil
	}
}

func firstNonEmptyString(values ...interface{}) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(fmt.Sprintf("%v", value))
		if trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return ""
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sortedUniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
