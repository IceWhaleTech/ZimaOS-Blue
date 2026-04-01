package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

func newRuntimeProxyControlTestHandler() (*proxy.ProxyHandler, *proxy.RouteConfig, *proxy.FailoverConfig) {
	routeCfg := &proxy.RouteConfig{
		LoadBalancing: "priority",
		Providers: []*proxy.ProviderConfig{
			{
				Name:     "primary",
				Enabled:  true,
				Priority: 1,
			},
		},
	}
	failoverCfg := proxy.DefaultProxyConfig().Routing.Failover
	router := proxy.NewRouter(routeCfg)
	failover := proxy.NewFailoverHandler(&failoverCfg, router)
	handler := proxy.NewProxyHandler(router, proxy.NewConnectionPool(nil), failover)
	handler.SetRuleEngine(proxy.DefaultRoutingConfig().ToRuleEngine())
	return handler, routeCfg, &failoverCfg
}

func newRuntimeProxyControlTestMasker(enabled bool, ruleEnabled bool) *proxy.DataMasker {
	return proxy.NewDataMasker(&proxy.MaskingConfig{
		Enabled: enabled,
		Rules: []*proxy.MaskingRule{
			{
				ID:          "secret",
				Name:        "Secret",
				Pattern:     "secret",
				Replacement: "[REDACTED]",
				Direction:   proxy.MaskingBoth,
				Enabled:     ruleEnabled,
			},
		},
	})
}

func runtimeProxyControlRuleEnabled(rules []proxy.RoutingRule, name string) (bool, bool) {
	for _, rule := range rules {
		if rule.Name != name || rule.Enabled == nil {
			continue
		}
		return *rule.Enabled, true
	}
	return false, false
}

func TestNewRuntimeProxyPipelineStatsCollector_FallbackDBWiresCollector(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "pipeline-stats.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	handler, routeCfg, failoverCfg := newRuntimeProxyControlTestHandler()
	smartFailover := proxy.NewSmartFailoverHandler(failoverCfg, proxy.NewRouter(routeCfg))
	closers := make([]interface{ Close() error }, 0, 1)

	collector := newRuntimeProxyPipelineStatsCollector(nil, db, db, handler, smartFailover, &closers)
	if collector == nil {
		t.Fatal("expected fallback collector to be created")
	}
	defer collector.Close()

	if handler.GetPipelineStats() != collector {
		t.Fatalf("expected handler pipeline stats to point to collector, got %#v", handler.GetPipelineStats())
	}
	if len(closers) != 1 || closers[0] != collector {
		t.Fatalf("expected collector appended to closers, got %#v", closers)
	}

	snapshot := collector.Snapshot()
	if snapshot.Failover.ByReason == nil {
		t.Fatalf("expected collector snapshot maps to be initialized, got %#v", snapshot)
	}
}

