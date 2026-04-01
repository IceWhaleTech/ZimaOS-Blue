package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuxiliaryLLM_IsSplitByRuntimeRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"auxiliary_llm.go": {
			maxLines: 81,
			tokens: []string{
				"type auxiliaryLLMCaller struct {",
				"func newAuxiliaryLLMCaller() *auxiliaryLLMCaller {",
				"func (c *auxiliaryLLMCaller) Chat(",
			},
		},
		"auxiliary_llm_smallmodel.go": {
			maxLines: 55,
			tokens: []string{
				"func callAuxiliarySmallModel(",
				"func shouldFallbackFromSmallModel(",
				"runtime.Generate(ctx, smallmodel.GenerateRequest{",
			},
		},
		"auxiliary_llm_render.go": {
			maxLines: 95,
			tokens: []string{
				"func renderAuxiliarySmallModelPrompt(",
				"func renderAuxiliarySmallModelMessage(",
				"func auxiliaryRoleLabel(",
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
