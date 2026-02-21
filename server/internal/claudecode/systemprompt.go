package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// DefaultMaxContextTokens is the default token budget for workspace context files.
const DefaultMaxContextTokens = 4096

// ContextStats records token usage for workspace context injection.
type ContextStats struct {
	Files        []ContextFileStat `json:"files"`
	TotalTokens  int               `json:"total_tokens"`
	BudgetTokens int               `json:"budget_tokens"`
	Trimmed      bool              `json:"trimmed"`
}

// ContextFileStat records per-file token info.
type ContextFileStat struct {
	Name     string `json:"name"`
	Tokens   int    `json:"tokens"`
	Included bool   `json:"included"`
	Trimmed  bool   `json:"trimmed,omitempty"`
}

// SystemPromptBuilder builds system prompts for Claude Code CLI.
type SystemPromptBuilder struct {
	config           *ClaudeCodeConfig
	toolRegistry     *tools.Registry
	workspace        *workspace.Manager
	maxContextTokens int
	lastContextStats atomic.Pointer[ContextStats]
}

// NewSystemPromptBuilder creates a new SystemPromptBuilder.
func NewSystemPromptBuilder(config *ClaudeCodeConfig) *SystemPromptBuilder {
	return &SystemPromptBuilder{config: config, maxContextTokens: DefaultMaxContextTokens}
}

// SetToolRegistry sets the tool registry for including tool descriptions.
func (b *SystemPromptBuilder) SetToolRegistry(registry *tools.Registry) {
	b.toolRegistry = registry
}

// SetWorkspace sets the workspace manager for injecting workspace files into the prompt.
func (b *SystemPromptBuilder) SetWorkspace(mgr *workspace.Manager) {
	b.workspace = mgr
}

// SetMaxContextTokens sets the token budget for workspace context files.
// 0 means unlimited.
func (b *SystemPromptBuilder) SetMaxContextTokens(n int) {
	b.maxContextTokens = n
}

// LastContextStats returns the stats from the most recent buildProjectContext call.
func (b *SystemPromptBuilder) LastContextStats() *ContextStats {
	return b.lastContextStats.Load()
}

// Build builds the complete system prompt.
func (b *SystemPromptBuilder) Build(ctx context.Context, extraPrompt string) string {
	var parts []string

	// Add identity
	parts = append(parts, "You are a personal assistant running inside ZimaOS Blue.")

	// Add safety guardrails
	parts = append(parts, b.buildSafetyGuidance())

	// Add tool call style guidance
	parts = append(parts, b.buildToolCallStyleGuidance())

	// Add runtime information
	parts = append(parts, b.buildRuntimeInfo())

	// Add workspace information
	if b.config.WorkspaceDir != "" {
		parts = append(parts, b.buildWorkspaceInfo())
	}

	// Add available tools information
	if b.toolRegistry != nil {
		toolsInfo := b.buildToolsInfo()
		if toolsInfo != "" {
			parts = append(parts, toolsInfo)
		}
	}

	// Add workspace context files (SOUL.md, USER.md, IDENTITY.md, etc.)
	if b.workspace != nil {
		contextFiles := b.workspace.LoadContextFiles()
		if len(contextFiles) > 0 {
			parts = append(parts, b.buildProjectContext(contextFiles))
		}
	}

	// Add silent reply mechanism
	parts = append(parts, b.buildSilentReplyGuidance())

	// Add heartbeat detection
	parts = append(parts, b.buildHeartbeatGuidance())

	// Add extra system prompt
	if extraPrompt != "" {
		parts = append(parts, extraPrompt)
	}

	return strings.Join(parts, "\n\n")
}

