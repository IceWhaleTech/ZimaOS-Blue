package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeExecSupport_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_exec_support.go": {
			maxLines: 25,
			tokens: []string{
				"func newRuntimeConvertSupport(",
				"newRuntimeConvertSupportWithReadDB(",
			},
		},
		"runtime_exec_support_convert.go": {
			maxLines: 50,
			tokens: []string{
				"func newRuntimeConvertSupportWithReadDB(",
				"func newRuntimeConvertConversationAuthorizer(",
				"convertsvc.NewHandler(service, newRuntimeConvertConversationAuthorizer(memoryStore))",
			},
		},
		"runtime_exec_support_skill_exec.go": {
			maxLines: 40,
			tokens: []string{
				"func newRuntimeExecSkillExecutor(",
				"resolveRuntimeSkillForExecution(",
			},
		},
		"runtime_exec_support_selector.go": {
			maxLines: 60,
			tokens: []string{
				"func newRuntimeExecSkillSelector(",
				"func newRuntimeExecSelectionOptions(",
			},
		},
		"runtime_exec_support_binding.go": {
			maxLines: 35,
			tokens: []string{
				"func bindRuntimeExecTool(",
				"target.SetPinnedSkills(agentcore.PinnedSkills())",
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
