// Package tools provides a framework for defining and executing tools/functions.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

// Common errors
var (
	ErrToolNotFound = errors.New("tool not found")
)

// ToolDefinition describes a tool that can be called by an LLM.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
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
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Definition().Name] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tool names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Definitions returns all tool definitions.
func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
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
		return nil, ErrToolNotFound
	}

	return tool.Execute(ctx, args)
}

// ExecuteJSON runs a tool by name with JSON-encoded arguments.
func (e *Executor) ExecuteJSON(ctx context.Context, name string, argsJSON string) (interface{}, error) {
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nil, err
		}
	}
	return e.Execute(ctx, name, args)
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
