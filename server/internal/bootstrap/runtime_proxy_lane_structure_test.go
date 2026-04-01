package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeProxyLane_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_proxy_lane.go": {
			maxLines: 75,
			tokens: []string{
				"func activateRuntimeProxyLane(",
				"newRuntimeProxySurfaceBundle(",
				"newRuntimeProxyLaneProviderBindingsOptions(",
				"newRuntimeProxyLaneRoutingOptions(",
				"bindRuntimeProxyLanePersistence(",
			},
		},
		"runtime_proxy_lane_types.go": {
			maxLines: 55,
			tokens: []string{
				"type runtimeProxyLaneOptions struct {",
				"type runtimeProxyLaneBundle struct {",
			},
		},
		"runtime_proxy_lane_wiring.go": {
			maxLines: 70,
			tokens: []string{
				"func newRuntimeProxyLaneProviderBindingsOptions(",
				"func newRuntimeProxyLaneRoutingOptions(",
				"func bindRuntimeProxyLanePersistence(",
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
