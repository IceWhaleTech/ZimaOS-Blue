package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type BuiltinRuntimeConfig struct {
	DataDir                     string
	WorkspaceDir                string
	Ripgrep                     config.ToolCallingRipgrepConfig
	SkillDynamicExposureEnabled func() bool
}

// FileReadTool reads content from a file.
type FileReadTool struct {
	// AllowedPaths restricts file access to specific directories.
	// When empty, current working directory is treated as workspace root.
	AllowedPaths []string
	// MaxFileSize is the maximum file size to read (default 2 MiB).
	MaxFileSize    int64
	pdfService     PDFService
	documentReader DocumentReadService
	scope          *fsToolScope
	skillExposure  *skillmanifest.SkillExposureManager
}

// DocumentReadService extracts readable text from local office-style documents.
type DocumentReadService interface {
	ReadDocument(ctx context.Context, path string) (*convertpkg.DocumentReadResult, error)
}

// NewFileReadTool creates a new file read tool.
func NewFileReadTool(allowedPaths []string, maxFileSize int64) *FileReadTool {
	if maxFileSize <= 0 || maxFileSize > maxFSToolBytes {
		maxFileSize = maxFSToolBytes
	}
	return &FileReadTool{
		AllowedPaths:   allowedPaths,
		MaxFileSize:    maxFileSize,
		documentReader: convertpkg.NewDocumentReader(),
		scope:          newFSToolScope(allowedPaths),
	}
}

// SetPDFService enables PDF-aware reads for local PDF files.
func (f *FileReadTool) SetPDFService(service PDFService) {
	if f == nil {
		return
	}
	f.pdfService = service
}

// SetDocumentReadService enables office-style document reads for local files.
func (f *FileReadTool) SetDocumentReadService(service DocumentReadService) {
	if f == nil {
		return
	}
	f.documentReader = service
}

func (f *FileReadTool) SetSkillExposureManager(manager *skillmanifest.SkillExposureManager) {
	if f == nil {
		return
	}
	f.skillExposure = manager
}

// Definition returns the tool's definition.
func (f *FileReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "file_read",
		Description: "Reads content from a local file as UTF-8 text with optional line slicing and byte limits.",
		Icon:        "file-read",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read",
				},
				"start_line": map[string]interface{}{
					"type":        "integer",
					"description": "Optional start line (1-based, default: 1)",
				},
				"end_line": map[string]interface{}{
					"type":        "integer",
					"description": "Optional end line (1-based, inclusive)",
				},
				"max_bytes": map[string]interface{}{
					"type":        "integer",
					"description": "Optional read size cap in bytes (1..2097152)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Optional single 1-based PDF page to extract.",
				},
				"pages": map[string]interface{}{
					"description": "Optional PDF page selection as '1,3-5', a single number, or an array of page numbers.",
				},
				"max_pages": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum PDF pages to extract.",
				},
				"max_chars": map[string]interface{}{
					"type":        "integer",
					"description": "Optional maximum PDF characters to return.",
				},
				"include_pages": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, include per-page PDF text alongside merged text.",
				},
				"ocr": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable OCR fallback for scanned PDFs.",
				},
				"disable_ocr": map[string]interface{}{
					"type":        "boolean",
					"description": "Disable OCR fallback for PDFs.",
				},
				"vision": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable vision fallback for PDFs when available.",
				},
				"disable_vision": map[string]interface{}{
					"type":        "boolean",
					"description": "Disable vision fallback for PDFs.",
				},
			},
			"required": []string{"path"},
		},
	}
}

