package bootstrap

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

type stubRuntimeProxyModelCatalog struct {
	models []*providerpool.Model
}

func (s *stubRuntimeProxyModelCatalog) ListAvailableModels() []*providerpool.Model {
	return s.models
}

type stubRuntimeProxyProviderChanges struct {
	listener func(provider *providerpool.Provider, action string)
}

func (s *stubRuntimeProxyProviderChanges) AddProviderChangeListener(cb func(provider *providerpool.Provider, action string)) {
	s.listener = cb
}

type stubRuntimeProxyProviderRouter struct {
	failoverCallback func(*providerpool.FailoverResult)
	latencies        map[string]time.Duration
}

func (s *stubRuntimeProxyProviderRouter) SetFailoverCallback(cb func(*providerpool.FailoverResult)) {
	s.failoverCallback = cb
}

func (s *stubRuntimeProxyProviderRouter) UpdateLatency(providerID string, latency time.Duration) {
	if s.latencies == nil {
		s.latencies = make(map[string]time.Duration)
	}
	s.latencies[providerID] = latency
}

type stubRuntimeProxyProviderRegistry struct {
	enabled  []*providerpool.Provider
	onStatus func(providerID string, oldStatus, newStatus providerpool.ProviderStatus)
}

func (s *stubRuntimeProxyProviderRegistry) SetOnStatusChange(cb func(providerID string, oldStatus, newStatus providerpool.ProviderStatus)) {
	s.onStatus = cb
}

func (s *stubRuntimeProxyProviderRegistry) ListEnabled() []*providerpool.Provider {
	return s.enabled
}

func waitForRuntimeProxyCondition(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestResolveRuntimeProxyDatabaseRefs_PrefersSplitDBConn(t *testing.T) {
	writer := &sql.DB{}
	reader := &sql.DB{}
	fallback := &sql.DB{}

	refs := resolveRuntimeProxyDatabaseRefs(&Services{
		DBConn: &database.SQLiteConn{
			Writer: writer,
			Reader: reader,
		},
	}, fallback)

	if refs.writeDB != writer {
		t.Fatalf("write db=%p, want %p", refs.writeDB, writer)
	}
	if refs.readDB != reader {
		t.Fatalf("read db=%p, want %p", refs.readDB, reader)
	}
}

func TestRuntimeProxyBootstrapGo_DelegatesEntryAssembly(t *testing.T) {
	bootstrapContent, err := os.ReadFile(filepath.Join("runtime_proxy_bootstrap.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_bootstrap.go: %v", err)
	}
	bootstrapSource := string(bootstrapContent)

	builderContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime_builder.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime_builder.go: %v", err)
	}
	builderSource := string(builderContent)

	if lines := strings.Count(bootstrapSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_proxy_bootstrap.go to stay below 60 lines after extraction, got %d", lines)
	}

	requiredBootstrap := []string{
		"func activateRuntimeProxyRuntime(",
		"newRuntimeProxyRuntimeResult(",
		"newRuntimeProxyEntryOptions(",
	}
	for _, token := range requiredBootstrap {
		if !strings.Contains(bootstrapSource, token) {
			t.Fatalf("expected runtime_proxy_bootstrap.go to keep token %q", token)
		}
	}

	forbiddenBootstrap := []string{
		"type runtimeProxyDatabaseRefs struct {",
		"func resolveRuntimeProxyDatabaseRefs(",
	}
	for _, token := range forbiddenBootstrap {
		if strings.Contains(bootstrapSource, token) {
			t.Fatalf("expected runtime_proxy_bootstrap.go to delegate token %q", token)
		}
	}

	requiredBuilder := []string{
		"type runtimeProxyDatabaseRefs struct {",
		"func resolveRuntimeProxyDatabaseRefs(",
		"func newRuntimeProxyRuntimeResult(",
		"func newRuntimeProxyEntryOptions(",
	}
	for _, token := range requiredBuilder {
		if !strings.Contains(builderSource, token) {
			t.Fatalf("expected runtime_proxy_runtime_builder.go to contain token %q", token)
		}
	}
}

