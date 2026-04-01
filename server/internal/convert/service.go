package convert

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	speechpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	ttspkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/google/uuid"
)

type Service struct {
	store        *Store
	dataDir      string
	rootDir      string
	sourcesDir   string
	tasksDir     string
	helper       *HelperRunner
	ttsProvider  *ttspkg.MacOSNativeTTS
	asrProvider  *speechpkg.MacOSNativeSTT
	locator      commandLocator
	cancelMu     sync.Mutex
	cancelByTask map[string]context.CancelFunc
	janitorStop  chan struct{}
}

func NewService(db *sql.DB, dataDir string) (*Service, error) {
	return NewServiceWithReadDB(db, db, dataDir)
}

func NewServiceWithReadDB(writeDB, readDB *sql.DB, dataDir string) (*Service, error) {
	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Join(dataDir, "convert")
	svc := &Service{
		store:        store,
		dataDir:      dataDir,
		rootDir:      rootDir,
		sourcesDir:   filepath.Join(rootDir, "sources"),
		tasksDir:     filepath.Join(rootDir, "tasks"),
		helper:       NewHelperRunner(),
		ttsProvider:  ttspkg.NewMacOSNativeTTS(),
		asrProvider:  speechpkg.NewMacOSNativeSTT(),
		locator:      defaultCommandLocator(),
		cancelByTask: make(map[string]context.CancelFunc),
		janitorStop:  make(chan struct{}),
	}
	for _, dir := range []string{svc.rootDir, svc.sourcesDir, svc.tasksDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create convert dir %s: %w", dir, err)
		}
	}
	_ = svc.store.MarkInterruptedTasksFailed("convert task interrupted by server restart")
	_ = svc.cleanupExpired()
	go svc.runJanitor()
	return svc, nil
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	select {
	case <-s.janitorStop:
	default:
		close(s.janitorStop)
	}
	return nil
}

func (s *Service) runJanitor() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = s.cleanupExpired()
		case <-s.janitorStop:
			return
		}
	}
}

func (s *Service) cleanupExpired() error {
	if s == nil || s.store == nil {
		return nil
	}
	cutoff := timeutil.NowTime().Add(-retentionWindow)
	taskIDs, _, sourcePaths, err := s.store.CleanupExpired(cutoff)
	if err != nil {
		return err
	}
	for _, taskID := range taskIDs {
		_ = os.RemoveAll(filepath.Join(s.tasksDir, taskID))
	}
	for _, sourcePath := range sourcePaths {
		if strings.TrimSpace(sourcePath) != "" {
			_ = os.Remove(sourcePath)
		}
	}
	return nil
}

func (s *Service) IsManagedPath(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	root := filepath.Clean(s.rootDir)
	abs = filepath.Clean(abs)
	return abs == root || strings.HasPrefix(abs, root+string(filepath.Separator))
}

func (s *Service) Capabilities(ctx context.Context) Capabilities {
	_ = ctx
	tools := map[string]bool{}
	for _, cmd := range []string{"textutil", "sips", "afconvert", "swift", "qlmanage", "cupsfilter", "say", "x2t", "libreoffice", "openoffice", "soffice", "unoconv", "pandoc"} {
		_, err := exec.LookPath(cmd)
		tools[cmd] = err == nil
	}
	engines := s.documentEngines(ctx)
	if x2t := s.findDocumentEngine(engines, documentEngineX2T); x2t != nil {
		tools["x2t"] = x2t.Available
	}
	hasDocumentEngines := hasAvailableDocumentEngine(engines)
	helper := s.helper.Status()
	actions := supportedActionsForPlatform(runtime.GOOS, hasDocumentEngines, s.ttsProvider.Available(), s.asrProvider.Available())
	documentFormats := aggregateDocumentFormats(engines)
	notes := []string{
		"Preferred form is input_path + output_path; relative paths resolve from the workspace root.",
		"Absolute local paths are also supported with approval when needed.",
		"Conversation attachments use att:<id> references; prior outputs use out:<task_id>:<output_id>.",
		"Document conversion auto-detects local engines and falls back by priority.",
		"Advanced PDF/video actions require blue-convert-helper or a local Swift fallback on macOS.",
	}
	if helper.Available {
		documentFormats["helper_pdf_preview"] = append([]string(nil), helperPDFFallbackFormats...)
		notes = append(notes, "When the helper is available, helper_pdf_preview can render supported document formats to preview PDFs if external document engines are missing.")
	}
	return Capabilities{
		Available:       len(actions) > 0,
		Platform:        runtime.GOOS,
		Actions:         actions,
		Tools:           tools,
		Helper:          helper,
		Document:        documentFormats,
		DocumentEngines: engines,
		Image: map[string][]string{
			"sips": {"png", "jpg", "jpeg", "tiff", "gif", "bmp", "heic", "pdf"},
		},
		Audio: map[string][]string{
			"afconvert": {"wav", "m4a", "aiff", "caf", "aac"},
		},
		Video: map[string]interface{}{
			"inputs":  []string{"mov", "mp4", "m4v"},
			"outputs": []string{"mov", "mp4", "m4v", "m4a", "png", "jpg"},
			"helper":  helper.Available,
		},
		Speech: map[string]interface{}{
			"tts":                runtime.GOOS == "darwin",
			"tts_default_format": "wav",
			"asr":                runtime.GOOS == "darwin",
			"asr_on_device":      s.asrProvider.SupportsOnDevice(),
		},
		Retention:  retentionWindow.String(),
		WorkingDir: s.rootDir,
		Notes:      notes,
	}
}

func (s *Service) documentEngines(ctx context.Context) []DocumentEngineInfo {
	return detectDocumentEngines(ctx, runtime.GOOS, s.locator)
}

func (s *Service) findDocumentEngine(engines []DocumentEngineInfo, id string) *DocumentEngineInfo {
	for i := range engines {
		if engines[i].ID == id {
			return &engines[i]
		}
	}
	return nil
}