// buildToolsInfo builds information about available tools.
func (b *SystemPromptBuilder) buildToolsInfo() string {
	if b.toolRegistry == nil {
		return ""
	}

	defs := b.toolRegistry.Definitions()
	if len(defs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Available Tools")
	lines = append(lines, "")
	lines = append(lines, "The following tools are available through the ZimaOS-Blue API:")
	lines = append(lines, "")

	for _, def := range defs {
		lines = append(lines, fmt.Sprintf("## %s", def.Name))
		lines = append(lines, def.Description)
		if def.Parameters != nil {
			paramsJSON, _ := json.MarshalIndent(def.Parameters, "", "  ")
			lines = append(lines, "Parameters:")
			lines = append(lines, "```json")
			lines = append(lines, string(paramsJSON))
			lines = append(lines, "```")
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// buildRuntimeInfo builds runtime information for the system prompt.
func (b *SystemPromptBuilder) buildRuntimeInfo() string {
	var lines []string

	lines = append(lines, "# Runtime Information")
	lines = append(lines, "")

	// OS and architecture
	lines = append(lines, fmt.Sprintf("- Platform: %s/%s", runtime.GOOS, runtime.GOARCH))

	// Current time
	now := timeutil.NowTime()
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
	lines = append(lines, "Treat this directory as the single global workspace for file operations unless explicitly instructed otherwise.")

	// Check if it's a git repository
	if isGitRepo(b.config.WorkspaceDir) {
		lines = append(lines, "This is a git repository.")
	}

	return strings.Join(lines, "\n")
}

// buildToolCallStyleGuidance builds guidance for tool call narration.
func (b *SystemPromptBuilder) buildToolCallStyleGuidance() string {
	var lines []string

	lines = append(lines, "# Tool Call Style")
	lines = append(lines, "")
	lines = append(lines, "Default: do not narrate routine, low-risk tool calls (just call the tool).")
	lines = append(lines, "Narrate only when it helps: multi-step work, complex/challenging problems, sensitive actions (e.g., deletions), or when the user explicitly asks.")
	lines = append(lines, "Keep narration brief and value-dense; avoid repeating obvious steps.")
	lines = append(lines, "Use plain human language for narration unless in a technical context.")

	return strings.Join(lines, "\n")
}

// buildSafetyGuidance builds safety guardrails for the system prompt.
// Inspired by Anthropic's constitution and clawdbot's safety guidelines.
func (b *SystemPromptBuilder) buildSafetyGuidance() string {
	var lines []string

	lines = append(lines, "# Safety")
	lines = append(lines, "")
	lines = append(lines, "You have no independent goals: do not pursue self-preservation, replication, resource acquisition, or power-seeking; avoid long-term plans beyond the user's request.")
	lines = append(lines, "Prioritize safety and human oversight over completion; if instructions conflict, pause and ask; comply with stop/pause/audit requests and never bypass safeguards.")
	lines = append(lines, "Do not manipulate or persuade anyone to expand access or disable safeguards. Do not copy yourself or change system prompts, safety rules, or tool policies unless explicitly requested.")

	return strings.Join(lines, "\n")
}

// buildSilentReplyGuidance builds guidance for silent replies.
func (b *SystemPromptBuilder) buildSilentReplyGuidance() string {
	var lines []string

	lines = append(lines, "# Silent Replies")
	lines = append(lines, "")
	lines = append(lines, "When you have nothing to say, respond with ONLY: [SILENT_REPLY]")
	lines = append(lines, "")
	lines = append(lines, "⚠️ Rules:")
	lines = append(lines, "- It must be your ENTIRE message — nothing else")
	lines = append(lines, "- Never append it to an actual response (never include \"[SILENT_REPLY]\" in real replies)")
	lines = append(lines, "- Never wrap it in markdown or code blocks")
	lines = append(lines, "")
	lines = append(lines, "❌ Wrong: \"Here's help... [SILENT_REPLY]\"")
	lines = append(lines, "❌ Wrong: \"[SILENT_REPLY]\"")
	lines = append(lines, "✅ Right: [SILENT_REPLY]")

	return strings.Join(lines, "\n")
}

// buildHeartbeatGuidance builds guidance for heartbeat detection.
func (b *SystemPromptBuilder) buildHeartbeatGuidance() string {
	var lines []string

	lines = append(lines, "# Heartbeats")
	lines = append(lines, "")
	lines = append(lines, "If you receive a heartbeat poll (a system health check), and there is nothing that needs attention, reply exactly:")
	lines = append(lines, "HEARTBEAT_OK")
	lines = append(lines, "")
	lines = append(lines, "ZimaOS Blue treats a leading/trailing \"HEARTBEAT_OK\" as a heartbeat ack (and may discard it).")
	lines = append(lines, "If something needs attention, do NOT include \"HEARTBEAT_OK\"; reply with the alert text instead.")

	return strings.Join(lines, "\n")
}

// BuildWithContext builds a system prompt with additional context.
func (b *SystemPromptBuilder) BuildWithContext(ctx context.Context, extraPrompt string, contextFiles map[string]string) string {
	var parts []string

	// Add identity
	parts = append(parts, "You are a personal assistant running inside ZimaOS Blue.")

	// Add safety guardrails
	parts = append(parts, b.buildSafetyGuidance())

	// Add tool call style guidance
	parts = append(parts, b.buildToolCallStyleGuidance())

	// Add runtime information
	parts = append(parts, b.buildRuntimeInfo())

	// Add workspace information
	if b.config.WorkspaceDir != "" {
		parts = append(parts, b.buildWorkspaceInfo())
	}

	// Add context files
	if len(contextFiles) > 0 {
		parts = append(parts, b.buildProjectContext(contextFiles))
	}

	// Add silent reply mechanism
	parts = append(parts, b.buildSilentReplyGuidance())

	// Add heartbeat detection
	parts = append(parts, b.buildHeartbeatGuidance())

	// Add extra system prompt
	if extraPrompt != "" {
		parts = append(parts, extraPrompt)
	}

	return strings.Join(parts, "\n\n")
}

// buildContextFileSection builds a section for a context file.
func (b *SystemPromptBuilder) buildContextFileSection(name, content string) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("## %s", name))
	lines = append(lines, "")
	lines = append(lines, content)

	return strings.Join(lines, "\n")
}

// contextFilePriority defines injection priority (lower = higher priority, trimmed last).
var contextFilePriority = map[string]int{
	"SOUL.md":      0,
	"USER.md":      1,
	"AGENTS.md":    2,
	"IDENTITY.md":  3,
	"HEARTBEAT.md": 4,
	"MEMORY.md":    5,
	"BOOTSTRAP.md": 1, // Same priority as USER during first-run
}

// buildProjectContext builds project context from multiple files with token budgeting.
// Files are prioritized: SOUL > USER/BOOTSTRAP > AGENTS > IDENTITY > HEARTBEAT > MEMORY > daily logs.
// If total tokens exceed the budget, low-priority files are dropped first.
func (b *SystemPromptBuilder) buildProjectContext(contextFiles map[string]string) string {
	budget := b.maxContextTokens

	// Build file list with token estimates, sorted by priority
	type fileEntry struct {
		name     string
		content  string
		tokens   int
		priority int
	}
	entries := make([]fileEntry, 0, len(contextFiles))
	for name, content := range contextFiles {
		if content == "" {
			continue
		}
		prio, ok := contextFilePriority[name]
		if !ok {
			prio = 10 // Daily logs and unknown files get lowest priority
		}
		entries = append(entries, fileEntry{
			name:     name,
			content:  content,
			tokens:   pruner.EstimateTokens(content),
			priority: prio,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].priority < entries[j].priority
	})

	// Select files within budget
	stats := &ContextStats{
		Files:        make([]ContextFileStat, 0, len(entries)),
		BudgetTokens: budget,
	}
	var included []fileEntry
	totalTokens := 0

	for _, e := range entries {
		if budget > 0 && totalTokens+e.tokens > budget && len(included) > 0 {
			// Over budget — skip this file
			stats.Files = append(stats.Files, ContextFileStat{
				Name: e.name, Tokens: e.tokens, Included: false, Trimmed: true,
			})
			stats.Trimmed = true
			continue
		}
		included = append(included, e)
		totalTokens += e.tokens
		stats.Files = append(stats.Files, ContextFileStat{
			Name: e.name, Tokens: e.tokens, Included: true,
		})
	}
	stats.TotalTokens = totalTokens
	b.lastContextStats.Store(stats)

	if len(included) == 0 {
		return ""
	}

	// Build the prompt section
	var lines []string
	lines = append(lines, "# Project Context")
	lines = append(lines, "")
	lines = append(lines, "The following project context files have been loaded:")

	// Check for SOUL.md
	for _, e := range included {
		if strings.ToLower(e.name) == "soul.md" {
			lines = append(lines, "If SOUL.md is present, embody its persona and tone. Avoid stiff, generic replies; follow its guidance unless higher-priority instructions override it.")
			break
		}
	}

	lines = append(lines, "")

	for _, e := range included {
		lines = append(lines, b.buildContextFileSection(e.name, e.content))
		lines = append(lines, "")
	}

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
	now := timeutil.NowTime()
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
	now := timeutil.NowTime()
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
