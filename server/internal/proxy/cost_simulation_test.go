package proxy

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
)

// ModelPricing: USD per 1M tokens (2025 public pricing)
var modelPrices = map[string]ModelPricing{
	"claude-3-opus":   {Input: 15.00, Output: 75.00},
	"claude-3-sonnet": {Input: 3.00, Output: 15.00},
	"claude-3-haiku":  {Input: 0.25, Output: 1.25},
	"gpt-4":           {Input: 30.00, Output: 60.00},
	"gpt-4o":          {Input: 2.50, Output: 10.00},
	"gpt-4o-mini":     {Input: 0.15, Output: 0.60},
	"deepseek-r1":     {Input: 0.55, Output: 2.19},
}

// simulatedRequest represents a single API call in the traffic mix.
type simulatedRequest struct {
	model         string
	inputTokens   int
	outputTokens  int
	bodySize      int
	headers       http.Header
	tools         []string
	systemMessage string
	label         string // human-readable description
}

// trafficMix returns 1000 requests modeling a realistic personal AI assistant workload.
// Distribution: 40% short Q&A, 20% tool calls, 15% background tasks,
// 15% long-form generation, 10% complex reasoning.
func trafficMix(rng *rand.Rand) []simulatedRequest {
	reqs := make([]simulatedRequest, 0, 1000)

	// 40% short Q&A (small body, simple questions)
	for i := 0; i < 400; i++ {
		models := []string{"claude-3-opus", "claude-3-sonnet", "gpt-4", "gpt-4o"}
		reqs = append(reqs, simulatedRequest{
			model:        models[rng.Intn(len(models))],
			inputTokens:  200 + rng.Intn(800),  // 200-1000 tokens
			outputTokens: 100 + rng.Intn(400),  // 100-500 tokens
			bodySize:     500 + rng.Intn(2000), // 500-2500 bytes
			headers:      http.Header{},
			label:        "short-qa",
		})
	}

	// 20% tool calls (file search, grep, etc.)
	for i := 0; i < 200; i++ {
		tools := [][]string{
			{"list_files", "grep"},
			{"file_search"},
			{"get_weather"},
			{"calculator"},
		}
		models := []string{"claude-3-opus", "claude-3-sonnet", "gpt-4"}
		reqs = append(reqs, simulatedRequest{
			model:        models[rng.Intn(len(models))],
			inputTokens:  500 + rng.Intn(1500),
			outputTokens: 200 + rng.Intn(800),
			bodySize:     1000 + rng.Intn(3000),
			headers:      http.Header{},
			tools:        tools[rng.Intn(len(tools))],
			label:        "tool-call",
		})
	}

	// 15% background tasks (title gen, summarize)
	for i := 0; i < 150; i++ {
		models := []string{"claude-3-opus", "claude-3-sonnet", "gpt-4", "gpt-4o"}
		h := http.Header{}
		h.Set("X-Background-Task", "true")
		reqs = append(reqs, simulatedRequest{
			model:        models[rng.Intn(len(models))],
			inputTokens:  300 + rng.Intn(700),
			outputTokens: 50 + rng.Intn(200),
			bodySize:     800 + rng.Intn(2000),
			headers:      h,
			label:        "background",
		})
	}

	// 15% long-form generation (large output)
	for i := 0; i < 150; i++ {
		models := []string{"claude-3-opus", "claude-3-sonnet", "gpt-4"}
		reqs = append(reqs, simulatedRequest{
			model:        models[rng.Intn(len(models))],
			inputTokens:  2000 + rng.Intn(4000),
			outputTokens: 1000 + rng.Intn(3000),
			bodySize:     8000 + rng.Intn(16000),
			headers:      http.Header{},
			label:        "long-form",
		})
	}

	// 10% complex reasoning (orchestrator tag)
	for i := 0; i < 100; i++ {
		models := []string{"claude-3-opus", "gpt-4"}
		reqs = append(reqs, simulatedRequest{
			model:         models[rng.Intn(len(models))],
			inputTokens:   3000 + rng.Intn(5000),
			outputTokens:  1500 + rng.Intn(2500),
			bodySize:      12000 + rng.Intn(20000),
			headers:       http.Header{},
			systemMessage: "[ORCHESTRATOR] You are a task router.",
			label:         "complex",
		})
	}

	return reqs
}

func costUSD(model string, inputTokens, outputTokens int) float64 {
	p, ok := modelPrices[model]
	if !ok {
		return 0
	}
	return (p.Input*float64(inputTokens) + p.Output*float64(outputTokens)) / 1_000_000
}