// Execute reads the file content.
func (f *FileReadTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return nil, errors.New("path must be a non-empty string")
		}
	}
	startLine, err := fsAsInt(args, "start_line", 1)
	if err != nil {
		return nil, err
	}
	endLine, err := fsAsInt(args, "end_line", 0)
	if err != nil {
		return nil, err
	}
	maxBytes, err := fsAsInt(args, "max_bytes", int(f.MaxFileSize))
	if err != nil {
		return nil, err
	}
	maxBytes = fsClamp(maxBytes, 1, int(f.MaxFileSize))

	absPath, relPath, _, err := f.scope.resolvePathWithContext(ctx, "file_read", path, false)
	if err != nil {
		return nil, err
	}

	// Check file info
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", relPath)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, errors.New("path is a directory, not a file")
	}
	f.observeSkillExposure(absPath)

	if f.shouldUseDocumentReader(absPath) {
		return f.executeDocumentRead(ctx, absPath, relPath, info, startLine, endLine, maxBytes)
	}

	if info.Size() > f.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d bytes)", info.Size(), f.MaxFileSize)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	truncated := false
	if len(content) > maxBytes {
		content = content[:maxBytes]
		truncated = true
		for len(content) > 0 && !utf8.Valid(content) {
			content = content[:len(content)-1]
		}
	}
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("file is not valid UTF-8 text: %s", relPath)
	}
	text := string(content)
	if startLine < 1 {
		return nil, errors.New("start_line must be >= 1")
	}
	lines := strings.Split(text, "\n")
	totalLines := len(lines)
	if totalLines == 0 {
		totalLines = 1
	}
	if startLine > totalLines {
		return nil, fmt.Errorf("start_line %d out of range (total lines: %d)", startLine, totalLines)
	}
	from := startLine - 1
	to := totalLines
	if endLine > 0 {
		to = endLine
	}
	if to > totalLines {
		to = totalLines
	}
	if to < startLine {
		return nil, errors.New("end_line must be >= start_line")
	}
	sliced := strings.Join(lines[from:to], "\n")

	response := map[string]interface{}{
		"path":        relPath,
		"size":        info.Size(),
		"start_line":  startLine,
		"end_line":    to,
		"total_lines": totalLines,
		"truncated":   truncated,
		"content":     sliced,
	}
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(absPath), ".")) {
	case "csv", "tsv":
		if summary, summaryErr := convertpkg.SummarizeDelimitedFile(absPath); summaryErr == nil && summary != nil {
			response["tabular_summary"] = summary
		}
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (f *FileReadTool) observeSkillExposure(absPath string) {
	if f == nil || f.skillExposure == nil {
		return
	}
	f.skillExposure.ObserveToolPath("file_read", absPath)
}

func (f *FileReadTool) shouldUseDocumentReader(path string) bool {
	return false
}

func (f *FileReadTool) executePDFRead(ctx context.Context, absPath, relPath string, args map[string]interface{}) (interface{}, error) {
	pdfArgs := make(map[string]interface{}, len(args)+1)
	for key, value := range args {
		pdfArgs[key] = value
	}
	pdfArgs["path"] = absPath

	result, err := NewPDFTool(f.pdfService).Execute(ctx, pdfArgs)
	if err != nil {
		return nil, err
	}

	if payload, ok := result.(map[string]interface{}); ok {
		if _, exists := payload["path"]; !exists {
			payload["path"] = relPath
		}
		return payload, nil
	}
	return result, nil
}

func (f *FileReadTool) executeDocumentRead(ctx context.Context, absPath, relPath string, info os.FileInfo, startLine, endLine, maxBytes int) (interface{}, error) {
	if f.documentReader == nil {
		return nil, fmt.Errorf("file_read does not support %s on this runtime: document extraction is unavailable", strings.TrimPrefix(strings.ToLower(filepath.Ext(absPath)), "."))
	}
	result, err := f.documentReader.ReadDocument(ctx, absPath)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("document extraction returned no result for %s", relPath)
	}

	contentBytes := []byte(result.Text)
	truncated := false
	if len(contentBytes) > maxBytes {
		contentBytes = contentBytes[:maxBytes]
		truncated = true
		for len(contentBytes) > 0 && !utf8.Valid(contentBytes) {
			contentBytes = contentBytes[:len(contentBytes)-1]
		}
	}
	if !utf8.Valid(contentBytes) {
		return nil, fmt.Errorf("document text is not valid UTF-8: %s", relPath)
	}
	content, rangeEnd, totalLines, err := sliceReadContentByLines(string(contentBytes), startLine, endLine)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"path":            relPath,
		"size":            info.Size(),
		"start_line":      startLine,
		"end_line":        rangeEnd,
		"total_lines":     totalLines,
		"truncated":       truncated,
		"content":         content,
		"document_format": result.Format,
	}
	if strings.TrimSpace(result.ExtractedVia) != "" {
		response["extracted_via"] = result.ExtractedVia
	}
	if result.TabularSummary != nil {
		response["tabular_summary"] = result.TabularSummary
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func sliceReadContentByLines(text string, startLine, endLine int) (string, int, int, error) {
	if startLine < 1 {
		return "", 0, 0, errors.New("start_line must be >= 1")
	}
	lines := strings.Split(text, "\n")
	totalLines := len(lines)
	if totalLines == 0 {
		totalLines = 1
	}
	if startLine > totalLines {
		return "", 0, totalLines, fmt.Errorf("start_line %d out of range (total lines: %d)", startLine, totalLines)
	}
	from := startLine - 1
	to := totalLines
	if endLine > 0 {
		to = endLine
	}
	if to > totalLines {
		to = totalLines
	}
	if to < startLine {
		return "", 0, totalLines, errors.New("end_line must be >= start_line")
	}
	return strings.Join(lines[from:to], "\n"), to, totalLines, nil
}

// validatePath checks if the path is allowed.
func (f *FileReadTool) validatePath(path string) error {
	_, _, _, err := f.scope.resolvePath(path, false)
	return err
}

// FileWriteTool writes content to a file.
type FileWriteTool struct {
	// AllowedPaths restricts file access to specific directories.
	AllowedPaths []string
	// MaxFileSize is the maximum file size to write (default 2 MiB).
	MaxFileSize   int64
	scope         *fsToolScope
	skillExposure *skillmanifest.SkillExposureManager
}

// NewFileWriteTool creates a new file write tool.
func NewFileWriteTool(allowedPaths []string, maxFileSize int64) *FileWriteTool {
	if maxFileSize <= 0 || maxFileSize > maxFSToolBytes {
		maxFileSize = maxFSToolBytes
	}
	return &FileWriteTool{
		AllowedPaths: allowedPaths,
		MaxFileSize:  maxFileSize,
		scope:        newFSToolScope(allowedPaths),
	}
}

func (f *FileWriteTool) SetSkillExposureManager(manager *skillmanifest.SkillExposureManager) {
	if f == nil {
		return
	}
	f.skillExposure = manager
}

// Definition returns the tool's definition.
func (f *FileWriteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "file_write",
		Description: "Writes content to a file. Creates the file if it doesn't exist, or overwrites if it does. Keep each write at or below 200 lines and 64 KiB. For larger files, prefer write_begin/write_chunk/write_commit; otherwise write the first chunk, then continue with append=true across multiple calls instead of sending one huge payload.",
		Icon:        "file-write",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content chunk to write. Keep each call at or below 200 lines and 64 KiB. For large files, split content across multiple calls instead of sending one huge string.",
				},
				"append": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, append this chunk to the file instead of overwriting (default: false). Use this for multi-part writes.",
				},
				"create_dirs": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, create parent directories when missing (default: true)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Replace a 1-based line in-place (append must be false; content may span multiple lines)",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

