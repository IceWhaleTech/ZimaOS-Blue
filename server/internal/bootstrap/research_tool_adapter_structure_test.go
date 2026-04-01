package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResearchAdapters_AreSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"research_harness_adapter.go": {
			maxLines: 40,
			tokens: []string{
				"type harnessResearchCreator struct",
				"func newHarnessResearchCreator(",
				"func (a *harnessResearchCreator) CreateJob(",
			},
		},
		"research_harness_submit.go": {
			maxLines: 110,
			tokens: []string{
				"type researchHarnessRunSubmitter interface {",
				"type researchHarnessJobStore interface {",
				"func submitResearchHarnessJob(",
				"func normalizeResearchHarnessSubmitter(",
				"func synthesizeResearchJobFromRun(",
			},
		},
		"research_tool_adapter.go": {
			maxLines: 40,
			tokens: []string{
				"type deepResearchToolAdapter struct",
				"func newDeepResearchToolAdapter(",
				"func (a *deepResearchToolAdapter) GetJobForUser(",
			},
		},
		"research_tool_adapter_create.go": {
			maxLines: 45,
			tokens: []string{
				"func (a *deepResearchToolAdapter) CreateJob(",
				"func toDeepResearchCreateJobRequest(",
				"submitResearchHarnessJob(ctx, a.manager, a.service",
			},
		},
		"research_tool_adapter_convert.go": {
			maxLines: 75,
			tokens: []string{
				"func toDeepResearchBudget(",
				"func toToolResearchJob(",
				"func cloneCalibrationForTool(",
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

func TestResearchAdapters_MainFilesStayThin(t *testing.T) {
	mainFiles := map[string][]string{
		"research_harness_adapter.go": {
			"func submitResearchHarnessJob(",
			"func synthesizeResearchJobFromRun(",
			"func researchHarnessRunInputFromJobRequest(",
		},
		"research_tool_adapter.go": {
			"func (a *deepResearchToolAdapter) CreateJob(",
			"func toToolResearchJob(",
			"func cloneCalibrationForTool(",
		},
	}

	for name, forbidden := range mainFiles {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		for _, token := range forbidden {
			if strings.Contains(source, token) {
				t.Fatalf("expected %s to delegate token %q", name, token)
			}
		}
	}
}
