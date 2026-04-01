package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeActivationSupportGo_DelegatesRouteDeferredAndHelperSlices(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_activation_support.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_support.go: %v", err)
	}
	mainSource := string(mainContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_activation_support_types.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_support_types.go: %v", err)
	}
	typeSource := string(typeContent)

	routeContent, err := os.ReadFile(filepath.Join("runtime_activation_support_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_support_routes.go: %v", err)
	}
	routeSource := string(routeContent)

	deferredContent, err := os.ReadFile(filepath.Join("runtime_activation_support_deferred.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_support_deferred.go: %v", err)
	}
	deferredSource := string(deferredContent)

	helperContent, err := os.ReadFile(filepath.Join("runtime_activation_support_helpers.go"))
	if err != nil {
		t.Fatalf("read runtime_activation_support_helpers.go: %v", err)
	}
	helperSource := string(helperContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_activation_support.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_activation_support_types.go to stay below 55 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(routeSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_activation_support_routes.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(deferredSource, "\n") + 1; lines > 75 {
		t.Fatalf("expected runtime_activation_support_deferred.go to stay below 75 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(helperSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_activation_support_helpers.go to stay below 55 lines after extraction, got %d", lines)
	}

	requiredMain := []string{
		"func registerRuntimeActivationSupportRoutes(",
		"func bindRuntimeActivationSupport(",
		"bindRuntimeActivationDeferred(",
		"bindRuntimeActivationApproval(",
	}
	for _, token := range requiredMain {
		if !strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_activation_support.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type runtimeActivationDeferredSupportOptions struct {") {
		t.Fatal("expected runtime_activation_support_types.go to keep deferred support options")
	}
	if !strings.Contains(routeSource, "func registerMemoryRoutes(") {
		t.Fatal("expected runtime_activation_support_routes.go to keep memory route registration")
	}
	if !strings.Contains(deferredSource, "func bindRuntimeActivationDeferred(") {
		t.Fatal("expected runtime_activation_support_deferred.go to keep deferred wiring")
	}
	if !strings.Contains(helperSource, "func runtimeActivationSessionMaxTokens(") {
		t.Fatal("expected runtime_activation_support_helpers.go to keep activation helper wiring")
	}

	forbiddenMain := []string{
		"type runtimeActivationRouteSupportOptions struct {",
		"func registerMemoryRoutes(",
		"func runtimeActivationToolRegistry(",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_activation_support.go to delegate token %q", token)
		}
	}
}

func TestRuntimeActivationSupportHelpers_HandleNilInputs(t *testing.T) {
	if runtimeActivationToolRegistry(nil) != nil {
		t.Fatal("expected nil services to return nil tool registry")
	}
	if runtimeActivationImageTool(nil) != nil {
		t.Fatal("expected nil services to return nil image tool")
	}
	if runtimeActivationExecAutoConfirm(nil) != nil {
		t.Fatal("expected nil services to return nil exec auto confirm")
	}
	if got := runtimeActivationSessionCompaction(nil); got != (config.SessionCompactionConfig{}) {
		t.Fatalf("unexpected nil session compaction config: %+v", got)
	}
	if got := runtimeActivationSessionMaxTokens(nil); got != 0 {
		t.Fatalf("unexpected nil session max tokens: %d", got)
	}
}

func TestRuntimeActivationImageTool_UsesImageToolFromRegistry(t *testing.T) {
	registry := tools.NewRegistry()
	imageTool := tools.NewImageTool(nil, nil, nil)
	registry.Register(imageTool)

	got := runtimeActivationImageTool(&Services{ToolRegistry: registry})
	if got != imageTool {
		t.Fatalf("image tool=%p, want %p", got, imageTool)
	}
}
