package claudecode

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// SystemPromptBuilder builds system prompts for Claude Code CLI.
type SystemPromptBuilder struct {
	config *ClaudeCodeConfig
}

// NewSystemPromptBuilder creates a new SystemPromptBuilder.
func NewSystemPromptBuilder(config *ClaudeCodeConfig) *SystemPromptBuilder {
	return &SystemPromptBuilder{config: config}
}

// Build builds the complete system prompt.
func (b *SystemPromptBuilder) Build(ctx context.Context, extraPrompt string) string {
	var parts []string

	// Add runtime information
	parts = append(parts, b.buildRuntimeInfo())

	// Add workspace information
	if b.config.WorkspaceDir != "" {
		parts = append(parts, b.buildWorkspaceInfo())
	}

	// Add extra system prompt
	if extraPrompt != "" {
		parts = append(parts, extraPrompt)
	}

	return strings.Join(parts, "\n\n")
}

// buildRuntimeInfo builds runtime information for the system prompt.
func (b *SystemPromptBuilder) buildRuntimeInfo() string {
	var lines []string

	lines = append(lines, "# Runtime Information")
	lines = append(lines, "")

	// OS and architecture
	lines = append(lines, fmt.Sprintf("- Platform: %s/%s", runtime.GOOS, runtime.GOARCH))

	// Current time
	now := time.Now()
	lines = append(lines, fmt.Sprintf("- Current time: %s", now.Format(time.RFC3339)))

	// Timezone
	zone, _ := now.Zone()
	lines = append(lines, fmt.Sprintf("- Timezone: %s", zone))

	// Model (if configured)
	if b.config.DefaultModel != "" {
		lines = append(lines, fmt.Sprintf("- Default model: %s", b.config.DefaultModel))
	}

	return strings.Join(lines, "\n")
}

// buildWorkspaceInfo builds workspace information for the system prompt.
func (b *SystemPromptBuilder) buildWorkspaceInfo() string {
	var lines []string

	lines = append(lines, "# Workspace")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Working directory: %s", b.config.WorkspaceDir))

	// Check if it's a git repository
	if isGitRepo(b.config.WorkspaceDir) {
		lines = append(lines, "This is a git repository.")
	}

	return strings.Join(lines, "\n")
}

// BuildWithContext builds a system prompt with additional context.
func (b *SystemPromptBuilder) BuildWithContext(ctx context.Context, extraPrompt string, contextFiles map[string]string) string {
	var parts []string

	// Add runtime information
	parts = append(parts, b.buildRuntimeInfo())

	// Add workspace information
	if b.config.WorkspaceDir != "" {
		parts = append(parts, b.buildWorkspaceInfo())
	}

	// Add context files
	for name, content := range contextFiles {
		if content != "" {
			parts = append(parts, b.buildContextFileSection(name, content))
		}
	}

	// Add extra system prompt
	if extraPrompt != "" {
		parts = append(parts, extraPrompt)
	}

	return strings.Join(parts, "\n\n")
}

// buildContextFileSection builds a section for a context file.
func (b *SystemPromptBuilder) buildContextFileSection(name, content string) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("# Context: %s", name))
	lines = append(lines, "")
	lines = append(lines, content)

	return strings.Join(lines, "\n")
}

// BuildMinimal builds a minimal system prompt without runtime info.
func (b *SystemPromptBuilder) BuildMinimal(extraPrompt string) string {
	if extraPrompt == "" {
		return ""
	}
	return extraPrompt
}

// BuildForHeartbeat builds a system prompt for heartbeat runs.
func (b *SystemPromptBuilder) BuildForHeartbeat(ctx context.Context) string {
	var parts []string

	// Minimal runtime info
	now := time.Now()
	parts = append(parts, fmt.Sprintf("Current time: %s", now.Format(time.RFC3339)))

	// Heartbeat instructions
	parts = append(parts, "This is a heartbeat check. Read HEARTBEAT.md if it exists and follow its instructions. If nothing needs attention, reply with HEARTBEAT_OK.")

	return strings.Join(parts, "\n\n")
}

// isGitRepo checks if a directory is a git repository.
func isGitRepo(dir string) bool {
	gitDir := dir + "/.git"
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// RuntimeInfo contains runtime information.
type RuntimeInfo struct {
	Platform     string    `json:"platform"`
	Architecture string    `json:"architecture"`
	CurrentTime  time.Time `json:"current_time"`
	Timezone     string    `json:"timezone"`
	Model        string    `json:"model,omitempty"`
	WorkspaceDir string    `json:"workspace_dir,omitempty"`
	IsGitRepo    bool      `json:"is_git_repo"`
}

// GetRuntimeInfo returns the current runtime information.
func (b *SystemPromptBuilder) GetRuntimeInfo() *RuntimeInfo {
	now := time.Now()
	zone, _ := now.Zone()

	info := &RuntimeInfo{
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		CurrentTime:  now,
		Timezone:     zone,
		Model:        b.config.DefaultModel,
		WorkspaceDir: b.config.WorkspaceDir,
	}

	if b.config.WorkspaceDir != "" {
		info.IsGitRepo = isGitRepo(b.config.WorkspaceDir)
	}

	return info
}