// Execute writes content to the file.
func (f *FileWriteTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return nil, errors.New("path must be a non-empty string")
		}
	}

	content, err := fsAsTextContent(args, "content")
	if err != nil {
		return nil, errors.New("content must be text-compatible (string, number, boolean, object, or array)")
	}

	appendMode, err := fsAsBool(args, "append", false)
	if err != nil {
		return nil, err
	}
	createDirs, err := fsAsBool(args, "create_dirs", true)
	if err != nil {
		return nil, err
	}
	line, hasLine := 0, false
	if _, ok := firstCompatValueDeep(args, "line"); ok {
		line, err = fsAsInt(args, "line", 0)
		if err != nil {
			return nil, err
		}
		hasLine = true
	}
	if hasLine && appendMode {
		return nil, errors.New("line mode cannot be used with append=true")
	}
	if hasLine && line < 1 {
		return nil, errors.New("line must be >= 1")
	}

	originalPath := path
	absPath, relPath, _, err := f.scope.resolvePathWithContext(ctx, "file_write", path, false)
	if err != nil {
		return nil, err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return nil, err
	}

	// Guard oversized single-call payloads. Large files should be written in
	// chunks so tool-call arguments do not balloon follow-up LLM requests.
	if err := validateWriteContentChunk(content, "write", "for very large files use write_begin/write_chunk/write_commit, or split into smaller chunks and use append=true for multi-part writes"); err != nil {
		return nil, err
	}

	// Check content size
	if int64(len(content)) > f.MaxFileSize {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", len(content), f.MaxFileSize)
	}

	// Ensure directory exists when requested.
	if createDirs {
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if hasLine {
		updatedContent, err := f.replaceSingleLine(absPath, line, content)
		if err != nil {
			return nil, err
		}
		if int64(len(updatedContent)) > f.MaxFileSize {
			return nil, fmt.Errorf("result too large: %d bytes (max: %d bytes)", len(updatedContent), f.MaxFileSize)
		}
		if err := os.WriteFile(absPath, []byte(updatedContent), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		info, _ := os.Stat(absPath)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		response := map[string]interface{}{
			"path":          relPath,
			"absolute_path": absPath,
			"original_path": originalPath,
			"size":          size,
			"success":       true,
			"line":          line,
		}
		f.observeSkillExposure(absPath)
		jsonResult, _ := json.Marshal(response)
		return string(jsonResult), nil
	}

	// Write file
	if appendMode {
		file, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		if _, err = file.WriteString(content); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	} else {
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}

	// Get file info after write
	info, _ := os.Stat(absPath)
	var size int64
	if info != nil {
		size = info.Size()
	}

	response := map[string]interface{}{
		"path":          relPath,
		"absolute_path": absPath,
		"original_path": originalPath,
		"size":          size,
		"success":       true,
		"append":        appendMode,
	}
	f.observeSkillExposure(absPath)
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (f *FileWriteTool) observeSkillExposure(absPath string) {
	if f == nil || f.skillExposure == nil {
		return
	}
	f.skillExposure.ObserveToolPath("file_write", absPath)
}

// validatePath checks if the path is allowed.
func (f *FileWriteTool) validatePath(path string) error {
	_, _, _, err := f.scope.resolvePath(path, false)
	return err
}

type FileDeleteTool struct {
	AllowedPaths []string
	scope        *fsToolScope
}

func NewFileDeleteTool(allowedPaths []string) *FileDeleteTool {
	return &FileDeleteTool{
		AllowedPaths: allowedPaths,
		scope:        newFSToolScope(allowedPaths),
	}
}

func (f *FileDeleteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "file_delete",
		Description: "Deletes a local file or directory. For directories, set recursive=true to remove non-empty contents.",
		Icon:        "trash",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file or directory to delete",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, delete directories recursively (default: false)",
				},
				"missing_ok": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, treat a missing path as a successful no-op (default: false)",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (f *FileDeleteTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return nil, errors.New("path must be a non-empty string")
		}
	}

	recursive, err := fsAsBool(args, "recursive", false)
	if err != nil {
		return nil, err
	}
	missingOK, err := fsAsBool(args, "missing_ok", false)
	if err != nil {
		return nil, err
	}

	absPath, relPath, _, err := f.scope.resolvePathWithContext(ctx, "file_delete", path, false)
	if err != nil {
		return nil, err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) && missingOK {
			response := map[string]interface{}{
				"path":       relPath,
				"success":    true,
				"deleted":    false,
				"missing_ok": true,
			}
			jsonResult, _ := json.Marshal(response)
			return string(jsonResult), nil
		}
		return nil, err
	}

	kind := "file"
	if info.IsDir() {
		kind = "directory"
	}

	if info.IsDir() && !recursive {
		if err := os.Remove(absPath); err != nil {
			return nil, fmt.Errorf("failed to delete directory %q without recursive=true: %w", relPath, err)
		}
	} else if recursive {
		if err := os.RemoveAll(absPath); err != nil {
			return nil, fmt.Errorf("failed to delete %s: %w", relPath, err)
		}
	} else {
		if err := os.Remove(absPath); err != nil {
			return nil, fmt.Errorf("failed to delete file %q: %w", relPath, err)
		}
	}

	response := map[string]interface{}{
		"path":      relPath,
		"success":   true,
		"deleted":   true,
		"kind":      kind,
		"recursive": recursive,
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func (f *FileDeleteTool) validatePath(path string) error {
	_, _, _, err := f.scope.resolvePath(path, false)
	return err
}

func (f *FileWriteTool) replaceSingleLine(path string, line int, content string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if line == 1 {
				return content, nil
			}
			return "", fmt.Errorf("line %d out of range (total lines: %d)", line, 0)
		}
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text: %s", path)
	}
	text := string(data)
	hasTrailingNewline := strings.HasSuffix(text, "\n")
	if hasTrailingNewline {
		text = strings.TrimSuffix(text, "\n")
	}
	lines := []string{}
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	totalLines := len(lines)
	if line > totalLines {
		if totalLines == 0 && line == 1 {
			lines = []string{content}
		} else {
			return "", fmt.Errorf("line %d out of range (total lines: %d)", line, totalLines)
		}
	} else {
		lines[line-1] = content
	}
	out := strings.Join(lines, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	return out, nil
}

