// Package mcp implements a Model Context Protocol (MCP) server.
// It exposes Blue's tools and skills to external agents via JSON-RPC over SSE transport.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "blue-mcp"
	ServerVersion   = "0.10.29"
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
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
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

	mu       sync.RWMutex
	sessions map[string]*Session
	closed   atomic.Bool

	toolCallTimeout time.Duration // 0 means use defaultToolCallTimeout

	toolsCacheMu sync.RWMutex
	toolsCached  *toolsCache
}

// NewServer creates a new MCP server.
func NewServer(registry *tools.Registry, executor *tools.Executor) *Server {
	return &Server{
		registry: registry,
		executor: executor,
		sessions: make(map[string]*Session),
	}
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
	mcpTools := make([]mcpTool, 0, len(defs))
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

	ctx, cancel := context.WithTimeout(ctx, s.getToolCallTimeout())
	defer cancel()

	result, err := s.executor.Execute(ctx, p.Name, p.Arguments)
	if err != nil {
		return s.successResponse(id, toolCallResult{
			Content: []contentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		})
	}

	var text string
	switch v := result.(type) {
	case string:
		text = v
	default:
		b, _ := json.Marshal(v)
		text = string(b)
	}

	return s.successResponse(id, toolCallResult{
		Content: []contentBlock{{Type: "text", Text: text}},
	})
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
	default:
		return s.errorResponse(id, -32602, fmt.Sprintf("unknown prompt: %s", p.Name))
	}
}
