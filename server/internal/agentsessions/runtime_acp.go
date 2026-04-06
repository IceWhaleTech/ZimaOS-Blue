package agentsessions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type ACPRuntime struct {
	authority   ClientAuthority
	credentials CredentialResolver
	logger      *zap.Logger

	mu       sync.Mutex
	sessions map[string]*acpRuntimeSession
}

type acpRuntimeSession struct {
	profileID        string
	externalSession  string
	process          *acpProcess
	remoteSessionID  string
	loadSession      bool
	currentRunID     string
	currentRunStream *runtimeStreamImpl
}

type acpProcess struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	logger    *zap.Logger
	authority ClientAuthority
	cleanup   func() error

	pendingMu sync.Mutex
	pending   map[string]chan acpCallResult

	onUpdate func(map[string]interface{})
	onFatal  func(error)

	closed     chan struct{}
	nextID     uint64
	closeMux   sync.Once
	cleanupMux sync.Once
}

type acpCallResult struct {
	result map[string]interface{}
	err    error
}

func NewACPRuntime(authority ClientAuthority, logger *zap.Logger, credentials CredentialResolver) *ACPRuntime {
	return &ACPRuntime{
		authority:   authority,
		credentials: credentials,
		logger:      logger,
		sessions:    make(map[string]*acpRuntimeSession),
	}
}

func (r *ACPRuntime) VerifyProfile(ctx context.Context, profile AgentProfile) (*ProfileVerifyResult, error) {
	proc, info, err := r.spawnACPProcess(ctx, profile)
	if err != nil {
		return nil, err
	}
	defer proc.close()
	capabilities := []string{"initialize", "session/new", "session/load", "session/prompt", "session/cancel"}
	if info.loadSession {
		capabilities = append(capabilities, "loadSession")
	}
	if len(info.authMethods) > 0 {
		capabilities = append(capabilities, "authenticate")
	}
	return &ProfileVerifyResult{
		OK:           true,
		MessageCode:  ProfileVerifyMessageCodeProfileVerified,
		Message:      "ACP profile verified",
		Capabilities: capabilities,
		Details: map[string]interface{}{
			"protocol_version": info.protocolVersion,
			"agent_info":       info.agentInfo,
			"auth_methods":     info.authMethods,
		},
	}, nil
}

func (r *ACPRuntime) EnsureSession(ctx context.Context, req EnsureSessionRequest) (*EnsureSessionResult, error) {
	state, info, err := r.ensureRuntimeSession(ctx, req.Profile, req.Session)
	if err != nil {
		return nil, err
	}

	cwd := resolveSessionCWD(req.Profile, req.Session)
	if req.Session.RemoteSessionID != "" && info.loadSession {
		if _, err := state.process.call(ctx, "session/load", map[string]interface{}{
			"sessionId":  req.Session.RemoteSessionID,
			"cwd":        cwd,
			"mcpServers": []interface{}{},
		}); err == nil {
			state.remoteSessionID = req.Session.RemoteSessionID
			return &EnsureSessionResult{RemoteSessionID: state.remoteSessionID}, nil
		}
	}

	resp, err := state.process.call(ctx, "session/new", map[string]interface{}{
		"cwd":        cwd,
		"mcpServers": []interface{}{},
	})
	if err != nil {
		return nil, err
	}
	sessionID := stringField(resp, "sessionId")
	if sessionID == "" {
		return nil, fmt.Errorf("ACP session/new did not return sessionId")
	}
	state.remoteSessionID = sessionID
	return &EnsureSessionResult{
		RemoteSessionID: sessionID,
		Metadata: map[string]interface{}{
			"load_session": info.loadSession,
		},
	}, nil
}

