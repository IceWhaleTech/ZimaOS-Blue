package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeResearchContractGo_DelegatesTypesThroughResearchSurface(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_research_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_research_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	compatContent, err := os.ReadFile(filepath.Join("runtime_research_contract_compat.go"))
	if err != nil {
		t.Fatalf("read runtime_research_contract_compat.go: %v", err)
	}
	compatSource := string(compatContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_research_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_research_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	bindingContent, err := os.ReadFile(filepath.Join("runtime_research_contract_binding.go"))
	if err != nil {
		t.Fatalf("read runtime_research_contract_binding.go: %v", err)
	}
	bindingSource := string(bindingContent)

	surfaceContent, err := os.ReadFile(filepath.Join("runtime_research_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_research_surface.go: %v", err)
	}
	surfaceSource := string(surfaceContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_research_contract.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(compatSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_research_contract_compat.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_research_contract_types.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(bindingSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_research_contract_binding.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_research_surface.go to stay below 30 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func bindRouteRuntimeResearchSurface(",
		"surface.HarnessRuntime()",
		"surface.ResearchService()",
		"newChatResearchRuntimeBinding(",
		"applyChatResearchRuntimeBinding(binding, chatResearchRuntimeBindingTargets{",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_research_contract.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func bindRouteRuntimeResearch(",
		"newRouteRuntimeResearchSurface(",
	} {
		if !strings.Contains(compatSource, token) {
			t.Fatalf("expected runtime_research_contract_compat.go to contain token %q", token)
		}
	}

	requiredSurface := []string{
		"type routeRuntimeResearchSurface interface {",
		"type routeRuntimeResearchAdapter struct {",
		"func newRouteRuntimeResearchSurface(",
		"func (adapter routeRuntimeResearchAdapter) HarnessRuntime(",
		"func (adapter routeRuntimeResearchAdapter) ResearchService(",
	}
	for _, token := range requiredSurface {
		if !strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_research_surface.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeContractResearchOptions struct {",
		"type routeRuntimeContractResearchResult struct {",
		"serviceBound",
		"driverRegistered",
		"toolRegistered",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_research_contract_types.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"type chatResearchRuntimeBindingTargets struct {",
		"func applyChatResearchRuntimeBinding(",
		"routeRuntimeContractResearchResult{",
		"toolRegistered:      targets.registry != nil && binding.toolAdapter != nil,",
	} {
		if !strings.Contains(bindingSource, token) {
			t.Fatalf("expected runtime_research_contract_binding.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractResearchOptions struct {",
		"type routeRuntimeContractResearchResult struct {",
		"type routeRuntimeResearchSurface interface {",
		"func bindRouteRuntimeResearch(",
		"func (binding *runtimeContractBinding) BindResearchRuntime(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_research_contract.go to delegate token %q", token)
		}
	}

	for _, token := range []string{
		"func bindRouteRuntimeResearchSurface(",
	} {
		if strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_research_surface.go to stay focused on research surface token %q", token)
		}
	}

	for _, token := range []string{
		"func bindRouteRuntimeResearchSurface(",
		"type routeRuntimeResearchSurface interface {",
	} {
		if strings.Contains(compatSource, token) {
			t.Fatalf("expected runtime_research_contract_compat.go to stay focused on raw-args compatibility wrapper token %q", token)
		}
	}
}

func TestBindRouteRuntimeResearch_ReturnsFirstClassLaneState(t *testing.T) {
	tmp := t.TempDir()
	_, bundle, _, _ := newTestAgentRuntimeFixture(t)
	target := &stubChatResearchRuntimeTarget{}
	registry := tools.NewRegistry()
	broker := sse.NewBroker()
	defer broker.Close()

	result := bindRouteRuntimeResearch(bundle, deepresearch.NewService(nil, nil), routeRuntimeContractResearchOptions{
		target:       target,
		broker:       broker,
		registry:     registry,
		workspaceDir: tmp,
	})

	if !result.serviceBound || result.turnHookBound || !result.eventPublisherBound || !result.driverRegistered || !result.toolRegistered {
		t.Fatalf("expected research contract to report bound service/publisher/driver/tool state without direct turn hook, got %#v", result)
	}
	if target.service == nil || target.hook != nil || target.hookCalls != 0 {
		t.Fatalf("expected research contract to wire service without direct turn hook, got %#v", target)
	}
	if registry.Get("deep_research") == nil {
		t.Fatalf("expected research contract to register deep_research tool, got %#v", registry.List())
	}
}

func TestBindRouteRuntimeResearch_ReturnsEmptyStateWithoutResearchService(t *testing.T) {
	result := bindRouteRuntimeResearch(nil, nil, routeRuntimeContractResearchOptions{})
	if result.serviceBound || result.turnHookBound || result.eventPublisherBound || result.driverRegistered || result.toolRegistered {
		t.Fatalf("expected empty research result without research runtime, got %#v", result)
	}
}

func TestBindRouteRuntimeResearchSurface_ReturnsEmptyStateWithoutSurface(t *testing.T) {
	result := bindRouteRuntimeResearchSurface(nil, routeRuntimeContractResearchOptions{})
	if result.serviceBound || result.turnHookBound || result.eventPublisherBound || result.driverRegistered || result.toolRegistered {
		t.Fatalf("expected empty research result without research surface, got %#v", result)
	}
}
