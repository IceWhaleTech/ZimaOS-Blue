// Package mcp implements a Model Context Protocol (MCP) server.
// It exposes Blue's tools and skills to external agents via JSON-RPC over SSE transport.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const (
	// sessionBufferSize is the per-session outbound message buffer capacity.
	sessionBufferSize = 64

	// defaultToolCallTimeout is the maximum duration for a single tool execution.
	defaultToolCallTimeout = 5 * time.Minute

	// maxRequestBodySize is the maximum allowed request body size (1 MB).
	maxRequestBodySize = 1 << 20

	// maxWorkspaceReadBytes caps single file reads by built-in workspace tools.
	maxWorkspaceReadBytes = 2 << 20 // 2 MiB
	// maxWorkspaceWriteBytes caps single file writes by built-in workspace tools.
	maxWorkspaceWriteBytes = 2 << 20 // 2 MiB
	// maxWorkspaceListEntries caps recursive listings.
	maxWorkspaceListEntries = 5000
	// maxWorkspaceSearchResults caps search hits.
	maxWorkspaceSearchResults = 200
	// defaultWorkspaceListDepth controls recursive listing depth when unspecified.
	defaultWorkspaceListDepth = 6
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "blue-mcp"
	ServerVersion   = "0.10.36"
)

const (
	workspaceListFilesTool   = "workspace.list_files"
	workspaceReadTextTool    = "workspace.read_text"
	workspaceWriteTextTool   = "workspace.write_text"
	workspaceSearchTextTool  = "workspace.search_text"
	workspaceReplaceTextTool = "workspace.replace_text"
	orchestratorRunTool      = "orchestrator.run"
)

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP types
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type capabilities struct {
	Tools     *toolsCap     `json:"tools,omitempty"`
	Resources *resourcesCap `json:"resources,omitempty"`
	Prompts   *promptsCap   `json:"prompts,omitempty"`
}

type toolsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type resourcesCap struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type promptsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

type mcpTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type toolCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCP resource types
type mcpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type resourcesListResult struct {
	Resources []mcpResource `json:"resources"`
}

type resourceReadParams struct {
	URI string `json:"uri"`
}

type resourceReadResult struct {
	Contents []resourceContent `json:"contents"`
}

type resourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// WorkspaceReader provides read access to workspace files for MCP resources.
type WorkspaceReader interface {
	LoadContextFiles() map[string]string
}

// MCP prompt types
type mcpPrompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []promptArgument `json:"arguments,omitempty"`
}

type promptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type promptsListResult struct {
	Prompts []mcpPrompt `json:"prompts"`
}

type promptGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

type promptMessage struct {
	Role    string       `json:"role"`
	Content contentBlock `json:"content"`
}

type promptGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []promptMessage `json:"messages"`
}

type workspaceListEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // file | dir
	Size int64  `json:"size,omitempty"`
}

type workspaceSearchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview"`
}

type orchestratorToolTask struct {
	ID        string                 `json:"id,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	TimeoutMs int                    `json:"timeout_ms,omitempty"`
}

type orchestratorGenerativeTask struct {
	ID        string                 `json:"id,omitempty"`
	Label     string                 `json:"label,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	Prompt    string                 `json:"prompt,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	TimeoutMs int                    `json:"timeout_ms,omitempty"`
	MaxTokens int                    `json:"max_tokens,omitempty"`
}

type orchestratorTransformTask struct {
	Op       string `json:"op"` // summarize | compress
	MaxItems int    `json:"max_items,omitempty"`
	MaxChars int    `json:"max_chars,omitempty"`
}

type orchestratorRunRequest struct {
	Goal            string                       `json:"goal,omitempty"`
	Deterministic   []orchestratorToolTask       `json:"deterministic_tasks,omitempty"`
	Transformative  []orchestratorTransformTask  `json:"transformative_tasks,omitempty"`
	Generative      []orchestratorGenerativeTask `json:"generative_tasks,omitempty"`
	MaxParallel     int                          `json:"max_parallel,omitempty"`
	MaxPerTaskChars int                          `json:"max_per_task_chars,omitempty"`
	MaxResultChars  int                          `json:"max_result_chars,omitempty"`
}

type orchestratorTaskOutput struct {
	TaskID string
	Label  string
	Tool   string
	Type   string
	Text   string
	Err    error
}

// Session represents an MCP client session.
type Session struct {
	ID       string
	Messages chan []byte // outbound messages (server → client)
	done     chan struct{}
	closed   atomic.Bool
}

func newSession() *Session {
	return &Session{
		ID:       uuid.New().String(),
		Messages: make(chan []byte, sessionBufferSize),
		done:     make(chan struct{}),
	}
}

func (s *Session) Close() {
	if s.closed.CompareAndSwap(false, true) {
		close(s.done)
	}
}

// toolsCache holds a pre-marshaled tools/list response keyed by registry version.
type toolsCache struct {
	version uint64
	json    []byte // pre-marshaled JSON of toolsListResult
}

// Server is the MCP protocol server.
type Server struct {
	registry  *tools.Registry
	executor  *tools.Executor
	workspace WorkspaceReader
	rootDir   string
	// Optional LLM subtask runner used by orchestrator generative tasks.
	generativeRunner func(ctx context.Context, prompt string, maxTokens int) (string, error)

	mu       sync.RWMutex
	sessions map[string]*Session
	closed   atomic.Bool

	toolCallTimeout time.Duration // 0 means use defaultToolCallTimeout

	toolsCacheMu sync.RWMutex
	toolsCached  *toolsCache
}

// NewServer creates a new MCP server.
func NewServer(registry *tools.Registry, executor *tools.Executor) *Server {
	root, err := os.Getwd()
	if err != nil || strings.TrimSpace(root) == "" {
		root = "."
	}
	return &Server{
		registry: registry,
		executor: executor,
		rootDir:  root,
		sessions: make(map[string]*Session),
	}
}

// SetWorkspaceRoot sets the root directory used by built-in workspace coding tools.
// Relative paths are resolved against the current process working directory.
func (s *Server) SetWorkspaceRoot(dir string) {
	clean := strings.TrimSpace(dir)
	if clean == "" {
		return
	}
	if !filepath.IsAbs(clean) {
		if abs, err := filepath.Abs(clean); err == nil {
			clean = abs
		}
	}
	s.rootDir = clean
}

// SetGenerativeRunner sets the optional LLM subtask runner used by orchestrator.run.
func (s *Server) SetGenerativeRunner(fn func(ctx context.Context, prompt string, maxTokens int) (string, error)) {
	s.generativeRunner = fn
}

// SetToolCallTimeout overrides the default tool call timeout.
func (s *Server) SetToolCallTimeout(d time.Duration) {
	s.toolCallTimeout = d
}

func (s *Server) getToolCallTimeout() time.Duration {
	if s.toolCallTimeout > 0 {
		return s.toolCallTimeout
	}
	return defaultToolCallTimeout
}

