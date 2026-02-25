package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
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

// DefaultExecConfig returns sensible defaults.
func DefaultExecConfig() ExecConfig {
	return ExecConfig{
		Host:           "local",
		Security:       ExecSecurityFull,
		DefaultTimeout: 30 * time.Second,
		MaxTimeout:     30 * time.Minute,
		MaxOutput:      200_000,
		SafeBins:       DefaultSafeBins,
		AllowPTY:       true,
	}
}

// ExecTool implements the Tool interface for shell command execution.
type ExecTool struct {
	config    ExecConfig
	policy    ExecPolicy
	sessions  *SessionRegistry
	approvals *ApprovalManager      // may be nil
	broker    *sse.Broker           // may be nil; used for lifecycle events
	safeBins  map[string]struct{}
	dirStore  *DirAllowlistStore    // may be nil; persistent directory allowlist
	sandbox   SandboxExecutor       // may be nil; when set, sandbox host mode is available
	toolNames map[string]struct{}   // known tool names; exec rejects commands that match
	retries   *RetryTracker         // prevents same-command retry loops
	audit     *ExecAuditStore       // may be nil; persistent audit log
}

// NewExecTool creates a new exec tool.
func NewExecTool(config ExecConfig, sessions *SessionRegistry, approvals *ApprovalManager, broker *sse.Broker, dirStore *DirAllowlistStore, sbx ...SandboxExecutor) *ExecTool {
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
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
		config:    config,
		policy:    policy,
		sessions:  sessions,
		approvals: approvals,
		broker:    broker,
		safeBins:  BuildSafeBinsSet(config.SafeBins),
		dirStore:  dirStore,
		sandbox:   firstOrNilIface(sbx),
		retries:   NewRetryTracker(policy.MaxRetries, policy.RetryWindow),
	}
}

func firstOrNilIface[T any](s []T) T {
	if len(s) > 0 {
		return s[0]
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
		if n != "exec" && n != "process" { // don't block exec itself
			m[n] = struct{}{}
		}
	}
	t.toolNames = m
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

// HasSandbox returns true if sandbox execution is available.
func (t *ExecTool) HasSandbox() bool {
	return t.sandbox != nil
}

// Policy returns the current exec policy (read-only).
func (t *ExecTool) Policy() ExecPolicy {
	return t.policy
}

// Definition returns the tool definition for the LLM.
func (t *ExecTool) Definition() ToolDefinition {
	desc := "Execute shell commands on the host. Returns stdout, stderr, exit code, and session ID. Use the 'process' tool to manage background sessions."
	if t.sandbox != nil {
		desc += " Sandbox mode is available for isolated execution — set host to 'sandbox' for filesystem-level isolation."
	}

	props := map[string]interface{}{
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
			"description": "Locale for the command execution environment (e.g. en-US, zh-CN, ja-JP). Sets LANG and LC_ALL for the process.",
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
			"description": "Timeout in seconds (default 30, max 1800)",
		},
		"pty": map[string]interface{}{
			"type":        "boolean",
			"description": "Run in a pseudo-terminal (for interactive commands)",
		},
	}

	// Only expose host parameter when sandbox is available.
	if t.sandbox != nil {
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
			"required":   []string{"command"},
		},
	}
}

// execResult is the JSON response returned to the LLM.
type execResult struct {
	SessionID  string   `json:"session_id"`
	Status     string   `json:"status"`
	ExitCode   *int     `json:"exit_code,omitempty"`
	Stdout     string   `json:"stdout,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	Truncated  bool     `json:"truncated,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Host       string   `json:"host,omitempty"`       // "local" or "sandbox"
	RiskLevel  string   `json:"risk_level,omitempty"` // risk assessment level
}

