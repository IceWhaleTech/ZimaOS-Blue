package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// WorkspaceTool allows the agent to read and write workspace files via tool calls.
type WorkspaceTool struct {
	mgr *Manager
}

// NewWorkspaceTool creates a workspace tool backed by the given manager.
func NewWorkspaceTool(mgr *Manager) *WorkspaceTool {
	return &WorkspaceTool{mgr: mgr}
}

// Definition returns the tool definition for LLM tool-use.
func (t *WorkspaceTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: "workspace_file",
		Description: `Read or write workspace files (MEMORY.md, USER.md, etc.). These files persist across conversations and are loaded into the system prompt. Actions:
- "read": read a workspace file
- "write": overwrite a workspace file (use for MEMORY.md to store long-term facts, preferences, and notes the user asks you to remember)
- "append_daily": append a timestamped note to today's daily log (memory/YYYY-MM-DD.md) — use for conversation summaries and observations
- "complete_bootstrap": finish first-run setup (deletes BOOTSTRAP.md)

When the user says "remember this", "don't forget", or "remind me next time": read MEMORY.md first, then write back with the new information appended.`,
		Icon: "file-text",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"read", "write", "complete_bootstrap", "append_daily"},
					"description": "The action to perform",
				},
				"filename": map[string]interface{}{
					"type":        "string",
					"enum":        []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileMEMORY, FileHEARTBEAT},
					"description": "The workspace file to operate on (required for read/write)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write (required for write/append_daily)",
				},
			},
			"required": []string{"action"},
		},
	}
}

// Execute runs the workspace file tool.
func (t *WorkspaceTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action, _ := args["action"].(string)
	filename, _ := args["filename"].(string)
	content, _ := args["content"].(string)

	switch action {
	case "read":
		if filename == "" {
			return nil, fmt.Errorf("filename is required for read")
		}
		data, err := t.mgr.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filename, err)
		}
		return jsonResult(map[string]interface{}{"filename": filename, "content": data}), nil

	case "write":
		if filename == "" {
			return nil, fmt.Errorf("filename is required for write")
		}
		if content == "" {
			return nil, fmt.Errorf("content is required for write")
		}
		if err := t.mgr.WriteFile(filename, content); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", filename, err)
		}
		return jsonResult(map[string]interface{}{"filename": filename, "status": "ok", "bytes": len(content)}), nil

	case "complete_bootstrap":
		if err := t.mgr.CompleteBootstrap(); err != nil {
			return nil, fmt.Errorf("failed to complete bootstrap: %w", err)
		}
		return jsonResult(map[string]interface{}{"status": "ok", "message": "bootstrap completed, BOOTSTRAP.md removed"}), nil

	case "append_daily":
		if content == "" {
			return nil, fmt.Errorf("content is required for append_daily")
		}
		if err := t.mgr.AppendDailyLog(content); err != nil {
			return nil, fmt.Errorf("failed to append daily log: %w", err)
		}
		return jsonResult(map[string]interface{}{"status": "ok", "action": "append_daily"}), nil

	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

func jsonResult(v map[string]interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