func (r *ACPRuntime) SubmitRun(ctx context.Context, req SubmitRunRequest) (*SubmitRunResult, error) {
	state, _, err := r.ensureRuntimeSession(ctx, req.Profile, req.Session)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(state.remoteSessionID) == "" {
		return nil, fmt.Errorf("ACP session is not initialized")
	}
	stream := newRuntimeStream(64)

	r.mu.Lock()
	state.currentRunID = req.Run.ID
	state.currentRunStream = stream
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			if current := r.sessions[req.Session.ID]; current == state && current.currentRunID == req.Run.ID {
				current.currentRunID = ""
				current.currentRunStream = nil
			}
			r.mu.Unlock()
		}()

		resp, err := state.process.call(ctx, "session/prompt", map[string]interface{}{
			"sessionId": state.remoteSessionID,
			"prompt": []map[string]interface{}{
				{
					"type": "text",
					"text": req.Prompt,
				},
			},
		})
		if err != nil {
			stream.push(RuntimeEvent{Type: "error", Error: err.Error()})
			stream.finish(err)
			return
		}
		stopReason := stringField(resp, "stopReason")
		if stopReason != "" {
			stream.push(RuntimeEvent{
				Type:   "done",
				Status: stopReason,
				Payload: map[string]interface{}{
					"stop_reason": stopReason,
				},
			})
		}
		stream.finish(nil)
	}()

	return &SubmitRunResult{RemoteRunID: req.Run.ID}, nil
}

func (r *ACPRuntime) StreamRun(_ context.Context, req StreamRunRequest) (RuntimeStream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.sessions[req.Session.ID]
	if state == nil || state.currentRunID != req.Run.ID || state.currentRunStream == nil {
		return nil, fmt.Errorf("ACP run stream is not available")
	}
	return state.currentRunStream, nil
}

func (r *ACPRuntime) CancelRun(ctx context.Context, session ExternalSession, _ ExternalRun) error {
	r.mu.Lock()
	state := r.sessions[session.ID]
	r.mu.Unlock()
	if state == nil || state.remoteSessionID == "" {
		return nil
	}
	return state.process.notify("session/cancel", map[string]interface{}{
		"sessionId": state.remoteSessionID,
	})
}

func (r *ACPRuntime) CloseSession(_ context.Context, _ AgentProfile, session ExternalSession) error {
	r.mu.Lock()
	state := r.sessions[session.ID]
	delete(r.sessions, session.ID)
	r.mu.Unlock()
	if state == nil {
		return nil
	}
	return state.process.close()
}

func (r *ACPRuntime) Health(ctx context.Context, profile AgentProfile) (*ProfileHealthResult, error) {
	proc, _, err := r.spawnACPProcess(ctx, profile)
	if err != nil {
		return &ProfileHealthResult{Healthy: false, Message: err.Error()}, nil
	}
	defer proc.close()
	return &ProfileHealthResult{
		Healthy:     true,
		MessageCode: ProfileHealthMessageCodeRuntimeHealthy,
		Message:     "ACP runtime is healthy",
	}, nil
}

type acpInitInfo struct {
	protocolVersion int
	loadSession     bool
	authMethods     []map[string]interface{}
	agentInfo       map[string]interface{}
}

func (r *ACPRuntime) ensureRuntimeSession(ctx context.Context, profile AgentProfile, session ExternalSession) (*acpRuntimeSession, *acpInitInfo, error) {
	r.mu.Lock()
	state := r.sessions[session.ID]
	r.mu.Unlock()
	if state != nil && state.process != nil {
		return state, &acpInitInfo{loadSession: state.loadSession}, nil
	}

	proc, info, err := r.spawnACPProcess(ctx, profile)
	if err != nil {
		return nil, nil, err
	}
	state = &acpRuntimeSession{
		profileID:       profile.ID,
		externalSession: session.ID,
		process:         proc,
		loadSession:     info.loadSession,
	}

	proc.onUpdate = func(msg map[string]interface{}) {
		event := parseACPUpdate(msg)
		if event == nil {
			return
		}
		r.mu.Lock()
		current := r.sessions[session.ID]
		runStream := (*runtimeStreamImpl)(nil)
		if current != nil {
			runStream = current.currentRunStream
		}
		r.mu.Unlock()
		if runStream != nil {
			runStream.push(*event)
		}
	}
	proc.onFatal = func(err error) {
		if err == nil {
			return
		}
		r.mu.Lock()
		current := r.sessions[session.ID]
		runStream := (*runtimeStreamImpl)(nil)
		if current != nil {
			runStream = current.currentRunStream
			current.currentRunID = ""
			current.currentRunStream = nil
		}
		delete(r.sessions, session.ID)
		r.mu.Unlock()
		if runStream != nil {
			runStream.push(RuntimeEvent{Type: "error", Error: err.Error()})
			runStream.finish(err)
		}
	}

	r.mu.Lock()
	r.sessions[session.ID] = state
	r.mu.Unlock()
	return state, info, nil
}

