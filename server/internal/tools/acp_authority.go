package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type ACPAuthorityConfig struct {
	AllowedPaths []string
	MaxFileSize  int64
	ExecConfig   ExecConfig
	Approvals    *ApprovalManager
	Broker       interface{}
	DirStore     *DirAllowlistStore
}

type ACPAuthority struct {
	read      *FileReadTool
	write     *FileWriteTool
	exec      *ExecTool
	sessions   *SessionRegistry
	mu        sync.RWMutex
	terminals map[string]*acpTerminalSession
}

type acpTerminalSession struct {
	session *ProcessSession
	cmd     *exec.Cmd
	ptmx    *ptyWrapper
	done    chan error
}

type ptyWrapper struct {
	file *os.File
}

func (p *ptyWrapper) Close() error {
	if p == nil || p.file == nil {
		return nil
	}
	return p.file.Close()
}

func (p *ptyWrapper) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

func (p *ptyWrapper) Write(buf []byte) (int, error) {
	return p.file.Write(buf)
}

func NewACPAuthority(cfg ACPAuthorityConfig) *ACPAuthority {
	sessions := NewSessionRegistry()
	execTool := NewExecTool(cfg.ExecConfig, sessions, cfg.Approvals, nil, cfg.DirStore)
	return &ACPAuthority{
		read:      NewFileReadTool(cfg.AllowedPaths, cfg.MaxFileSize),
		write:     NewFileWriteTool(cfg.AllowedPaths, cfg.MaxFileSize),
		exec:      execTool,
		sessions:  sessions,
		terminals: make(map[string]*acpTerminalSession),
	}
}

func (a *ACPAuthority) SetAuditStore(store *ExecAuditStore) {
	if a == nil || a.exec == nil {
		return
	}
	a.exec.SetAuditStore(store)
}

func (a *ACPAuthority) ReadTextFile(ctx context.Context, path string, startLine, endLine int) (map[string]interface{}, error) {
	if a == nil || a.read == nil {
		return nil, errors.New("ACP authority file read is unavailable")
	}
	args := map[string]interface{}{
		"path": path,
	}
	if startLine > 0 {
		args["start_line"] = startLine
	}
	if endLine > 0 {
		args["end_line"] = endLine
	}
	return parseJSONToolMap(a.read.Execute(ctx, args))
}

func (a *ACPAuthority) WriteTextFile(ctx context.Context, path, content string, appendMode bool) (map[string]interface{}, error) {
	if a == nil || a.write == nil {
		return nil, errors.New("ACP authority file write is unavailable")
	}
	return parseJSONToolMap(a.write.Execute(ctx, map[string]interface{}{
		"path":    path,
		"content": content,
		"append":  appendMode,
	}))
}

func (a *ACPAuthority) CreateTerminal(ctx context.Context, command, workdir string, env map[string]string) (map[string]interface{}, error) {
	if a == nil || a.exec == nil {
		return nil, errors.New("ACP authority terminal support is unavailable")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("command is required")
	}

	safetyMatch := MatchCommandSafety(command)
	if safetyMatch != nil {
		if safetyMatch.RequiresApproval {
			if err := a.exec.requestCommandSafetyApproval(ctx, command, workdir, "local", safetyMatch); err != nil {
				a.exec.recordAudit(ctx, command, workdir, 100, safetyMatch.RiskLevel, "blocked", nil, 0, 0, 0, err.Error())
				return nil, err
			}
		} else {
			err := fmt.Errorf("terminal blocked: %s", safetyMatch.Reason)
			a.exec.recordAudit(ctx, command, workdir, 100, safetyMatch.RiskLevel, "blocked", nil, 0, 0, 0, err.Error())
			return nil, err
		}
	}

	risk := AnalyzeRisk(command)
	if safetyMatch != nil && safetyMatch.RequiresApproval {
		risk = AnalyzeRiskWithSuppressedReasons(command, safetyMatch.SuppressRiskReasons)
	}
	if risk.Total >= a.exec.policy.MaxRiskThreshold {
		err := fmt.Errorf("terminal blocked: risk score %d exceeds policy threshold %d", risk.Total, a.exec.policy.MaxRiskThreshold)
		a.exec.recordAudit(ctx, command, workdir, risk.Total, risk.Level, "blocked", nil, 0, 0, 0, err.Error())
		return nil, err
	}
	if err := a.exec.checkSecurity(ctx, command, workdir, &[]string{}); err != nil {
		return nil, err
	}
	if err := ValidateHostEnv(env); err != nil {
		return nil, err
	}
	resolvedWorkdir, _ := ResolveWorkdir(workdir)
	if workdir == "" && len(a.exec.config.AllowedDirs) > 0 {
		resolvedWorkdir = a.exec.config.AllowedDirs[0]
	}
	if err := a.exec.validateWorkdir(ctx, resolvedWorkdir); err != nil {
		return nil, err
	}
	if err := a.exec.validateCommandPaths(ctx, command, resolvedWorkdir, &[]string{}); err != nil {
		return nil, err
	}

	sessionID := NewSessionID()
	processSession := &ProcessSession{
		ID:        sessionID,
		Command:   command,
		Workdir:   resolvedWorkdir,
		StartedAt: timeutil.NowTime(),
		Stdout:    NewOutputBuffer(a.exec.config.MaxOutput),
		Stderr:    NewOutputBuffer(a.exec.config.MaxOutput),
		Status:    ProcessRunning,
	}
	shell, shellArgs := a.exec.getShellConfig()
	cmd := exec.Command(shell, append(shellArgs, command)...)
	cmd.Dir = resolvedWorkdir
	cmd.Env = buildExecEnv(env)
	cmd.SysProcAttr = newSysProcAttr()

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 120})
	if err != nil {
		return nil, fmt.Errorf("start terminal: %w", err)
	}
	processSession.PID = cmd.Process.Pid
	a.sessions.Add(processSession)

	terminal := &acpTerminalSession{
		session: processSession,
		cmd:     cmd,
		ptmx:    &ptyWrapper{file: ptmx},
		done:    make(chan error, 1),
	}
	a.mu.Lock()
	a.terminals[sessionID] = terminal
	a.mu.Unlock()

	go func() {
		readIntoBuffer(terminal.ptmx, processSession.Stdout)
	}()
	go func() {
		err := cmd.Wait()
		exitCode := 0
		status := ProcessCompleted
		if err != nil {
			status = ProcessFailed
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		a.sessions.MarkExited(sessionID, &exitCode, "", status)
		a.exec.recordAudit(ctx, command, resolvedWorkdir, risk.Total, risk.Level, "allowed", &exitCode, time.Since(processSession.StartedAt).Milliseconds(), processSession.Stdout.Len(), processSession.Stderr.Len(), "")
		terminal.done <- err
		close(terminal.done)
	}()

	return map[string]interface{}{
		"id":         sessionID,
		"pid":        processSession.PID,
		"status":     string(processSession.Status),
		"workdir":    resolvedWorkdir,
		"risk_level": string(risk.Level),
	}, nil
}

