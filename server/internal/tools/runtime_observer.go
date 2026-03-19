package tools

// RuntimeEventObserver receives normalized runtime lifecycle notifications.
// Implementations may ignore events that do not belong to a tracked run.
type RuntimeEventObserver interface {
	OnToolRequested(event ToolRuntimeEvent)
	OnToolFinished(event ToolRuntimeEvent)
	OnApprovalRequested(event ApprovalRuntimeEvent)
	OnApprovalResolved(event ApprovalRuntimeEvent)
	OnQuestionRequested(event QuestionRuntimeEvent)
	OnQuestionResolved(event QuestionRuntimeEvent)
}

type ToolRuntimeEvent struct {
	RunID          string                 `json:"run_id,omitempty"`
	StepIndex      int                    `json:"step_index,omitempty"`
	ToolCallID     string                 `json:"tool_call_id,omitempty"`
	ToolName       string                 `json:"tool_name"`
	CapabilityKind string                 `json:"capability_kind,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	RouteKind      ToolRouteKind          `json:"route_kind,omitempty"`
	UserID         string                 `json:"user_id,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	Model          string                 `json:"model,omitempty"`
	AgentID        string                 `json:"agent_id,omitempty"`
	Arguments      map[string]interface{} `json:"arguments,omitempty"`
	Result         interface{}            `json:"result,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

type ApprovalRuntimeEvent struct {
	RunID        string `json:"run_id,omitempty"`
	StepIndex    int    `json:"step_index,omitempty"`
	Kind         string `json:"kind,omitempty"`
	ID           string `json:"id,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	ToolCallID   string `json:"tool_call_id,omitempty"`
	Command      string `json:"command,omitempty"`
	Directory    string `json:"directory,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	PolicySource string `json:"policy_source,omitempty"`
	RiskLevel    string `json:"risk_level,omitempty"`
	BindingHash  string `json:"binding_hash,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	Decision     string `json:"decision,omitempty"`
	Error        string `json:"error,omitempty"`
}

type QuestionRuntimeEvent struct {
	RunID     string                 `json:"run_id,omitempty"`
	StepIndex int                    `json:"step_index,omitempty"`
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Questions []QuestionItem         `json:"questions,omitempty"`
	Answers   []QuestionAnswerResult `json:"answers,omitempty"`
	ExpiresAt int64                  `json:"expires_at,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Silent    bool                   `json:"silent,omitempty"`
	TimedOut  bool                   `json:"timed_out,omitempty"`
	Error     string                 `json:"error,omitempty"`
}
