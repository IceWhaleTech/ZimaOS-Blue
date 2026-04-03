package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/tidwall/gjson"
)

func newRoutingTestProviderPool(t *testing.T, providerID string, modelIDs ...string) *providerpool.Pool {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "proxy-routing-pool-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	provider := &providerpool.Provider{
		ID:        providerID,
		Name:      providerID,
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   "https://example.invalid",
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  1,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	models := make([]*providerpool.Model, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		models = append(models, &providerpool.Model{
			ID:           modelID,
			Name:         modelID,
			ProviderID:   providerID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true},
		})
	}
	if err := storage.SaveModels(providerID, models); err != nil {
		t.Fatalf("failed to save models: %v", err)
	}
	router.RebuildCandidates()

	return &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
		Storage:   storage,
	}
}

func TestApplyModelRouting_RuleEngineSwapsModel(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "small-body-haiku",
			Priority:    1,
			Condition:   RouteCondition{MaxBodyBytes: 1000},
			TargetModel: "claude-3-haiku",
			Tier:        TierSmall,
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
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
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

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
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

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetModelRouter(mr)
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "header-small",
			Priority:    1,
			Condition:   RouteCondition{Header: "X-Tier", HeaderValue: "small"},
			TargetModel: "claude-3-5-haiku",
			Tier:        TierSmall,
		},
	}))

	body := []byte(`{"model":"claude-3-opus","messages":[]}`)
	pr := &parsedRequest{body: body, model: "claude-3-opus"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("X-Tier", "small")
	r.Header.Set("X-Background-Task", "true") // would trigger model router too

	ph.applyModelRouting(r, pr)

	// Rule engine should win over model router
	if pr.model != "claude-3-5-haiku" {
		t.Errorf("expected rule engine to win with 'claude-3-5-haiku', got %q", pr.model)
	}
	if pr.routed == nil || pr.routed.Rule != "header-small" {
		t.Error("expected routed decision from rule engine")
	}
}

func TestApplyModelRouting_EmptyModel(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
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

func TestApplyModelRouting_DisabledInContextPreservesExplicitModel(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "small-body-haiku",
			Priority:    1,
			Condition:   RouteCondition{MaxBodyBytes: 4096},
			TargetModel: "claude-3-5-haiku",
		},
	}))

	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"verify the task record"}]}`)
	pr := &parsedRequest{
		body:           body,
		model:          "claude-sonnet-4-6",
		requestedModel: "claude-sonnet-4-6",
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r = r.WithContext(WithDisableModelRouting(context.Background()))

	ph.applyModelRouting(r, pr)

	if pr.model != "claude-sonnet-4-6" {
		t.Fatalf("expected explicit model to stay unchanged, got %q", pr.model)
	}
	if pr.routed != nil {
		t.Fatalf("expected no routed decision when model routing is disabled, got %#v", pr.routed)
	}
	if got := gjson.GetBytes(pr.body, "model").Str; got != "claude-sonnet-4-6" {
		t.Fatalf("expected body model to stay claude-sonnet-4-6, got %q", got)
	}
}

func TestApplyModelRouting_EmptyModelBackgroundUsesTierSmall(t *testing.T) {
	mr, _ := NewModelRouter(DefaultModelRouterConfig())
	tr := NewTierResolver()
	if !tr.Resolve([]*providerpool.Model{
		{
			ID:          "claude-opus-4-6",
			ProviderID:  "p1",
			Enabled:     true,
			InputPrice:  15.0,
			OutputPrice: 75.0,
		},
		{
			ID:          "claude-haiku-4-5",
			ProviderID:  "p1",
			Enabled:     true,
			InputPrice:  0.25,
			OutputPrice: 1.25,
		},
	}) {
		t.Fatal("expected tier resolver to be enabled with both small and large tiers")
	}
	mr.SetTierResolver(tr)

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetModelRouter(mr)

	body := []byte(`{"model":"auto","messages":[]}`)
	pr := &parsedRequest{body: body, model: "", requestedModel: "auto"}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set("X-Background-Task", "true")

	ph.applyModelRouting(r, pr)

	if pr.model != "claude-haiku-4-5" {
		t.Fatalf("expected background auto to downgrade to small tier model, got %q", pr.model)
	}
	if got := gjson.GetBytes(pr.body, "model").Str; got != "claude-haiku-4-5" {
		t.Fatalf("expected request body model rewrite to small tier model, got %q", got)
	}
}

func TestApplyModelRouting_LazyToolExtraction(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "tool-match",
			Priority:    1,
			Condition:   RouteCondition{ToolPattern: "^get_weather$"},
			TargetModel: "gpt-4o-mini",
			Tier:        TierSmall,
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

func TestApplyModelRouting_PinnedExplicitModelSkipsTierRewrite(t *testing.T) {
	pool := newRoutingTestProviderPool(t, "relay-openai", "gpt-5.4", "claude-sonnet-4.6", "glm-5")
	models, err := pool.Discovery.GetFilteredModels("relay-openai")
	if err != nil {
		t.Fatalf("failed to load models: %v", err)
	}

	tr := NewTierResolver()
	tr.Resolve(models)

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetProviderPool(pool)
	ph.SetRuleEngine(DefaultRoutingConfig().ToRuleEngine(tr))
	ph.SetTierResolver(tr)

	for _, requestedModel := range []string{"gpt-5.4", "claude-sonnet-4.6"} {
		t.Run(requestedModel, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"grep"}}]}`, requestedModel))
			pr := &parsedRequest{body: body, model: requestedModel, requestedModel: requestedModel}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req = req.WithContext(WithPinnedProvider(req.Context(), "relay-openai"))

			ph.applyModelRouting(req, pr)

			if pr.model != requestedModel {
				t.Fatalf("expected pinned explicit model to be preserved, got %q", pr.model)
			}
			if got := gjson.GetBytes(pr.body, "model").Str; got != requestedModel {
				t.Fatalf("expected body model %q, got %q", requestedModel, got)
			}
			if pr.routed != nil {
				t.Fatalf("expected no routed decision when preserving pinned explicit model, got %+v", pr.routed)
			}
		})
	}
}

