package bootstrap

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func TestActivateRuntimeProxyEntry_ProxyDisabledPreservesDefaultCaller(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	defaultCaller := &fakeAgentLLMCaller{}

	result := activateRuntimeProxyEntry(runtimeProxyEntryOptions{
		v1:                 v1,
		maskingAuth:        nil,
		maskingPage:        nil,
		appConfig:          &config.Config{},
		defaultAgentCaller: defaultCaller,
		logger:             zap.NewNop(),
	})

	if result.lane != nil {
		t.Fatalf("expected no proxy lane when proxy disabled, got %#v", result.lane)
	}
	if result.agentLLMCaller != defaultCaller {
		t.Fatalf("expected default agent caller to be preserved, got %#v", result.agentLLMCaller)
	}
	if result.dataMasker == nil {
		t.Fatal("expected masking data masker to be initialized")
	}
	if result.maskingOnToggle != nil {
		t.Fatalf("expected no masking toggle callback, got %T", result.maskingOnToggle)
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/masking/stats") {
		t.Fatalf("expected masking routes to stay available, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy/masking/toggle", strings.NewReader(`{"enabled":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT masking toggle status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !result.dataMasker.IsEnabled() {
		t.Fatal("expected masking toggle route to update data masker")
	}
}

func TestRuntimeProxyEntryGo_DelegatesMaskingAndLaneAssembly(t *testing.T) {
	entryContent, err := os.ReadFile(filepath.Join("runtime_proxy_entry.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_entry.go: %v", err)
	}
	entrySource := string(entryContent)

	bindingContent, err := os.ReadFile(filepath.Join("runtime_proxy_entry_binding.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_entry_binding.go: %v", err)
	}
	bindingSource := string(bindingContent)

	if lines := strings.Count(entrySource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_proxy_entry.go to stay below 80 lines after extraction, got %d", lines)
	}

	requiredEntry := []string{
		"func activateRuntimeProxyEntry(",
		"newRuntimeProxyEntryMaskingBinding(",
		"newRuntimeProxyLaneOptions(",
	}
	for _, token := range requiredEntry {
		if !strings.Contains(entrySource, token) {
			t.Fatalf("expected runtime_proxy_entry.go to keep token %q", token)
		}
	}

	forbiddenEntry := []string{
		"registerRuntimeProxyMaskingRoutes(",
		"func resolveRuntimeProxyRouteConfig(",
		"func newRuntimeProxyLaneOptions(",
	}
	for _, token := range forbiddenEntry {
		if strings.Contains(entrySource, token) {
			t.Fatalf("expected runtime_proxy_entry.go to delegate token %q", token)
		}
	}

	requiredBinding := []string{
		"func newRuntimeProxyEntryMaskingBinding(",
		"func resolveRuntimeProxyRouteConfig(",
		"func newRuntimeProxyLaneOptions(",
	}
	for _, token := range requiredBinding {
		if !strings.Contains(bindingSource, token) {
			t.Fatalf("expected runtime_proxy_entry_binding.go to contain token %q", token)
		}
	}
}

func TestActivateRuntimeProxyEntry_ActivatesLaneAndBindsRuntimeProvider(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", tmp+"/proxy-entry.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api")
	restrictions := e.Group("/api/v1")
	closers := make([]interface{ Close() error }, 0, 1)
	appCfg := &config.Config{
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
	}
	runtimeLLM := newRuntimeLLMProviderRef()
	defaultCaller := &fakeAgentLLMCaller{}

	result := activateRuntimeProxyEntry(runtimeProxyEntryOptions{
		e:                  e,
		v1:                 v1,
		protected:          protected,
		restrictionGroup:   restrictions,
		failoverGuard:      nil,
		authMiddleware:     nil,
		pageMiddleware:     nil,
		maskingAuth:        nil,
		maskingPage:        nil,
		appConfig:          appCfg,
		dataDir:            tmp,
		kv:                 kvstore.NewMemoryStore(),
		prunerConfig:       nil,
		fallbackWriteDB:    db,
		fallbackReadDB:     db,
		closers:            &closers,
		runtimeLLM:         runtimeLLM,
		auxiliaryLLM:       newAuxiliaryLLMCaller(),
		defaultAgentCaller: defaultCaller,
		logger:             zap.NewNop(),
	})

	if result.lane == nil || result.lane.handler == nil {
		t.Fatalf("expected proxy lane activation, got %#v", result)
	}
	if result.dataMasker == nil {
		t.Fatal("expected masking data masker to be initialized")
	}
	if result.agentLLMCaller != runtimeLLM {
		t.Fatalf("expected runtime llm caller to replace default, got %#v", result.agentLLMCaller)
	}
	if result.maskingOnToggle == nil {
		t.Fatal("expected masking toggle callback from proxy lane")
	}
	if runtimeLLM.currentProvider() == nil {
		t.Fatal("expected runtime provider to be bound via proxy bridge")
	}
	if len(closers) != 1 || closers[0] != result.lane.pipelineStats {
		t.Fatalf("expected pipeline stats closer captured, got %#v", closers)
	}
	if !routeExists(e, http.MethodGet, "/v1/chat/completions") {
		t.Fatalf("expected proxy gateway routes, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy/masking/toggle", strings.NewReader(`{"enabled":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT masking toggle status=%d body=%s", rec.Code, rec.Body.String())
	}
	saved, err := result.lane.persistence.store.Load(context.Background())
	if err != nil {
		t.Fatalf("load persisted toggle state: %v", err)
	}
	if saved == nil || !saved.MaskingEnabled {
		t.Fatalf("expected masking toggle persistence to be wired, got %#v", saved)
	}
}
