package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeHarnessAgentFeatureLaneGo_DelegatesBindingAndSupportSlices(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_harness_agent_feature_lane.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_agent_feature_lane.go: %v", err)
	}
	mainSource := string(mainContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_harness_agent_feature_types.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_agent_feature_types.go: %v", err)
	}
	typeSource := string(typeContent)

	bindingContent, err := os.ReadFile(filepath.Join("runtime_harness_agent_feature_binding.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_agent_feature_binding.go: %v", err)
	}
	bindingSource := string(bindingContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_harness_agent_feature_support.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_agent_feature_support.go: %v", err)
	}
	supportSource := string(supportContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 65 {
		t.Fatalf("expected runtime_harness_agent_feature_lane.go to stay below 65 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_harness_agent_feature_types.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(bindingSource, "\n") + 1; lines > 90 {
		t.Fatalf("expected runtime_harness_agent_feature_binding.go to stay below 90 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_harness_agent_feature_support.go to stay below 60 lines after extraction, got %d", lines)
	}

	requiredMain := []string{
		"func registerHarnessRuntimeAgentFeature(",
		"func registerHarnessRuntimeAgentFeatureWithReadDB(",
		"newHarnessRuntimeAgentStore(",
		"bindHarnessRuntimeToAgentRunner(",
		"registerHarnessRuntimeAgentDisabled(",
	}
	for _, token := range requiredMain {
		if !strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_harness_agent_feature_lane.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type agentRuntimeBinding struct {") {
		t.Fatal("expected runtime_harness_agent_feature_types.go to keep agent runtime binding type")
	}
	if !strings.Contains(bindingSource, "func newAgentRuntimeBinding(") {
		t.Fatal("expected runtime_harness_agent_feature_binding.go to keep agent runtime binding wiring")
	}
	if !strings.Contains(supportSource, "func startHarnessRuntimeAgentRecovery(") {
		t.Fatal("expected runtime_harness_agent_feature_support.go to keep agent recovery support")
	}

	forbiddenMain := []string{
		"type agentRuntimeBinding struct {",
		"func newAgentRuntimeBinding(",
		"func startHarnessRuntimeAgentRecovery(",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_harness_agent_feature_lane.go to delegate token %q", token)
		}
	}
}
