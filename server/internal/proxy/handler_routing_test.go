package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tidwall/gjson"
)

func TestApplyModelRouting_RuleEngineSwapsModel(t *testing.T) {
	ph := &ProxyHandler{}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "small-body-haiku",
			Priority:    1,
			Condition:   RouteCondition{MaxBodyBytes: 1000},
			TargetModel: "claude-3-haiku",
			Tier:        TierEconomy,
		},
	}))

	body := []byte(`{"model":"claude-3-opus","messages":[{"role":"user","content":"hi"}]}`)
	pr := &parsedRequest{body: body, model: "claude-3-opus"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	ph.applyModelRouting(r, pr)

	if pr.model != "claude-3-haiku" {
		t.Errorf("expected model 'claude-3-haiku', got %q", pr.model)
	}
	if pr.routed == nil {
		t.Fatal("expected routed decision to be set")
	}
	if pr.routed.Rule != "small-body-haiku" {
		t.Errorf("expected rule 'small-body-haiku', got %q", pr.routed.Rule)
	}
	// Verify body was rewritten
	if got := gjson.GetBytes(pr.body, "model").Str; got != "claude-3-haiku" {
		t.Errorf("expected body model 'claude-3-haiku', got %q", got)
	}
}

func TestApplyModelRouting_NoMatchPassthrough(t *testing.T) {
	ph := &ProxyHandler{}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "tool-only",
			Priority:    1,
			Condition:   RouteCondition{ToolPattern: "^calculator$"},
			TargetModel: "gpt-4o-mini",
		},
	}))

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	pr := &parsedRequest{body: body, model: "gpt-4"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	ph.applyModelRouting(r, pr)

	if pr.model != "gpt-4" {
		t.Errorf("expected model unchanged 'gpt-4', got %q", pr.model)
	}
	if pr.routed != nil {
		t.Error("expected routed to be nil when no rule matches")
	}
}

func TestApplyModelRouting_ModelRouterBackgroundDowngrade(t *testing.T) {
	mr, _ := NewModelRouter(&ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
		},
	})

	ph := &ProxyHandler{}
	ph.SetModelRouter(mr)

	body := []byte(`{"model":"claude-3-opus","messages":[]}`)
	pr := &parsedRequest{body: body, model: "claude-3-opus"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("X-Background-Task", "true")

	ph.applyModelRouting(r, pr)

	if pr.model != "claude-3-haiku" {
		t.Errorf("expected background downgrade to 'claude-3-haiku', got %q", pr.model)
	}
}

func TestApplyModelRouting_RuleEngineTakesPriority(t *testing.T) {
	mr, _ := NewModelRouter(&ModelRouterConfig{
		Enabled: true,
		Families: []*ModelFamily{
			{
				Name:     "claude-3",
				Patterns: []string{"^claude-3.*"},
				Provider: "anthropic",
				Fallback: "claude-3-haiku",
			},
		},
	})

	ph := &ProxyHandler{}
	ph.SetModelRouter(mr)
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "header-economy",
			Priority:    1,
			Condition:   RouteCondition{Header: "X-Tier", HeaderValue: "economy"},
			TargetModel: "claude-3-5-haiku",
			Tier:        TierEconomy,
		},
	}))

	body := []byte(`{"model":"claude-3-opus","messages":[]}`)
	pr := &parsedRequest{body: body, model: "claude-3-opus"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("X-Tier", "economy")
	r.Header.Set("X-Background-Task", "true") // would trigger model router too

	ph.applyModelRouting(r, pr)

	// Rule engine should win over model router
	if pr.model != "claude-3-5-haiku" {
		t.Errorf("expected rule engine to win with 'claude-3-5-haiku', got %q", pr.model)
	}
	if pr.routed == nil || pr.routed.Rule != "header-economy" {
		t.Error("expected routed decision from rule engine")
	}
}

func TestApplyModelRouting_EmptyModel(t *testing.T) {
	ph := &ProxyHandler{}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{Name: "catch-all", Priority: 1, Condition: RouteCondition{MaxBodyBytes: 999999}, TargetModel: "haiku"},
	}))

	pr := &parsedRequest{body: []byte(`{}`), model: ""}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	ph.applyModelRouting(r, pr)

	if pr.model != "" {
		t.Errorf("expected empty model to be unchanged, got %q", pr.model)
	}
}

func TestApplyModelRouting_LazyToolExtraction(t *testing.T) {
	ph := &ProxyHandler{}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "tool-match",
			Priority:    1,
			Condition:   RouteCondition{ToolPattern: "^get_weather$"},
			TargetModel: "gpt-4o-mini",
			Tier:        TierEconomy,
		},
	}))

	body := []byte(`{"model":"gpt-4","tools":[{"type":"function","function":{"name":"get_weather"}}]}`)
	pr := &parsedRequest{body: body, model: "gpt-4"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	ph.applyModelRouting(r, pr)

	if pr.model != "gpt-4o-mini" {
		t.Errorf("expected tool match to route to 'gpt-4o-mini', got %q", pr.model)
	}
}

func TestSetRouteHeaders(t *testing.T) {
	ph := &ProxyHandler{}
	w := httptest.NewRecorder()

	// No routing — no headers
	pr := &parsedRequest{model: "gpt-4"}
	ph.setRouteHeaders(w, pr)
	if w.Header().Get("X-Route-Rule") != "" {
		t.Error("expected no route headers when not routed")
	}

	// With routing
	w = httptest.NewRecorder()
	pr.routed = &RouteDecision{
		Matched: true,
		Model:   "haiku",
		Rule:    "test-rule",
		Tier:    TierEconomy,
	}
	ph.setRouteHeaders(w, pr)
	if w.Header().Get("X-Route-Rule") != "test-rule" {
		t.Errorf("expected X-Route-Rule 'test-rule', got %q", w.Header().Get("X-Route-Rule"))
	}
	if w.Header().Get("X-Route-Model") != "haiku" {
		t.Errorf("expected X-Route-Model 'haiku', got %q", w.Header().Get("X-Route-Model"))
	}
	if w.Header().Get("X-Route-Tier") != "economy" {
		t.Errorf("expected X-Route-Tier 'economy', got %q", w.Header().Get("X-Route-Tier"))
	}
}