func supportedActionsForPlatform(goos string, hasDocumentEngine, hasTTS, hasASR bool) []string {
	if goos == "darwin" {
		actions := []string{ActionConvert, ActionMerge, ActionSplit, ActionTrim, ActionExtractAudio, ActionExtractFrame}
		if hasTTS {
			actions = append(actions, ActionTTS)
		}
		if hasASR {
			actions = append(actions, ActionASR)
		}
		return actions
	}
	if hasDocumentEngine {
		return []string{ActionConvert}
	}
	return nil
}

func (s *Service) ensureActionSupported(ctx context.Context, action string) error {
	engines := s.documentEngines(ctx)
	hasDocumentEngine := hasAvailableDocumentEngine(engines)
	actions := supportedActionsForPlatform(runtime.GOOS, hasDocumentEngine, s.ttsProvider.Available(), s.asrProvider.Available())
	if stringInSlice(strings.TrimSpace(action), actions) {
		return nil
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("action %q is unsupported on %s", action, runtime.GOOS)
	}
	return fmt.Errorf("action %q is unavailable on this host", action)
}

func (s *Service) RecordAttachment(ctx context.Context, userID, conversationID, name, mimeType string, data []byte) (string, error) {
	_ = ctx
	if s == nil || s.store == nil {
		return "", fmt.Errorf("convert service unavailable")
	}
	id := uuid.New().String()
	cleanName := sanitizeName(name)
	if cleanName == "" {
		cleanName = "attachment"
	}
	dir := filepath.Join(s.sourcesDir, conversationID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	filePath := filepath.Join(dir, id+"-"+cleanName)
	if err := os.WriteFile(filePath, data, 0o640); err != nil {
		return "", err
	}
	source := &StoredSource{
		ID:             id,
		UserID:         userID,
		ConversationID: conversationID,
		Name:           cleanName,
		MimeType:       mimeType,
		Path:           filePath,
		SizeBytes:      int64(len(data)),
	}
	if err := s.store.CreateSource(source); err != nil {
		_ = os.Remove(filePath)
		return "", err
	}
	return "att:" + id, nil
}

func (s *Service) BuildPromptSummary(ctx context.Context, userID, conversationID string, limit int) string {
	_ = ctx
	if s == nil || s.store == nil || strings.TrimSpace(conversationID) == "" {
		return ""
	}
	if limit <= 0 || limit > 20 {
		limit = 12
	}
	sources, err := s.store.ListSources(userID, conversationID, limit)
	if err != nil {
		return ""
	}
	tasks, err := s.store.ListTasks(userID, conversationID, limit)
	if err != nil {
		return ""
	}
	var lines []string
	for _, source := range sources {
		if source == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("- att:%s — %s (%s, %s)", source.ID, source.Name, nonEmpty(source.MimeType, "file"), humanSize(source.SizeBytes)))
	}
	for _, task := range tasks {
		if task == nil || task.Status != StatusSucceeded {
			continue
		}
		for _, output := range task.Outputs {
			lines = append(lines, fmt.Sprintf("- out:%s:%s — %s (%s)", task.ID, output.ID, output.Name, nonEmpty(string(output.PreviewKind), nonEmpty(output.MimeType, "file"))))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return "\n\n[convert session sources]\nPrefer convert with relative input_path/output_path for workspace files. Use these exact att:/out: refs when you need conversation attachments or prior outputs.\n" + strings.Join(lines, "\n") + "\n[/convert session sources]\n"
}

func (s *Service) GetTask(ctx context.Context, userID, conversationID, taskID string) (*ConvertTask, error) {
	_ = ctx
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) != "" && task.UserID != "" && task.UserID != userID {
		return nil, sql.ErrNoRows
	}
	if strings.TrimSpace(conversationID) != "" && task.ConversationID != conversationID {
		return nil, sql.ErrNoRows
	}
	return s.decorateTask(task), nil
}

func (s *Service) ListTasks(ctx context.Context, userID, conversationID string, limit int) ([]*ConvertTask, error) {
	_ = ctx
	tasks, err := s.store.ListTasks(userID, conversationID, limit)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i] = s.decorateTask(tasks[i])
	}
	return tasks, nil
}

