package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeManagementRuntimeContractGo_DelegatesMgmtSupportUserAndChannelLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_management_runtime_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_management_runtime_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_management_runtime_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_management_runtime_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_management_runtime_contract.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_management_runtime_contract_types.go to stay below 35 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindManagementRuntime(",
		"func bindRouteRuntimeManagement(",
		"binding routeRuntimeManagementBinding,",
		"binding.RegisterMgmtTool(",
		"binding.BindManagementSupport(",
		"binding.BindUserSurfaceRuntime(",
		"binding.BindMgmtUpgrade(",
		"binding.BindChannelRuntime(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_management_runtime_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeManagementBinding interface {",
		"var _ routeRuntimeManagementBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractManagementRuntimeOptions struct {",
		"type routeRuntimeContractManagementRuntimeResult struct {",
		"mgmtTool",
		"support          routeRuntimeContractManagementSupportResult",
		"userSurfaceBound",
		"upgradeBound",
		"channelBound",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_management_runtime_contract_types.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractManagementRuntimeOptions struct {",
		"type routeRuntimeContractManagementRuntimeResult struct {",
		"binding *runtimeContractBinding,",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_management_runtime_contract.go to delegate token %q", token)
		}
	}
}

func TestBindManagementRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-management.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	registry := tools.NewRegistry()
	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api")
	api := e.Group("/api")
	workspaceHandler := &stubRouteRegistrar{
		register: func(g *echo.Group) {
			g.GET("/live", func(c echo.Context) error { return c.NoContent(204) })
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	contract := newRuntimeContractBinding(db, db, cfg, zap.NewNop(), nil, buildWebSearchConfig(cfg))
	if contract == nil {
		t.Fatal("expected runtime contract binding")
	}

	result := contract.BindManagementRuntime(routeRuntimeContractManagementRuntimeOptions{
		mgmt: routeRuntimeContractMgmtOptions{
			registry:     registry,
			workspaceDir: tmp,
			version:      "1.2.3",
		},
		support: routeRuntimeContractManagementSupportOptions{
			authPageV1Group: func(string) *echo.Group { return v1 },
			protected:       protected,
			config:          cfg,
			serverConfig: &ServerConfig{
				Version: "1.2.3",
				DataDir: tmp,
				Port:    8080,
			},
			ctx:      ctx,
			logger:   zap.NewNop(),
			configKV: kvstore.NewMemoryStore(),
		},
		user: routeRuntimeContractUserSurfaceOptions{
			v1:        v1,
			protected: protected,
			writeDB:   db,
			readDB:    db,
			workspace: workspaceHandler,
			logger:    zap.NewNop(),
		},
		channel: routeRuntimeContractChannelOptions{
			api:    api,
			config: cfg,
			logger: zap.NewNop(),
		},
	})

	if result.mgmtTool == nil || registry.Get("config") != result.mgmtTool {
		t.Fatalf("expected management runtime to register config tool, got tool=%#v registered=%#v", result.mgmtTool, registry.Get("config"))
	}
	if result.support.updateHandler == nil || result.support.otaChecker == nil || result.support.providerSettings == nil {
		t.Fatalf("expected management runtime to aggregate management support, got %#v", result.support)
	}
	if !result.userSurfaceBound || !result.upgradeBound || !result.channelBound {
		t.Fatalf("expected management runtime to bind user/upgrade/channel lanes, got %#v", result)
	}

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/api/v1/system/update/info"},
		{method: "GET", path: "/api/providers/settings"},
		{method: "GET", path: "/api/workspace/live"},
		{method: "GET", path: "/api/my/usage"},
		{method: "GET", path: "/api/channels"},
	} {
		if !routeExists(e, route.method, route.path) {
			t.Fatalf("expected %s %s to be registered through management runtime, got %#v", route.method, route.path, e.Routes())
		}
	}
}
