package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

// AgentsListTool lists configured agents and defaults.
type AgentsListTool struct {
	cfg *config.Config
}

// SubagentsTool inspects subagent availability and policy.
type SubagentsTool struct {
	cfg *config.Config
}

// NewAgentsListTool creates a native agents_list tool.
func NewAgentsListTool(cfg *config.Config) *AgentsListTool {
	return &AgentsListTool{cfg: cfg}
}

// NewSubagentsTool creates a native subagents tool.
func NewSubagentsTool(cfg *config.Config) *SubagentsTool {
	return &SubagentsTool{cfg: cfg}
}

// Definition returns the tool schema.
func (t *AgentsListTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "agents_list",
		Description: "List configured agents, defaults, and effective tool-policy hints.",
		Icon:        "agents",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"active_only": map[string]interface{}{"type": "boolean", "description": "If true, only return enabled agents."},
				"profile":     map[string]interface{}{"type": "string", "description": "Optional tool policy profile filter."},
			},
		},
	}
}

// Definition returns the tool schema.
func (t *SubagentsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "subagents",
		Description: "Inspect effective subagent policy, or spawn a harness-backed child agent when action=spawn or action=run.",
		Icon:        "subagents",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":          map[string]interface{}{"type": "string", "description": "Optional action: inspect/list (default) or spawn/run to create a child agent run."},
				"goal":            map[string]interface{}{"type": "string", "description": "Child-agent goal. Required when action=spawn or action=run."},
				"context":         map[string]interface{}{"type": "string", "description": "Optional extra context passed to the child agent."},
				"wait":            map[string]interface{}{"type": "boolean", "description": "If true, wait for the child run to reach a terminal state before returning. Defaults to true for spawn/run."},
				"agent_id":        map[string]interface{}{"type": "string", "description": "Optional agent ID filter"},
				"model":           map[string]interface{}{"type": "string", "description": "Optional child model override."},
				"enabled_only":    map[string]interface{}{"type": "boolean", "description": "If true, only return agents with subagents effectively enabled"},
				"max_steps":       map[string]interface{}{"type": "integer", "description": "Optional child max_steps override."},
				"max_tool_rounds": map[string]interface{}{"type": "integer", "description": "Optional child max_tool_rounds override."},
				"max_duration":    map[string]interface{}{"type": "string", "description": "Optional child max duration, for example 2m or 30s."},
				"metadata":        map[string]interface{}{"type": "object", "description": "Optional child run metadata."},
			},
		},
	}
}

// Execute returns the configured agent catalog.
func (t *AgentsListTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	_ = ctx
	if t == nil || t.cfg == nil {
		return map[string]interface{}{"agents": []interface{}{}, "defaults": map[string]interface{}{}}, nil
	}
	activeOnly, _ := compatBoolArg(args, "active_only", "activeOnly")
	profile := strings.ToLower(strings.TrimSpace(firstCompatString(args, "profile")))

	agents := make([]map[string]interface{}, 0, len(t.cfg.Agents.List))
	for _, agent := range t.cfg.Agents.List {
		if activeOnly && !agent.Enabled {
			continue
		}
		agentProfile := strings.ToLower(strings.TrimSpace(agent.ToolPolicy.Profile))
		if profile != "" && agentProfile != profile {
			continue
		}
		agents = append(agents, map[string]interface{}{
			"id":          agent.ID,
			"enabled":     agent.Enabled,
			"description": agent.Description,
			"model":       agent.Model,
			"thinking":    agent.Thinking,
			"tool_policy": agent.ToolPolicy,
			"sandbox":     agent.Sandbox,
			"browser":     agent.Browser,
			"subagents":   agent.Subagents,
		})
	}

	return map[string]interface{}{
		"defaults": t.cfg.Agents.Defaults,
		"agents":   agents,
		"count":    len(agents),
	}, nil
}