// Execute runs the shell command.
func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	command, _ := args["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("command is required")
	}

	// Reject commands that look like tool invocations.
	// The LLM sometimes tries to call tools via exec (e.g. "web_search query").
	// Return a clear error so the LLM retries with the correct tool.
	if len(t.toolNames) > 0 {
		firstWord := command
		if idx := strings.IndexAny(command, " \t\n"); idx > 0 {
			firstWord = command[:idx]
		}
		if _, isToolName := t.toolNames[firstWord]; isToolName {
			return nil, fmt.Errorf(
				"%s is a tool, not a shell command. Call the %s tool directly instead of using exec",
				firstWord, firstWord,
			)
		}
	}

	workdirArg, _ := args["workdir"].(string)
	langArg, _ := args["lang"].(string)
	envArg := parseEnvArg(args["env"])
	timeoutSec := parseFloatArg(args["timeout"], t.config.DefaultTimeout.Seconds())
	usePTY, _ := args["pty"].(bool)
	hostArg, _ := args["host"].(string)
	hostExplicit := hostArg != "" // user explicitly chose a host
	if hostArg == "" {
		hostArg = t.config.Host
	}

	// Dangerous command blocklist — always enforced regardless of security mode.
	if err := ValidateCommandSafety(command); err != nil {
		t.recordAudit(ctx, command, workdirArg, 100, RiskLevelCritical, "blocked", nil, 0, 0, 0, err.Error())
		return nil, err
	}

	// Command length check.
	if t.policy.MaxCommandLen > 0 && len(command) > t.policy.MaxCommandLen {
		err := fmt.Errorf("exec blocked: command length %d exceeds maximum %d", len(command), t.policy.MaxCommandLen)
		return nil, err
	}

	// Risk scoring — evaluate command risk and enforce policy threshold.
	risk := AnalyzeRisk(command)
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

	// Validate absolute paths referenced in the command against allowed dirs.
	if err := t.validateCommandPaths(ctx, command, &warnings); err != nil {
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

	// Build environment.
	env := buildExecEnv(envArg)

	// Auto-upgrade to sandbox for medium+ risk commands when sandbox is available
	// and the caller didn't explicitly choose a host.
	if !hostExplicit && t.sandbox != nil && risk.Total >= 30 {
		warnings = append(warnings, fmt.Sprintf("auto-sandboxed: risk level %s (score %d)", risk.Level, risk.Total))
		hostArg = "sandbox"
	}

	// Sandbox mode: delegate to sandbox.Manager for filesystem-level isolation.
	if hostArg == "sandbox" {
		return t.runSandbox(ctx, command, workdir, envArg, timeout, warnings)
	}

	// Create session.
	sessionID := NewSessionID()
	session := &ProcessSession{
		ID:        sessionID,
		Command:   command,
		Workdir:   workdir,
		StartedAt: time.Now(),
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
		exitCode, execErr = t.runWithPTY(ctx, session, command, workdir, env, timeout)
	} else {
		exitCode, execErr = t.runDirect(ctx, session, command, workdir, env, timeout)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		readIntoBuffer(stdoutPipe, session.Stdout)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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

// runSandbox delegates execution to the SandboxExecutor for filesystem-level
// isolation. The sandbox enforces AllowedPaths/DeniedPaths so commands cannot
// access directories outside the sandbox even via cd or absolute paths.
func (t *ExecTool) runSandbox(ctx context.Context, command, workdir string, envMap map[string]string, timeout time.Duration, warnings []string) (interface{}, error) {
	if t.sandbox == nil {
		return nil, errors.New("exec denied: sandbox mode requested but no sandbox manager configured")
	}

	userID := GetUserID(ctx)
	sessionID := NewSessionID()
	t.publishEvent(userID, "exec:started", map[string]interface{}{
		"session_id": sessionID,
		"command":    truncateStr(command, 200),
		"workdir":    workdir,
		"host":       "sandbox",
	})

	startedAt := time.Now()
	stdout, stderr, exitCode, err := t.sandbox.RunInSandbox(ctx, command, workdir, envMap, timeout)
	durationMs := time.Since(startedAt).Milliseconds()

	if err != nil {
		return nil, fmt.Errorf("sandbox execution failed: %w", err)
	}

	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}

	t.publishEvent(userID, "exec:completed", map[string]interface{}{
		"session_id":  sessionID,
		"status":      status,
		"exit_code":   exitCode,
		"duration_ms": durationMs,
		"host":        "sandbox",
	})

	result := execResult{
		SessionID:  sessionID,
		Status:     status,
		ExitCode:   &exitCode,
		Stdout:     SanitizeBinaryOutput(stdout),
		Stderr:     SanitizeBinaryOutput(stderr),
		DurationMs: durationMs,
		Warnings:   warnings,
		Host:       "sandbox",
	}
	data, _ := json.Marshal(result)
	return string(data), nil
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
		Timestamp:  time.Now(),
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
	if len(t.config.AllowedDirs) == 0 {
		return nil
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("exec denied: cannot resolve workdir %q: %w", workdir, err)
	}
	absWorkdir = filepath.Clean(absWorkdir)

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
		return fmt.Errorf("exec denied: workdir %q is outside allowed directories", workdir)
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
		return fmt.Errorf("exec denied: user denied access to directory %q", workdir)
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
	if len(extra) == 0 {
		return base
	}
	// Merge: extra overrides base.
	env := make(map[string]string, len(base)+len(extra))
	for _, kv := range base {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			env[kv[:idx]] = kv[idx+1:]
		}
	}
	for k, v := range extra {
		env[k] = v
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
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
	case int:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
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
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	return paths
}

// validateCommandPaths extracts absolute paths from the command and checks
// each against the allowed directories and persistent dir allowlist. Paths
// outside the allowlist trigger an approval request (same as validateWorkdir).
func (t *ExecTool) validateCommandPaths(ctx context.Context, command string, warnings *[]string) error {
	if len(t.config.AllowedDirs) == 0 {
		return nil // unrestricted
	}

	dirs := extractAbsolutePaths(command)
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
			return fmt.Errorf("exec denied: command references path %q which is outside allowed directories", dir)
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
			return fmt.Errorf("exec denied: user denied access to directory %q", dir)
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
