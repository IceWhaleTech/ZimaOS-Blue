package tools

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

type stubSubagentExecutor struct {
	req    SubagentRequest
	result *SubagentResult
	err    error
}

func (s *stubSubagentExecutor) ExecuteSubagent(_ context.Context, req SubagentRequest) (*SubagentResult, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &SubagentResult{RunID: "child-1", Status: "completed", Completed: true, Terminal: true}, nil
}

func TestAgentsListToolExecute(t *testing.T) {
	cfg := &config.Config{Agents: *config.DefaultAgentsConfig()}
	tool := NewAgentsListTool(cfg)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"active_only": true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if payload["count"].(int) < 1 {
		t.Fatalf("expected at least one agent, got %#v", payload)
	}
}

func TestSessionsListToolExecute(t *testing.T) {
	tool := NewSessionsListTool(&stubSessionsService{sessions: []SessionSummary{{ID: "conv_1", Title: "First", Pinned: true}, {ID: "conv_2", Title: "Second"}}})
	result, err := tool.Execute(context.Background(), map[string]interface{}{"pinned_only": true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	sessions, ok := payload["sessions"].([]SessionSummary)
	if !ok {
		t.Fatalf("expected []SessionSummary payload, got %T", payload["sessions"])
	}
	if len(sessions) != 1 || sessions[0].ID != "conv_1" {
		t.Fatalf("unexpected filtered sessions: %#v", sessions)
	}
}

func TestSubagentsToolExecute(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentConfig{
				Enabled:  true,
				Thinking: "medium",
				ToolPolicy: config.ToolPolicyConfig{
					Profile: "coding",
				},
				Subagents: config.AgentSubagentPolicyConfig{
					Enabled:      true,
					MaxParallel:  4,
					MaxDepth:     2,
					Timeout:      time.Minute,
					CallbackMode: "summary_only",
				},
			},
			List: []config.AgentConfig{
				{ID: "worker", Enabled: true},
				{ID: "disabled", Enabled: false},
			},
		},
	}
	tool := NewSubagentsTool(cfg)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"enabled_only": true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	agents := payload["agents"].([]map[string]interface{})
	if len(agents) != 1 || agents[0]["id"] != "worker" {
		t.Fatalf("unexpected agents: %#v", agents)
	}
	subagents := agents[0]["subagents"].(config.AgentSubagentPolicyConfig)
	if !subagents.Enabled || subagents.MaxParallel != 4 {
		t.Fatalf("unexpected effective subagents config: %#v", subagents)
	}
	if inherited, _ := agents[0]["inherits_defaults"].(bool); !inherited {
		t.Fatalf("expected worker to inherit default subagent config")
	}
}

func TestAgentsAndSubagentsSupportNestedCamelCaseArgs(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentConfig{
				Enabled:    true,
				ToolPolicy: config.ToolPolicyConfig{Profile: "coding"},
				Subagents:  config.AgentSubagentPolicyConfig{Enabled: true, MaxParallel: 2, MaxDepth: 1, Timeout: time.Minute},
			},
			List: []config.AgentConfig{
				{ID: "worker", Enabled: true, ToolPolicy: config.ToolPolicyConfig{Profile: "coding"}},
				{ID: "viewer", Enabled: false, ToolPolicy: config.ToolPolicyConfig{Profile: "reader"}},
			},
		},
	}

	agentsTool := NewAgentsListTool(cfg)
	result, err := agentsTool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"activeOnly": true,
			"profile":    "coding",
		},
	})
	if err != nil {
		t.Fatalf("agents Execute returned error: %v", err)
	}
	agentsPayload := result.(map[string]interface{})
	agents := agentsPayload["agents"].([]map[string]interface{})
	if len(agents) != 1 || agents[0]["id"] != "worker" {
		t.Fatalf("unexpected filtered agents: %#v", agents)
	}

	subagentsTool := NewSubagentsTool(cfg)
	result, err = subagentsTool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"agentId":     "worker",
			"enabledOnly": true,
		},
	})
	if err != nil {
		t.Fatalf("subagents Execute returned error: %v", err)
	}
	subagentsPayload := result.(map[string]interface{})
	subagents := subagentsPayload["agents"].([]map[string]interface{})
	if len(subagents) != 1 || subagents[0]["id"] != "worker" {
		t.Fatalf("unexpected filtered subagents: %#v", subagents)
	}
}

func TestSubagentsToolExecuteAgentIDNotFound(t *testing.T) {
	cfg := &config.Config{Agents: *config.DefaultAgentsConfig()}
	tool := NewSubagentsTool(cfg)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"agent_id": "missing"}); err == nil {
		t.Fatal("expected missing agent error")
	}
}

func TestSubagentsToolSpawnUsesExecutor(t *testing.T) {
	cfg := &config.Config{Agents: *config.DefaultAgentsConfig()}
	tool := NewSubagentsTool(cfg)
	executor := &stubSubagentExecutor{
		result: &SubagentResult{
			RunID:       "child-1",
			ParentRunID: "parent-1",
			Status:      "completed",
			Result:      "done",
			Completed:   true,
			Terminal:    true,
		},
	}
	ctx := WithRunID(context.Background(), "parent-1")
	ctx = WithSubagentExecutor(ctx, executor)

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":          "run",
		"goal":            "investigate the failing migration",
		"agent_id":        "worker",
		"model":           "gpt-test",
		"context":         "Focus on the DB layer.",
		"wait":            false,
		"max_steps":       5,
		"max_tool_rounds": 9,
		"max_duration":    "45s",
		"metadata": map[string]interface{}{
			"source": "unit-test",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got, ok := result.(*SubagentResult)
	if !ok {
		t.Fatalf("expected *SubagentResult, got %T", result)
	}
	if got.RunID != "child-1" || got.Status != "completed" {
		t.Fatalf("unexpected result: %#v", got)
	}
	if executor.req.Goal != "investigate the failing migration" {
		t.Fatalf("goal = %q", executor.req.Goal)
	}
	if executor.req.AgentID != "worker" || executor.req.Model != "gpt-test" {
		t.Fatalf("unexpected target selection: %#v", executor.req)
	}
	if executor.req.Context != "Focus on the DB layer." {
		t.Fatalf("context = %q", executor.req.Context)
	}
	if executor.req.Wait {
		t.Fatal("expected wait=false to propagate")
	}
	if executor.req.MaxSteps != 5 || executor.req.MaxToolRounds != 9 {
		t.Fatalf("unexpected budgets: %#v", executor.req)
	}
	if executor.req.MaxDuration != 45*time.Second {
		t.Fatalf("max_duration = %v", executor.req.MaxDuration)
	}
	if source, _ := executor.req.Metadata["source"].(string); source != "unit-test" {
		t.Fatalf("unexpected metadata: %#v", executor.req.Metadata)
	}
}

func TestSubagentsToolSpawnRequiresExecutor(t *testing.T) {
	cfg := &config.Config{Agents: *config.DefaultAgentsConfig()}
	tool := NewSubagentsTool(cfg)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "spawn",
		"goal":   "do something",
	})
	if err == nil {
		t.Fatal("expected missing executor error")
	}
}