// Execute returns effective subagent policy information.
func (t *SubagentsTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.cfg == nil {
		return map[string]interface{}{"agents": []interface{}{}, "defaults": map[string]interface{}{}}, nil
	}
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	goal := strings.TrimSpace(firstCompatString(args, "goal", "task", "objective", "prompt"))
	if shouldSpawnSubagent(action, goal) {
		return t.executeSpawn(ctx, args, goal)
	}
	agentID := strings.TrimSpace(firstCompatString(args, "agent_id", "agentId", "id", "agent"))
	enabledOnly, _ := compatBoolArg(args, "enabled_only", "enabledOnly")

	agents := make([]map[string]interface{}, 0, len(t.cfg.Agents.List))
	for _, agent := range t.cfg.Agents.List {
		if agentID != "" && agent.ID != agentID {
			continue
		}
		effective := effectiveAgentConfig(t.cfg.Agents.Defaults, agent)
		available := effective.Enabled && effective.Subagents.Enabled
		if enabledOnly && !available {
			continue
		}
		agents = append(agents, map[string]interface{}{
			"id":                 agent.ID,
			"description":        firstNonEmptyAgent(agent.Description, effective.Description),
			"enabled":            effective.Enabled,
			"available":          available,
			"model":              effective.Model,
			"thinking":           effective.Thinking,
			"tool_profile":       effective.ToolPolicy.Profile,
			"subagents":          effective.Subagents,
			"sandbox":            effective.Sandbox,
			"browser":            effective.Browser,
			"inherits_defaults":  isZeroSubagentPolicy(agent.Subagents),
			"source_tool_policy": agent.ToolPolicy,
		})
	}
	if agentID != "" && len(agents) == 0 {
		return nil, fmt.Errorf("agent %q not found", agentID)
	}

	return map[string]interface{}{
		"defaults": t.cfg.Agents.Defaults.Subagents,
		"agents":   agents,
		"count":    len(agents),
	}, nil
}

func (t *SubagentsTool) executeSpawn(ctx context.Context, args map[string]interface{}, goal string) (interface{}, error) {
	executor := GetSubagentExecutor(ctx)
	if executor == nil {
		return nil, fmt.Errorf("subagent execution is not available in this runtime")
	}
	if goal == "" {
		return nil, fmt.Errorf("goal is required when action=spawn or action=run")
	}

	wait := true
	if explicitWait, ok := compatBoolArg(args, "wait", "sync", "block"); ok {
		wait = explicitWait
	}

	req := normalizeSubagentRequest(SubagentRequest{
		Goal:          goal,
		AgentID:       strings.TrimSpace(firstCompatString(args, "agent_id", "agentId", "id", "agent")),
		Model:         strings.TrimSpace(firstCompatString(args, "model")),
		Context:       strings.TrimSpace(firstCompatString(args, "context", "instructions", "brief")),
		Wait:          wait,
		MaxSteps:      compatIntOrZero(args, "max_steps", "maxSteps"),
		MaxToolRounds: compatIntOrZero(args, "max_tool_rounds", "maxToolRounds"),
		MaxDuration:   compatDurationOrZero(args, "max_duration", "maxDuration"),
		Metadata:      compatMapOrNil(args, "metadata"),
	})

	result, err := executor.ExecuteSubagent(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]interface{}{}, nil
	}
	return result, nil
}

func shouldSpawnSubagent(action, goal string) bool {
	switch action {
	case "spawn", "run", "start", "create":
		return true
	}
	return strings.TrimSpace(goal) != ""
}

func compatIntOrZero(args map[string]interface{}, keys ...string) int {
	value, ok := firstCompatIntDeep(args, keys...)
	if !ok || value <= 0 {
		return 0
	}
	return value
}

