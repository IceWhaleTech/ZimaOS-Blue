package proxy

import (
	"net/http"
	"testing"
)

// --- Types under test (defined in routing_rules.go) ---

func TestHeaderRuleMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "header-edge",
		Priority: 10,
		Condition: RouteCondition{
			Header:      "X-Model-Tier",
			HeaderValue: "edge",
		},
		TargetModel: "qwen2.5-7b",
		Origin:      OriginLocal,
		Fallback:    "claude-sonnet-4-20250514",
	}

	req := &RouteRequest{
		Headers: http.Header{"X-Model-Tier": {"edge"}},
	}
	decision := rule.Evaluate(req)
	if !decision.Matched {
		t.Fatal("expected header rule to match")
	}
	if decision.Model != "qwen2.5-7b" {
		t.Errorf("expected model qwen2.5-7b, got %s", decision.Model)
	}
	if decision.Origin != OriginLocal {
		t.Errorf("expected origin local, got %s", decision.Origin)
	}
	if decision.Fallback != "claude-sonnet-4-20250514" {
		t.Errorf("expected fallback claude-sonnet-4-20250514, got %s", decision.Fallback)
	}
	if decision.Rule != "header-edge" {
		t.Errorf("expected rule name header-edge, got %s", decision.Rule)
	}
}

func TestHeaderRuleNoMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "header-edge",
		Priority: 10,
		Condition: RouteCondition{
			Header:      "X-Model-Tier",
			HeaderValue: "edge",
		},
		TargetModel: "qwen2.5-7b",
		Origin:      OriginLocal,
	}

	req := &RouteRequest{
		Headers: http.Header{"X-Model-Tier": {"cloud"}},
	}
	decision := rule.Evaluate(req)
	if decision.Matched {
		t.Fatal("expected header rule NOT to match when value differs")
	}
}

func TestBodySizeRuleMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "small-prompt",
		Priority: 20,
		Condition: RouteCondition{
			MaxBodyBytes: 4096,
		},
		TargetModel: "qwen2.5-7b",
		Origin:      OriginLocal,
		Fallback:    "claude-sonnet-4-20250514",
	}

	req := &RouteRequest{BodySize: 2000}
	decision := rule.Evaluate(req)
	if !decision.Matched {
		t.Fatal("expected body size rule to match for small body")
	}
	if decision.Model != "qwen2.5-7b" {
		t.Errorf("expected model qwen2.5-7b, got %s", decision.Model)
	}
}

func TestBodySizeRuleNoMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "small-prompt",
		Priority: 20,
		Condition: RouteCondition{
			MaxBodyBytes: 4096,
		},
		TargetModel: "qwen2.5-7b",
		Origin:      OriginLocal,
	}

	req := &RouteRequest{BodySize: 8000}
	decision := rule.Evaluate(req)
	if decision.Matched {
		t.Fatal("expected body size rule NOT to match for large body")
	}
}

func TestToolPatternRuleMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "tool-filter",
		Priority: 30,
		Condition: RouteCondition{
			ToolPattern: "^(list_files|file_search|grep)$",
		},
		TargetModel: "deepseek-coder-6.7b",
		Origin:      OriginLocal,
		Fallback:    "claude-sonnet-4-20250514",
	}

	req := &RouteRequest{ToolNames: []string{"file_search"}}
	decision := rule.Evaluate(req)
	if !decision.Matched {
		t.Fatal("expected tool pattern rule to match file_search")
	}

	req2 := &RouteRequest{ToolNames: []string{"grep"}}
	decision2 := rule.Evaluate(req2)
	if !decision2.Matched {
		t.Fatal("expected tool pattern rule to match grep")
	}
}

func TestToolPatternRuleNoMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "tool-filter",
		Priority: 30,
		Condition: RouteCondition{
			ToolPattern: "^(list_files|file_search|grep)$",
		},
		TargetModel: "deepseek-coder-6.7b",
		Origin:      OriginLocal,
	}

	req := &RouteRequest{ToolNames: []string{"code_edit"}}
	decision := rule.Evaluate(req)
	if decision.Matched {
		t.Fatal("expected tool pattern rule NOT to match code_edit")
	}
}

func TestSystemTagRuleMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "orchestrator-tag",
		Priority: 40,
		Condition: RouteCondition{
			SystemTag: "[ORCHESTRATOR]",
		},
		TargetModel: "llama-3.1-8b-instruct",
		Origin:      OriginLocal,
		Fallback:    "claude-sonnet-4-20250514",
	}

	req := &RouteRequest{SystemMessage: "[ORCHESTRATOR] You are a task router."}
	decision := rule.Evaluate(req)
	if !decision.Matched {
		t.Fatal("expected system tag rule to match")
	}
}

func TestSystemTagRuleNoMatch(t *testing.T) {
	rule := RoutingRule{
		Name:     "orchestrator-tag",
		Priority: 40,
		Condition: RouteCondition{
			SystemTag: "[ORCHESTRATOR]",
		},
		TargetModel: "llama-3.1-8b-instruct",
		Origin:      OriginLocal,
	}

	req := &RouteRequest{SystemMessage: "You are a helpful assistant."}
	decision := rule.Evaluate(req)
	if decision.Matched {
		t.Fatal("expected system tag rule NOT to match")
	}
}

func TestPriorityOrdering(t *testing.T) {
	rules := []RoutingRule{
		{
			Name:     "low-priority",
			Priority: 50,
			Condition: RouteCondition{
				Header: "X-Model-Tier", HeaderValue: "edge",
			},
			TargetModel: "low-model",
			Origin:      OriginLocal,
		},
		{
			Name:     "high-priority",
			Priority: 10,
			Condition: RouteCondition{
				Header: "X-Model-Tier", HeaderValue: "edge",
			},
			TargetModel: "high-model",
			Origin:      OriginLocal,
		},
	}

	engine := NewRuleEngine(rules)
	req := &RouteRequest{
		Headers: http.Header{"X-Model-Tier": {"edge"}},
	}
	decision := engine.Evaluate(req)
	if decision == nil {
		t.Fatal("expected a match")
	}
	if decision.Model != "high-model" {
		t.Errorf("expected high-priority model, got %s", decision.Model)
	}
	if decision.Rule != "high-priority" {
		t.Errorf("expected rule high-priority, got %s", decision.Rule)
	}
}

func TestNoMatchPassthrough(t *testing.T) {
	rules := []RoutingRule{
		{
			Name:     "header-edge",
			Priority: 10,
			Condition: RouteCondition{
				Header: "X-Model-Tier", HeaderValue: "edge",
			},
			TargetModel: "qwen2.5-7b",
			Origin:      OriginLocal,
		},
	}

	engine := NewRuleEngine(rules)
	req := &RouteRequest{
		Headers: http.Header{},
	}
	decision := engine.Evaluate(req)
	if decision != nil {
		t.Fatalf("expected nil decision for no match, got %+v", decision)
	}
}

func TestEmptyConditionNeverMatches(t *testing.T) {
	rule := RoutingRule{
		Name:        "empty",
		Priority:    1,
		Condition:   RouteCondition{},
		TargetModel: "should-not-match",
		Origin:      OriginLocal,
	}

	req := &RouteRequest{
		Headers:       http.Header{"X-Foo": {"bar"}},
		BodySize:      100,
		ToolNames:     []string{"grep"},
		SystemMessage: "hello",
	}
	decision := rule.Evaluate(req)
	if decision.Matched {
		t.Fatal("empty condition should never match")
	}
}

func TestMultipleConditionsAllMustMatch(t *testing.T) {
	// Rule with both header AND body size conditions — both must match
	rule := RoutingRule{
		Name:     "combo",
		Priority: 10,
		Condition: RouteCondition{
			Header:       "X-Model-Tier",
			HeaderValue:  "edge",
			MaxBodyBytes: 4096,
		},
		TargetModel: "qwen2.5-7b",
		Origin:      OriginLocal,
	}

	// Both match
	req := &RouteRequest{
		Headers:  http.Header{"X-Model-Tier": {"edge"}},
		BodySize: 2000,
	}
	if !rule.Evaluate(req).Matched {
		t.Fatal("expected combo rule to match when both conditions met")
	}

	// Header matches, body too large
	req2 := &RouteRequest{
		Headers:  http.Header{"X-Model-Tier": {"edge"}},
		BodySize: 8000,
	}
	if rule.Evaluate(req2).Matched {
		t.Fatal("expected combo rule NOT to match when body too large")
	}

	// Body matches, header missing
	req3 := &RouteRequest{
		Headers:  http.Header{},
		BodySize: 2000,
	}
	if rule.Evaluate(req3).Matched {
		t.Fatal("expected combo rule NOT to match when header missing")
	}
}

