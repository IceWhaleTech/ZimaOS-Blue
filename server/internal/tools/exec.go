package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"

	cardconv "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cards"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ExecConfig configures the exec tool.
type ExecConfig struct {
	Host           string        // "local" or "sandbox"
	Security       ExecSecurity  // "deny", "allowlist", "full"
	DefaultTimeout time.Duration // default per-command timeout
	MaxTimeout     time.Duration // maximum allowed timeout
	MaxOutput      int           // max output chars per stream
	SafeBins       []string      // auto-allowed binaries
	DataDir        string        // for allowlist JSON persistence
	AllowPTY       bool          // whether PTY mode is available
	AllowedDirs    []string      // directories the exec tool may operate in; empty = unrestricted
	Shell          string        // override shell binary (empty = auto-detect via GetShellConfig)
	ShellArgs      []string      // override shell args (empty = auto-detect)
	Policy         *ExecPolicy   // runtime policy (nil = default)
}

// SandboxExecutor is the interface for sandbox execution, decoupled from the
// sandbox package to avoid pulling in heavy transitive dependencies.
type SandboxExecutor interface {
	// RunInSandbox executes a shell command in an isolated environment.
	// Returns stdout, stderr, exit code, and any error.
	RunInSandbox(ctx context.Context, command, workdir string, env map[string]string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
}

// SandboxTier describes the isolation level selected for a sandboxed exec.
type SandboxTier string

const (
	SandboxTierLight  SandboxTier = "light"
	SandboxTierStrong SandboxTier = "strong"
)

// DefaultExecConfig returns sensible defaults.
func DefaultExecConfig() ExecConfig {
	return ExecConfig{
		Host:           "local",
		Security:       ExecSecurityFull,
		DefaultTimeout: 5 * time.Minute,
		MaxTimeout:     30 * time.Minute,
		MaxOutput:      200_000,
		SafeBins:       DefaultSafeBins,
		AllowPTY:       true,
	}
}

// ExecTool implements the Tool interface for shell command execution.
type ExecTool struct {
	config        ExecConfig
	policy        ExecPolicy
	sessions      *SessionRegistry
	approvals     *ApprovalManager // may be nil
	broker        *sse.Broker      // may be nil; used for lifecycle events
	safeBins      map[string]struct{}
	dirStore      *DirAllowlistStore  // may be nil; persistent directory allowlist
	sandboxLight  SandboxExecutor     // may be nil; lightweight sandbox backend
	sandboxStrong SandboxExecutor     // may be nil; stronger sandbox backend
	toolNames     map[string]struct{} // known tool names; exec rejects commands that match
	registry      *Registry           // may be nil; when set, exec auto-forwards tool-name commands
	retries       *RetryTracker       // prevents same-command retry loops
	audit         *ExecAuditStore     // may be nil; persistent audit log
	skillExec     SkillExecFunc       // may be nil; short-circuits `blue <skill>` commands
	pinnedSkills  map[string]struct{} // pinned skill names for short-circuit (e.g. web_search, browser)
	skillSelect   SkillSelectFunc     // may be nil; selector fallback for unknown skills
	autoConfirm   func() bool         // optional dynamic auto-confirm getter
	approvalMu    sync.RWMutex
	approvedCmds  map[string]struct{} // exact command digests approved with "allow always"
}

// NewExecTool creates a new exec tool.
func NewExecTool(config ExecConfig, sessions *SessionRegistry, approvals *ApprovalManager, broker *sse.Broker, dirStore *DirAllowlistStore, sbx ...SandboxExecutor) *ExecTool {
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 5 * time.Minute
	}
	if config.MaxTimeout <= 0 {
		config.MaxTimeout = 30 * time.Minute
	}
	if config.MaxOutput <= 0 {
		config.MaxOutput = 200_000
	}
	policy := DefaultExecPolicy()
	if config.Policy != nil {
		policy = *config.Policy
	}
	return &ExecTool{
		config:        config,
		policy:        policy,
		sessions:      sessions,
		approvals:     approvals,
		broker:        broker,
		safeBins:      BuildSafeBinsSet(config.SafeBins),
		dirStore:      dirStore,
		sandboxLight:  firstOrNilIface(sbx),
		sandboxStrong: secondOrNilIface(sbx),
		retries:       NewRetryTracker(policy.MaxRetries, policy.RetryWindow),
		approvedCmds:  make(map[string]struct{}),
	}
}

func firstOrNilIface[T any](s []T) T {
	if len(s) > 0 {
		return s[0]
	}
	var zero T
	return zero
}

func secondOrNilIface[T any](s []T) T {
	if len(s) > 1 {
		return s[1]
	}
	var zero T
	return zero
}

// SetToolNames sets the known tool names so exec can reject commands that
// look like tool invocations (e.g. "web_search query" instead of calling
// the web_search tool directly).
func (t *ExecTool) SetToolNames(names []string) {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n != "exec" && n != "process" && n != "bash" { // don't block shell entrypoints themselves
			m[n] = struct{}{}
		}
	}
	t.toolNames = m
	slog.Info("[exec] SetToolNames", "count", len(m), "names", names)
}

// SetRegistry sets the tool registry so exec can auto-forward commands that
// match a tool name (e.g. "web_search latest news") to the actual tool
// instead of returning an error.
func (t *ExecTool) SetRegistry(r *Registry) {
	t.registry = r
}

// SkillExecFunc executes a skill by ID with the given input.
// Returns (map[string]string, error) matching the IPC SkillExecutor interface.
type SkillExecFunc func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error)

// SkillSelectionDecision is the lightweight selector output consumed by exec fallback.
type SkillSelectionDecision struct {
	SelectedSkill string
	Confidence    float64
	NeedClarify   bool
	Candidates    []string
	Reason        string
}

// SkillSelectFunc is called when exec receives an unknown/disabled skill command.
type SkillSelectFunc func(ctx context.Context, query string) SkillSelectionDecision

// SetSkillExecutor sets the skill executor for short-circuiting `blue <skill>` commands.
func (t *ExecTool) SetSkillExecutor(fn SkillExecFunc) {
	t.skillExec = fn
}

// SetSkillSelector sets the progressive selector for unknown skills.
func (t *ExecTool) SetSkillSelector(fn SkillSelectFunc) {
	t.skillSelect = fn
}

// SetAutoConfirmFunc sets a dynamic auto-confirm getter for destructive skill gating.
func (t *ExecTool) SetAutoConfirmFunc(fn func() bool) {
	t.autoConfirm = fn
}

// SetPinnedSkills sets the pinned skill names for short-circuiting skill commands
// (e.g. "web_search query" → skill executor). Unlike tool auto-forward, this avoids
// registering skills as tools (which would consume extra prompt tokens).
func (t *ExecTool) SetPinnedSkills(names []string) {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[n] = struct{}{}
	}
	t.pinnedSkills = m
	slog.Info("[exec] SetPinnedSkills", "count", len(m), "names", names)
}

// getShellConfig returns the shell and args, preferring config overrides.
func (t *ExecTool) getShellConfig() (string, []string) {
	if t.config.Shell != "" {
		return t.config.Shell, t.config.ShellArgs
	}
	return GetShellConfig()
}

// SetAuditStore sets the persistent audit log store. Call after construction
// when the DB is available (deferred wiring pattern).
func (t *ExecTool) SetAuditStore(store *ExecAuditStore) {
	t.audit = store
}

func (t *ExecTool) isCommandApprovedAlways(command string) bool {
	if t == nil {
		return false
	}
	digest := approvalCommandDigest(command)
	t.approvalMu.RLock()
	defer t.approvalMu.RUnlock()
	_, ok := t.approvedCmds[digest]
	return ok
}

func (t *ExecTool) rememberApprovedCommand(command string) {
	if t == nil {
		return
	}
	digest := approvalCommandDigest(command)
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	t.approvedCmds[digest] = struct{}{}
}

func (t *ExecTool) requestCommandSafetyApproval(ctx context.Context, command, workdir, host string, match *CommandSafetyMatch) error {
	if match == nil || !match.RequiresApproval {
		return nil
	}
	if t.isCommandApprovedAlways(command) {
		return nil
	}
	if t.approvals == nil {
		return newToolRuntimeError("exec_approval_unavailable", fmt.Sprintf("exec blocked: %s requires approval and no approval manager is configured", match.Reason), nil, map[string]interface{}{
			"kind":       "command",
			"command":    strings.TrimSpace(command),
			"workdir":    strings.TrimSpace(workdir),
			"host":       strings.TrimSpace(host),
			"risk_level": string(match.RiskLevel),
		})
	}

	userID := GetUserID(ctx)
	decision, err := t.approvals.RequestApproval(ctx, ApprovalRequest{
		Type:         "command",
		Command:      command,
		Workdir:      workdir,
		Host:         host,
		Security:     string(t.config.Security),
		UserID:       userID,
		PolicySource: "exec_command_safety",
		RiskLevel:    string(match.RiskLevel),
	})
	if err != nil {
		return fmt.Errorf("exec denied: approval failed: %w", err)
	}

	switch decision {
	case ApprovalAllowOnce:
		return nil
	case ApprovalAllowAlways:
		t.rememberApprovedCommand(command)
		return nil
	default:
		return newToolRuntimeError("exec_approval_denied", "exec denied: user denied the command", nil, map[string]interface{}{
			"kind":    "command",
			"command": strings.TrimSpace(command),
			"workdir": strings.TrimSpace(workdir),
			"host":    strings.TrimSpace(host),
		})
	}
}

// HasSandbox returns true if sandbox execution is available.
func (t *ExecTool) HasSandbox() bool {
	return t.sandboxLight != nil || t.sandboxStrong != nil
}

// Policy returns the current exec policy (read-only).
func (t *ExecTool) Policy() ExecPolicy {
	return t.policy
}

