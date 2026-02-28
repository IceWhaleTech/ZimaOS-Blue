package claudecode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	config       *ClaudeCodeConfig
	toolRegistry *tools.Registry
	workspace    *workspace.Manager

	maxContextTokens     int
	lastContextStats     atomic.Pointer[ContextStats]
	agentMode            bool          // when true, inject agent mode guidance
	agentModeFunc        func() bool   // dynamic agent mode getter (takes precedence over static)
	agentAutoConfirmFunc func() bool   // dynamic auto-confirm getter
	locale               string        // user locale (e.g. "en-US", "zh-CN") — static fallback
	localeFunc           func() string // dynamic locale getter (takes precedence over static)

	// staticSystemOnce caches the StaticSystem block (never changes within a process).
	staticSystemOnce sync.Once
	staticSystemStr  string

	// skillsCache caches the skills section with a TTL.
	skillsCacheMu   sync.Mutex
	skillsCacheStr  string
	skillsCacheTime time.Time

	// projectContextCache caches the rendered <project_context> block by file-content hash.
	projectContextCacheMu   sync.Mutex
	projectContextCacheKey  string
	projectContextCacheStr  string
	projectContextCacheStat *ContextStats
	projectContextCacheHit  uint64
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
		sb.WriteString("You are a personal assistant running inside ZimaOS Blue. Be clear and concise. Match the user's language. Your result will be cross-reviewed by claude & codex.")
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

	sb.WriteString("<exec_guide>Shell/CLI commands on host. REQUIRED: command parameter must be a non-empty string — never call exec without a concrete command. `lang` and `timeout` are optional (defaults apply when omitted).")

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
	sb.WriteString("<orchestrator_fsm>State machine is mandatory and explicit: INTAKE -> CLARIFY -> PLAN -> CONFIRM_GATE -> EXECUTE -> VERIFY -> REPORT -> DONE, with RECOVER/ABORTED as controlled exits. Do not skip states. State transitions must be rule-driven, not free-form.</orchestrator_fsm>")

	sb.WriteString("<planning>For multi-step tasks, manage TODOs via plan IPC skills. If no plan exists, call `blue plan_create ...`. If a plan exists, NEVER recreate it. Update incrementally with `blue plan_update ...` or `blue plan_append ...`. Keep the checklist in one place; do not re-output duplicate TODO lists.</planning>")

	sb.WriteString("<execution>")
	sb.WriteString("Before each tool call, briefly state which task you are working on. ")
	if autoConfirm {
		sb.WriteString("Auto-confirm enabled — execute without asking. ")
	} else {
		sb.WriteString("Ask confirmation before destructive actions (delete, install, modify production config). Proceed without confirmation for safe operations. ")
	}
	sb.WriteString("Use exec for file ops, installs, builds, tests. Do NOT stop early. Do NOT call exec without a concrete command — think first, then execute.")
	sb.WriteString(" When facing multiple valid approaches, missing preferences, or trade-offs, call ask before proceeding (do not guess).")
	sb.WriteString(" Ask is a hard gate (not a suggestion). Required ask triggers: missing critical parameters; high-risk actions; conflicting instructions; unclear acceptance criteria; significant strategy trade-offs.")
	sb.WriteString(" Ask protocol: one-line question + 2-5 mutually exclusive options + consequence summary for each option. Mark pending confirmation explicitly with `<awaiting_user_input>true</awaiting_user_input>` and clear it after user response.")
	sb.WriteString(" Ask format: prefer q/mq + a, where a contains 2-4 options. Prefer option objects {label, description, value}; put the recommended option first and append '(Recommended)' to its label.")
	sb.WriteString("</execution>")

	sb.WriteString("<verification>After all steps, verify: run build/tests. Fix and re-verify if needed. If verification fails, enter RECOVER with bounded retries and a clear fallback path.</verification>")

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
	sb.WriteString("Routing: ask→ask, search→web_search, URL→browser, UI review→ui_reviewer, analyze→analyze, plan→plan_create/plan_update/plan_append, sandbox→sandbox, workflows→workflows, scheduler→scheduler, research→deep_research, admin→mgmt.{domain}.{op}. ")
	sb.WriteString("Use progressive skill selection: prefer routed/pinned commands first, then inspect likely SKILL.md files on demand. ")
	sb.WriteString("`blue help <cmd>` for usage. More skills in workspace `.claude/skills/` and user default `~/.claude/skills/`.")

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

	/*
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
		}*/

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

// Per-file soft caps keep single files from dominating prompt budget.
// 0 means no cap.
var contextFileTokenCap = map[string]int{
	"SOUL.md":      1200,
	"USER.md":      900,
	"BOOTSTRAP.md": 900,
	"AGENTS.md":    700,
	"IDENTITY.md":  600,
	"HEARTBEAT.md": 400,
	"MEMORY.md":    700,
}

const (
	defaultContextFileTokenCap = 300
	minContextSliceTokens      = 48
)

