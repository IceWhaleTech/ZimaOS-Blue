package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationOperationalPhaseGo_PersistsOperationalRuntimeSnapshot(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_route_registration_operational_phase.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_operational_phase.go: %v", err)
	}
	source := string(content)

	optionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_operational_phase_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_operational_phase_options.go: %v", err)
	}
	optionSource := string(optionContent)

	taskSurfaceContent, err := os.ReadFile(filepath.Join("runtime_route_registration_operational_phase_task_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_operational_phase_task_surface_options.go: %v", err)
	}
	taskSurfaceSource := string(taskSurfaceContent)

	activationContent, err := os.ReadFile(filepath.Join("runtime_route_registration_operational_phase_activation_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_operational_phase_activation_options.go: %v", err)
	}
	activationSource := string(activationContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_route_registration_operational_phase_support_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_operational_phase_support_options.go: %v", err)
	}
	supportSource := string(supportContent)

	deferredContent, err := os.ReadFile(filepath.Join("runtime_route_registration_operational_phase_deferred_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_operational_phase_deferred_options.go: %v", err)
	}
	deferredSource := string(deferredContent)

	if lines := strings.Count(source, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_operational_phase.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(optionSource, "\n") + 1; lines > 75 {
		t.Fatalf("expected runtime_route_registration_operational_phase_options.go to stay below 75 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(taskSurfaceSource, "\n") + 1; lines > 18 {
		t.Fatalf("expected runtime_route_registration_operational_phase_task_surface_options.go to stay below 18 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(activationSource, "\n") + 1; lines > 16 {
		t.Fatalf("expected runtime_route_registration_operational_phase_activation_options.go to stay below 16 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_route_registration_operational_phase_support_options.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(deferredSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_route_registration_operational_phase_deferred_options.go to stay below 40 lines after extraction, got %d", lines)
	}

	required := []string{
		"func bindRouteRuntimeOperationalPhase(",
		"state.setOperationalRuntime(state.runtimeContract.BindOperationalRuntime(",
		"newRouteRuntimeOperationalOptions(state)",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_route_registration_operational_phase.go to contain token %q", token)
		}
	}

	requiredOptions := []string{
		"func newRouteRuntimeOperationalOptions(",
		"newRouteRuntimeOperationalTaskSurfaceOptions(state)",
		"newRouteRuntimeOperationalActivationOptions(state)",
		"newRouteRuntimeOperationalSupportOptions(state)",
		"newRouteRuntimeOperationalDeferredOptions(state)",
	}
	for _, token := range requiredOptions {
		if !strings.Contains(optionSource, token) {
			t.Fatalf("expected runtime_route_registration_operational_phase_options.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func newRouteRuntimeOperationalTaskSurfaceOptions(",
		"runtimeTaskSurfaceOptions{",
		"chatPermission:",
		"execApprovals:",
	} {
		if !strings.Contains(taskSurfaceSource, token) {
			t.Fatalf("expected runtime_route_registration_operational_phase_task_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeOperationalActivationOptions(",
		"routeRuntimeContractActivationOptions{",
		"runtimeLLM:",
		"authPageV1Group:",
	} {
		if !strings.Contains(activationSource, token) {
			t.Fatalf("expected runtime_route_registration_operational_phase_activation_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeOperationalSupportOptions(",
		"routeRuntimeContractSupportOptions{",
		"pageMiddleware:",
		"memoryHandler:",
	} {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_route_registration_operational_phase_support_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeOperationalDeferredOptions(",
		"routeRuntimeContractDeferredSupportOptions{",
		"skillRerankerDefaults: runtimeSkillRerankerDefaults{",
		"approvalHandler:",
		"workflowTarget:",
	} {
		if !strings.Contains(deferredSource, token) {
			t.Fatalf("expected runtime_route_registration_operational_phase_deferred_options.go to contain token %q", token)
		}
	}

	forbidden := []string{
		"taskSurface: runtimeTaskSurfaceOptions{",
		"activation: routeRuntimeContractActivationOptions{",
		"support: routeRuntimeContractSupportOptions{",
		"deferred: routeRuntimeContractDeferredSupportOptions{",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) || strings.Contains(optionSource, token) {
			t.Fatalf("expected operational phase assembly to delegate token %q", token)
		}
	}
}