func TestApplyModelRouting_UnpinnedExplicitModelStillAllowsTierRewrite(t *testing.T) {
	pool := newRoutingTestProviderPool(t, "relay-openai", "gpt-5.4", "glm-5")
	models, err := pool.Discovery.GetFilteredModels("relay-openai")
	if err != nil {
		t.Fatalf("failed to load models: %v", err)
	}

	tr := NewTierResolver()
	tr.Resolve(models)

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetProviderPool(pool)
	ph.SetRuleEngine(DefaultRoutingConfig().ToRuleEngine(tr))
	ph.SetTierResolver(tr)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"grep"}}]}`)
	pr := &parsedRequest{body: body, model: "gpt-5.4", requestedModel: "gpt-5.4"}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	ph.applyModelRouting(req, pr)

	if pr.model != "glm-5" {
		t.Fatalf("expected unpinned request to still rewrite via tier rule, got %q", pr.model)
	}
	if got := gjson.GetBytes(pr.body, "model").Str; got != "glm-5" {
		t.Fatalf("expected body model rewrite to glm-5, got %q", got)
	}
	if pr.routed == nil || pr.routed.Rule != "small-body-small" {
		t.Fatalf("expected small-body-small route decision, got %+v", pr.routed)
	}
}

func TestApplyModelRouting_TargetAvailabilityDynamicProviderChanges(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-routing-dynamic-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	baseProvider := &providerpool.Provider{
		ID: "base-provider", Name: "Base", Type: providerpool.ProviderTypeCustom,
		Enabled: true, Status: providerpool.ProviderStatusActive, Priority: 50,
		Location: providerpool.ProviderLocationCloud,
		APIKeys:  []providerpool.APIKey{{ID: "k1", Key: "key-base", Enabled: true}},
	}
	if err := registry.Register(baseProvider); err != nil {
		t.Fatalf("register base provider failed: %v", err)
	}
	if err := storage.SaveModels(baseProvider.ID, []*providerpool.Model{{
		ID: "gpt-4", ProviderID: baseProvider.ID, Name: "gpt-4", Enabled: true,
	}}); err != nil {
		t.Fatalf("save base models failed: %v", err)
	}
	router.RebuildCandidates()

	ph := NewProxyHandler(nil, nil, nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	}
	ph.SetRuleEngine(NewRuleEngine([]RoutingRule{
		{
			Name:        "always-to-spark",
			Priority:    1,
			Condition:   RouteCondition{MaxBodyBytes: 99999},
			TargetModel: "gpt-5.3-codex-spark",
		},
	}))

	// 1) Target model unavailable => keep requested model fixed.
	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.applyModelRouting(r, pr)
	if pr.model != "gpt-4" {
		t.Fatalf("expected model to stay fixed at gpt-4 when target unavailable, got %q", pr.model)
	}
	if got := gjson.GetBytes(pr.body, "model").Str; got != "gpt-4" {
		t.Fatalf("expected body model gpt-4 when target unavailable, got %q", got)
	}

	// 2) Dynamically add provider that has target model => switch should take effect.
	dynamicProvider := &providerpool.Provider{
		ID: "dynamic-provider", Name: "Dynamic", Type: providerpool.ProviderTypeCustom,
		Enabled: true, Status: providerpool.ProviderStatusActive, Priority: 120,
		Location: providerpool.ProviderLocationCloud,
		APIKeys:  []providerpool.APIKey{{ID: "k2", Key: "key-dynamic", Enabled: true}},
	}
	if err := storage.SaveModels(dynamicProvider.ID, []*providerpool.Model{{
		ID: "gpt-5.3-codex-spark", ProviderID: dynamicProvider.ID, Name: "gpt-5.3-codex-spark", Enabled: true,
	}}); err != nil {
		t.Fatalf("save dynamic models failed: %v", err)
	}
	if err := registry.Register(dynamicProvider); err != nil {
		t.Fatalf("register dynamic provider failed: %v", err)
	}
	router.RebuildCandidates()

	pr2 := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}
	ph.applyModelRouting(r, pr2)
	if pr2.model != "gpt-5.3-codex-spark" {
		t.Fatalf("expected switch to gpt-5.3-codex-spark after dynamic provider add, got %q", pr2.model)
	}
	if got := gjson.GetBytes(pr2.body, "model").Str; got != "gpt-5.3-codex-spark" {
		t.Fatalf("expected rewritten body model gpt-5.3-codex-spark, got %q", got)
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
		Tier:    TierSmall,
	}
	ph.setRouteHeaders(w, pr)
	if w.Header().Get("X-Route-Rule") != "test-rule" {
		t.Errorf("expected X-Route-Rule 'test-rule', got %q", w.Header().Get("X-Route-Rule"))
	}
	if w.Header().Get("X-Route-Model") != "haiku" {
		t.Errorf("expected X-Route-Model 'haiku', got %q", w.Header().Get("X-Route-Model"))
	}
	if w.Header().Get("X-Route-Tier") != "small" {
		t.Errorf("expected X-Route-Tier 'small', got %q", w.Header().Get("X-Route-Tier"))
	}
}
