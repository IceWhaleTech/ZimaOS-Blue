package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHarnessWorkflowExecution_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_harness_workflow_execution.go": {
			maxLines: 75,
			tokens: []string{
				"type harnessWorkflowExecutionLauncher struct",
				"type harnessWorkflowNodesService struct",
				"func bindRuntimeWorkflowExecution(",
				"func newHarnessWorkflowExecutionLauncher(",
			},
		},
		"runtime_harness_workflow_launcher.go": {
			maxLines: 120,
			tokens: []string{
				"func (l *harnessWorkflowExecutionLauncher) LaunchExecution(",
				"func (l *harnessWorkflowExecutionLauncher) CancelExecution(",
				"func (l *harnessWorkflowExecutionLauncher) ResumeExecution(",
			},
		},
		"runtime_harness_workflow_nodes.go": {
			maxLines: 105,
			tokens: []string{
				"func (s harnessWorkflowNodesService) ListWorkflows(",
				"func (s harnessWorkflowNodesService) ExecuteWorkflow(",
				"func (s harnessWorkflowNodesService) Templates(",
			},
		},
		"runtime_harness_workflow_execution_helpers.go": {
			maxLines: 130,
			tokens: []string{
				"func workflowConversationID(",
				"func workflowExecutionFromRun(",
				"func workflowExecutionStatusFromRun(",
				"func firstNonEmptyWorkflowString(",
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
