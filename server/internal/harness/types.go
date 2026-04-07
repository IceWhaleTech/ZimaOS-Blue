package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
)

type RunKind string

const (
	RunKindAgentTask RunKind = "agent_task"
	RunKindResearch  RunKind = "research"
	RunKindSubagent  RunKind = "subagent"
	RunKindWorkflow  RunKind = "workflow"
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
	ID             string                 `json:"id"`
	RootRunID      string                 `json:"root_run_id"`
	ParentRunID    string                 `json:"parent_run_id,omitempty"`
	GroupID        string                 `json:"group_id,omitempty"`
	GroupItemID    string                 `json:"group_item_id,omitempty"`
	AttemptIndex   int                    `json:"attempt_index,omitempty"`
	Kind           RunKind                `json:"kind"`
	Status         RunStatus              `json:"status"`
	RuntimeState   agentpkg.RuntimeState  `json:"runtime_state,omitempty"`
	UserID         string                 `json:"user_id,omitempty"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	AgentID        string                 `json:"agent_id,omitempty"`
	Goal           string                 `json:"goal"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	Model          string                 `json:"model,omitempty"`
	Result         string                 `json:"result,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Depth          int                    `json:"depth"`
	CurrentStep    int                    `json:"current_step"`
	Progress       int                    `json:"progress"`
	WorkspaceRoot  string                 `json:"workspace_root,omitempty"`
	ArtifactRoot   string                 `json:"artifact_root,omitempty"`
	SandboxMode    string                 `json:"sandbox_mode,omitempty"`
	ApprovalMode   ApprovalMode           `json:"approval_mode,omitempty"`
	MaxDuration    time.Duration          `json:"max_duration,omitempty"`
	MaxSteps       int                    `json:"max_steps,omitempty"`
	MaxToolRounds  int                    `json:"max_tool_rounds,omitempty"`
	MaxSubagents   int                    `json:"max_subagents,omitempty"`
	MaxDepth       int                    `json:"max_depth,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	FinishedAt     *time.Time             `json:"finished_at,omitempty"`
}

type RunSpec struct {
	Kind           RunKind                `json:"kind"`
	Goal           string                 `json:"goal"`
	UserID         string                 `json:"user_id,omitempty"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	ParentRunID    string                 `json:"parent_run_id,omitempty"`
	GroupID        string                 `json:"group_id,omitempty"`
	GroupItemID    string                 `json:"group_item_id,omitempty"`
	AttemptIndex   int                    `json:"attempt_index,omitempty"`
	AgentID        string                 `json:"agent_id,omitempty"`
	ProviderID     string                 `json:"provider_id,omitempty"`
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
	UserID         string
	Kind           RunKind
	Kinds          []RunKind
	Statuses       []RunStatus
	ConversationID string
	GroupID        string
	GroupItemID    string
	ParentRunID    string
	RootRunID      string
	Limit          int
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
	ID           string `json:"id"`
	RunID        string `json:"run_id"`
	Kind         string `json:"kind"`
	Label        string `json:"label,omitempty"`
	PathOrURL    string `json:"path_or_url,omitempty"`
	MIMEType     string `json:"mime_type,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	MetadataJSON string `json:"metadata_json,omitempty"`
}

