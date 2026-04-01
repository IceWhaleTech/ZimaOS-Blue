package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationStateGo_DelegatesInitializationSteps(t *testing.T) {
	stateContent, err := os.ReadFile(filepath.Join("runtime_route_registration_state.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_state.go: %v", err)
	}
	stateSource := string(stateContent)

	initContent, err := os.ReadFile(filepath.Join("runtime_route_registration_state_init.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_state_init.go: %v", err)
	}
	initSource := string(initContent)

	if lines := strings.Count(stateSource, "\n") + 1; lines > 85 {
		t.Fatalf("expected runtime_route_registration_state.go to stay below 85 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(initSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_route_registration_state_init.go to stay below 70 lines after extraction, got %d", lines)
	}

	for _, token := range []string{
		"type routeRegistrationState struct {",
		"func newRouteRegistrationState(",
		"newRouteRegistrationStateBase(e, deps)",
		"bindRouteRegistrationStateRuntime(state)",
		"func (state *routeRegistrationState) lookupProviderAPIKey(",
	} {
		if !strings.Contains(stateSource, token) {
			t.Fatalf("expected runtime_route_registration_state.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func newRouteRegistrationStateBase(",
		"registerStart: time.Now()",
		"NewStartupTrace(\"bootstrap.register_routes\"",
		"func bindRouteRegistrationStateRuntime(",
		"ResolveWorkspaceDir(",
		"ResolveBuiltinToolAllowedPaths(",
		"buildWebSearchConfig(",
		"bindRouteRegistrationStateDB(state)",
		"newRouteRuntimeContract(",
		"func bindRouteRegistrationStateDB(",
		"func bindRouteRegistrationStateFlags(",
		"config.NewFlagEvaluator(",
	} {
		if !strings.Contains(initSource, token) {
			t.Fatalf("expected runtime_route_registration_state_init.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"registerStart: time.Now()",
		"ResolveWorkspaceDir(",
		"ResolveBuiltinToolAllowedPaths(",
		"buildWebSearchConfig(",
		"newRouteRuntimeContract(",
		"config.NewFlagEvaluator(",
	} {
		if strings.Contains(stateSource, token) {
			t.Fatalf("expected runtime_route_registration_state.go to delegate initialization token %q", token)
		}
	}
}
