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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

const maxFileWriteChunkBytes = 32 << 10 // 32 KiB per write call; use append=true for larger files.

// FileReadTool reads content from a file.
type FileReadTool struct {
	// AllowedPaths restricts file access to specific directories.
	// When empty, current working directory is treated as workspace root.
	AllowedPaths []string
	// MaxFileSize is the maximum file size to read (default 2 MiB).
	MaxFileSize int64
	scope       *fsToolScope
}

// NewFileReadTool creates a new file read tool.
func NewFileReadTool(allowedPaths []string, maxFileSize int64) *FileReadTool {
	if maxFileSize <= 0 || maxFileSize > maxFSToolBytes {
		maxFileSize = maxFSToolBytes
	}
	return &FileReadTool{
		AllowedPaths: allowedPaths,
		MaxFileSize:  maxFileSize,
		scope:        newFSToolScope(allowedPaths),
	}
}

// Definition returns the tool's definition.
func (f *FileReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "read",
		Description: "Reads content from a file. Returns the file content as text.",
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
			},
			"required": []string{"path"},
		},
	}
}

// Execute reads the file content.
func (f *FileReadTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	_ = ctx

	path, err := fsAsString(args, "path")
	if err != nil || path == "" {
		return nil, errors.New("path must be a non-empty string")
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

	absPath, relPath, _, err := f.scope.resolvePath(path, false)
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
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
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
	MaxFileSize int64
	scope       *fsToolScope
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

// Definition returns the tool's definition.
func (f *FileWriteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write",
		Description: "Writes content to a file. Creates the file if it doesn't exist, or overwrites if it does. For large files, write the first chunk, then continue with append=true across multiple calls instead of sending one huge payload.",
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
					"description": "The content chunk to write. For large files, split content across multiple calls instead of sending one huge string.",
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
	_ = ctx

	path, err := fsAsString(args, "path")
	if err != nil || path == "" {
		return nil, errors.New("path must be a non-empty string")
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
	if rawLine, ok := args["line"]; ok {
		_ = rawLine
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

	absPath, relPath, _, err := f.scope.resolvePath(path, false)
	if err != nil {
		return nil, err
	}

	// Guard oversized single-call payloads. Large files should be written in
	// chunks so tool-call arguments do not balloon follow-up LLM requests.
	if len(content) > maxFileWriteChunkBytes {
		return nil, fmt.Errorf("content chunk too large: %d bytes (max: %d bytes per write); split into smaller chunks and use append=true for multi-part writes", len(content), maxFileWriteChunkBytes)
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
			"path":    relPath,
			"size":    size,
			"success": true,
			"line":    line,
		}
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
		"path":    relPath,
		"size":    size,
		"success": true,
		"append":  appendMode,
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

// validatePath checks if the path is allowed.
func (f *FileWriteTool) validatePath(path string) error {
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
	registry.Register(NewFileReadTool(nil, 0))
	registry.Register(NewFileWriteTool(nil, 0))
	registry.Register(NewEditTool(nil, 0))
	registry.Register(NewGrepTool(nil, 0))
	registry.Register(NewFindTool(nil))
	registry.Register(NewLsTool(nil))
	registry.Register(NewWebSearchTool(WebSearchConfig{}))
	registry.Register(NewWebFetchTool(WebFetchConfig{}))
	registry.Register(NewWebReadTool(WebFetchConfig{}))
	registry.Register(NewWebExtractTool(WebFetchConfig{}))
	registry.Register(NewWebCrawlTool(WebFetchConfig{}))
	registry.Register(NewMCPTool(registry))
}

// RegisterBuiltinToolsWithConfig registers built-in core tools with custom configuration.
func RegisterBuiltinToolsWithConfig(registry *Registry, webSearchConfig WebSearchConfig, webFetchConfig WebFetchConfig, allowedPaths []string, maxFileSize int64) {
	if registry == nil {
		return
	}
	registry.Register(NewFileReadTool(allowedPaths, maxFileSize))
	registry.Register(NewFileWriteTool(allowedPaths, maxFileSize))
	registry.Register(NewEditTool(allowedPaths, maxFileSize))
	registry.Register(NewGrepTool(allowedPaths, maxFileSize))
	registry.Register(NewFindTool(allowedPaths))
	registry.Register(NewLsTool(allowedPaths))
	registry.Register(NewWebSearchTool(webSearchConfig))
	registry.Register(NewWebFetchTool(webFetchConfig))
	registry.Register(NewWebReadTool(webFetchConfig))
	registry.Register(NewWebExtractTool(webFetchConfig))
	registry.Register(NewWebCrawlTool(webFetchConfig))
	registry.Register(NewMCPTool(registry))
}

// RegisterExecTools registers exec + process tools with shared session state.
func RegisterExecTools(registry *Registry, config ExecConfig, approvals *ApprovalManager, broker *sse.Broker, dirStore *DirAllowlistStore, sbx ...SandboxExecutor) {
	sessions := NewSessionRegistry()
	registry.Register(NewExecTool(config, sessions, approvals, broker, dirStore, sbx...))
	registry.Register(NewProcessTool(sessions))
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

// RegisterMemoryTools creates a disabled unified memory tool for internal use
// and native OpenClaw-style memory_* wrappers for model-visible compatibility.
func RegisterMemoryTools(registry *Registry, memoryService MemoryServiceInterface) {
	if memoryService == nil {
		return
	}
	registry.Register(NewMemoryTool(memoryService))
	registry.Disable("memory")
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
