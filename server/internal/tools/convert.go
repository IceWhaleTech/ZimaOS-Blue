package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

type ConvertTool struct {
	service   *convertpkg.Service
	approvals *ApprovalManager
	dirStore  *DirAllowlistStore
	scope     *fsToolScope
}

type parsedConvertTaskRequest struct {
	TaskRequest convertpkg.TaskRequest
	SimpleMode  bool
}

const (
	defaultConvertSyncWait      = 5 * time.Minute
	maxConvertWait              = 300 * time.Second
	largeDocumentAsyncThreshold = 8 << 20
	largePDFAsyncThreshold      = 12 << 20
	largeMediaAsyncThreshold    = 64 << 20
)

func NewConvertTool(service *convertpkg.Service, approvals *ApprovalManager, dirStore *DirAllowlistStore, allowedPaths []string) *ConvertTool {
	scope := newFSToolScope(allowedPaths)
	scope = scope.withApprovalFlow(approvals, dirStore)
	return &ConvertTool{service: service, approvals: approvals, dirStore: dirStore, scope: scope}
}

func RegisterConvertTool(registry *Registry, service *convertpkg.Service, approvals *ApprovalManager, dirStore *DirAllowlistStore, allowedPaths []string) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewConvertTool(service, approvals, dirStore, allowedPaths))
}

func (t *ConvertTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "convert",
		Description: "Convert local files, attachments, and prior outputs. Preferred form: input_path + output_path using relative paths. Normal single-file jobs return synchronously; only heavier jobs return async=true with a task_id for polling.",
		Icon:        "wand-sparkles",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Optional. Omit for normal conversions: input_path/output_path defaults to convert, and task_id alone defaults to status.",
					"enum":        []string{"convert", "merge", "split", "trim", "extract_audio", "extract_frames", "tts", "asr", "status", "list", "cancel", "capabilities"},
				},
				"input_path": map[string]interface{}{
					"type":        "string",
					"description": "Preferred source file path for simple conversions. Relative paths resolve from the workspace root; absolute local paths are allowed with approval.",
				},
				"output_path": map[string]interface{}{
					"type":        "string",
					"description": "Preferred destination file path for simple conversions. Relative paths resolve from the workspace root; the output format is inferred from the file extension when possible.",
				},
				"sources": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Legacy multi-source input. Supports att:<id>, out:<task_id>:<output_id>, relative workspace paths, and absolute local paths.",
				},
				"target_format": map[string]interface{}{
					"type":        "string",
					"description": "Optional legacy format override. Usually omit this and let output_path's extension decide the format.",
				},
				"text": map[string]interface{}{"type": "string"},
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "Use only to check/cancel an async task, or after convert returned async=true.",
				},
				"wait_ms": map[string]interface{}{"type": "integer"},
				"options": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
	}
}

func (t *ConvertTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("convert service not available")
	}
	action := normalizeConvertAction(args)
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

	parsed, err := parseConvertTaskRequest(args)
	if err != nil {
		return nil, err
	}
	req, err := t.resolveTaskPaths(ctx, parsed.TaskRequest)
	if err != nil {
		return nil, err
	}
	task, err := t.service.Submit(ctx, userID, conversationID, req)
	if err != nil {
		return nil, err
	}
	EmitCard(ctx, convertpkg.CardData(task))

	if parsed.SimpleMode {
		if t.shouldReturnAsyncImmediately(req) {
			return simpleConvertTaskResult(task, true), nil
		}
		wait := requestedConvertWait(req.WaitMS)
		if wait == 0 {
			wait = defaultConvertSyncWait
		}
		waited, waitErr := t.service.WaitForTask(ctx, userID, conversationID, task.ID, wait)
		if waitErr != nil {
			return nil, waitErr
		}
		if waited != nil {
			task = waited
			EmitCard(ctx, convertpkg.CardData(task))
		}
		return simpleConvertTaskResult(task, task != nil && !task.Status.IsTerminal()), nil
	}

	if wait := requestedConvertWait(req.WaitMS); wait > 0 {
		if waited, waitErr := t.service.WaitForTask(ctx, userID, conversationID, task.ID, wait); waitErr == nil && waited != nil {
			task = waited
			EmitCard(ctx, convertpkg.CardData(task))
		}
	}
	return task, nil
}