// RegisterBuiltinTools registers built-in core tools with default configuration.
func RegisterBuiltinTools(registry *Registry) {
	if registry == nil {
		return
	}
	writeSessions := NewWriteSessionManager(0)
	registry.Register(NewFileReadTool(nil, 0))
	registry.Register(NewFileWriteTool(nil, 0))
	registry.Register(NewFileDeleteTool(nil))
	registry.Register(NewFileWriteBeginTool(nil, writeSessions))
	registry.Register(NewFileWriteChunkTool(writeSessions))
	registry.Register(NewFileWriteCommitTool(writeSessions))
	registry.Register(NewFileWriteAbortTool(writeSessions))
	registry.Register(NewEditTool(nil, 0))
	registry.Register(NewGrepTool(nil, 0))
	registry.Register(NewRgTool(nil, 0))
	registry.Register(NewFindTool(nil))
	registry.Register(NewLsTool(nil))
	registry.Register(NewDOCXTool(nil, nil, nil))
	registry.Register(NewXLSXTool(nil, nil, nil))
	registry.Register(NewPPTXTool(nil, nil, nil))
	registerCanonicalFileSurface(registry)
	registry.Register(NewToolSearchTool(registry))
	registerWebTools(registry, WebSearchConfig{}, WebFetchConfig{})
	registry.Register(NewMCPTool(registry))
}

