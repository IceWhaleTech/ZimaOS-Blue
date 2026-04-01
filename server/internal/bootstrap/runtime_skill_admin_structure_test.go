package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeSkillAdminGo_DelegatesListingRegistryAndHelperSlices(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_skill_admin.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_admin.go: %v", err)
	}
	mainSource := string(mainContent)

	listingContent, err := os.ReadFile(filepath.Join("runtime_skill_admin_listing.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_admin_listing.go: %v", err)
	}
	listingSource := string(listingContent)

	registryContent, err := os.ReadFile(filepath.Join("runtime_skill_admin_registry.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_admin_registry.go: %v", err)
	}
	registrySource := string(registryContent)

	helperContent, err := os.ReadFile(filepath.Join("runtime_skill_admin_helpers.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_admin_helpers.go: %v", err)
	}
	helperSource := string(helperContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 5 {
		t.Fatalf("expected runtime_skill_admin.go to stay below 5 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(listingSource, "\n") + 1; lines > 110 {
		t.Fatalf("expected runtime_skill_admin_listing.go to stay below 110 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(registrySource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_skill_admin_registry.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(helperSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_skill_admin_helpers.go to stay below 35 lines after extraction, got %d", lines)
	}

	if !strings.Contains(listingSource, "func listRuntimeAdminSkills(") {
		t.Fatal("expected runtime_skill_admin_listing.go to keep skill listing")
	}
	if !strings.Contains(registrySource, "func ensureRuntimeManagedSkillRegistered(") {
		t.Fatal("expected runtime_skill_admin_registry.go to keep managed skill registration")
	}
	if !strings.Contains(helperSource, "func adminSkillInfoFromDocument(") {
		t.Fatal("expected runtime_skill_admin_helpers.go to keep admin skill info normalization")
	}

	forbiddenMain := []string{
		"func listRuntimeAdminSkills(",
		"func ensureRuntimeManagedSkillRegistered(",
		"func adminSkillInfoFromDocument(",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_skill_admin.go to delegate token %q", token)
		}
	}
}