func TestActivateRuntimeProxyRuntime_WiresRuntimeBundle(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", tmp+"/proxy-runtime.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api")
	restrictions := e.Group("/api/v1")
	runtimeLLM := newRuntimeLLMProviderRef()
	deps := &RoutesDeps{
		DB: db,
		Config: &config.Config{
			Proxy: &proxy.ProxyConfig{
				Enabled: true,
				Route: &proxy.RouteConfig{
					LoadBalancing: "priority",
					Providers: []*proxy.ProviderConfig{
						{
							Name:     "primary",
							Enabled:  true,
							Priority: 1,
						},
					},
					Failover: proxy.DefaultProxyConfig().Routing.Failover,
				},
			},
		},
		ServerConfig: &ServerConfig{DataDir: tmp},
		Services:     &Services{},
		ConfigKV:     kvstore.NewMemoryStore(),
	}

	result := activateRuntimeProxyRuntime(runtimeProxyRuntimeOptions{
		e:                e,
		v1:               v1,
		protected:        protected,
		restrictionGroup: restrictions,
		deps:             deps,
		runtimeLLM:       runtimeLLM,
		logger:           zap.NewNop(),
	})

	if result.auxiliaryLLM == nil {
		t.Fatal("expected auxiliary llm caller to be initialized")
	}
	if result.dataMasker == nil {
		t.Fatal("expected data masker to be initialized")
	}
	if result.lane == nil || result.lane.handler == nil {
		t.Fatalf("expected proxy lane activation, got %#v", result.lane)
	}
	if result.agentLLMCaller != runtimeLLM {
		t.Fatalf("expected runtime llm caller to replace default, got %#v", result.agentLLMCaller)
	}
	if len(deps.Closers) != 1 || deps.Closers[0] != result.lane.pipelineStats {
		t.Fatalf("expected pipeline stats closer captured, got %#v", deps.Closers)
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/masking/stats") {
		t.Fatalf("expected masking routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/v1/chat/completions") {
		t.Fatalf("expected proxy gateway routes, got %#v", e.Routes())
	}
}

func TestRegisterRuntimeProxyMaskingRoutes_UpdatesRulesAndToggle(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	dataMasker := newRuntimeProxyDataMasker()
	toggleCalls := 0

	if !registerRuntimeProxyMaskingRoutes(runtimeProxyMaskingRoutesOptions{
		v1:         v1,
		dataMasker: dataMasker,
		onToggle: func() {
			toggleCalls++
		},
	}) {
		t.Fatal("expected masking routes to register")
	}

	if !routeExists(e, http.MethodGet, "/api/v1/proxy/masking/stats") {
		t.Fatalf("expected masking stats route, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodPut, "/api/v1/proxy/masking/toggle") {
		t.Fatalf("expected masking toggle route, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy/masking/toggle", strings.NewReader(`{"enabled":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT masking toggle status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !dataMasker.IsEnabled() {
		t.Fatal("expected masking to be enabled by route")
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy/masking/rules/email", strings.NewReader(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT masking rule status=%d body=%s", rec.Code, rec.Body.String())
	}
	rule, ok := dataMasker.GetRule("email")
	if !ok || rule.Enabled {
		t.Fatalf("expected email rule to be disabled, got %#v ok=%v", rule, ok)
	}
	if toggleCalls != 2 {
		t.Fatalf("toggle callback calls=%d, want 2", toggleCalls)
	}
}

func TestNewRuntimeProxyPrunerRuntime_RegistersRoutesAndCreatesLazyMiddleware(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	handler, _, _ := newRuntimeProxyControlTestHandler()
	cfg := pruner.DefaultConfig()

	runtime := newRuntimeProxyPrunerRuntime(runtimeProxyPrunerOptions{
		v1:            v1,
		dataDir:       t.TempDir(),
		initialConfig: &cfg,
		handler:       handler,
	})
	if runtime == nil || runtime.handler == nil || runtime.config == nil {
		t.Fatalf("expected pruner runtime bundle, got %#v", runtime)
	}
	if runtime.config.ModelDir != filepath.Join(filepath.Dir(runtime.config.ModelDir), "models") {
		t.Fatalf("expected pruner model dir to end with /models, got %q", runtime.config.ModelDir)
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/pruner/config") {
		t.Fatalf("expected pruner config route, got %#v", e.Routes())
	}

	mw := runtime.ensure()
	if mw == nil {
		t.Fatal("expected pruner middleware to be created lazily")
	}
	if runtime.current() != mw {
		t.Fatalf("expected current middleware getter to return created middleware, got %#v", runtime.current())
	}
}

func TestNewRuntimeProxyRoutingSetup_AppliesRoutingAndTracksProviderChanges(t *testing.T) {
	catalog := &stubRuntimeProxyModelCatalog{
		models: []*providerpool.Model{
			{ID: "gpt-4o", ProviderID: "openai", Enabled: true, InputPrice: 5, OutputPrice: 15},
			{ID: "gpt-4o-mini", ProviderID: "openai", Enabled: true, InputPrice: 0.15, OutputPrice: 0.6},
		},
	}
	changes := &stubRuntimeProxyProviderChanges{}
	setup := newRuntimeProxyRoutingSetup(runtimeProxyRoutingOptions{
		modelCatalog:    catalog,
		providerChanges: changes,
		modelRouterConfig: &proxy.ModelRouterConfig{
			Enabled: true,
			RegexCustomRules: []*proxy.RegexRule{
				{
					Pattern:     "^foo$",
					Target:      "bar",
					Priority:    1,
					Description: "foo -> bar",
				},
			},
		},
		ruleRoutingConfig: &proxy.RoutingConfig{
			Enabled: false,
			Rules: []proxy.RoutingRule{
				{
					Name:      "small-body",
					Priority:  1,
					Condition: proxy.RouteCondition{MaxBodyBytes: 10},
					Tier:      proxy.TierSmall,
				},
			},
		},
	})
	if setup == nil || setup.tierResolver == nil || setup.modelRouter == nil || setup.ruleEngine == nil {
		t.Fatalf("expected routing setup bundle, got %#v", setup)
	}

	waitForRuntimeProxyCondition(t, func() bool {
		return setup.tierResolver.BestModelForTier(proxy.TierSmall) == "gpt-4o-mini"
	})

	decision := setup.ruleEngine.Evaluate(&proxy.RouteRequest{BodySize: 1})
	if decision == nil || decision.Model != "gpt-4o-mini" {
		t.Fatalf("expected tier-based rule to resolve small model, got %#v", decision)
	}

	route, err := setup.modelRouter.RouteModel("foo", false)
	if err != nil {
		t.Fatalf("route model: %v", err)
	}
	if route.TargetModel != "bar" {
		t.Fatalf("expected regex model routing to apply, got %#v", route)
	}

	handler, _, _ := newRuntimeProxyControlTestHandler()
	if !setup.apply(handler) {
		t.Fatal("expected routing setup to apply to handler")
	}
	if handler.IsRoutingEnabled() {
		t.Fatal("expected handler routing toggle to follow setup config")
	}
	if enabled, ok := runtimeProxyControlRuleEnabled(handler.GetRoutingRules(), "small-body"); !ok || !enabled {
		t.Fatalf("expected handler rules to be wired, got %#v", handler.GetRoutingRules())
	}

	if changes.listener == nil {
		t.Fatal("expected provider change listener to be registered")
	}
	catalog.models = []*providerpool.Model{
		{ID: "gpt-4o", ProviderID: "openai", Enabled: true, InputPrice: 5, OutputPrice: 15},
		{ID: "o4-mini", ProviderID: "openai", Enabled: true, InputPrice: 0.2, OutputPrice: 0.8},
	}
	changes.listener(&providerpool.Provider{ID: "openai"}, "update")
	waitForRuntimeProxyCondition(t, func() bool {
		return setup.tierResolver.BestModelForTier(proxy.TierSmall) == "o4-mini"
	})
}

func TestRuntimeProxyWarmupBaseURLs_TrimsAndSkipsEmptyProviders(t *testing.T) {
	urls := runtimeProxyWarmupBaseURLs([]*providerpool.Provider{
		nil,
		{ID: "a", BaseURL: " https://a.example.com "},
		{ID: "b", BaseURL: ""},
		{ID: "c", BaseURL: "   "},
		{ID: "d", BaseURL: "http://d.example.com"},
	})

	if len(urls) != 2 {
		t.Fatalf("warmup urls=%#v, want 2 items", urls)
	}
	if urls[0] != "https://a.example.com" || urls[1] != "http://d.example.com" {
		t.Fatalf("warmup urls=%#v, want trimmed non-empty urls", urls)
	}
}

func TestBindRuntimeProxyProviderBindings_WiresCallbacksAndBroadcasts(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "provider-bindings.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	handler, routeCfg, failoverCfg := newRuntimeProxyControlTestHandler()
	smartFailover := proxy.NewSmartFailoverHandler(failoverCfg, proxy.NewRouter(routeCfg))
	pipelineStats := newRuntimeProxyPipelineStatsCollector(nil, db, db, handler, smartFailover, nil)
	if pipelineStats == nil {
		t.Fatal("expected pipeline stats collector")
	}
	defer pipelineStats.Close()

	router := &stubRuntimeProxyProviderRouter{}
	registry := &stubRuntimeProxyProviderRegistry{}
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	if !bindRuntimeProxyProviderBindings(runtimeProxyProviderBindingsOptions{
		handler:       handler,
		router:        router,
		registry:      registry,
		smartFailover: smartFailover,
		pipelineStats: pipelineStats,
		broker:        broker,
	}) {
		t.Fatal("expected provider bindings helper to apply")
	}
	if router.failoverCallback == nil {
		t.Fatal("expected failover callback to be wired")
	}
	if registry.onStatus == nil {
		t.Fatalf("expected registry callbacks to be wired, got %#v", registry)
	}

	now := time.Now()
	router.failoverCallback(&providerpool.FailoverResult{
		RequestID:       "req-1",
		StartTime:       now,
		EndTime:         now.Add(25 * time.Millisecond),
		TotalAttempts:   2,
		SuccessProvider: "provider-b",
		SuccessModel:    "gpt-4o",
		FailedAttempts: []*providerpool.FailoverRecord{
			{
				ProviderID: "provider-a",
				Reason:     providerpool.FailoverReasonTimeout,
				Latency:    10 * time.Millisecond,
			},
		},
	})

	stats := smartFailover.GetMetrics().GetStats()
	if total, _ := stats["failover_total"].(int64); total != 1 {
		t.Fatalf("smart failover total=%v, want 1", stats["failover_total"])
	}
	if snapshot := pipelineStats.Snapshot(); snapshot.Failover.Total != 1 || snapshot.Failover.Success != 1 {
		t.Fatalf("pipeline snapshot=%#v, want total=1 success=1", snapshot.Failover)
	}

	registry.onStatus("provider-a", providerpool.ProviderStatusInactive, providerpool.ProviderStatusActive)
	select {
	case event := <-sub:
		if event.Type != "provider_status_changed" {
			t.Fatalf("event type=%q, want provider_status_changed", event.Type)
		}
		data, ok := event.Data.(map[string]string)
		if !ok {
			t.Fatalf("event data type=%T, want map[string]string", event.Data)
		}
		if data["provider_id"] != "provider-a" || data["old_status"] != string(providerpool.ProviderStatusInactive) || data["status"] != string(providerpool.ProviderStatusActive) {
			t.Fatalf("event data=%#v, want provider status payload", data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for provider status broadcast")
	}
}
