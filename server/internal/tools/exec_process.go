package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
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
		Description: "Manage running and recently finished exec sessions. Actions: list, poll (pending output), log (page retained output), kill. Log offsets are UTF-8 bytes; follow next_offset while has_more is true. A truncated buffer cannot prove absence.",
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
				"offset": map[string]interface{}{"type": "integer", "minimum": 0, "description": "For log: UTF-8 byte offset, default 0."},
				"limit":  map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 16384, "description": "For log: maximum UTF-8 bytes, default 4096."},
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
	SessionID  string `json:"session_id"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Offset     int    `json:"offset"`
	NextOffset int    `json:"next_offset"`
	TotalBytes int    `json:"total_bytes"`
	HasMore    bool   `json:"has_more"`
}

type processKillResult struct {
	SessionID string `json:"session_id"`
	Killed    bool   `json:"killed"`
}

// Execute performs the requested process management action.
func (t *ProcessTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	return executeProcessAction(t.sessions, args)
}

func executeProcessAction(sessions *SessionRegistry, args map[string]interface{}) (interface{}, error) {
	if sessions == nil {
		return nil, errors.New("session registry unavailable")
	}
	action := strings.TrimSpace(strings.ToLower(firstCompatString(args, "action", "op", "operation", "command")))
	sessionID := firstCompatString(args, "session_id", "sessionId", "id")

	switch action {
	case "list", "list_sessions", "sessions":
		return listProcessSessions(sessions)
	case "poll", "poll_session":
		return pollProcessSession(sessions, sessionID)
	case "log", "session_log":
		return logProcessSession(sessions, sessionID, args)
	case "kill", "kill_session":
		return killProcessSession(sessions, sessionID)
	default:
		return nil, fmt.Errorf("unknown action %q; use list, poll, log, or kill", action)
	}
}

func listProcessSessions(sessions *SessionRegistry) (interface{}, error) {
	running := sessions.ListRunning()
	finished := sessions.ListFinished()

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

func pollProcessSession(sessions *SessionRegistry, sessionID string) (interface{}, error) {
	if sessionID == "" {
		return nil, errors.New("session_id is required for poll")
	}

	s := sessions.Get(sessionID)
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

	f := sessions.GetFinished(sessionID)
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

func logProcessSession(sessions *SessionRegistry, sessionID string, args map[string]interface{}) (interface{}, error) {
	if sessionID == "" {
		return nil, errors.New("session_id is required for log")
	}

	var result processLogResult
	if s := sessions.Get(sessionID); s != nil {
		result = processLogResult{
			SessionID: s.ID,
			Status:    string(s.Status),
			Output:    SanitizeBinaryOutput(s.Stdout.String() + s.Stderr.String()),
			Truncated: s.Stdout.Truncated() || s.Stderr.Truncated(),
		}
	} else if f := sessions.GetFinished(sessionID); f != nil {
		result = processLogResult{
			SessionID: f.ID,
			Status:    string(f.Status),
			Output:    SanitizeBinaryOutput(f.Output),
			ExitCode:  f.ExitCode,
			Truncated: f.Truncated,
		}
	} else {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	parse := func(key string, fallback int) (int, error) {
		value, exists := firstCompatValueDeep(args, key)
		if !exists {
			return fallback, nil
		}
		n, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		return n, nil
	}
	offset, err := parse("offset", 0)
	if err != nil {
		return nil, err
	}
	limit, err := parse("limit", 4096)
	if err != nil {
		return nil, err
	}
	output := sanitizeToolString(result.Output, 0)
	if offset < 0 || offset > len(output) || (offset < len(output) && !utf8.RuneStart(output[offset])) {
		return nil, errors.New("offset must be a UTF-8 byte boundary within retained output")
	}
	if limit < 1 || limit > 16384 {
		return nil, errors.New("limit must be between 1 and 16384 bytes")
	}
	end := offset + min(limit, len(output)-offset)
	for end > offset && end < len(output) && !utf8.RuneStart(output[end]) {
		end--
	}
	if end == offset && offset < len(output) {
		return nil, errors.New("limit is too small for the next UTF-8 character")
	}
	result.Output = output[offset:end]
	result.Offset = offset
	result.NextOffset = end
	result.TotalBytes = len(output)
	result.HasMore = end < len(output)
	data, _ := json.Marshal(result)
	return string(data), nil
}

func killProcessSession(sessions *SessionRegistry, sessionID string) (interface{}, error) {
	if sessionID == "" {
		return nil, errors.New("session_id is required for kill")
	}

	s := sessions.Get(sessionID)
	if s == nil {
		return nil, fmt.Errorf("session %q not found or already finished", sessionID)
	}

	if s.PID > 0 {
		KillProcessTree(s.PID)
	}

	code := -1
	sessions.MarkExited(sessionID, &code, "SIGKILL", ProcessKilled)

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
