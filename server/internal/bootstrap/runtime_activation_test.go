package bootstrap

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeActivationGo_DelegatesAssemblyHelpers(t *testing.T) {
	activationContent, err := os.ReadFile(filepath.Join("runtime_activation.go"))
	if err != nil {
		t.Fatalf("read runtime_activation.go: %v", err)
	}
	activationSource := string(activationContent)

	builderContent, err := os.ReadFile(filepath.Join("runtime_activation_builder.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_builder.go: %v", err)
	}
	builderSource := string(builderContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_activation_types.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(activationSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_activation.go to stay below 40 lines after extraction, got %d", lines)
	}

	requiredActivation := []string{
		"func activateRuntimeActivation(",
		"func activateRouteRuntimeActivation(",
		"newRuntimeProxyRuntimeOptions(",
		"newRuntimeActivationToolSurfacesOptions(",
		"newRuntimeActivationResult(",
		"newRuntimeActivationOptionsFromRoute(",
	}
	for _, token := range requiredActivation {
		if !strings.Contains(activationSource, token) {
			t.Fatalf("expected runtime_activation.go to keep token %q", token)
		}
	}

	forbiddenActivation := []string{
		"type runtimeActivationOptions struct {",
		"type runtimeActivationResult struct {",
		"type routeRuntimeActivationOptions struct {",
	}
	for _, token := range forbiddenActivation {
		if strings.Contains(activationSource, token) {
			t.Fatalf("expected runtime_activation.go to delegate token %q", token)
		}
	}

	requiredBuilder := []string{
		"func newRuntimeProxyRuntimeOptions(",
		"func newRuntimeActivationToolSurfacesOptions(",
		"func newRuntimeActivationResult(",
		"func newRuntimeActivationOptionsFromRoute(",
	}
	for _, token := range requiredBuilder {
		if !strings.Contains(builderSource, token) {
			t.Fatalf("expected runtime_activation_builder.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type runtimeActivationOptions struct {",
		"type runtimeActivationResult struct {",
		"type routeRuntimeActivationOptions struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_activation_types.go to contain token %q", token)
		}
	}
}

func TestActivateRuntimeActivation_WiresUnifiedRuntimeBundle(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-activation.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api/v1")
	restrictions := e.Group("/api/v1")
	runtimeLLM := newRuntimeLLMProviderRef()
	skills := skillpkg.NewRegistry()
	reflectSkill := builtin.NewSelfReflect()
	if err := skills.Register(reflectSkill, true); err != nil {
		t.Fatalf("register self_reflect skill: %v", err)
	}

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
		Services: &Services{
			ToolRegistry:  tools.NewRegistry(),
			SkillRegistry: skills,
		},
		ConfigKV:  kvstore.NewMemoryStore(),
		SSEBroker: sse.NewBroker(),
	}
	defer deps.SSEBroker.Close()

	result := activateRuntimeActivation(runtimeActivationOptions{
		e:                e,
		v1:               v1,
		protected:        protected,
		restrictionGroup: restrictions,
		services:         deps.Services,
		cfg:              deps.ServerConfig,
		deps:             deps,
		logger:           zap.NewNop(),
		runtimeLLM:       runtimeLLM,
		reflectService:   selfreflect.NewService(nil, nil),
		skillRegistry:    skills,
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
	if result.agentRunner == nil {
		t.Fatal("expected unified runtime activation to return agent runner")
	}
	defer result.agentRunner.Shutdown()
	if !result.mcpRegistered {
		t.Fatal("expected unified runtime activation to register MCP routes")
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/masking/stats") {
		t.Fatalf("expected masking routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/agent/tasks") {
		t.Fatalf("expected agent routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodPost, "/api/v1/mcp/message") {
		t.Fatalf("expected mcp routes, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/message?session_id=test-session", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/mcp/message status=%d body=%s", rec.Code, rec.Body.String())
	}

	execResult, execErr := reflectSkill.Execute(context.Background(), map[string]any{
		"goal":         "verify reflection binding",
		"final_status": "completed",
	})
	if execErr != nil {
		t.Fatalf("self_reflect skill execute error: %v", execErr)
	}
	if execResult == nil || !execResult.Success {
		t.Fatalf("expected self_reflect executor to be wired, got %#v", execResult)
	}
}

func TestActivateRouteRuntimeActivation_UsesRoutePermissionClosures(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "route-runtime-activation.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api/v1")
	runtimeLLM := newRuntimeLLMProviderRef()
	skills := skillpkg.NewRegistry()
	reflectSkill := builtin.NewSelfReflect()
	if err := skills.Register(reflectSkill, true); err != nil {
		t.Fatalf("register self_reflect skill: %v", err)
	}

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
		Services: &Services{
			ToolRegistry:  tools.NewRegistry(),
			SkillRegistry: skills,
		},
		ConfigKV:  kvstore.NewMemoryStore(),
		SSEBroker: sse.NewBroker(),
	}
	defer deps.SSEBroker.Close()

	var requestedPages []string
	var groupPages []string
	result := activateRouteRuntimeActivation(routeRuntimeActivationOptions{
		e:              e,
		v1:             v1,
		protected:      protected,
		deps:           deps,
		logger:         zap.NewNop(),
		runtimeLLM:     runtimeLLM,
		reflectService: selfreflect.NewService(nil, nil),
		requirePagePermission: func(page string) echo.MiddlewareFunc {
			requestedPages = append(requestedPages, page)
			return nil
		},
		authPageV1Group: func(page string) *echo.Group {
			groupPages = append(groupPages, page)
			return v1.Group("")
		},
	})

	if !slices.Contains(requestedPages, permission.PageProviders) {
		t.Fatalf("expected providers permission to be requested, got %#v", requestedPages)
	}
	if !slices.Contains(requestedPages, permission.PageSecurity) {
		t.Fatalf("expected security permission to be requested, got %#v", requestedPages)
	}
	if !slices.Contains(requestedPages, permission.PageTools) {
		t.Fatalf("expected tools permission to be requested, got %#v", requestedPages)
	}
	if len(groupPages) != 1 || groupPages[0] != permission.PageProviders {
		t.Fatalf("expected providers auth group to be requested once, got %#v", groupPages)
	}
	if result.dataMasker == nil || result.agentRunner == nil || !result.mcpRegistered {
		t.Fatalf("expected route runtime activation to wire runtime bundle, got %#v", result)
	}
	defer result.agentRunner.Shutdown()
	if result.agentLLMCaller != runtimeLLM {
		t.Fatalf("expected runtime llm caller to replace default, got %#v", result.agentLLMCaller)
	}
	if !routeExists(e, http.MethodGet, "/api/v1/agent/tasks") {
		t.Fatalf("expected agent routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/cache/stats") {
		t.Fatalf("expected proxy cache routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodPost, "/api/v1/mcp/message") {
		t.Fatalf("expected mcp routes, got %#v", e.Routes())
	}

	execResult, execErr := reflectSkill.Execute(context.Background(), map[string]any{
		"goal":         "verify reflection binding",
		"final_status": "completed",
	})
	if execErr != nil {
		t.Fatalf("self_reflect skill execute error: %v", execErr)
	}
	if execResult == nil || !execResult.Success {
		t.Fatalf("expected self_reflect executor to be wired, got %#v", execResult)
	}
}
