package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationHTTPBootstrapGo_PersistsEntrySnapshot(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_route_registration_http_bootstrap.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_http_bootstrap.go: %v", err)
	}
	source := string(content)

	if lines := strings.Count(source, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_http_bootstrap.go to stay below 20 lines after extraction, got %d", lines)
	}

	required := []string{
		"func bindRouteRuntimeHTTPBootstrap(",
		"connManager := connection.NewManager(",
		"state.setHTTPBootstrap(connManager, state.e.Group(\"/api/v1\"), state.e.Group(\"/api\"))",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_route_registration_http_bootstrap.go to contain token %q", token)
		}
	}
}