func (s *Service) WaitForTask(ctx context.Context, userID, conversationID, taskID string, wait time.Duration) (*ConvertTask, error) {
	if wait <= 0 {
		return s.GetTask(ctx, userID, conversationID, taskID)
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for {
		task, err := s.GetTask(ctx, userID, conversationID, taskID)
		if err != nil {
			return nil, err
		}
		if task.Status.IsTerminal() {
			return task, nil
		}
		select {
		case <-ctx.Done():
			return task, ctx.Err()
		case <-deadline.C:
			return task, nil
		case <-tick.C:
		}
	}
}

func (s *Service) CancelTask(ctx context.Context, userID, conversationID, taskID string) (*ConvertTask, error) {
	task, err := s.GetTask(ctx, userID, conversationID, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status.IsTerminal() {
		return task, nil
	}
	s.cancelMu.Lock()
	cancel := s.cancelByTask[taskID]
	s.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	now := timeutil.NowTime().UTC()
	task.Status = StatusCancelled
	task.Message = "Task cancelled"
	task.Error = ""
	task.Progress = 100
	task.CompletedAt = &now
	if err := s.store.UpdateTask(task); err != nil {
		return nil, err
	}
	return s.decorateTask(task), nil
}

func (s *Service) ResolveDownload(ctx context.Context, userID, conversationID, taskID, outputID string) (*ConvertTask, *ConvertOutput, error) {
	task, err := s.GetTask(ctx, userID, conversationID, taskID)
	if err != nil {
		return nil, nil, err
	}
	for i := range task.Outputs {
		if task.Outputs[i].ID == outputID {
			return task, &task.Outputs[i], nil
		}
	}
	return task, nil, sql.ErrNoRows
}

func (s *Service) Submit(ctx context.Context, userID, conversationID string, req TaskRequest) (*ConvertTask, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}
	if err := s.ensureActionSupported(ctx, req.Action); err != nil {
		return nil, err
	}
	resolved, err := s.resolveSources(ctx, userID, conversationID, req)
	if err != nil {
		return nil, err
	}
	task := &ConvertTask{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		UserID:         userID,
		Action:         req.Action,
		Status:         StatusPending,
		Sources:        append([]string(nil), req.Sources...),
		TargetFormat:   normalizeFormat(req.TargetFormat, req.Options.Speech.TTS.Format),
		SourceSummary:  summarizeSources(resolved, req.Text),
		Progress:       0,
		Message:        "Queued",
		Request:        &req,
	}
	if err := s.store.CreateTask(task); err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	s.cancelMu.Lock()
	s.cancelByTask[task.ID] = cancel
	s.cancelMu.Unlock()
	go s.runTask(runCtx, task.ID, req, resolved)
	return s.decorateTask(task), nil
}

func (s *Service) runTask(ctx context.Context, taskID string, req TaskRequest, resolved []ResolvedSource) {
	defer func() {
		s.cancelMu.Lock()
		delete(s.cancelByTask, taskID)
		s.cancelMu.Unlock()
	}()
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return
	}
	now := timeutil.NowTime().UTC()
	task.Status = StatusProcessing
	task.Progress = 5
	task.Message = "Processing"
	task.StartedAt = &now
	_ = s.store.UpdateTask(task)

	var outputs []ConvertOutput
	var transcript string
	message := "Completed"
	err = nil
	switch req.Action {
	case ActionConvert:
		outputs, message, err = s.executeConvert(ctx, task, req, resolved)
	case ActionMerge:
		outputs, message, err = s.executeMerge(ctx, task, req, resolved)
	case ActionSplit:
		outputs, message, err = s.executeSplit(ctx, task, req, resolved)
	case ActionTrim:
		outputs, message, err = s.executeTrim(ctx, task, req, resolved)
	case ActionExtractAudio:
		outputs, message, err = s.executeExtractAudio(ctx, task, req, resolved)
	case ActionExtractFrame:
		outputs, message, err = s.executeExtractFrames(ctx, task, req, resolved)
	case ActionTTS:
		outputs, message, err = s.executeTTS(ctx, task, req)
	case ActionASR:
		outputs, transcript, message, err = s.executeASR(ctx, task, req, resolved)
	default:
		err = fmt.Errorf("unsupported action: %s", req.Action)
	}

	completedAt := timeutil.NowTime().UTC()
	if err != nil {
		task.Outputs = nil
		task.TranscriptPreview = transcript
		task.Progress = 100
		task.CompletedAt = &completedAt
		if errors.Is(err, context.Canceled) {
			task.Status = StatusCancelled
			task.Message = "Task cancelled"
			task.Error = ""
		} else {
			task.Status = StatusFailed
			task.Message = "Task failed"
			task.Error = err.Error()
		}
		_ = s.store.UpdateTask(task)
		return
	}
	outputs, err = s.applyRequestedOutputPath(task.ID, req, outputs)
	if err != nil {
		task.Outputs = nil
		task.TranscriptPreview = transcript
		task.Progress = 100
		task.CompletedAt = &completedAt
		task.Status = StatusFailed
		task.Message = "Task failed"
		task.Error = err.Error()
		_ = s.store.UpdateTask(task)
		return
	}
	task.Status = StatusSucceeded
	task.Progress = 100
	task.Message = message
	task.Error = ""
	task.Outputs = outputs
	task.TranscriptPreview = transcript
	task.CompletedAt = &completedAt
	_ = s.store.UpdateTask(task)
}

func (s *Service) executeConvert(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, error) {
	if len(sources) != 1 {
		return nil, "", fmt.Errorf("convert requires exactly one source")
	}
	source := sources[0]
	target := normalizeFormat(req.TargetFormat, "")
	if target == "" {
		return nil, "", fmt.Errorf("target_format is required")
	}
	if source.Category == "document" {
		return s.convertDocument(ctx, task, source, target)
	}
	if source.Category == "pdf" && (isDocumentFormat(target) || target == "pdf") {
		return s.convertDocument(ctx, task, source, target)
	}
	if source.Category == "image" {
		if runtime.GOOS != "darwin" {
			return nil, "", fmt.Errorf("image conversion is unsupported on %s", runtime.GOOS)
		}
		if target == "pdf" {
			return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
				"action":        "image_to_pdf",
				"sources":       []string{source.Path},
				"target_format": target,
				"output_dir":    s.outputDir(task.ID),
			}, "Image PDF created")
		}
		return s.convertImage(ctx, task, source, target)
	}
	if source.Category == "audio" {
		if runtime.GOOS != "darwin" {
			return nil, "", fmt.Errorf("audio conversion is unsupported on %s", runtime.GOOS)
		}
		return s.convertAudio(ctx, task, source, target)
	}
	if source.Category == "video" {
		if runtime.GOOS != "darwin" {
			return nil, "", fmt.Errorf("video conversion is unsupported on %s", runtime.GOOS)
		}
		if isVideoFormat(target) {
			return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
				"action":        "video_convert",
				"sources":       []string{source.Path},
				"target_format": target,
				"output_dir":    s.outputDir(task.ID),
				"options":       map[string]interface{}{"video": req.Options.Video},
			}, "Video converted")
		}
		if isAudioFormat(target) {
			return s.executeExtractAudio(ctx, task, req, sources)
		}
	}
	if source.Category == "pdf" && isImageFormat(target) {
		return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
			"action":        "pdf_to_images",
			"sources":       []string{source.Path},
			"target_format": target,
			"output_dir":    s.outputDir(task.ID),
			"options":       map[string]interface{}{"document": req.Options.Document},
		}, "PDF pages exported")
	}
	return nil, "", fmt.Errorf("unsupported conversion from %s to %s", source.Category, target)
}

