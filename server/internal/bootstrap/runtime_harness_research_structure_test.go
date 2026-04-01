package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeHarnessResearchLaneGo_DelegatesRouteAndSupportSlices(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_harness_research_lane.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_research_lane.go: %v", err)
	}
	mainSource := string(mainContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_harness_research_types.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_research_types.go: %v", err)
	}
	typeSource := string(typeContent)

	routeContent, err := os.ReadFile(filepath.Join("runtime_harness_research_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_research_routes.go: %v", err)
	}
	routeSource := string(routeContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_harness_research_support.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_research_support.go: %v", err)
	}
	supportSource := string(supportContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_harness_research_lane.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_harness_research_types.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(routeSource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_harness_research_routes.go to stay below 45 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_harness_research_support.go to stay below 35 lines after extraction, got %d", lines)
	}

	requiredMain := []string{
		"func registerDeepResearchRuntimeRoutes(",
		"func newHarnessRuntimeAutoHarnessTurnHook(",
		"bindHarnessRuntimeToDeepResearchHandler(",
		"registerDeepResearchRouteGroups(",
	}
	for _, token := range requiredMain {
		if !strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_harness_research_lane.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type deepResearchRuntimeRouteTarget interface {") {
		t.Fatal("expected runtime_harness_research_types.go to keep research route target")
	}
	if !strings.Contains(routeSource, "func registerHarnessResearchCapabilityRoutes(") {
		t.Fatal("expected runtime_harness_research_routes.go to keep research capability route alias")
	}
	if !strings.Contains(supportSource, "func bindHarnessRuntimeJudgeEvaluator(") {
		t.Fatal("expected runtime_harness_research_support.go to keep judge evaluator binding")
	}

	forbiddenMain := []string{
		"type deepResearchRuntimeRouteTarget interface {",
		"func bindHarnessRuntimeToDeepResearchHandler(",
		"func bindHarnessRuntimeJudgeEvaluator(",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_harness_research_lane.go to delegate token %q", token)
		}
	}
}
