package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServicesGo_DelegatesWorkspacePrimaryDBProviderAndSessionHelpers(t *testing.T) {
	servicesContent, err := os.ReadFile(filepath.Join("services.go"))
	if err != nil {
		t.Fatalf("read services.go: %v", err)
	}
	servicesSource := string(servicesContent)

	workspaceContent, err := os.ReadFile(filepath.Join("services_workspace.go"))
	if err != nil {
		t.Fatalf("read services_workspace.go: %v", err)
	}
	workspaceSource := string(workspaceContent)

	allowedPathsContent, err := os.ReadFile(filepath.Join("services_workspace_allowed_paths.go"))
	if err != nil {
		t.Fatalf("read services_workspace_allowed_paths.go: %v", err)
	}
	allowedPathsSource := string(allowedPathsContent)

	primaryDBContent, err := os.ReadFile(filepath.Join("services_primary_db.go"))
	if err != nil {
		t.Fatalf("read services_primary_db.go: %v", err)
	}
	primaryDBSource := string(primaryDBContent)

	initIdentityContent, err := os.ReadFile(filepath.Join("services_init_identity.go"))
	if err != nil {
		t.Fatalf("read services_init_identity.go: %v", err)
	}
	initIdentitySource := string(initIdentityContent)

	initRuntimeContent, err := os.ReadFile(filepath.Join("services_init_runtime.go"))
	if err != nil {
		t.Fatalf("read services_init_runtime.go: %v", err)
	}
	initRuntimeSource := string(initRuntimeContent)

	providerPoolContent, err := os.ReadFile(filepath.Join("services_provider_pool.go"))
	if err != nil {
		t.Fatalf("read services_provider_pool.go: %v", err)
	}
	providerPoolSource := string(providerPoolContent)

	llmRegistryContent, err := os.ReadFile(filepath.Join("services_llm_registry.go"))
	if err != nil {
		t.Fatalf("read services_llm_registry.go: %v", err)
	}
	llmRegistrySource := string(llmRegistryContent)

	lifecycleContent, err := os.ReadFile(filepath.Join("services_lifecycle.go"))
	if err != nil {
		t.Fatalf("read services_lifecycle.go: %v", err)
	}
	lifecycleSource := string(lifecycleContent)

	if lines := strings.Count(workspaceSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected services_workspace.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(allowedPathsSource, "\n") + 1; lines > 90 {
		t.Fatalf("expected services_workspace_allowed_paths.go to stay below 90 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(primaryDBSource, "\n") + 1; lines > 110 {
		t.Fatalf("expected services_primary_db.go to stay below 110 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(servicesSource, "\n") + 1; lines > 90 {
		t.Fatalf("expected services.go to stay below 90 lines after orchestration extraction, got %d", lines)
	}
	if lines := strings.Count(initIdentitySource, "\n") + 1; lines > 90 {
		t.Fatalf("expected services_init_identity.go to stay below 90 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(initRuntimeSource, "\n") + 1; lines > 95 {
		t.Fatalf("expected services_init_runtime.go to stay below 95 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(providerPoolSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected services_provider_pool.go to stay below 60 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(llmRegistrySource, "\n") + 1; lines > 30 {
		t.Fatalf("expected services_llm_registry.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(lifecycleSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected services_lifecycle.go to stay below 25 lines after extraction, got %d", lines)
	}
	for _, token := range []string{
		"func ResolveWorkspaceDir(",
		"func normalizeExplicitWorkspaceDir(",
		"func normalizeWorkspacePath(",
		"func ResolveBuiltinToolAllowedPaths(",
	} {
		if !strings.Contains(workspaceSource, token) {
			t.Fatalf("expected services_workspace.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func resolveBuiltinToolAllowedPaths(",
		"func shouldAllowTmpForWorkspace(",
		"func pathWithinRoot(",
	} {
		if !strings.Contains(allowedPathsSource, token) {
			t.Fatalf("expected services_workspace_allowed_paths.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func openPrimaryDatabase(",
		"backup.NewManager(",
		"database.RotateCorruptSQLiteDatabase(",
	} {
		if !strings.Contains(primaryDBSource, token) {
			t.Fatalf("expected services_primary_db.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func initServicesDatabaseAndIdentity(",
		"user.NewSQLiteRepositoryWithReadDB(",
		"auth.NewAPIKeyServiceWithReadDB(",
	} {
		if !strings.Contains(initIdentitySource, token) {
			t.Fatalf("expected services_init_identity.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func initServicesMemoryAndMedia(",
		"func initServicesRuntimeRegistries(",
		"func initServicesWorkerPool(",
	} {
		if !strings.Contains(initRuntimeSource, token) {
			t.Fatalf("expected services_init_runtime.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func LoadProvidersFromPool(",
		"llm.NewOpenAIProvider(",
		"llm.NewGLMProvider(",
	} {
		if !strings.Contains(providerPoolSource, token) {
			t.Fatalf("expected services_provider_pool.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func registerLLMProviders(",
		"llm.NewSiliconFlowProvider(",
	} {
		if !strings.Contains(llmRegistrySource, token) {
			t.Fatalf("expected services_llm_registry.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func (s *Services) Close(",
		"s.DBConn.Close()",
	} {
		if !strings.Contains(lifecycleSource, token) {
			t.Fatalf("expected services_lifecycle.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func initServicesDatabaseAndIdentity(",
		"func initServicesMemoryAndMedia(",
		"func initServicesRuntimeRegistries(",
		"func initServicesWorkerPool(",
		"func ResolveWorkspaceDir(",
		"func ResolveBuiltinToolAllowedPaths(",
		"func openPrimaryDatabase(",
		"func LoadProvidersFromPool(",
		"func registerLLMProviders(",
		"func (s *Services) Close(",
		"type sessionListAdapter struct",
		"func (a sessionListAdapter) HandleRuntimeSessionAction(",
	} {
		if strings.Contains(servicesSource, token) {
			t.Fatalf("expected services.go to delegate helper token %q", token)
		}
	}
}
