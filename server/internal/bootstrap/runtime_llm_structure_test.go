package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeLLM_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_llm.go": {
			maxLines: 20,
			tokens: []string{
				"var errRuntimeLLMUnavailable = fmt.Errorf(",
				"func newProxyBridgeProvider(",
			},
		},
		"runtime_llm_ref.go": {
			maxLines: 75,
			tokens: []string{
				"type runtimeLLMProviderRef struct {",
				"func newRuntimeLLMProviderRef() *runtimeLLMProviderRef {",
				"func (r *runtimeLLMProviderRef) Chat(",
				"func (r *runtimeLLMProviderRef) ChatStreamCallback(",
			},
		},
		"runtime_llm_proxy.go": {
			maxLines: 65,
			tokens: []string{
				"type proxyBridgeProvider struct {",
				"func (p *proxyBridgeProvider) normalize(",
				"func (p *proxyBridgeProvider) Chat(",
				"func (p *proxyBridgeProvider) ChatStream(",
			},
		},
		"runtime_llm_model.go": {
			maxLines: 40,
			tokens: []string{
				"const defaultRuntimeModel =",
				"func shouldUseDefaultRuntimeModel(",
				"func resolveDefaultRuntimeModel(",
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