func (t *ConvertTool) pathScope() *fsToolScope {
	if t != nil && t.scope != nil {
		return t.scope
	}
	return newFSToolScope(nil).withApprovalFlow(t.approvals, t.dirStore)
}

func normalizeConvertAction(args map[string]interface{}) string {
	action := strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))
	if action != "" {
		return action
	}
	if strings.TrimSpace(firstCompatString(args, "task_id", "taskId", "id")) != "" {
		return "status"
	}
	if hasSimpleConvertPaths(args) {
		return convertpkg.ActionConvert
	}
	if rawSources, ok := compatArgValue(args, "sources"); ok {
		if sources, err := toStringSlice(rawSources); err == nil && len(sources) > 0 {
			if strings.TrimSpace(firstCompatString(args, "target_format", "targetFormat", "format")) != "" {
				return convertpkg.ActionConvert
			}
		}
	}
	return ""
}

func hasSimpleConvertPaths(args map[string]interface{}) bool {
	return strings.TrimSpace(firstCompatString(
		args,
		"input_path", "inputPath", "source_path", "sourcePath", "output_path", "outputPath",
		"destination_path", "destinationPath", "output", "destination", "dest", "to",
	)) != ""
}

func (t *ConvertTool) resolveTaskPaths(ctx context.Context, req convertpkg.TaskRequest) (convertpkg.TaskRequest, error) {
	if len(req.Sources) > 0 {
		resolved, err := t.resolveLocalPaths(ctx, req.Sources)
		if err != nil {
			return req, err
		}
		req.Sources = resolved
	}
	if strings.TrimSpace(req.OutputPath) != "" {
		resolved, err := t.resolveLocalPaths(ctx, []string{req.OutputPath})
		if err != nil {
			return req, err
		}
		req.OutputPath = resolved[0]
	}
	return req, nil
}

func (t *ConvertTool) resolveLocalPaths(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	resolved := make([]string, len(paths))
	dirs := make([]string, 0, len(paths))
	for i, rawPath := range paths {
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" || strings.HasPrefix(trimmed, "att:") || strings.HasPrefix(trimmed, "out:") {
			resolved[i] = trimmed
			continue
		}
		absPath, approvalDir, err := t.resolveLocalPathCandidate(ctx, trimmed, false)
		if err != nil {
			return nil, err
		}
		resolved[i] = absPath
		if approvalDir != "" {
			dirs = append(dirs, approvalDir)
		}
	}
	if err := t.authorizeDirectories(ctx, dirs); err != nil {
		return nil, err
	}
	return resolved, nil
}

func (t *ConvertTool) requestLocalPathApproval(ctx context.Context, paths []string) error {
	_, err := t.resolveLocalPaths(ctx, paths)
	return err
}

func (t *ConvertTool) resolveLocalPathCandidate(ctx context.Context, rawPath string, allowDot bool) (string, string, error) {
	scope := t.pathScope()
	roots := scope.rootsWithContext(ctx)
	aliases := scope.aliasesWithContext(ctx)
	for _, aliasRoot := range aliases {
		roots = append(roots, aliasRoot)
	}
	tempScope := &fsToolScope{roots: uniqueCleanPaths(roots)}

	candidate := strings.TrimSpace(rawPath)
	if resolved, ok := resolveFSAliasPath(candidate, aliases); ok {
		candidate = resolved
	}
	if !filepath.IsAbs(candidate) {
		absPath, _, _, err := tempScope.resolvePath(candidate, allowDot)
		return absPath, "", err
	}

	candidate = filepath.Clean(candidate)
	absPath, _, _, err := tempScope.resolvePath(candidate, allowDot)
	if err == nil {
		return absPath, "", nil
	}
	if !strings.Contains(err.Error(), "path escapes workspace root") {
		return "", "", err
	}
	if t.service != nil && t.service.IsManagedPath(candidate) {
		return candidate, "", nil
	}
	return candidate, externalApprovalDir(candidate, allowDot), nil
}

