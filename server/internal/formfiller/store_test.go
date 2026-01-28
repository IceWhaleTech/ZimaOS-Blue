package formfiller

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "formfiller-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Should have default template
	templates := store.ListTemplates()
	if len(templates) != 1 {
		t.Errorf("Expected 1 default template, got %d", len(templates))
	}

	if !templates[0].IsDefault {
		t.Error("Default template should have IsDefault=true")
	}
}

func TestTemplateOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "formfiller-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create template
	template, err := store.CreateTemplate(&CreateTemplateRequest{
		Name:      "Work",
		IsDefault: false,
		Fields: map[string]string{
			"email": "work@example.com",
			"phone": "+1-555-1234",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	if template.Name != "Work" {
		t.Errorf("Expected name 'Work', got '%s'", template.Name)
	}

	// Get template
	retrieved, err := store.GetTemplate(template.ID)
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if retrieved.Fields["email"] != "work@example.com" {
		t.Errorf("Expected email 'work@example.com', got '%s'", retrieved.Fields["email"])
	}

	// Update template
	isDefault := true
	updated, err := store.UpdateTemplate(template.ID, &UpdateTemplateRequest{
		Name:      "Work Profile",
		IsDefault: &isDefault,
	})
	if err != nil {
		t.Fatalf("Failed to update template: %v", err)
	}

	if updated.Name != "Work Profile" {
		t.Errorf("Expected name 'Work Profile', got '%s'", updated.Name)
	}

	if !updated.IsDefault {
		t.Error("Expected IsDefault=true after update")
	}

	// Delete template
	err = store.DeleteTemplate(template.ID)
	if err != nil {
		t.Fatalf("Failed to delete template: %v", err)
	}

	_, err = store.GetTemplate(template.ID)
	if err == nil {
		t.Error("Expected error when getting deleted template")
	}
}

func TestPatternOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "formfiller-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Get default patterns
	patterns := store.GetPatterns()
	if patterns == nil {
		t.Fatal("Expected default patterns")
	}

	if len(patterns.Patterns[FieldEmail]) == 0 {
		t.Error("Expected email patterns")
	}

	// Update patterns
	newPatterns := map[FieldType][]string{
		FieldEmail: {"email", "correo", "邮箱"},
	}
	err = store.UpdatePatterns(newPatterns)
	if err != nil {
		t.Fatalf("Failed to update patterns: %v", err)
	}

	updated := store.GetPatterns()
	if len(updated.Patterns[FieldEmail]) != 3 {
		t.Errorf("Expected 3 email patterns, got %d", len(updated.Patterns[FieldEmail]))
	}
}

func TestSiteMappingOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "formfiller-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save site mapping
	mapping := &SiteMapping{
		Domain: "example.com",
		FieldMappings: map[string]string{
			"#email-input": "email",
			"#phone-input": "phone",
		},
		UserCorrections: 2,
	}

	err = store.SaveSiteMapping(mapping)
	if err != nil {
		t.Fatalf("Failed to save site mapping: %v", err)
	}

	// Get site mapping
	retrieved, err := store.GetSiteMapping("example.com")
	if err != nil {
		t.Fatalf("Failed to get site mapping: %v", err)
	}

	if retrieved.FieldMappings["#email-input"] != "email" {
		t.Errorf("Expected email mapping, got '%s'", retrieved.FieldMappings["#email-input"])
	}

	// Verify file was created
	sitePath := filepath.Join(tmpDir, "sites", "example.com.json")
	if _, err := os.Stat(sitePath); os.IsNotExist(err) {
		t.Error("Site mapping file was not created")
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "formfiller-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store and add template
	store1, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	_, err = store1.CreateTemplate(&CreateTemplateRequest{
		Name:   "Persistent",
		Fields: map[string]string{"email": "test@example.com"},
	})
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Create new store instance (simulating restart)
	store2, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second store: %v", err)
	}

	// Should have 2 templates (default + created)
	templates := store2.ListTemplates()
	if len(templates) != 2 {
		t.Errorf("Expected 2 templates after reload, got %d", len(templates))
	}

	// Find the persistent template
	var found bool
	for _, tmpl := range templates {
		if tmpl.Name == "Persistent" {
			found = true
			if tmpl.Fields["email"] != "test@example.com" {
				t.Errorf("Expected email 'test@example.com', got '%s'", tmpl.Fields["email"])
			}
		}
	}
	if !found {
		t.Error("Persistent template not found after reload")
	}
}
