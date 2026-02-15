package proxy

import (
	"net/http"
	"testing"
)

func BenchmarkRuleEvaluation_10Rules(b *testing.B) {
	rules := []RoutingRule{
		{Name: "r1", Priority: 10, Condition: RouteCondition{Header: "X-Model-Tier", HeaderValue: "edge"}, TargetModel: "m1", Origin: OriginLocal},
		{Name: "r2", Priority: 20, Condition: RouteCondition{MaxBodyBytes: 4096}, TargetModel: "m2", Origin: OriginLocal},
		{Name: "r3", Priority: 30, Condition: RouteCondition{ToolPattern: "^(list_files|grep)$"}, TargetModel: "m3", Origin: OriginLocal},
		{Name: "r4", Priority: 40, Condition: RouteCondition{SystemTag: "[ORCHESTRATOR]"}, TargetModel: "m4", Origin: OriginLocal},
		{Name: "r5", Priority: 50, Condition: RouteCondition{Header: "X-Task-Type", HeaderValue: "classify"}, TargetModel: "m5", Origin: OriginLocal},
		{Name: "r6", Priority: 60, Condition: RouteCondition{MaxBodyBytes: 8192}, TargetModel: "m6", Origin: OriginLocal},
		{Name: "r7", Priority: 70, Condition: RouteCondition{ToolPattern: "^file_search$"}, TargetModel: "m7", Origin: OriginLocal},
		{Name: "r8", Priority: 80, Condition: RouteCondition{SystemTag: "[SIMPLE]"}, TargetModel: "m8", Origin: OriginLocal},
		{Name: "r9", Priority: 90, Condition: RouteCondition{Header: "X-Priority", HeaderValue: "low"}, TargetModel: "m9", Origin: OriginLocal},
		{Name: "r10", Priority: 100, Condition: RouteCondition{MaxBodyBytes: 16384}, TargetModel: "m10", Origin: OriginLocal},
	}
	engine := NewRuleEngine(rules)
	// Request that matches last rule (worst case)
	req := &RouteRequest{
		Headers:  http.Header{},
		BodySize: 10000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(req)
	}
}

func BenchmarkRuleEvaluation_FirstMatch(b *testing.B) {
	rules := []RoutingRule{
		{Name: "r1", Priority: 10, Condition: RouteCondition{Header: "X-Model-Tier", HeaderValue: "edge"}, TargetModel: "m1", Origin: OriginLocal},
	}
	engine := NewRuleEngine(rules)
	req := &RouteRequest{
		Headers: http.Header{"X-Model-Tier": {"edge"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(req)
	}
}

func BenchmarkOriginResolution(b *testing.B) {
	reg := NewOriginRegistry(map[string]string{
		"claude-*":        "cloud",
		"gpt-*":           "cloud",
		"deepseek-r1*":    "cloud",
		"qwen*":           "local",
		"llama*":          "local",
		"deepseek-coder*": "local",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg.Resolve("claude-sonnet-4-20250514")
	}
}

func BenchmarkOriginResolution_Miss(b *testing.B) {
	reg := NewOriginRegistry(map[string]string{
		"claude-*": "cloud",
		"gpt-*":    "cloud",
		"qwen*":    "local",
		"llama*":   "local",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg.Resolve("unknown-model-xyz")
	}
}

func BenchmarkApplyModelRouting_HeaderOnly(b *testing.B) {
	ph := &ProxyHandler{}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{Name: "economy", Priority: 1, Condition: RouteCondition{Header: "X-Tier", HeaderValue: "economy"}, TargetModel: "haiku", Tier: TierEconomy},
	}))
	body := []byte(`{"model":"claude-3-opus","messages":[{"role":"user","content":"hi"}]}`)
	r, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("X-Tier", "economy")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr := &parsedRequest{body: body, model: "claude-3-opus"}
		ph.applyModelRouting(r, pr)
	}
}

func BenchmarkApplyModelRouting_NoMatch(b *testing.B) {
	ph := &ProxyHandler{}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{Name: "r1", Priority: 1, Condition: RouteCondition{Header: "X-Tier", HeaderValue: "economy"}, TargetModel: "haiku"},
		{Name: "r2", Priority: 2, Condition: RouteCondition{MaxBodyBytes: 50}, TargetModel: "mini"},
	}))
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello world, this is a longer message"}]}`)
	r, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr := &parsedRequest{body: body, model: "gpt-4"}
		ph.applyModelRouting(r, pr)
	}
}
