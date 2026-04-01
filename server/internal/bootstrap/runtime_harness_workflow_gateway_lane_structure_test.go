package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeHarnessWorkflowGatewayLane_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_harness_workflow_gateway_lane.go": {
			maxLines: 30,
			tokens: []string{
				"func newRuntimeToolGateway(",
				"tools.NewToolGateway(",
				"gateway.SetApprover(approver)",
			},
		},
		"runtime_harness_workflow_gateway_lane_types.go": {
			maxLines: 40,
			tokens: []string{
				"type runtimeToolGatewayMetricsRecorder interface {",
				"type workflowRuntimeApplyTarget interface {",
				"type workflowRuntimeHookTarget interface {",
				"type workflowRuntimeBinding struct {",
			},
		},
		"runtime_harness_workflow_gateway_lane_binding.go": {
			maxLines: 50,
			tokens: []string{
				"func newWorkflowRuntimeBinding(",
				"func (binding workflowRuntimeBinding) apply(",
				"func (binding workflowRuntimeBinding) register(",
			},
		},
		"runtime_harness_workflow_gateway_lane_adapters.go": {
			maxLines: 45,
			tokens: []string{
				"func (a workflowMetricsRecorderAdapter) RecordCounter(",
				"func (a workflowToolRuntimeAdapter) Execute(",
				"tools.WithRouteKind(ctx, tools.ToolRouteKindWorkflow)",
			},
		},
	}

	for name, expectation := range files {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		if lines := strings.Count(source, "\n") + 1; lines > expectation.maxLines {
			t.Fatalf("expected %s to stay below %d lines, got %d", name, expectation.maxLines, lines)
		}
		for _, token := range expectation.tokens {
			if !strings.Contains(source, token) {
				t.Fatalf("expected %s to contain token %q", name, token)
			}
		}
	}
}