func TestApplyRuntimeProxyToggleState_MigratesLegacyV0(t *testing.T) {
	handler, _, _ := newRuntimeProxyControlTestHandler()
	handler.SetRoutingEnabled(false)
	handler.SetPromptCacheEnabled(false)

	masker := newRuntimeProxyControlTestMasker(true, true)
	prunerCfg := pruner.DefaultConfig()
	prunerCfg.Enabled = false

	ensureCalls := 0
	failoverCfg := proxy.DefaultProxyConfig().Routing.Failover
	saved := &proxy.ToggleState{
		Version:            0,
		PrunerEnabled:      true,
		PrunerBackend:      "remote",
		PromptCacheEnabled: false,
		RoutingEnabled:     false,
		MaskingEnabled:     true,
		MaskingRules: map[string]bool{
			"secret": false,
		},
		RoutingRules: map[string]bool{
			"small-body-small": false,
		},
		FailoverConfig: &proxy.FailoverConfig{
			ProviderRace: proxy.ProviderRaceConfig{
				Enabled:     true,
				MaxParallel: 2,
			},
		},
	}

	migrated := applyRuntimeProxyToggleState(
		saved,
		handler,
		masker,
		&prunerCfg,
		nil,
		func() { ensureCalls++ },
		&failoverCfg,
	)
	if !migrated {
		t.Fatal("expected legacy toggle state to be marked as migrated")
	}
	if saved.Version != 1 {
		t.Fatalf("saved version=%d, want 1", saved.Version)
	}
	if !handler.IsRoutingEnabled() {
		t.Fatal("expected routing to migrate to enabled")
	}
	if !handler.IsPromptCacheEnabled() {
		t.Fatal("expected prompt cache to migrate to enabled")
	}
	if masker.IsEnabled() {
		t.Fatal("expected masking to migrate to disabled")
	}
	if rule, ok := masker.GetRule("secret"); !ok || rule.Enabled {
		t.Fatalf("expected masking rule state restored, got %#v ok=%v", rule, ok)
	}
	if enabled, ok := runtimeProxyControlRuleEnabled(handler.GetRoutingRules(), "small-body-small"); !ok || enabled {
		t.Fatalf("expected routing rule override restored, got %#v", handler.GetRoutingRules())
	}
	if !prunerCfg.Enabled {
		t.Fatal("expected pruner config to restore as enabled")
	}
	if prunerCfg.Backend != "local" {
		t.Fatalf("expected pruner backend to normalize to local, got %q", prunerCfg.Backend)
	}
	if ensureCalls != 1 {
		t.Fatalf("ensure pruner factory calls=%d, want 1", ensureCalls)
	}
	if !failoverCfg.ProviderRace.Enabled || failoverCfg.ProviderRace.MaxParallel != 2 {
		t.Fatalf("expected failover config restore, got %#v", failoverCfg.ProviderRace)
	}
}

func TestNewRuntimeProxyTogglePersistence_RoundTripsState(t *testing.T) {
	store := kvstore.NewMemoryStore()

	handlerA, _, failoverCfgA := newRuntimeProxyControlTestHandler()
	handlerA.SetRoutingEnabled(false)
	handlerA.SetPromptCacheEnabled(false)
	if !handlerA.SetRoutingRuleEnabled("small-body-small", false) {
		t.Fatal("expected routing rule to exist for snapshot")
	}
	maskerA := newRuntimeProxyControlTestMasker(true, false)
	prunerCfgA := pruner.DefaultConfig()
	prunerCfgA.Enabled = true
	failoverCfgA.ProviderRace.Enabled = true
	failoverCfgA.ProviderRace.MaxParallel = 3

	persistenceA := newRuntimeProxyTogglePersistence(
		store,
		handlerA,
		maskerA,
		&prunerCfgA,
		nil,
		nil,
		failoverCfgA,
	)
	if err := persistenceA.Save(context.Background()); err != nil {
		t.Fatalf("save toggle state: %v", err)
	}

	handlerB, _, failoverCfgB := newRuntimeProxyControlTestHandler()
	handlerB.SetRoutingEnabled(true)
	handlerB.SetPromptCacheEnabled(true)
	maskerB := newRuntimeProxyControlTestMasker(false, true)
	prunerCfgB := pruner.DefaultConfig()
	prunerCfgB.Enabled = false
	ensureCalls := 0

	persistenceB := newRuntimeProxyTogglePersistence(
		store,
		handlerB,
		maskerB,
		&prunerCfgB,
		nil,
		func() { ensureCalls++ },
		failoverCfgB,
	)
	if !persistenceB.enabled() {
		t.Fatal("expected persistence to remain active after restore")
	}

	if handlerB.IsRoutingEnabled() {
		t.Fatal("expected routing state restored to disabled")
	}
	if handlerB.IsPromptCacheEnabled() {
		t.Fatal("expected prompt cache state restored to disabled")
	}
	if !maskerB.IsEnabled() {
		t.Fatal("expected masking state restored to enabled")
	}
	if rule, ok := maskerB.GetRule("secret"); !ok || rule.Enabled {
		t.Fatalf("expected masking rule state restored, got %#v ok=%v", rule, ok)
	}
	if enabled, ok := runtimeProxyControlRuleEnabled(handlerB.GetRoutingRules(), "small-body-small"); !ok || enabled {
		t.Fatalf("expected routing rule state restored, got %#v", handlerB.GetRoutingRules())
	}
	if !prunerCfgB.Enabled {
		t.Fatal("expected pruner enabled state restored")
	}
	if ensureCalls != 1 {
		t.Fatalf("ensure pruner factory calls=%d, want 1", ensureCalls)
	}
	if !failoverCfgB.ProviderRace.Enabled || failoverCfgB.ProviderRace.MaxParallel != 3 {
		t.Fatalf("expected failover config restored, got %#v", failoverCfgB.ProviderRace)
	}
}

