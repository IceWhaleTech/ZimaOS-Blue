package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePlatformSurfaceContractGo_DelegatesBillingMetricsNetworkSystemAndPluginLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_platform_surface_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_platform_surface_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_platform_surface_types.go"))
	if err != nil {
		t.Fatalf("read runtime_platform_surface_types.go: %v", err)
	}
	typeSource := string(typeContent)

	billingMetricsContent, err := os.ReadFile(filepath.Join("runtime_platform_surface_billing_metrics.go"))
	if err != nil {
		t.Fatalf("read runtime_platform_surface_billing_metrics.go: %v", err)
	}
	billingMetricsSource := string(billingMetricsContent)

	networkSystemContent, err := os.ReadFile(filepath.Join("runtime_platform_surface_network_system.go"))
	if err != nil {
		t.Fatalf("read runtime_platform_surface_network_system.go: %v", err)
	}
	networkSystemSource := string(networkSystemContent)

	pluginContent, err := os.ReadFile(filepath.Join("runtime_platform_surface_plugin.go"))
	if err != nil {
		t.Fatalf("read runtime_platform_surface_plugin.go: %v", err)
	}
	pluginSource := string(pluginContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_platform_surface_contract.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_platform_surface_types.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(billingMetricsSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_platform_surface_billing_metrics.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(networkSystemSource, "\n") + 1; lines > 95 {
		t.Fatalf("expected runtime_platform_surface_network_system.go to stay below 95 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(pluginSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_platform_surface_plugin.go to stay below 30 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindPlatformSurfaceRuntime(",
		"func bindRouteRuntimePlatformSurfaces(",
		"registerRouteRuntimeBillingSurface(",
		"registerRouteRuntimeNetworkSurface(",
		"registerRouteRuntimeMetricsSurface(",
		"registerRouteRuntimeSystemSurface(",
		"registerRouteRuntimePluginSurface(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_platform_surface_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeContractPlatformSurfaceOptions struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_platform_surface_types.go to contain token %q", token)
		}
	}

	requiredBillingMetrics := []string{
		"func registerRouteRuntimeBillingSurface(",
		"func registerRouteRuntimeMetricsSurface(",
	}
	for _, token := range requiredBillingMetrics {
		if !strings.Contains(billingMetricsSource, token) {
			t.Fatalf("expected runtime_platform_surface_billing_metrics.go to contain token %q", token)
		}
	}

	requiredNetworkSystem := []string{
		"func registerRouteRuntimeNetworkSurface(",
		"func registerRouteRuntimeSystemSurface(",
		"func routeRuntimeResolvedPort(",
	}
	for _, token := range requiredNetworkSystem {
		if !strings.Contains(networkSystemSource, token) {
			t.Fatalf("expected runtime_platform_surface_network_system.go to contain token %q", token)
		}
	}

	requiredPlugin := []string{
		"func registerRouteRuntimePluginSurface(",
	}
	for _, token := range requiredPlugin {
		if !strings.Contains(pluginSource, token) {
			t.Fatalf("expected runtime_platform_surface_plugin.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractPlatformSurfaceOptions struct {",
		"func registerRouteRuntimeBillingSurface(",
		"func routeRuntimeResolvedPort(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_platform_surface_contract.go to delegate heavy implementation via token %q", token)
		}
	}

	forbiddenTypes := []string{
		"func registerRouteRuntimeBillingSurface(",
		"func registerRouteRuntimeNetworkSurface(",
	}
	for _, token := range forbiddenTypes {
		if strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_platform_surface_types.go to delegate token %q", token)
		}
	}

	forbiddenBillingMetrics := []string{
		"func registerRouteRuntimeNetworkSurface(",
		"func registerRouteRuntimeSystemSurface(",
		"func registerRouteRuntimePluginSurface(",
	}
	for _, token := range forbiddenBillingMetrics {
		if strings.Contains(billingMetricsSource, token) {
			t.Fatalf("expected runtime_platform_surface_billing_metrics.go to delegate token %q", token)
		}
	}

	forbiddenNetworkSystem := []string{
		"func registerRouteRuntimeBillingSurface(",
		"func registerRouteRuntimeMetricsSurface(",
		"func registerRouteRuntimePluginSurface(",
	}
	for _, token := range forbiddenNetworkSystem {
		if strings.Contains(networkSystemSource, token) {
			t.Fatalf("expected runtime_platform_surface_network_system.go to delegate token %q", token)
		}
	}

	forbiddenPlugin := []string{
		"func registerRouteRuntimeBillingSurface(",
		"func registerRouteRuntimeNetworkSurface(",
		"func registerRouteRuntimeMetricsSurface(",
	}
	for _, token := range forbiddenPlugin {
		if strings.Contains(pluginSource, token) {
			t.Fatalf("expected runtime_platform_surface_plugin.go to delegate token %q", token)
		}
	}
}

func TestRouteRuntimeResolvedPort_PrefersPrimaryThenFallsBack(t *testing.T) {
	if got := routeRuntimeResolvedPort(8080, 3000); got != 8080 {
		t.Fatalf("routeRuntimeResolvedPort(8080, 3000) = %d, want 8080", got)
	}
	if got := routeRuntimeResolvedPort(0, 3000); got != 3000 {
		t.Fatalf("routeRuntimeResolvedPort(0, 3000) = %d, want 3000", got)
	}
}