func (r *ACPRuntime) spawnACPProcess(ctx context.Context, profile AgentProfile) (*acpProcess, *acpInitInfo, error) {
	command, args, err := resolveACPCommand(profile.Command, exec.LookPath, "")
	if err != nil {
		return nil, nil, err
	}
	if resolvedCommand := resolveACPCommandBinary(command, exec.LookPath, ""); resolvedCommand != command && r.logger != nil {
		r.logger.Debug("resolved ACP runtime command from scenario path",
			zap.String("profile_id", profile.ID),
			zap.String("requested_command", command),
			zap.String("resolved_command", resolvedCommand),
		)
	}
	cwd := resolveProcessCWD(profile)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = cwd

	runtimeEnv := mergeStringMap(nil, profile.Env)
	var cleanup func() error
	if r.credentials != nil {
		materialized, materializeErr := r.credentials.Materialize(ctx, profile)
		if materializeErr != nil {
			if r.logger != nil {
				r.logger.Warn("failed to materialize ACP runtime credentials",
					zap.String("profile_id", profile.ID),
					zap.Error(materializeErr),
				)
			}
		} else if materialized != nil {
			runtimeEnv = mergeStringMap(runtimeEnv, materialized.Env)
			cleanup = materialized.Cleanup
		}
	}
	cmd.Env = buildRuntimeEnv(runtimeEnv)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ACP stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ACP stderr pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ACP stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start ACP runtime: %w", err)
	}

	proc := &acpProcess{
		cmd:       cmd,
		stdin:     stdin,
		logger:    r.logger,
		authority: r.authority,
		cleanup:   cleanup,
		pending:   make(map[string]chan acpCallResult),
		closed:    make(chan struct{}),
	}
	go proc.readStdout(stdout)
	go proc.readStderr(stderr)
	go proc.waitForExit()

	initResp, err := proc.call(ctx, "initialize", map[string]interface{}{
		"protocolVersion": 1,
		"clientCapabilities": map[string]interface{}{
			"fs": map[string]interface{}{
				"readTextFile":  true,
				"writeTextFile": true,
			},
			"terminal": true,
		},
		"clientInfo": map[string]interface{}{
			"name":    "blue-agent-sessions",
			"title":   "Blue Agent Sessions",
			"version": "1.0.0",
		},
	})
	if err != nil {
		_ = proc.close()
		return nil, nil, err
	}
	info := parseACPInitializeResponse(initResp)
	if profile.AuthMethodID != "" && len(info.authMethods) > 0 {
		_, err := proc.call(ctx, "authenticate", map[string]interface{}{
			"methodId": profile.AuthMethodID,
		})
		if err != nil && r.logger != nil {
			r.logger.Debug("ACP authenticate failed", zap.String("profile_id", profile.ID), zap.Error(err))
		}
	}
	return proc, info, nil
}

func (p *acpProcess) waitForExit() {
	err := p.cmd.Wait()
	p.closeMux.Do(func() {
		close(p.closed)
	})
	p.runCleanup()
	if err != nil && p.onFatal != nil {
		p.onFatal(err)
	}
}

func (p *acpProcess) close() error {
	var err error
	p.closeMux.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			err = p.cmd.Process.Kill()
		}
		close(p.closed)
	})
	p.runCleanup()
	return err
}

func (p *acpProcess) runCleanup() {
	p.cleanupMux.Do(func() {
		if p.cleanup == nil {
			return
		}
		if err := p.cleanup(); err != nil && p.logger != nil {
			p.logger.Debug("ACP runtime cleanup failed", zap.Error(err))
		}
	})
}

func (p *acpProcess) readStdout(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			if p.logger != nil {
				p.logger.Debug("ignoring malformed ACP line", zap.String("line", line), zap.Error(err))
			}
			continue
		}
		p.dispatch(payload)
	}
	if err := scanner.Err(); err != nil && p.onFatal != nil {
		p.onFatal(err)
	}
}

