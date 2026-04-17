package tools

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
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
		Description: "Convert local files. Prefer input_path/output_path. Large jobs may return async with task_id. For office-specific args, reuse docx/xlsx/pptx/pdf params.",
		Icon:        "wand-sparkles",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Optional. Omit for convert; task_id alone means status.",
					"enum":        []string{"convert", "merge", "split", "trim", "extract_audio", "extract_frames", "tts", "asr", "status", "list", "cancel", "capabilities"},
				},
				"input_path": map[string]interface{}{
					"type": "string",
				},
				"output_path": map[string]interface{}{
					"type": "string",
				},
				"sources": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Legacy multi-source input (att:, out:, workspace path, or absolute path).",
				},
				"target_format": map[string]interface{}{
					"type": "string",
				},
				"content": map[string]interface{}{"type": "string"},
				"title":   map[string]interface{}{"type": "string"},
				"summary": map[string]interface{}{},
				"theme":   map[string]interface{}{"type": "string"},
				"task_id": map[string]interface{}{
					"type": "string",
				},
				"wait_ms": map[string]interface{}{"type": "integer"},
				"options": map[string]interface{}{
					"type":                 "object",
					"description":          "Office overrides; reuse docx/xlsx/pptx/pdf params.",
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
	if nativeResult, handled, nativeErr := t.maybeHandleNativeOfficeConvert(ctx, args, req); handled {
		return nativeResult, nativeErr
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

func (t *ConvertTool) maybeHandleNativeOfficeConvert(ctx context.Context, args map[string]interface{}, req convertpkg.TaskRequest) (interface{}, bool, error) {
	if normalizeConvertFormatName(req.Action) != "" && strings.TrimSpace(req.Action) != convertpkg.ActionConvert {
		return nil, false, nil
	}
	target := normalizeConvertFormatName(req.TargetFormat)
	if !isNativeOfficeConvertTarget(target) {
		return nil, false, nil
	}

	mergedArgs, wantsNative := buildNativeOfficeConvertArgs(args, target)
	if !wantsNative {
		return nil, false, nil
	}
	if len(req.Sources) > 1 {
		return nil, true, fmt.Errorf("styled native %s convert currently supports exactly one source file", target)
	}

	sourcePath := ""
	if len(req.Sources) == 1 {
		sourcePath = strings.TrimSpace(req.Sources[0])
		if strings.HasPrefix(sourcePath, "att:") || strings.HasPrefix(sourcePath, "out:") || !filepath.IsAbs(sourcePath) {
			return nil, true, fmt.Errorf("styled native %s convert requires a local workspace source file", target)
		}
	}

	if err := prepareNativeOfficeConvertSourceArgs(ctx, mergedArgs, target, sourcePath); err != nil {
		return nil, true, err
	}

	data, err := buildNativeOfficeConvertBytes(ctx, target, mergedArgs)
	if err != nil {
		return nil, true, err
	}

	outputPath, err := resolveNativeOfficeOutputPath(target, sourcePath, args)
	if err != nil {
		return nil, true, err
	}
	createDirs, err := parseCreateDirsArg(mergedArgs)
	if err != nil {
		return nil, true, err
	}
	absPath, _, err := executeCreateLikeDocumentWrite(ctx, target, t.pathScope(), outputPath, createDirs, data)
	if err != nil {
		return nil, true, err
	}

	previewKind := convertpkg.PreviewFile
	if target == "pdf" {
		previewKind = convertpkg.PreviewPDF
	}
	task := &convertpkg.ConvertTask{
		Action:       convertpkg.ActionConvert,
		Status:       convertpkg.StatusSucceeded,
		Message:      "Document converted",
		TargetFormat: target,
		Outputs: []convertpkg.ConvertOutput{{
			ID:          "native-office-output",
			Name:        filepath.Base(absPath),
			MimeType:    firstNonEmptyConvertValue(mime.TypeByExtension(filepath.Ext(absPath)), convertMimeTypeForFormat(target)),
			SizeBytes:   int64(len(data)),
			PreviewKind: previewKind,
			Path:        absPath,
		}},
	}
	return simpleConvertTaskResult(task, false), true, nil
}

func resolveNativeOfficeOutputPath(target, sourcePath string, args map[string]interface{}) (string, error) {
	outputPath := strings.TrimSpace(firstCompatString(args, "output_path", "outputPath", "destination_path", "destinationPath", "output", "destination", "dest", "to"))
	if outputPath != "" {
		return outputPath, nil
	}

	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", errors.New("output_path is required for native office convert")
	}

	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if base == "" || base == "." {
		base = "converted"
	}
	target = normalizeConvertFormatName(target)
	outputPath = filepath.Join(filepath.Dir(sourcePath), base+"."+target)
	if strings.EqualFold(filepath.Clean(outputPath), filepath.Clean(sourcePath)) {
		outputPath = filepath.Join(filepath.Dir(sourcePath), base+"_converted."+target)
	}
	return outputPath, nil
}

func isNativeOfficeConvertTarget(target string) bool {
	switch normalizeConvertFormatName(target) {
	case "docx", "xlsx", "pptx", "pdf":
		return true
	default:
		return false
	}
}

func buildNativeOfficeConvertArgs(args map[string]interface{}, target string) (map[string]interface{}, bool) {
	merged := cloneConvertArgs(args)
	rawOptions, ok := compatArgValue(args, "options")
	if ok {
		if options, ok := coerceCompatMap(rawOptions); ok {
			for _, key := range nativeOfficeOptionKeysForTarget(target) {
				if nested, ok := coerceCompatMap(options[key]); ok {
					mergeMissingConvertArgs(merged, nested)
				}
			}
		}
	}
	return merged, hasNativeOfficeConvertInputs(merged, target)
}

func nativeOfficeOptionKeysForTarget(target string) []string {
	keys := []string{"office"}
	switch normalizeConvertFormatName(target) {
	case "docx":
		return append(keys, "docx", "document")
	case "xlsx":
		return append(keys, "xlsx", "spreadsheet", "workbook")
	case "pdf":
		return append(keys, "pdf", "document")
	case "pptx":
		return append(keys, "presentation", "pptx")
	default:
		return keys
	}
}

func cloneConvertArgs(args map[string]interface{}) map[string]interface{} {
	cloned := make(map[string]interface{}, len(args))
	for key, value := range args {
		cloned[key] = value
	}
	return cloned
}

func mergeMissingConvertArgs(dst, src map[string]interface{}) {
	for key, value := range src {
		if _, exists := dst[key]; !exists {
			dst[key] = value
		}
	}
}

func hasNativeOfficeConvertInputs(args map[string]interface{}, target string) bool {
	keys := []string{
		"theme", "style_hint", "styleHint", "title", "subtitle", "summary",
		"content", "markdown", "body", "text", "notes",
	}
	switch normalizeConvertFormatName(target) {
	case "xlsx":
		keys = append(keys, "sheets", "sheet", "columns", "headers", "rows", "table")
	default:
		keys = append(keys, "sections", "paragraphs")
	}
	for _, key := range keys {
		if value, ok := compatArgValue(args, key); ok && value != nil {
			if text, ok := value.(string); ok {
				if strings.TrimSpace(text) == "" {
					continue
				}
			}
			return true
		}
	}
	return false
}

func prepareNativeOfficeConvertSourceArgs(ctx context.Context, args map[string]interface{}, target, sourcePath string) error {
	if nativeOfficeHasContentArgs(args, target) {
		return nil
	}
	if strings.TrimSpace(sourcePath) == "" {
		return fmt.Errorf("styled native %s convert requires a source file or explicit structured content", target)
	}

	sourceExt := normalizeConvertFormatName(filepath.Ext(sourcePath))
	switch normalizeConvertFormatName(target) {
	case "xlsx":
		switch {
		case sourceExt == "csv" || sourceExt == "tsv":
			rows, err := readNativeOfficeDelimitedRows(sourcePath)
			if err != nil {
				return err
			}
			populateNativeOfficeWorkbookArgsFromRows(args, rows)
			return nil
		case isNativeOfficeTextSource(sourceExt):
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return err
			}
			args["content"] = string(content)
			return nil
		default:
			return fmt.Errorf("styled native xlsx convert currently supports markdown/text/html/csv/tsv sources or explicit workbook content")
		}
	default:
		switch {
		case sourceExt == "csv" || sourceExt == "tsv":
			rows, err := readNativeOfficeDelimitedRows(sourcePath)
			if err != nil {
				return err
			}
			args["content"] = nativeOfficeMarkdownTableFromRows(rows)
			return nil
		case isNativeOfficeTextSource(sourceExt):
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return err
			}
			args["content"] = string(content)
			return nil
		case convertpkg.SupportsDocumentReadFormat(sourceExt):
			reader := convertpkg.NewDocumentReader()
			doc, err := reader.ReadDocument(ctx, sourcePath)
			if err != nil {
				return err
			}
			args["content"] = doc.Text
			return nil
		default:
			return fmt.Errorf("styled native %s convert currently supports markdown/text/html/csv/tsv or readable office sources", target)
		}
	}
}

