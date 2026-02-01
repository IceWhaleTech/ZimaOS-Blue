package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewProfileManager(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, err := NewProfileManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create profile manager: %v", err)
	}

	if pm == nil {
		t.Fatal("expected non-nil profile manager")
	}
}

func TestProfileManager_CreateProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	t.Run("create profile", func(t *testing.T) {
		profile := &Profile{
			ID:          "test-profile",
			Name:        "Test Profile",
			Description: "A test profile",
			UserAgent:   "Custom UA",
		}

		created, err := pm.CreateProfile(ctx, profile)
		if err != nil {
			t.Fatalf("failed to create profile: %v", err)
		}

		if created.ID != "test-profile" {
			t.Errorf("expected ID 'test-profile', got '%s'", created.ID)
		}
		if created.Viewport == nil {
			t.Error("expected default viewport to be set")
		}
	})

	t.Run("duplicate profile", func(t *testing.T) {
		profile := &Profile{
			ID:   "test-profile",
			Name: "Duplicate",
		}

		_, err := pm.CreateProfile(ctx, profile)
		if err == nil {
			t.Error("expected error for duplicate profile")
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		profile := &Profile{
			Name: "No ID",
		}

		_, err := pm.CreateProfile(ctx, profile)
		if err == nil {
			t.Error("expected error for missing ID")
		}
	})
}

func TestProfileManager_GetProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	profile := &Profile{
		ID:   "get-test",
		Name: "Get Test",
	}
	pm.CreateProfile(ctx, profile)

	t.Run("existing profile", func(t *testing.T) {
		p, err := pm.GetProfile("get-test")
		if err != nil {
			t.Fatalf("failed to get profile: %v", err)
		}
		if p.Name != "Get Test" {
			t.Errorf("expected name 'Get Test', got '%s'", p.Name)
		}
	})

	t.Run("non-existent profile", func(t *testing.T) {
		_, err := pm.GetProfile("non-existent")
		if err == nil {
			t.Error("expected error for non-existent profile")
		}
	})
}

func TestProfileManager_UpdateProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	profile := &Profile{
		ID:   "update-test",
		Name: "Original Name",
	}
	pm.CreateProfile(ctx, profile)

	t.Run("update profile", func(t *testing.T) {
		profile.Name = "Updated Name"
		profile.Description = "New description"

		updated, err := pm.UpdateProfile(ctx, profile)
		if err != nil {
			t.Fatalf("failed to update profile: %v", err)
		}

		if updated.Name != "Updated Name" {
			t.Errorf("expected name 'Updated Name', got '%s'", updated.Name)
		}
		if updated.Description != "New description" {
			t.Errorf("expected description 'New description', got '%s'", updated.Description)
		}
	})
}

func TestProfileManager_DeleteProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	profile := &Profile{
		ID:   "delete-test",
		Name: "Delete Test",
	}
	pm.CreateProfile(ctx, profile)

	t.Run("delete profile", func(t *testing.T) {
		err := pm.DeleteProfile(ctx, "delete-test")
		if err != nil {
			t.Fatalf("failed to delete profile: %v", err)
		}

		_, err = pm.GetProfile("delete-test")
		if err == nil {
			t.Error("expected error for deleted profile")
		}
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := pm.DeleteProfile(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent profile")
		}
	})
}

func TestProfileManager_ListProfiles(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	// Create multiple profiles
	for i := 0; i < 3; i++ {
		profile := &Profile{
			ID:   "list-test-" + string(rune('a'+i)),
			Name: "List Test " + string(rune('A'+i)),
		}
		pm.CreateProfile(ctx, profile)
	}

	profiles := pm.ListProfiles()
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestProfileManager_DefaultProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	// Create profiles
	profile1 := &Profile{ID: "profile-1", Name: "Profile 1"}
	profile2 := &Profile{ID: "profile-2", Name: "Profile 2"}
	pm.CreateProfile(ctx, profile1)
	pm.CreateProfile(ctx, profile2)

	t.Run("set default", func(t *testing.T) {
		err := pm.SetDefaultProfile(ctx, "profile-2")
		if err != nil {
			t.Fatalf("failed to set default: %v", err)
		}

		defaultProfile := pm.GetDefaultProfile()
		if defaultProfile == nil {
			t.Fatal("expected non-nil default profile")
		}
		if defaultProfile.ID != "profile-2" {
			t.Errorf("expected default ID 'profile-2', got '%s'", defaultProfile.ID)
		}
	})

	t.Run("change default", func(t *testing.T) {
		err := pm.SetDefaultProfile(ctx, "profile-1")
		if err != nil {
			t.Fatalf("failed to change default: %v", err)
		}

		defaultProfile := pm.GetDefaultProfile()
		if defaultProfile.ID != "profile-1" {
			t.Errorf("expected default ID 'profile-1', got '%s'", defaultProfile.ID)
		}

		// Verify old default is no longer default
		p2, _ := pm.GetProfile("profile-2")
		if p2.IsDefault {
			t.Error("expected profile-2 to not be default")
		}
	})
}

