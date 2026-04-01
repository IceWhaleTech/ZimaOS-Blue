package bootstrap

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func TestWorkflowToolRuntimeAdapter_ExecuteReturnsErrorWithoutGateway(t *testing.T) {
	adapter := workflowToolRuntimeAdapter{}

	result, err := adapter.Execute(context.Background(), workflow.ToolExecutionRequest{ToolName: "search"})
	if err == nil {
		t.Fatal("expected missing gateway error")
	}
	if result != nil {
		t.Fatalf("result=%#v, want nil", result)
	}
}

func TestWorkflowToolRuntimeAdapter_ExecuteMapsGatewayResult(t *testing.T) {
	registry := tools.NewRegistry()
	tool := &stubRuntimeBindingTool{
		def: tools.ToolDefinition{
			Name:        "search",
			Description: "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"query"},
				"additionalProperties": true,
			},
		},
		result: map[string]interface{}{"ok": true},
	}
	registry.Register(tool)
	approver := &stubToolApprover{
		decision: tools.ToolApprovalDecision{
			Allowed:  true,
			Approval: tools.ToolApprovalEnvelope{Mode: "auto"},
		},
	}

	adapter := workflowToolRuntimeAdapter{gateway: newRuntimeToolGateway(registry, approver, nil, nil)}
	result, err := adapter.Execute(context.Background(), workflow.ToolExecutionRequest{
		ToolName:  "search",
		Arguments: map[string]interface{}{"query": "latest"},
		SessionID: "session-1",
		UserID:    "user-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected workflow result")
	}
	if tool.calls != 1 || approver.calls != 1 {
		t.Fatalf("expected tool gateway execution, tool=%#v approver=%#v", tool, approver)
	}
	if result.ExecutionResult == nil || result.CompactPayload == nil || result.AuditPayload == nil {
		t.Fatalf("expected workflow result payloads to be populated, got %#v", result)
	}
}

func TestWorkflowMetricsRecorderAdapter_PassesThrough(t *testing.T) {
	recorder := &stubMetricsRecorder{}
	adapter := workflowMetricsRecorderAdapter{recorder: recorder}

	adapter.RecordCounter("workflow_tool_runs_total", 1, map[string]string{"route": "workflow"})
	if !recorder.hasCall("workflow_tool_runs_total") {
		t.Fatalf("expected metric passthrough, got %#v", recorder.calls)
	}
}
