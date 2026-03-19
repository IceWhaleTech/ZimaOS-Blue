package workflow

import "context"

// ToolRuntime is the workflow-side abstraction for shared tool execution.
type ToolRuntime interface {
	Execute(ctx context.Context, req ToolExecutionRequest) (*ToolExecutionResult, error)
}

// ToolExecutionRequest is the workflow-local representation of a tool call.
type ToolExecutionRequest struct {
	ToolName   string
	Arguments  map[string]interface{}
	SessionID  string
	UserID     string
	Provider   string
	ProviderID string
	Model      string
	AgentID    string
}

// ToolExecutionResult is the workflow-local result envelope returned by ToolRuntime.
type ToolExecutionResult struct {
	ExecutionResult interface{}
	CompactPayload  interface{}
	AuditPayload    interface{}
}
