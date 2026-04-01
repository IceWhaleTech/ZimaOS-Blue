package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationEntryGo_DelegatesOnlyPathRegistration(t *testing.T) {
	entryContent, err := os.ReadFile(filepath.Join("runtime_route_registration_entry.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_entry.go: %v", err)
	}
	entrySource := string(entryContent)

	pathContent, err := os.ReadFile(filepath.Join("runtime_route_registration_paths.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_paths.go: %v", err)
	}
	pathSource := string(pathContent)

	if lines := strings.Count(entrySource, "\n") + 1; lines > 10 {
		t.Fatalf("expected runtime_route_registration_entry.go to stay below 10 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(pathSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_paths.go to stay below 20 lines after extraction, got %d", lines)
	}

	for _, token := range []string{
		"func bindRouteRuntimeRegistration(",
		"bindRouteRuntimeFastPath(state)",
		"bindRouteRuntimeDeferredPath(state)",
	} {
		if !strings.Contains(entrySource, token) {
			t.Fatalf("expected runtime_route_registration_entry.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func bindRouteRuntimeFastPath(",
		"bindRouteRuntimeHTTPBootstrap(state)",
		"bindRouteRuntimeStartupAuthSurfaces(state)",
		"bindRouteRuntimeBootstrapSupportPhase(state)",
		"func bindRouteRuntimeDeferredPath(",
		"bindRouteRuntimeCorePhase(state)",
		"bindRouteRuntimeManagementPhase(state)",
		"bindRouteRuntimeOperationalPhase(state)",
	} {
		if !strings.Contains(pathSource, token) {
			t.Fatalf("expected runtime_route_registration_paths.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func bindRouteRuntimeFastPath(",
		"func bindRouteRuntimeDeferredPath(",
		"bindRouteRuntimeHTTPBootstrap(state)",
		"bindRouteRuntimeOperationalPhase(state)",
	} {
		if strings.Contains(entrySource, token) {
			t.Fatalf("expected runtime_route_registration_entry.go to delegate token %q", token)
		}
	}
}