func (s *Service) executeMerge(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, error) {
	if len(sources) < 2 {
		return nil, "", fmt.Errorf("merge requires at least two sources")
	}
	target := normalizeFormat(req.TargetFormat, "")
	if target == "pdf" {
		paths := make([]string, 0, len(sources))
		for _, source := range sources {
			if source.Category != "image" && source.Category != "pdf" {
				return nil, "", fmt.Errorf("merge to pdf only supports image/pdf sources")
			}
			paths = append(paths, source.Path)
		}
		return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
			"action":        "merge_pdf",
			"sources":       paths,
			"target_format": "pdf",
			"output_dir":    s.outputDir(task.ID),
		}, "PDF merged")
	}
	if isVideoFormat(target) {
		paths := make([]string, 0, len(sources))
		for _, source := range sources {
			if source.Category != "video" {
				return nil, "", fmt.Errorf("merge to %s only supports video sources", target)
			}
			paths = append(paths, source.Path)
		}
		return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
			"action":        "video_merge",
			"sources":       paths,
			"target_format": target,
			"output_dir":    s.outputDir(task.ID),
			"options":       map[string]interface{}{"video": req.Options.Video},
		}, "Video merged")
	}
	return nil, "", fmt.Errorf("unsupported merge target: %s", target)
}

func (s *Service) executeSplit(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, error) {
	if len(sources) != 1 {
		return nil, "", fmt.Errorf("split requires exactly one source")
	}
	source := sources[0]
	if source.Category == "pdf" {
		pages := req.Options.Document.Pages
		return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
			"action":     "pdf_split",
			"sources":    []string{source.Path},
			"output_dir": s.outputDir(task.ID),
			"options": map[string]interface{}{
				"document": map[string]interface{}{"pages": pages},
			},
		}, "PDF split")
	}
	if source.Category == "video" {
		if len(req.Options.Video.Segments) == 0 {
			return nil, "", fmt.Errorf("options.video.segments is required for video split")
		}
		return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
			"action":        "video_split",
			"sources":       []string{source.Path},
			"target_format": normalizeFormat(req.TargetFormat, videoExt(source.Path)),
			"output_dir":    s.outputDir(task.ID),
			"options":       map[string]interface{}{"video": req.Options.Video},
		}, "Video split")
	}
	return nil, "", fmt.Errorf("split is unsupported for %s", source.Category)
}

func (s *Service) executeTrim(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, error) {
	if len(sources) != 1 || sources[0].Category != "video" {
		return nil, "", fmt.Errorf("trim requires a single video source")
	}
	if req.Options.Video.EndMS <= 0 {
		return nil, "", fmt.Errorf("options.video.end_ms is required for trim")
	}
	return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
		"action":        "video_trim",
		"sources":       []string{sources[0].Path},
		"target_format": normalizeFormat(req.TargetFormat, videoExt(sources[0].Path)),
		"output_dir":    s.outputDir(task.ID),
		"options":       map[string]interface{}{"video": req.Options.Video},
	}, "Video trimmed")
}

func (s *Service) executeExtractAudio(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, error) {
	if len(sources) != 1 || sources[0].Category != "video" {
		return nil, "", fmt.Errorf("extract_audio requires a single video source")
	}
	target := normalizeFormat(req.TargetFormat, "m4a")
	if !isAudioFormat(target) {
		return nil, "", fmt.Errorf("unsupported audio target format: %s", target)
	}
	return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
		"action":        "extract_audio",
		"sources":       []string{sources[0].Path},
		"target_format": target,
		"output_dir":    s.outputDir(task.ID),
		"options":       map[string]interface{}{"video": req.Options.Video},
	}, "Audio extracted")
}

func (s *Service) executeExtractFrames(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, error) {
	if len(sources) != 1 || sources[0].Category != "video" {
		return nil, "", fmt.Errorf("extract_frames requires a single video source")
	}
	target := normalizeFormat(req.TargetFormat, "png")
	if !isImageFormat(target) {
		return nil, "", fmt.Errorf("unsupported frame target format: %s", target)
	}
	return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
		"action":        "extract_frames",
		"sources":       []string{sources[0].Path},
		"target_format": target,
		"output_dir":    s.outputDir(task.ID),
		"options":       map[string]interface{}{"video": req.Options.Video},
	}, "Frames extracted")
}

func (s *Service) executeTTS(ctx context.Context, task *ConvertTask, req TaskRequest) ([]ConvertOutput, string, error) {
	if err := s.ttsProvider.Initialize(); err != nil {
		return nil, "", err
	}
	format := normalizeFormat(req.Options.Speech.TTS.Format, "wav")
	speed := float32(1.0)
	if req.Options.Speech.TTS.Rate > 0 {
		speed = float32(req.Options.Speech.TTS.Rate) / 200.0
		if speed < 0.4 {
			speed = 0.4
		}
	}
	resp, err := s.ttsProvider.Synthesize(ctx, &ttspkg.SynthesizeRequest{
		Text:   req.Text,
		Voice:  req.Options.Speech.TTS.Voice,
		Format: ttspkg.FormatWAV,
		Speed:  speed,
	})
	if err != nil {
		return nil, "", err
	}
	defer resp.Audio.Close()
	wavData, err := io.ReadAll(resp.Audio)
	if err != nil {
		return nil, "", err
	}
	baseName := "tts-output"
	outputDir := s.outputDir(task.ID)
	wavPath := filepath.Join(outputDir, baseName+".wav")
	if err := os.WriteFile(wavPath, wavData, 0o640); err != nil {
		return nil, "", err
	}
	finalPath := wavPath
	finalMime := "audio/wav"
	finalExt := "wav"
	if format == "m4a" || format == "aac" {
		converted := filepath.Join(outputDir, baseName+".m4a")
		if err := convertAudioFile(ctx, wavPath, converted, "m4a", req.Options.Audio); err != nil {
			return nil, "", err
		}
		finalPath = converted
		finalMime = "audio/mp4"
		finalExt = "m4a"
	}
	output, err := s.outputForPath(task.ID, finalPath, "")
	if err != nil {
		return nil, "", err
	}
	output.Name = baseName + "." + finalExt
	output.MimeType = finalMime
	output.PreviewKind = PreviewAudio
	return []ConvertOutput{output}, "Speech audio created", nil
}