func (t *ConvertTool) authorizeDirectories(ctx context.Context, dirs []string) error {
	dirs = uniqueCleanPaths(dirs)
	if len(dirs) == 0 {
		return nil
	}
	if t.approvals == nil {
		return errors.New("local path approval is unavailable")
	}
	userID := strings.TrimSpace(GetUserID(ctx))
	for _, dir := range dirs {
		if t.dirStore != nil {
			if entry := t.dirStore.Match(dir); entry != nil {
				continue
			}
		}
		decision, err := t.approvals.RequestApproval(ctx, ApprovalRequest{
			Type:      "directory",
			Directory: dir,
			Command:   "convert " + dir,
			Security:  "The convert tool wants to access a local path outside the managed conversation workspace.",
			UserID:    userID,
		})
		if err != nil {
			return err
		}
		if decision != ApprovalAllowOnce && decision != ApprovalAllowAlways {
			return fmt.Errorf("access denied for local path: %s", dir)
		}
		if decision == ApprovalAllowAlways && t.dirStore != nil {
			if err := t.dirStore.Add(dir, userID); err != nil {
				return fmt.Errorf("persist approved directory: %w", err)
			}
		}
	}
	return nil
}

func (t *ConvertTool) shouldReturnAsyncImmediately(req convertpkg.TaskRequest) bool {
	switch req.Action {
	case convertpkg.ActionMerge, convertpkg.ActionSplit, convertpkg.ActionTrim, convertpkg.ActionExtractAudio, convertpkg.ActionExtractFrame:
		return true
	}
	if len(req.Sources) == 0 {
		return false
	}
	sourcePath := strings.TrimSpace(req.Sources[0])
	if sourcePath == "" || strings.HasPrefix(sourcePath, "att:") || strings.HasPrefix(sourcePath, "out:") {
		return false
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(sourcePath))
	switch {
	case isConvertVideoExtension(ext):
		return true
	case ext == ".pdf" && info.Size() >= largePDFAsyncThreshold:
		return true
	case isConvertDocumentExtension(ext) && info.Size() >= largeDocumentAsyncThreshold:
		return true
	case info.Size() >= largeMediaAsyncThreshold:
		return true
	default:
		return false
	}
}