func TestBodySizeZeroIsValid(t *testing.T) {
	rule := RoutingRule{
		Name:     "small-prompt",
		Priority: 20,
		Condition: RouteCondition{
			MaxBodyBytes: 4096,
		},
		TargetModel: "qwen2.5-7b",
		Origin:      OriginLocal,
	}
	// BodySize 0 = empty request body, should match
	req := &RouteRequest{BodySize: 0}
	if !rule.Evaluate(req).Matched {
		t.Fatal("BodySize 0 should match MaxBodyBytes rule")
	}
}

func TestBodySizeBoundary(t *testing.T) {
	rule := RoutingRule{
		Name:     "small-prompt",
		Priority: 20,
		Condition: RouteCondition{
			MaxBodyBytes: 4096,
		},
		TargetModel: "m",
		Origin:      OriginLocal,
	}
	// Exactly at boundary
	if !rule.Evaluate(&RouteRequest{BodySize: 4096}).Matched {
		t.Fatal("BodySize == MaxBodyBytes should match")
	}
	// One over
	if rule.Evaluate(&RouteRequest{BodySize: 4097}).Matched {
		t.Fatal("BodySize > MaxBodyBytes should not match")
	}
}

func TestHeaderCaseInsensitive(t *testing.T) {
	rule := RoutingRule{
		Name:     "header-test",
		Priority: 10,
		Condition: RouteCondition{
			Header: "X-Model-Tier", HeaderValue: "small",
		},
		TargetModel: "haiku",
		Origin:      OriginCloud,
	}
	req := &RouteRequest{Headers: http.Header{"X-Model-Tier": {"SMALL"}}}
	if !rule.Evaluate(req).Matched {
		t.Fatal("header value match should be case-insensitive")
	}
}

func TestToolPatternEmptyToolNames(t *testing.T) {
	rule := RoutingRule{
		Name:     "tool-filter",
		Priority: 30,
		Condition: RouteCondition{
			ToolPattern: "^grep$",
		},
		TargetModel: "m",
		Origin:      OriginLocal,
	}
	// Empty tool names should not match
	req := &RouteRequest{ToolNames: []string{}}
	if rule.Evaluate(req).Matched {
		t.Fatal("empty ToolNames should not match tool pattern rule")
	}
	// Nil tool names
	req2 := &RouteRequest{ToolNames: nil}
	if rule.Evaluate(req2).Matched {
		t.Fatal("nil ToolNames should not match tool pattern rule")
	}
}

func TestTierFieldPropagated(t *testing.T) {
	rule := RoutingRule{
		Name:     "small-route",
		Priority: 10,
		Condition: RouteCondition{
			Header: "X-Model-Tier", HeaderValue: "small",
		},
		TargetModel: "haiku",
		Origin:      OriginCloud,
		Tier:        TierSmall,
	}
	req := &RouteRequest{Headers: http.Header{"X-Model-Tier": {"small"}}}
	d := rule.Evaluate(req)
	if d.Tier != TierSmall {
		t.Errorf("expected tier small, got %s", d.Tier)
	}
}

func TestRuleEnginePreBuiltReason(t *testing.T) {
	rules := []RoutingRule{
		{
			Name:     "combo",
			Priority: 10,
			Condition: RouteCondition{
				Header: "X-Tier", HeaderValue: "eco",
				MaxBodyBytes: 1024,
			},
			TargetModel: "m",
			Origin:      OriginLocal,
		},
	}
	engine := NewRuleEngine(rules)
	req := &RouteRequest{
		Headers:  http.Header{"X-Tier": {"eco"}},
		BodySize: 500,
	}
	d := engine.Evaluate(req)
	if d == nil {
		t.Fatal("expected match")
	}
	if d.Reason != "header X-Tier=eco, body_size<=1024" {
		t.Errorf("unexpected reason: %q", d.Reason)
	}
}