func (s *Service) executeASR(ctx context.Context, task *ConvertTask, req TaskRequest, sources []ResolvedSource) ([]ConvertOutput, string, string, error) {
	if len(sources) != 1 {
		return nil, "", "", fmt.Errorf("asr requires exactly one source")
	}
	source := sources[0]
	inputPath := source.Path
	if source.Category == "video" {
		outputs, _, err := s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
			"action":        "extract_audio",
			"sources":       []string{source.Path},
			"target_format": "m4a",
			"output_dir":    s.outputDir(task.ID),
		}, "")
		if err != nil {
			return nil, "", "", err
		}
		if len(outputs) == 0 {
			return nil, "", "", fmt.Errorf("helper did not produce extracted audio")
		}
		inputPath = outputs[0].Path
	}
	if err := s.asrProvider.Initialize(); err != nil {
		return nil, "", "", err
	}
	previousRequire := s.asrProvider.RequireOnDevice()
	s.asrProvider.SetRequireOnDevice(req.Options.Speech.ASR.OnDeviceOnly)
	defer s.asrProvider.SetRequireOnDevice(previousRequire)

	transcribePath := inputPath
	format := sttFormatFromPath(inputPath)
	if format == "" || strings.EqualFold(filepath.Ext(inputPath), ".m4a") {
		wavPath := filepath.Join(s.outputDir(task.ID), "asr-input.wav")
		if err := convertAudioFile(ctx, inputPath, wavPath, "wav", AudioOptions{}); err != nil {
			return nil, "", "", err
		}
		transcribePath = wavPath
		format = stt.FormatWAV
	}
	file, err := os.Open(transcribePath)
	if err != nil {
		return nil, "", "", err
	}
	defer file.Close()
	resp, err := s.asrProvider.Transcribe(ctx, &stt.TranscribeRequest{
		Audio:    file,
		Format:   format,
		Language: req.Options.Speech.ASR.Language,
	})
	if err != nil {
		return nil, "", "", err
	}
	text := strings.TrimSpace(resp.Text)
	outputDir := s.outputDir(task.ID)
	textPath := filepath.Join(outputDir, "transcript.txt")
	if err := os.WriteFile(textPath, []byte(text), 0o640); err != nil {
		return nil, "", "", err
	}
	preview := text
	if len([]rune(preview)) > 280 {
		preview = string([]rune(preview)[:280]) + "…"
	}
	output, err := s.outputForPath(task.ID, textPath, preview)
	if err != nil {
		return nil, "", "", err
	}
	output.Name = "transcript.txt"
	output.MimeType = "text/plain; charset=utf-8"
	output.PreviewKind = PreviewText
	output.PreviewText = preview
	return []ConvertOutput{output}, preview, "Transcription completed", nil
}

func (s *Service) convertDocument(ctx context.Context, task *ConvertTask, source ResolvedSource, target string) ([]ConvertOutput, string, error) {
	sourceExt := docExt(source.Path)
	target = normalizeFormat(target, "")
	if target == "" {
		return nil, "", fmt.Errorf("target_format is required")
	}

	if outputPath, preview, handled, err := convertSpreadsheetLocally(s.outputDir(task.ID), source, target); handled {
		if err != nil {
			return nil, "", err
		}
		output, err := s.outputForPath(task.ID, outputPath, preview)
		if err != nil {
			return nil, "", err
		}
		if target == "json" || target == "csv" || target == "txt" || target == "md" {
			output.PreviewKind = PreviewText
			output.PreviewText = preview
		}
		return []ConvertOutput{output}, "Spreadsheet converted", nil
	}

	engines := availableDocumentEngines(s.documentEngines(ctx))
	if len(engines) == 0 && !helperSupportsDocumentPDFFallback(sourceExt, target) {
		return nil, "", fmt.Errorf("no document conversion engine is available on this host")
	}

	outputDir := s.outputDir(task.ID)
	base := trimExt(source.Name)
	outputPath := filepath.Join(outputDir, base+"."+target)
	_ = os.Remove(outputPath)

	attempts := make([]string, 0, len(engines))
	for _, engine := range engines {
		if !engineSupportsConversion(engine.ID, sourceExt, target) {
			continue
		}
		_ = os.Remove(outputPath)
		if err := s.convertDocumentWithEngine(ctx, engine, source.Path, outputPath, target); err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", engine.ID, err))
			continue
		}
		output, err := s.outputForPath(task.ID, outputPath, "")
		if err != nil {
			return nil, "", err
		}
		return []ConvertOutput{output}, "Document converted", nil
	}

	if helperSupportsDocumentPDFFallback(sourceExt, target) {
		outputs, message, err := s.convertDocumentWithHelperPDF(ctx, task, source)
		if err == nil {
			return outputs, message, nil
		}
		attempts = append(attempts, fmt.Sprintf("helper_pdf: %v", err))
	}

	if len(attempts) == 0 {
		return nil, "", fmt.Errorf("no document conversion engine supports %s -> %s", sourceExt, target)
	}
	return nil, "", fmt.Errorf("document conversion failed after trying converters (%s)", strings.Join(attempts, "; "))
}

func (s *Service) convertDocumentWithEngine(ctx context.Context, engine DocumentEngineInfo, sourcePath, outputPath, target string) error {
	return runDocumentConversionWithEngine(ctx, engine, sourcePath, outputPath, target)
}

func runDocumentConversionWithEngine(ctx context.Context, engine DocumentEngineInfo, sourcePath, outputPath, target string) error {
	switch engine.ID {
	case documentEngineX2T:
		return runCommand(ctx, engine.Path, sourcePath, outputPath)
	case documentEngineLibreOffice, documentEngineOpenOffice:
		outputDir := filepath.Dir(outputPath)
		if err := runCommand(ctx, engine.Path, "--headless", "--convert-to", target, "--outdir", outputDir, sourcePath); err != nil {
			return err
		}
		actualPath, err := officeOutputPath(outputDir, sourcePath, target)
		if err != nil {
			return err
		}
		if actualPath != outputPath {
			_ = os.Remove(outputPath)
			if err := os.Rename(actualPath, outputPath); err != nil {
				return err
			}
		}
		return nil
	case documentEngineUnoconv:
		return runCommand(ctx, engine.Path, "-f", target, "-o", outputPath, sourcePath)
	case documentEnginePandoc:
		return runCommand(ctx, engine.Path, sourcePath, "-o", outputPath)
	case documentEngineTextutil:
		return runCommand(ctx, engine.Path, "-convert", target, "-output", outputPath, sourcePath)
	default:
		return fmt.Errorf("unsupported engine: %s", engine.ID)
	}
}

