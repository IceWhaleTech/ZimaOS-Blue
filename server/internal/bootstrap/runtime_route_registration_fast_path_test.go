package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRouteRegistrationFastPathGo_PersistsAuthBootstrapAndEntrySnapshots(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path.go: %v", err)
	}
	source := string(content)

	startupAuthOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_startup_auth_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_startup_auth_options.go: %v", err)
	}
	startupAuthOptionSource := string(startupAuthOptionContent)

	bootstrapPhaseOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_bootstrap_phase_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_bootstrap_phase_options.go: %v", err)
	}
	bootstrapPhaseOptionSource := string(bootstrapPhaseOptionContent)

	startupSurfaceContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_startup_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_startup_surface_options.go: %v", err)
	}
	startupSurfaceSource := string(startupSurfaceContent)

	authSurfaceContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_auth_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_auth_surface_options.go: %v", err)
	}
	authSurfaceSource := string(authSurfaceContent)

	accountSurfaceContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_account_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_account_surface_options.go: %v", err)
	}
	accountSurfaceSource := string(accountSurfaceContent)

	shellOptionContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_shell_surface_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_shell_surface_options.go: %v", err)
	}
	shellOptionSource := string(shellOptionContent)

	bootstrapSupportContent, err := os.ReadFile(filepath.Join("runtime_route_registration_fast_path_bootstrap_support_options.go"))
	if err != nil {
		t.Fatalf("read runtime_route_registration_fast_path_bootstrap_support_options.go: %v", err)
	}
	bootstrapSupportSource := string(bootstrapSupportContent)

	if lines := strings.Count(source, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_fast_path.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(startupAuthOptionSource, "\n") + 1; lines > 12 {
		t.Fatalf("expected runtime_route_registration_fast_path_startup_auth_options.go to stay below 12 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(bootstrapPhaseOptionSource, "\n") + 1; lines > 12 {
		t.Fatalf("expected runtime_route_registration_fast_path_bootstrap_phase_options.go to stay below 12 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(startupSurfaceSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_route_registration_fast_path_startup_surface_options.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(authSurfaceSource, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_route_registration_fast_path_auth_surface_options.go to stay below 15 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(accountSurfaceSource, "\n") + 1; lines > 14 {
		t.Fatalf("expected runtime_route_registration_fast_path_account_surface_options.go to stay below 14 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(shellOptionSource, "\n") + 1; lines > 24 {
		t.Fatalf("expected runtime_route_registration_fast_path_shell_surface_options.go to stay below 24 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(bootstrapSupportSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_route_registration_fast_path_bootstrap_support_options.go to stay below 20 lines after extraction, got %d", lines)
	}

	required := []string{
		"func bindRouteRuntimeStartupAuthSurfaces(",
		"state.setStartupAuthRuntime(state.runtimeContract.BindStartupAuthRuntime(",
		"func bindRouteRuntimeBootstrapSupportPhase(",
		"state.setBootstrapPhaseRuntime(state.runtimeContract.BindBootstrapPhaseRuntime(",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_route_registration_fast_path.go to contain token %q", token)
		}
	}

	requiredStartupAuthOptions := []string{
		"func newRouteRuntimeStartupAuthOptions(",
		"newRouteRuntimeStartupSurfaceOptions(state)",
		"newRouteRuntimeAuthSurfaceOptions(state)",
		"newRouteRuntimeAccountSurfaceOptions(state)",
	}
	for _, token := range requiredStartupAuthOptions {
		if !strings.Contains(startupAuthOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_startup_auth_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeBootstrapPhaseOptions(",
		"newRouteRuntimeShellSurfaceOptions(state)",
		"newRouteRuntimeBootstrapSupportOptions(state)",
		"onEarlyReady: state.deps.OnEarlyReady",
	} {
		if !strings.Contains(bootstrapPhaseOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_bootstrap_phase_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeStartupSurfaceOptions(",
		"routeRuntimeContractStartupSurfaceOptions{",
	} {
		if !strings.Contains(startupSurfaceSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_startup_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeAuthSurfaceOptions(",
		"routeRuntimeContractAuthSurfaceOptions{",
		"userRepo:",
	} {
		if !strings.Contains(authSurfaceSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_auth_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeAccountSurfaceOptions(",
		"routeRuntimeContractAccountSurfaceOptions{",
		"apiKeyHandler:",
		"autoreplyHandler:",
	} {
		if !strings.Contains(accountSurfaceSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_account_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeShellSurfaceOptions(",
		"routeRuntimeContractShellSurfaceOptions{",
		"toolRegistry:",
	} {
		if !strings.Contains(shellOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_shell_surface_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func newRouteRuntimeBootstrapSupportOptions(",
		"routeRuntimeContractBootstrapSupportOptions{",
		"ngrokConfigStore:",
		"jwtService:",
	} {
		if !strings.Contains(bootstrapSupportSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_bootstrap_support_options.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractStartupSurfaceOptions{",
		"routeRuntimeContractAuthSurfaceOptions{",
		"routeRuntimeContractAccountSurfaceOptions{",
	} {
		if strings.Contains(startupAuthOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_startup_auth_options.go to delegate option token %q", token)
		}
	}
	for _, token := range []string{
		"routeRuntimeContractShellSurfaceOptions{",
		"routeRuntimeContractBootstrapSupportOptions{",
	} {
		if strings.Contains(bootstrapPhaseOptionSource, token) {
			t.Fatalf("expected runtime_route_registration_fast_path_bootstrap_phase_options.go to delegate option token %q", token)
		}
	}

	forbidden := []string{
		".BindStartupSurfaceRuntime(",
		".BindAuthSurfaceRuntime(",
		".BindAccountSurfaceRuntime(",
		".BindShellSurfaceRuntime(",
		".BindBootstrapSupportRuntime(",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_route_registration_fast_path.go to delegate token %q", token)
		}
	}
}