func (p *acpProcess) readStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 8*1024), 1<<20)
	for scanner.Scan() {
		if p.logger != nil {
			p.logger.Debug("ACP stderr", zap.String("line", scanner.Text()))
		}
	}
}

func (p *acpProcess) dispatch(payload map[string]interface{}) {
	if method, ok := payload["method"].(string); ok && method != "" {
		if _, hasID := payload["id"]; hasID {
			go p.handleServerRequest(payload)
			return
		}
		if p.onUpdate != nil {
			p.onUpdate(payload)
		}
		return
	}
	id := strings.TrimSpace(fmt.Sprintf("%v", payload["id"]))
	if id == "" {
		return
	}
	result := acpCallResult{result: map[string]interface{}{}}
	if errPayload, ok := payload["error"].(map[string]interface{}); ok {
		result.err = fmt.Errorf("%s", stringField(errPayload, "message"))
	} else if resultPayload, ok := payload["result"].(map[string]interface{}); ok {
		result.result = resultPayload
	} else if payload["result"] == nil {
		result.result = map[string]interface{}{}
	}

	p.pendingMu.Lock()
	ch := p.pending[id]
	delete(p.pending, id)
	p.pendingMu.Unlock()
	if ch != nil {
		ch <- result
		close(ch)
	}
}

func (p *acpProcess) handleServerRequest(payload map[string]interface{}) {
	id := payload["id"]
	method := strings.TrimSpace(stringField(payload, "method"))
	params, _ := payload["params"].(map[string]interface{})

	var (
		result interface{}
		err    error
	)
	switch method {
	case "fs/read_text_file":
		result, err = p.handleReadTextFile(context.Background(), params)
	case "fs/write_text_file":
		result, err = p.handleWriteTextFile(context.Background(), params)
	case "terminal/create":
		result, err = p.handleTerminalCreate(context.Background(), params)
	case "terminal/output":
		result, err = p.handleTerminalOutput(context.Background(), params)
	case "terminal/wait_for_exit":
		result, err = p.handleTerminalWait(context.Background(), params)
	case "terminal/kill":
		result, err = p.handleTerminalKill(context.Background(), params)
	case "terminal/release":
		result, err = p.handleTerminalRelease(context.Background(), params)
	case "session/request_permission":
		result, err = handlePermissionRequest(params)
	default:
		err = fmt.Errorf("unsupported method: %s", method)
	}
	if err != nil {
		_ = p.writeJSON(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": err.Error(),
			},
		})
		return
	}
	_ = p.writeJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func (p *acpProcess) handleReadTextFile(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP file authority is unavailable")
	}
	path := stringField(params, "path")
	line := intField(params, "line")
	limit := intField(params, "limit")
	endLine := 0
	if line > 0 && limit > 0 {
		endLine = line + limit - 1
	}
	result, err := p.authority.ReadTextFile(ctx, path, line, endLine)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"content": stringField(result, "content"),
	}, nil
}

func (p *acpProcess) handleWriteTextFile(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP file authority is unavailable")
	}
	_, err := p.authority.WriteTextFile(ctx, stringField(params, "path"), stringField(params, "content"), false)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (p *acpProcess) handleTerminalCreate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP terminal authority is unavailable")
	}
	command := strings.TrimSpace(stringField(params, "command"))
	if args, ok := params["args"].([]interface{}); ok && len(args) > 0 {
		parts := make([]string, 0, len(args)+1)
		if command != "" {
			parts = append(parts, command)
		}
		for _, arg := range args {
			parts = append(parts, fmt.Sprintf("%v", arg))
		}
		command = strings.Join(parts, " ")
	}
	env := make(map[string]string)
	if raw, ok := params["env"].([]interface{}); ok {
		for _, item := range raw {
			record, _ := item.(map[string]interface{})
			name := stringField(record, "name")
			value := stringField(record, "value")
			if name != "" {
				env[name] = value
			}
		}
	}
	result, err := p.authority.CreateTerminal(ctx, command, stringField(params, "cwd"), env)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"terminalId": stringField(result, "id"),
	}, nil
}

