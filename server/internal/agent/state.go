package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/google/uuid"
)

const (
	StateUpdateKindFSObservation = "fs_observation"
	StateUpdateKindFSMutation    = "fs_mutation"
	StateUpdateKindCommand       = "command_history"
	StateUpdateKindEvidence      = "evidence"
)

const (
	GroundingStatusGrounded = "grounded"
	GroundingStatusUnknown  = "unknown"
	GroundingStatusFallback = "fallback"
	GroundingStatusRejected = "rejected"
)

type RuntimeEvent struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	StepIndex    int       `json:"step_index"`
	PlannerRound int       `json:"planner_round"`
	EventType    string    `json:"event_type"`
	PayloadJSON  string    `json:"payload_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type GroundTruthState struct {
	Files    map[string]GroundedFileFact    `json:"files,omitempty"`
	Commands map[string]GroundedCommandFact `json:"commands,omitempty"`
	Calls    map[string]GroundedToolCall    `json:"calls,omitempty"`
	Results  map[string]GroundedToolResult  `json:"results,omitempty"`
}

type GroundedFileFact struct {
	Path           string    `json:"path"`
	Exists         bool      `json:"exists"`
	Size           int64     `json:"size,omitempty"`
	ContentSHA256  string    `json:"content_sha256,omitempty"`
	LastObservedBy string    `json:"last_observed_by,omitempty"`
	LastMutatedBy  string    `json:"last_mutated_by,omitempty"`
	ObservedAt     time.Time `json:"observed_at,omitempty"`
}

type GroundedCommandFact struct {
	ToolCallID string    `json:"tool_call_id"`
	Tool       string    `json:"tool"`
	Command    string    `json:"command,omitempty"`
	ArgsHash   string    `json:"args_hash,omitempty"`
	ExitCode   int       `json:"exit_code"`
	OutputHash string    `json:"output_hash,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

