package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeBootstrapSupportContract_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_bootstrap_support_contract.go": {
			maxLines: 60,
			tokens: []string{
				"func (binding *runtimeContractBinding) BindBootstrapSupportRuntime(",
				"func bindRouteRuntimeBootstrapSupport(",
				"serverpkg.NewSettingsHandler(",
				"sse.NewHandler(options.sseBroker).RegisterRoutes(",
				"registerRouteRuntimeVoiceWakeSurface(",
				"registerRouteRuntimeTunnelSurface(",
			},
		},
		"runtime_bootstrap_support_contract_types.go": {
			maxLines: 40,
			tokens: []string{
				"type routeRuntimeContractBootstrapSupportOptions struct {",
				"type routeRuntimeContractBootstrapSupportResult struct {",
			},
		},
		"runtime_bootstrap_support_contract_voicewake.go": {
			maxLines: 55,
			tokens: []string{
				"func registerRouteRuntimeVoiceWakeSurface(",
				"voicewake.NewManager(",
				"voicewake.NewHandler(voiceWakeManager).RegisterRoutes(",
			},
		},
		"runtime_bootstrap_support_contract_tunnel.go": {
			maxLines: 20,
			tokens: []string{
				"func registerRouteRuntimeTunnelSurface(",
				"networkapi.NewTunnelHandler(",
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
