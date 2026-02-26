// Package tools provides a framework for defining and executing tools/functions.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Common errors
var (
	ErrToolNotFound = errors.New("tool not found")
)

// ForwardedResult wraps a tool result that was auto-forwarded from exec.
// The ActualTool field indicates which tool actually executed the request,
// allowing the UI to display the correct tool name (e.g. "web_search" instead of "exec").
type ForwardedResult struct {
	ActualTool string
	Result     interface{}
}

// ToolDefinition describes a tool that can be called by an LLM.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Icon        string                 `json:"icon,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// Tool is the interface that all tools must implement.
type Tool interface {
	// Definition returns the tool's definition.
	Definition() ToolDefinition

	// Execute runs the tool with the given arguments.
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Registry manages registered tools.
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	disabled map[string]Tool // disabled tools (still registered, but hidden from Definitions/List)
	version  uint64         // incremented on every mutation (Register/Disable/Enable)
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]Tool),
		disabled: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Definition().Name
	delete(r.disabled, name)
	r.tools[name] = tool
	r.version++
}

// Version returns the current mutation version of the registry.
// It increments on every Register, Disable, or Enable call.
func (r *Registry) Version() uint64 {
	r.mu.RLock()
	v := r.version
	r.mu.RUnlock()
	return v
}

// Disable moves a tool from active to disabled. Disabled tools are hidden from
// Definitions() and List() (not sent to LLM) but still accessible via Get().
func (r *Registry) Disable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool, ok := r.tools[name]
	if !ok {
		return false
	}
	delete(r.tools, name)
	r.disabled[name] = tool
	r.version++
	return true
}

// Enable moves a tool from disabled back to active.
func (r *Registry) Enable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool, ok := r.disabled[name]
	if !ok {
		return false
	}
	delete(r.disabled, name)
	r.tools[name] = tool
	r.version++
	return true
}

// IsDisabled returns true if the tool exists but is disabled.
func (r *Registry) IsDisabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.disabled[name]
	return ok
}

// Get retrieves a tool by name. Returns both active and disabled tools.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t := r.tools[name]; t != nil {
		return t
	}
	return r.disabled[name]
}

// List returns all active (enabled) tool names sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListDisabled returns all disabled tool names sorted alphabetically.
func (r *Registry) ListDisabled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.disabled))
	for name := range r.disabled {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Definitions returns all tool definitions sorted by name.
func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// Executor handles tool execution.
type Executor struct {
	registry *Registry
}

// NewExecutor creates a new tool executor.
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry: registry,
	}
}

// Execute runs a tool by name with the given arguments.
func (e *Executor) Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	// Check context first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	tool := e.registry.Get(name)
	if tool == nil {
		// Skill fallback: if the LLM calls a tool that doesn't exist in the
		// tool registry, try forwarding to `blue <name> key=value` via exec.
		// This handles skills (analyze, web_search, etc.) that the LLM may
		// call as native tools despite the system prompt saying to use exec.
		if execTool := e.registry.Get("exec"); execTool != nil {
			cmd := buildSkillCommand(name, args)
			return execTool.Execute(ctx, map[string]interface{}{"command": cmd})
		}
		return nil, ErrToolNotFound
	}

	return tool.Execute(ctx, args)
}

// ExecuteJSON runs a tool by name with JSON-encoded arguments.
func (e *Executor) ExecuteJSON(ctx context.Context, name string, argsJSON string) (interface{}, error) {
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			// Some providers double-encode arguments as a JSON string wrapping
			// a JSON object (e.g. "\"{\\\"command\\\":\\\"ls\\\"}\"").
			// Try unwrapping one level of string encoding.
			var inner string
			if json.Unmarshal([]byte(argsJSON), &inner) == nil && len(inner) > 0 && inner[0] == '{' {
				if err2 := json.Unmarshal([]byte(inner), &args); err2 == nil {
					return e.Execute(ctx, name, args)
				}
			}
			return nil, err
		}
	}
	return e.Execute(ctx, name, args)
}

// buildSkillCommand builds a `blue <name> key=value ...` command string
// from a tool name and its arguments for skill fallback execution.
func buildSkillCommand(name string, args map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("blue ")
	sb.WriteString(name)
	for k, v := range args {
		sb.WriteByte(' ')
		sb.WriteString(k)
		sb.WriteByte('=')
		switch val := v.(type) {
		case string:
			// Quote strings that contain spaces
			if strings.ContainsAny(val, " \t\n\"") {
				sb.WriteString(fmt.Sprintf("%q", val))
			} else {
				sb.WriteString(val)
			}
		default:
			b, _ := json.Marshal(val)
			sb.WriteString(fmt.Sprintf("%q", string(b)))
		}
	}
	return sb.String()
}

// MockTool is a mock implementation of Tool for testing.
type MockTool struct {
	name        string
	description string
	result      interface{}
	err         error
	delay       bool
}

// NewMockTool creates a new mock tool.
func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
	}
}

// SetResult sets the result to return from Execute.
func (m *MockTool) SetResult(result interface{}) {
	m.result = result
}

// SetError sets the error to return from Execute.
func (m *MockTool) SetError(err error) {
	m.err = err
}

// SetDelay sets whether Execute should wait for context cancellation.
func (m *MockTool) SetDelay(delay bool) {
	m.delay = delay
}

// Definition returns the tool's definition.
func (m *MockTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        m.name,
		Description: m.description,
	}
}

// Execute runs the mock tool.
func (m *MockTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.delay {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	return m.result, nil
}
