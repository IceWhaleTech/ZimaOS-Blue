package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
		Name:        "calculator",
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

// evaluateExpression safely evaluates a mathematical expression using a
// hand-written recursive descent parser (avoids importing go/parser).
func evaluateExpression(expr string) (float64, error) {
	p := &exprParser{input: expr}
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %c", p.pos, p.input[p.pos])
	}
	return result, nil
}

type exprParser struct {
	input string
	pos   int
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

// parseExpr handles + and -
func (p *exprParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			return left, nil
		}
		op := p.input[p.pos]
		if op != '+' && op != '-' {
			return left, nil
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
}

// parseTerm handles * and /
func (p *exprParser) parseTerm() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			return left, nil
		}
		op := p.input[p.pos]
		if op != '*' && op != '/' {
			return left, nil
		}
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			left /= right
		}
	}
}

// parseUnary handles unary + and -
func (p *exprParser) parseUnary() (float64, error) {
	p.skipSpace()
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
		val, err := p.parseUnary()
		return -val, err
	}
	if p.pos < len(p.input) && p.input[p.pos] == '+' {
		p.pos++
		return p.parseUnary()
	}
	return p.parsePrimary()
}

// parsePrimary handles numbers and parenthesized expressions
func (p *exprParser) parsePrimary() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return 0, errors.New("unexpected end of expression")
	}
	if p.input[p.pos] == '(' {
		p.pos++
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, errors.New("missing closing parenthesis")
		}
		p.pos++
		return val, nil
	}
	start := p.pos
	for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9' || p.input[p.pos] == '.') {
		p.pos++
	}
	if p.pos == start {
		return 0, fmt.Errorf("unexpected character: %c", p.input[p.pos])
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
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
		Name:        "system_info",
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
		Name:        "current_time",
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
	now := timeutil.NowTime()
	lang := GetLang(ctx)

	// Determine display timezone
	loc := now.Location()
	tzName := loc.String()
	if tz, ok := args["timezone"].(string); ok && tz != "" {
		parsed, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone: %w", err)
		}
		loc = parsed
		tzName = tz
	}

	display := now.In(loc)

	info := map[string]interface{}{
		"datetime": formatDateTimeLocale(display, lang),
		"timezone": tzName,
		"unix":     strconv.FormatInt(now.Unix(), 10),
	}

	jsonResult, _ := json.Marshal(info)
	return string(jsonResult), nil
}

// formatDateTimeLocale formats a time in a human-readable locale-appropriate string.
func formatDateTimeLocale(t time.Time, lang string) string {
	switch {
	case strings.HasPrefix(strings.ToLower(lang), "zh"):
		h := t.Hour()
		period := "上午"
		h12 := h
		if h >= 12 {
			period = "下午"
			if h > 12 {
				h12 = h - 12
			}
		}
		if h == 0 {
			h12 = 12
		}
		return fmt.Sprintf("%d年%d月%d日 %s%d:%02d", t.Year(), t.Month(), t.Day(), period, h12, t.Minute())
	case strings.HasPrefix(strings.ToLower(lang), "ja"):
		return fmt.Sprintf("%d年%d月%d日 %02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
	case strings.HasPrefix(strings.ToLower(lang), "ko"):
		return fmt.Sprintf("%d년 %d월 %d일 %02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
	default:
		return t.Format("January 2, 2006 3:04 PM")
	}
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

// RegisterMemoryTools registers the unified memory tool with the registry.
// This should be called after the memory service is initialized.
func RegisterMemoryTools(registry *Registry, memoryService MemoryServiceInterface) {
	if memoryService == nil {
		return
	}
	registry.Register(NewMemoryTool(memoryService))
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
