package proxy

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// ModelPricing holds per-model token pricing (USD per 1M tokens).
type ModelPricing struct {
	Input  float64 `yaml:"input" json:"input"`
	Output float64 `yaml:"output" json:"output"`
}

// RoutingConfig is the top-level routing configuration.
type RoutingConfig struct {
	Enabled      bool                    `yaml:"enabled" json:"enabled"`
	Rules        []RoutingRule           `yaml:"rules" json:"rules"`
	ModelOrigins map[string]string       `yaml:"model_origins" json:"model_origins"`
	Pricing      map[string]ModelPricing `yaml:"pricing" json:"pricing"`
}

// ParseRoutingConfig parses YAML bytes into a RoutingConfig.
func ParseRoutingConfig(data []byte) (*RoutingConfig, error) {
	cfg := &RoutingConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse routing config: %w", err)
	}
	if cfg.Rules == nil {
		cfg.Rules = []RoutingRule{}
	}
	if cfg.ModelOrigins == nil {
		cfg.ModelOrigins = map[string]string{}
	}
	if cfg.Pricing == nil {
		cfg.Pricing = map[string]ModelPricing{}
	}
	return cfg, nil
}

// ValidateRoutingConfig checks for common configuration errors.
func ValidateRoutingConfig(cfg *RoutingConfig) []error {
	var errs []error
	names := make(map[string]bool, len(cfg.Rules))
	for i, rule := range cfg.Rules {
		// Duplicate rule names
		if rule.Name != "" {
			if names[rule.Name] {
				errs = append(errs, fmt.Errorf("rule[%d] %q: duplicate rule name", i, rule.Name))
			}
			names[rule.Name] = true
		}
		// Local/edge rules must have a fallback
		if (rule.Origin == OriginLocal || rule.Origin == OriginEdge) && rule.Fallback == "" {
			errs = append(errs, fmt.Errorf("rule[%d] %q: local/edge rules must have a fallback model", i, rule.Name))
		}
		// Rule must have a condition
		if !rule.Condition.hasCondition() {
			errs = append(errs, fmt.Errorf("rule[%d] %q: rule has no condition", i, rule.Name))
		}
		// Validate tool pattern regex
		if rule.Condition.ToolPattern != "" {
			if _, err := regexp.Compile(rule.Condition.ToolPattern); err != nil {
				errs = append(errs, fmt.Errorf("rule[%d] %q: invalid tool_pattern: %w", i, rule.Name, err))
			}
		}
	}
	return errs
}

// ToRuleEngine creates a RuleEngine from this config.
func (c *RoutingConfig) ToRuleEngine() *RuleEngine {
	return NewRuleEngine(c.Rules)
}