// Definition returns the tool definition for the LLM.
func (t *ExecTool) Definition() ToolDefinition {
	desc := "Execute shell commands on the host. Returns stdout, stderr, exit code, and session ID. Also manages exec sessions via action=list|poll|log|kill with session_id."
	if t.HasSandbox() {
		desc += " Sandbox mode is available for isolated execution — set host to 'sandbox' for filesystem-level isolation."
	}

	props := map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"description": "Optional exec session action: list, poll, log, or kill. When set, exec manages existing sessions instead of running a new command.",
			"enum":        []string{"list", "poll", "log", "kill", "list_sessions", "poll_session", "session_log", "kill_session"},
		},
		"session_id": map[string]interface{}{
			"type":        "string",
			"description": "Session ID used with action=poll, log, or kill.",
		},
		"command": map[string]interface{}{
			"type":        "string",
			"description": "Shell command to execute",
		},
		"workdir": map[string]interface{}{
			"type":        "string",
			"description": "Working directory (defaults to server cwd)",
		},
		"lang": map[string]interface{}{
			"type":        "string",
			"description": "Optional locale for command execution (e.g. en-US, zh-CN, ja-JP). When omitted, uses the system/default environment locale.",
		},
		"env": map[string]interface{}{
			"type":        "object",
			"description": "Additional environment variables",
			"additionalProperties": map[string]interface{}{
				"type": "string",
			},
		},
		"timeout": map[string]interface{}{
			"type":        "number",
			"description": "Optional timeout in seconds (default 300, max 1800).",
		},
		"pty": map[string]interface{}{
			"type":        "boolean",
			"description": "Run in a pseudo-terminal (for interactive commands)",
		},
	}

	// Only expose host parameter when sandbox is available.
	if t.HasSandbox() {
		props["host"] = map[string]interface{}{
			"type":        "string",
			"enum":        []string{"local", "sandbox"},
			"description": "Execution environment. 'local' runs directly on host (default). 'sandbox' runs in an isolated environment with filesystem restrictions.",
		}
	}

	return ToolDefinition{
		Name:        "exec",
		Description: desc,
		Icon:        "terminal",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": props,
			"anyOf": []map[string]interface{}{
				{"required": []string{"command"}},
				{"required": []string{"action"}},
			},
		},
	}
}