// RegisterBuiltinToolsWithConfig registers built-in core tools with custom configuration.
func RegisterBuiltinToolsWithConfig(registry *Registry, webSearchConfig WebSearchConfig, webFetchConfig WebFetchConfig, allowedPaths []string, maxFileSize int64) {
	RegisterBuiltinToolsWithRuntimeConfig(registry, webSearchConfig, webFetchConfig, allowedPaths, maxFileSize, BuiltinRuntimeConfig{})
}

// RegisterBuiltinToolsWithRuntimeConfig registers built-in tools with runtime resource configuration.
func RegisterBuiltinToolsWithRuntimeConfig(registry *Registry, webSearchConfig WebSearchConfig, webFetchConfig WebFetchConfig, allowedPaths []string, maxFileSize int64, runtimeCfg BuiltinRuntimeConfig) {
	if registry == nil {
		return
	}
	ripgrep := newBuiltinRipgrepResolver(runtimeCfg)
	writeSessions := NewWriteSessionManager(maxFileSize)
	skillExposure := builtinSkillExposureManager(runtimeCfg)
	read := NewFileReadTool(allowedPaths, maxFileSize)
	read.SetSkillExposureManager(skillExposure)
	registry.Register(read)
	write := NewFileWriteTool(allowedPaths, maxFileSize)
	write.SetSkillExposureManager(skillExposure)
	registry.Register(write)
	registry.Register(NewFileDeleteTool(allowedPaths))
	registry.Register(NewFileWriteBeginTool(allowedPaths, writeSessions))
	registry.Register(NewFileWriteChunkTool(writeSessions))
	registry.Register(NewFileWriteCommitTool(writeSessions))
	registry.Register(NewFileWriteAbortTool(writeSessions))
	edit := NewEditTool(allowedPaths, maxFileSize)
	edit.SetSkillExposureManager(skillExposure)
	registry.Register(edit)
	registry.Register(NewGrepToolWithRipgrep(allowedPaths, maxFileSize, ripgrep))
	registry.Register(NewRgToolWithRipgrep(allowedPaths, maxFileSize, ripgrep))
	registry.Register(NewFindToolWithRipgrep(allowedPaths, ripgrep))
	registry.Register(NewLsTool(allowedPaths))
	registry.Register(NewDOCXTool(allowedPaths, nil, nil))
	registry.Register(NewXLSXTool(allowedPaths, nil, nil))
	registry.Register(NewPPTXTool(allowedPaths, nil, nil))
	registerCanonicalFileSurface(registry)
	toolSearch := NewToolSearchTool(registry)
	toolSearch.SetSkillExposureManager(skillExposure)
	registry.Register(toolSearch)
	registerWebTools(registry, webSearchConfig, webFetchConfig)
	registry.Register(NewMCPTool(registry))
}