func TestRegisterRuntimeProxyControlRoutes_RegistersAndPersists(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	handler, _, failoverCfg := newRuntimeProxyControlTestHandler()
	handler.SetPromptCacheEnabled(true)
	store := kvstore.NewMemoryStore()
	masker := newRuntimeProxyControlTestMasker(false, true)
	prunerCfg := pruner.DefaultConfig()
	persistence := newRuntimeProxyTogglePersistence(
		store,
		handler,
		masker,
		&prunerCfg,
		nil,
		nil,
		failoverCfg,
	)

	if !registerRuntimeProxyControlRoutes(runtimeProxyControlRoutes{
		v1:          v1,
		handler:     handler,
		persistence: persistence,
	}) {
		t.Fatal("expected proxy control routes to register")
	}

	if !routeExists(e, http.MethodGet, "/api/v1/proxy/routing/config") {
		t.Fatalf("expected routing config route, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/pipeline/stats") {
		t.Fatalf("expected pipeline stats route, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodPut, "/api/v1/proxy/prompt-cache/config") {
		t.Fatalf("expected prompt-cache route, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/routing/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET routing config status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy/routing/config", strings.NewReader(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT routing config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if handler.IsRoutingEnabled() {
		t.Fatal("expected routing to be disabled by route")
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy/routing/rules/small-body-small", strings.NewReader(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT routing rule status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/proxy/pipeline/stats", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET pipeline stats status=%d body=%s", rec.Code, rec.Body.String())
	}
	var pipelineBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &pipelineBody); err != nil {
		t.Fatalf("decode pipeline stats: %v", err)
	}
	if pipelineBody["status"] != "not configured" {
		t.Fatalf("pipeline stats=%#v, want not configured", pipelineBody)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy/prompt-cache/config", strings.NewReader(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT prompt cache config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if handler.IsPromptCacheEnabled() {
		t.Fatal("expected prompt cache to be disabled by route")
	}

	saved, err := persistence.store.Load(context.Background())
	if err != nil {
		t.Fatalf("load persisted toggle state: %v", err)
	}
	if saved == nil {
		t.Fatal("expected toggle state to be persisted")
	}
	if saved.RoutingEnabled {
		t.Fatalf("expected persisted routing disabled, got %#v", saved)
	}
	if saved.PromptCacheEnabled {
		t.Fatalf("expected persisted prompt cache disabled, got %#v", saved)
	}
	enabled, ok := saved.RoutingRules["small-body-small"]
	if !ok || enabled {
		t.Fatalf("expected persisted routing rule disabled, got %#v", saved.RoutingRules)
	}
}

func TestRegisterRuntimeProxyControlRoutes_RequiresV1AndHandler(t *testing.T) {
	if registerRuntimeProxyControlRoutes(runtimeProxyControlRoutes{}) {
		t.Fatal("expected route registration to fail without v1 group and handler")
	}

	e := echo.New()
	if registerRuntimeProxyControlRoutes(runtimeProxyControlRoutes{
		v1: e.Group("/api/v1"),
	}) {
		t.Fatal("expected route registration to fail without proxy handler")
	}
}