// execResult is the JSON response returned to the LLM.
type execResult struct {
	SessionID   string            `json:"session_id"`
	Status      string            `json:"status"`
	ExitCode    *int              `json:"exit_code,omitempty"`
	Stdout      string            `json:"stdout,omitempty"`
	Stderr      string            `json:"stderr,omitempty"`
	Data        map[string]string `json:"data,omitempty"`
	DurationMs  int64             `json:"duration_ms"`
	Truncated   bool              `json:"truncated,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
	Host        string            `json:"host,omitempty"`       // "local", "sandbox", or "builtin"
	RiskLevel   string            `json:"risk_level,omitempty"` // risk assessment level
	SandboxTier string            `json:"sandbox_tier,omitempty"`
}

// Execute runs the shell command.
func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if action := strings.TrimSpace(strings.ToLower(firstCompatString(args, "action", "op", "operation"))); action != "" {
		return executeProcessAction(t.sessions, args)
	}

	command := strings.TrimSpace(firstCompatString(args, "command", "cmd"))
	if command == "" {
		return nil, errors.New("command is required")
	}
	workdirArg := firstCompatString(args, "workdir", "cwd", "work_dir", "working_dir", "workDir", "workingDir")
	if normalizedCommand, normalizedWorkdir, ok := normalizeBlueCLIExecCommand(command, workdirArg); ok {
		command = normalizedCommand
		workdirArg = normalizedWorkdir
	}
	strictShell, _ := compatBoolArg(args, execStrictShellArg)

	if !strictShell {
		if result, intercepted, err := t.tryCompatAskCarrier(ctx, command); intercepted {
			if err != nil {
				return nil, err
			}
			return result, nil
		}
	}

	// Intercept commands that look like tool invocations.
	// The LLM sometimes tries to call tools via exec (e.g. "web_search query").
	// If we have the registry, auto-forward to the real tool. Otherwise return error.
	// Also check pinned skills for short-circuit.
	firstWord := command
	restArgs := ""
	if idx := strings.IndexAny(command, " \t\n"); idx > 0 {
		firstWord = command[:idx]
		restArgs = strings.TrimSpace(command[idx+1:])
	}
	isBlueCommand := strings.HasPrefix(strings.TrimSpace(command), "blue ")
	_, isPinnedSkill := t.pinnedSkills[firstWord]
	_, isToolName := t.toolNames[firstWord]
	slog.Info("[exec] dispatch",
		"first_word", firstWord,
		"is_blue_command", isBlueCommand,
		"is_tool_name", isToolName,
		"is_pinned_skill", isPinnedSkill,
		"has_registry", t.registry != nil,
		"has_skill_exec", t.skillExec != nil,
		"has_rest_args", restArgs != "",
		"strict_shell", strictShell,
	)

	// Short-circuit simple `blue <skill> ...` commands before shell-specific
	// security/path handling. Once we dispatch into the skill executor, the
	// request is no longer a shell command and should not be blocked by
	// shell-level PATH or directory validation. In strict-shell mode we only
	// allow this for single blue CLI invocations without shell operators, so
	// the public bash surface keeps its shell-hardening boundary.
	if t.skillExec != nil && isBlueCommand && (!strictShell || canStrictShellBlueSkillShortCircuit(command)) {
		slog.Info("[exec] trying blue skill short-circuit", "command", truncateStr(command, 200))
		if result, ok := t.trySkillShortCircuit(ctx, command, nil, workdirArg); ok {
			return result, nil
		}
		slog.Info("[exec] blue skill short-circuit not taken", "command", truncateStr(command, 200))
	}

	if !strictShell && len(t.toolNames) > 0 {
		if isToolName {
			if t.registry != nil {
				if tool := t.registry.Get(firstWord); tool != nil {
					slog.Info("[exec] auto-forwarding to tool", "tool", firstWord, "args", restArgs)
					// Build args map from the rest of the command.
					fwdArgs := make(map[string]interface{})
					if restArgs != "" {
						if firstWord == "web_search" {
							fwdArgs["query"] = restArgs
						} else {
							fwdArgs["input"] = restArgs
						}
					} else if !toolAllowsEmptyExecForward(tool) {
						// No arguments provided and the tool schema marks parameters as required.
						return nil, fmt.Errorf(
							"%s requires arguments. Call the %s tool directly with the proper parameters instead of using exec",
							firstWord, firstWord,
						)
					}
					result, err := tool.Execute(ctx, fwdArgs)
					if err != nil {
						return nil, err
					}
					return &ForwardedResult{ActualTool: firstWord, Result: result}, nil
				}
				slog.Warn("[exec] tool in registry returned nil", "tool", firstWord)
			}
			slog.Warn("[exec] tool name matched but no registry or tool not found", "tool", firstWord, "hasRegistry", t.registry != nil)
			return nil, fmt.Errorf(
				"%s is a tool, not a shell command. Call the %s tool directly instead of using exec",
				firstWord, firstWord,
			)
		}
	}

	// Short-circuit pinned skills (e.g. "web_search query" → skill executor).
	// This avoids registering skills as tools (which would consume extra prompt tokens).
	if !strictShell && t.skillExec != nil && len(t.pinnedSkills) > 0 {
		if isPinnedSkill {
			if restArgs == "" {
				return nil, fmt.Errorf(
					"%s requires arguments. Call with 'blue %s <args>' or use the %s skill directly",
					firstWord, firstWord, firstWord,
				)
			}
			if shouldParsePinnedSkillArgs(firstWord, restArgs) {
				if result, ok := t.trySkillShortCircuit(ctx, command, nil, ""); ok {
					return result, nil
				}
				return nil, fmt.Errorf("invalid %s arguments", firstWord)
			}

			input, err := buildPinnedSkillFreeTextInput(firstWord, restArgs)
			if err != nil {
				return nil, err
			}
			slog.Info("[exec] pinned skill short-circuit", "skill", firstWord, "input", restArgs, "source", "direct_command")
			data, err := t.skillExec(ctx, firstWord, input)
			if err != nil {
				slog.Warn("[exec] pinned skill short-circuit failed",
					"skill", firstWord,
					"input", restArgs,
					"error", err)
				return nil, err
			}
			slog.Info("[exec] pinned skill short-circuit success",
				"skill", firstWord,
				"input", restArgs)
			emitSkillResultCardFromData(ctx, firstWord, data)
			return &ForwardedResult{ActualTool: firstWord, Result: data}, nil
		}
	} else if !strictShell && t.toolNames == nil {
		slog.Warn("[exec] toolNames is nil, auto-forward disabled", "command", command)
	}

	langArg := firstCompatString(args, "lang", "language")
	envRaw, _ := compatArgValue(args, "env")
	envArg := parseEnvArg(envRaw)
	timeoutRaw, _ := compatArgValue(args, "timeout", "timeout_sec", "timeout_seconds", "timeoutSeconds")
	timeoutSec := parseFloatArg(timeoutRaw, t.config.DefaultTimeout.Seconds())
	usePTY, _ := compatBoolArg(args, "pty", "use_pty", "usePty")
	hostArg := firstCompatString(args, "host")
	hostExplicit := hostArg != "" // user explicitly chose a host
	if hostArg == "" {
		hostArg = t.config.Host
	}

	safetyMatch := MatchCommandSafety(command)
	if safetyMatch != nil {
		if safetyMatch.RequiresApproval {
			if err := t.requestCommandSafetyApproval(ctx, command, workdirArg, hostArg, safetyMatch); err != nil {
				t.recordAudit(ctx, command, workdirArg, 100, safetyMatch.RiskLevel, "blocked", nil, 0, 0, 0, err.Error())
				return nil, err
			}
		} else {
			err := fmt.Errorf("exec blocked: %s", safetyMatch.Reason)
			t.recordAudit(ctx, command, workdirArg, 100, safetyMatch.RiskLevel, "blocked", nil, 0, 0, 0, err.Error())
			return nil, err
		}
	}

	// Command length check.
	if t.policy.MaxCommandLen > 0 && len(command) > t.policy.MaxCommandLen {
		err := fmt.Errorf("exec blocked: command length %d exceeds maximum %d", len(command), t.policy.MaxCommandLen)
		return nil, err
	}

	// Risk scoring — evaluate command risk and enforce policy threshold.
	risk := AnalyzeRisk(command)
	if safetyMatch != nil && safetyMatch.RequiresApproval {
		risk = AnalyzeRiskWithSuppressedReasons(command, safetyMatch.SuppressRiskReasons)
	}
	if risk.Total >= t.policy.MaxRiskThreshold {
		err := fmt.Errorf("exec blocked: risk score %d (%s) exceeds policy threshold %d. Reasons: %s",
			risk.Total, risk.Level, t.policy.MaxRiskThreshold, strings.Join(risk.Reasons, ", "))
		t.recordAudit(ctx, command, workdirArg, risk.Total, risk.Level, "blocked", nil, 0, 0, 0, err.Error())
		return nil, err
	}

	// Retry control — prevent same-command retry loops.
	normalizedCmd := NormalizeCommand(command)
	if err := t.retries.Check(normalizedCmd); err != nil {
		t.recordAudit(ctx, command, workdirArg, risk.Total, risk.Level, "blocked", nil, 0, 0, 0, err.Error())
		return nil, err
	}

	// Resolve timeout.
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout <= 0 {
		timeout = t.config.DefaultTimeout
	}
	if timeout > t.config.MaxTimeout {
		timeout = t.config.MaxTimeout
	}

	var warnings []string

	// Security check.
	if err := t.checkSecurity(ctx, command, workdirArg, &warnings); err != nil {
		return nil, err
	}

	// Validate env vars for host execution.
	if len(envArg) > 0 {
		if err := ValidateHostEnv(envArg); err != nil {
			return nil, err
		}
	}

	// Resolve workdir.
	workdir, wdWarnings := ResolveWorkdir(workdirArg)
	warnings = append(warnings, wdWarnings...)

	// When AllowedDirs is configured and no explicit workdir was given,
	// default to the first allowed directory instead of os.Getwd().
	if workdirArg == "" && len(t.config.AllowedDirs) > 0 {
		workdir = t.config.AllowedDirs[0]
	}

	// Enforce allowed directories.
	if err := t.validateWorkdir(ctx, workdir); err != nil {
		return nil, err
	}

	// Validate paths referenced in the command against allowed dirs.
	if err := t.validateCommandPaths(ctx, command, workdir, &warnings); err != nil {
		return nil, err
	}

	// If lang (locale) is specified, inject LANG/LC_ALL into the environment.
	if langArg = strings.TrimSpace(langArg); langArg != "" {
		locale := localeFromTag(langArg)
		if envArg == nil {
			envArg = make(map[string]string, 2)
		}
		envArg["LANG"] = locale
		envArg["LC_ALL"] = locale
	}

	// Propagate user/session context into blue CLI subprocesses so IPC fallback
	// keeps skill actions (e.g. reminder delivery) bound to the source chat.
	if isBlueCommand {
		if envArg == nil {
			envArg = make(map[string]string, 2)
		}
		if _, exists := envArg["BLUE_USER_ID"]; !exists {
			if userID := strings.TrimSpace(GetUserID(ctx)); userID != "" {
				envArg["BLUE_USER_ID"] = userID
			}
		}
		if _, exists := envArg["BLUE_SESSION_ID"]; !exists {
			if sessionID := strings.TrimSpace(GetSessionID(ctx)); sessionID != "" {
				envArg["BLUE_SESSION_ID"] = sessionID
			}
		}
	}

	// Build environment.
	env := buildExecEnv(envArg)
	execCommand := rewriteBlueCLIExecutable(command)

	// Auto-upgrade to sandbox for medium+ risk commands when sandbox is available
	// and the caller didn't explicitly choose a host.
	if !hostExplicit && t.HasSandbox() && risk.Total >= 30 {
		warnings = append(warnings, fmt.Sprintf("auto-sandboxed: risk level %s (score %d)", risk.Level, risk.Total))
		hostArg = "sandbox"
	}

	// Force `blue <subcommand>` to run on host — these are IPC calls to the
	// main process and the sandbox doesn't have the blue binary in PATH.
	// Use "builtin" as host value so UI can display a green shield (safe).
	if hostArg == "sandbox" && strings.HasPrefix(strings.TrimSpace(command), "blue ") {
		hostArg = "builtin"
		warnings = append(warnings, "blue subcommand forced to host (IPC)")
	}

	// Sandbox mode: delegate to sandbox.Manager for filesystem-level isolation.
	if hostArg == "sandbox" {
		return t.runSandbox(ctx, execCommand, workdir, envArg, timeout, risk, warnings)
	}

	// Create session.
	sessionID := NewSessionID()
	session := &ProcessSession{
		ID:        sessionID,
		Command:   command,
		Workdir:   workdir,
		StartedAt: timeutil.NowTime(),
		Stdout:    NewOutputBuffer(t.config.MaxOutput),
		Stderr:    NewOutputBuffer(t.config.MaxOutput),
		Status:    ProcessRunning,
	}
	t.sessions.Add(session)

	// Publish start event.
	userID := GetUserID(ctx)
	t.publishEvent(userID, "exec:started", map[string]interface{}{
		"session_id": sessionID,
		"command":    truncateStr(command, 200),
		"workdir":    workdir,
	})

	// Execute.
	startedAt := time.Now()
	var exitCode *int
	var execErr error

	if usePTY && t.config.AllowPTY {
		exitCode, execErr = t.runWithPTY(ctx, session, execCommand, workdir, env, timeout)
	} else {
		exitCode, execErr = t.runDirect(ctx, session, execCommand, workdir, env, timeout)
	}

	durationMs := time.Since(startedAt).Milliseconds()

	// Determine status.
	status := ProcessCompleted
	if execErr != nil {
		status = ProcessFailed
	}
	if exitCode != nil && *exitCode != 0 {
		status = ProcessFailed
	}

	t.sessions.MarkExited(sessionID, exitCode, "", status)

	// Publish completion event.
	t.publishEvent(userID, "exec:completed", map[string]interface{}{
		"session_id":  sessionID,
		"status":      string(status),
		"exit_code":   exitCode,
		"duration_ms": durationMs,
	})

	// Build result.
	stdout := session.Stdout.String()
	stderr := session.Stderr.String()
	truncated := session.Stdout.Truncated() || session.Stderr.Truncated()

	result := execResult{
		SessionID:  sessionID,
		Status:     string(status),
		ExitCode:   exitCode,
		Stdout:     SanitizeBinaryOutput(stdout),
		Stderr:     SanitizeBinaryOutput(stderr),
		DurationMs: durationMs,
		Truncated:  truncated,
		Warnings:   warnings,
		Host:       "local",
		RiskLevel:  string(risk.Level),
	}

	// Track retries — record failure so repeated identical commands get blocked.
	failed := status == ProcessFailed
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
	} else if failed && len(stderr) > 0 {
		errMsg = truncateStr(stderr, 200)
	}
	t.retries.Record(normalizedCmd, failed, errMsg)

	// Audit log.
	t.recordAudit(ctx, command, workdir, risk.Total, risk.Level, "allowed", exitCode, durationMs, len(stdout), len(stderr), errMsg)

	data, _ := json.Marshal(result)
	return string(data), execErr
}

func (t *ExecTool) checkSecurity(ctx context.Context, command, workdir string, warnings *[]string) error {
	switch t.config.Security {
	case ExecSecurityDeny:
		return errors.New("exec denied: security mode is 'deny'")

	case ExecSecurityFull:
		return nil

	case ExecSecurityAllowlist:
		analysis := AnalyzeCommand(command, workdir)
		if !analysis.OK {
			return fmt.Errorf("exec denied: unable to analyze command: %s", analysis.Reason)
		}

		// Load allowlist.
		allowlistPath := filepath.Join(t.config.DataDir, "exec-approvals.json")
		cfg, err := LoadAllowlist(allowlistPath)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("Warning: failed to load allowlist: %v", err))
			cfg = &AllowlistConfig{Version: 1}
		}

		satisfied, matches := EvaluateAllowlist(analysis, cfg.Entries, t.safeBins)
		if satisfied {
			// Record usage.
			for i := range matches {
				RecordAllowlistUse(cfg, &matches[i], command, analysis.Segments[0].ResolvedPath)
			}
			_ = SaveAllowlist(allowlistPath, cfg)
			return nil
		}

		// Need approval.
		if t.approvals == nil {
			return errors.New("exec denied: command not in allowlist and no approval manager configured")
		}

		userID := GetUserID(ctx)
		decision, err := t.approvals.RequestApproval(ctx, ApprovalRequest{
			Type:     "command",
			Command:  command,
			Workdir:  workdir,
			Host:     t.config.Host,
			Security: string(t.config.Security),
			UserID:   userID,
		})
		if err != nil {
			return fmt.Errorf("exec denied: approval failed: %w", err)
		}

		switch decision {
		case ApprovalAllowOnce:
			return nil
		case ApprovalAllowAlways:
			// Add to allowlist.
			if len(analysis.Segments) > 0 && analysis.Segments[0].ResolvedPath != "" {
				AddAllowlistEntry(cfg, analysis.Segments[0].ResolvedPath)
				_ = SaveAllowlist(allowlistPath, cfg)
			}
			return nil
		default:
			return errors.New("exec denied: user denied the command")
		}

	default:
		return fmt.Errorf("exec denied: unknown security mode %q", t.config.Security)
	}
}

func (t *ExecTool) runDirect(ctx context.Context, session *ProcessSession, command, workdir string, env []string, timeout time.Duration) (*int, error) {
	shell, shellArgs := t.getShellConfig()

	// Use plain exec.Command (not CommandContext) so we control the kill path.
	// exec.CommandContext sends SIGKILL to the process only (not the group),
	// which orphans child processes. We use a manual timer + KillProcessTree instead.
	cmd := exec.Command(shell, append(shellArgs, command)...)
	cmd.Dir = workdir
	cmd.Env = env
	cmd.SysProcAttr = newSysProcAttr()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}
	session.PID = cmd.Process.Pid

	// Read stdout and stderr concurrently.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readIntoBufferWithCards(ctx, stdoutPipe, session.Stdout)
	}()
	go func() {
		defer wg.Done()
		readIntoBuffer(stderrPipe, session.Stderr)
	}()

	// Wait for process in a goroutine so we can select on timeout/cancel.
	doneCh := make(chan error, 1)
	go func() {
		wg.Wait()
		doneCh <- cmd.Wait()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-doneCh:
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				code = -1
				return &code, err
			}
		}
		return &code, nil

	case <-timer.C:
		KillProcessTree(session.PID)
		code := -1
		return &code, fmt.Errorf("command timed out after %s", timeout)

	case <-ctx.Done():
		KillProcessTree(session.PID)
		code := -1
		return &code, ctx.Err()
	}
}

func (t *ExecTool) runWithPTY(ctx context.Context, session *ProcessSession, command, workdir string, env []string, timeout time.Duration) (*int, error) {
	shell, shellArgs := t.getShellConfig()

	cmd := exec.Command(shell, append(shellArgs, command)...)
	cmd.Dir = workdir
	cmd.Env = env
	cmd.SysProcAttr = newSysProcAttr()

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 120})
	if err != nil {
		// Fall back to direct execution.
		return t.runDirect(ctx, session, command, workdir, env, timeout)
	}
	defer ptmx.Close()

	session.PID = cmd.Process.Pid

	// Set up timeout.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Read PTY output.
	doneCh := make(chan error, 1)
	go func() {
		readIntoBuffer(ptmx, session.Stdout)
		doneCh <- cmd.Wait()
	}()

	select {
	case err := <-doneCh:
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				code = -1
				return &code, err
			}
		}
		return &code, nil

	case <-timer.C:
		KillProcessTree(session.PID)
		code := -1
		return &code, fmt.Errorf("command timed out after %s", timeout)

	case <-ctx.Done():
		KillProcessTree(session.PID)
		code := -1
		return &code, ctx.Err()
	}
}

func readIntoBuffer(r io.Reader, buf *OutputBuffer) {
	tmp := make([]byte, 8192)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Append(string(tmp[:n]))
		}
		if err != nil {
			return
		}
	}
}

const (
	cardPrefix = "__CARD__"
	cardSuffix = "__END__"
)

// readIntoBufferWithCards reads from r line-by-line, extracting __CARD__...__END__
// lines and forwarding them to the CardEmitter via ctx. Non-card content is
// appended to buf as usual. This enables streaming card emission from blue
// subcommands running as child processes.
func readIntoBufferWithCards(ctx context.Context, r io.Reader, buf *OutputBuffer) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024) // up to 256KB lines
	for scanner.Scan() {
		line := scanner.Text()
		if card, ok := extractCardPayload(line); ok {
			EmitCard(ctx, card)
			continue
		}
		buf.Append(line + "\n")
	}
}

// extractCardPayload checks if a line matches __CARD__{json}__END__ and returns
// the parsed JSON map. Returns nil, false if the line is not a card.
func extractCardPayload(line string) (map[string]interface{}, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, cardPrefix) || !strings.HasSuffix(line, cardSuffix) {
		return nil, false
	}
	payload := line[len(cardPrefix) : len(line)-len(cardSuffix)]
	var card map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &card); err != nil {
		return nil, false
	}
	return card, true
}

// trySkillShortCircuit attempts to execute a `blue <skill>` or `<skill>` command
// by calling the skill executor directly, avoiding subprocess + IPC overhead.
// Returns (result, true) on success, (nil, false) if the command doesn't match
// a skill or the skill executor fails (fall through to normal exec).
func (t *ExecTool) trySkillShortCircuit(ctx context.Context, command string, warnings []string, workdirHint string) (interface{}, bool) {
	// Parse: "blue <skillName> key=value key2=value2 ..." or "<skillName> key=value ..."
	trimmed := strings.TrimSpace(command)

	var rest string
	var isBluePrefix bool
	if strings.HasPrefix(trimmed, "blue ") {
		rest = strings.TrimPrefix(trimmed, "blue ")
		isBluePrefix = true
	} else {
		spaceIdx := strings.IndexAny(trimmed, " \t")
		if spaceIdx <= 0 {
			slog.Info("[exec] skill short-circuit skipped", "reason", "no_skill_token", "command", truncateStr(command, 200))
			return nil, false
		}
		rest = trimmed
	}

	if rest == "" {
		slog.Info("[exec] skill short-circuit skipped", "reason", "empty_rest", "command", truncateStr(command, 200))
		return nil, false
	}

	// Extract skill name (first word) and args
	parts := strings.SplitN(rest, " ", 2)
	skillName := parts[0]
	if skillName == "" || skillName == "help" || skillName == "version" {
		slog.Info("[exec] skill short-circuit skipped", "reason", "invalid_skill_name", "skill", skillName, "command", truncateStr(command, 200))
		return nil, false
	}

	// Parse key=value pairs from the rest
	input := make(map[string]any)
	if len(parts) > 1 {
		// For action-oriented skills, support positional action syntax:
		// "blue reminder add message=... time=..."
		restArgs := strings.TrimSpace(parts[1])
		if supportsPositionalAction(skillName) {
			if action, remaining := consumeLeadingBareToken(restArgs); action != "" {
				if _, hasAction := input["action"]; !hasAction {
					input["action"] = action
				}
				restArgs = remaining
			}
		}
		if strings.Contains(restArgs, "=") {
			parseKeyValuePairs(restArgs, input)
		} else if strings.TrimSpace(restArgs) != "" {
			if freeTextInput, err := buildPinnedSkillFreeTextInput(skillName, restArgs); err == nil {
				for key, value := range freeTextInput {
					input[key] = value
				}
			}
		}
	}
	if trimmedWorkdir := strings.TrimSpace(workdirHint); trimmedWorkdir != "" {
		input["__blue_workdir"] = trimmedWorkdir
	}

	// Support dotted skill aliases (e.g. "reminder.add ...") by mapping to
	// skill "reminder" with implicit action=add, preserving explicit action.
	execSkillName := skillName
	if dot := strings.Index(skillName, "."); dot > 0 && dot < len(skillName)-1 {
		base := strings.TrimSpace(skillName[:dot])
		action := strings.TrimSpace(skillName[dot+1:])
		if base != "" && action != "" && isValidSkillName(base) {
			execSkillName = base
			if _, hasAction := input["action"]; !hasAction {
				input["action"] = action
			}
			slog.Info("[exec] dotted skill alias mapped",
				"raw_skill", skillName,
				"skill", execSkillName,
				"action", action)
		}
	}
	inferImplicitSkillAction(execSkillName, input)

	// Try to execute the skill
	slog.Info("[exec] skill short-circuit", "skill", execSkillName, "raw_skill", skillName, "input", input, "source", "blue_prefix")
	data, err := t.skillExec(ctx, execSkillName, input)

	// If skill not found or disabled:
	// - Try progressive selector fallback first.
	// - For direct calls (non-blue): fall through to "blue <command>" if selector is unavailable.
	// - For blue prefix calls: fall through to normal exec when no fallback is available.
	if err != nil {
		if strings.Contains(err.Error(), "unknown skill") || strings.Contains(err.Error(), "is disabled") {
			if toolResp, toolOK := t.tryToolCompatFallback(ctx, execSkillName, input); toolOK {
				return toolResp, true
			}
			if t.skillSelect != nil {
				decision := t.skillSelect(ctx, strings.TrimSpace(rest))
				if decision.SelectedSkill != "" {
					// For high-risk operations we force clarification unless auto-confirm is enabled
					// and the selected skill is in the safe auto-run list.
					forceClarify := decision.NeedClarify
					if isDestructiveSkill(decision.SelectedSkill) {
						autoOK := t.autoConfirm != nil && t.autoConfirm()
						if !autoOK || !isSafeSkillAutoRun(decision.SelectedSkill) {
							forceClarify = true
						}
					}

					if forceClarify {
						if askResp, askOK := t.askForSkillClarification(ctx, skillName, decision, input, warnings); askOK {
							return askResp, true
						}
					} else {
						slog.Info("[exec] selector fallback chose skill", "original", skillName, "selected", decision.SelectedSkill, "confidence", decision.Confidence)
						selectedInput := adaptClarifiedSkillInput(skillName, decision.SelectedSkill, input)
						selectedData, selErr := t.skillExec(ctx, decision.SelectedSkill, selectedInput)
						if selErr == nil {
							return t.buildSkillResult(ctx, decision.SelectedSkill, selectedData, warnings), true
						}
						slog.Warn("[exec] selector fallback execution failed", "skill", decision.SelectedSkill, "err", selErr)
					}
				} else if askResp, askOK := t.askForSkillClarification(ctx, skillName, decision, input, warnings); askOK {
					return askResp, true
				}
			}
			if !isBluePrefix {
				// For direct calls, try to execute as "blue <original command>"
				slog.Info("[exec] skill not found, trying blue prefix", "skill", skillName)
				return nil, false // fall through to exec "blue <command>"
			}
			return nil, false
		}
		// Skill found but execution failed — return error as exec result
		// instead of falling through to subprocess (which would fail the same way).
		slog.Warn("[exec] skill short-circuit failed", "skill", skillName, "err", err)
		emitSkillErrorCard(ctx, execSkillName, err)
		exitCode := 1
		result := execResult{
			SessionID: NewSessionID(),
			Status:    "failed",
			ExitCode:  &exitCode,
			Stderr:    err.Error(),
			Warnings:  warnings,
			Host:      "local",
		}
		b, _ := json.Marshal(result)
		return string(b), true
	}

	return t.buildSkillResult(ctx, execSkillName, data, warnings), true
}

func (t *ExecTool) tryCompatAskCarrier(ctx context.Context, command string) (interface{}, bool, error) {
	input, ok := extractCompatAskInput(command)
	if !ok {
		return nil, false, nil
	}

	slog.Info("[exec] compat ask carrier detected", "command", truncateStr(command, 200))

	if t.skillExec != nil {
		data, err := t.skillExec(ctx, "ask", input)
		if err != nil {
			return nil, true, err
		}
		return t.buildSkillResult(ctx, "ask", data, nil), true, nil
	}

	if t.registry != nil {
		if tool := t.registry.Get("ask"); tool != nil {
			result, err := tool.Execute(ctx, input)
			if err != nil {
				return nil, true, err
			}
			return &ForwardedResult{ActualTool: "ask", Result: result}, true, nil
		}
	}

	return nil, false, nil
}

func (t *ExecTool) tryToolCompatFallback(ctx context.Context, skillName string, input map[string]any) (interface{}, bool) {
	if t == nil || t.registry == nil {
		return nil, false
	}

	trimmed := strings.TrimSpace(skillName)
	if trimmed == "" {
		return nil, false
	}

	actualTool := trimmed
	if t.registry.Get(trimmed) == nil {
		normalized := normalizeCompatToolName(trimmed)
		if normalized == trimmed || t.registry.Get(normalized) == nil {
			return nil, false
		}
		actualTool = normalized
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}

	result, err := NewExecutor(t.registry).Execute(ctx, trimmed, args)
	if err != nil {
		slog.Warn("[exec] tool compat fallback failed",
			"skill", trimmed,
			"tool", actualTool,
			"err", err)
		return nil, false
	}

	slog.Info("[exec] tool compat fallback succeeded",
		"skill", trimmed,
		"tool", actualTool)

	if forwarded, ok := result.(*ForwardedResult); ok {
		return forwarded, true
	}
	return &ForwardedResult{ActualTool: actualTool, Result: result}, true
}

func (t *ExecTool) buildSkillResult(ctx context.Context, skillName string, data map[string]string, warnings []string) interface{} {
	var stdout strings.Builder
	for k, v := range data {
		if k == "success" || k == "_card" {
			continue
		}
		stdout.WriteString(k)
		stdout.WriteString(": ")
		stdout.WriteString(v)
		stdout.WriteByte('\n')
	}

	emitSkillResultCardFromData(ctx, skillName, data)

	exitCode := 0
	result := execResult{
		SessionID: NewSessionID(),
		Status:    "completed",
		ExitCode:  &exitCode,
		Stdout:    stdout.String(),
		Data:      data,
		Warnings:  warnings,
		Host:      "local",
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON)
}

// emitSkillResultCardFromData converts skill short-circuit output to a typeless
// card and emits it to the streaming channel when possible.
func emitSkillResultCardFromData(ctx context.Context, skillName string, data map[string]string) {
	if len(data) == 0 {
		return
	}

	payload := parseSkillResultPayload(data)
	if len(payload) == 0 {
		return
	}

	// Prefer explicit card hint when provided by the skill output.
	if hint := strings.TrimSpace(data["_card"]); hint != "" {
		if shouldConvertSkillCardHint(hint) {
			// Legacy IPC shape: {"_card":"ui_reviewer","result":"{...json...}"}
			if raw := strings.TrimSpace(data["result"]); raw != "" {
				if card := cardconv.ToCard(hint, raw); card != nil {
					EmitCard(ctx, card)
					return
				}
			}

			if b, err := json.Marshal(payload); err == nil {
				if card := cardconv.ToCard(hint, string(b)); card != nil {
					EmitCard(ctx, card)
					return
				}
			}
		}

		// Fallback: emit hinted card as-is for frontend-native types like "search".
		payload["type"] = hint
		EmitCard(ctx, payload)
		return
	}

	// No hint: infer by skill name (e.g. deep_research/analyze/ui_reviewer).
	if b, err := json.Marshal(payload); err == nil {
		if card := cardconv.ToCard(skillName, string(b)); card != nil {
			EmitCard(ctx, card)
		}
	}
}

func emitSkillErrorCard(ctx context.Context, skillName string, err error) {
	if err == nil || strings.TrimSpace(skillName) == "" {
		return
	}
	b, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
	if marshalErr != nil {
		return
	}
	if card := cardconv.ToCard(skillName, string(b)); card != nil {
		EmitCard(ctx, card)
	}
}

func parseSkillResultPayload(data map[string]string) map[string]interface{} {
	payload := make(map[string]interface{}, len(data))
	for k, v := range data {
		if k == "_card" || k == "success" {
			continue
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			payload[k] = parsed
			continue
		}
		payload[k] = v
	}
	return payload
}

func shouldConvertSkillCardHint(hint string) bool {
	switch strings.TrimSpace(hint) {
	case "ui_reviewer", "deep_research", "deep-research", "analyze":
		return true
	default:
		return false
	}
}

func (t *ExecTool) askForSkillClarification(ctx context.Context, originalSkill string, d SkillSelectionDecision, input map[string]any, warnings []string) (interface{}, bool) {
	if t.skillExec == nil {
		return nil, false
	}
	options := make([]string, 0, 3)
	for _, c := range d.Candidates {
		c = strings.TrimSpace(c)
		if c != "" {
			options = append(options, c)
		}
		if len(options) >= 3 {
			break
		}
	}
	if len(options) == 0 {
		options = []string{"web_query", "browser"}
	}
	// In auto-confirm mode, treat clarification as a deterministic router:
	// directly execute the highest-priority candidate instead of blocking on ask.
	if t.autoConfirm != nil && t.autoConfirm() {
		choices := make([]string, 0, len(options)+1)
		if chosen := strings.TrimSpace(d.SelectedSkill); chosen != "" {
			choices = append(choices, chosen)
		}
		for _, candidate := range options {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			seen := false
			for _, picked := range choices {
				if picked == candidate {
					seen = true
					break
				}
			}
			if !seen {
				choices = append(choices, candidate)
			}
		}
		for _, chosen := range choices {
			selectedInput := adaptClarifiedSkillInput(originalSkill, chosen, input)
			selectedData, selErr := t.skillExec(ctx, chosen, selectedInput)
			if selErr == nil {
				autoWarnings := append(warnings, "skill clarification auto-resolved: "+chosen)
				return t.buildSkillResult(ctx, chosen, selectedData, autoWarnings), true
			}
			slog.Warn("[exec] auto-resolved skill clarification execution failed",
				"skill", chosen,
				"original", originalSkill,
				"err", selErr)
		}
	}
	choicesJSON, _ := json.Marshal(options)
	question := fmt.Sprintf("无法确定 skill `%s`，请选择要执行的技能。", originalSkill)
	if d.SelectedSkill != "" {
		question = fmt.Sprintf("当前建议 skill 为 `%s`（置信度 %.2f），是否确认执行？", d.SelectedSkill, d.Confidence)
	}
	askInput := map[string]any{
		"q": question,
		"a": string(choicesJSON),
	}
	askData, err := t.skillExec(ctx, "ask", askInput)
	if err != nil {
		return nil, false
	}
	clarifyWarning := "skill clarification required"
	if strings.EqualFold(strings.TrimSpace(askData["silent"]), "true") {
		clarifyWarning = "skill clarification auto-answered"
	}
	return t.buildSkillResult(ctx, "ask", askData, append(warnings, clarifyWarning)), true
}

func isDestructiveSkill(skillName string) bool {
	lower := strings.ToLower(strings.TrimSpace(skillName))
	if strings.HasPrefix(lower, "config") || strings.HasPrefix(lower, "admin") {
		return true
	}
	for _, kw := range []string{"delete", "remove", "drop", "reset", "overwrite", "uninstall"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func adaptClarifiedSkillInput(originalSkill, selectedSkill string, input map[string]any) map[string]any {
	orig := strings.ToLower(strings.TrimSpace(originalSkill))
	sel := strings.ToLower(strings.TrimSpace(selectedSkill))
	if orig == "" || sel == "" || len(input) == 0 {
		return input
	}
	adapted := make(map[string]any, len(input)+1)
	for k, v := range input {
		adapted[k] = v
	}

	// Factory compatibility: `web_fetch` commonly carries only `url`.
	// If routed to browser, default to navigate so the call is executable.
	if orig == "web_fetch" && sel == "browser" {
		action := strings.TrimSpace(asCompatString(adapted["action"]))
		if action == "" {
			adapted["action"] = "navigate"
		}
	}

	// If `web_fetch` is routed to web_query, lift url/input into canonical input.
	if orig == "web_fetch" && sel == "web_query" {
		value := strings.TrimSpace(asCompatString(adapted["input"]))
		if value == "" {
			if url := strings.TrimSpace(asCompatString(adapted["url"])); url != "" {
				adapted["input"] = url
			}
		}
	}

	// If `web_fetch` is routed to web_search, lift url/input into query.
	if orig == "web_fetch" && sel == "web_search" {
		query := strings.TrimSpace(asCompatString(adapted["query"]))
		if query == "" {
			if url := strings.TrimSpace(asCompatString(adapted["url"])); url != "" {
				adapted["query"] = url
			} else if text := strings.TrimSpace(asCompatString(adapted["input"])); text != "" {
				adapted["query"] = text
			}
		}
	}
	return adapted
}

func asCompatString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func isSafeSkillAutoRun(skillName string) bool {
	switch strings.ToLower(strings.TrimSpace(skillName)) {
	case "ask", "browser", "web_search", "deep_research", "analyze", "ui_reviewer", "plan_create", "plan_update", "plan_append":
		return true
	default:
		return false
	}
}

var compatAskCarrierHeaderRe = regexp.MustCompile(`^cat\s*>\s*/dev/stdin\s*<<-?\s*`)

func extractCompatAskInput(command string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil, false
	}

	if strings.HasPrefix(trimmed, "cd ") {
		idx := strings.Index(trimmed, "&&")
		if idx < 0 {
			return nil, false
		}
		prefix := strings.TrimSpace(trimmed[:idx])
		if !strings.HasPrefix(prefix, "cd ") {
			return nil, false
		}
		trimmed = strings.TrimSpace(trimmed[idx+2:])
	}

	loc := compatAskCarrierHeaderRe.FindStringIndex(trimmed)
	if loc == nil || loc[0] != 0 {
		return nil, false
	}

	delimiter, remainder, ok := consumeCompatHereDocDelimiter(trimmed[loc[1]:])
	if !ok {
		return nil, false
	}

	start := strings.IndexByte(remainder, '{')
	if start < 0 {
		return nil, false
	}

	payload, payloadLen, ok := extractBalancedJSONObject(remainder[start:])
	if !ok {
		return nil, false
	}

	tail := strings.TrimSpace(remainder[start+payloadLen:])
	if tail != delimiter {
		return nil, false
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(payload), &input); err != nil {
		return nil, false
	}
	if !looksLikeAskCompatInput(input) {
		return nil, false
	}

	return input, true
}

func consumeCompatHereDocDelimiter(s string) (string, string, bool) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return "", "", false
	}

	if s[0] == '\'' || s[0] == '"' {
		quote := s[0]
		end := 1
		for end < len(s) && s[end] != quote {
			end++
		}
		if end >= len(s) {
			return "", "", false
		}
		delimiter := s[1:end]
		if delimiter == "" {
			return "", "", false
		}
		return delimiter, s[end+1:], true
	}

	end := 0
	for end < len(s) {
		switch s[end] {
		case ' ', '\t', '\r', '\n':
			delimiter := s[:end]
			if delimiter == "" {
				return "", "", false
			}
			return delimiter, s[end:], true
		default:
			end++
		}
	}

	if s == "" {
		return "", "", false
	}
	return s, "", true
}

func extractBalancedJSONObject(s string) (string, int, bool) {
	if s == "" || s[0] != '{' {
		return "", 0, false
	}

	depth := 0
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], i + 1, true
			}
			if depth < 0 {
				return "", 0, false
			}
		}
	}

	return "", 0, false
}

func looksLikeAskCompatInput(input map[string]any) bool {
	if input == nil {
		return false
	}

	if hasCompatAskQuestions(input["questions"]) {
		return true
	}

	if strings.TrimSpace(asCompatString(input["mq"])) != "" {
		return true
	}

	if strings.TrimSpace(asCompatString(input["q"])) != "" {
		_, hasAnswers := compatArgValue(input, "a")
		return hasAnswers
	}

	if strings.TrimSpace(asCompatString(input["question"])) != "" {
		if qType := strings.TrimSpace(asCompatString(input["type"])); qType == "radio" || qType == "checkbox" || qType == "text" {
			return true
		}
		_, hasOptions := compatArgValue(input, "options")
		return hasOptions
	}

	return false
}

func hasCompatAskQuestions(raw any) bool {
	switch typed := raw.(type) {
	case []interface{}:
		return len(typed) > 0
	case map[string]interface{}:
		return len(typed) > 0
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed != "" && trimmed != "[]" && trimmed != "{}"
	default:
		return false
	}
}

// parseKeyValuePairs parses "key=value key2=\"value with spaces\"" into a map.
func parseKeyValuePairs(s string, out map[string]any) {
	s = strings.TrimSpace(s)
	for s != "" {
		// Find key
		eqIdx := strings.IndexByte(s, '=')
		if eqIdx < 0 {
			break
		}
		key := strings.TrimSpace(s[:eqIdx])
		s = s[eqIdx+1:]

		// Parse value (may be quoted)
		var val string
		if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
			quote := s[0]
			end := 1
			for end < len(s) {
				if s[end] == '\\' && end+1 < len(s) {
					end += 2
					continue
				}
				if s[end] == quote {
					break
				}
				end++
			}
			if end < len(s) {
				val = s[1:end]
				s = s[end+1:]
			} else {
				val = s[1:]
				s = ""
			}
			if quote == '"' {
				val = strings.ReplaceAll(val, `\"`, `"`)
			} else {
				val = strings.ReplaceAll(val, `\'`, `'`)
			}
		} else {
			// Unquoted: read until next space
			spIdx := strings.IndexByte(s, ' ')
			if spIdx < 0 {
				val = s
				s = ""
			} else {
				val = s[:spIdx]
				s = s[spIdx+1:]
			}
		}
		if key != "" {
			appendParsedKeyValue(out, key, val)
		}
	}
}

func shouldParsePinnedSkillArgs(skillName, restArgs string) bool {
	if strings.Contains(restArgs, "=") {
		return true
	}
	if !supportsPositionalAction(skillName) {
		return false
	}
	action, _ := consumeLeadingBareToken(restArgs)
	return action != ""
}

func buildPinnedSkillFreeTextInput(skillName, restArgs string) (map[string]any, error) {
	arg := strings.TrimSpace(restArgs)
	if arg == "" {
		return nil, fmt.Errorf("%s requires arguments", skillName)
	}

	switch strings.ToLower(strings.TrimSpace(skillName)) {
	case "web_query":
		return map[string]any{"input": arg}, nil
	case "web_search", "deep_research":
		return map[string]any{"query": arg}, nil
	case "analyze":
		return map[string]any{"topic": arg}, nil
	case "browser":
		if !looksLikeURL(arg) {
			return nil, fmt.Errorf("browser free-text calls require a URL or structured args like action=navigate url=...")
		}
		return map[string]any{"action": "navigate", "url": arg}, nil
	case "ui_reviewer":
		if !looksLikeURL(arg) {
			return nil, fmt.Errorf("ui_reviewer free-text calls require a URL or structured args like action=review_url url=...")
		}
		return map[string]any{"url": arg}, nil
	case "ask":
		return nil, fmt.Errorf("ask requires structured args: q=... a='[...]' or questions='[...]'")
	default:
		return nil, fmt.Errorf("%s requires structured arguments. Use key=value pairs or 'blue %s <args>'", skillName, skillName)
	}
}

func looksLikeURL(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func consumeLeadingBareToken(s string) (token string, remaining string) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", ""
	}

	token = trimmed
	remaining = ""
	if idx := strings.IndexAny(trimmed, " \t\r\n"); idx >= 0 {
		token = trimmed[:idx]
		remaining = strings.TrimSpace(trimmed[idx+1:])
	}

	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, "=") || strings.HasPrefix(token, "-") {
		return "", trimmed
	}

	return strings.ToLower(token), remaining
}

func supportsPositionalAction(skillName string) bool {
	switch strings.ToLower(strings.TrimSpace(skillName)) {
	case "reminder", "scheduler", "workflows":
		return true
	default:
		return false
	}
}

func inferImplicitSkillAction(skillName string, input map[string]any) {
	if input == nil {
		return
	}
	if _, hasAction := input["action"]; hasAction {
		return
	}

	switch strings.ToLower(strings.TrimSpace(skillName)) {
	case "reminder":
		if hasNonEmptyInputKey(input, "message", "time", "recurring") {
			input["action"] = "add"
			return
		}
		if hasNonEmptyInputKey(input, "id") {
			input["action"] = "delete"
		}
	}
}

func hasNonEmptyInputKey(input map[string]any, keys ...string) bool {
	for _, key := range keys {
		v, ok := input[key]
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(v)) != "" {
			return true
		}
	}
	return false
}

func toolAllowsEmptyExecForward(tool Tool) bool {
	if tool == nil {
		return false
	}
	params := tool.Definition().Parameters
	if len(params) == 0 {
		return true
	}
	required, err := normalizeStringSlice(params["required"])
	if err != nil {
		return false
	}
	return len(required) == 0
}

func appendParsedKeyValue(out map[string]any, key, value string) {
	if isRepeatedListKey(key) {
		existing, _ := out[key].(string)
		out[key] = appendRepeatedListValue(existing, value)
		return
	}
	out[key] = value
}

func isRepeatedListKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "option", "options", "a":
		return true
	default:
		return false
	}
}

func appendRepeatedListValue(existing, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return existing
	}
	if existing == "" {
		return value
	}

	items := make([]string, 0, 4)
	trimmedExisting := strings.TrimSpace(existing)
	if strings.HasPrefix(trimmedExisting, "[") && strings.HasSuffix(trimmedExisting, "]") {
		if err := json.Unmarshal([]byte(trimmedExisting), &items); err != nil {
			items = []string{existing}
		}
	} else {
		items = []string{existing}
	}
	items = append(items, value)
	b, err := json.Marshal(items)
	if err != nil {
		return existing
	}
	return string(b)
}

// isValidSkillName checks if a string looks like a valid skill name.
// Skill names are lowercase alphanumeric with optional underscores/hyphens.
func isValidSkillName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c == '_' || c == '-' {
			continue
		}
		if c < 'a' || c > 'z' {
			// Also allow digits but not at start
			if c >= '0' && c <= '9' && len(name) > 1 {
				continue
			}
			return false
		}
	}
	return true
}

// runSandbox delegates execution to the SandboxExecutor for filesystem-level
// isolation. The sandbox enforces AllowedPaths/DeniedPaths so commands cannot
// access directories outside the sandbox even via cd or absolute paths.
func (t *ExecTool) runSandbox(ctx context.Context, command, workdir string, envMap map[string]string, timeout time.Duration, risk RiskScore, warnings []string) (interface{}, error) {
	executor, tier, tierWarnings, err := t.selectSandboxExecutor(risk)
	if err != nil {
		return nil, errors.New("exec denied: sandbox mode requested but no sandbox manager configured")
	}
	warnings = append(warnings, tierWarnings...)

	// Ensure the running executable's directory is in PATH so `blue <subcommand>`
	// works inside the sandbox (the sandbox builds its own minimal PATH).
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if envMap == nil {
			envMap = map[string]string{}
		}
		if cur, ok := envMap["PATH"]; ok {
			if !strings.Contains(cur, exeDir) {
				envMap["PATH"] = exeDir + string(filepath.ListSeparator) + cur
			}
		} else {
			envMap["PATH"] = exeDir
		}
	}

	userID := GetUserID(ctx)
	sessionID := NewSessionID()
	t.publishEvent(userID, "exec:started", map[string]interface{}{
		"session_id":   sessionID,
		"command":      truncateStr(command, 200),
		"workdir":      workdir,
		"host":         "sandbox",
		"sandbox_tier": string(tier),
	})

	startedAt := time.Now()
	stdout, stderr, exitCode, err := executor.RunInSandbox(ctx, command, workdir, envMap, timeout)
	if err != nil && tier == SandboxTierStrong && t.sandboxLight != nil && errors.Is(err, sandbox.ErrSandboxNotSupported) {
		warnings = append(warnings, "strong sandbox unavailable at runtime; using light sandbox")
		executor = t.sandboxLight
		tier = SandboxTierLight
		stdout, stderr, exitCode, err = executor.RunInSandbox(ctx, command, workdir, envMap, timeout)
	}
	durationMs := time.Since(startedAt).Milliseconds()

	if err != nil {
		return nil, fmt.Errorf("sandbox execution failed: %w", err)
	}

	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}

	t.publishEvent(userID, "exec:completed", map[string]interface{}{
		"session_id":   sessionID,
		"status":       status,
		"exit_code":    exitCode,
		"duration_ms":  durationMs,
		"host":         "sandbox",
		"sandbox_tier": string(tier),
	})

	result := execResult{
		SessionID:   sessionID,
		Status:      status,
		ExitCode:    &exitCode,
		Stdout:      SanitizeBinaryOutput(stdout),
		Stderr:      SanitizeBinaryOutput(stderr),
		DurationMs:  durationMs,
		Warnings:    warnings,
		Host:        "sandbox",
		SandboxTier: string(tier),
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func (t *ExecTool) selectSandboxExecutor(risk RiskScore) (SandboxExecutor, SandboxTier, []string, error) {
	desired := desiredSandboxTier(risk)
	switch desired {
	case SandboxTierStrong:
		if t.sandboxStrong != nil {
			return t.sandboxStrong, SandboxTierStrong, nil, nil
		}
		if t.sandboxLight != nil {
			return t.sandboxLight, SandboxTierLight, []string{"strong sandbox unavailable; using light sandbox"}, nil
		}
	default:
		if t.sandboxLight != nil {
			return t.sandboxLight, SandboxTierLight, nil, nil
		}
		if t.sandboxStrong != nil {
			return t.sandboxStrong, SandboxTierStrong, []string{"light sandbox unavailable; using strong sandbox"}, nil
		}
	}
	return nil, "", nil, errors.New("no sandbox executor configured")
}

func desiredSandboxTier(risk RiskScore) SandboxTier {
	switch risk.Level {
	case RiskLevelMedium, RiskLevelHigh, RiskLevelCritical:
		return SandboxTierStrong
	default:
		return SandboxTierLight
	}
}

// publishEvent sends an SSE lifecycle event if a broker is configured.
func (t *ExecTool) publishEvent(userID, eventType string, data interface{}) {
	if t.broker == nil {
		return
	}
	if userID == "" {
		userID = "default"
	}
	t.broker.Publish(userID, eventType, data)
}

// recordAudit writes an audit entry if the audit store is configured.
func (t *ExecTool) recordAudit(ctx context.Context, command, workdir string, riskScore int, riskLevel RiskLevel, decision string, exitCode *int, durationMs int64, stdoutLen, stderrLen int, errMsg string) {
	if t.audit == nil {
		return
	}
	_ = t.audit.Record(ExecAuditEntry{
		ID:         NewSessionID(),
		Timestamp:  timeutil.NowTime(),
		UserID:     GetUserID(ctx),
		Command:    command,
		Workdir:    workdir,
		RiskScore:  riskScore,
		RiskLevel:  riskLevel,
		PolicyMode: string(t.policy.Mode),
		Decision:   decision,
		ExitCode:   exitCode,
		Duration:   time.Duration(durationMs) * time.Millisecond,
		StdoutLen:  stdoutLen,
		StderrLen:  stderrLen,
		Error:      errMsg,
	})
}

// validateWorkdir checks that the resolved workdir is within AllowedDirs or
// the persistent directory allowlist. If neither matches and an approval
// manager is configured, it triggers an SSE approval request. On "allow-always"
// the directory is added to the persistent allowlist (without merging
// parent/child entries). On "allow-once" the command proceeds without
// persisting. On deny (or timeout) the command is rejected.
func (t *ExecTool) validateWorkdir(ctx context.Context, workdir string) error {
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("exec denied: cannot resolve workdir %q: %w", workdir, err)
	}
	absWorkdir = filepath.Clean(absWorkdir)
	if err := enforceExecWorkdirGuard(ctx, absWorkdir); err != nil {
		return err
	}
	if len(t.config.AllowedDirs) == 0 {
		return nil
	}

	// 1. Check static AllowedDirs.
	for _, dir := range t.config.AllowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absDir = filepath.Clean(absDir)
		if absWorkdir == absDir || strings.HasPrefix(absWorkdir, absDir+string(filepath.Separator)) {
			return nil
		}
	}

	// 2. Check persistent directory allowlist (SQLite).
	if t.dirStore != nil {
		if entry := t.dirStore.Match(absWorkdir); entry != nil {
			return nil
		}
	}

	// 3. Request approval if manager is available.
	if t.approvals == nil {
		return newToolRuntimeError("exec_directory_not_allowed", fmt.Sprintf("exec denied: workdir %q is outside allowed directories", workdir), nil, map[string]interface{}{
			"kind":      "directory",
			"directory": absWorkdir,
		})
	}

	userID := GetUserID(ctx)
	decision, err := t.approvals.RequestApproval(ctx, ApprovalRequest{
		Type:      "directory",
		Directory: absWorkdir,
		UserID:    userID,
	})
	if err != nil {
		return fmt.Errorf("exec denied: directory approval failed: %w", err)
	}

	switch decision {
	case ApprovalAllowOnce:
		return nil
	case ApprovalAllowAlways:
		// Persist to directory allowlist.
		if t.dirStore != nil {
			_ = t.dirStore.Add(absWorkdir, userID)
		}
		return nil
	default:
		return newToolRuntimeError("exec_directory_denied", fmt.Sprintf("exec denied: user denied access to directory %q", workdir), nil, map[string]interface{}{
			"kind":      "directory",
			"directory": absWorkdir,
		})
	}
}

// localeFromTag converts a BCP-47 language tag (e.g. "en-US", "zh-CN") to a
// POSIX locale string (e.g. "en_US.UTF-8", "zh_CN.UTF-8").
func localeFromTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "en_US.UTF-8"
	}
	// Already looks like a POSIX locale (contains underscore or dot).
	if strings.Contains(tag, "_") || strings.Contains(tag, ".") {
		if !strings.Contains(tag, ".") {
			return tag + ".UTF-8"
		}
		return tag
	}
	// BCP-47: "en-US" → "en_US"
	parts := strings.SplitN(tag, "-", 2)
	if len(parts) == 2 {
		return parts[0] + "_" + strings.ToUpper(parts[1]) + ".UTF-8"
	}
	// Bare language code: "en" → "en_US.UTF-8" (best guess)
	return tag + ".UTF-8"
}

func buildExecEnv(extra map[string]string) []string {
	base := os.Environ()
	// Merge: extra overrides base. Also ensure the running executable's
	// directory is in PATH so `blue <subcommand>` works regardless of install location.
	env := make(map[string]string, len(base)+len(extra))
	for _, kv := range base {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			env[kv[:idx]] = kv[idx+1:]
		}
	}
	for k, v := range extra {
		env[k] = v
	}
	// Prepend executable's directory to PATH.
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if cur, ok := env["PATH"]; ok {
			if !strings.Contains(cur, exeDir) {
				env["PATH"] = exeDir + string(filepath.ListSeparator) + cur
			}
		} else {
			env["PATH"] = exeDir
		}
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

func normalizeBlueCLIExecCommand(command, workdirArg string) (string, string, bool) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return command, workdirArg, false
	}
	if strings.HasPrefix(trimmed, "blue ") {
		return trimmed, workdirArg, true
	}
	if strings.HasPrefix(trimmed, "/") {
		return "blue " + trimmed, workdirArg, true
	}
	if strings.TrimSpace(workdirArg) != "" {
		return command, workdirArg, false
	}

	left, right, ok := strings.Cut(trimmed, "&&")
	if !ok {
		return command, workdirArg, false
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if !strings.HasPrefix(left, "cd ") || right == "" {
		return command, workdirArg, false
	}

	cdTarget := strings.TrimSpace(strings.TrimPrefix(left, "cd "))
	cdTarget = strings.Trim(cdTarget, `"'`)
	if cdTarget == "" {
		return command, workdirArg, false
	}

	switch {
	case strings.HasPrefix(right, "blue "):
		return right, cdTarget, true
	case strings.HasPrefix(right, "/"):
		return "blue " + right, cdTarget, true
	default:
		return command, workdirArg, false
	}
}

