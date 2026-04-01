package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeInfrastructureContractGo_DelegatesTLSMediaGatewayAndChatSurfaceLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_infrastructure_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_infrastructure_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_infrastructure_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_infrastructure_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_infrastructure_contract.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_infrastructure_contract_types.go to stay below 30 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindInfrastructureRuntime(",
		"func bindRouteRuntimeInfrastructure(",
		"binding routeRuntimeInfrastructureBinding,",
		"binding.BindTLSRuntime(",
		"binding.BindMediaRuntime(",
		"binding.BindGatewayRuntime(",
		"binding.BindChatSurfaceRuntime(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_infrastructure_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeInfrastructureBinding interface {",
		"var _ routeRuntimeInfrastructureBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractInfrastructureOptions struct {",
		"type routeRuntimeContractInfrastructureResult struct {",
		"tlsConfigured",
		"media            routeRuntimeContractMediaResult",
		"gateway          routeRuntimeContractGatewayResult",
		"chatSurfaceBound",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_infrastructure_contract_types.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractInfrastructureOptions struct {",
		"type routeRuntimeContractInfrastructureResult struct {",
		"binding *runtimeContractBinding,",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_infrastructure_contract.go to delegate token %q", token)
		}
	}
}

func TestBindInfrastructureRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api")
	cfg := &config.Config{}
	registry := tools.NewRegistry()
	chatHandler := &serverpkg.ChatHandler{}
	broker := sse.NewBroker()
	defer broker.Close()

	contract := newRuntimeContractBinding(nil, nil, cfg, zap.NewNop(), nil, buildWebSearchConfig(cfg))
	gw := gateway.NewGateway(gateway.DefaultConfig(), zap.NewNop())
	handler := gateway.NewHandler(gw, zap.NewNop())
	closers := make([]interface{ Close() error }, 0, 1)

	result := contract.BindInfrastructureRuntime(routeRuntimeContractInfrastructureOptions{
		tls: routeRuntimeContractTLSOptions{
			e:       e,
			config:  cfg,
			dataDir: t.TempDir(),
			logger:  zap.NewNop(),
		},
		gateway: routeRuntimeContractGatewayOptions{
			gateway:      gw,
			handler:      handler,
			e:            e,
			protected:    protected,
			toolRegistry: registry,
			closers:      &closers,
		},
		chatSurface: routeRuntimeContractChatSurfaceOptions{
			v1:          v1,
			chatHandler: chatHandler,
			sseBroker:   broker,
		},
	})

	if !result.tlsConfigured {
		t.Fatalf("expected infrastructure contract to configure TLS lane, got %#v", result)
	}
	if result.media.ipcServer != nil {
		t.Fatalf("expected infrastructure contract to leave media lane empty without media deps, got %#v", result.media)
	}
	if !result.gateway.toolRegistered || !result.gateway.methodsRegistered || !result.gateway.routesRegistered || !result.gateway.closerRegistered {
		t.Fatalf("expected infrastructure contract to aggregate gateway state, got %#v", result.gateway)
	}
	if !result.chatSurfaceBound {
		t.Fatalf("expected infrastructure contract to bind chat surface, got %#v", result)
	}
	if registry.Get("gateway") == nil {
		t.Fatalf("expected infrastructure contract to register gateway tool, got %#v", registry.List())
	}
	if len(closers) != 1 {
		t.Fatalf("expected infrastructure contract to capture gateway closer, got %d", len(closers))
	}
	if !routeExists(e, "GET", "/ws") || !routeExists(e, "GET", "/api/gateway/status") {
		t.Fatalf("expected infrastructure contract to register gateway routes, got %#v", e.Routes())
	}
	if !routeExists(e, "POST", "/api/v1/conversations") || !routeExists(e, "GET", "/api/v1/tools") {
		t.Fatalf("expected infrastructure contract to register chat surface routes, got %#v", e.Routes())
	}
}
