package proxy

import (
	"testing"
)

func TestParseRoutingConfig(t *testing.T) {
	yaml := `
enabled: true
rules:
  - name: "header-edge"
    priority: 10
    condition:
      header: "X-Model-Tier"
      header_value: "edge"
    target_model: "qwen2.5-7b-instruct"
    origin: "local"
    fallback: "claude-sonnet-4-20250514"
  - name: "small-prompt"
    priority: 20
    condition:
      max_body_bytes: 4096
    target_model: "qwen2.5-7b-instruct"
    origin: "local"
    fallback: "claude-sonnet-4-20250514"
model_origins:
  "claude-*": "cloud"
  "gpt-*": "cloud"
  "qwen*": "local"
pricing:
  "claude-sonnet-4-20250514":
    input: 3.0
    output: 15.0
  "qwen2.5-7b-instruct":
    input: 0.0
    output: 0.0
`
	cfg, err := ParseRoutingConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseRoutingConfig failed: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "header-edge" {
		t.Errorf("expected first rule name header-edge, got %s", cfg.Rules[0].Name)
	}
	if cfg.Rules[0].Origin != OriginLocal {
		t.Errorf("expected origin local, got %s", cfg.Rules[0].Origin)
	}
	if len(cfg.ModelOrigins) != 3 {
		t.Errorf("expected 3 model origins, got %d", len(cfg.ModelOrigins))
	}
	if len(cfg.Pricing) != 2 {
		t.Errorf("expected 2 pricing entries, got %d", len(cfg.Pricing))
	}
	if cfg.Pricing["claude-sonnet-4-20250514"].Input != 3.0 {
		t.Errorf("expected input price 3.0, got %f", cfg.Pricing["claude-sonnet-4-20250514"].Input)
	}
}

func TestParseRoutingConfigDefaults(t *testing.T) {
	cfg, err := ParseRoutingConfig([]byte("{}"))
	if err != nil {
		t.Fatalf("ParseRoutingConfig failed: %v", err)
	}
	if cfg.Enabled {
		t.Error("default should be disabled")
	}
	if cfg.Rules == nil {
		t.Error("rules should be initialized (empty)")
	}
}

func TestValidateRoutingConfig_MissingFallback(t *testing.T) {
	cfg := &RoutingConfig{
		Enabled: true,
		Rules: []RoutingRule{
			{
				Name:        "no-fallback",
				Priority:    10,
				TargetModel: "local-model",
				Origin:      OriginLocal,
				// Fallback intentionally empty
			},
		},
	}
	errs := ValidateRoutingConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for missing fallback on local rule")
	}
}

func TestValidateRoutingConfig_CloudNoFallbackOK(t *testing.T) {
	cfg := &RoutingConfig{
		Enabled: true,
		Rules: []RoutingRule{
			{
				Name:        "cloud-rule",
				Priority:    10,
				Condition:   RouteCondition{Header: "X-Tier", HeaderValue: "premium"},
				TargetModel: "claude-sonnet",
				Origin:      OriginCloud,
				// No fallback needed for cloud
			},
		},
	}
	errs := ValidateRoutingConfig(cfg)
	if len(errs) != 0 {
		t.Errorf("cloud rules should not require fallback, got errors: %v", errs)
	}
}

func TestValidateRoutingConfig_InvalidToolPattern(t *testing.T) {
	cfg := &RoutingConfig{
		Enabled: true,
		Rules: []RoutingRule{
			{
				Name:     "bad-regex",
				Priority: 10,
				Condition: RouteCondition{
					ToolPattern: "[invalid",
				},
				TargetModel: "model",
				Origin:      OriginCloud,
			},
		},
	}
	errs := ValidateRoutingConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid tool pattern regex")
	}
}

func TestValidateRoutingConfig_DuplicateNames(t *testing.T) {
	cfg := &RoutingConfig{
		Enabled: true,
		Rules: []RoutingRule{
			{Name: "dup", Priority: 10, Condition: RouteCondition{SystemTag: "[A]"}, TargetModel: "m", Origin: OriginCloud},
			{Name: "dup", Priority: 20, Condition: RouteCondition{SystemTag: "[B]"}, TargetModel: "m", Origin: OriginCloud},
		},
	}
	errs := ValidateRoutingConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for duplicate rule names")
	}
}

func TestValidateRoutingConfig_EmptyCondition(t *testing.T) {
	cfg := &RoutingConfig{
		Enabled: true,
		Rules: []RoutingRule{
			{Name: "empty", Priority: 10, Condition: RouteCondition{}, TargetModel: "m", Origin: OriginCloud},
		},
	}
	errs := ValidateRoutingConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for empty condition")
	}
}

func TestRoutingConfigToRuleEngine(t *testing.T) {
	cfg := &RoutingConfig{
		Enabled: true,
		Rules: []RoutingRule{
			{Name: "r1", Priority: 20, Condition: RouteCondition{SystemTag: "[SIMPLE]"}, TargetModel: "m1", Origin: OriginLocal, Fallback: "f1"},
			{Name: "r2", Priority: 10, Condition: RouteCondition{SystemTag: "[EDGE]"}, TargetModel: "m2", Origin: OriginLocal, Fallback: "f2"},
		},
	}
	engine := cfg.ToRuleEngine()
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	// r2 (priority 10) should be evaluated first
	req := &RouteRequest{SystemMessage: "[EDGE] [SIMPLE] do something"}
	d := engine.Evaluate(req)
	if d == nil || d.Rule != "r2" {
		t.Errorf("expected r2 (higher priority) to match first, got %v", d)
	}
}
