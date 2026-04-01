package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillManagerAdapter_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"skillmgr_adapter.go": {
			maxLines: 110,
			tokens: []string{
				"type skillManagerAdapter struct",
				"func newSkillManagerAdapter(",
				"func (a *skillManagerAdapter) Search(",
				"func (a *skillManagerAdapter) Enable(",
				"func (a *skillManagerAdapter) Disable(",
			},
		},
		"skillmgr_adapter_install.go": {
			maxLines: 220,
			tokens: []string{
				"func (a *skillManagerAdapter) Install(",
				"func (a *skillManagerAdapter) InstallURL(",
				"func deriveInstalledSkillID(",
				"func writeInstalledSkillContent(",
			},
		},
		"skillmgr_adapter_catalog.go": {
			maxLines: 210,
			tokens: []string{
				"type runtimeSkillEntry struct",
				"func (a *skillManagerAdapter) List(",
				"func (a *skillManagerAdapter) Info(",
				"func (a *skillManagerAdapter) resolveInstalledSkill(",
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
