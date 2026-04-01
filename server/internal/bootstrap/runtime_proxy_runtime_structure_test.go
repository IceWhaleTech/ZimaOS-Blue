package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeProxyRuntimeGo_DelegatesFocusedRuntimeSlices(t *testing.T) {
	runtimeContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime.go: %v", err)
	}
	runtimeSource := string(runtimeContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime_types.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime_types.go: %v", err)
	}
	typeSource := string(typeContent)

	maskingContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime_masking.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime_masking.go: %v", err)
	}
	maskingSource := string(maskingContent)

	prunerContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime_pruner.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime_pruner.go: %v", err)
	}
	prunerSource := string(prunerContent)

	routingContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime_routing.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime_routing.go: %v", err)
	}
	routingSource := string(routingContent)

	providerContent, err := os.ReadFile(filepath.Join("runtime_proxy_runtime_provider.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_runtime_provider.go: %v", err)
	}
	providerSource := string(providerContent)

	if lines := strings.Count(runtimeSource, "\n") + 1; lines > 5 {
		t.Fatalf("expected runtime_proxy_runtime.go to stay below 5 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 90 {
		t.Fatalf("expected runtime_proxy_runtime_types.go to stay below 90 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(maskingSource, "\n") + 1; lines > 100 {
		t.Fatalf("expected runtime_proxy_runtime_masking.go to stay below 100 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(prunerSource, "\n") + 1; lines > 65 {
		t.Fatalf("expected runtime_proxy_runtime_pruner.go to stay below 65 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(routingSource, "\n") + 1; lines > 75 {
		t.Fatalf("expected runtime_proxy_runtime_routing.go to stay below 75 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(providerSource, "\n") + 1; lines > 105 {
		t.Fatalf("expected runtime_proxy_runtime_provider.go to stay below 105 lines after extraction, got %d", lines)
	}

	if !strings.Contains(typeSource, "type runtimeProxyRoutingOptions struct {") {
		t.Fatal("expected runtime_proxy_runtime_types.go to keep routing option types")
	}
	if !strings.Contains(maskingSource, "func registerRuntimeProxyMaskingRoutes(") {
		t.Fatal("expected runtime_proxy_runtime_masking.go to keep masking routes")
	}
	if !strings.Contains(prunerSource, "func newRuntimeProxyPrunerRuntime(") {
		t.Fatal("expected runtime_proxy_runtime_pruner.go to keep pruner runtime")
	}
	if !strings.Contains(routingSource, "func newRuntimeProxyRoutingSetup(") {
		t.Fatal("expected runtime_proxy_runtime_routing.go to keep routing setup")
	}
	if !strings.Contains(providerSource, "func bindRuntimeProxyProviderBindings(") {
		t.Fatal("expected runtime_proxy_runtime_provider.go to keep provider bindings")
	}

	forbiddenRuntimeTokens := []string{
		"func registerRuntimeProxyMaskingRoutes(",
		"func newRuntimeProxyPrunerRuntime(",
		"func newRuntimeProxyRoutingSetup(",
		"func bindRuntimeProxyProviderBindings(",
	}
	for _, token := range forbiddenRuntimeTokens {
		if strings.Contains(runtimeSource, token) {
			t.Fatalf("expected runtime_proxy_runtime.go to delegate token %q", token)
		}
	}
}

func TestRegisterRuntimeProxyMaskingRoutes_ReturnsFalseOnNilInputs(t *testing.T) {
	if registerRuntimeProxyMaskingRoutes(runtimeProxyMaskingRoutesOptions{}) {
		t.Fatal("expected masking route registration to reject nil inputs")
	}
}

func TestRuntimeProxyRoutingSetupApply_ReturnsFalseOnNilInputs(t *testing.T) {
	if (*runtimeProxyRoutingSetup)(nil).apply(nil) {
		t.Fatal("expected nil routing setup to reject nil handler")
	}
}

func TestBindRuntimeProxyProviderBindings_ReturnsFalseWithoutHandler(t *testing.T) {
	if bindRuntimeProxyProviderBindings(runtimeProxyProviderBindingsOptions{}) {
		t.Fatal("expected provider bindings helper to reject nil handler")
	}
}