// RegisterApprovalAwareFileTools re-registers filesystem tools with the same
// scope as the default builtins plus exec-style approval handling for
// out-of-scope absolute paths.
func RegisterApprovalAwareFileTools(registry *Registry, allowedPaths []string, maxFileSize int64, approvals *ApprovalManager, dirStore *DirAllowlistStore) {
	RegisterApprovalAwareFileToolsWithRuntimeConfig(registry, allowedPaths, maxFileSize, approvals, dirStore, BuiltinRuntimeConfig{})
}

func RegisterApprovalAwareFileToolsWithRuntimeConfig(registry *Registry, allowedPaths []string, maxFileSize int64, approvals *ApprovalManager, dirStore *DirAllowlistStore, runtimeCfg BuiltinRuntimeConfig) {
	if registry == nil {
		return
	}
	ripgrep := newBuiltinRipgrepResolver(runtimeCfg)
	skillExposure := builtinSkillExposureManager(runtimeCfg)
	var (
		pdfService     PDFService
		documentReader DocumentReadService
	)
	if existing := registry.Get("file_read"); existing != nil {
		if prior, ok := existing.(*FileReadTool); ok {
			pdfService = prior.pdfService
			documentReader = prior.documentReader
		}
	}
	read := NewFileReadTool(allowedPaths, maxFileSize)
	read.scope = read.scope.withApprovalFlow(approvals, dirStore)
	read.SetSkillExposureManager(skillExposure)
	if pdfService != nil {
		read.SetPDFService(pdfService)
	}
	if documentReader != nil {
		read.SetDocumentReadService(documentReader)
	}
	registry.Register(read)

	write := NewFileWriteTool(allowedPaths, maxFileSize)
	write.scope = write.scope.withApprovalFlow(approvals, dirStore)
	write.SetSkillExposureManager(skillExposure)
	registry.Register(write)

	del := NewFileDeleteTool(allowedPaths)
	del.scope = del.scope.withApprovalFlow(approvals, dirStore)
	registry.Register(del)

	writeSessions := NewWriteSessionManager(maxFileSize)
	writeBegin := NewFileWriteBeginTool(allowedPaths, writeSessions)
	writeBegin.scope = writeBegin.scope.withApprovalFlow(approvals, dirStore)
	registry.Register(writeBegin)
	registry.Register(NewFileWriteChunkTool(writeSessions))
	registry.Register(NewFileWriteCommitTool(writeSessions))
	registry.Register(NewFileWriteAbortTool(writeSessions))

	edit := NewEditTool(allowedPaths, maxFileSize)
	edit.Scope = edit.Scope.withApprovalFlow(approvals, dirStore)
	edit.SetSkillExposureManager(skillExposure)
	registry.Register(edit)

	grep := NewGrepToolWithRipgrep(allowedPaths, maxFileSize, ripgrep)
	grep.Scope = grep.Scope.withApprovalFlow(approvals, dirStore)
	registry.Register(grep)

	rg := NewRgToolWithRipgrep(allowedPaths, maxFileSize, ripgrep)
	rg.Scope = rg.Scope.withApprovalFlow(approvals, dirStore)
	registry.Register(rg)

	find := NewFindToolWithRipgrep(allowedPaths, ripgrep)
	find.Scope = find.Scope.withApprovalFlow(approvals, dirStore)
	registry.Register(find)

	ls := NewLsTool(allowedPaths)
	ls.Scope = ls.Scope.withApprovalFlow(approvals, dirStore)
	registry.Register(ls)

	registry.Register(NewDOCXTool(allowedPaths, approvals, dirStore))
	registry.Register(NewXLSXTool(allowedPaths, approvals, dirStore))
	registry.Register(NewPPTXTool(allowedPaths, approvals, dirStore))
	registerCanonicalFileSurface(registry)
}

