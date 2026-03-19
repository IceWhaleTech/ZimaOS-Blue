package harness

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
)

type RunKind string

const (
	RunKindAgentTask RunKind = "agent_task"
	RunKindResearch  RunKind = "research"
	RunKindSubagent  RunKind = "subagent"
)

type RunStatus string

const (
	RunStatusPending      RunStatus = "pending"
	RunStatusPlanning     RunStatus = "planning"
	RunStatusWaitingInput RunStatus = "waiting_input"
	RunStatusExecuting    RunStatus = "executing"
	RunStatusVerifying    RunStatus = "verifying"
	RunStatusCompleted    RunStatus = "completed"
	RunStatusFailed       RunStatus = "failed"
	RunStatusCancelled    RunStatus = "cancelled"
	RunStatusAborted      RunStatus = "aborted"
)

type ApprovalMode string

const (
	ApprovalModeAsk   ApprovalMode = "ask"
	ApprovalModeAllow ApprovalMode = "allow"
	ApprovalModeDeny  ApprovalMode = "deny"
)

type Run struct {
	ID             string                   `json:"id"`
	RootRunID      string                   `json:"root_run_id"`
	ParentRunID    string                   `json:"parent_run_id,omitempty"`
	Kind           RunKind                  `json:"kind"`
	Status         RunStatus                `json:"status"`
	RuntimeState   agentpkg.RuntimeState    `json:"runtime_state,omitempty"`
	UserID         string                   `json:"user_id,omitempty"`
	ConversationID string                   `json:"conversation_id,omitempty"`
	SessionID      string                   `json:"session_id,omitempty"`
	AgentID        string                   `json:"agent_id,omitempty"`
	Goal           string                   `json:"goal"`
	Model          string                   `json:"model,omitempty"`
	Result         string                   `json:"result,omitempty"`
	Error          string                   `json:"error,omitempty"`
	Depth          int                      `json:"depth"`
	CurrentStep    int                      `json:"current_step"`
	Progress       int                      `json:"progress"`
	WorkspaceRoot  string                   `json:"workspace_root,omitempty"`
	ArtifactRoot   string                   `json:"artifact_root,omitempty"`
	SandboxMode    string                   `json:"sandbox_mode,omitempty"`
	ApprovalMode   ApprovalMode             `json:"approval_mode,omitempty"`
	MaxDuration    time.Duration            `json:"max_duration,omitempty"`
	MaxSteps       int                      `json:"max_steps,omitempty"`
	MaxToolRounds  int                      `json:"max_tool_rounds,omitempty"`
	MaxSubagents   int                      `json:"max_subagents,omitempty"`
	MaxDepth       int                      `json:"max_depth,omitempty"`
	Metadata       map[string]interface{}   `json:"metadata,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	StartedAt      *time.Time               `json:"started_at,omitempty"`
	FinishedAt     *time.Time               `json:"finished_at,omitempty"`
}

type RunSpec struct {
	Kind           RunKind                `json:"kind"`
	Goal           string                 `json:"goal"`
	UserID         string                 `json:"user_id,omitempty"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	ParentRunID    string                 `json:"parent_run_id,omitempty"`
	AgentID        string                 `json:"agent_id,omitempty"`
	Model          string                 `json:"model,omitempty"`
	WorkspaceRoot  string                 `json:"workspace_root,omitempty"`
	SandboxMode    string                 `json:"sandbox_mode,omitempty"`
	ApprovalMode   ApprovalMode           `json:"approval_mode,omitempty"`
	MaxDuration    time.Duration          `json:"max_duration,omitempty"`
	MaxSteps       int                    `json:"max_steps,omitempty"`
	MaxToolRounds  int                    `json:"max_tool_rounds,omitempty"`
	MaxSubagents   int                    `json:"max_subagents,omitempty"`
	MaxDepth       int                    `json:"max_depth,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type RunFilter struct {
	UserID      string
	Kind        RunKind
	Kinds       []RunKind
	Statuses    []RunStatus
	ParentRunID string
	RootRunID   string
	Limit       int
}

type RunEvent struct {
	ID             string    `json:"id"`
	RunID          string    `json:"run_id"`
	RootRunID      string    `json:"root_run_id,omitempty"`
	ParentRunID    string    `json:"parent_run_id,omitempty"`
	Type           string    `json:"type"`
	StepIndex      int       `json:"step_index,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	CapabilityKind string    `json:"capability_kind,omitempty"`
	Message        string    `json:"message,omitempty"`
	PayloadJSON    string    `json:"payload_json,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ArtifactRef struct {
	ID          string `json:"id"`
	RunID       string `json:"run_id"`
	Kind        string `json:"kind"`
	Label       string `json:"label,omitempty"`
	PathOrURL   string `json:"path_or_url,omitempty"`
	MIMEType    string `json:"mime_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	MetadataJSON string `json:"metadata_json,omitempty"`
}

type Manager interface {
	Submit(ctx context.Context, spec RunSpec) (*Run, error)
	Get(ctx context.Context, id string) (*Run, error)
	List(ctx context.Context, filter RunFilter) ([]Run, error)
	Cancel(ctx context.Context, id string, reason string) error
	SpawnChild(ctx context.Context, parentID string, spec RunSpec) (*Run, error)
	AppendEvent(ctx context.Context, event RunEvent) error
	AttachArtifact(ctx context.Context, ref ArtifactRef) error
}

type Driver interface {
	Kind() RunKind
	Validate(spec RunSpec) error
	Start(ctx context.Context, run *Run, env RunEnv) error
	Cancel(ctx context.Context, run *Run) error
}

type Store interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run *Run) error
	GetRun(ctx context.Context, id string) (*Run, error)
	ListRuns(ctx context.Context, filter RunFilter) ([]Run, error)
	AppendEvent(ctx context.Context, event RunEvent) error
	ListEvents(ctx context.Context, runID string, limit int) ([]RunEvent, error)
	AttachArtifact(ctx context.Context, ref ArtifactRef) error
	ListArtifacts(ctx context.Context, runID string) ([]ArtifactRef, error)
}

type RunEnv struct {
	Manager *Controller
}

type SnapshotDriver interface {
	Sync(ctx context.Context, run *Run) (*Run, error)
}

func cloneMetadataMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func marshalMetadata(in map[string]interface{}) string {
	if len(in) == 0 {
		return "{}"
	}
	raw, _ := json.Marshal(in)
	return string(raw)
}

func unmarshalMetadata(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
