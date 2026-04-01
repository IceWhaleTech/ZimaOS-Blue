package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserSkillAdapter_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"browser_skill_adapter.go": {
			maxLines: 50,
			tokens: []string{
				"type rodServiceProvider struct",
				"func (p rodServiceProvider) get(",
				"func (p rodServiceProvider) acquireLease(",
			},
		},
		"browser_skill_adapter_browser.go": {
			maxLines: 220,
			tokens: []string{
				"type browserSkillAdapter struct",
				"func newLazyBrowserSkillAdapter(",
				"func (a *browserSkillAdapter) Navigate(",
				"func (a *browserSkillAdapter) ExecuteRecipe(",
			},
		},
		"browser_skill_adapter_fallback.go": {
			maxLines: 110,
			tokens: []string{
				"type fallbackBrowserAdapter struct",
				"func newLeaseAwareFallbackBrowserAdapter(",
				"func (a *fallbackBrowserAdapter) Scrape(",
				"func (a *fallbackBrowserAdapter) PageInfo(",
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