// Close gracefully shuts down the server, closing all active sessions.
func (s *Server) Close() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.mu.Lock()
	for id, sess := range s.sessions {
		sess.Close()
		delete(s.sessions, id)
	}
	s.mu.Unlock()
}

// CreateSession creates a new client session.
func (s *Server) CreateSession() *Session {
	sess := newSession()
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
	return sess
}

// RemoveSession removes a client session.
func (s *Server) RemoveSession(id string) {
	s.mu.Lock()
	if sess, ok := s.sessions[id]; ok {
		sess.Close()
		delete(s.sessions, id)
	}
	s.mu.Unlock()
}

// HandleMessage processes a JSON-RPC message from a client.
func (s *Server) HandleMessage(ctx context.Context, sessionID string, raw []byte) ([]byte, error) {
	var req jsonRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.errorResponse(nil, -32700, "parse error")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID)
	case "initialized":
		// Client acknowledgment — no response needed
		return nil, nil
	case "tools/list":
		return s.handleToolsList(req.ID)
	case "tools/call":
		return s.handleToolsCall(ctx, req.ID, req.Params)
	case "resources/list":
		return s.handleResourcesList(req.ID)
	case "resources/read":
		return s.handleResourcesRead(req.ID, req.Params)
	case "prompts/list":
		return s.handlePromptsList(req.ID)
	case "prompts/get":
		return s.handlePromptsGet(req.ID, req.Params)
	case "ping":
		return s.successResponse(req.ID, map[string]string{})
	default:
		return s.errorResponse(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(id any) ([]byte, error) {
	return s.successResponse(id, initializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: capabilities{
			Tools:     &toolsCap{},
			Resources: &resourcesCap{},
			Prompts:   &promptsCap{},
		},
		ServerInfo: serverInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
	})
}

func (s *Server) handleToolsList(id any) ([]byte, error) {
	cached := s.getCachedToolsList()

	// Build JSON-RPC response by splicing cached result into the envelope.
	// This avoids re-marshaling the (potentially large) tools list.
	idJSON, err := json.Marshal(id)
	if err != nil {
		idJSON = []byte("null")
	}
	buf := make([]byte, 0, len(cached)+len(idJSON)+40)
	buf = append(buf, `{"jsonrpc":"2.0","id":`...)
	buf = append(buf, idJSON...)
	buf = append(buf, `,"result":`...)
	buf = append(buf, cached...)
	buf = append(buf, '}')
	return buf, nil
}

// getCachedToolsList returns the pre-marshaled toolsListResult JSON,
// rebuilding it only when the registry version changes.
func (s *Server) getCachedToolsList() []byte {
	ver := s.registry.Version()

	s.toolsCacheMu.RLock()
	if c := s.toolsCached; c != nil && c.version == ver {
		data := c.json
		s.toolsCacheMu.RUnlock()
		return data
	}
	s.toolsCacheMu.RUnlock()

	// Rebuild
	defs := s.registry.Definitions()
	builtin := builtinWorkspaceTools()
	mcpTools := make([]mcpTool, 0, len(defs)+len(builtin))
	for _, def := range defs {
		schema := def.Parameters
		if schema == nil {
			schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		mcpTools = append(mcpTools, mcpTool{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: schema,
		})
	}
	mcpTools = append(mcpTools, builtin...)
	sort.Slice(mcpTools, func(i, j int) bool {
		return mcpTools[i].Name < mcpTools[j].Name
	})
	data, _ := json.Marshal(toolsListResult{Tools: mcpTools})

	s.toolsCacheMu.Lock()
	s.toolsCached = &toolsCache{version: ver, json: data}
	s.toolsCacheMu.Unlock()

	return data
}

func (s *Server) handleToolsCall(ctx context.Context, id any, params json.RawMessage) ([]byte, error) {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.errorResponse(id, -32602, "invalid params")
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return s.successResponse(id, toolCallResult{
			Content: []contentBlock{{Type: "text", Text: "Error: tool name is required"}},
			IsError: true,
		})
	}
	if p.Arguments == nil {
		p.Arguments = map[string]interface{}{}
	}

	if handled, text, callErr := s.handleBuiltinTool(ctx, p.Name, p.Arguments); handled {
		if callErr != nil {
			return s.successResponse(id, toolCallResult{
				Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", callErr)}},
				IsError: true,
			})
		}
		return s.successResponse(id, toolCallResult{
			Content: []contentBlock{{Type: "text", Text: text}},
		})
	}

	// Explicitly validate tool existence before execution.
	// This prevents executor fallback (unknown tool -> exec) from masking invalid calls.
	if s.registry.Get(p.Name) == nil {
		return s.successResponse(id, toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("Error: tool not found: %s", p.Name)}},
			IsError: true,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, s.getToolCallTimeout())
	defer cancel()

	result, err := s.executor.Execute(ctx, p.Name, p.Arguments)
	if err != nil {
		return s.successResponse(id, toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		})
	}

	text := toolResultToText(result)

	return s.successResponse(id, toolCallResult{
		Content: []contentBlock{{Type: "text", Text: text}},
	})
}

func builtinWorkspaceTools() []mcpTool {
	return []mcpTool{
		{
			Name:        orchestratorRunTool,
			Description: "Run a schedulable MCP tool graph with deterministic, transformative, and generative phases. Returns only compressed final context.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"goal": map[string]interface{}{
						"type":        "string",
						"description": "Overall orchestration goal",
					},
					"deterministic_tasks": map[string]interface{}{
						"type":        "array",
						"description": "Parallel deterministic tasks (SQL/cache/vector/file/tool calls)",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
					"transformative_tasks": map[string]interface{}{
						"type":        "array",
						"description": "Transform tasks (summarize/compress) applied on deterministic aggregate",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
					"generative_tasks": map[string]interface{}{
						"type":        "array",
						"description": "Generative LLM subtasks or generative-capable tool calls",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
					"max_parallel": map[string]interface{}{
						"type":        "integer",
						"description": "Parallel worker limit (1-16, default 4)",
					},
					"max_per_task_chars": map[string]interface{}{
						"type":        "integer",
						"description": "Per task output clamp (default 1200)",
					},
					"max_result_chars": map[string]interface{}{
						"type":        "integer",
						"description": "Final compressed output clamp (default 5000)",
					},
				},
			},
		},
	}
}

func (s *Server) handleBuiltinTool(ctx context.Context, name string, args map[string]interface{}) (bool, string, error) {
	switch name {
	case orchestratorRunTool:
		out, err := s.orchestratorRun(ctx, args)
		return true, out, err
	case workspaceListFilesTool:
		out, err := s.workspaceListFiles(args)
		return true, out, err
	case workspaceReadTextTool:
		out, err := s.workspaceReadText(args)
		return true, out, err
	case workspaceWriteTextTool:
		out, err := s.workspaceWriteText(args)
		return true, out, err
	case workspaceSearchTextTool:
		out, err := s.workspaceSearchText(args)
		return true, out, err
	case workspaceReplaceTextTool:
		out, err := s.workspaceReplaceText(args)
		return true, out, err
	default:
		return false, "", nil
	}
}

