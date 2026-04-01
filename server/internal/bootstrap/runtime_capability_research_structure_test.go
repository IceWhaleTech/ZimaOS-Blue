package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeCapabilityResearch_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_capability_research.go": {
			maxLines: 30,
			tokens: []string{
				"func bindHarnessRuntimeChatResearch(",
				"func bindHarnessRuntimeToResearchService(",
				"bindChatResearchRuntime(handler, service, binding)",
			},
		},
		"runtime_capability_research_types.go": {
			maxLines: 30,
			tokens: []string{
				"type researchEventPublisherTarget interface {",
				"type researchRuntimeBinding struct {",
				"type chatResearchRuntimeBinding struct {",
			},
		},
		"runtime_capability_research_binding.go": {
			maxLines: 55,
			tokens: []string{
				"func newResearchRuntimeBinding(",
				"func bindResearchRuntimeService(",
				"func registerResearchRuntime(",
			},
		},
		"runtime_capability_research_chat.go": {
			maxLines: 55,
			tokens: []string{
				"func newChatResearchRuntimeBinding(",
				"func bindChatResearchRuntime(",
				"func registerChatResearchRuntime(",
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

func TestRuntimeCapabilityResearch_MainFileStaysThin(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_capability_research.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_research.go: %v", err)
	}
	source := string(content)
	for _, token := range []string{
		"type researchRuntimeBinding struct {",
		"func newResearchRuntimeBinding(",
		"func newChatResearchRuntimeBinding(",
		"func registerChatResearchRuntime(",
	} {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_capability_research.go to delegate token %q", token)
		}
	}
}
