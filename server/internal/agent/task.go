// Package agent provides autonomous task execution for Blue.
package agent

import (
	"encoding/json"
	"strings"
	"time"
)

// TaskStatus represents the status of an agent task.
type TaskStatus string

const (
	TaskStatusPending      TaskStatus = "pending"
	TaskStatusPlanning     TaskStatus = "planning"
	TaskStatusExecuting    TaskStatus = "executing"
	TaskStatusWaitingInput TaskStatus = "waiting_input" // blocked on ask_user
	TaskStatusAborted      TaskStatus = "aborted"
	TaskStatusCompleted    TaskStatus = "completed"
	TaskStatusFailed       TaskStatus = "failed"
	TaskStatusCancelled    TaskStatus = "cancelled"
)

// StepStatus represents the status of a plan step.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// Task represents an autonomous agent task.
type Task struct {
	ID                 string                 `json:"id"`
	UserID             string                 `json:"user_id"`
	ConversationID     string                 `json:"conversation_id,omitempty"`
	Goal               string                 `json:"goal"`
	Plan               []PlanStep             `json:"plan,omitempty"`
	Status             TaskStatus             `json:"status"`
	RuntimeState       RuntimeState           `json:"runtime_state,omitempty"`
	RuntimeAudit       []RuntimeAuditEvent    `json:"runtime_audit,omitempty"`
	SuccessCriteria    []string               `json:"success_criteria,omitempty"`
	FallbackPlan       []string               `json:"fallback_plan,omitempty"`
	CurrentStep        int                    `json:"current_step"`
	Progress           int                    `json:"progress"` // 0-100
	Result             string                 `json:"result,omitempty"`
	VerifiedOutput     string                 `json:"verified_output,omitempty"`
	VerificationErrors []string               `json:"verification_errors,omitempty"`
	GroundingStatus    string                 `json:"grounding_status,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	Error              string                 `json:"error,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	WorkspaceRoot      string                 `json:"-"`

	GroundState *GroundTruthState `json:"-"`
}

func preferredTaskModel(task *Task) string {
	if task == nil {
		return "auto"
	}
	for _, source := range []map[string]interface{}{
		task.Metadata,
		metadataMapValue(task.Metadata, "group_input"),
	} {
		if model := metadataStringValue(source, "model"); model != "" {
			return model
		}
	}
	for _, source := range []map[string]interface{}{
		task.Metadata,
		metadataMapValue(task.Metadata, "group_input"),
	} {
		if model := metadataStringValue(source, "policy_model_hint"); model != "" {
			return model
		}
		if model := metadataStringValue(source, "policyModelHint"); model != "" {
			return model
		}
	}
	return "auto"
}

// PlanStep represents a single step in the agent's plan.
type PlanStep struct {
	Index       int        `json:"index"`
	Description string     `json:"description"`
	Status      StepStatus `json:"status"`
	Output      string     `json:"output,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// CreateTaskRequest is the API request to create a new agent task.
type CreateTaskRequest struct {
	Goal           string `json:"goal"`
	ConversationID string `json:"conversation_id,omitempty"`
	Context        string `json:"context,omitempty"` // recent conversation context for the agent
}

// TaskEvent is an SSE event for task progress.
type TaskEvent struct {
	TaskID     string          `json:"task_id"`
	EventType  string          `json:"event_type"` // task_created, task_planning, task_progress, task_step_completed, task_reflection_started, task_reflection_completed, task_completed, task_failed, task_question
	StepIndex  int             `json:"step_index,omitempty"`
	Progress   int             `json:"progress,omitempty"`
	Message    string          `json:"message,omitempty"`
	Output     string          `json:"output,omitempty"`
	DurationMs int64           `json:"duration_ms,omitempty"` // step execution time in milliseconds
	Questions  []AgentQuestion `json:"questions,omitempty"`   // for task_question events
	FromState  RuntimeState    `json:"from_state,omitempty"`
	ToState    RuntimeState    `json:"to_state,omitempty"`
}

// AgentQuestion is a question the agent asks the user during execution.
type AgentQuestion struct {
	ID          string           `json:"id"`
	Question    string           `json:"question"`
	Detail      string           `json:"detail,omitempty"`
	Header      string           `json:"header"` // short tab label (max 12 chars)
	Type        string           `json:"type,omitempty"`
	Options     []QuestionOption `json:"options,omitempty"`
	MultiSelect bool             `json:"multi_select,omitempty"` // true = checkboxes, false = radio
	Required    bool             `json:"required,omitempty"`
}

// QuestionOption is a selectable option for a question.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value"`
}

// QuestionAnswer is the user's answer to a question.
type QuestionAnswer struct {
	QuestionID string   `json:"question_id"`
	Values     []string `json:"values"` // selected option values or free text
	OtherText  string   `json:"other_text,omitempty"`
}

// MarshalPlan serializes plan steps to JSON for storage.
func MarshalPlan(steps []PlanStep) string {
	b, _ := json.Marshal(steps)
	return string(b)
}

// UnmarshalPlan deserializes plan steps from JSON.
func UnmarshalPlan(data string) []PlanStep {
	if data == "" {
		return nil
	}
	var steps []PlanStep
	_ = json.Unmarshal([]byte(data), &steps)
	return steps
}

func marshalAudit(events []RuntimeAuditEvent) string {
	b, _ := json.Marshal(events)
	return string(b)
}

func unmarshalAudit(data string) []RuntimeAuditEvent {
	if data == "" {
		return nil
	}
	var events []RuntimeAuditEvent
	_ = json.Unmarshal([]byte(data), &events)
	return events
}

func marshalStringSlice(values []string) string {
	b, _ := json.Marshal(values)
	return string(b)
}

func unmarshalStringSlice(data string) []string {
	if data == "" {
		return nil
	}
	var out []string
	_ = json.Unmarshal([]byte(data), &out)
	return out
}

func marshalGroundTruthState(state *GroundTruthState) string {
	if state == nil {
		return ""
	}
	b, _ := json.Marshal(state)
	return string(b)
}

func marshalMetadataMap(values map[string]interface{}) string {
	if len(values) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(values)
	return string(b)
}

func unmarshalMetadataMap(data string) map[string]interface{} {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func unmarshalGroundTruthState(data string) *GroundTruthState {
	if data == "" {
		return nil
	}
	var out GroundTruthState
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return nil
	}
	out.ensureMaps()
	return &out
}