func TestCostSimulation_RealisticTraffic(t *testing.T) {
	// Set up rule engine with realistic routing rules
	rules := []RoutingRule{
		// Short Q&A with small body -> small-model tier
		{Name: "small-body-small", Priority: 10, Condition: RouteCondition{MaxBodyBytes: 3000}, TargetModel: "claude-3-haiku", Tier: TierSmall},
		// Tool calls for file ops → smaller model
		{Name: "file-tools-small", Priority: 20, Condition: RouteCondition{ToolPattern: "^(list_files|file_search|grep|get_weather|calculator)$"}, TargetModel: "gpt-4o-mini", Tier: TierSmall},
		// Orchestrator tasks → deepseek (cheaper reasoning)
		{Name: "orchestrator-large", Priority: 30, Condition: RouteCondition{SystemTag: "[ORCHESTRATOR]"}, TargetModel: "deepseek-r1", Tier: TierLarge},
	}
	engine := NewRuleEngine(rules)

	// Model router for background task downgrade
	mr, _ := NewModelRouter(&ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{Name: "claude-3", Patterns: []string{"^claude-3.*"}, Provider: "anthropic", Fallback: "claude-3-haiku"},
			{Name: "gpt-4", Patterns: []string{"^gpt-4.*"}, Provider: "openai", Fallback: "gpt-4o-mini"},
		},
	})

	rng := rand.New(rand.NewSource(42)) // deterministic
	traffic := trafficMix(rng)

	var totalBefore, totalAfter float64
	categoryStats := map[string][2]float64{} // [before, after]
	routeHits := map[string]int{}

	for _, req := range traffic {
		before := costUSD(req.model, req.inputTokens, req.outputTokens)
		totalBefore += before

		// Simulate rule engine evaluation
		routeReq := &RouteRequest{
			Headers:       req.headers,
			BodySize:      req.bodySize,
			ToolNames:     req.tools,
			SystemMessage: req.systemMessage,
		}

		routedModel := req.model
		ruleName := "none"

		decision := engine.Evaluate(routeReq)
		if decision != nil && decision.Matched {
			routedModel = decision.Model
			ruleName = decision.Rule
		} else if mr != nil {
			// Fall back to model router for background tasks
			isBg := req.headers.Get("X-Background-Task") == "true"
			route, err := mr.RouteModel(req.model, isBg)
			if err == nil && route.TargetModel != req.model {
				routedModel = route.TargetModel
				ruleName = "model-router-bg"
			}
		}

		after := costUSD(routedModel, req.inputTokens, req.outputTokens)
		totalAfter += after
		routeHits[ruleName]++

		cat := categoryStats[req.label]
		cat[0] += before
		cat[1] += after
		categoryStats[req.label] = cat
	}

	savings := totalBefore - totalAfter
	pct := (savings / totalBefore) * 100

	t.Logf("\n=== Cost Simulation: 1000 Requests (Realistic Traffic Mix) ===")
	t.Logf("  Before routing:  $%.4f", totalBefore)
	t.Logf("  After routing:   $%.4f", totalAfter)
	t.Logf("  Savings:         $%.4f (%.1f%%)", savings, pct)
	t.Logf("")
	t.Logf("  By category:")
	for cat, costs := range categoryStats {
		catSavings := costs[0] - costs[1]
		catPct := float64(0)
		if costs[0] > 0 {
			catPct = (catSavings / costs[0]) * 100
		}
		t.Logf("    %-12s  $%.4f → $%.4f  (saved $%.4f, %.1f%%)", cat, costs[0], costs[1], catSavings, catPct)
	}
	t.Logf("")
	t.Logf("  Route hits:")
	for rule, count := range routeHits {
		t.Logf("    %-25s %d requests", rule, count)
	}
	t.Logf("")

	// Extrapolate to monthly (assume 1000 req/day)
	monthly := savings * 30
	t.Logf("  Monthly projection (1K req/day): saved $%.2f/month", monthly)
	t.Logf("  Annual projection:               saved $%.2f/year", monthly*12)

	// Sanity: routing should save money, not cost more
	if totalAfter > totalBefore {
		t.Error("routing should not increase cost")
	}
	if pct < 10 {
		t.Logf("  WARNING: savings below 10%% — consider tuning rules")
	}

	// Print per-1K-request cost for easy comparison
	t.Logf("")
	t.Logf("  Cost per 1K requests: $%.4f → $%.4f", totalBefore, totalAfter)
	fmt.Printf("\n  [RESULT] Routing saves %.1f%% ($%.4f per 1K requests)\n", pct, savings)
}