func (p *acpProcess) handleTerminalOutput(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP terminal authority is unavailable")
	}
	result, err := p.authority.TerminalOutput(ctx, stringField(params, "terminalId"))
	if err != nil {
		return nil, err
	}
	resp := map[string]interface{}{
		"output":    stringField(result, "output"),
		"truncated": boolField(result, "truncated"),
	}
	if status, ok := buildTerminalExitStatus(result); ok {
		resp["exitStatus"] = status
	}
	return resp, nil
}

func (p *acpProcess) handleTerminalWait(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP terminal authority is unavailable")
	}
	timeout := time.Duration(intField(params, "timeoutMs")) * time.Millisecond
	result, err := p.authority.TerminalWaitForExit(ctx, stringField(params, "terminalId"), timeout)
	if err != nil {
		return nil, err
	}
	resp := map[string]interface{}{
		"exitCode": nullableInt(result, "exit_code"),
		"signal":   nullableString(result, "signal"),
	}
	return resp, nil
}

func (p *acpProcess) handleTerminalKill(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP terminal authority is unavailable")
	}
	_, err := p.authority.TerminalKill(ctx, stringField(params, "terminalId"))
	return nil, err
}

func (p *acpProcess) handleTerminalRelease(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if p.authority == nil {
		return nil, fmt.Errorf("ACP terminal authority is unavailable")
	}
	_, err := p.authority.TerminalRelease(ctx, stringField(params, "terminalId"))
	return nil, err
}