func (s *Service) convertDocumentWithHelperPDF(ctx context.Context, task *ConvertTask, source ResolvedSource) ([]ConvertOutput, string, error) {
	if s == nil || s.helper == nil {
		return nil, "", fmt.Errorf("convert helper unavailable")
	}
	return s.runHelperOutputs(ctx, task.ID, map[string]interface{}{
		"action":        "render_document_pdf",
		"sources":       []string{source.Path},
		"target_format": "pdf",
		"output_dir":    s.outputDir(task.ID),
	}, "Document rendered to PDF")
}

func (s *Service) convertWithTextutil(ctx context.Context, task *ConvertTask, source ResolvedSource, target string) ([]ConvertOutput, string, error) {
	engine := s.findDocumentEngine(s.documentEngines(ctx), documentEngineTextutil)
	if engine == nil || !engine.Available {
		return nil, "", fmt.Errorf("textutil unavailable")
	}
	outputDir := s.outputDir(task.ID)
	base := trimExt(source.Name)
	outputPath := filepath.Join(outputDir, base+"."+target)
	if err := runCommand(ctx, engine.Path, "-convert", target, "-output", outputPath, source.Path); err != nil {
		return nil, "", err
	}
	output, err := s.outputForPath(task.ID, outputPath, "")
	if err != nil {
		return nil, "", err
	}
	return []ConvertOutput{output}, "Document converted", nil
}

func (s *Service) convertImage(ctx context.Context, task *ConvertTask, source ResolvedSource, target string) ([]ConvertOutput, string, error) {
	outputDir := s.outputDir(task.ID)
	base := trimExt(source.Name)
	outputPath := filepath.Join(outputDir, base+"."+target)
	if err := runCommand(ctx, "sips", "-s", "format", sipsFormat(target), source.Path, "--out", outputPath); err != nil {
		return nil, "", err
	}
	output, err := s.outputForPath(task.ID, outputPath, "")
	if err != nil {
		return nil, "", err
	}
	return []ConvertOutput{output}, "Image converted", nil
}

func (s *Service) convertAudio(ctx context.Context, task *ConvertTask, source ResolvedSource, target string) ([]ConvertOutput, string, error) {
	outputDir := s.outputDir(task.ID)
	base := trimExt(source.Name)
	outputPath := filepath.Join(outputDir, base+"."+target)
	if err := convertAudioFile(ctx, source.Path, outputPath, target, task.Request.Options.Audio); err != nil {
		return nil, "", err
	}
	output, err := s.outputForPath(task.ID, outputPath, "")
	if err != nil {
		return nil, "", err
	}
	return []ConvertOutput{output}, "Audio converted", nil
}

func (s *Service) runHelperOutputs(ctx context.Context, taskID string, payload map[string]interface{}, successMessage string) ([]ConvertOutput, string, error) {
	resp, err := s.helper.Run(ctx, payload)
	if err != nil {
		return nil, "", err
	}
	outputs := make([]ConvertOutput, 0, len(resp.Outputs))
	for _, output := range resp.Outputs {
		if output.Path == "" {
			continue
		}
		materialized, err := s.outputForPath(taskID, output.Path, output.PreviewText)
		if err != nil {
			return nil, "", err
		}
		if output.ID != "" {
			materialized.ID = output.ID
		}
		if strings.TrimSpace(output.Name) != "" {
			materialized.Name = output.Name
		}
		if strings.TrimSpace(output.MimeType) != "" {
			materialized.MimeType = output.MimeType
		}
		if output.PreviewKind != "" {
			materialized.PreviewKind = output.PreviewKind
		}
		if output.PreviewText != "" {
			materialized.PreviewText = output.PreviewText
		}
		outputs = append(outputs, materialized)
	}
	if len(outputs) == 0 {
		return nil, "", fmt.Errorf("helper produced no outputs")
	}
	if strings.TrimSpace(resp.Message) != "" {
		successMessage = resp.Message
	}
	return outputs, successMessage, nil
}

func (s *Service) applyRequestedOutputPath(taskID string, req TaskRequest, outputs []ConvertOutput) ([]ConvertOutput, error) {
	targetPath := strings.TrimSpace(req.OutputPath)
	if targetPath == "" {
		return outputs, nil
	}
	if len(outputs) != 1 {
		return nil, fmt.Errorf("output_path only supports single-output conversions")
	}
	current := outputs[0]
	currentPath := filepath.Clean(strings.TrimSpace(current.Path))
	if currentPath == "" {
		return nil, fmt.Errorf("conversion output is missing a file path")
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, err
	}
	targetAbs = filepath.Clean(targetAbs)
	if currentPath == targetAbs {
		return outputs, nil
	}
	if info, statErr := os.Stat(targetAbs); statErr == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("output_path is a directory: %s", targetAbs)
		}
		if removeErr := os.Remove(targetAbs); removeErr != nil {
			return nil, removeErr
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o750); err != nil {
		return nil, err
	}
	if err := moveConvertOutputFile(currentPath, targetAbs); err != nil {
		return nil, err
	}
	info, err := os.Stat(targetAbs)
	if err != nil {
		return nil, err
	}
	current.Path = targetAbs
	current.Name = filepath.Base(targetAbs)
	current.MimeType = mimeTypeForPath(targetAbs)
	current.SizeBytes = info.Size()
	current.PreviewKind = inferPreviewKind(targetAbs)
	if strings.TrimSpace(taskID) != "" {
		current.Ref = fmt.Sprintf("out:%s:%s", taskID, current.ID)
		current.DownloadURL = fmt.Sprintf("/api/v1/convert/tasks/%s/download/%s", taskID, current.ID)
	}
	outputs[0] = current
	return outputs, nil
}

