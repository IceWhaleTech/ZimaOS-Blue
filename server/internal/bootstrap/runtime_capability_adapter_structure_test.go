package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeCapabilityAdapter_IsSplitByFacadeRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_capability_adapter.go": {
			maxLines: 10,
			tokens: []string{
				"type runtimeCapabilityAdapter struct {",
				"func newRuntimeCapabilityAdapter(",
			},
		},
		"runtime_capability_adapter_surface.go": {
			maxLines: 20,
			tokens: []string{
				"func (adapter runtimeCapabilityAdapter) HarnessRuntime(",
				"func (adapter runtimeCapabilityAdapter) ResearchService(",
				"func (adapter runtimeCapabilityAdapter) ReflectService(",
			},
		},
		"runtime_capability_adapter_routes.go": {
			maxLines: 15,
			tokens: []string{
				"func (adapter runtimeCapabilityAdapter) registerTaskSurface(",
				"func (adapter runtimeCapabilityAdapter) registerResearchTaskSurface(",
			},
		},
		"runtime_capability_adapter_chat.go": {
			maxLines: 20,
			tokens: []string{
				"func (adapter runtimeCapabilityAdapter) bindChatResearch(",
				"bindRuntimeCapabilityChatResearch(adapter, target, broker, registry, workspaceDir)",
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

func TestRuntimeCapabilityAdapter_MainFileStaysThin(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_capability_adapter.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_adapter.go: %v", err)
	}
	source := string(content)
	for _, token := range []string{
		"func (adapter runtimeCapabilityAdapter) HarnessRuntime(",
		"func (adapter runtimeCapabilityAdapter) registerTaskSurface(",
		"func (adapter runtimeCapabilityAdapter) bindChatResearch(",
	} {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_capability_adapter.go to delegate token %q", token)
		}
	}
}
