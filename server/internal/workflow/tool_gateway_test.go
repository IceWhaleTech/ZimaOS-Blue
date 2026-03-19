package workflow

import (
	"context"
	"testing"
)

type recursiveWorkflowPayload struct {
	Name string                    `json:"name"`
	Self *recursiveWorkflowPayload `json:"self,omitempty"`
}

type workflowRuntimeMock struct {
	lastReq ToolExecutionRequest
	result  *ToolExecutionResult
	err     error
}

func (m *workflowRuntimeMock) Execute(_ context.Context, req ToolExecutionRequest) (*ToolExecutionResult, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &ToolExecutionResult{
		ExecutionResult: map[string]interface{}{"ok": true},
	}, nil
}

func TestEngineBrowserActionUsesToolGateway(t *testing.T) {
	runtime := &workflowRuntimeMock{}

	engine := NewEngine(nil)
	defer engine.Close()
	engine.SetToolGateway(runtime)

	node := &Node{
		ID:   "action-1",
		Type: NodeTypeAction,
		Name: "Browser",
		Config: map[string]interface{}{
			"type":   string(ActionTypeBrowser),
			"action": "navigate",
			"url":    "https://example.com",
		},
	}

	output, err := engine.executeActionNode(context.Background(), node, nil, map[string]interface{}{})
	if err != nil {
		t.Fatalf("executeActionNode() error = %v", err)
	}
	if runtime.lastReq.ToolName != "browser" {
		t.Fatalf("tool name = %q, want browser", runtime.lastReq.ToolName)
	}
	if runtime.lastReq.Arguments["action"] != "navigate" {
		t.Fatalf("browser action = %v, want navigate", runtime.lastReq.Arguments["action"])
	}
	if output["ok"] != true {
		t.Fatalf("output = %+v, want ok=true", output)
	}
}

func TestEngineSkillActionUsesToolGateway(t *testing.T) {
	runtime := &workflowRuntimeMock{}

	engine := NewEngine(nil)
	defer engine.Close()
	engine.SetToolGateway(runtime)

	node := &Node{
		ID:   "action-2",
		Type: NodeTypeAction,
		Name: "Skill",
		Config: map[string]interface{}{
			"type":     string(ActionTypeSkill),
			"skill_id": "translate_skill",
			"parameters": map[string]interface{}{
				"text": "hello",
			},
		},
	}

	_, err := engine.executeActionNode(context.Background(), node, nil, map[string]interface{}{})
	if err != nil {
		t.Fatalf("executeActionNode() error = %v", err)
	}
	if runtime.lastReq.ToolName != "translate_skill" {
		t.Fatalf("tool name = %q, want translate_skill", runtime.lastReq.ToolName)
	}
	if runtime.lastReq.Arguments["text"] != "hello" {
		t.Fatalf("skill args = %+v, want text=hello", runtime.lastReq.Arguments)
	}
}

func TestWorkflowToolOutputMapGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveWorkflowPayload{Name: "root"}
	payload.Self = payload

	out, err := workflowToolOutputMap(&ToolExecutionResult{ExecutionResult: payload})
	if err != nil {
		t.Fatalf("workflowToolOutputMap() error = %v", err)
	}
	got, ok := out["result"].(string)
	if !ok || got == "" {
		t.Fatalf("result = %#v, want non-empty cycle error", out["result"])
	}
	if got != "json: unsupported value: encountered a cycle via *workflow.recursiveWorkflowPayload" {
		t.Fatalf("result = %q, want cycle error", got)
	}
}
