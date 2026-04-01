package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeCapabilityRoutesGo_DelegatesResearchHarnessReflectAndLoggingLanes(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_capability_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes.go: %v", err)
	}
	mainSource := string(mainContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_capability_routes_types.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes_types.go: %v", err)
	}
	typeSource := string(typeContent)

	researchContent, err := os.ReadFile(filepath.Join("runtime_capability_routes_research.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes_research.go: %v", err)
	}
	researchSource := string(researchContent)

	researchOptionContent, err := os.ReadFile(filepath.Join("runtime_capability_routes_research_options.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes_research_options.go: %v", err)
	}
	researchOptionSource := string(researchOptionContent)

	surfaceRuntimeContent, err := os.ReadFile(filepath.Join("runtime_capability_surface_runtime.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_surface_runtime.go: %v", err)
	}
	surfaceRuntimeSource := string(surfaceRuntimeContent)

	surfaceRuntimeChatContent, err := os.ReadFile(filepath.Join("runtime_capability_surface_runtime_chat.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_surface_runtime_chat.go: %v", err)
	}
	surfaceRuntimeChatSource := string(surfaceRuntimeChatContent)

	surfaceRuntimeResearchContent, err := os.ReadFile(filepath.Join("runtime_capability_surface_runtime_research.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_surface_runtime_research.go: %v", err)
	}
	surfaceRuntimeResearchSource := string(surfaceRuntimeResearchContent)

	harnessContent, err := os.ReadFile(filepath.Join("runtime_capability_routes_harness.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes_harness.go: %v", err)
	}
	harnessSource := string(harnessContent)

	reflectContent, err := os.ReadFile(filepath.Join("runtime_capability_routes_reflect.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes_reflect.go: %v", err)
	}
	reflectSource := string(reflectContent)

	logContent, err := os.ReadFile(filepath.Join("runtime_capability_routes_logging.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_routes_logging.go: %v", err)
	}
	logSource := string(logContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_routes.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_routes_types.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(researchSource, "\n") + 1; lines > 50 {
		t.Fatalf("expected runtime_capability_routes_research.go to stay below 50 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(researchOptionSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_routes_research_options.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceRuntimeSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_surface_runtime.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceRuntimeChatSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_capability_surface_runtime_chat.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(surfaceRuntimeResearchSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_surface_runtime_research.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(harnessSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_routes_harness.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(reflectSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_routes_reflect.go to stay below 25 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(logSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_capability_routes_logging.go to stay below 25 lines after extraction, got %d", lines)
	}

	requiredMain := []string{
		"func runtimeTaskSurfaceGroup(",
	}
	for _, token := range requiredMain {
		if !strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_capability_routes.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type runtimeTaskSurfaceOptions struct {",
		"type runtimeTaskHarnessSurfaceRegistration struct {",
		"type runtimeTaskSurfaceRegistration struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_capability_routes_types.go to contain token %q", token)
		}
	}

	requiredResearch := []string{
		"type runtimeTaskResearchSurfaceOptions struct {",
		"type runtimeTaskResearchSurfaceRegistration struct {",
	}
	for _, token := range requiredResearch {
		if !strings.Contains(researchSource, token) {
			t.Fatalf("expected runtime_capability_routes_research.go to contain token %q", token)
		}
	}
	if !strings.Contains(researchOptionSource, "func newRuntimeTaskResearchSurfaceOptions(") {
		t.Fatal("expected runtime_capability_routes_research_options.go to keep research option assembly")
	}
	requiredResearchOptions := []string{
		"contract runtimeCapabilityResearchSurface",
		"contract.ResearchService()",
		"contract.HarnessRuntime()",
	}
	for _, token := range requiredResearchOptions {
		if !strings.Contains(researchOptionSource, token) {
			t.Fatalf("expected runtime_capability_routes_research_options.go to contain token %q", token)
		}
	}
	requiredSurfaceRuntime := []string{
		"func registerRuntimeTaskSurface(",
		"newRuntimeTaskResearchSurfaceOptions(contract, options)",
		"registerTaskHarnessSurface(",
		"registerReflectTaskSurface(",
		"logTaskSurfaceRegistration(",
	}
	for _, token := range requiredSurfaceRuntime {
		if !strings.Contains(surfaceRuntimeSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func bindRuntimeCapabilityChatResearch(",
		"applyChatResearchRuntimeBinding(binding, chatResearchRuntimeBindingTargets{",
	} {
		if !strings.Contains(surfaceRuntimeChatSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime_chat.go to contain token %q", token)
		}
	}
	for _, token := range []string{
		"func registerRuntimeTaskResearchSurface(",
		"registerHarnessRuntimeResearchTaskRoutes(",
	} {
		if !strings.Contains(surfaceRuntimeResearchSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime_research.go to contain token %q", token)
		}
	}
	if !strings.Contains(harnessSource, "func registerTaskHarnessSurface(") {
		t.Fatal("expected runtime_capability_routes_harness.go to keep harness lane")
	}
	if !strings.Contains(reflectSource, "func registerReflectTaskSurface(") {
		t.Fatal("expected runtime_capability_routes_reflect.go to keep reflect lane")
	}
	if !strings.Contains(logSource, "func logTaskSurfaceRegistration(") {
		t.Fatal("expected runtime_capability_routes_logging.go to keep task surface logging")
	}

	forbiddenMain := []string{
		"type runtimeTaskSurfaceOptions struct {",
		"func (contract runtimeCapabilityContract) registerTaskSurface(",
		"func registerTaskHarnessSurface(",
		"func registerReflectTaskSurface(",
		"func logTaskSurfaceRegistration(",
		"contract.registerResearchTaskSurface(",
		"contract.Harness,",
		"contract.Harness)",
		"contract.Reflect,",
		"contract.Reflect)",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_capability_routes.go to delegate token %q", token)
		}
	}

	for _, token := range []string{
		"deepresearch.NewHandler(",
		"registerHarnessRuntimeResearchTaskRoutes(",
		"func (contract runtimeCapabilityContract) registerResearchTaskSurface(",
	} {
		if strings.Contains(researchSource, token) {
			t.Fatalf("expected runtime_capability_routes_research.go to delegate token %q", token)
		}
	}

	for _, token := range []string{"contract.Research,", "contract.Research)", "contract.Harness,", "contract.Harness)"} {
		if strings.Contains(researchOptionSource, token) {
			t.Fatalf("expected runtime_capability_routes_research_options.go to delegate token %q", token)
		}
	}

	for _, token := range []string{
		"func (adapter runtimeCapabilityAdapter) registerTaskSurface(",
		"func (adapter runtimeCapabilityAdapter) registerResearchTaskSurface(",
		"type runtimeTaskSurfaceOptions struct {",
		"func bindRuntimeCapabilityChatResearch(",
		"func registerRuntimeTaskResearchSurface(",
	} {
		if strings.Contains(surfaceRuntimeSource, token) {
			t.Fatalf("expected runtime_capability_surface_runtime.go to stay focused on surface helpers and delegate token %q", token)
		}
	}
}