// buildProjectContext builds project context from multiple files with token budgeting.
// Files are prioritized: SOUL > USER/BOOTSTRAP > AGENTS > IDENTITY > HEARTBEAT > MEMORY > daily logs.
// If total tokens exceed the budget, low-priority files are dropped first.
func (b *SystemPromptBuilder) buildProjectContext(contextFiles map[string]string) string {
	budget := b.maxContextTokens
	cacheKey := buildProjectContextCacheKey(contextFiles, budget)

	b.projectContextCacheMu.Lock()
	if b.projectContextCacheKey == cacheKey {
		cached := b.projectContextCacheStr
		if stat := cloneContextStats(b.projectContextCacheStat); stat != nil {
			b.lastContextStats.Store(stat)
		}
		b.projectContextCacheHit++
		b.projectContextCacheMu.Unlock()
		return cached
	}
	b.projectContextCacheMu.Unlock()

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
		content = workspace.NormalizeDefaultTemplateToEnglish(name, content)
		// Ultra-compact mode: normalize markdown and collapse line breaks/whitespace.
		content = pruner.MarkdownToTextMinimal(content)
		if content == "" {
			continue
		}
		if cap := contextFileTokenSoftCap(name); cap > 0 {
			content, _, _ = truncateToTokenBudget(content, cap)
			if content == "" {
				continue
			}
		}
		prio, ok := contextFilePriority[name]
		if !ok {
			prio = 10 // Daily logs and unknown files get lowest priority
		}
		tokens := pruner.EstimateTokens(content)
		entries = append(entries, fileEntry{
			name:     name,
			content:  content,
			tokens:   tokens,
			priority: prio,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].priority != entries[j].priority {
			return entries[i].priority < entries[j].priority
		}
		// Keep deterministic order for same-priority files to maximize prompt-cache hits.
		return entries[i].name < entries[j].name
	})

	// Select files within budget
	stats := &ContextStats{
		Files:        make([]ContextFileStat, 0, len(entries)),
		BudgetTokens: budget,
	}
	var included []fileEntry
	totalTokens := 0

	for _, e := range entries {
		entry := e
		trimmed := false
		if budget > 0 {
			remaining := budget - totalTokens
			if remaining <= 0 {
				stats.Files = append(stats.Files, ContextFileStat{
					Name: entry.name, Tokens: entry.tokens, Included: false, Trimmed: true,
				})
				stats.Trimmed = true
				continue
			}
			if entry.tokens > remaining {
				if remaining < minContextSliceTokens && len(included) > 0 {
					// Keep at least one high-priority file and skip tiny tail slices.
					stats.Files = append(stats.Files, ContextFileStat{
						Name: entry.name, Tokens: entry.tokens, Included: false, Trimmed: true,
					})
					stats.Trimmed = true
					continue
				}
				// Last-fit slicing: include a trimmed slice instead of dropping whole file.
				sliced, slicedTokens, ok := truncateToTokenBudget(entry.content, remaining)
				if !ok || sliced == "" || slicedTokens == 0 {
					stats.Files = append(stats.Files, ContextFileStat{
						Name: entry.name, Tokens: entry.tokens, Included: false, Trimmed: true,
					})
					stats.Trimmed = true
					continue
				}
				entry.content = sliced
				entry.tokens = slicedTokens
				trimmed = true
				stats.Trimmed = true
			}
		}
		included = append(included, entry)
		totalTokens += entry.tokens
		stats.Files = append(stats.Files, ContextFileStat{
			Name: entry.name, Tokens: entry.tokens, Included: true, Trimmed: trimmed,
		})
	}
	stats.TotalTokens = totalTokens
	b.lastContextStats.Store(stats)

	projectContext := ""
	if len(included) == 0 {
		// Keep empty cached too, to avoid repeated heavy token estimation work.
	} else {
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
		projectContext = sb.String()
	}

	b.projectContextCacheMu.Lock()
	b.projectContextCacheKey = cacheKey
	b.projectContextCacheStr = projectContext
	b.projectContextCacheStat = cloneContextStats(stats)
	b.projectContextCacheMu.Unlock()

	return projectContext
}

func contextFileTokenSoftCap(name string) int {
	if cap, ok := contextFileTokenCap[name]; ok {
		return cap
	}
	return defaultContextFileTokenCap
}

func buildProjectContextCacheKey(contextFiles map[string]string, budget int) string {
	keys := make([]string, 0, len(contextFiles))
	for name := range contextFiles {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	h := sha256.New()
	h.Write([]byte(strconv.Itoa(budget)))
	h.Write([]byte{0})
	for _, name := range keys {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(contextFiles[name]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func cloneContextStats(stats *ContextStats) *ContextStats {
	if stats == nil {
		return nil
	}
	cp := &ContextStats{
		TotalTokens:  stats.TotalTokens,
		BudgetTokens: stats.BudgetTokens,
		Trimmed:      stats.Trimmed,
	}
	if len(stats.Files) > 0 {
		cp.Files = make([]ContextFileStat, len(stats.Files))
		copy(cp.Files, stats.Files)
	}
	return cp
}

// truncateToTokenBudget truncates content to fit maxTokens and returns:
// truncated content, estimated tokens, and whether truncation happened.
func truncateToTokenBudget(content string, maxTokens int) (string, int, bool) {
	if content == "" {
		return "", 0, false
	}
	if maxTokens <= 0 {
		return "", 0, true
	}
	tokens := pruner.EstimateTokens(content)
	if tokens <= maxTokens {
		return content, tokens, false
	}

	runes := []rune(content)
	lo, hi := 0, len(runes)
	best := 0
	for lo <= hi {
		mid := lo + (hi-lo)/2
		cur := strings.TrimSpace(string(runes[:mid]))
		curTokens := pruner.EstimateTokens(cur)
		if curTokens <= maxTokens {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best <= 0 {
		return "", 0, true
	}

	cut := strings.TrimSpace(string(runes[:best]))
	if idx := strings.LastIndex(cut, "\n"); idx > 0 && idx >= len(cut)/2 {
		cut = strings.TrimSpace(cut[:idx])
	}
	if cut == "" {
		return "", 0, true
	}
	trimmed := cut + "\n\n[Context truncated to fit token budget.]"
	trimmedTokens := pruner.EstimateTokens(trimmed)
	if trimmedTokens <= maxTokens {
		return trimmed, trimmedTokens, true
	}
	// Suffix pushed us over budget; return raw cut.
	return cut, pruner.EstimateTokens(cut), true
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
