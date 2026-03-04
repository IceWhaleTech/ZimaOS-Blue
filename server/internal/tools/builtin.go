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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

// FileReadTool reads content from a file.
type FileReadTool struct {
	// AllowedPaths restricts file access to specific directories.
	// If empty, all paths are allowed (use with caution).
	AllowedPaths []string
	// MaxFileSize is the maximum file size to read (default 1MB).
	MaxFileSize int64
}

// NewFileReadTool creates a new file read tool.
func NewFileReadTool(allowedPaths []string, maxFileSize int64) *FileReadTool {
	if maxFileSize <= 0 {
		maxFileSize = 1024 * 1024 // 1MB default
	}
	return &FileReadTool{
		AllowedPaths: allowedPaths,
		MaxFileSize:  maxFileSize,
	}
}

// Definition returns the tool's definition.
func (f *FileReadTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "file_read",
		Description: "Reads content from a file. Returns the file content as text.",
		Icon:        "file-read",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read",
				},
				"encoding": map[string]interface{}{
					"type":        "string",
					"description": "The encoding of the file (default: utf-8)",
				},
			},
			"required": []string{"path"},
		},
	}
}

// Execute reads the file content.
func (f *FileReadTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, errors.New("path is required")
	}

	// Security: validate path
	if err := f.validatePath(path); err != nil {
		return nil, err
	}

	// Check file info
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
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
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	response := map[string]interface{}{
		"path":    path,
		"size":    info.Size(),
		"content": string(content),
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

// validatePath checks if the path is allowed.
func (f *FileReadTool) validatePath(path string) error {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)

	// If no allowed paths are configured, allow all
	if len(f.AllowedPaths) == 0 {
		return nil
	}

	// Check if path is within allowed directories
	for _, allowed := range f.AllowedPaths {
		allowedClean := filepath.Clean(allowed)
		if strings.HasPrefix(cleanPath, allowedClean) {
			return nil
		}
	}

	return fmt.Errorf("access denied: path %s is not in allowed directories", path)
}

// FileWriteTool writes content to a file.
type FileWriteTool struct {
	// AllowedPaths restricts file access to specific directories.
	AllowedPaths []string
	// MaxFileSize is the maximum file size to write (default 1MB).
	MaxFileSize int64
}

// NewFileWriteTool creates a new file write tool.
func NewFileWriteTool(allowedPaths []string, maxFileSize int64) *FileWriteTool {
	if maxFileSize <= 0 {
		maxFileSize = 1024 * 1024 // 1MB default
	}
	return &FileWriteTool{
		AllowedPaths: allowedPaths,
		MaxFileSize:  maxFileSize,
	}
}

// Definition returns the tool's definition.
func (f *FileWriteTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "file_write",
		Description: "Writes content to a file. Creates the file if it doesn't exist, or overwrites if it does.",
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
					"description": "The content to write to the file",
				},
				"append": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, append to the file instead of overwriting (default: false)",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

// Execute writes content to the file.
func (f *FileWriteTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, errors.New("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, errors.New("content is required")
	}

	appendMode := false
	if v, ok := args["append"].(bool); ok {
		appendMode = v
	}

	// Security: validate path
	if err := f.validatePath(path); err != nil {
		return nil, err
	}

	// Check content size
	if int64(len(content)) > f.MaxFileSize {
		return nil, fmt.Errorf("content too large: %d bytes (max: %d bytes)", len(content), f.MaxFileSize)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	var err error
	if appendMode {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()
		_, err = file.WriteString(content)
	} else {
		err = os.WriteFile(path, []byte(content), 0644)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Get file info after write
	info, _ := os.Stat(path)
	var size int64
	if info != nil {
		size = info.Size()
	}

	response := map[string]interface{}{
		"path":    path,
		"size":    size,
		"success": true,
		"append":  appendMode,
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

// validatePath checks if the path is allowed.
func (f *FileWriteTool) validatePath(path string) error {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)

	// If no allowed paths are configured, allow all
	if len(f.AllowedPaths) == 0 {
		return nil
	}

	// Check if path is within allowed directories
	for _, allowed := range f.AllowedPaths {
		allowedClean := filepath.Clean(allowed)
		if strings.HasPrefix(cleanPath, allowedClean) {
			return nil
		}
	}

	return fmt.Errorf("access denied: path %s is not in allowed directories", path)
}

// RegisterBuiltinTools registers built-in core tools with default configuration.
func RegisterBuiltinTools(registry *Registry) {
	if registry == nil {
		return
	}
	registry.Register(NewFileReadTool(nil, 0))
	registry.Register(NewFileWriteTool(nil, 0))
	registry.Register(NewWebSearchTool(WebSearchConfig{}))
}

// RegisterBuiltinToolsWithConfig registers built-in core tools with custom configuration.
func RegisterBuiltinToolsWithConfig(registry *Registry, webSearchConfig WebSearchConfig, allowedPaths []string, maxFileSize int64) {
	if registry == nil {
		return
	}
	registry.Register(NewFileReadTool(allowedPaths, maxFileSize))
	registry.Register(NewFileWriteTool(allowedPaths, maxFileSize))
	registry.Register(NewWebSearchTool(webSearchConfig))
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

// RegisterMemoryTools creates a MemoryTool for internal use (e.g., MgmtTool).
// As of v0.10.31, memory is no longer exposed as a native LLM tool —
// it's invoked via `blue mgmt memory` skill instead.
// The tool is registered as disabled so GetMemoryTool() still works.
func RegisterMemoryTools(registry *Registry, memoryService MemoryServiceInterface) {
	if memoryService == nil {
		return
	}
	registry.Register(NewMemoryTool(memoryService))
	registry.Disable("memory")
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