func nativeOfficeHasContentArgs(args map[string]interface{}, target string) bool {
	keys := []string{"content", "markdown", "body", "text"}
	switch normalizeConvertFormatName(target) {
	case "xlsx":
		keys = append(keys, "sheets", "sheet", "columns", "headers", "rows", "table")
	default:
		keys = append(keys, "sections", "paragraphs")
	}
	for _, key := range keys {
		if value, ok := compatArgValue(args, key); ok && value != nil {
			if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
				continue
			}
			return true
		}
	}
	return false
}

func isNativeOfficeTextSource(ext string) bool {
	switch normalizeConvertFormatName(ext) {
	case "txt", "text", "md", "markdown", "html", "htm":
		return true
	default:
		return false
	}
}

func readNativeOfficeDelimitedRows(path string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	if normalizeConvertFormatName(filepath.Ext(path)) == "tsv" {
		reader.Comma = '\t'
	}
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("source table contained no rows")
	}
	return rows, nil
}

func populateNativeOfficeWorkbookArgsFromRows(args map[string]interface{}, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	columns := make([]interface{}, 0, len(rows[0]))
	dataRows := make([][]string, 0)
	if len(rows) > 1 {
		for _, header := range rows[0] {
			columns = append(columns, header)
		}
		dataRows = rows[1:]
	} else {
		for idx := range rows[0] {
			columns = append(columns, fmt.Sprintf("Column %d", idx+1))
		}
		dataRows = rows
	}

	normalizedRows := make([]interface{}, 0, len(dataRows))
	for _, row := range dataRows {
		items := make([]interface{}, len(row))
		for idx, value := range row {
			items[idx] = value
		}
		normalizedRows = append(normalizedRows, items)
	}
	args["columns"] = columns
	args["rows"] = normalizedRows
}

