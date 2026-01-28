package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// BootstrapHandler handles loading of bootstrap and context files.
type BootstrapHandler struct {
	config   *ClaudeCodeConfig
	maxChars int
}

// ContextFiles contains loaded context files.
type ContextFiles struct {
	// ClaudeMD is the content of CLAUDE.md
	ClaudeMD string

	// Settings is the content of .claude/settings.json
	Settings string

	// ProjectContext is any project-specific context
	ProjectContext string

	// Custom contains any custom context files
	Custom map[string]string
}

// NewBootstrapHandler creates a new BootstrapHandler.
func NewBootstrapHandler(config *ClaudeCodeConfig) *BootstrapHandler {
	maxChars := 20000 // Default max chars for context files
	return &BootstrapHandler{
		config:   config,
		maxChars: maxChars,
	}
}

// ResolveContextForRun loads all context files for a run.
func (h *BootstrapHandler) ResolveContextForRun(ctx context.Context, workspaceDir string) (*ContextFiles, error) {
	if workspaceDir == "" {
		workspaceDir = h.config.WorkspaceDir
	}

	files := &ContextFiles{
		Custom: make(map[string]string),
	}

	// Load CLAUDE.md
	claudeMDPath := filepath.Join(workspaceDir, "CLAUDE.md")
	if content, err := h.LoadContextFile(claudeMDPath); err == nil {
		files.ClaudeMD = content
	}

	// Load .claude/settings.json
	settingsPath := filepath.Join(workspaceDir, ".claude", "settings.json")
	if content, err := h.LoadContextFile(settingsPath); err == nil {
		files.Settings = content
	}

	// Load project-specific context files
	projectContextPaths := []string{
		filepath.Join(workspaceDir, "CONTEXT.md"),
		filepath.Join(workspaceDir, ".context.md"),
		filepath.Join(workspaceDir, "PROJECT.md"),
	}

	for _, path := range projectContextPaths {
		if content, err := h.LoadContextFile(path); err == nil && content != "" {
			files.ProjectContext = content
			break
		}
	}

	return files, nil
}

// LoadContextFile loads and validates a context file.
func (h *BootstrapHandler) LoadContextFile(path string) (string, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// Check file size
	if info.Size() > int64(h.maxChars*4) { // Rough estimate: 4 bytes per char
		// File is too large, will need truncation
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Truncate if necessary
	text := string(content)
	if len(text) > h.maxChars {
		text = h.TruncateContextFile(text, h.maxChars)
	}

	return text, nil
}

// TruncateContextFile truncates a context file to the specified max chars.
func (h *BootstrapHandler) TruncateContextFile(content string, maxChars int) string {
	if len(content) <= maxChars {
		return content
	}

	// Try to truncate at a natural boundary
	truncated := content[:maxChars]

	// Find the last newline
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > maxChars/2 {
		truncated = truncated[:lastNewline]
	}

	// Add truncation notice
	truncated += "\n\n[Content truncated due to length...]"

	return truncated
}

// LoadCustomContextFile loads a custom context file.
func (h *BootstrapHandler) LoadCustomContextFile(path string) (string, error) {
	return h.LoadContextFile(path)
}

// BuildContextMap builds a map of context file names to contents.
func (h *BootstrapHandler) BuildContextMap(files *ContextFiles) map[string]string {
	result := make(map[string]string)

	if files.ClaudeMD != "" {
		result["CLAUDE.md"] = files.ClaudeMD
	}

	if files.Settings != "" {
		result[".claude/settings.json"] = files.Settings
	}

	if files.ProjectContext != "" {
		result["Project Context"] = files.ProjectContext
	}

	for name, content := range files.Custom {
		result[name] = content
	}

	return result
}

// FindContextFiles searches for context files in a directory.
func (h *BootstrapHandler) FindContextFiles(dir string) ([]string, error) {
	var files []string

	// Known context file patterns
	patterns := []string{
		"CLAUDE.md",
		".claude/settings.json",
		"CONTEXT.md",
		".context.md",
		"PROJECT.md",
		"README.md",
		"CONTRIBUTING.md",
	}

	for _, pattern := range patterns {
		path := filepath.Join(dir, pattern)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	return files, nil
}

// SetMaxChars sets the maximum characters for context files.
func (h *BootstrapHandler) SetMaxChars(maxChars int) {
	h.maxChars = maxChars
}

// GetMaxChars returns the maximum characters for context files.
func (h *BootstrapHandler) GetMaxChars() int {
	return h.maxChars
}