func (a *ACPAuthority) TerminalOutput(_ context.Context, terminalID string) (map[string]interface{}, error) {
	terminal, err := a.lookupTerminal(terminalID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":        terminalID,
		"status":    string(terminal.session.Status),
		"output":    SanitizeBinaryOutput(terminal.session.Stdout.Drain()),
		"truncated": terminal.session.Stdout.Truncated(),
	}, nil
}

func (a *ACPAuthority) TerminalWaitForExit(ctx context.Context, terminalID string, timeout time.Duration) (map[string]interface{}, error) {
	terminal, err := a.lookupTerminal(terminalID)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err, ok := <-terminal.done:
		if !ok {
			err = nil
		}
		finished := a.sessions.GetFinished(terminalID)
		result := map[string]interface{}{
			"id": terminalID,
		}
		if finished != nil {
			result["status"] = string(finished.Status)
			result["output"] = SanitizeBinaryOutput(finished.Output)
			result["truncated"] = finished.Truncated
			if finished.ExitCode != nil {
				result["exit_code"] = *finished.ExitCode
			}
		} else {
			result["status"] = string(terminal.session.Status)
		}
		return result, err
	case <-timer.C:
		return map[string]interface{}{
			"id":     terminalID,
			"status": string(terminal.session.Status),
			"done":   false,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *ACPAuthority) TerminalKill(_ context.Context, terminalID string) (map[string]interface{}, error) {
	terminal, err := a.lookupTerminal(terminalID)
	if err != nil {
		return nil, err
	}
	if terminal.session.PID > 0 {
		KillProcessTree(terminal.session.PID)
	}
	code := -1
	a.sessions.MarkExited(terminalID, &code, "SIGKILL", ProcessKilled)
	return map[string]interface{}{
		"id":      terminalID,
		"killed":  true,
		"status":  string(ProcessKilled),
		"pid":     terminal.session.PID,
		"exit_code": code,
	}, nil
}

func (a *ACPAuthority) TerminalRelease(_ context.Context, terminalID string) (map[string]interface{}, error) {
	a.mu.Lock()
	terminal, ok := a.terminals[terminalID]
	if ok {
		delete(a.terminals, terminalID)
	}
	a.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("terminal %q not found", terminalID)
	}
	_ = terminal.ptmx.Close()
	return map[string]interface{}{
		"id":       terminalID,
		"released": true,
	}, nil
}

func (a *ACPAuthority) lookupTerminal(terminalID string) (*acpTerminalSession, error) {
	a.mu.RLock()
	terminal := a.terminals[strings.TrimSpace(terminalID)]
	a.mu.RUnlock()
	if terminal == nil {
		return nil, fmt.Errorf("terminal %q not found", terminalID)
	}
	return terminal, nil
}

func parseJSONToolMap(result interface{}, err error) (map[string]interface{}, error) {
	if err != nil {
		return nil, err
	}
	switch typed := result.(type) {
	case map[string]interface{}:
		return typed, nil
	case string:
		var payload map[string]interface{}
		if unmarshalErr := json.Unmarshal([]byte(typed), &payload); unmarshalErr != nil {
			return map[string]interface{}{"result": typed}, nil
		}
		return payload, nil
	default:
		data, marshalErr := json.Marshal(typed)
		if marshalErr != nil {
			return nil, marshalErr
		}
		var payload map[string]interface{}
		if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
			return map[string]interface{}{"result": string(data)}, nil
		}
		return payload, nil
	}
}
