package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteRegistrationStateRuntime_IsSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"runtime_route_registration_state_runtime.go": {
			maxLines: 50,
			tokens: []string{
				"type routeRegistrationEntrySnapshot struct {",
				"type routeRegistrationRuntimeSnapshot struct {",
				"type routeRegistrationUtilitySnapshot struct {",
			},
		},
		"runtime_route_registration_state_runtime_entry.go": {
			maxLines: 65,
			tokens: []string{
				"func (state *routeRegistrationState) setHTTPBootstrap(",
				"func (state *routeRegistrationState) setSkillAutoReranker(",
				"func (state *routeRegistrationState) setMgmtTool(",
			},
		},
		"runtime_route_registration_state_runtime_core.go": {
			maxLines: 150,
			tokens: []string{
				"func (state *routeRegistrationState) setStartupAuthRuntime(",
				"func (state *routeRegistrationState) setCoreToolingRuntime(",
				"func (state *routeRegistrationState) setExecSupport(",
			},
		},
		"runtime_route_registration_state_runtime_management.go": {
			maxLines: 35,
			tokens: []string{
				"func (state *routeRegistrationState) setManagementRuntime(",
				"func (state *routeRegistrationState) setOperationalRuntime(",
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
