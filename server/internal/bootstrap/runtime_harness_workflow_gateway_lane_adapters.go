package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func (a workflowMetricsRecorderAdapter) RecordCounter(name string, value int64, tags map[string]string) {
	if a.recorder == nil {
		return
	}
	a.recorder.RecordCounter(name, value, tags)
}

func (a workflowToolRuntimeAdapter) Execute(ctx context.Context, req workflow.ToolExecutionRequest) (*workflow.ToolExecutionResult, error) {
	if a.gateway == nil {
		return nil, fmt.Errorf("workflow tool gateway is not configured")
	}
	argsJSON, _ := json.Marshal(req.Arguments)
	result, err := a.gateway.Execute(tools.WithRouteKind(ctx, tools.ToolRouteKindWorkflow), tools.ToolGatewayRequest{
		ToolName:   strings.TrimSpace(req.ToolName),
		Arguments:  string(argsJSON),
		RouteKind:  tools.ToolRouteKindWorkflow,
		SessionID:  strings.TrimSpace(req.SessionID),
		UserID:     strings.TrimSpace(req.UserID),
		Provider:   strings.TrimSpace(req.Provider),
		ProviderID: strings.TrimSpace(req.ProviderID),
		Model:      strings.TrimSpace(req.Model),
		AgentID:    strings.TrimSpace(req.AgentID),
	})
	if err != nil {
		return nil, err
	}
	return &workflow.ToolExecutionResult{
		ExecutionResult: result.ExecutionResult,
		CompactPayload:  result.CompactLLMPayload,
		AuditPayload:    result.AuditPayload,
	}, nil
}
