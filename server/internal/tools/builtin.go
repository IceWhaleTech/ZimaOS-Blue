package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CalculatorTool performs basic arithmetic operations.
type CalculatorTool struct{}

// NewCalculatorTool creates a new calculator tool.
func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

// Definition returns the tool's definition.
func (c *CalculatorTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "Calculator",
		Description: "Performs basic arithmetic operations. Supports +, -, *, /, and parentheses.",
		Icon:        "calculator",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The arithmetic expression to evaluate (e.g., '2 + 3 * 4')",
				},
			},
			"required": []string{"expression"},
		},
	}
}

// Execute evaluates the arithmetic expression.
func (c *CalculatorTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	expr, ok := args["expression"].(string)
	if !ok || expr == "" {
		return nil, errors.New("expression is required")
	}

	result, err := evaluateExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	response := map[string]interface{}{
		"expression": expr,
		"result":     result,
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

// evaluateExpression safely evaluates a mathematical expression.
func evaluateExpression(expr string) (float64, error) {
	// Parse the expression as a Go expression
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %w", err)
	}

	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("unsupported literal type: %v", n.Kind)

	case *ast.UnaryExpr:
		val, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.SUB:
			return -val, nil
		case token.ADD:
			return val, nil
		default:
			return 0, fmt.Errorf("unsupported unary operator: %v", n.Op)
		}

	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}

		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %v", n.Op)
		}

	case *ast.ParenExpr:
		return evalNode(n.X)

	default:
		return 0, fmt.Errorf("unsupported expression type: %T", node)
	}
}

// SystemInfoTool returns system information.
type SystemInfoTool struct{}

// NewSystemInfoTool creates a new system info tool.
func NewSystemInfoTool() *SystemInfoTool {
	return &SystemInfoTool{}
}

// Definition returns the tool's definition.
func (s *SystemInfoTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "System Info",
		Description: "Returns information about the system (OS, architecture, hostname, CPU count, Go version).",
		Icon:        "system-info",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

// Execute returns system information.
func (s *SystemInfoTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	hostname, _ := os.Hostname()

	info := map[string]interface{}{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"hostname":   hostname,
		"num_cpu":    runtime.NumCPU(),
		"go_version": runtime.Version(),
	}

	jsonResult, _ := json.Marshal(info)
	return string(jsonResult), nil
}

// CurrentTimeTool returns the current time.
type CurrentTimeTool struct{}

// NewCurrentTimeTool creates a new current time tool.
func NewCurrentTimeTool() *CurrentTimeTool {
	return &CurrentTimeTool{}
}

// Definition returns the tool's definition.
func (t *CurrentTimeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "Current Time",
		Description: "Returns the current time in UTC, local time, and Unix timestamp. Optionally accepts a timezone.",
		Icon:        "current-time",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "Optional timezone (e.g., 'America/New_York', 'Europe/London')",
				},
			},
		},
	}
}

// Execute returns the current time.
func (t *CurrentTimeTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	now := time.Now()

	info := map[string]interface{}{
		"utc":   now.UTC().Format(time.RFC3339),
		"local": now.Format(time.RFC3339),
		"unix":  now.Unix(),
	}

	// Handle optional timezone
	if tz, ok := args["timezone"].(string); ok && tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone: %w", err)
		}
		info["requested_timezone"] = now.In(loc).Format(time.RFC3339)
		info["timezone"] = tz
	}

	jsonResult, _ := json.Marshal(info)
	return string(jsonResult), nil
}

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
		Name:        "File Read",
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
		Name:        "File Write",
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

// RegisterBuiltinTools registers all built-in tools with the registry.
func RegisterBuiltinTools(registry *Registry) {
	registry.Register(NewCalculatorTool())
	registry.Register(NewSystemInfoTool())
	registry.Register(NewCurrentTimeTool())
	// File tools with default settings (allow all paths, 1MB max)
	registry.Register(NewFileReadTool(nil, 0))
	registry.Register(NewFileWriteTool(nil, 0))
	// Web search with default settings (DuckDuckGo)
	registry.Register(NewWebSearchTool(WebSearchConfig{}))
}

// RegisterBuiltinToolsWithConfig registers all built-in tools with custom configuration.
func RegisterBuiltinToolsWithConfig(registry *Registry, webSearchConfig WebSearchConfig, allowedPaths []string, maxFileSize int64) {
	registry.Register(NewCalculatorTool())
	registry.Register(NewSystemInfoTool())
	registry.Register(NewCurrentTimeTool())
	registry.Register(NewFileReadTool(allowedPaths, maxFileSize))
	registry.Register(NewFileWriteTool(allowedPaths, maxFileSize))
	registry.Register(NewWebSearchTool(webSearchConfig))
}

// RegisterMemoryTools registers memory-related tools with the registry.
// This should be called after the memory service is initialized.
func RegisterMemoryTools(registry *Registry, memoryService MemoryServiceInterface) {
	if memoryService == nil {
		return
	}
	registry.Register(NewMemorySearchToolWithInterface(memoryService))
	registry.Register(NewMemoryGetToolWithInterface(memoryService))
	registry.Register(NewMemoryStatsToolWithInterface(memoryService))
	if progressiveSearchTool != nil {
		registry.Register(progressiveSearchTool)
	}
}

// SetProgressiveSearchTool sets the progressive search tool for registration.
// This is called from the memory package to avoid circular imports.
func SetProgressiveSearchTool(tool Tool) {
	progressiveSearchTool = tool
}

// MemoryServiceInterface defines the interface for memory service used by tools.
// This avoids circular imports with the memory package.
type MemoryServiceInterface interface {
	Recall(ctx context.Context, query string, limit int) ([]MemorySearchResult, error)
	Get(ctx context.Context, id string) (*MemoryChunkResult, error)
	Stats(ctx context.Context) (*MemoryStatsResult, error)
	GetActiveBackend() string
}

// ProgressiveSearchTool is an optional tool that can be registered for progressive search.
// It implements the Tool interface directly in the memory package to avoid circular imports.
// Use RegisterProgressiveSearchTool to register it.
var progressiveSearchTool Tool

// MemorySearchResult represents a search result from memory service.
type MemorySearchResult struct {
	Chunk         MemoryChunkResult
	VectorScore   float32
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
