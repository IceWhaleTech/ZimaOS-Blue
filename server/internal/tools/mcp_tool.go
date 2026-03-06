package tools

import (
	"context"
	"errors"
	"strings"
)

// MCPTool is a unified fallback dispatcher:
// - Prefer native built-in tool execution when available.
// - Fall back to existing executor behavior (unknown tool -> exec "blue <tool> ...").
type MCPTool struct {
	registry *Registry
}

// NewMCPTool creates an MCP dispatcher bound to the given registry.
func NewMCPTool(registry *Registry) *MCPTool {
	return &MCPTool{registry: registry}
}

// Definition returns the tool definition.
func (t *MCPTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "mcp",
		Description: "Unified MCP server call. Built-in tools are preferred; unknown tools fall back through exec.",
		Icon:        "plug",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tool": map[string]interface{}{
					"type":        "string",
					"description": "Target tool name",
				},
				"params": map[string]interface{}{
					"type":                 "object",
					"description":          "Target tool parameters",
					"additionalProperties": true,
				},
			},
			"required": []string{"tool"},
		},
	}
}

// Execute dispatches to a target tool.
func (t *MCPTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	toolName, err := fsAsString(args, "tool")
	if err != nil || strings.TrimSpace(toolName) == "" {
		return nil, errors.New("tool must be a non-empty string")
	}
	toolName = strings.TrimSpace(toolName)
	if strings.EqualFold(toolName, "mcp") {
		return nil, errors.New("mcp tool cannot call itself")
	}

	params := map[string]interface{}{}
	if raw, ok := args["params"]; ok && raw != nil {
		cast, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("params must be an object")
		}
		params = cast
	}

	if t.registry == nil {
		return nil, errors.New("mcp registry is not configured")
	}

	executor := NewExecutor(t.registry)
	return executor.Execute(ctx, toolName, params)
}