func nativeOfficeMarkdownTableFromRows(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	headers := append([]string(nil), rows[0]...)
	dataRows := rows[1:]
	if len(dataRows) == 0 {
		headers = make([]string, len(rows[0]))
		for idx := range headers {
			headers[idx] = fmt.Sprintf("Column %d", idx+1)
		}
		dataRows = rows
	}

	var sb strings.Builder
	sb.WriteString("| ")
	sb.WriteString(strings.Join(headers, " | "))
	sb.WriteString(" |\n| ")
	separators := make([]string, len(headers))
	for idx := range separators {
		separators[idx] = "---"
	}
	sb.WriteString(strings.Join(separators, " | "))
	sb.WriteString(" |\n")
	for _, row := range dataRows {
		cells := make([]string, len(headers))
		for idx := range headers {
			if idx < len(row) {
				cells[idx] = row[idx]
			}
		}
		sb.WriteString("| ")
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteString(" |\n")
	}
	return sb.String()
}

func buildNativeOfficeConvertBytes(ctx context.Context, target string, args map[string]interface{}) ([]byte, error) {
	styleHint := strings.TrimSpace(firstCompatString(args, "style_hint", "styleHint", "style", "visual_style", "visualStyle"))
	if target == "pptx" && styleHint == "" {
		styleHint = "board presentation"
	}
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), styleHint)
	title := strings.TrimSpace(firstCompatString(args, "title"))
	subtitle := strings.TrimSpace(firstCompatString(args, "subtitle"))

	switch normalizeConvertFormatName(target) {
	case "docx":
		spec, err := parseOfficeDocSpec(args, title, subtitle, theme, styleHint)
		if err != nil {
			return nil, err
		}
		data, _, err := buildOfficeDOCX(spec)
		return data, err
	case "xlsx":
		spec, err := parseOfficeWorkbookSpec(args, title, subtitle, theme)
		if err != nil {
			return nil, err
		}
		data, _, err := buildOfficeXLSX(spec)
		return data, err
	case "pptx":
		spec, err := parseOfficeDocSpec(args, title, subtitle, theme, styleHint)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(spec.Language) == "" {
			if lang := strings.TrimSpace(GetLang(ctx)); lang != "" && !strings.EqualFold(lang, "en-US") {
				spec.Language = lang
			}
		}
		data, _, err := buildOfficePPTX(spec)
		return data, err
	case "pdf":
		spec, err := parseOfficeDocSpec(args, title, subtitle, theme, styleHint)
		if err != nil {
			return nil, errors.New(strings.Replace(err.Error(), "docx", "pdf", 1))
		}
		data, _, err := pdfextract.CreateDocument(nativePDFCreateRequest(spec))
		return data, err
	default:
		return nil, fmt.Errorf("unsupported native office convert target %q", target)
	}
}

func firstNonEmptyConvertValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func convertMimeTypeForFormat(format string) string {
	switch normalizeConvertFormatName(format) {
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "pdf":
		return "application/pdf"
	default:
		return ""
	}
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
	req.Options.Presentation = mergePresentationOptions(req.Options.Presentation, parsePresentationOptions(args))
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
	if presentation, ok := coerceCompatMap(raw["presentation"]); ok {
		out.Presentation = mergePresentationOptions(out.Presentation, parsePresentationOptions(presentation))
	}
	if pptx, ok := coerceCompatMap(raw["pptx"]); ok {
		out.Presentation = mergePresentationOptions(out.Presentation, parsePresentationOptions(pptx))
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

func parsePresentationOptions(raw map[string]interface{}) convertpkg.PresentationOptions {
	var out convertpkg.PresentationOptions
	out.Theme = strings.TrimSpace(firstCompatString(raw, "theme"))
	out.StyleHint = strings.TrimSpace(firstCompatString(raw, "style_hint", "styleHint", "style", "visual_style", "visualStyle"))
	out.Title = strings.TrimSpace(firstCompatString(raw, "title"))
	out.Subtitle = strings.TrimSpace(firstCompatString(raw, "subtitle"))
	if value, ok := compatArgValue(raw, "summary"); ok {
		out.Summary = parsePresentationText(value)
	}
	return out
}

func mergePresentationOptions(base, override convertpkg.PresentationOptions) convertpkg.PresentationOptions {
	if value := strings.TrimSpace(override.Theme); value != "" {
		base.Theme = value
	}
	if value := strings.TrimSpace(override.StyleHint); value != "" {
		base.StyleHint = value
	}
	if value := strings.TrimSpace(override.Title); value != "" {
		base.Title = value
	}
	if value := strings.TrimSpace(override.Subtitle); value != "" {
		base.Subtitle = value
	}
	if value := strings.TrimSpace(override.Summary); value != "" {
		base.Summary = value
	}
	return base
}

func parsePresentationText(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		for _, key := range []string{"text", "body", "content", "summary"} {
			if value, ok := compatArgValue(typed, key); ok {
				if text := parsePresentationText(value); text != "" {
					return text
				}
			}
		}
		return ""
	case []interface{}:
		lines := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := parsePresentationText(item); text != "" {
				lines = append(lines, text)
			}
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	default:
		rendered := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if rendered == "" || rendered == "<nil>" {
			return ""
		}
		return rendered
	}
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
