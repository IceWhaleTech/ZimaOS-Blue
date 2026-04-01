package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

func TestRuntimeFastPathContractGo_DelegatesStartupAuthAndBootstrapLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_fast_path_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_fast_path_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_fast_path_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_fast_path_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_fast_path_contract.go to stay below 60 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_fast_path_contract_types.go to stay below 40 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindStartupAuthRuntime(",
		"func bindRouteRuntimeStartupAuth(",
		"binding routeRuntimeFastPathBinding,",
		"binding.BindStartupSurfaceRuntime(",
		"binding.BindAuthSurfaceRuntime(",
		"binding.BindAccountSurfaceRuntime(",
		"func (binding *runtimeContractBinding) BindBootstrapPhaseRuntime(",
		"func bindRouteRuntimeBootstrapPhase(",
		"binding.BindShellSurfaceRuntime(",
		"binding.BindBootstrapSupportRuntime(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_fast_path_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeFastPathBinding interface {",
		"var _ routeRuntimeFastPathBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractStartupAuthOptions struct {",
		"type routeRuntimeContractStartupAuthResult struct {",
		"type routeRuntimeContractBootstrapPhaseOptions struct {",
		"type routeRuntimeContractBootstrapPhaseResult struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_fast_path_contract_types.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"binding *runtimeContractBinding,",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_fast_path_contract.go to delegate token %q", token)
		}
	}
}

func TestBindStartupAuthRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	contract := newRuntimeContractBinding(nil, nil, &config.Config{}, zap.NewNop(), nil, buildWebSearchConfig(&config.Config{}))
	if contract == nil {
		t.Fatal("expected runtime contract binding")
	}

	e := echo.New()
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	result := contract.BindStartupAuthRuntime(routeRuntimeContractStartupAuthOptions{
		startup: routeRuntimeContractStartupSurfaceOptions{
			e:           e,
			v1:          v1,
			userHandler: user.NewHandler(nil),
			logger:      zap.NewNop(),
		},
		auth: routeRuntimeContractAuthSurfaceOptions{
			v1:     v1,
			api:    api,
			logger: zap.NewNop(),
		},
		account: routeRuntimeContractAccountSurfaceOptions{
			v1:          v1,
			userHandler: user.NewHandler(nil),
			logger:      zap.NewNop(),
		},
	})

	if !result.startupBound || !result.accountBound {
		t.Fatalf("expected fast path startup/auth contract to bind startup and account lanes, got %#v", result)
	}
	if result.auth.protected == nil || result.auth.apiProtected == nil || result.auth.authPageV1Group == nil {
		t.Fatalf("expected fast path startup/auth contract to aggregate auth lane, got %#v", result.auth)
	}
	for _, route := range []struct {
		method string
		path   string
	}{
		{method: "POST", path: "/api/v1/auth/login"},
		{method: "GET", path: "/api/v1/users/me"},
	} {
		if !routeExists(e, route.method, route.path) {
			t.Fatalf("expected %s %s to be registered through startup/auth contract, got %#v", route.method, route.path, e.Routes())
		}
	}
}

func TestBindBootstrapPhaseRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	contract := newRuntimeContractBinding(nil, nil, &config.Config{}, zap.NewNop(), nil, buildWebSearchConfig(&config.Config{}))
	if contract == nil {
		t.Fatal("expected runtime contract binding")
	}

	e := echo.New()
	v1 := e.Group("/api/v1")
	apiProtected := e.Group("/api")
	calls := 0

	result := contract.BindBootstrapPhaseRuntime(routeRuntimeContractBootstrapPhaseOptions{
		dataDir: t.TempDir(),
		shell: routeRuntimeContractShellSurfaceOptions{
			e:            e,
			v1:           v1,
			serverConfig: &ServerConfig{},
			toolRegistry: tools.NewRegistry(),
			logger:       zap.NewNop(),
		},
		bootstrap: routeRuntimeContractBootstrapSupportOptions{
			apiProtected:    apiProtected,
			authPageV1Group: func(string) *echo.Group { return v1 },
			serverConfig:    &ServerConfig{},
			configKV:        kvstore.NewMemoryStore(),
			logger:          zap.NewNop(),
		},
		onEarlyReady: func() {
			calls++
		},
	})

	if !result.shellBound || !result.earlyReadyTriggered || calls != 1 {
		t.Fatalf("expected bootstrap phase contract to bind shell lane and trigger early-ready once, got %#v calls=%d", result, calls)
	}
	if result.bootstrap.settingsHandler == nil || result.bootstrap.smallModelManager == nil {
		t.Fatalf("expected bootstrap phase contract to aggregate bootstrap support, got %#v", result.bootstrap)
	}
	if !strings.HasSuffix(result.mediaDir, string(filepath.Separator)+"media") {
		t.Fatalf("expected bootstrap phase media dir to end with /media, got %q", result.mediaDir)
	}
	for _, route := range []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/api/v1/health"},
		{method: "GET", path: "/api/v1/settings"},
	} {
		if !routeExists(e, route.method, route.path) {
			t.Fatalf("expected %s %s to be registered through bootstrap phase contract, got %#v", route.method, route.path, e.Routes())
		}
	}
}
