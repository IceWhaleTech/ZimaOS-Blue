package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMgmtAdapters_AreSplitByDomain(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"mgmt_adapters_provider.go": {
			maxLines: 200,
			tokens: []string{
				"type mgmtProviderAdapter struct",
				"func (a *mgmtProviderAdapter) ListProviders(",
				"func (a *mgmtProviderAdapter) AddProvider(",
				"func (a *mgmtProviderAdapter) ListModels(",
			},
		},
		"mgmt_adapters_settings_channel.go": {
			maxLines: 105,
			tokens: []string{
				"type mgmtSettingsAdapter struct",
				"func (a *mgmtSettingsAdapter) GetAll(",
				"type mgmtChannelAdapter struct",
				"func (a *mgmtChannelAdapter) GetStatus(",
			},
		},
		"mgmt_adapters_runtime_assets.go": {
			maxLines: 130,
			tokens: []string{
				"type mgmtSkillAdapter struct",
				"type mgmtToolAdapter struct",
				"type mgmtSystemAdapter struct",
				"func formatDuration(",
			},
		},
		"mgmt_adapters_identity.go": {
			maxLines: 120,
			tokens: []string{
				"type mgmtUserAdapter struct",
				"type mgmtAPIKeyAdapter struct",
				"func parseUserUUID(",
			},
		},
		"mgmt_adapters_upgrade.go": {
			maxLines: 170,
			tokens: []string{
				"type mgmtUpgradeAdapter struct",
				"func (a *mgmtUpgradeAdapter) GetOTAStatus(",
				"func (a *mgmtUpgradeAdapter) ApplyUpdate(",
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
