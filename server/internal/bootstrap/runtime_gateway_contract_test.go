package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeGatewayContractGo_DelegatesMethodsAndPluginSlices(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_gateway_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_gateway_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_gateway_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_gateway_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	methodContent, err := os.ReadFile(filepath.Join("runtime_gateway_contract_methods.go"))
	if err != nil {
		t.Fatalf("read runtime_gateway_contract_methods.go: %v", err)
	}
	methodSource := string(methodContent)

	pluginContent, err := os.ReadFile(filepath.Join("plugin_tools_test_helper_test.go"))
	if err != nil {
		t.Fatalf("read plugin_tools_test_helper_test.go: %v", err)
	}
	pluginSource := string(pluginContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_gateway_contract.go to stay below 45 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_gateway_contract_types.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(methodSource, "\n") + 1; lines > 150 {
		t.Fatalf("expected runtime_gateway_contract_methods.go to stay below 150 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(pluginSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected plugin_tools_test_helper_test.go to stay below 80 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindGatewayRuntime(",
		"func bindRouteRuntimeGateway(",
		"registerRouteRuntimeGatewayMethods(",
		"tools.RegisterGatewayTool(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_gateway_contract.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type routeRuntimeContractGatewayOptions struct {") {
		t.Fatal("expected runtime_gateway_contract_types.go to keep gateway option types")
	}
	if !strings.Contains(typeSource, "type routeRuntimeContractGatewayResult struct {") {
		t.Fatal("expected runtime_gateway_contract_types.go to keep gateway result types")
	}
	if !strings.Contains(methodSource, "func decodeRouteRuntimeGatewayPayload(") {
		t.Fatal("expected runtime_gateway_contract_methods.go to keep gateway payload helpers")
	}
	if !strings.Contains(pluginSource, "func registerPluginTools(") {
		t.Fatal("expected plugin_tools_test_helper_test.go to keep plugin tool registration")
	}

	forbiddenContract := []string{
		"type routeRuntimeContractGatewayOptions struct {",
		"func decodeRouteRuntimeGatewayPayload(",
		"type pluginToolAdapter struct {",
		"func registerPluginTools(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_gateway_contract.go to delegate token %q", token)
		}
	}
}

func TestDecodeRouteRuntimeGatewayPayload_RejectsInvalidInputs(t *testing.T) {
	if err := decodeRouteRuntimeGatewayPayload(nil, &struct{}{}); err == nil {
		t.Fatal("expected nil message to fail")
	}
	if err := decodeRouteRuntimeGatewayPayload(&gateway.Message{}, &struct{}{}); err == nil {
		t.Fatal("expected empty payload to fail")
	}
	if err := decodeRouteRuntimeGatewayPayload(&gateway.Message{Payload: []byte(`{}`)}, nil); err == nil {
		t.Fatal("expected nil target to fail")
	}
}

func TestBindRouteRuntimeGateway_ReturnsFirstClassRegistrationState(t *testing.T) {
	e := echo.New()
	gw := gateway.NewGateway(gateway.DefaultConfig(), zap.NewNop())
	handler := gateway.NewHandler(gw, zap.NewNop())
	registry := tools.NewRegistry()
	closers := make([]interface{ Close() error }, 0, 1)

	result := bindRouteRuntimeGateway(routeRuntimeContractGatewayOptions{
		gateway:      gw,
		handler:      handler,
		e:            e,
		protected:    e.Group("/api"),
		toolRegistry: registry,
		closers:      &closers,
	})

	if !result.toolRegistered || !result.methodsRegistered || !result.routesRegistered || !result.closerRegistered {
		t.Fatalf("expected gateway contract to report tool/method/route/closer registration, got %#v", result)
	}
	if registry.Get("gateway") == nil {
		t.Fatalf("expected gateway tool registration, got tools=%v", registry.List())
	}
	if len(closers) != 1 {
		t.Fatalf("expected gateway closer to be captured, got %d", len(closers))
	}
	if !routeExists(e, "GET", "/ws") || !routeExists(e, "GET", "/api/gateway/status") {
		t.Fatalf("expected gateway routes to register, got %#v", e.Routes())
	}
}

func TestBindRouteRuntimeGateway_ReturnsEmptyResultWithoutGateway(t *testing.T) {
	result := bindRouteRuntimeGateway(routeRuntimeContractGatewayOptions{})
	if result.toolRegistered || result.methodsRegistered || result.routesRegistered || result.closerRegistered {
		t.Fatalf("expected empty gateway result without gateway runtime, got %#v", result)
	}
}
