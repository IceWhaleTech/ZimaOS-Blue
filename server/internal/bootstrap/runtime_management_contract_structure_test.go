package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeManagementContract_IsSplitBySupportRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_management_contract.go": {
			maxLines: 35,
			tokens: []string{
				"func (binding *runtimeContractBinding) BindManagementSupport(",
				"func bindRouteRuntimeManagementSupport(",
				"bindRouteRuntimeRemoteAccessSupport(options)",
				"bindRouteRuntimeUpdateSupport(ctx, options)",
				"bindRouteRuntimeProviderSettings(options)",
			},
		},
		"runtime_management_contract_types.go": {
			maxLines: 55,
			tokens: []string{
				"type routeRuntimeContractManagementSupportOptions struct {",
				"type routeRuntimeContractManagementSupportResult struct {",
				"type routeRuntimeRemoteAccessBinding struct {",
				"type routeRuntimeUpdateBinding struct {",
			},
		},
		"runtime_management_contract_remote_access.go": {
			maxLines: 30,
			tokens: []string{
				"func bindRouteRuntimeRemoteAccessSupport(",
				"networkapi.NewSDKRemoteAccessHandler(",
			},
		},
		"runtime_management_contract_update.go": {
			maxLines: 60,
			tokens: []string{
				"func bindRouteRuntimeUpdateSupport(",
				"func newRouteRuntimeUpdateConfig(",
				"update.NewHandler(",
				"update.NewOTAChecker(",
			},
		},
		"runtime_management_contract_provider_settings.go": {
			maxLines: 35,
			tokens: []string{
				"func bindRouteRuntimeProviderSettings(",
				"serverpkg.NewProviderSettingsHandler(",
				"options.protected.Group(",
			},
		},
		"runtime_management_contract_server.go": {
			maxLines: 45,
			tokens: []string{
				"func routeRuntimeServerPort(",
				"func routeRuntimeServerVersion(",
				"func routeRuntimeServerMode(",
				"func routeRuntimeServerBuildTime(",
				"func routeRuntimeServerGitCommit(",
				"func routeRuntimeServerDataDir(",
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
