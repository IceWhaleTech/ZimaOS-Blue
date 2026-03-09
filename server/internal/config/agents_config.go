package config

import "time"

// ToolPolicyConfig controls which tools are visible to a runtime scope.
type ToolPolicyConfig struct {
	Profile    string                      `yaml:"profile" json:"profile"`
	Allow      []string                    `yaml:"allow" json:"allow,omitempty"`
	Deny       []string                    `yaml:"deny" json:"deny,omitempty"`
	ByProvider map[string]ToolPolicyConfig `yaml:"by_provider" json:"by_provider,omitempty"`
}

// AgentSandboxPolicyConfig controls sandbox defaults for an agent.
type AgentSandboxPolicyConfig struct {
	Mode  string `yaml:"mode" json:"mode"`
	Scope string `yaml:"scope" json:"scope"`
}

// AgentBrowserPolicyConfig controls browser defaults for an agent.
type AgentBrowserPolicyConfig struct {
	Profile     string `yaml:"profile" json:"profile"`
	PersistAuth bool   `yaml:"persist_auth" json:"persist_auth"`
}

// AgentSubagentPolicyConfig controls child-agent defaults.
type AgentSubagentPolicyConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	MaxParallel  int           `yaml:"max_parallel" json:"max_parallel"`
	MaxDepth     int           `yaml:"max_depth" json:"max_depth"`
	DefaultModel string        `yaml:"default_model" json:"default_model,omitempty"`
	CheapModel   string        `yaml:"cheap_model" json:"cheap_model,omitempty"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
	CallbackMode string        `yaml:"callback" json:"callback"`
}

// AgentConfig defines one logical agent persona/runtime target.
type AgentConfig struct {
	ID          string                    `yaml:"id" json:"id"`
	Enabled     bool                      `yaml:"enabled" json:"enabled"`
	Description string                    `yaml:"description" json:"description,omitempty"`
	Model       string                    `yaml:"model" json:"model,omitempty"`
	Thinking    string                    `yaml:"thinking" json:"thinking,omitempty"`
	ToolPolicy  ToolPolicyConfig          `yaml:"tool_policy" json:"tool_policy"`
	Sandbox     AgentSandboxPolicyConfig  `yaml:"sandbox" json:"sandbox"`
	Browser     AgentBrowserPolicyConfig  `yaml:"browser" json:"browser"`
	Subagents   AgentSubagentPolicyConfig `yaml:"subagents" json:"subagents"`
}

// AgentsConfig holds agent defaults and named agent entries.
type AgentsConfig struct {
	Defaults AgentConfig   `yaml:"defaults" json:"defaults"`
	List     []AgentConfig `yaml:"list" json:"list,omitempty"`
}

// DefaultAgentsConfig returns the default agent policy configuration.
func DefaultAgentsConfig() *AgentsConfig {
	return &AgentsConfig{
		Defaults: AgentConfig{
			Enabled:  true,
			Thinking: "medium",
			ToolPolicy: ToolPolicyConfig{
				Profile: "coding",
			},
			Sandbox: AgentSandboxPolicyConfig{
				Mode:  "inherit",
				Scope: "session",
			},
			Browser: AgentBrowserPolicyConfig{
				Profile:     "agent-isolated",
				PersistAuth: true,
			},
			Subagents: AgentSubagentPolicyConfig{
				Enabled:      true,
				MaxParallel:  3,
				MaxDepth:     2,
				Timeout:      5 * time.Minute,
				CallbackMode: "summary_only",
			},
		},
		List: []AgentConfig{
			{
				ID:          "main",
				Enabled:     true,
				Description: "Primary default agent",
				ToolPolicy: ToolPolicyConfig{
					Profile: "full",
				},
				Sandbox: AgentSandboxPolicyConfig{
					Mode:  "inherit",
					Scope: "session",
				},
				Browser: AgentBrowserPolicyConfig{
					Profile:     "agent-isolated",
					PersistAuth: true,
				},
				Subagents: AgentSubagentPolicyConfig{
					Enabled:      true,
					MaxParallel:  3,
					MaxDepth:     2,
					Timeout:      5 * time.Minute,
					CallbackMode: "summary_only",
				},
			},
		},
	}
}