func newBuiltinRipgrepResolver(runtimeCfg BuiltinRuntimeConfig) ripgrepResolver {
	if !runtimeCfg.Ripgrep.Enabled {
		return nil
	}
	manager, err := NewRipgrepManager(runtimeCfg.Ripgrep, runtimeCfg.DataDir)
	if err != nil {
		return nil
	}
	return manager
}

func builtinSkillExposureManager(runtimeCfg BuiltinRuntimeConfig) *skillmanifest.SkillExposureManager {
	workspaceDir := strings.TrimSpace(runtimeCfg.WorkspaceDir)
	if workspaceDir == "" {
		return nil
	}
	manager := skillmanifest.SharedSkillExposureManager(workspaceDir)
	manager.SetDynamicExposureEnabledFunc(runtimeCfg.SkillDynamicExposureEnabled)
	return manager
}

// RegisterExecTools registers exec with shared session state.
func RegisterExecTools(registry *Registry, config ExecConfig, approvals *ApprovalManager, broker *sse.Broker, dirStore *DirAllowlistStore, sbx ...SandboxExecutor) {
	sessions := NewSessionRegistry()
	registry.Register(NewExecTool(config, sessions, approvals, broker, dirStore, sbx...))
	registerCanonicalShellSurface(registry)
}

// GetUIReviewerTool retrieves the UIReviewerTool from the registry for dependency injection.
func GetUIReviewerTool(registry *Registry) *UIReviewerTool {
	tool := registry.Get("ui_reviewer")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*UIReviewerTool); ok {
		return t
	}
	return nil
}

// GetExecTool retrieves the ExecTool from the registry for dependency injection.
func GetExecTool(registry *Registry) *ExecTool {
	tool := registry.Get("exec")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*ExecTool); ok {
		return t
	}
	return nil
}

// GetWebFetchTool retrieves the WebFetchTool from the registry for dependency injection.
func GetWebFetchTool(registry *Registry) *WebFetchTool {
	tool := registry.Get("web_fetch")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*WebFetchTool); ok {
		return t
	}
	return nil
}

// GetToolSearchTool retrieves the ToolSearchTool from the registry for dependency injection.
func GetToolSearchTool(registry *Registry) *ToolSearchTool {
	tool := registry.Get("tool_search")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*ToolSearchTool); ok {
		return t
	}
	return nil
}

func GetWebQueryTool(registry *Registry) *WebTool {
	tool := registry.Get("web_query")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*WebTool); ok {
		return t
	}
	return nil
}

func GetWebTool(registry *Registry) *WebTool {
	return GetWebQueryTool(registry)
}

func GetWebReadTool(registry *Registry) *WebReadTool {
	tool := registry.Get("web_read")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*WebReadTool); ok {
		return t
	}
	return nil
}

func GetWebExtractTool(registry *Registry) *WebExtractTool {
	tool := registry.Get("web_extract")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*WebExtractTool); ok {
		return t
	}
	return nil
}

func GetWebCrawlTool(registry *Registry) *WebCrawlTool {
	tool := registry.Get("web_crawl")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*WebCrawlTool); ok {
		return t
	}
	return nil
}

func AttachPDFServiceToWebTools(registry *Registry, service PDFService) {
	if registry == nil || service == nil {
		return
	}
	if tool := registry.Get("file_read"); tool != nil {
		if t, ok := tool.(*FileReadTool); ok {
			t.SetPDFService(service)
		}
	}
	if tool := GetWebTool(registry); tool != nil {
		tool.SetPDFService(service)
	}
	if tool := GetWebFetchTool(registry); tool != nil {
		tool.SetPDFService(service)
	}
	if tool := GetWebReadTool(registry); tool != nil {
		tool.SetPDFService(service)
	}
	if tool := GetWebExtractTool(registry); tool != nil {
		tool.SetPDFService(service)
	}
	if tool := GetWebCrawlTool(registry); tool != nil {
		tool.SetPDFService(service)
	}
}

func AttachDocumentReadServiceToWebTools(registry *Registry, service DocumentReadService) {
	if registry == nil || service == nil {
		return
	}
	if tool := registry.Get("file_read"); tool != nil {
		if t, ok := tool.(*FileReadTool); ok {
			t.SetDocumentReadService(service)
		}
	}
	if tool := GetWebTool(registry); tool != nil {
		tool.SetDocumentReadService(service)
	}
	if tool := GetWebFetchTool(registry); tool != nil {
		tool.SetDocumentReadService(service)
	}
	if tool := GetWebReadTool(registry); tool != nil {
		tool.SetDocumentReadService(service)
	}
}