func parseConvertTaskRequest(args map[string]interface{}) (parsedConvertTaskRequest, error) {
	var parsed parsedConvertTaskRequest
	req := &parsed.TaskRequest
	req.Action = strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command"))
	if req.Action == "" {
		req.Action = normalizeConvertAction(args)
	}
	req.TargetFormat = strings.TrimSpace(firstCompatString(args, "target_format", "targetFormat", "format"))
	req.TaskID = strings.TrimSpace(firstCompatString(args, "task_id", "taskId", "id"))
	req.WaitMS = compatInt(args, "wait_ms", "waitMs")

	simpleInput := strings.TrimSpace(firstCompatString(args, "input_path", "inputPath", "source_path", "sourcePath"))
	simpleOutput := strings.TrimSpace(firstCompatString(args, "output_path", "outputPath", "destination_path", "destinationPath", "output", "destination", "dest", "to"))
	useInputAsPath := simpleInput != "" || (simpleOutput != "" && req.Action != convertpkg.ActionTTS)

	req.Text = strings.TrimSpace(firstCompatString(args, "text", "content", "message"))
	if req.Text == "" && !useInputAsPath {
		req.Text = strings.TrimSpace(firstCompatString(args, "input"))
	}
	if rawSources, ok := compatArgValue(args, "sources"); ok {
		sources, err := toStringSlice(rawSources)
		if err != nil {
			return parsed, err
		}
		req.Sources = sources
	}
	if len(req.Sources) == 0 && useInputAsPath {
		fallbackInput := strings.TrimSpace(firstCompatString(args, "input", "source", "src", "from"))
		if simpleInput != "" {
			fallbackInput = simpleInput
		}
		if fallbackInput != "" && req.Action != convertpkg.ActionTTS {
			req.Sources = []string{fallbackInput}
		}
	}
	if simpleOutput != "" {
		req.OutputPath = simpleOutput
	}
	parsed.SimpleMode = simpleInput != "" || simpleOutput != ""
	if parsed.SimpleMode && req.Action == "" {
		req.Action = convertpkg.ActionConvert
	}
	if rawOptions, ok := compatArgValue(args, "options"); ok {
		if optMap, ok := coerceCompatMap(rawOptions); ok {
			req.Options = parseOptions(optMap)
		}
	}
	if req.Action == convertpkg.ActionTTS {
		req.TargetFormat = strings.TrimSpace(nonEmpty(req.Options.Speech.TTS.Format, req.TargetFormat))
	}
	if req.OutputPath != "" {
		outputFormat := inferConvertFormatFromPath(req.OutputPath)
		if req.TargetFormat == "" {
			req.TargetFormat = outputFormat
		} else if outputFormat != "" && normalizeConvertFormatName(outputFormat) != normalizeConvertFormatName(req.TargetFormat) {
			return parsed, fmt.Errorf("target_format %q does not match output_path extension %q", req.TargetFormat, outputFormat)
		}
	}
	if parsed.SimpleMode && req.Action == convertpkg.ActionConvert && strings.TrimSpace(req.TargetFormat) == "" {
		return parsed, errors.New("output_path must include a file extension or target_format must be specified")
	}
	return parsed, nil
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

func requestedConvertWait(waitMS int) time.Duration {
	if waitMS <= 0 {
		return 0
	}
	wait := time.Duration(waitMS) * time.Millisecond
	if wait > maxConvertWait {
		return maxConvertWait
	}
	return wait
}

func simpleConvertTaskResult(task *convertpkg.ConvertTask, async bool) map[string]interface{} {
	result := map[string]interface{}{
		"async": async,
	}
	if task == nil {
		return result
	}
	if async {
		result["task_id"] = task.ID
	}
	result["status"] = string(task.Status)
	result["action"] = task.Action
	result["message"] = task.Message
	if task.Error != "" {
		result["error"] = task.Error
	}
	if task.TargetFormat != "" {
		result["target_format"] = task.TargetFormat
	}
	if task.TranscriptPreview != "" {
		result["transcript_preview"] = task.TranscriptPreview
	}
	outputs := simpleConvertOutputs(task.Outputs)
	if len(outputs) > 0 {
		result["outputs"] = outputs
		result["output_path"] = outputs[0]["path"]
		result["output_ref"] = outputs[0]["ref"]
		result["download_url"] = outputs[0]["download_url"]
	}
	return result
}

func simpleConvertOutputs(outputs []convertpkg.ConvertOutput) []map[string]interface{} {
	if len(outputs) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]interface{}{
			"output_id":    output.ID,
			"name":         output.Name,
			"mime_type":    output.MimeType,
			"size_bytes":   output.SizeBytes,
			"preview_kind": string(output.PreviewKind),
			"download_url": output.DownloadURL,
			"ref":          output.Ref,
			"preview_text": output.PreviewText,
			"path":         output.Path,
		})
	}
	return out
}

func inferConvertFormatFromPath(path string) string {
	ext := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."))
	if ext == "" {
		return ""
	}
	return ext
}

func normalizeConvertFormatName(format string) string {
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(format), "."))
	switch normalized {
	case "text":
		return "txt"
	default:
		return normalized
	}
}

func isConvertVideoExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".mov", ".mp4", ".m4v", ".avi", ".mkv", ".webm":
		return true
	default:
		return false
	}
}

func isConvertDocumentExtension(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".txt", ".text", ".md", ".markdown", ".rtf", ".doc", ".docx", ".odt", ".html", ".htm", ".xml", ".csv", ".tsv", ".pages":
		return true
	default:
		return false
	}
}

func uniqueCleanPaths(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		clean := filepath.Clean(trimmed)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		cleaned = append(cleaned, clean)
	}
	return cleaned
}
