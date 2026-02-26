package claudecode

import (
	"context"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// BuildResult holds the structured system prompt split into cache-friendly blocks.
// Static is byte-stable across requests (identity, safety, tool guidance, platform).
// Config changes rarely (agent mode, workspace context files, tools, skills).
// Dynamic changes every request (timestamp, extra prompt).
type BuildResult struct {
	Static  string // core prompt, never changes — Anthropic cache_control: ephemeral
	Config  string // agent mode, tools, skills, workspace — Anthropic cache_control: ephemeral
	Dynamic string // timestamp + extra prompt — NOT cached
}

// String returns the full system prompt as a single string (for non-Anthropic providers).
// No newline separators — XML tags are self-delimiting.
func (r BuildResult) String() string {
	return r.Static + r.Config + r.Dynamic
}

// IsEmpty returns true if all blocks are empty.
func (r BuildResult) IsEmpty() bool {
	return r.Static == "" && r.Config == "" && r.Dynamic == ""
}

// StaticTokenEstimate returns an approximate token count for the static+config blocks.
func (r BuildResult) StaticTokenEstimate() int {
	return pruner.EstimateTokens(r.Static) + pruner.EstimateTokens(r.Config)
}

// TotalTokenEstimate returns an approximate token count for the full prompt.
func (r BuildResult) TotalTokenEstimate() int {
	return r.StaticTokenEstimate() + pruner.EstimateTokens(r.Dynamic)
}

// SystemPromptBuilder builds system prompts for Claude Code CLI.
type SystemPromptBuilder struct {
	config           *ClaudeCodeConfig
	toolRegistry     *tools.Registry
	workspace        *workspace.Manager

	maxContextTokens int
	lastContextStats atomic.Pointer[ContextStats]
	agentMode            bool         // when true, inject agent mode guidance
	agentModeFunc        func() bool  // dynamic agent mode getter (takes precedence over static)
	agentAutoConfirmFunc func() bool  // dynamic auto-confirm getter
	locale           string       // user locale (e.g. "en-US", "zh-CN") — static fallback
	localeFunc       func() string // dynamic locale getter (takes precedence over static)

	// staticSystemOnce caches the StaticSystem block (never changes within a process).
	staticSystemOnce sync.Once
	staticSystemStr  string

	// skillsCache caches the skills section with a TTL.
	skillsCacheMu   sync.Mutex
	skillsCacheStr  string
	skillsCacheTime time.Time
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

// SetLastUserMessage is a no-op retained for backward compatibility.
// Dynamic scenario enhancement has been removed to improve prompt cache hit rate.
func (b *SystemPromptBuilder) SetLastUserMessage(_ string) {}

// SetAgentMode enables or disables agent mode guidance in the system prompt.
func (b *SystemPromptBuilder) SetAgentMode(enabled bool) {
	b.agentMode = enabled
}

// SetAgentModeFunc sets a dynamic agent mode getter that is called on each Build().
// Takes precedence over the static value set via SetAgentMode.
func (b *SystemPromptBuilder) SetAgentModeFunc(fn func() bool) {
	b.agentModeFunc = fn
}

// SetAgentAutoConfirmFunc sets a dynamic getter for the auto-confirm setting.
func (b *SystemPromptBuilder) SetAgentAutoConfirmFunc(fn func() bool) {
	b.agentAutoConfirmFunc = fn
}

// isAgentMode returns whether agent mode is active, preferring the dynamic getter.
func (b *SystemPromptBuilder) isAgentMode() bool {
	if b.agentModeFunc != nil {
		return b.agentModeFunc()
	}
	return b.agentMode
}

// isAgentAutoConfirm returns whether auto-confirm is enabled in agent mode.
func (b *SystemPromptBuilder) isAgentAutoConfirm() bool {
	if b.agentAutoConfirmFunc != nil {
		return b.agentAutoConfirmFunc()
	}
	return false
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

// Build builds the complete system prompt as a single string.
// For cache-aware construction, use BuildStructured() instead.
func (b *SystemPromptBuilder) Build(ctx context.Context, extraPrompt string) string {
	return b.BuildStructured(ctx, extraPrompt).String()
}

// skillsCacheTTL is how long the skills section is cached before re-scanning.
const skillsCacheTTL = 30 * time.Second

// BuildStructured builds the system prompt split into STATIC, CONFIG, and TURN_DYNAMIC blocks.
// This enables Anthropic prompt caching: STATIC and CONFIG blocks get cache_control breakpoints,
// while TURN_DYNAMIC (timestamps, extra prompt) is left uncached.
func (b *SystemPromptBuilder) BuildStructured(ctx context.Context, extraPrompt string) BuildResult {
	var result BuildResult

	// ── STATIC_SYSTEM: byte-stable across all requests ──
	// Computed once per process lifetime.
	b.staticSystemOnce.Do(func() {
		var sb strings.Builder
		sb.WriteString("You are a personal assistant running inside ZimaOS Blue. Be clear and concise. Match the user's language. Your result wiil be cross-reviewed by claude & codex.")
		// If evidence is insufficient or conflicting, state uncertainty explicitly. Never fabricate sources. Distinguish verified information from inference. 
		sb.WriteString(b.buildSafetyGuidance())
		sb.WriteString(b.buildToolCallStyleGuidance())
		sb.WriteString(b.buildSilentReplyGuidance())
		sb.WriteString(b.buildHeartbeatGuidance())
		b.writePlatformInfoTo(&sb)
		b.staticSystemStr = sb.String()
	})
	result.Static = b.staticSystemStr

	// ── CONFIG_SYSTEM: changes when agent mode, tools, skills, or workspace files change ──
	var cfg strings.Builder

	// Agent mode guidance (if enabled)
	if b.isAgentMode() {
		b.writeAgentModeGuidanceTo(&cfg)
	}

	// Workspace information
	if b.config.WorkspaceDir != "" {
		b.writeWorkspaceInfoTo(&cfg)
	}

	// Available tools information
	if b.toolRegistry != nil {
		b.writeToolsInfoTo(&cfg)
	}

	// Available skills (XML index — cached string)
	if s := b.buildSkillsSection(); s != "" {
		cfg.WriteString(s)
	}

	// Workspace context files (SOUL.md, USER.md, etc.)
	if b.workspace != nil {
		if contextFiles := b.workspace.LoadContextFiles(); len(contextFiles) > 0 {
			cfg.WriteString(b.buildProjectContext(contextFiles))
		}
	}

	result.Config = cfg.String()

	// ── TURN_DYNAMIC: changes every request ──
	// Runtime information (contains timestamp — must be dynamic)
	var dyn strings.Builder
	b.writeRuntimeInfoTo(&dyn)
	if extraPrompt != "" {
		dyn.WriteString(extraPrompt)
	}
	result.Dynamic = dyn.String()

	return result
}

// writeToolsInfoTo writes lightweight tool guidance directly into sb.
// Returns true if anything was written.
func (b *SystemPromptBuilder) writeToolsInfoTo(sb *strings.Builder) bool {
	if b.toolRegistry == nil {
		return false
	}

	defs := b.toolRegistry.Definitions()
	if len(defs) == 0 {
		return false
	}

	sb.WriteString("<tool_guidance>Built-in API tools. Call via tool_use — never through exec/shell.")
	b.writeExecGuidanceTo(sb)
	sb.WriteString("</tool_guidance>")
	return true
}

// writeExecGuidanceTo writes compressed exec tool guidance directly into sb.
func (b *SystemPromptBuilder) writeExecGuidanceTo(sb *strings.Builder) {
	hasSandbox := false
	if et := tools.GetExecTool(b.toolRegistry); et != nil {
		hasSandbox = et.HasSandbox()
	}

	sb.WriteString("<exec_guide>Shell/CLI commands on host. REQUIRED: command parameter must be a non-empty string — never call exec without a concrete command.")

	if hasSandbox {
		sb.WriteString("<sandbox>host=sandbox for isolation. Medium+ risk auto-sandboxed.</sandbox>")
	}

	sb.WriteString("<scope>Project/workspace/temp dirs only. No /etc /usr /System. No sudo — inform user. Dangerous commands are auto-blocked by risk scoring.</scope>")

	sb.WriteString("<style>Simple single-purpose commands. && to chain, || fallback. No servers/watchers/interactive editors.")
	if runtime.GOOS == "darwin" {
		sb.WriteString(" macOS: grep -E not -P; sed no -i; BSD date.")
	}
	sb.WriteString("</style>")

	sb.WriteString("<retry>Analyze error before retry. NEVER retry identical failing command — system blocks repeats. Change command or try different approach.</retry>")

	sb.WriteString("</exec_guide>")
}

// writeRuntimeInfoTo writes the dynamic runtime tag directly into sb.
func (b *SystemPromptBuilder) writeRuntimeInfoTo(sb *strings.Builder) {
	sb.WriteString("<now>")
	sb.WriteString(strconv.FormatInt(timeutil.Now(), 10))
	sb.WriteString("</now>")
}

// buildRuntimeInfo returns the dynamic runtime tag as a string.
func (b *SystemPromptBuilder) buildRuntimeInfo() string {
	var sb strings.Builder
	b.writeRuntimeInfoTo(&sb)
	return sb.String()
}

// writePlatformInfoTo writes the static environment tag directly into sb.
func (b *SystemPromptBuilder) writePlatformInfoTo(sb *strings.Builder) {
	sb.WriteString("<env>")
	sb.WriteString(runtime.GOOS)
	sb.WriteByte('/')
	sb.WriteString(runtime.GOARCH)
	sb.WriteByte(';')
	if locale := b.getLocale(); locale != "" {
		sb.WriteString(locale)
	}
	sb.WriteByte(';')
	zone, _ := timeutil.NowTime().Zone()
	sb.WriteString(zone)
	sb.WriteString("</env>")
}

// buildPlatformInfo returns the static environment tag as a string.
func (b *SystemPromptBuilder) buildPlatformInfo() string {
	var sb strings.Builder
	b.writePlatformInfoTo(&sb)
	return sb.String()
}

// writeWorkspaceInfoTo writes workspace information directly into sb.
func (b *SystemPromptBuilder) writeWorkspaceInfoTo(sb *strings.Builder) {
	sb.WriteString("<workspace dir=\"")
	sb.WriteString(b.config.WorkspaceDir)
	sb.WriteString("\">Single global workspace for file operations unless explicitly instructed otherwise.")
	if isGitRepo(b.config.WorkspaceDir) {
		sb.WriteString(" Git repository.")
	}
	sb.WriteString("</workspace>")
}

// buildWorkspaceInfo returns workspace information as a string.
func (b *SystemPromptBuilder) buildWorkspaceInfo() string {
	var sb strings.Builder
	b.writeWorkspaceInfoTo(&sb)
	return sb.String()
}

// buildToolCallStyleGuidance builds compressed guidance for tool call narration.
func (b *SystemPromptBuilder) buildToolCallStyleGuidance() string {
	return "<tool_style>Do not narrate routine tool calls. Narrate only for multi-step work, complex problems, sensitive actions, or when asked. Keep narration brief.</tool_style>" +
		"<research_style>After search or investigation tool calls (web_search, browser, etc.): summarize key findings, then suggest next steps (open a URL for details, refine query, or answer directly).</research_style>"
}

// buildSafetyGuidance builds compressed safety guardrails for the system prompt.
func (b *SystemPromptBuilder) buildSafetyGuidance() string {
	return "<safety>No independent goals (no self-preservation/replication/power-seeking). Prioritize safety and human oversight; pause and ask on conflicting instructions; comply with stop/audit requests. Do not manipulate access, copy yourself, or change system prompts/safety rules unless explicitly requested.</safety>"
}

// buildSilentReplyGuidance builds compressed guidance for silent replies.
func (b *SystemPromptBuilder) buildSilentReplyGuidance() string {
	return "<silent_reply>When you have nothing to say, respond with ONLY: [SILENT_REPLY] (entire message, no wrapping, never appended to real content).</silent_reply>"
}

// buildHeartbeatGuidance builds compressed guidance for heartbeat detection.
func (b *SystemPromptBuilder) buildHeartbeatGuidance() string {
	return "<heartbeat>On heartbeat poll with nothing to report, reply exactly: HEARTBEAT_OK. If something needs attention, reply with alert text instead (no HEARTBEAT_OK).</heartbeat>"
}

// writeAgentModeGuidanceTo writes agent mode guidance directly into sb.
func (b *SystemPromptBuilder) writeAgentModeGuidanceTo(sb *strings.Builder) {
	autoConfirm := b.isAgentAutoConfirm()

	sb.WriteString("<agent_mode>You are in agent mode with unlimited autonomy for complex, multi-step tasks. No tool round limit — keep working until fully done.")

	sb.WriteString("<planning>For multi-step tasks, FIRST output a TODO checklist using markdown checkboxes (- [ ] step). The system auto-marks completed items and injects <tp> with current task — use it to decide what to do next. Do NOT re-output the checklist.</planning>")

	sb.WriteString("<execution>")
	sb.WriteString("Before each tool call, briefly state which task you are working on. ")
	if autoConfirm {
		sb.WriteString("Auto-confirm enabled — execute without asking. ")
	} else {
		sb.WriteString("Ask confirmation before destructive actions (delete, install, modify production config). Proceed without confirmation for safe operations. ")
	}
	sb.WriteString("Use exec for file ops, installs, builds, tests. Do NOT stop early. Do NOT call exec without a concrete command — think first, then execute.")
	sb.WriteString(` When facing multiple valid approaches or ambiguous requirements, use ask instead of guessing. Use "sq" for single-select or "mq" for multi-select, with "a" as the options array (2-4 strings). Example: {"sq":"Which approach?","a":["Option A","Option B"]}`)
	sb.WriteString("</execution>")

	sb.WriteString("<verification>After all steps, verify: run build/tests. Fix and re-verify if needed.</verification>")

	sb.WriteString("<completion>Your LAST response MUST be plain text (not a tool call). Include: 1) What was accomplished. 2) How to use/test the result. 3) Suggested next steps. Never end with a tool call.</completion>")

	sb.WriteString("</agent_mode>")
}

// buildAgentModeGuidance returns agent mode guidance as a string.
func (b *SystemPromptBuilder) buildAgentModeGuidance() string {
	var sb strings.Builder
	b.writeAgentModeGuidanceTo(&sb)
	return sb.String()
}

// buildSkillsSection builds the Skills section for the system prompt.
// Only pinned/important skills are listed explicitly. The LLM is told
// where to discover additional skills on disk.
func (b *SystemPromptBuilder) buildSkillsSection() string {
	if b.config.WorkspaceDir == "" {
		return ""
	}

	b.skillsCacheMu.Lock()
	defer b.skillsCacheMu.Unlock()

	if b.skillsCacheStr != "" && time.Since(b.skillsCacheTime) < skillsCacheTTL {
		return b.skillsCacheStr
	}

	var sb strings.Builder
	sb.WriteString("<skills>Invoke via exec: `blue <cmd> key=value ...` (e.g. `blue web_search query=\"latest news\"`). ")
	sb.WriteString("Routing: search→web_search, URL→browser, UI review→ui_reviewer, analyze→analyze, admin→mgmt.{domain}.{op}. ")
	sb.WriteString("`blue help <cmd>` for usage. More skills in `.claude/skills/`.")

	// Only pinned skills get listed explicitly
	sb.WriteString(FormatPinnedSkills(b.config.WorkspaceDir))
	sb.WriteString("</skills>")

	b.skillsCacheStr = sb.String()
	b.skillsCacheTime = time.Now()
	return b.skillsCacheStr
}

// BuildWithContext builds a system prompt with additional context.
func (b *SystemPromptBuilder) BuildWithContext(ctx context.Context, extraPrompt string, contextFiles map[string]string) string {
	var sb strings.Builder

	// Identity + merged behavioral guidance (string literals — no intermediate alloc)
	sb.WriteString("You are a personal assistant running inside ZimaOS Blue. Be clear and concise. Match the user's language. If evidence is insufficient or conflicting, state uncertainty explicitly. Never fabricate sources. Distinguish verified information from inference.")
	sb.WriteString(b.buildSafetyGuidance())
	sb.WriteString(b.buildToolCallStyleGuidance())

	// Agent mode guidance (if enabled)
	if b.isAgentMode() {
		b.writeAgentModeGuidanceTo(&sb)
	}

	// Workspace information
	if b.config.WorkspaceDir != "" {
		b.writeWorkspaceInfoTo(&sb)
	}

	// Available skills (XML index — cached string, fine as-is)
	if s := b.buildSkillsSection(); s != "" {
		sb.WriteString(s)
	}

	// Context files
	if len(contextFiles) > 0 {
		sb.WriteString(b.buildProjectContext(contextFiles))
	}

	sb.WriteString(b.buildSilentReplyGuidance())
	sb.WriteString(b.buildHeartbeatGuidance())
	b.writePlatformInfoTo(&sb)
	b.writeRuntimeInfoTo(&sb)

	// Extra system prompt
	if extraPrompt != "" {
		sb.WriteString(extraPrompt)
	}

	return sb.String()
}

// writeContextFileSectionTo writes a context file section directly into sb.
func (b *SystemPromptBuilder) writeContextFileSectionTo(sb *strings.Builder, name, content string) {
	sb.WriteString("<file name=\"")
	sb.WriteString(name)
	sb.WriteString("\">\n")
	sb.WriteString(content)
	sb.WriteString("\n</file>")
}

// buildContextFileSection returns a context file section as a string.
func (b *SystemPromptBuilder) buildContextFileSection(name, content string) string {
	var sb strings.Builder
	b.writeContextFileSectionTo(&sb, name, content)
	return sb.String()
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
	var sb strings.Builder
	sb.WriteString("<project_context>")

	// Check for SOUL.md
	for _, e := range included {
		if strings.ToLower(e.name) == "soul.md" {
			sb.WriteString("If SOUL.md is present, embody its persona and tone. Avoid stiff, generic replies; follow its guidance unless higher-priority instructions override it.")
			break
		}
	}

	for _, e := range included {
		b.writeContextFileSectionTo(&sb, e.name, e.content)
	}

	sb.WriteString("</project_context>")
	return sb.String()
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
	return "Current time: " + timeutil.NowTime().Format(time.RFC3339) + "\n\nThis is a heartbeat check. Read HEARTBEAT.md if it exists and follow its instructions. If nothing needs attention, reply with HEARTBEAT_OK."
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