func compatDurationOrZero(args map[string]interface{}, keys ...string) time.Duration {
	value, ok := firstCompatValueDeep(args, keys...)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case time.Duration:
		if typed > 0 {
			return typed
		}
	case string:
		dur, err := time.ParseDuration(strings.TrimSpace(typed))
		if err == nil && dur > 0 {
			return dur
		}
	case int:
		if typed > 0 {
			return time.Duration(typed) * time.Second
		}
	case int32:
		if typed > 0 {
			return time.Duration(typed) * time.Second
		}
	case int64:
		if typed > 0 {
			return time.Duration(typed) * time.Second
		}
	case float64:
		if typed > 0 {
			return time.Duration(typed * float64(time.Second))
		}
	}
	return 0
}

func compatMapOrNil(args map[string]interface{}, keys ...string) map[string]interface{} {
	value, ok := firstCompatValueDeep(args, keys...)
	if !ok {
		return nil
	}
	typed, ok := coerceCompatMap(value)
	if !ok || len(typed) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(typed))
	for k, v := range typed {
		out[k] = v
	}
	return out
}

func effectiveAgentConfig(defaults, agent config.AgentConfig) config.AgentConfig {
	out := agent
	if out.Thinking == "" {
		out.Thinking = defaults.Thinking
	}
	if out.Description == "" {
		out.Description = defaults.Description
	}
	if out.Model == "" {
		out.Model = defaults.Model
	}
	if out.ToolPolicy.Profile == "" {
		out.ToolPolicy.Profile = defaults.ToolPolicy.Profile
	}
	if len(out.ToolPolicy.Allow) == 0 && len(defaults.ToolPolicy.Allow) > 0 {
		out.ToolPolicy.Allow = append([]string(nil), defaults.ToolPolicy.Allow...)
	}
	if len(out.ToolPolicy.Deny) == 0 && len(defaults.ToolPolicy.Deny) > 0 {
		out.ToolPolicy.Deny = append([]string(nil), defaults.ToolPolicy.Deny...)
	}
	if len(out.ToolPolicy.ByProvider) == 0 && len(defaults.ToolPolicy.ByProvider) > 0 {
		out.ToolPolicy.ByProvider = defaults.ToolPolicy.ByProvider
	}
	if out.Sandbox.Mode == "" {
		out.Sandbox.Mode = defaults.Sandbox.Mode
	}
	if out.Sandbox.Scope == "" {
		out.Sandbox.Scope = defaults.Sandbox.Scope
	}
	if out.Browser.Profile == "" {
		out.Browser.Profile = defaults.Browser.Profile
	}
	if isZeroSubagentPolicy(out.Subagents) {
		out.Subagents = defaults.Subagents
	} else {
		if out.Subagents.MaxParallel == 0 {
			out.Subagents.MaxParallel = defaults.Subagents.MaxParallel
		}
		if out.Subagents.MaxDepth == 0 {
			out.Subagents.MaxDepth = defaults.Subagents.MaxDepth
		}
		if out.Subagents.Timeout == 0 {
			out.Subagents.Timeout = defaults.Subagents.Timeout
		}
		if out.Subagents.CallbackMode == "" {
			out.Subagents.CallbackMode = defaults.Subagents.CallbackMode
		}
		if out.Subagents.DefaultModel == "" {
			out.Subagents.DefaultModel = defaults.Subagents.DefaultModel
		}
		if out.Subagents.CheapModel == "" {
			out.Subagents.CheapModel = defaults.Subagents.CheapModel
		}
	}
	return out
}

func isZeroSubagentPolicy(policy config.AgentSubagentPolicyConfig) bool {
	return !policy.Enabled && policy.MaxParallel == 0 && policy.MaxDepth == 0 && policy.DefaultModel == "" && policy.CheapModel == "" && policy.Timeout == 0 && policy.CallbackMode == ""
}

func firstNonEmptyAgent(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

// RegisterAgentTools registers native agent-management tools.
func RegisterAgentTools(registry *Registry, cfg *config.Config) {
	if registry == nil || cfg == nil {
		return
	}
	registry.Register(NewAgentsListTool(cfg))
	registry.Register(NewSubagentsTool(cfg))
}
