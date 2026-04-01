package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeProxyControlsGo_DelegatesTypesStatsToggleHooksAndRoutes(t *testing.T) {
	typeContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_types.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_types.go: %v", err)
	}
	typeSource := string(typeContent)

	statsContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_stats.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_stats.go: %v", err)
	}
	statsSource := string(statsContent)

	toggleContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_toggle.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_toggle.go: %v", err)
	}
	toggleSource := string(toggleContent)
	toggleStateContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_toggle_state.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_toggle_state.go: %v", err)
	}
	toggleStateSource := string(toggleStateContent)
	togglePersistenceContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_toggle_persistence.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_toggle_persistence.go: %v", err)
	}
	togglePersistenceSource := string(togglePersistenceContent)

	hooksContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_hooks.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_hooks.go: %v", err)
	}
	hooksSource := string(hooksContent)

	routesContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_routes.go: %v", err)
	}
	routesSource := string(routesContent)

	routingRouteContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_routes_routing.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_routes_routing.go: %v", err)
	}
	routingRouteSource := string(routingRouteContent)

	promptCacheRouteContent, err := os.ReadFile(filepath.Join("runtime_proxy_controls_routes_prompt_cache.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_controls_routes_prompt_cache.go: %v", err)
	}
	promptCacheRouteSource := string(promptCacheRouteContent)

	if lines := strings.Count(typeSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_proxy_controls_types.go to stay below 60 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(statsSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_proxy_controls_stats.go to stay below 60 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toggleSource, "\n") + 1; lines > 5 {
		t.Fatalf("expected runtime_proxy_controls_toggle.go to stay below 5 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toggleStateSource, "\n") + 1; lines > 120 {
		t.Fatalf("expected runtime_proxy_controls_toggle_state.go to stay below 120 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(togglePersistenceSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_proxy_controls_toggle_persistence.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(hooksSource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_proxy_controls_hooks.go to stay below 45 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(routesSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_proxy_controls_routes.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(routingRouteSource, "\n") + 1; lines > 75 {
		t.Fatalf("expected runtime_proxy_controls_routes_routing.go to stay below 75 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(promptCacheRouteSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_proxy_controls_routes_prompt_cache.go to stay below 35 lines after extraction, got %d", lines)
	}

	requiredTypes := []string{
		"type runtimeProxyTogglePersistence struct {",
		"type runtimeProxyControlRoutes struct {",
		"func (p runtimeProxyTogglePersistence) enabled(",
		"func (p runtimeProxyTogglePersistence) Save(",
		"func (p runtimeProxyTogglePersistence) SaveWithTimeout(",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_proxy_controls_types.go to contain token %q", token)
		}
	}

	if !strings.Contains(statsSource, "func newRuntimeProxyPipelineStatsCollector(") {
		t.Fatalf("expected runtime_proxy_controls_stats.go to contain newRuntimeProxyPipelineStatsCollector")
	}

	requiredToggleState := []string{
		"func newRuntimeProxyToggleState(",
		"func applyRuntimeProxyToggleState(",
	}
	for _, token := range requiredToggleState {
		if !strings.Contains(toggleStateSource, token) {
			t.Fatalf("expected runtime_proxy_controls_toggle_state.go to contain token %q", token)
		}
	}
	if !strings.Contains(togglePersistenceSource, "func newRuntimeProxyTogglePersistence(") {
		t.Fatalf("expected runtime_proxy_controls_toggle_persistence.go to contain newRuntimeProxyTogglePersistence")
	}

	if !strings.Contains(hooksSource, "func bindRuntimeProxyPersistenceHooks(") {
		t.Fatalf("expected runtime_proxy_controls_hooks.go to contain bindRuntimeProxyPersistenceHooks")
	}
	if !strings.Contains(routesSource, "func registerRuntimeProxyControlRoutes(") {
		t.Fatalf("expected runtime_proxy_controls_routes.go to contain registerRuntimeProxyControlRoutes")
	}
	if !strings.Contains(routingRouteSource, "func registerRuntimeProxyRoutingRoutes(") {
		t.Fatalf("expected runtime_proxy_controls_routes_routing.go to contain registerRuntimeProxyRoutingRoutes")
	}
	if !strings.Contains(promptCacheRouteSource, "func registerRuntimeProxyPromptCacheRoutes(") {
		t.Fatalf("expected runtime_proxy_controls_routes_prompt_cache.go to contain registerRuntimeProxyPromptCacheRoutes")
	}

	forbiddenTypes := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func newRuntimeProxyToggleState(",
		"func bindRuntimeProxyPersistenceHooks(",
		"func registerRuntimeProxyControlRoutes(",
	}
	for _, token := range forbiddenTypes {
		if strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_proxy_controls_types.go to delegate token %q", token)
		}
	}

	forbiddenStats := []string{
		"func newRuntimeProxyToggleState(",
		"func bindRuntimeProxyPersistenceHooks(",
		"func registerRuntimeProxyControlRoutes(",
	}
	for _, token := range forbiddenStats {
		if strings.Contains(statsSource, token) {
			t.Fatalf("expected runtime_proxy_controls_stats.go to delegate token %q", token)
		}
	}

	forbiddenToggle := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func bindRuntimeProxyPersistenceHooks(",
		"func registerRuntimeProxyControlRoutes(",
		"func newRuntimeProxyToggleState(",
		"func applyRuntimeProxyToggleState(",
		"func newRuntimeProxyTogglePersistence(",
	}
	for _, token := range forbiddenToggle {
		if strings.Contains(toggleSource, token) {
			t.Fatalf("expected runtime_proxy_controls_toggle.go to delegate token %q", token)
		}
	}

	forbiddenToggleState := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func newRuntimeProxyTogglePersistence(",
		"func bindRuntimeProxyPersistenceHooks(",
	}
	for _, token := range forbiddenToggleState {
		if strings.Contains(toggleStateSource, token) {
			t.Fatalf("expected runtime_proxy_controls_toggle_state.go to delegate token %q", token)
		}
	}

	forbiddenTogglePersistence := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func bindRuntimeProxyPersistenceHooks(",
		"func registerRuntimeProxyControlRoutes(",
	}
	for _, token := range forbiddenTogglePersistence {
		if strings.Contains(togglePersistenceSource, token) {
			t.Fatalf("expected runtime_proxy_controls_toggle_persistence.go to delegate token %q", token)
		}
	}

	forbiddenHooks := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func newRuntimeProxyToggleState(",
		"func registerRuntimeProxyControlRoutes(",
	}
	for _, token := range forbiddenHooks {
		if strings.Contains(hooksSource, token) {
			t.Fatalf("expected runtime_proxy_controls_hooks.go to delegate token %q", token)
		}
	}

	forbiddenRoutes := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func newRuntimeProxyToggleState(",
		"func bindRuntimeProxyPersistenceHooks(",
	}
	for _, token := range forbiddenRoutes {
		if strings.Contains(routesSource, token) {
			t.Fatalf("expected runtime_proxy_controls_routes.go to delegate token %q", token)
		}
	}

	forbiddenRoutingRoutes := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func newRuntimeProxyToggleState(",
		"func bindRuntimeProxyPersistenceHooks(",
		"func registerRuntimeProxyPromptCacheRoutes(",
	}
	for _, token := range forbiddenRoutingRoutes {
		if strings.Contains(routingRouteSource, token) {
			t.Fatalf("expected runtime_proxy_controls_routes_routing.go to delegate token %q", token)
		}
	}

	forbiddenPromptCacheRoutes := []string{
		"func newRuntimeProxyPipelineStatsCollector(",
		"func newRuntimeProxyToggleState(",
		"func bindRuntimeProxyPersistenceHooks(",
		"func registerRuntimeProxyRoutingRoutes(",
	}
	for _, token := range forbiddenPromptCacheRoutes {
		if strings.Contains(promptCacheRouteSource, token) {
			t.Fatalf("expected runtime_proxy_controls_routes_prompt_cache.go to delegate token %q", token)
		}
	}
}