type StateUpdate struct {
	UpdateID      string         `json:"update_id"`
	ToolCallID    string         `json:"tool_call_id"`
	Kind          string         `json:"kind"`
	Path          string         `json:"path,omitempty"`
	Exists        bool           `json:"exists,omitempty"`
	Size          int64          `json:"size,omitempty"`
	ContentSHA256 string         `json:"content_sha256,omitempty"`
	ObservedAt    time.Time      `json:"observed_at"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type PlannerAssertion struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
	Tool string `json:"tool,omitempty"`
}

type GroundTruthStateStore struct{}

func NewGroundTruthStateStore() *GroundTruthStateStore {
	return &GroundTruthStateStore{}
}

func NewGroundTruthState() *GroundTruthState {
	state := &GroundTruthState{}
	state.ensureMaps()
	return state
}

func (s *GroundTruthState) ensureMaps() {
	if s.Files == nil {
		s.Files = make(map[string]GroundedFileFact)
	}
	if s.Commands == nil {
		s.Commands = make(map[string]GroundedCommandFact)
	}
	if s.Calls == nil {
		s.Calls = make(map[string]GroundedToolCall)
	}
	if s.Results == nil {
		s.Results = make(map[string]GroundedToolResult)
	}
}

func (s *GroundTruthStateStore) ApplyExecution(state *GroundTruthState, exec *GroundedExecution) {
	if state == nil || exec == nil {
		return
	}
	state.ensureMaps()
	state.Calls[exec.Call.ToolCallID] = exec.Call
	state.Results[exec.Result.ToolCallID] = exec.Result
	for _, update := range exec.Updates {
		applyStateUpdate(state, update)
	}
}

func applyStateUpdate(state *GroundTruthState, update StateUpdate) {
	state.ensureMaps()
	switch update.Kind {
	case StateUpdateKindFSObservation, StateUpdateKindFSMutation:
		path := normalizeGroundPath(update.Path)
		if path == "" {
			return
		}
		fact := state.Files[path]
		fact.Path = path
		fact.Exists = update.Exists
		if update.Size > 0 || update.Exists {
			fact.Size = update.Size
		}
		if update.ContentSHA256 != "" {
			fact.ContentSHA256 = update.ContentSHA256
		}
		fact.ObservedAt = update.ObservedAt
		fact.LastObservedBy = update.ToolCallID
		if update.Kind == StateUpdateKindFSMutation {
			fact.LastMutatedBy = update.ToolCallID
		}
		state.Files[path] = fact
	case StateUpdateKindCommand:
		command := GroundedCommandFact{
			ToolCallID: update.ToolCallID,
			ObservedAt: update.ObservedAt,
		}
		if update.Metadata != nil {
			command.Tool = asString(update.Metadata["tool"])
			command.Command = asString(update.Metadata["command"])
			command.ArgsHash = asString(update.Metadata["args_hash"])
			command.OutputHash = asString(update.Metadata["output_hash"])
			command.ExitCode = asInt(update.Metadata["exit_code"])
		}
		state.Commands[update.ToolCallID] = command
	}
}

func (s *GroundTruthStateStore) DeriveStateUpdates(call GroundedToolCall, result GroundedToolResult) []StateUpdate {
	now := result.FinishedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tool := normalizeGroundToolName(call.Tool)
	switch tool {
	case "write_file":
		return deriveWriteStateUpdates(call, result, now)
	case "read_file":
		return deriveReadStateUpdates(call, result, now)
	case "ls":
		return deriveLSStateUpdates(call, result, now)
	case "exec":
		return deriveExecStateUpdates(call, result, now)
	default:
		return []StateUpdate{{
			UpdateID:   uuid.NewString(),
			ToolCallID: call.ToolCallID,
			Kind:       StateUpdateKindEvidence,
			ObservedAt: now,
			Metadata: map[string]any{
				"tool": tool,
			},
		}}
	}
}

func deriveWriteStateUpdates(call GroundedToolCall, result GroundedToolResult, now time.Time) []StateUpdate {
	path := normalizeGroundPath(pathFromResult(result.Result, call.Args))
	if path == "" {
		return nil
	}
	update := StateUpdate{
		UpdateID:   uuid.NewString(),
		ToolCallID: call.ToolCallID,
		Kind:       StateUpdateKindFSMutation,
		Path:       path,
		Exists:     result.OK,
		ObservedAt: now,
	}
	if size, ok := int64FromAny(extractField(result.Result, "size")); ok {
		update.Size = size
	}
	appendMode := asBool(call.Args["append"])
	lineMode := call.Args["line"] != nil
	if !appendMode && !lineMode {
		if content, ok := call.Args["content"].(string); ok {
			update.ContentSHA256 = hashString(content)
		}
	}
	return []StateUpdate{update}
}

func deriveReadStateUpdates(call GroundedToolCall, result GroundedToolResult, now time.Time) []StateUpdate {
	path := normalizeGroundPath(pathFromResult(result.Result, call.Args))
	if path == "" {
		return nil
	}
	update := StateUpdate{
		UpdateID:   uuid.NewString(),
		ToolCallID: call.ToolCallID,
		Kind:       StateUpdateKindFSObservation,
		Path:       path,
		Exists:     result.OK,
		ObservedAt: now,
	}
	if size, ok := int64FromAny(extractField(result.Result, "size")); ok {
		update.Size = size
	}
	content := asString(extractField(result.Result, "content"))
	truncated := asBool(extractField(result.Result, "truncated"))
	if content != "" && !truncated {
		update.ContentSHA256 = hashString(content)
	}
	return []StateUpdate{update}
}

func deriveLSStateUpdates(call GroundedToolCall, result GroundedToolResult, now time.Time) []StateUpdate {
	basePath := normalizeGroundPath(asString(extractField(result.Result, "base_path")))
	rawEntries, ok := extractField(result.Result, "entries").([]any)
	if !ok {
		return nil
	}
	updates := make([]StateUpdate, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := normalizeGroundPath(joinGroundPaths(basePath, asString(entry["path"])))
		if path == "" {
			continue
		}
		update := StateUpdate{
			UpdateID:   uuid.NewString(),
			ToolCallID: call.ToolCallID,
			Kind:       StateUpdateKindFSObservation,
			Path:       path,
			Exists:     true,
			ObservedAt: now,
		}
		if size, ok := int64FromAny(entry["size"]); ok {
			update.Size = size
		}
		updates = append(updates, update)
	}
	return updates
}

func deriveExecStateUpdates(call GroundedToolCall, result GroundedToolResult, now time.Time) []StateUpdate {
	stdout := asString(extractField(result.Result, "stdout"))
	stderr := asString(extractField(result.Result, "stderr"))
	command := asString(call.Args["command"])
	return []StateUpdate{{
		UpdateID:   uuid.NewString(),
		ToolCallID: call.ToolCallID,
		Kind:       StateUpdateKindCommand,
		ObservedAt: now,
		Metadata: map[string]any{
			"tool":        "exec",
			"command":     command,
			"args_hash":   hashAny(call.Args),
			"exit_code":   result.ExitCode,
			"output_hash": hashString(stdout + "\n" + stderr),
		},
	}}
}

func EvaluateAssertions(state *GroundTruthState, assertions []PlannerAssertion) []string {
	if len(assertions) == 0 {
		return nil
	}
	var failures []string
	for _, assertion := range assertions {
		switch strings.ToLower(strings.TrimSpace(assertion.Type)) {
		case "file_exists":
			path := normalizeGroundPath(assertion.Path)
			if path == "" {
				failures = append(failures, "ASSERT file_exists(path) missing path")
				continue
			}
			fact, ok := state.Files[path]
			if !ok || !fact.Exists {
				failures = append(failures, fmt.Sprintf("ASSERT file_exists(%q) failed", path))
			}
		case "tool_called":
			tool := normalizeGroundToolName(assertion.Tool)
			if tool == "" {
				failures = append(failures, "ASSERT tool_called(name) missing tool")
				continue
			}
			found := false
			for _, call := range state.Calls {
				if normalizeGroundToolName(call.Tool) == tool {
					found = true
					break
				}
			}
			if !found {
				failures = append(failures, fmt.Sprintf("ASSERT tool_called(%q) failed", tool))
			}
		default:
			failures = append(failures, fmt.Sprintf("unsupported ASSERT type %q", assertion.Type))
		}
	}
	return failures
}

func BuildGroundStateSummary(state *GroundTruthState) string {
	if state == nil {
		return "No grounded state is available yet."
	}
	state.ensureMaps()
	var fileLines []string
	for path, fact := range state.Files {
		status := "missing"
		if fact.Exists {
			status = "exists"
		}
		fileLines = append(fileLines, fmt.Sprintf("- %s: %s size=%d", path, status, fact.Size))
		if len(fileLines) >= 12 {
			break
		}
	}
	var commandLines []string
	for _, command := range state.Commands {
		commandLines = append(commandLines, fmt.Sprintf("- %s exit_code=%d", command.Command, command.ExitCode))
		if len(commandLines) >= 6 {
			break
		}
	}
	var sb strings.Builder
	if len(fileLines) == 0 {
		sb.WriteString("Files:\n- none\n")
	} else {
		sb.WriteString("Files:\n")
		sb.WriteString(strings.Join(fileLines, "\n"))
		sb.WriteByte('\n')
	}
	if len(commandLines) == 0 {
		sb.WriteString("Commands:\n- none")
	} else {
		sb.WriteString("Commands:\n")
		sb.WriteString(strings.Join(commandLines, "\n"))
	}
	return strings.TrimSpace(sb.String())
}

func hashAny(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalizeGroundPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return strings.TrimSpace(path)
}

func normalizeGroundToolName(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "read", "read_file", "file_read":
		return "file_read"
	case "write", "write_file", "file_write":
		return "file_write"
	default:
		return strings.TrimSpace(strings.ToLower(name))
	}
}

func joinGroundPaths(basePath, relPath string) string {
	if relPath == "" {
		return basePath
	}
	if basePath == "" || basePath == "." {
		return relPath
	}
	return strings.TrimPrefix(strings.TrimSuffix(basePath, "/")+"/"+strings.TrimPrefix(relPath, "/"), "./")
}

func pathFromResult(result any, args map[string]any) string {
	if path := asString(extractField(result, "path")); path != "" {
		return path
	}
	for _, key := range []string{"path", "file_path", "filePath", "filename"} {
		if path := asString(args[key]); path != "" {
			return path
		}
	}
	return ""
}

func extractField(value any, key string) any {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return obj[key]
}

func int64FromAny(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		i, err := v.Int64()
		return i, err == nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		i, err := strconv.ParseInt(v, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(tools.SafeToolPayloadString(v, 8*1024))
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && parsed
	default:
		return false
	}
}

func asInt(value any) int {
	if v, ok := int64FromAny(value); ok {
		return int(v)
	}
	return 0
}