func TestProfileManager_CloneProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	source := &Profile{
		ID:        "source",
		Name:      "Source Profile",
		UserAgent: "Custom UA",
		Headers:   map[string]string{"X-Custom": "value"},
	}
	pm.CreateProfile(ctx, source)

	t.Run("clone profile", func(t *testing.T) {
		cloned, err := pm.CloneProfile(ctx, "source", "cloned", "Cloned Profile")
		if err != nil {
			t.Fatalf("failed to clone profile: %v", err)
		}

		if cloned.ID != "cloned" {
			t.Errorf("expected ID 'cloned', got '%s'", cloned.ID)
		}
		if cloned.Name != "Cloned Profile" {
			t.Errorf("expected name 'Cloned Profile', got '%s'", cloned.Name)
		}
		if cloned.UserAgent != "Custom UA" {
			t.Errorf("expected UserAgent 'Custom UA', got '%s'", cloned.UserAgent)
		}
		if cloned.Headers["X-Custom"] != "value" {
			t.Error("expected headers to be cloned")
		}
	})
}

func TestProfileManager_ExportImport(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	original := &Profile{
		ID:          "export-test",
		Name:        "Export Test",
		Description: "Test export/import",
		UserAgent:   "Test UA",
	}
	pm.CreateProfile(ctx, original)

	t.Run("export profile", func(t *testing.T) {
		data, err := pm.ExportProfile("export-test")
		if err != nil {
			t.Fatalf("failed to export profile: %v", err)
		}

		if len(data) == 0 {
			t.Error("expected non-empty export data")
		}
	})

	t.Run("import profile", func(t *testing.T) {
		data, _ := pm.ExportProfile("export-test")

		// Delete original
		pm.DeleteProfile(ctx, "export-test")

		// Import
		imported, err := pm.ImportProfile(ctx, data)
		if err != nil {
			t.Fatalf("failed to import profile: %v", err)
		}

		if imported.Name != "Export Test" {
			t.Errorf("expected name 'Export Test', got '%s'", imported.Name)
		}
	})
}

func TestProfileManager_CreateDefaultProfile(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "profile-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	pm, _ := NewProfileManager(tempDir)
	ctx := context.Background()

	t.Run("create default", func(t *testing.T) {
		profile, err := pm.CreateDefaultProfile(ctx)
		if err != nil {
			t.Fatalf("failed to create default profile: %v", err)
		}

		if profile.ID != "default" {
			t.Errorf("expected ID 'default', got '%s'", profile.ID)
		}
		if !profile.IsDefault {
			t.Error("expected profile to be default")
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		profile, err := pm.CreateDefaultProfile(ctx)
		if err != nil {
			t.Fatalf("failed to create default profile: %v", err)
		}

		// Should return existing default
		if profile.ID != "default" {
			t.Errorf("expected ID 'default', got '%s'", profile.ID)
		}
	})
}

func TestProfile_WithCookies(t *testing.T) {
	expires := time.Now().Add(24 * time.Hour)
	profile := &Profile{
		ID:   "cookie-test",
		Name: "Cookie Test",
		Cookies: []Cookie{
			{
				Name:     "session",
				Value:    "abc123",
				Domain:   "example.com",
				Path:     "/",
				Expires:  &expires,
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Strict",
			},
		},
	}

	if len(profile.Cookies) != 1 {
		t.Errorf("expected 1 cookie, got %d", len(profile.Cookies))
	}
	if profile.Cookies[0].Name != "session" {
		t.Errorf("expected cookie name 'session', got '%s'", profile.Cookies[0].Name)
	}
}

func TestProfile_WithProxy(t *testing.T) {
	profile := &Profile{
		ID:   "proxy-test",
		Name: "Proxy Test",
		Proxy: &ProxyConfig{
			Server:   "http://proxy.example.com:23456",
			Username: "user",
			Password: "pass",
			Bypass:   []string{"localhost", "127.0.0.1"},
		},
	}

	if profile.Proxy == nil {
		t.Fatal("expected non-nil proxy config")
	}
	if profile.Proxy.Server != "http://proxy.example.com:23456" {
		t.Errorf("expected proxy server, got '%s'", profile.Proxy.Server)
	}
}

func TestProfile_WithGeolocation(t *testing.T) {
	profile := &Profile{
		ID:   "geo-test",
		Name: "Geolocation Test",
		Geolocation: &Geolocation{
			Latitude:  37.7749,
			Longitude: -122.4194,
			Accuracy:  100,
		},
	}

	if profile.Geolocation == nil {
		t.Fatal("expected non-nil geolocation")
	}
	if profile.Geolocation.Latitude != 37.7749 {
		t.Errorf("expected latitude 37.7749, got %f", profile.Geolocation.Latitude)
	}
}
