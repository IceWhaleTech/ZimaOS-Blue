package tools

// ToolApprovalEnvelope captures approval details for a tool run.
type ToolApprovalEnvelope struct {
	Required bool   `json:"required"`
	Mode     string `json:"mode,omitempty"`
	Reason   string `json:"reason,omitempty"`
	ID       string `json:"id,omitempty"`
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