func AttachSTTServiceToWebTools(registry *Registry, service stt.Service) {
	if registry == nil || service == nil {
		return
	}
	if tool := GetWebTool(registry); tool != nil {
		tool.SetSTTService(service)
	}
}

func registerWebTools(registry *Registry, webSearchConfig WebSearchConfig, webFetchConfig WebFetchConfig) {
	if registry == nil {
		return
	}
	searchTool := NewWebSearchTool(webSearchConfig)
	fetchTool := NewWebFetchTool(webFetchConfig)
	readTool := NewWebReadTool(webFetchConfig)
	extractTool := NewWebExtractTool(webFetchConfig)
	crawlTool := NewWebCrawlTool(webFetchConfig)
	webQueryTool := NewWebQueryTool(searchTool, fetchTool, readTool, extractTool, crawlTool)
	if imageTool := registry.Get("image"); imageTool != nil {
		webQueryTool.SetImageTool(imageTool)
	}
	registry.Register(searchTool)
	registry.Register(fetchTool)
	registry.Register(readTool)
	registry.Register(extractTool)
	registry.Register(crawlTool)
	registry.Register(webQueryTool)
	for _, name := range []string{"web_search", "web_fetch", "web_read", "web_extract", "web_crawl"} {
		registry.Disable(name)
	}
}

// RegisterMemoryTools registers the unified memory tool plus legacy
// memory_* wrappers for compatibility.
func RegisterMemoryTools(registry *Registry, memoryService MemoryServiceInterface) {
	if memoryService == nil {
		return
	}
	registry.Register(NewMemoryTool(memoryService))
	RegisterMemoryCompatTools(registry, memoryService)
}

// GetMemoryTool retrieves the MemoryTool from the registry for dependency injection.
func GetMemoryTool(registry *Registry) *MemoryTool {
	tool := registry.Get("memory")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*MemoryTool); ok {
		return t
	}
	return nil
}

// MemoryServiceInterface defines the interface for memory service used by tools.
// This avoids circular imports with the memory package.
type MemoryServiceInterface interface {
	Recall(ctx context.Context, query string, limit int) ([]MemorySearchResult, error)
	Get(ctx context.Context, id string) (*MemoryChunkResult, error)
	Stats(ctx context.Context) (*MemoryStatsResult, error)
	Remember(ctx context.Context, content string, tags []string) (*MemoryChunkResult, error)
	Forget(ctx context.Context, id string) error
	GetActiveBackend() string
}

// MemorySearchResult represents a search result from memory service.
type MemorySearchResult struct {
	Chunk         MemoryChunkResult
	KeywordScore  float32
	CombinedScore float32
	MatchTypes    []string
}

// MemoryChunkResult represents a memory chunk.
type MemoryChunkResult struct {
	ID        string
	Content   string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MemoryStatsResult represents memory statistics.
type MemoryStatsResult struct {
	TotalChunks    int
	TotalSizeBytes int64
	OldestChunk    string
	NewestChunk    string
	Backend        string
}

// RegisterAnalyzeTool registers the analyze tool with the registry.
func RegisterAnalyzeTool(registry *Registry, mediaDir string) *AnalyzeTool {
	t := NewAnalyzeTool()
	t.SetMediaDir(mediaDir)
	registry.Register(t)
	return t
}

// GetAnalyzeTool retrieves the AnalyzeTool from the registry for dependency injection.
func GetAnalyzeTool(registry *Registry) *AnalyzeTool {
	tool := registry.Get("analyze")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*AnalyzeTool); ok {
		return t
	}
	return nil
}

// RegisterAdvisorTool registers the advisor tool with the registry.
func RegisterAdvisorTool(registry *Registry) *AdvisorTool {
	t := NewAdvisorTool()
	if registry != nil {
		registry.Register(t)
	}
	return t
}

// GetAdvisorTool retrieves the AdvisorTool from the registry for dependency injection.
func GetAdvisorTool(registry *Registry) *AdvisorTool {
	if registry == nil {
		return nil
	}
	tool := registry.Get("advisor")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*AdvisorTool); ok {
		return t
	}
	return nil
}
