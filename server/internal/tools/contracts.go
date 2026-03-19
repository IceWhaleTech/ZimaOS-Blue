package tools

import "context"

// ToolApprovalEnvelope captures approval details for a tool run.
type ToolApprovalEnvelope struct {
	Required     bool   `json:"required"`
	Mode         string `json:"mode,omitempty"`
	Reason       string `json:"reason,omitempty"`
	ID           string `json:"id,omitempty"`
	BindingHash  string `json:"binding_hash,omitempty"`
	PolicySource string `json:"policy_source,omitempty"`
	RiskLevel    string `json:"risk_level,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

// ToolCostEnvelope captures normalized token / cost accounting.
type ToolCostEnvelope struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	USD          float64 `json:"usd,omitempty"`
}

// ToolResultEnvelope is the normalized response shape for future native tools.
type ToolResultEnvelope struct {
	Status   string                `json:"status"`
	TraceID  string                `json:"trace_id,omitempty"`
	AuditID  string                `json:"audit_id,omitempty"`
	Warnings []string              `json:"warnings,omitempty"`
	Approval *ToolApprovalEnvelope `json:"approval,omitempty"`
	Cost     *ToolCostEnvelope     `json:"cost,omitempty"`
	Data     interface{}           `json:"data,omitempty"`
}

// ToolApprovalRequest carries runtime context for approval policy evaluation.
type ToolApprovalRequest struct {
	ToolName     string                 `json:"tool_name"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
	Arguments    map[string]interface{} `json:"arguments,omitempty"`
	Provider     string                 `json:"provider,omitempty"`
	ProviderID   string                 `json:"provider_id,omitempty"`
	Model        string                 `json:"model,omitempty"`
	AgentID      string                 `json:"agent_id,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	RouteKind    ToolRouteKind          `json:"route_kind,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	RiskLevel    string                 `json:"risk_level,omitempty"`
	PolicySource string                 `json:"policy_source,omitempty"`
	BindingHash  string                 `json:"binding_hash,omitempty"`
}

// ToolApprovalDecision is the normalized response from an approval backend.
type ToolApprovalDecision struct {
	Allowed  bool                 `json:"allowed"`
	Approval ToolApprovalEnvelope `json:"approval"`
}

// ToolApprover decides whether a tool call may proceed at runtime.
type ToolApprover interface {
	AuthorizeToolCall(ctx context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error)
}