type RuntimeEvidenceEntry struct {
	ID           string    `json:"id"`
	RunID        string    `json:"run_id"`
	StepIndex    int       `json:"step_index,omitempty"`
	PlannerRound int       `json:"planner_round,omitempty"`
	EventType    string    `json:"event_type"`
	Summary      string    `json:"summary,omitempty"`
	PayloadJSON  string    `json:"payload_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type RuntimeEvidenceProvider interface {
	ListRuntimeEvidence(ctx context.Context, run *Run) ([]RuntimeEvidenceEntry, error)
}

type CheckpointArtifact struct {
	RunID       string                 `json:"run_id"`
	GroupItemID string                 `json:"group_item_id,omitempty"`
	Artifact    ArtifactRef            `json:"artifact"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

type Manager interface {
	Submit(ctx context.Context, spec RunSpec) (*Run, error)
	Get(ctx context.Context, id string) (*Run, error)
	List(ctx context.Context, filter RunFilter) ([]Run, error)
	Cancel(ctx context.Context, id string, reason string) error
	PerformAction(ctx context.Context, id string, action string, input map[string]interface{}) (*Run, error)
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
	Manager    *Controller
	RunContext *RunContext
}

type SnapshotDriver interface {
	Sync(ctx context.Context, run *Run) (*Run, error)
}

type ActionDriver interface {
	PerformAction(ctx context.Context, run *Run, action string, input map[string]interface{}) (*Run, error)
}

type RunGroupKind string

const (
	RunGroupKindEval       RunGroupKind = "eval"
	RunGroupKindExperiment RunGroupKind = "experiment"
	RunGroupKindBatch      RunGroupKind = "batch"
)

type RunGroupStatus string

const (
	RunGroupStatusPending   RunGroupStatus = "pending"
	RunGroupStatusQueued    RunGroupStatus = "queued"
	RunGroupStatusRunning   RunGroupStatus = "running"
	RunGroupStatusScoring   RunGroupStatus = "scoring"
	RunGroupStatusCompleted RunGroupStatus = "completed"
	RunGroupStatusPartial   RunGroupStatus = "partial"
	RunGroupStatusFailed    RunGroupStatus = "failed"
	RunGroupStatusCancelled RunGroupStatus = "cancelled"
)

type RunGroupItemStatus string

const (
	RunGroupItemStatusPending   RunGroupItemStatus = "pending"
	RunGroupItemStatusQueued    RunGroupItemStatus = "queued"
	RunGroupItemStatusRunning   RunGroupItemStatus = "running"
	RunGroupItemStatusScoring   RunGroupItemStatus = "scoring"
	RunGroupItemStatusPassed    RunGroupItemStatus = "passed"
	RunGroupItemStatusFailed    RunGroupItemStatus = "failed"
	RunGroupItemStatusError     RunGroupItemStatus = "error"
	RunGroupItemStatusCancelled RunGroupItemStatus = "cancelled"
)

type ScoringMode string

const (
	ScoringModeRule   ScoringMode = "rule"
	ScoringModeJudge  ScoringMode = "judge"
	ScoringModeHybrid ScoringMode = "hybrid"
)

type ScoreVerdict string

const (
	ScoreVerdictPass    ScoreVerdict = "pass"
	ScoreVerdictFail    ScoreVerdict = "fail"
	ScoreVerdictPartial ScoreVerdict = "partial"
	ScoreVerdictError   ScoreVerdict = "error"
)

type GroupSchedulerConfig struct {
	MaxConcurrency int           `json:"max_concurrency,omitempty"`
	MaxAttempts    int           `json:"max_attempts,omitempty"`
	LeaseTTL       time.Duration `json:"lease_ttl,omitempty"`
	RetryBackoff   time.Duration `json:"retry_backoff,omitempty"`
}

type GroupScoringConfig struct {
	Mode          ScoringMode `json:"mode,omitempty"`
	RuleProfile   string      `json:"rule_profile,omitempty"`
	JudgeModel    string      `json:"judge_model,omitempty"`
	PassThreshold float64     `json:"pass_threshold,omitempty"`
}

type RunGroup struct {
	ID              string                 `json:"id"`
	Kind            RunGroupKind           `json:"kind"`
	Title           string                 `json:"title,omitempty"`
	Status          RunGroupStatus         `json:"status"`
	OwnerUserID     string                 `json:"owner_user_id,omitempty"`
	Subject         string                 `json:"subject,omitempty"`
	SchedulerConfig GroupSchedulerConfig   `json:"scheduler_config,omitempty"`
	ScoringConfig   GroupScoringConfig     `json:"scoring_config,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Summary         map[string]interface{} `json:"summary,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	FinishedAt      *time.Time             `json:"finished_at,omitempty"`
}

type RunGroupItem struct {
	ID             string                 `json:"id"`
	GroupID        string                 `json:"group_id"`
	Index          int                    `json:"index"`
	RunKind        RunKind                `json:"run_kind"`
	Profile        string                 `json:"profile,omitempty"`
	Input          map[string]interface{} `json:"input,omitempty"`
	Expected       map[string]interface{} `json:"expected,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Status         RunGroupItemStatus     `json:"status"`
	LatestRunID    string                 `json:"latest_run_id,omitempty"`
	AttemptCount   int                    `json:"attempt_count,omitempty"`
	MaxAttempts    int                    `json:"max_attempts,omitempty"`
	LeaseOwner     string                 `json:"lease_owner,omitempty"`
	LeaseExpiresAt *time.Time             `json:"lease_expires_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type Scorecard struct {
	ID             string       `json:"id"`
	GroupID        string       `json:"group_id"`
	GroupItemID    string       `json:"group_item_id"`
	RunID          string       `json:"run_id,omitempty"`
	Mode           ScoringMode  `json:"mode"`
	Verdict        ScoreVerdict `json:"verdict"`
	Score          float64      `json:"score"`
	BreakdownJSON  string       `json:"breakdown_json,omitempty"`
	EvidenceJSON   string       `json:"evidence_json,omitempty"`
	JudgeTraceJSON string       `json:"judge_trace_json,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

type RunGroupSpec struct {
	Kind            RunGroupKind           `json:"kind"`
	Title           string                 `json:"title,omitempty"`
	Subject         string                 `json:"subject,omitempty"`
	OwnerUserID     string                 `json:"owner_user_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	SchedulerConfig GroupSchedulerConfig   `json:"scheduler"`
	ScoringConfig   GroupScoringConfig     `json:"scoring"`
	Items           []RunGroupItemSpec     `json:"items"`
}

type RunGroupItemSpec struct {
	RunKind  RunKind                `json:"run_kind"`
	Profile  string                 `json:"profile,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Expected map[string]interface{} `json:"expected,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type RunGroupFilter struct {
	OwnerUserID string
	Kinds       []RunGroupKind
	Statuses    []RunGroupStatus
	Limit       int
}

type RunGroupReport struct {
	Group           *RunGroup                         `json:"group"`
	Items           []RunGroupItem                    `json:"items,omitempty"`
	VerdictCounts   map[string]int                    `json:"verdict_counts,omitempty"`
	OverallScore    float64                           `json:"overall_score,omitempty"`
	PassRate        float64                           `json:"pass_rate,omitempty"`
	Breakdown       map[string]interface{}            `json:"breakdown,omitempty"`
	FailedItems     []map[string]interface{}          `json:"failed_items,omitempty"`
	LinkedRuns      []Run                             `json:"linked_runs,omitempty"`
	Artifacts       []ArtifactRef                     `json:"artifacts,omitempty"`
	Scorecards      []Scorecard                       `json:"scorecards,omitempty"`
	RuntimeEvidence map[string][]RuntimeEvidenceEntry `json:"runtime_evidence,omitempty"`
	RuntimeTraces   map[string]RunTrace               `json:"runtime_traces,omitempty"`
	ItemContracts   map[string]HarnessContract        `json:"item_contracts,omitempty"`
	Checkpoints     []CheckpointArtifact              `json:"checkpoints,omitempty"`
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

func cloneRunTraceMap(in map[string]RunTrace) map[string]RunTrace {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]RunTrace, len(in))
	for key, value := range in {
		out[key] = cloneRunTrace(value)
	}
	return out
}

func cloneRunTrace(in RunTrace) RunTrace {
	out := in
	out.StartedAt = cloneRunTimePtr(in.StartedAt)
	out.FinishedAt = cloneRunTimePtr(in.FinishedAt)
	if len(in.Stages) > 0 {
		out.Stages = make([]RunTraceStage, len(in.Stages))
		for i, stage := range in.Stages {
			out.Stages[i] = RunTraceStage{
				Stage:     stage.Stage,
				Message:   stage.Message,
				Status:    stage.Status,
				Details:   cloneMetadataMap(stage.Details),
				CreatedAt: stage.CreatedAt,
			}
		}
	}
	if len(in.Events) > 0 {
		out.Events = append([]RunTraceEvent(nil), in.Events...)
	}
	if len(in.Artifacts) > 0 {
		out.Artifacts = append([]ArtifactRef(nil), in.Artifacts...)
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

func metadataString(meta map[string]interface{}, key string) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[key]
	if !ok {
		return ""
	}
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}
