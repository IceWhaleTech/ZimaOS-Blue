package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestGroundedExecutorPreservesExecSessionCalls(t *testing.T) {
	registry := tools.NewRegistry()
	sessions := tools.NewSessionRegistry()
	defer sessions.Cleanup()
	native := tools.NewExecTool(tools.DefaultExecConfig(), sessions, nil, nil, nil)
	registry.Register(native)
	registry.Register(tools.NewPublicBashTool(registry))
	registry.ExposeDefinition(native.Definition())
	registry.Disable("exec")
	executor := tools.NewExecutor(registry)
	grounded := NewGroundedExecutor(executor, registry, NewGroundTruthStateStore(), nil, nil)
	grounded.SetToolGateway(tools.NewToolGateway(registry, executor))
	task := &Task{ID: "session-evidence", GroundState: NewGroundTruthState()}
	first, err := grounded.Execute(context.Background(), task, 0, 0, PlannerToolCall{Tool: "bash", Args: map[string]any{"command": `printf 'actual result\n'`}})
	if err != nil {
		t.Fatal(err)
	}
	session := asString(extractField(first.Result.Result, "session_id"))
	if session == "" {
		t.Fatal("missing recovery session")
	}
	task.GroundState.Calls[first.Call.ToolCallID] = first.Call
	logged, err := grounded.Execute(context.Background(), task, 0, 1, PlannerToolCall{Tool: "exec", Args: map[string]any{"action": "log", "session_id": session, "offset": 0, "limit": 4096}})
	if err != nil || !logged.Result.OK {
		t.Fatalf("exec log was routed into command-only bash: %v, %#v", err, logged.Result)
	}
	if !strings.Contains(serializedResult(logged.Result.Result), "actual result") {
		t.Fatalf("log lost result: %#v", logged.Result)
	}
}
