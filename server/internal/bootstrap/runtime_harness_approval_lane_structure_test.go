package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeHarnessApprovalLane_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_harness_approval_lane.go": {
			maxLines: 30,
			tokens: []string{
				"func bindHarnessRuntimeApproval(",
				"newApprovalRuntimeBinding(",
				"newWorkflowHarnessDriverBinding(bundle).register(workflowTarget)",
			},
		},
		"runtime_harness_approval_lane_types.go": {
			maxLines: 30,
			tokens: []string{
				"type runtimeExecResolverTarget interface {",
				"type approvalRuntimeHandlerTarget interface {",
				"type approvalRuntimeDetailTarget interface {",
				"type approvalRuntimeBinding struct {",
			},
		},
		"runtime_harness_approval_lane_binding.go": {
			maxLines: 70,
			tokens: []string{
				"func newHarnessRuntimeExecApprovals(",
				"func newApprovalRuntimeBinding(",
				"func (binding approvalRuntimeBinding) applyDetail(",
				"func (binding approvalRuntimeBinding) applyHandler(",
				"func (binding approvalRuntimeBinding) applyApprover(",
				"func (binding approvalRuntimeBinding) registerWorkflow(",
			},
		},
		"runtime_harness_approval_lane_exec.go": {
			maxLines: 25,
			tokens: []string{
				"type execApprovalAdapter struct {",
				"func (a execApprovalAdapter) ResolveApproval(",
				"ApprovalResolveBindingMismatch",
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