func (s *Server) orchestratorRun(ctx context.Context, args map[string]interface{}) (string, error) {
	req, err := decodeOrchestratorRunRequest(args)
	if err != nil {
		return "", err
	}
	maxParallel := clampInt(req.MaxParallel, 1, 16, 4)
	maxPerTaskChars := clampInt(req.MaxPerTaskChars, 200, 8000, 1200)
	maxResultChars := clampInt(req.MaxResultChars, 800, 12000, 5000)

	deterministicOut := s.runDeterministicTasks(ctx, req.Deterministic, maxParallel, maxPerTaskChars)
	deterministicOut, deterministicDedup := dedupeTaskOutputs(deterministicOut)
	deterministicSummary := aggregateTaskOutputs(deterministicOut, maxResultChars*2)
	if len(req.Transformative) > 0 {
		deterministicSummary = applyTransformPipeline(deterministicSummary, req.Transformative, maxResultChars)
	} else {
		deterministicSummary = compressTextBlock(deterministicSummary, maxResultChars)
	}

	generativeOut := s.runGenerativeTasks(ctx, req.Generative, deterministicSummary, maxParallel, maxPerTaskChars)
	generativeOut, generativeDedup := dedupeTaskOutputs(generativeOut)
	generativeSummary := aggregateTaskOutputs(generativeOut, maxResultChars*2)
	generativeSummary = compressTextBlock(generativeSummary, maxResultChars)

	final, stats := buildOrchestratorFinal(req.Goal, deterministicSummary, generativeSummary, deterministicOut, generativeOut)
	stats["dedup_removed_deterministic"] = deterministicDedup
	stats["dedup_removed_generative"] = generativeDedup
	stats["max_parallel"] = maxParallel
	stats["max_per_task_chars"] = maxPerTaskChars
	stats["max_result_chars"] = maxResultChars
	final = truncateRunes(strings.TrimSpace(final), maxResultChars)
	if final == "" {
		return "", errors.New("orchestrator produced empty output")
	}

	payload := map[string]interface{}{
		"compressed_result": final,
		"stats":             stats,
		"failures": map[string]interface{}{
			"deterministic": collectTaskFailures(deterministicOut, 12),
			"generative":    collectTaskFailures(generativeOut, 12),
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeOrchestratorRunRequest(args map[string]interface{}) (orchestratorRunRequest, error) {
	var req orchestratorRunRequest
	if args == nil {
		args = map[string]interface{}{}
	}
	b, err := json.Marshal(args)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return req, fmt.Errorf("invalid orchestrator request: %w", err)
	}
	if len(req.Deterministic) == 0 && len(req.Generative) == 0 {
		return req, errors.New("orchestrator requires at least one deterministic_tasks or generative_tasks item")
	}
	return req, nil
}

func (s *Server) runDeterministicTasks(ctx context.Context, tasks []orchestratorToolTask, maxParallel, maxChars int) []orchestratorTaskOutput {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]orchestratorTaskOutput, len(tasks))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i := range tasks {
		i := i
		task := tasks[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out[i] = orchestratorTaskOutput{
					TaskID: defaultTaskID(task.ID, i+1, "det"),
					Label:  strings.TrimSpace(task.Label),
					Tool:   task.Tool,
					Type:   "deterministic",
					Err:    ctx.Err(),
				}
				return
			}
			text, err := s.executeToolTask(ctx, task.Tool, task.Arguments, task.TimeoutMs, maxChars)
			out[i] = orchestratorTaskOutput{
				TaskID: defaultTaskID(task.ID, i+1, "det"),
				Label:  strings.TrimSpace(task.Label),
				Tool:   strings.TrimSpace(task.Tool),
				Type:   "deterministic",
				Text:   text,
				Err:    err,
			}
		}()
	}
	wg.Wait()
	return out
}

func (s *Server) runGenerativeTasks(ctx context.Context, tasks []orchestratorGenerativeTask, compressedContext string, maxParallel, maxChars int) []orchestratorTaskOutput {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]orchestratorTaskOutput, len(tasks))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i := range tasks {
		i := i
		task := tasks[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out[i] = orchestratorTaskOutput{
					TaskID: defaultTaskID(task.ID, i+1, "gen"),
					Label:  strings.TrimSpace(task.Label),
					Tool:   strings.TrimSpace(task.Tool),
					Type:   "generative",
					Err:    ctx.Err(),
				}
				return
			}

			taskID := defaultTaskID(task.ID, i+1, "gen")
			label := strings.TrimSpace(task.Label)
			toolName := strings.TrimSpace(task.Tool)

			// Tool-backed generative task
			if toolName != "" {
				args := copyArgs(task.Arguments)
				if compressedContext != "" {
					if _, ok := args["context"]; !ok {
						args["context"] = compressedContext
					}
				}
				text, err := s.executeToolTask(ctx, toolName, args, task.TimeoutMs, maxChars)
				out[i] = orchestratorTaskOutput{
					TaskID: taskID,
					Label:  label,
					Tool:   toolName,
					Type:   "generative",
					Text:   text,
					Err:    err,
				}
				return
			}

			// Prompt-backed LLM subtask
			prompt := strings.TrimSpace(task.Prompt)
			if prompt == "" {
				out[i] = orchestratorTaskOutput{
					TaskID: taskID,
					Label:  label,
					Tool:   "",
					Type:   "generative",
					Err:    errors.New("generative task requires either tool or prompt"),
				}
				return
			}
			if s.generativeRunner == nil {
				out[i] = orchestratorTaskOutput{
					TaskID: taskID,
					Label:  label,
					Tool:   "",
					Type:   "generative",
					Err:    errors.New("generative runner is not configured"),
				}
				return
			}
			finalPrompt := renderGenerativePrompt(prompt, compressedContext)
			maxTokens := clampInt(task.MaxTokens, 64, 4096, 512)
			runCtx := ctx
			cancel := func() {}
			if task.TimeoutMs > 0 {
				runCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutMs)*time.Millisecond)
			}
			defer cancel()
			text, err := s.generativeRunner(runCtx, finalPrompt, maxTokens)
			out[i] = orchestratorTaskOutput{
				TaskID: taskID,
				Label:  label,
				Tool:   "",
				Type:   "generative",
				Text:   truncateRunes(strings.TrimSpace(text), maxChars),
				Err:    err,
			}
		}()
	}
	wg.Wait()
	return out
}