func moveConvertOutputFile(sourcePath, targetPath string) error {
	if err := os.Rename(sourcePath, targetPath); err == nil {
		return nil
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		_ = os.Remove(targetPath)
		return err
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(targetPath)
		return err
	}
	if err := os.Remove(sourcePath); err != nil {
		return err
	}
	return nil
}

func (s *Service) outputForPath(taskID, path, previewText string) (ConvertOutput, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return ConvertOutput{}, err
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return ConvertOutput{}, err
	}
	if info.IsDir() {
		return ConvertOutput{}, fmt.Errorf("output path is a directory: %s", cleanPath)
	}
	output := ConvertOutput{
		ID:          uuid.New().String(),
		Name:        filepath.Base(cleanPath),
		MimeType:    mimeTypeForPath(cleanPath),
		SizeBytes:   info.Size(),
		PreviewKind: inferPreviewKind(cleanPath),
		PreviewText: previewText,
		Path:        cleanPath,
	}
	if strings.TrimSpace(taskID) != "" {
		output.Ref = fmt.Sprintf("out:%s:%s", taskID, output.ID)
		output.DownloadURL = fmt.Sprintf("/api/v1/convert/tasks/%s/download/%s", taskID, output.ID)
	}
	return output, nil
}

func (s *Service) decorateTask(task *ConvertTask) *ConvertTask {
	if task == nil {
		return nil
	}
	clone := *task
	clone.Sources = append([]string(nil), task.Sources...)
	clone.Outputs = make([]ConvertOutput, len(task.Outputs))
	copy(clone.Outputs, task.Outputs)
	for i := range clone.Outputs {
		clone.Outputs[i].Ref = fmt.Sprintf("out:%s:%s", task.ID, clone.Outputs[i].ID)
		clone.Outputs[i].DownloadURL = fmt.Sprintf("/api/v1/convert/tasks/%s/download/%s", task.ID, clone.Outputs[i].ID)
	}
	return &clone
}

func (s *Service) validateRequest(req TaskRequest) error {
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return fmt.Errorf("action is required")
	}
	switch action {
	case ActionTTS:
		if strings.TrimSpace(req.Text) == "" {
			return fmt.Errorf("text is required for tts")
		}
	case ActionASR:
		if len(req.Sources) != 1 {
			return fmt.Errorf("asr requires exactly one source")
		}
	case ActionConvert, ActionMerge, ActionSplit, ActionTrim, ActionExtractAudio, ActionExtractFrame:
		if len(req.Sources) == 0 {
			return fmt.Errorf("sources is required")
		}
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
	return nil
}

func (s *Service) resolveSources(ctx context.Context, userID, conversationID string, req TaskRequest) ([]ResolvedSource, error) {
	if req.Action == ActionTTS {
		return nil, nil
	}
	resolved := make([]ResolvedSource, 0, len(req.Sources))
	for _, raw := range req.Sources {
		item, err := s.resolveSource(ctx, userID, conversationID, raw)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, item)
	}
	return resolved, nil
}

func (s *Service) resolveSource(ctx context.Context, userID, conversationID, ref string) (ResolvedSource, error) {
	_ = ctx
	trimmed := strings.TrimSpace(ref)
	if strings.HasPrefix(trimmed, "att:") {
		id := strings.TrimPrefix(trimmed, "att:")
		source, err := s.store.GetSource(id)
		if err != nil {
			return ResolvedSource{}, err
		}
		if source.UserID != "" && source.UserID != userID {
			return ResolvedSource{}, sql.ErrNoRows
		}
		if source.ConversationID != conversationID {
			return ResolvedSource{}, sql.ErrNoRows
		}
		return ResolvedSource{Ref: trimmed, Name: source.Name, MimeType: source.MimeType, Path: source.Path, Size: source.SizeBytes, Category: categorizeSource(source.Path, source.MimeType)}, nil
	}
	if strings.HasPrefix(trimmed, "out:") {
		parts := strings.Split(trimmed, ":")
		if len(parts) != 3 {
			return ResolvedSource{}, fmt.Errorf("invalid out reference: %s", trimmed)
		}
		task, err := s.store.GetTask(parts[1])
		if err != nil {
			return ResolvedSource{}, err
		}
		if task.UserID != "" && task.UserID != userID {
			return ResolvedSource{}, sql.ErrNoRows
		}
		if task.ConversationID != conversationID {
			return ResolvedSource{}, sql.ErrNoRows
		}
		for _, output := range task.Outputs {
			if output.ID == parts[2] {
				return ResolvedSource{Ref: trimmed, Name: output.Name, MimeType: output.MimeType, Path: output.Path, Size: output.SizeBytes, Category: categorizeSource(output.Path, output.MimeType)}, nil
			}
		}
		return ResolvedSource{}, sql.ErrNoRows
	}
	if !filepath.IsAbs(trimmed) {
		return ResolvedSource{}, fmt.Errorf("source must be att:<id>, out:<task_id>:<output_id>, or an absolute path")
	}
	cleanPath, err := filepath.Abs(trimmed)
	if err != nil {
		return ResolvedSource{}, err
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return ResolvedSource{}, err
	}
	if info.IsDir() {
		return ResolvedSource{}, fmt.Errorf("source path is a directory: %s", cleanPath)
	}
	return ResolvedSource{Ref: cleanPath, Name: filepath.Base(cleanPath), MimeType: mimeTypeForPath(cleanPath), Path: cleanPath, Size: info.Size(), Category: categorizeSource(cleanPath, "")}, nil
}

