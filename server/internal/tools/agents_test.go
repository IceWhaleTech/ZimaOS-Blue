package tools

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

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

func TestSubagentsToolExecuteAgentIDNotFound(t *testing.T) {
	cfg := &config.Config{Agents: *config.DefaultAgentsConfig()}
	tool := NewSubagentsTool(cfg)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"agent_id": "missing"}); err == nil {
		t.Fatal("expected missing agent error")
	}
}