func renderGenerativePrompt(prompt, compressedContext string) string {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return ""
	}
	if compressedContext == "" {
		return p
	}
	if strings.Contains(p, "{{context}}") {
		return strings.ReplaceAll(p, "{{context}}", compressedContext)
	}
	var sb strings.Builder
	sb.WriteString(p)
	sb.WriteString("\n\nContext:\n")
	sb.WriteString(compressedContext)
	return sb.String()
}

func (s *Server) executeToolTask(ctx context.Context, toolName string, args map[string]interface{}, timeoutMs, maxChars int) (string, error) {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "", errors.New("tool is required")
	}
	if name == orchestratorRunTool {
		return "", errors.New("orchestrator.run cannot recursively call itself")
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	runCtx := ctx
	cancel := func() {}
	if timeoutMs > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	}
	defer cancel()

	if handled, text, err := s.handleBuiltinTool(runCtx, name, args); handled {
		if err != nil {
			return "", err
		}
		return truncateRunes(strings.TrimSpace(text), maxChars), nil
	}

	if s.registry.Get(name) == nil {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	result, err := s.executor.Execute(runCtx, name, args)
	if err != nil {
		return "", err
	}
	return truncateRunes(strings.TrimSpace(toolResultToText(result)), maxChars), nil
}

func applyTransformPipeline(input string, tasks []orchestratorTransformTask, maxResultChars int) string {
	current := strings.TrimSpace(input)
	if current == "" {
		return ""
	}
	for _, task := range tasks {
		op := strings.ToLower(strings.TrimSpace(task.Op))
		switch op {
		case "summarize":
			maxItems := clampInt(task.MaxItems, 1, 100, 12)
			maxChars := clampInt(task.MaxChars, 200, 12000, maxResultChars)
			current = summarizeTextBlock(current, maxItems, maxChars)
		case "compress", "":
			maxChars := clampInt(task.MaxChars, 200, 12000, maxResultChars)
			current = compressTextBlock(current, maxChars)
		default:
			// Ignore unknown transformative operators for forward compatibility.
		}
	}
	return compressTextBlock(current, maxResultChars)
}

func summarizeTextBlock(input string, maxItems, maxChars int) string {
	lines := strings.Split(input, "\n")
	seen := make(map[string]struct{}, len(lines))
	selected := make([]string, 0, maxItems)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key := normalizeDedupKey(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, "- "+truncateRunes(line, 220))
		if len(selected) >= maxItems {
			break
		}
	}
	if len(selected) == 0 {
		return ""
	}
	return truncateRunes(strings.Join(selected, "\n"), maxChars)
}

func compressTextBlock(input string, maxChars int) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		clean = append(clean, line)
	}
	joined := strings.Join(clean, "\n")
	if joined == "" {
		joined = strings.Join(strings.Fields(input), " ")
	}
	return truncateRunes(joined, maxChars)
}

func dedupeTaskOutputs(outputs []orchestratorTaskOutput) ([]orchestratorTaskOutput, int) {
	if len(outputs) == 0 {
		return outputs, 0
	}
	seen := make(map[string]struct{}, len(outputs))
	out := make([]orchestratorTaskOutput, 0, len(outputs))
	removed := 0
	for _, item := range outputs {
		if item.Err != nil {
			out = append(out, item)
			continue
		}
		key := normalizeDedupKey(item.Text)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			removed++
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out, removed
}

func normalizeDedupKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	return truncateRunes(s, 512)
}

func aggregateTaskOutputs(outputs []orchestratorTaskOutput, maxChars int) string {
	if len(outputs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, item := range outputs {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = item.Tool
		}
		if label == "" {
			label = item.TaskID
		}

		var line string
		if item.Err != nil {
			line = fmt.Sprintf("[%s] ERROR: %s", label, truncateRunes(strings.TrimSpace(item.Err.Error()), 240))
		} else {
			text := strings.TrimSpace(item.Text)
			if text == "" {
				continue
			}
			line = fmt.Sprintf("[%s] %s", label, text)
		}

		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(truncateRunes(line, 320))
		if utf8.RuneCountInString(sb.String()) >= maxChars {
			break
		}
	}
	return truncateRunes(strings.TrimSpace(sb.String()), maxChars)
}

