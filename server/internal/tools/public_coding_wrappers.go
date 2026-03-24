package tools

import (
	"context"
	"errors"
	"fmt"
)

const execStrictShellArg = "_strict_shell"

type delegatingTool struct {
	def        ToolDefinition
	registry   *Registry
	targetName string
	mapArgs    func(map[string]interface{}) map[string]interface{}
}

func (t *delegatingTool) Definition() ToolDefinition {
	return t.def
}

func (t *delegatingTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.registry == nil {
		return nil, errors.New("tool registry unavailable")
	}
	target := t.registry.Get(t.targetName)
	if target == nil {
		return nil, fmt.Errorf("backend tool %q is not registered", t.targetName)
	}
	mapped := args
	if t.mapArgs != nil {
		mapped = t.mapArgs(args)
	}
	return target.Execute(ctx, mapped)
}

func NewPublicReadTool(registry *Registry) Tool {
	return &delegatingTool{
		registry:   registry,
		targetName: "file_read",
		def: ToolDefinition{
			Name:        "read",
			Description: "Read a local workspace file. Use offset and limit to read a line range without exposing the full legacy file_read surface.",
			Icon:        "file-read",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read.",
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Optional 0-based line offset to start reading from.",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Optional maximum number of lines to return.",
					},
				},
				"required": []string{"path"},
			},
		},
		mapArgs: normalizePublicReadArgs,
	}
}

func NewPublicWriteTool(registry *Registry) Tool {
	return &delegatingTool{
		registry:   registry,
		targetName: "file_write",
		def: ToolDefinition{
			Name:        "write",
			Description: "Write text to a local workspace file. Creates the file if needed and supports append mode for chunked writes.",
			Icon:        "file-write",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to write.",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The text content to write.",
					},
					"append": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, append instead of overwrite.",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		mapArgs: normalizePublicWriteArgs,
	}
}

func NewPublicBashTool(registry *Registry) Tool {
	return &delegatingTool{
		registry:   registry,
		targetName: "exec",
		def: ToolDefinition{
			Name:        "bash",
			Description: "Execute a real shell command and return the actual stdout, stderr, and exit code. This public shell surface does not auto-forward into tools or skills.",
			Icon:        "terminal",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Shell command to execute.",
					},
					"timeout": map[string]interface{}{
						"type":        "number",
						"description": "Optional timeout in seconds.",
					},
				},
				"required": []string{"command"},
			},
			RiskLevel: string(RiskLevelHigh),
		},
		mapArgs: normalizePublicBashArgs,
	}
}

func normalizePublicReadArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeFileReadCompatArgs(cloneJSONInterfaceMap(args))
	if normalized == nil {
		normalized = map[string]interface{}{}
	}
	if _, hasStart := firstCompatValue(normalized, "start_line", "startLine"); !hasStart {
		if offset, ok := firstCompatIntDeep(normalized, "offset"); ok {
			if offset < 0 {
				offset = 0
			}
			normalized["start_line"] = offset + 1
		}
	}
	if _, hasEnd := firstCompatValue(normalized, "end_line", "endLine"); !hasEnd {
		if limit, ok := firstCompatIntDeep(normalized, "limit"); ok && limit > 0 {
			startLine := 1
			if start, ok := firstCompatIntDeep(normalized, "start_line", "startLine"); ok && start > 0 {
				startLine = start
			}
			normalized["end_line"] = startLine + limit - 1
		}
	}
	delete(normalized, "offset")
	delete(normalized, "limit")
	return normalized
}

func normalizePublicWriteArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeFileWriteCompatArgs(cloneJSONInterfaceMap(args))
	if normalized == nil {
		return map[string]interface{}{}
	}
	return normalized
}

func normalizePublicBashArgs(args map[string]interface{}) map[string]interface{} {
	normalized := normalizeExecCompatArgs(cloneJSONInterfaceMap(args))
	if normalized == nil {
		normalized = map[string]interface{}{}
	}
	out := map[string]interface{}{
		execStrictShellArg: true,
	}
	if command := firstCompatString(normalized, "command", "cmd"); command != "" {
		out["command"] = command
	}
	if timeout, ok := compatArgValue(normalized, "timeout", "timeout_sec", "timeout_seconds", "timeoutSeconds"); ok {
		out["timeout"] = timeout
	}
	return out
}

func registerCanonicalFileSurface(registry *Registry) {
	if registry == nil {
		return
	}
	registry.Register(NewPublicReadTool(registry))
	registry.Register(NewPublicWriteTool(registry))
	hideLegacyCodingTools(registry, "file_read", "file_write", "file_delete", "write_begin", "write_chunk", "write_commit", "write_abort", "rg")
}

func registerCanonicalShellSurface(registry *Registry) {
	if registry == nil {
		return
	}
	registry.Register(NewPublicBashTool(registry))
	hideLegacyCodingTools(registry, "exec", "process")
}

func hideLegacyCodingTools(registry *Registry, names ...string) {
	if registry == nil {
		return
	}
	for _, name := range names {
		registry.Disable(name)
	}
}