func (p *acpProcess) call(ctx context.Context, method string, params interface{}) (map[string]interface{}, error) {
	id := strconv.FormatUint(atomic.AddUint64(&p.nextID, 1), 10)
	wait := make(chan acpCallResult, 1)
	p.pendingMu.Lock()
	p.pending[id] = wait
	p.pendingMu.Unlock()

	if err := p.writeJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		return nil, err
	}

	select {
	case result := <-wait:
		return result.result, result.err
	case <-p.closed:
		return nil, fmt.Errorf("ACP runtime closed while waiting for %s", method)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *acpProcess) notify(method string, params interface{}) error {
	return p.writeJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (p *acpProcess) writeJSON(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = p.stdin.Write(data)
	return err
}

func parseACPInitializeResponse(result map[string]interface{}) *acpInitInfo {
	info := &acpInitInfo{
		protocolVersion: intField(result, "protocolVersion"),
		loadSession:     false,
		agentInfo:       map[string]interface{}{},
	}
	if capabilities, ok := result["agentCapabilities"].(map[string]interface{}); ok {
		info.loadSession = boolField(capabilities, "loadSession")
	}
	if agentInfo, ok := result["agentInfo"].(map[string]interface{}); ok {
		info.agentInfo = agentInfo
	}
	if rawMethods, ok := result["authMethods"].([]interface{}); ok {
		for _, item := range rawMethods {
			record, _ := item.(map[string]interface{})
			if len(record) > 0 {
				info.authMethods = append(info.authMethods, record)
			}
		}
	}
	return info
}

func parseACPUpdate(message map[string]interface{}) *RuntimeEvent {
	if strings.TrimSpace(stringField(message, "method")) != "session/update" {
		return nil
	}
	params, _ := message["params"].(map[string]interface{})
	update, _ := params["update"].(map[string]interface{})
	kind := strings.TrimSpace(stringField(update, "sessionUpdate"))
	switch kind {
	case "agent_message_chunk", "user_message_chunk":
		role := "assistant"
		if kind == "user_message_chunk" {
			role = "user"
		}
		text := nestedTextField(update, "content")
		return &RuntimeEvent{
			Type: role + "_delta",
			Role: role,
			Text: text,
			Payload: map[string]interface{}{
				"session_update": kind,
			},
		}
	case "agent_thought_chunk":
		return &RuntimeEvent{
			Type: "thought_delta",
			Role: "assistant",
			Text: nestedTextField(update, "content"),
		}
	case "tool_call", "tool_call_update":
		return &RuntimeEvent{
			Type:   "tool_call",
			Status: stringField(update, "status"),
			Text:   stringField(update, "title"),
			Payload: map[string]interface{}{
				"tool_call_id": stringField(update, "toolCallId"),
				"kind":         stringField(update, "kind"),
			},
		}
	case "plan", "session_info_update", "available_commands_update", "current_mode_update", "config_option_update", "client_operation":
		return &RuntimeEvent{
			Type:   "status",
			Status: kind,
			Text:   stringField(update, "title"),
			Payload: map[string]interface{}{
				"update": update,
			},
		}
	default:
		return nil
	}
}

func handlePermissionRequest(params map[string]interface{}) (map[string]interface{}, error) {
	options, _ := params["options"].([]interface{})
	selected := ""
	for _, item := range options {
		record, _ := item.(map[string]interface{})
		kind := stringField(record, "kind")
		if kind == "allow_once" || kind == "allow_always" || kind == "" {
			selected = stringField(record, "optionId")
			break
		}
	}
	if selected == "" && len(options) > 0 {
		record, _ := options[0].(map[string]interface{})
		selected = stringField(record, "optionId")
	}
	if selected == "" {
		return map[string]interface{}{
			"outcome": map[string]interface{}{
				"outcome": "cancelled",
			},
		}, nil
	}
	return map[string]interface{}{
		"outcome": map[string]interface{}{
			"outcome":  "selected",
			"optionId": selected,
		},
	}, nil
}

func splitCommand(command []string) (string, []string, error) {
	if len(command) == 0 {
		return "", nil, fmt.Errorf("ACP profile command is required")
	}
	return command[0], append([]string(nil), command[1:]...), nil
}

func resolveACPCommand(command []string, lookPath func(string) (string, error), homeDir string) (string, []string, error) {
	binary, args, err := splitCommand(command)
	if err != nil {
		return "", nil, err
	}
	trimmed := strings.TrimSpace(binary)
	if trimmed == "" {
		return "", nil, fmt.Errorf("ACP profile command is required")
	}
	if filepath.Base(trimmed) != trimmed {
		return trimmed, args, nil
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if resolved, err := lookPath(trimmed); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved, args, nil
	}
	if scenarioPath := resolveACPCommandScenarioPath(trimmed, homeDir); scenarioPath != "" {
		return scenarioPath, args, nil
	}
	if fallbackCommand, fallbackArgs, ok := resolveACPCommandFallback(trimmed, args); ok {
		if resolved, err := lookPath(fallbackCommand); err == nil && strings.TrimSpace(resolved) != "" {
			return resolved, fallbackArgs, nil
		}
		return fallbackCommand, fallbackArgs, nil
	}
	return trimmed, args, nil
}

func resolveACPCommandBinary(command string, lookPath func(string) (string, error), homeDir string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	if filepath.Base(trimmed) != trimmed {
		return trimmed
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if resolved, err := lookPath(trimmed); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}
	if scenarioPath := resolveACPCommandScenarioPath(trimmed, homeDir); scenarioPath != "" {
		return scenarioPath
	}
	return trimmed
}

func resolveACPCommandFallback(command string, args []string) (string, []string, bool) {
	switch normalizedCommandBinary([]string{command}) {
	case "claude-agent-acp":
		return "npx", append([]string{"-y", "@zed-industries/claude-agent-acp"}, args...), true
	case "codex-acp":
		return "npx", append([]string{"@zed-industries/codex-acp"}, args...), true
	default:
		return "", nil, false
	}
}

func resolveACPCommandScenarioPath(command, homeDir string) string {
	normalized := normalizedCommandBinary([]string{command})
	if normalized == "" {
		return ""
	}
	if strings.TrimSpace(homeDir) == "" {
		resolvedHome, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		homeDir = resolvedHome
	}
	if strings.TrimSpace(homeDir) == "" {
		return ""
	}
	for _, pattern := range acpCommandScenarioPatterns(normalized, homeDir) {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		if resolved := preferredExecutableMatch(matches); resolved != "" {
			return resolved
		}
	}
	return ""
}

func acpCommandScenarioPatterns(command, homeDir string) []string {
	roots := acpScenarioExtensionRoots(homeDir)
	patterns := make([]string, 0, len(roots)*3)
	for _, root := range roots {
		switch command {
		case "claude", "claude-code":
			patterns = append(patterns,
				filepath.Join(root, "anthropic.claude-code-*", "resources", "native-binary", "claude"),
				filepath.Join(root, "*claude*", "resources", "native-binary", "claude"),
				filepath.Join(root, "*claude*", "bin", "*", "claude"),
			)
		case "codex":
			patterns = append(patterns,
				filepath.Join(root, "openai.chatgpt-*", "bin", "*", "codex"),
				filepath.Join(root, "openai.codex-*", "bin", "*", "codex"),
				filepath.Join(root, "*codex*", "bin", "*", "codex"),
			)
		case "gemini":
			patterns = append(patterns,
				filepath.Join(root, "google.gemini-*", "bin", "*", "gemini"),
				filepath.Join(root, "*gemini*", "bin", "*", "gemini"),
				filepath.Join(root, "*gemini*", "resources", "native-binary", "gemini"),
				filepath.Join(root, "*gemini*", "gemini"),
			)
		}
	}
	return patterns
}

func acpScenarioExtensionRoots(homeDir string) []string {
	return []string{
		filepath.Join(homeDir, ".vscode", "extensions"),
		filepath.Join(homeDir, ".vscode-insiders", "extensions"),
		filepath.Join(homeDir, ".cursor", "extensions"),
		filepath.Join(homeDir, ".cursor-server", "extensions"),
		filepath.Join(homeDir, ".vscode-server", "extensions"),
		filepath.Join(homeDir, ".vscode-server-insiders", "extensions"),
	}
}

func preferredExecutableMatch(matches []string) string {
	if len(matches) == 0 {
		return ""
	}
	sorted := append([]string(nil), matches...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	for _, match := range sorted {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0111 != 0 {
			return match
		}
	}
	return ""
}

func resolveProcessCWD(profile AgentProfile) string {
	if strings.TrimSpace(profile.CWD) != "" {
		return profile.CWD
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func resolveSessionCWD(profile AgentProfile, session ExternalSession) string {
	if strings.TrimSpace(session.CWD) != "" {
		return filepath.Clean(session.CWD)
	}
	return filepath.Clean(resolveProcessCWD(profile))
}

func buildRuntimeEnv(extra map[string]string) []string {
	envMap := make(map[string]string)
	order := make([]string, 0, len(extra)+len(os.Environ()))

	for _, item := range os.Environ() {
		sep := strings.IndexByte(item, '=')
		if sep <= 0 {
			continue
		}
		key := item[:sep]
		value := item[sep+1:]
		if _, ok := envMap[key]; !ok {
			order = append(order, key)
		}
		envMap[key] = value
	}

	for key, value := range extra {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := envMap[key]; !ok {
			order = append(order, key)
		}
		envMap[key] = value
	}

	env := make([]string, 0, len(order))
	for _, key := range order {
		env = append(env, fmt.Sprintf("%s=%s", key, envMap[key]))
	}
	return env
}

func stringField(record map[string]interface{}, key string) string {
	if record == nil {
		return ""
	}
	if value, ok := record[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func nestedTextField(record map[string]interface{}, key string) string {
	if nested, ok := record[key].(map[string]interface{}); ok {
		return stringField(nested, "text")
	}
	return stringField(record, key)
}

func intField(record map[string]interface{}, key string) int {
	if record == nil {
		return 0
	}
	switch value := record[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func boolField(record map[string]interface{}, key string) bool {
	if record == nil {
		return false
	}
	value, _ := record[key].(bool)
	return value
}

func nullableInt(record map[string]interface{}, key string) interface{} {
	if record == nil {
		return nil
	}
	if _, ok := record[key]; !ok {
		return nil
	}
	return intField(record, key)
}

func nullableString(record map[string]interface{}, key string) interface{} {
	if record == nil {
		return nil
	}
	value := stringField(record, key)
	if value == "" {
		return nil
	}
	return value
}

func buildTerminalExitStatus(result map[string]interface{}) (map[string]interface{}, bool) {
	if result == nil {
		return nil, false
	}
	if _, ok := result["exit_code"]; !ok {
		return nil, false
	}
	status := map[string]interface{}{
		"exitCode": intField(result, "exit_code"),
		"signal":   nullableString(result, "signal"),
	}
	return status, true
}
