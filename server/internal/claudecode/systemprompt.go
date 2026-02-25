package claudecode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	lastUserMessage  string       // set before Build() for scenario-based enhancement
	locale           string       // user locale (e.g. "en-US", "zh-CN") — static fallback
	localeFunc       func() string // dynamic locale getter (takes precedence over static)
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

// SetLastUserMessage stores the latest user message for scenario-based
// enhancement detection. Call this before Build().
func (b *SystemPromptBuilder) SetLastUserMessage(msg string) {
	b.lastUserMessage = msg
}

// SetLocale sets the user's locale for system prompt injection (e.g. "en-US", "zh-CN").
func (b *SystemPromptBuilder) SetLocale(locale string) {
	b.locale = locale
}

// SetLocaleFunc sets a dynamic locale getter that is called on each Build().
// Takes precedence over the static locale set via SetLocale.
func (b *SystemPromptBuilder) SetLocaleFunc(fn func() string) {
	b.localeFunc = fn
}

// getLocale returns the current locale, preferring the dynamic getter.
func (b *SystemPromptBuilder) getLocale() string {
	if b.localeFunc != nil {
		if v := b.localeFunc(); v != "" {
			return v
		}
	}
	return b.locale
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

	// Add scenario-based response enhancement
	if enhancement := b.buildEnhancement(b.lastUserMessage); enhancement != "" {
		parts = append(parts, enhancement)
	}

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

	// Add available tools information (file_read, file_write, web_search, memory)
	if b.toolRegistry != nil {
		toolsInfo := b.buildToolsInfo()
		if toolsInfo != "" {
			parts = append(parts, toolsInfo)
		}
	}

	// Add available skills (XML index — model reads SKILL.md on demand via file_read)
	if skillsSection := b.buildSkillsSection(); skillsSection != "" {
		parts = append(parts, skillsSection)
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

// toolUsageHints provides intent-based descriptions that help the LLM choose
// the right tool. Only covers tools registered in the tool registry (not skills).
var toolUsageHints = map[string]string{
	"web_search":  "Keyword search only. Returns result listings (title+URL+snippet). Never opens or reads any page. Not for URLs you already have.",
	"browser":     "Open a URL, read content, interact with elements, take screenshots. Not for keyword search (→ web_search) or UI scoring (→ ui_reviewer).",
	"ui_reviewer": "Score and audit UI/UX quality. Use only when asked to evaluate/rate/review visual design or accessibility. Not for browsing or searching.",
	"memory": "Unified memory tool. Use action='search' to find relevant memories by query. Use action='remember' to store important facts, preferences, and notes the user asks you to remember. Use action='get' to retrieve a specific memory by ID. Use action='forget' to delete a memory. Use action='stats' for memory system statistics.",
}

// buildToolsInfo builds lightweight tool guidance for the system prompt.
func (b *SystemPromptBuilder) buildToolsInfo() string {
	if b.toolRegistry == nil {
		return ""
	}

	defs := b.toolRegistry.Definitions()
	if len(defs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Tool Guidance")
	lines = append(lines, "")

	hasHints := false
	for _, def := range defs {
		if hint, ok := toolUsageHints[def.Name]; ok {
			lines = append(lines, fmt.Sprintf("- **%s**: %s", def.Name, hint))
			hasHints = true
		}
	}
	if hasHints {
		lines = append(lines, "")
	}

	// Check if exec tool is registered and add detailed guidance.
	for _, def := range defs {
		if def.Name == "exec" {
			lines = append(lines, b.buildExecGuidance()...)
			break
		}
	}

	lines = append(lines, "## Tool Routing Rules")
	lines = append(lines, "- No URL, need to find info → web_search")
	lines = append(lines, "- Have a URL, need to read/interact/screenshot → browser")
	lines = append(lines, "- Need to evaluate/rate/score UI or accessibility → ui_reviewer")
	lines = append(lines, "- After web_search, user says \"open it\" → browser")
	lines = append(lines, "- \"How does this look?\" / \"Is this well-designed?\" → ui_reviewer")
	lines = append(lines, "- \"What does this page say?\" / \"Read this for me\" → browser")
	lines = append(lines, "- Just screenshot, no scoring → browser (screenshot)")
	lines = append(lines, "- \"Remember this\" / \"don't forget\" → memory (action=remember)")
	lines = append(lines, "- NEVER use exec to call other tools — call them directly by name")
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// buildExecGuidance returns detailed exec tool usage and safety guidance lines.
func (b *SystemPromptBuilder) buildExecGuidance() []string {
	var lines []string

	// Check sandbox availability.
	hasSandbox := false
	if et := tools.GetExecTool(b.toolRegistry); et != nil {
		hasSandbox = et.HasSandbox()
	}

	lines = append(lines, "## Exec Tool Guidelines")
	lines = append(lines, "")

	// Purpose
	lines = append(lines, "### Purpose")
	lines = append(lines, "Execute shell/CLI commands on the host OS (ls, git, curl, npm, pip, make, etc.).")
	lines = append(lines, "NEVER use exec to invoke other tools — call them directly by name.")
	lines = append(lines, "")

	// Sandbox
	if hasSandbox {
		lines = append(lines, "### Sandbox Protection")
		lines = append(lines, "A sandbox environment is available for isolated command execution.")
		lines = append(lines, "- Set `host: \"sandbox\"` to run commands in a sandboxed environment with filesystem restrictions")
		lines = append(lines, "- Medium-risk and above commands are automatically sandboxed when no host is specified")
		lines = append(lines, "- Sandbox prevents commands from accessing directories outside the allowed paths")
		lines = append(lines, "- When a command runs in sandbox, the result includes `host: \"sandbox\"` — mention this to the user so they know the command was protected")
		lines = append(lines, "")
	}

	// Security restrictions
	lines = append(lines, "### Security Restrictions")
	lines = append(lines, "Commands are risk-scored. High-risk commands are blocked automatically:")
	lines = append(lines, "- Critical (blocked): rm -rf /, mkfs, dd to devices, pipe-to-shell, fork bombs")
	lines = append(lines, "- High (may be blocked): shutdown, reboot, useradd/userdel, recursive chmod on system dirs")
	lines = append(lines, "- Medium: sudo, recursive rm, crontab modification, netcat")
	lines = append(lines, "- Low (allowed): ls, git, npm, curl, echo, etc.")
	lines = append(lines, "")

	// Scope
	lines = append(lines, "### Scope")
	lines = append(lines, "- Operate within the project/workspace directory or temp directories")
	lines = append(lines, "- Avoid accessing system directories (/etc, /usr, /System, C:\\Windows) unless explicitly needed")
	lines = append(lines, "- If a task requires elevated privileges (sudo), inform the user instead of attempting it")
	lines = append(lines, "")

	// Command style
	lines = append(lines, "### Command Style")
	lines = append(lines, "- Prefer simple, single-purpose commands")
	lines = append(lines, "- Use && to chain dependent commands; use || for fallback")
	lines = append(lines, "- For risky or unfamiliar commands, briefly explain what the command does before executing")
	lines = append(lines, "- Do not start long-running servers or watch-mode processes (npm run dev, webpack --watch)")
	lines = append(lines, "- Do not launch interactive editors (vim, nano, less)")
	lines = append(lines, "")

	// Error handling & retry
	lines = append(lines, "### Error Handling")
	lines = append(lines, "- If a command fails, analyze the error output before retrying")
	lines = append(lines, "- NEVER retry the exact same failing command — the system will block repeated identical failures")
	lines = append(lines, "- Change the command, fix the underlying issue, or try a different approach")
	lines = append(lines, "- Report non-zero exit codes and stderr to the user")
	lines = append(lines, "")

	return lines
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

	// Locale
	if locale := b.getLocale(); locale != "" {
		lines = append(lines, fmt.Sprintf("- Locale: %s", locale))
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

// buildSkillsSection scans the workspace skills directory and builds the
// Skills section for the system prompt. Returns empty string if no skills found.
func (b *SystemPromptBuilder) buildSkillsSection() string {
	if b.config.WorkspaceDir == "" {
		return ""
	}

	skillsDir := filepath.Join(b.config.WorkspaceDir, ".claude", "skills")
	skills := ScanSkillsDir(skillsDir)
	if len(skills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "# Skills")
	lines = append(lines, "")
	lines = append(lines, "Before replying: scan <available_skills> <description> entries.")
	lines = append(lines, "- If exactly one skill clearly applies: read its SKILL.md at <location> with `file_read`, then follow it.")
	lines = append(lines, "- If multiple could apply: choose the most specific one, then read/follow it.")
	lines = append(lines, "- If none clearly apply: do not read any SKILL.md.")
	lines = append(lines, "Constraints: never read more than one skill up front; only read after selecting.")
	lines = append(lines, "")
	lines = append(lines, FormatSkillsPrompt(skills))

	return strings.Join(lines, "\n")
}

// BuildWithContext builds a system prompt with additional context.
func (b *SystemPromptBuilder) BuildWithContext(ctx context.Context, extraPrompt string, contextFiles map[string]string) string {
	var parts []string

	// Add identity
	parts = append(parts, "You are a personal assistant running inside ZimaOS Blue.")

	// Add scenario-based response enhancement
	if enhancement := b.buildEnhancement(b.lastUserMessage); enhancement != "" {
		parts = append(parts, enhancement)
	}

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

	// Add available skills (XML index — model reads SKILL.md on demand via file_read)
	if skillsSection := b.buildSkillsSection(); skillsSection != "" {
		parts = append(parts, skillsSection)
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
		// Compact markdown: strip blank lines, HTML comments, empty sections
		content = pruner.CompactMarkdown(content)
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