func rewriteBlueCLIExecutable(command string) string {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "blue ") {
		return command
	}
	if bluePath, err := exec.LookPath("blue"); err == nil && strings.EqualFold(filepath.Base(bluePath), "blue") {
		return command
	}
	exePath, err := os.Executable()
	if err != nil {
		return command
	}
	if strings.EqualFold(filepath.Base(exePath), "blue") {
		return command
	}
	return strconv.Quote(exePath) + strings.TrimPrefix(trimmed, "blue")
}

func canStrictShellBlueSkillShortCircuit(command string) bool {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "blue ") {
		return false
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return false
	}
	for _, token := range []string{"&&", "||", ";", "|", "`", "$(", ">", "<"} {
		if strings.Contains(trimmed, token) {
			return false
		}
	}
	return true
}

func parseEnvArg(v interface{}) map[string]string {
	if v == nil {
		return nil
	}
	switch m := v.(type) {
	case map[string]interface{}:
		result := make(map[string]string, len(m))
		for k, val := range m {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
		return result
	case map[string]string:
		return m
	}
	return nil
}

func parseFloatArg(v interface{}, defaultVal float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err == nil {
			return f
		}
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f
		}
	}
	return defaultVal
}

// absPathRe matches absolute Unix paths (e.g. /etc/passwd, /home/user/.ssh).
// It avoids matching common flags like --option=/value by requiring the path
// to start at a word boundary or after whitespace/quotes.
var absPathRe = regexp.MustCompile(`(?:^|[\s"'=])(/(?:[a-zA-Z0-9._~-]+/)*[a-zA-Z0-9._~-]+)`)
var redirectionPrefixRe = regexp.MustCompile(`^(?:\d*>>?|\d*<<?|&>>?|&>)`)

