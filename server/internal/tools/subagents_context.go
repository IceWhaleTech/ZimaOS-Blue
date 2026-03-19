package tools

import (
	"context"
	"strings"
	"time"
)

const subagentExecutorKey toolContextKey = "tool_subagent_executor"

// SubagentRequest describes a child-agent execution request initiated by the
// subagents tool.
type SubagentRequest struct {
	Goal          string                 `json:"goal"`
	AgentID       string                 `json:"agent_id,omitempty"`
	Model         string                 `json:"model,omitempty"`
	Context       string                 `json:"context,omitempty"`
	Wait          bool                   `json:"wait"`
	MaxDuration   time.Duration          `json:"max_duration,omitempty"`
	MaxSteps      int                    `json:"max_steps,omitempty"`
	MaxToolRounds int                    `json:"max_tool_rounds,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SubagentResult is the normalized child-run payload returned to tools.
type SubagentResult struct {
	RunID       string `json:"run_id"`
	RootRunID   string `json:"root_run_id,omitempty"`
	ParentRunID string `json:"parent_run_id,omitempty"`
	Status      string `json:"status"`
	Goal        string `json:"goal,omitempty"`
	Result      string `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	Model       string `json:"model,omitempty"`
	Depth       int    `json:"depth,omitempty"`
	Waited      bool   `json:"waited,omitempty"`
	Terminal    bool   `json:"terminal,omitempty"`
	Completed   bool   `json:"completed,omitempty"`
}

// SubagentExecutor executes harness-backed child runs for the subagents tool.
type SubagentExecutor interface {
	ExecuteSubagent(ctx context.Context, req SubagentRequest) (*SubagentResult, error)
}

// WithSubagentExecutor returns a context carrying a child-run executor.
func WithSubagentExecutor(ctx context.Context, exec SubagentExecutor) context.Context {
	if exec == nil {
		return ctx
	}
	return context.WithValue(ctx, subagentExecutorKey, exec)
}

// GetSubagentExecutor extracts the child-run executor from the context.
func GetSubagentExecutor(ctx context.Context) SubagentExecutor {
	if ctx == nil {
		return nil
	}
	exec, _ := ctx.Value(subagentExecutorKey).(SubagentExecutor)
	return exec
}

func normalizeSubagentRequest(req SubagentRequest) SubagentRequest {
	req.Goal = strings.TrimSpace(req.Goal)
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Model = strings.TrimSpace(req.Model)
	req.Context = strings.TrimSpace(req.Context)
	if len(req.Metadata) == 0 {
		req.Metadata = nil
	}
	return req
}
