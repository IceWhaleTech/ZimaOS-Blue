package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeChannelContract_IsSplitByRuntimeRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_channel_contract.go": {
			maxLines: 45,
			tokens: []string{
				"func (binding *runtimeContractBinding) BindChannelRuntime(",
				"func bindRouteRuntimeChannels(",
				"newRouteRuntimeChannelManager(options)",
				"bindRouteRuntimeChannelChatTarget(",
				"bindRouteRuntimeChannelWatcherTarget(",
				"bindRouteRuntimeChannelHandler(",
				"startRouteRuntimeEnabledChannelsAsync(",
			},
		},
		"runtime_channel_contract_types.go": {
			maxLines: 75,
			tokens: []string{
				"type routeRuntimeContractChannelOptions struct {",
				"type runtimeChannelSender interface {",
				"type runtimeChannelChatTarget interface {",
				"type runtimeChannelAutoreplyTarget interface {",
				"type runtimeChannelFactory interface {",
			},
		},
		"runtime_channel_contract_runtime.go": {
			maxLines: 105,
			tokens: []string{
				"func newRouteRuntimeChannelManager(",
				"func resolveRouteRuntimeChannelConfig(",
				"func bindRouteRuntimeChannelChatTarget(",
				"func bindRouteRuntimeChannelWatcherTarget(",
				"func bindRouteRuntimeChannelHandler(",
			},
		},
		"runtime_channel_contract_startup.go": {
			maxLines: 70,
			tokens: []string{
				"func startRouteRuntimeEnabledChannelsAsync(",
				"func startRouteRuntimeEnabledChannels(",
				"func routeRuntimeChannelSupportedOnPlatform(",
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