func CardData(task *ConvertTask) map[string]interface{} {
	if task == nil {
		return nil
	}
	outputs := make([]map[string]interface{}, 0, len(task.Outputs))
	for _, output := range task.Outputs {
		outputs = append(outputs, map[string]interface{}{
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
	payload := map[string]interface{}{
		"type":               "convert-task",
		"id":                 "convert-task-" + task.ID,
		"task_id":            task.ID,
		"status":             string(task.Status),
		"action":             task.Action,
		"sources":            append([]string(nil), task.Sources...),
		"source_summary":     task.SourceSummary,
		"target_format":      task.TargetFormat,
		"progress":           task.Progress,
		"message":            nonEmpty(task.Message, task.Error),
		"outputs":            outputs,
		"transcript_preview": task.TranscriptPreview,
		"created_at":         task.CreatedAt,
		"updated_at":         task.UpdatedAt,
	}
	if task.Error != "" {
		payload["error"] = task.Error
	}
	return payload
}

func (s *Service) outputDir(taskID string) string {
	dir := filepath.Join(s.tasksDir, taskID, "outputs")
	_ = os.MkdirAll(dir, 0o750)
	return dir
}

func convertAudioFile(ctx context.Context, src, dst, target string, options AudioOptions) error {
	args, err := afconvertArgs(target, options)
	if err != nil {
		return err
	}
	args = append(args, src, dst)
	return runCommand(ctx, "afconvert", args...)
}

func afconvertArgs(target string, options AudioOptions) ([]string, error) {
	_ = options
	switch normalizeFormat(target, "") {
	case "wav":
		return []string{"-f", "WAVE", "-d", "LEI16"}, nil
	case "m4a", "aac":
		return []string{"-f", "m4af", "-d", "aac"}, nil
	case "aiff":
		return []string{"-f", "AIFF", "-d", "BEI16"}, nil
	case "caf":
		return []string{"-f", "caff", "-d", "LEI16"}, nil
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", target)
	}
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w (%s)", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sanitizeName(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.ReplaceAll(trimmed, string(filepath.Separator), "-")
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	if trimmed == "" {
		return ""
	}
	return trimmed
}

func summarizeSources(sources []ResolvedSource, text string) string {
	if strings.TrimSpace(text) != "" && len(sources) == 0 {
		preview := strings.TrimSpace(text)
		if len([]rune(preview)) > 48 {
			preview = string([]rune(preview)[:48]) + "…"
		}
		return preview
	}
	if len(sources) == 0 {
		return ""
	}
	parts := make([]string, 0, min(3, len(sources)))
	for i, source := range sources {
		if i >= 3 {
			break
		}
		parts = append(parts, source.Name)
	}
	summary := strings.Join(parts, ", ")
	if len(sources) > 3 {
		summary += fmt.Sprintf(" +%d more", len(sources)-3)
	}
	return summary
}

func humanSize(size int64) string {
	if size <= 0 {
		return "0 B"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return strings.TrimSuffix(name, ext)
}

func docExt(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

func normalizeFormat(primary, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(primary))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(fallback))
	}
	if value == "text" {
		value = "txt"
	}
	if value == "jpeg" {
		value = "jpg"
	}
	if value == "htm" {
		value = "html"
	}
	return value
}

func sipsFormat(format string) string {
	switch normalizeFormat(format, "") {
	case "jpg":
		return "jpeg"
	default:
		return normalizeFormat(format, "")
	}
}

func videoExt(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return "mp4"
	}
	return ext
}

func categorizeSource(path, mimeType string) string {
	ext := normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."), "")
	switch {
	case ext == "pdf" || strings.Contains(mimeType, "pdf"):
		return "pdf"
	case isDocumentFormat(ext) || strings.HasPrefix(mimeType, "text/"):
		return "document"
	case isImageFormat(ext) || strings.HasPrefix(mimeType, "image/"):
		return "image"
	case isVideoFormat(ext) || strings.HasPrefix(mimeType, "video/"):
		return "video"
	case isAudioFormat(ext) || strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	default:
		return "file"
	}
}

func isDocumentFormat(ext string) bool {
	switch normalizeFormat(ext, "") {
	case "txt", "rtf", "rtfd", "html", "htm", "md", "doc", "docx", "odt", "wordml", "webarchive", "xls", "xlsx", "ods", "csv", "tsv", "ppt", "pptx", "odp":
		return true
	default:
		return false
	}
}

func isImageFormat(ext string) bool {
	switch normalizeFormat(ext, "") {
	case "png", "jpg", "tiff", "gif", "bmp", "heic", "heif":
		return true
	default:
		return false
	}
}

func isAudioFormat(ext string) bool {
	switch normalizeFormat(ext, "") {
	case "wav", "m4a", "aac", "aiff", "caf", "mp3", "flac", "ogg":
		return true
	default:
		return false
	}
}

func isVideoFormat(ext string) bool {
	switch normalizeFormat(ext, "") {
	case "mov", "mp4", "m4v":
		return true
	default:
		return false
	}
}

func inferPreviewKind(path string) OutputPreviewKind {
	ext := normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."), "")
	switch {
	case ext == "pdf":
		return PreviewPDF
	case isImageFormat(ext):
		return PreviewImage
	case isVideoFormat(ext):
		return PreviewVideo
	case isAudioFormat(ext):
		return PreviewAudio
	case ext == "txt":
		return PreviewText
	default:
		return PreviewFile
	}
}

func mimeTypeForPath(path string) string {
	if detected := mime.TypeByExtension(filepath.Ext(path)); strings.TrimSpace(detected) != "" {
		return detected
	}
	switch inferPreviewKind(path) {
	case PreviewPDF:
		return "application/pdf"
	case PreviewAudio:
		return "audio/*"
	case PreviewVideo:
		return "video/*"
	case PreviewText:
		return "text/plain; charset=utf-8"
	default:
		return http.DetectContentType([]byte(filepath.Base(path)))
	}
}

func sttFormatFromPath(path string) stt.AudioFormat {
	switch normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."), "") {
	case "wav":
		return stt.FormatWAV
	case "mp3":
		return stt.FormatMP3
	case "ogg":
		return stt.FormatOGG
	case "webm":
		return stt.FormatWebM
	case "flac":
		return stt.FormatFLAC
	default:
		return ""
	}
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
