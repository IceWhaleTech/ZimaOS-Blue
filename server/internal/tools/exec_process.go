package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ProcessTool implements the Tool interface for managing exec sessions.
type ProcessTool struct {
	sessions *SessionRegistry
}

// NewProcessTool creates a new process management tool.
func NewProcessTool(sessions *SessionRegistry) *ProcessTool {
	return &ProcessTool{sessions: sessions}
}

// Definition returns the tool definition for the LLM.
func (t *ProcessTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "process",
		Description: "Manage running and recently finished exec sessions. Actions: list (show all sessions), poll (get pending output), log (get full output), kill (terminate a session).",
		Icon:        "process",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform: list, poll, log, kill",
					"enum":        []string{"list", "poll", "log", "kill"},
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID (required for poll, log, kill)",
				},
			},
			"required": []string{"action"},
		},
	}
}

type processListEntry struct {
	SessionID string `json:"session_id"`
	Command   string `json:"command"`
	Status    string `json:"status"`
	PID       int    `json:"pid,omitempty"`
	StartedAt int64  `json:"started_at"`
	ExitCode  *int   `json:"exit_code,omitempty"`
}

type processListResult struct {
	Running  []processListEntry `json:"running"`
	Finished []processListEntry `json:"finished"`
}

type processPollResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type processLogResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Output    string `json:"output"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type processKillResult struct {
	SessionID string `json:"session_id"`
	Killed    bool   `json:"killed"`
}

// Execute performs the requested process management action.
func (t *ProcessTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	action := strings.TrimSpace(strings.ToLower(firstCompatString(args, "action", "op", "operation", "command")))
	sessionID := firstCompatString(args, "session_id", "sessionId", "id")

	switch action {
	case "list":
		return t.list()
	case "poll":
		return t.poll(sessionID)
	case "log":
		return t.log(sessionID)
	case "kill":
		return t.kill(sessionID)
	default:
		return nil, fmt.Errorf("unknown action %q; use list, poll, log, or kill", action)
	}
}

func (t *ProcessTool) list() (interface{}, error) {
	running := t.sessions.ListRunning()
	finished := t.sessions.ListFinished()

	result := processListResult{
		Running:  make([]processListEntry, 0, len(running)),
		Finished: make([]processListEntry, 0, len(finished)),
	}

	for _, s := range running {
		result.Running = append(result.Running, processListEntry{
			SessionID: s.ID,
			Command:   truncateStr(s.Command, 120),
			Status:    string(s.Status),
			PID:       s.PID,
			StartedAt: s.StartedAt.UnixMilli(),
		})
	}

	for _, s := range finished {
		result.Finished = append(result.Finished, processListEntry{
			SessionID: s.ID,
			Command:   truncateStr(s.Command, 120),
			Status:    string(s.Status),
			StartedAt: s.StartedAt.UnixMilli(),
			ExitCode:  s.ExitCode,
		})
	}

	data, _ := json.Marshal(result)
	return string(data), nil
}

func (t *ProcessTool) poll(sessionID string) (interface{}, error) {
	if sessionID == "" {
		return nil, errors.New("session_id is required for poll")
	}

	s := t.sessions.Get(sessionID)
	if s != nil {
		result := processPollResult{
			SessionID: s.ID,
			Status:    string(s.Status),
			Stdout:    SanitizeBinaryOutput(s.Stdout.Drain()),
			Stderr:    SanitizeBinaryOutput(s.Stderr.Drain()),
			Truncated: s.Stdout.Truncated() || s.Stderr.Truncated(),
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	f := t.sessions.GetFinished(sessionID)
	if f != nil {
		result := processPollResult{
			SessionID: f.ID,
			Status:    string(f.Status),
			Stdout:    SanitizeBinaryOutput(f.Output),
			Truncated: f.Truncated,
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	return nil, fmt.Errorf("session %q not found", sessionID)
}

func (t *ProcessTool) log(sessionID string) (interface{}, error) {
	if sessionID == "" {
		return nil, errors.New("session_id is required for log")
	}

	s := t.sessions.Get(sessionID)
	if s != nil {
		result := processLogResult{
			SessionID: s.ID,
			Status:    string(s.Status),
			Output:    SanitizeBinaryOutput(s.Stdout.String() + s.Stderr.String()),
			Truncated: s.Stdout.Truncated() || s.Stderr.Truncated(),
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	f := t.sessions.GetFinished(sessionID)
	if f != nil {
		result := processLogResult{
			SessionID: f.ID,
			Status:    string(f.Status),
			Output:    SanitizeBinaryOutput(f.Output),
			ExitCode:  f.ExitCode,
			Truncated: f.Truncated,
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	return nil, fmt.Errorf("session %q not found", sessionID)
}

func (t *ProcessTool) kill(sessionID string) (interface{}, error) {
	if sessionID == "" {
		return nil, errors.New("session_id is required for kill")
	}

	s := t.sessions.Get(sessionID)
	if s == nil {
		return nil, fmt.Errorf("session %q not found or already finished", sessionID)
	}

	if s.PID > 0 {
		KillProcessTree(s.PID)
	}

	code := -1
	t.sessions.MarkExited(sessionID, &code, "SIGKILL", ProcessKilled)

	result := processKillResult{
		SessionID: sessionID,
		Killed:    true,
	}
	data, _ := json.Marshal(result)
	return string(data), nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