func collectTaskFailures(outputs []orchestratorTaskOutput, maxItems int) []map[string]string {
	if len(outputs) == 0 || maxItems <= 0 {
		return nil
	}
	items := make([]map[string]string, 0, maxItems)
	for _, item := range outputs {
		if item.Err == nil {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = strings.TrimSpace(item.Tool)
		}
		if label == "" {
			label = strings.TrimSpace(item.TaskID)
		}
		out := map[string]string{
			"task_id": strings.TrimSpace(item.TaskID),
			"type":    strings.TrimSpace(item.Type),
			"error":   truncateRunes(strings.TrimSpace(item.Err.Error()), 240),
		}
		if label != "" {
			out["label"] = label
		}
		if tool := strings.TrimSpace(item.Tool); tool != "" {
			out["tool"] = tool
		}
		items = append(items, out)
		if len(items) >= maxItems {
			break
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func buildOrchestratorFinal(goal, deterministicSummary, generativeSummary string, detOut, genOut []orchestratorTaskOutput) (string, map[string]interface{}) {
	var sb strings.Builder
	goal = strings.TrimSpace(goal)
	if goal != "" {
		sb.WriteString("Goal: ")
		sb.WriteString(goal)
		sb.WriteString("\n\n")
	}
	if deterministicSummary != "" {
		sb.WriteString("Deterministic Context:\n")
		sb.WriteString(deterministicSummary)
		sb.WriteString("\n\n")
	}
	if generativeSummary != "" {
		sb.WriteString("Generative Insights:\n")
		sb.WriteString(generativeSummary)
		sb.WriteString("\n\n")
	}

	stats := map[string]interface{}{
		"deterministic_total":   len(detOut),
		"deterministic_success": countTaskSuccess(detOut),
		"generative_total":      len(genOut),
		"generative_success":    countTaskSuccess(genOut),
	}
	sb.WriteString("Orchestrator Notes:\n")
	sb.WriteString(fmt.Sprintf("- deterministic: %d/%d succeeded\n", stats["deterministic_success"], stats["deterministic_total"]))
	sb.WriteString(fmt.Sprintf("- generative: %d/%d succeeded", stats["generative_success"], stats["generative_total"]))
	return strings.TrimSpace(sb.String()), stats
}

func countTaskSuccess(outputs []orchestratorTaskOutput) int {
	n := 0
	for _, out := range outputs {
		if out.Err == nil && strings.TrimSpace(out.Text) != "" {
			n++
		}
	}
	return n
}

func defaultTaskID(taskID string, idx int, prefix string) string {
	id := strings.TrimSpace(taskID)
	if id != "" {
		return id
	}
	return fmt.Sprintf("%s-%d", prefix, idx)
}

func copyArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return map[string]interface{}{}
	}
	cp := make(map[string]interface{}, len(args))
	for k, v := range args {
		cp[k] = v
	}
	return cp
}

func clampInt(v, minV, maxV, def int) int {
	if v == 0 {
		v = def
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func toolResultToText(result interface{}) string {
	switch v := result.(type) {
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func (s *Server) workspaceListFiles(args map[string]interface{}) (string, error) {
	args = normalizeWorkspaceListFilesArgs(args)
	scope := "."
	if path := workspaceCompatPathString(args); path != "" {
		scope = path
	}
	maxDepth := defaultWorkspaceListDepth
	if v, ok := workspaceCompatValue(args, "max_depth", "maxDepth"); ok {
		n, err := asInt(v)
		if err != nil {
			return "", fmt.Errorf("max_depth must be an integer")
		}
		maxDepth = n
	}
	if maxDepth < 0 {
		maxDepth = 0
	}
	if maxDepth > 20 {
		maxDepth = 20
	}
	includeHidden, err := asBoolDefault(args, "include_hidden", false)
	if err != nil {
		return "", err
	}

	baseAbs, baseRel, err := s.resolveWorkspacePath(scope, true)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(baseAbs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", baseRel)
	}

	root := s.workspaceAbsRoot()
	entries := make([]workspaceListEntry, 0, 256)
	truncated := false

	var walk func(absDir string, depth int) error
	walk = func(absDir string, depth int) error {
		children, err := os.ReadDir(absDir)
		if err != nil {
			return err
		}
		for _, child := range children {
			name := child.Name()
			if !includeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			full := filepath.Clean(filepath.Join(absDir, name))
			if !pathWithinRoot(root, full) {
				continue
			}
			rel, err := filepath.Rel(root, full)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			item := workspaceListEntry{Path: rel, Type: "file"}
			if child.IsDir() {
				item.Type = "dir"
			} else if fi, statErr := child.Info(); statErr == nil {
				item.Size = fi.Size()
			}
			entries = append(entries, item)
			if len(entries) >= maxWorkspaceListEntries {
				truncated = true
				return nil
			}
			if child.IsDir() && depth < maxDepth {
				if err := walk(full, depth+1); err != nil {
					return err
				}
				if truncated {
					return nil
				}
			}
		}
		return nil
	}
	if err := walk(baseAbs, 0); err != nil {
		return "", err
	}

	out, err := json.Marshal(map[string]interface{}{
		"base_path":      baseRel,
		"workspace_root": root,
		"max_depth":      maxDepth,
		"entries":        entries,
		"truncated":      truncated,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s *Server) workspaceReadText(args map[string]interface{}) (string, error) {
	args = normalizeWorkspaceReadTextArgs(args)
	relPath := workspaceCompatPathString(args)
	if strings.TrimSpace(relPath) == "" {
		return "", errors.New("path must be a non-empty string")
	}
	startLine := 1
	if v, ok := workspaceCompatValue(args, "start_line", "startLine"); ok {
		n, convErr := asInt(v)
		if convErr != nil {
			return "", fmt.Errorf("start_line must be an integer")
		}
		startLine = n
	}
	endLine := 0
	if v, ok := workspaceCompatValue(args, "end_line", "endLine"); ok {
		n, convErr := asInt(v)
		if convErr != nil {
			return "", fmt.Errorf("end_line must be an integer")
		}
		endLine = n
	}
	maxBytes := maxWorkspaceReadBytes
	if v, ok := workspaceCompatValue(args, "max_bytes", "maxBytes"); ok {
		n, convErr := asInt(v)
		if convErr != nil {
			return "", fmt.Errorf("max_bytes must be an integer")
		}
		maxBytes = n
	}
	if maxBytes <= 0 || maxBytes > maxWorkspaceReadBytes {
		maxBytes = maxWorkspaceReadBytes
	}

	absPath, normPath, err := s.resolveWorkspacePath(relPath, false)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", normPath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
		for len(data) > 0 && !utf8.Valid(data) {
			data = data[:len(data)-1]
		}
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text: %s", normPath)
	}
	text := string(data)

	if startLine < 1 {
		return "", errors.New("start_line must be >= 1")
	}
	lines := strings.Split(text, "\n")
	totalLines := len(lines)
	if totalLines == 0 {
		totalLines = 1
	}
	if startLine > totalLines {
		return "", fmt.Errorf("start_line %d out of range (total lines: %d)", startLine, totalLines)
	}
	from := startLine - 1
	to := totalLines
	if endLine > 0 {
		to = endLine
	}
	if to > totalLines {
		to = totalLines
	}
	if to < startLine {
		return "", errors.New("end_line must be >= start_line")
	}
	content := strings.Join(lines[from:to], "\n")

	out, err := json.Marshal(map[string]interface{}{
		"path":        normPath,
		"start_line":  startLine,
		"end_line":    to,
		"total_lines": totalLines,
		"truncated":   truncated,
		"content":     content,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s *Server) workspaceWriteText(args map[string]interface{}) (string, error) {
	args = normalizeWorkspaceWriteTextArgs(args)
	relPath := workspaceCompatPathString(args)
	if strings.TrimSpace(relPath) == "" {
		return "", errors.New("path must be a non-empty string")
	}
	contentVal, ok := workspaceCompatValue(args, "content", "text", "body", "value", "chunk")
	if !ok {
		return "", errors.New("content is required")
	}
	content, err := asTextContent(contentVal)
	if err != nil {
		return "", errors.New("content must be text-compatible (string, number, boolean, object, or array)")
	}
	if len(content) > maxWorkspaceWriteBytes {
		return "", fmt.Errorf("content too large: %d bytes (max %d)", len(content), maxWorkspaceWriteBytes)
	}
	appendMode, err := asBoolDefault(args, "append", false)
	if err != nil {
		return "", err
	}
	createDirs, err := asBoolDefault(args, "create_dirs", true)
	if err != nil {
		return "", err
	}

	absPath, normPath, err := s.resolveWorkspacePath(relPath, false)
	if err != nil {
		return "", err
	}
	if createDirs {
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return "", err
		}
	}

	if appendMode {
		f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		_, err = f.WriteString(content)
		closeErr := f.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
	} else {
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return "", err
		}
	}

	out, err := json.Marshal(map[string]interface{}{
		"path":          normPath,
		"bytes_written": len(content),
		"append":        appendMode,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s *Server) workspaceSearchText(args map[string]interface{}) (string, error) {
	args = normalizeWorkspaceSearchTextArgs(args)
	queryVal, ok := workspaceCompatValue(args, "query", "q", "search", "keyword", "text", "content")
	if !ok {
		return "", errors.New("query is required")
	}
	query, err := asString(queryVal)
	if err != nil || strings.TrimSpace(query) == "" {
		return "", errors.New("query must be a non-empty string")
	}
	scope := "."
	if path := workspaceCompatPathString(args); path != "" {
		scope = path
	}
	maxResults := 50
	if v, ok := workspaceCompatValue(args, "max_results", "maxResults", "limit"); ok {
		n, convErr := asInt(v)
		if convErr != nil {
			return "", fmt.Errorf("max_results must be an integer")
		}
		maxResults = n
	}
	if maxResults <= 0 {
		maxResults = 1
	}
	if maxResults > maxWorkspaceSearchResults {
		maxResults = maxWorkspaceSearchResults
	}
	includeHidden, err := asBoolDefault(args, "include_hidden", false)
	if err != nil {
		return "", err
	}
	caseSensitive, err := asBoolDefault(args, "case_sensitive", false)
	if err != nil {
		return "", err
	}

	baseAbs, baseRel, err := s.resolveWorkspacePath(scope, true)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(baseAbs)
	if err != nil {
		return "", err
	}

	root := s.workspaceAbsRoot()
	matches := make([]workspaceSearchMatch, 0, maxResults)
	searchNeedle := query
	if !caseSensitive {
		searchNeedle = strings.ToLower(query)
	}

	searchFile := func(absPath string) error {
		fi, err := os.Stat(absPath)
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if fi.Size() > maxWorkspaceReadBytes {
			return nil
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil
		}
		if len(data) == 0 || strings.IndexByte(string(data), 0) >= 0 || !utf8.Valid(data) {
			return nil
		}
		rel, err := filepath.Rel(root, absPath)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			haystack := line
			if !caseSensitive {
				haystack = strings.ToLower(line)
			}
			idx := strings.Index(haystack, searchNeedle)
			if idx < 0 {
				continue
			}
			matches = append(matches, workspaceSearchMatch{
				Path:    rel,
				Line:    i + 1,
				Column:  idx + 1,
				Preview: truncateRunes(line, 220),
			})
			if len(matches) >= maxResults {
				return nil
			}
		}
		return nil
	}

	if info.IsDir() {
		_ = filepath.WalkDir(baseAbs, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if !pathWithinRoot(root, path) {
				return filepath.SkipDir
			}
			name := d.Name()
			if !includeHidden && strings.HasPrefix(name, ".") && path != baseAbs {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			_ = searchFile(path)
			if len(matches) >= maxResults {
				return errors.New("done")
			}
			return nil
		})
	} else {
		_ = searchFile(baseAbs)
	}

	out, err := json.Marshal(map[string]interface{}{
		"query":       query,
		"path":        baseRel,
		"matches":     matches,
		"count":       len(matches),
		"truncated":   len(matches) >= maxResults,
		"max_results": maxResults,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (s *Server) workspaceReplaceText(args map[string]interface{}) (string, error) {
	args = normalizeWorkspaceReplaceTextArgs(args)
	relPath := workspaceCompatPathString(args)
	if strings.TrimSpace(relPath) == "" {
		return "", errors.New("path must be a non-empty string")
	}
	oldVal, ok := workspaceCompatValue(args, "old_text", "oldText")
	if !ok {
		return "", errors.New("old_text is required")
	}
	oldText, err := asString(oldVal)
	if err != nil || oldText == "" {
		return "", errors.New("old_text must be a non-empty string")
	}
	newVal, ok := workspaceCompatValue(args, "new_text", "newText")
	if !ok {
		return "", errors.New("new_text is required")
	}
	newText, err := asString(newVal)
	if err != nil {
		return "", errors.New("new_text must be a string")
	}
	replaceAll, err := asBoolDefault(args, "replace_all", false)
	if err != nil {
		return "", err
	}

	absPath, normPath, err := s.resolveWorkspacePath(relPath, false)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", normPath)
	}
	if fi.Size() > maxWorkspaceReadBytes {
		return "", fmt.Errorf("file too large to replace safely: %s", normPath)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text: %s", normPath)
	}
	text := string(data)

	var replaced string
	count := 0
	if replaceAll {
		count = strings.Count(text, oldText)
		replaced = strings.ReplaceAll(text, oldText, newText)
	} else {
		if strings.Contains(text, oldText) {
			count = 1
		}
		replaced = strings.Replace(text, oldText, newText, 1)
	}
	if count == 0 {
		return "", fmt.Errorf("target text not found in %s", normPath)
	}
	if len(replaced) > maxWorkspaceWriteBytes {
		return "", fmt.Errorf("result too large: %d bytes (max %d)", len(replaced), maxWorkspaceWriteBytes)
	}
	if err := os.WriteFile(absPath, []byte(replaced), 0o644); err != nil {
		return "", err
	}

	out, err := json.Marshal(map[string]interface{}{
		"path":         normPath,
		"replacements": count,
		"replace_all":  replaceAll,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func normalizeWorkspaceListFilesArgs(args map[string]interface{}) map[string]interface{} {
	normalized := cloneWorkspaceCompatArgs(args)
	if _, ok := normalized["path"]; !ok || strings.TrimSpace(asStringOrEmpty(normalized["path"])) == "" {
		if path := workspaceCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["max_depth"]; !ok {
		if maxDepth, ok := workspaceCompatValue(normalized, "max_depth", "maxDepth"); ok {
			normalized["max_depth"] = maxDepth
		}
	}
	if _, ok := normalized["include_hidden"]; !ok {
		if includeHidden, ok := workspaceCompatValue(normalized, "include_hidden", "includeHidden"); ok {
			normalized["include_hidden"] = includeHidden
		}
	}
	return normalized
}

func normalizeWorkspaceReadTextArgs(args map[string]interface{}) map[string]interface{} {
	normalized := cloneWorkspaceCompatArgs(args)
	if strings.TrimSpace(asStringOrEmpty(normalized["path"])) == "" {
		if path := workspaceCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["start_line"]; !ok {
		if startLine, ok := workspaceCompatValue(normalized, "start_line", "startLine"); ok {
			normalized["start_line"] = startLine
		}
	}
	if _, ok := normalized["end_line"]; !ok {
		if endLine, ok := workspaceCompatValue(normalized, "end_line", "endLine"); ok {
			normalized["end_line"] = endLine
		}
	}
	if _, ok := normalized["max_bytes"]; !ok {
		if maxBytes, ok := workspaceCompatValue(normalized, "max_bytes", "maxBytes"); ok {
			normalized["max_bytes"] = maxBytes
		}
	}
	return normalized
}

func normalizeWorkspaceWriteTextArgs(args map[string]interface{}) map[string]interface{} {
	normalized := cloneWorkspaceCompatArgs(args)
	if strings.TrimSpace(asStringOrEmpty(normalized["path"])) == "" {
		if path := workspaceCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["content"]; !ok {
		if content, ok := workspaceCompatValue(normalized, "content", "text", "body", "value", "chunk"); ok {
			normalized["content"] = content
		}
	}
	if _, ok := normalized["append"]; !ok {
		if appendMode, ok := workspaceCompatValue(normalized, "append"); ok {
			normalized["append"] = appendMode
		}
	}
	if _, ok := normalized["create_dirs"]; !ok {
		if createDirs, ok := workspaceCompatValue(normalized, "create_dirs", "createDirs"); ok {
			normalized["create_dirs"] = createDirs
		}
	}
	return normalized
}

func normalizeWorkspaceSearchTextArgs(args map[string]interface{}) map[string]interface{} {
	normalized := cloneWorkspaceCompatArgs(args)
	if _, ok := normalized["query"]; !ok || strings.TrimSpace(asStringOrEmpty(normalized["query"])) == "" {
		if query, ok := workspaceCompatValue(normalized, "query", "q", "search", "keyword", "text", "content"); ok {
			normalized["query"] = query
		}
	}
	if _, ok := normalized["path"]; !ok || strings.TrimSpace(asStringOrEmpty(normalized["path"])) == "" {
		if path := workspaceCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["max_results"]; !ok {
		if maxResults, ok := workspaceCompatValue(normalized, "max_results", "maxResults", "limit"); ok {
			normalized["max_results"] = maxResults
		}
	}
	if _, ok := normalized["case_sensitive"]; !ok {
		if caseSensitive, ok := workspaceCompatValue(normalized, "case_sensitive", "caseSensitive"); ok {
			normalized["case_sensitive"] = caseSensitive
		}
	}
	if _, ok := normalized["include_hidden"]; !ok {
		if includeHidden, ok := workspaceCompatValue(normalized, "include_hidden", "includeHidden"); ok {
			normalized["include_hidden"] = includeHidden
		}
	}
	return normalized
}

func normalizeWorkspaceReplaceTextArgs(args map[string]interface{}) map[string]interface{} {
	normalized := cloneWorkspaceCompatArgs(args)
	if strings.TrimSpace(asStringOrEmpty(normalized["path"])) == "" {
		if path := workspaceCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["old_text"]; !ok {
		if oldText, ok := workspaceCompatValue(normalized, "old_text", "oldText"); ok {
			normalized["old_text"] = oldText
		}
	}
	if _, ok := normalized["new_text"]; !ok {
		if newText, ok := workspaceCompatValue(normalized, "new_text", "newText"); ok {
			normalized["new_text"] = newText
		}
	}
	if _, ok := normalized["replace_all"]; !ok {
		if replaceAll, ok := workspaceCompatValue(normalized, "replace_all", "replaceAll"); ok {
			normalized["replace_all"] = replaceAll
		}
	}
	return normalized
}

func cloneWorkspaceCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}
	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}
	return normalized
}

func workspaceCompatValue(args map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if v, ok := args[key]; ok && v != nil {
			return v, true
		}
	}
	for _, containerKey := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := workspaceCompatMap(args[containerKey])
		if !ok {
			continue
		}
		for _, key := range keys {
			if v, ok := nested[key]; ok && v != nil {
				return v, true
			}
		}
	}
	return nil, false
}

func workspaceCompatMap(v interface{}) (map[string]interface{}, bool) {
	switch typed := v.(type) {
	case map[string]interface{}:
		return typed, true
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return nil, false
		}
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(raw), &parsed) == nil {
			return parsed, true
		}
	}
	return nil, false
}

func workspaceCompatPathString(args map[string]interface{}) string {
	return workspaceCompatString(
		args,
		"path",
		"file_path",
		"filePath",
		"filepath",
		"path_name",
		"pathName",
		"pathname",
		"filename",
		"fileName",
		"target_path",
		"targetPath",
		"target_file",
		"targetFile",
		"output_path",
		"outputPath",
		"file",
	)
}

func workspaceCompatString(args map[string]interface{}, keys ...string) string {
	v, ok := workspaceCompatValue(args, keys...)
	if !ok {
		return ""
	}
	return strings.TrimSpace(asStringOrEmpty(v))
}

func asStringOrEmpty(v interface{}) string {
	s, err := asString(v)
	if err != nil {
		return ""
	}
	return s
}

func (s *Server) workspaceAbsRoot() string {
	root := strings.TrimSpace(s.rootDir)
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

func (s *Server) resolveWorkspacePath(raw string, allowDot bool) (absPath string, relPath string, err error) {
	rel, err := cleanWorkspaceRelPath(raw, allowDot)
	if err != nil {
		return "", "", err
	}
	root := s.workspaceAbsRoot()
	if rel == "." {
		return root, ".", nil
	}
	target := filepath.Clean(filepath.Join(root, rel))
	if !pathWithinRoot(root, target) {
		return "", "", errors.New("path escapes workspace root")
	}
	return target, filepath.ToSlash(rel), nil
}

func cleanWorkspaceRelPath(raw string, allowDot bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.ContainsRune(trimmed, 0) {
		return "", errors.New("invalid path")
	}
	if trimmed == "" {
		if allowDot {
			return ".", nil
		}
		return "", errors.New("path is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", errors.New("path must be relative to workspace root")
	}
	clean := filepath.Clean(trimmed)
	cleanSlash := filepath.ToSlash(clean)
	if clean == "." {
		if allowDot {
			return clean, nil
		}
		return "", errors.New("path must point to a file")
	}
	if clean == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", errors.New("path cannot escape workspace root")
	}
	return clean, nil
}

func pathWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func asString(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	default:
		return "", fmt.Errorf("expected string, got %T", v)
	}
}

func asTextContent(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case bool:
		return strconv.FormatBool(t), nil
	case int:
		return strconv.Itoa(t), nil
	case int8:
		return strconv.FormatInt(int64(t), 10), nil
	case int16:
		return strconv.FormatInt(int64(t), 10), nil
	case int32:
		return strconv.FormatInt(int64(t), 10), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case uint:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint64:
		return strconv.FormatUint(t, 10), nil
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case map[string]interface{}, []interface{}:
		raw, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("expected text-compatible JSON, got %T", v)
		}
		return string(raw), nil
	case nil:
		return "", errors.New("expected non-null content")
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("expected text-compatible value, got %T", v)
		}
		return string(raw), nil
	}
}

func asInt(v interface{}) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int32:
		return int(t), nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case float32:
		return int(t), nil
	case json.Number:
		i, err := t.Int64()
		return int(i), err
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", v)
	}
}

func asBoolDefault(args map[string]interface{}, key string, def bool) (bool, error) {
	v, ok := workspaceCompatValue(args, key)
	if !ok {
		return def, nil
	}
	switch t := v.(type) {
	case bool:
		return t, nil
	case string:
		normalized := strings.TrimSpace(strings.ToLower(t))
		if normalized == "true" {
			return true, nil
		}
		if normalized == "false" {
			return false, nil
		}
	}
	return false, fmt.Errorf("%s must be a boolean", key)
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= maxRunes {
		return s
	}
	return string(rs[:maxRunes]) + "..."
}

func (s *Server) successResponse(id any, result interface{}) ([]byte, error) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(resp)
}

func (s *Server) errorResponse(id any, code int, message string) ([]byte, error) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	}
	return json.Marshal(resp)
}

// SendToSession sends a message to a specific session's SSE stream.
func (s *Server) SendToSession(sessionID string, data []byte) {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case sess.Messages <- data:
	default:
		log.Printf("[mcp] WARNING: message buffer full for session %s, dropping message (%d bytes)", sessionID, len(data))
	}
}

// SetWorkspace sets the workspace reader for MCP resource access.
func (s *Server) SetWorkspace(w WorkspaceReader) {
	s.workspace = w
}

func (s *Server) handleResourcesList(id any) ([]byte, error) {
	resources := []mcpResource{}
	if s.workspace != nil {
		files := s.workspace.LoadContextFiles()
		for name := range files {
			resources = append(resources, mcpResource{
				URI:      "workspace:///" + name,
				Name:     name,
				MimeType: "text/markdown",
			})
		}
	}
	return s.successResponse(id, resourcesListResult{Resources: resources})
}

func (s *Server) handleResourcesRead(id any, params json.RawMessage) ([]byte, error) {
	var p resourceReadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.errorResponse(id, -32602, "invalid params")
	}

	if s.workspace == nil {
		return s.errorResponse(id, -32002, "workspace not configured")
	}

	// Strip workspace:/// prefix
	name := strings.TrimPrefix(p.URI, "workspace:///")
	if name == "" || name == p.URI {
		return s.errorResponse(id, -32602, "invalid resource URI — expected workspace:///FILENAME")
	}
	// Reject path traversal attempts (e.g. "../../etc/passwd")
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return s.errorResponse(id, -32602, "invalid resource name")
	}

	files := s.workspace.LoadContextFiles()
	content, ok := files[name]
	if !ok {
		return s.errorResponse(id, -32002, fmt.Sprintf("resource not found: %s", name))
	}

	return s.successResponse(id, resourceReadResult{
		Contents: []resourceContent{{
			URI:      p.URI,
			MimeType: "text/markdown",
			Text:     content,
		}},
	})
}