// urlLikePathPrefixes are path prefixes that look like URL routes, not filesystem paths.
// Paths starting with these are skipped by extractAbsolutePaths.
var urlLikePathPrefixes = []string{
	"/api/", "/v1/", "/v2/", "/v3/",
	"/http", "/https",
	"/graphql", "/webhook", "/ws/",
}

// extractAbsolutePaths returns all unique absolute paths referenced in a shell
// command string. These are checked against the directory allowlist — the
// prefix-based matching in isDirAllowed handles both file and directory paths.
func extractAbsolutePaths(command string) []string {
	seen := make(map[string]struct{})
	var paths []string

	matches := absPathRe.FindAllStringSubmatch(command, -1)
	for _, m := range matches {
		p := filepath.Clean(m[1])
		// Skip common system device paths.
		if p == "/dev/null" || p == "/dev/stdin" || p == "/dev/stdout" || p == "/dev/stderr" {
			continue
		}
		if p == "/" {
			continue
		}
		// Skip URL-like paths (e.g. /api/new-game, /v1/chat/completions).
		if isURLLikePath(m[1]) {
			continue
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	return paths
}

// extractCommandPaths returns all unique absolute filesystem paths referenced in
// a shell command, including relative paths resolved against the supplied
// workdir. It also tracks simple `cd <dir>` chains so later segments resolve
// relative paths against the updated cwd.
func extractCommandPaths(command, workdir string) []string {
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		if cwd, err := os.Getwd(); err == nil {
			workdir = cwd
		}
	}
	if workdir == "" {
		workdir = "."
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		absWorkdir = filepath.Clean(workdir)
	}

	chains := splitChainOperators(command)
	if len(chains) == 0 {
		chains = []string{command}
	}

	seen := make(map[string]struct{})
	paths := make([]string, 0, 8)
	currentCwd := absWorkdir
	for _, chain := range chains {
		parts := splitPipelineForPathExtraction(chain)
		if len(parts) == 0 {
			parts = []string{chain}
		}
		if len(parts) == 1 {
			currentCwd = collectPathsFromCommandPart(parts[0], currentCwd, seen, &paths, true)
			continue
		}
		for _, part := range parts {
			_ = collectPathsFromCommandPart(part, currentCwd, seen, &paths, false)
		}
	}
	return paths
}

func splitPipelineForPathExtraction(command string) []string {
	var parts []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false

	flush := func() {
		s := strings.TrimSpace(buf.String())
		buf.Reset()
		if s != "" {
			parts = append(parts, s)
		}
	}

	for _, ch := range command {
		if escaped {
			buf.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			buf.WriteRune(ch)
			continue
		}
		if inSingle {
			buf.WriteRune(ch)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			buf.WriteRune(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			buf.WriteRune(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			buf.WriteRune(ch)
			continue
		}
		if ch == '|' {
			flush()
			continue
		}
		buf.WriteRune(ch)
	}

	if escaped || inSingle || inDouble {
		return nil
	}
	flush()
	return parts
}

func collectPathsFromCommandPart(part, cwd string, seen map[string]struct{}, paths *[]string, persistCwd bool) string {
	tokens := tokenizeShell(part)
	if len(tokens) == 0 {
		return cwd
	}
	nextCwd := cwd
	commandName := filepath.Base(tokens[0])

	for i, token := range tokens {
		resolved, updatesCwd := resolveCommandPathToken(commandName, i, token, cwd)
		if resolved == "" {
			continue
		}
		if _, ok := seen[resolved]; !ok {
			seen[resolved] = struct{}{}
			*paths = append(*paths, resolved)
		}
		if updatesCwd {
			nextCwd = resolved
		}
	}

	if persistCwd {
		return nextCwd
	}
	return cwd
}

func resolveCommandPathToken(commandName string, index int, token string, cwd string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	if isBlueSlashCommandToken(commandName, index, token) {
		return "", false
	}
	if strings.EqualFold(commandName, "cd") && index == 1 {
		return normalizeCommandPathToken(token, cwd, true), true
	}
	return normalizeCommandPathToken(token, cwd, false), false
}

func isBlueSlashCommandToken(commandName string, index int, token string) bool {
	if index != 1 || !strings.EqualFold(strings.TrimSpace(commandName), "blue") {
		return false
	}
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "/") || len(token) <= 1 {
		return false
	}
	return !strings.Contains(token[1:], "/") && !strings.Contains(token, `\`)
}

func normalizeCommandPathToken(token string, cwd string, allowBare bool) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	token = redirectionPrefixRe.ReplaceAllString(token, "")
	token = strings.TrimSpace(token)
	if token == "" || token == "|" || token == ">" || token == "<" || token == ">>" || token == "<<" {
		return ""
	}
	if idx := strings.IndexByte(token, '='); idx >= 0 && idx < len(token)-1 {
		if normalized := normalizeCommandPathToken(token[idx+1:], cwd, allowBare); normalized != "" {
			return normalized
		}
	}
	if strings.Contains(token, "://") || strings.HasPrefix(token, "$") {
		return ""
	}
	if filepath.IsAbs(token) {
		return cleanCommandPath(token)
	}
	if strings.HasPrefix(token, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			return cleanCommandPath(filepath.Join(home, token[2:]))
		}
		return ""
	}
	if !looksLikeRelativeCommandPath(token, allowBare) {
		return ""
	}
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	return cleanCommandPath(filepath.Join(cwd, token))
}

func looksLikeRelativeCommandPath(token string, allowBare bool) bool {
	if token == "." || token == ".." {
		return true
	}
	if strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") {
		return true
	}
	if strings.Contains(token, "/") || strings.Contains(token, `\`) {
		return true
	}
	if allowBare && !strings.HasPrefix(token, "-") {
		return true
	}
	return false
}

func cleanCommandPath(raw string) string {
	raw = filepath.Clean(raw)
	if raw == "/" || raw == "." || raw == "" {
		return ""
	}
	if raw == "/dev/null" || raw == "/dev/stdin" || raw == "/dev/stdout" || raw == "/dev/stderr" {
		return ""
	}
	if isURLLikePath(raw) {
		return ""
	}
	return raw
}

// isURLLikePath returns true if the path looks like a URL route rather than
// a filesystem path (e.g. /api/new-game, /v1/chat/completions).
func isURLLikePath(path string) bool {
	lower := strings.ToLower(path)
	for _, prefix := range urlLikePathPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// validateCommandPaths extracts filesystem paths from the command and checks
// each against the allowed directories and persistent dir allowlist. Paths
// outside the allowlist trigger an approval request (same as validateWorkdir).
func (t *ExecTool) validateCommandPaths(ctx context.Context, command, workdir string, warnings *[]string) error {
	dirs := extractCommandPaths(command, workdir)
	for _, dir := range dirs {
		if err := enforceExecPathGuard(ctx, dir); err != nil {
			return err
		}
	}
	if len(t.config.AllowedDirs) == 0 {
		return nil // unrestricted
	}

	for _, dir := range dirs {
		if t.isDirAllowed(dir) {
			continue
		}

		// Check persistent dir allowlist.
		if t.dirStore != nil {
			if entry := t.dirStore.Match(dir); entry != nil {
				continue
			}
		}

		// Request approval.
		if t.approvals == nil {
			return newToolRuntimeError("exec_path_not_allowed", fmt.Sprintf("exec denied: command references path %q which is outside allowed directories", dir), nil, map[string]interface{}{
				"kind":      "directory",
				"directory": dir,
				"command":   strings.TrimSpace(command),
				"workdir":   strings.TrimSpace(workdir),
			})
		}

		userID := GetUserID(ctx)
		decision, err := t.approvals.RequestApproval(ctx, ApprovalRequest{
			Type:      "directory",
			Directory: dir,
			Command:   command,
			UserID:    userID,
		})
		if err != nil {
			return fmt.Errorf("exec denied: directory approval for %q failed: %w", dir, err)
		}

		switch decision {
		case ApprovalAllowOnce:
			continue
		case ApprovalAllowAlways:
			if t.dirStore != nil {
				_ = t.dirStore.Add(dir, userID)
			}
			continue
		default:
			return newToolRuntimeError("exec_directory_denied", fmt.Sprintf("exec denied: user denied access to directory %q", dir), nil, map[string]interface{}{
				"kind":      "directory",
				"directory": dir,
				"command":   strings.TrimSpace(command),
				"workdir":   strings.TrimSpace(workdir),
			})
		}
	}
	return nil
}

// isDirAllowed checks if a directory is within the static AllowedDirs list.
func (t *ExecTool) isDirAllowed(dir string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absDir = filepath.Clean(absDir)

	for _, allowed := range t.config.AllowedDirs {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		absAllowed = filepath.Clean(absAllowed)
		if absDir == absAllowed || strings.HasPrefix(absDir, absAllowed+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