func (s *Server) handlePromptsList(id any) ([]byte, error) {
	prompts := []mcpPrompt{
		{
			Name:        "agent_task",
			Description: "Create an autonomous agent task with a goal",
			Arguments: []promptArgument{
				{Name: "goal", Description: "The task goal to accomplish", Required: true},
			},
		},
		{
			Name:        "code_review",
			Description: "Review code for bugs, security issues, and improvements",
			Arguments: []promptArgument{
				{Name: "code", Description: "The code to review", Required: true},
				{Name: "language", Description: "Programming language"},
			},
		},
		{
			Name:        "summarize",
			Description: "Summarize text or conversation context",
			Arguments: []promptArgument{
				{Name: "text", Description: "The text to summarize", Required: true},
			},
		},
		{
			Name:        "coding_task",
			Description: "Autonomously complete a coding task with MCP tools in CLI and non-CLI environments",
			Arguments: []promptArgument{
				{Name: "goal", Description: "Coding goal to complete", Required: true},
				{Name: "constraints", Description: "Optional constraints (language, style, tests, etc.)"},
				{Name: "repository_path", Description: "Optional repository path relative to MCP workspace root"},
			},
		},
	}
	return s.successResponse(id, promptsListResult{Prompts: prompts})
}

func (s *Server) handlePromptsGet(id any, params json.RawMessage) ([]byte, error) {
	var p promptGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.errorResponse(id, -32602, "invalid params")
	}

	switch p.Name {
	case "agent_task":
		goal := p.Arguments["goal"]
		if goal == "" {
			return s.errorResponse(id, -32602, "goal argument is required")
		}
		return s.successResponse(id, promptGetResult{
			Description: "Autonomous agent task",
			Messages: []promptMessage{
				{Role: "user", Content: contentBlock{Type: "text", Text: fmt.Sprintf("Please complete this task autonomously: %s", goal)}},
			},
		})
	case "code_review":
		code := p.Arguments["code"]
		if code == "" {
			return s.errorResponse(id, -32602, "code argument is required")
		}
		lang := p.Arguments["language"]
		prompt := fmt.Sprintf("Review this code for bugs, security issues, and improvements:\n\n```%s\n%s\n```", lang, code)
		return s.successResponse(id, promptGetResult{
			Description: "Code review",
			Messages: []promptMessage{
				{Role: "user", Content: contentBlock{Type: "text", Text: prompt}},
			},
		})
	case "summarize":
		text := p.Arguments["text"]
		if text == "" {
			return s.errorResponse(id, -32602, "text argument is required")
		}
		return s.successResponse(id, promptGetResult{
			Description: "Summarize text",
			Messages: []promptMessage{
				{Role: "user", Content: contentBlock{Type: "text", Text: fmt.Sprintf("Please summarize the following:\n\n%s", text)}},
			},
		})
	case "coding_task":
		goal := strings.TrimSpace(p.Arguments["goal"])
		if goal == "" {
			return s.errorResponse(id, -32602, "goal argument is required")
		}
		constraints := strings.TrimSpace(p.Arguments["constraints"])
		repo := strings.TrimSpace(p.Arguments["repository_path"])

		var sb strings.Builder
		sb.WriteString("Complete this coding task autonomously until done.\n")
		sb.WriteString("Goal: ")
		sb.WriteString(goal)
		sb.WriteString("\n\nExecution protocol:\n")
		sb.WriteString("1. Inspect repository structure and relevant files.\n")
		sb.WriteString("2. Implement minimal, correct code changes.\n")
		sb.WriteString("3. Run targeted tests/verification.\n")
		sb.WriteString("4. Fix regressions and re-run verification.\n")
		sb.WriteString("5. Return concise result summary with changed files and validation.\n\n")
		sb.WriteString("Tooling guidance:\n")
		sb.WriteString("- Prefer orchestrator.run to schedule deterministic/transformative/generative phases with parallel execution and compressed final context.\n")
		sb.WriteString("- Use read/write/edit/ls/grep/find for deterministic workspace file work.\n")
		sb.WriteString("- Use bash when command execution is needed.\n")
		sb.WriteString("- Do not depend on `blue` CLI subcommands; fall back to standard shell and MCP workspace tools if CLI is unavailable.\n")
		if repo != "" {
			sb.WriteString("- Repository path hint: ")
			sb.WriteString(repo)
			sb.WriteString("\n")
		}
		if constraints != "" {
			sb.WriteString("\nConstraints:\n")
			sb.WriteString(constraints)
			sb.WriteString("\n")
		}

		return s.successResponse(id, promptGetResult{
			Description: "Autonomous coding task",
			Messages: []promptMessage{
				{Role: "user", Content: contentBlock{Type: "text", Text: sb.String()}},
			},
		})
	default:
		return s.errorResponse(id, -32602, fmt.Sprintf("unknown prompt: %s", p.Name))
	}
}
