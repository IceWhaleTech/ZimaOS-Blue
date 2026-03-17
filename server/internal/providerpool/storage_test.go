package providerpool

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func TestHashAPIKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"sk-1807890abcdef", "sk-18078...cdef"},
		{"short", "*****"},
		{"exactly12ch", "***********"},
		{"sk-proj-1807890abcdefghijklmnop", "sk-proj-...mnop"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := HashAPIKey(tt.key)
			if result != tt.expected {
				t.Errorf("HashAPIKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("prov")
	id2 := GenerateID("prov")

	if id1 == id2 {
		t.Error("GenerateID should produce unique IDs")
	}

	if len(id1) < 10 {
		t.Errorf("GenerateID produced too short ID: %s", id1)
	}
}

func TestFileStorage(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "providerpool-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}

	t.Run("SaveAndLoadProvider", func(t *testing.T) {
		provider := &Provider{
			ID:       "test-provider",
			Name:     "Test Provider",
			Type:     ProviderTypeCustom,
			Enabled:  true,
			Status:   ProviderStatusActive,
			BaseURL:  "https://api.test.com/v1",
			Priority: 10,
			APIKeys: []APIKey{
				{
					ID:      "key-1",
					Key:     "sk-secret-key-12345",
					KeyHash: HashAPIKey("sk-secret-key-12345"),
					Label:   "Primary",
					Enabled: true,
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Save
		if err := storage.SaveProvider(provider); err != nil {
			t.Fatalf("SaveProvider failed: %v", err)
		}

		// Load
		loaded, err := storage.LoadProvider("test-provider")
		if err != nil {
			t.Fatalf("LoadProvider failed: %v", err)
		}

		// Verify
		if loaded.ID != provider.ID {
			t.Errorf("ID mismatch: got %s, want %s", loaded.ID, provider.ID)
		}
		if loaded.Name != provider.Name {
			t.Errorf("Name mismatch: got %s, want %s", loaded.Name, provider.Name)
		}
		if len(loaded.APIKeys) != 1 {
			t.Fatalf("APIKeys count mismatch: got %d, want 1", len(loaded.APIKeys))
		}
		if loaded.APIKeys[0].Key != "sk-secret-key-12345" {
			t.Errorf("API key not decrypted correctly: got %s", loaded.APIKeys[0].Key)
		}
	})

	t.Run("LoadAllProviders", func(t *testing.T) {
		// Add another provider
		provider2 := &Provider{
			ID:        "test-provider-2",
			Name:      "Test Provider 2",
			Type:      ProviderTypeBuiltin,
			Enabled:   true,
			Status:    ProviderStatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := storage.SaveProvider(provider2); err != nil {
			t.Fatalf("SaveProvider failed: %v", err)
		}

		providers, err := storage.LoadAllProviders()
		if err != nil {
			t.Fatalf("LoadAllProviders failed: %v", err)
		}

		if len(providers) != 2 {
			t.Errorf("Expected 2 providers, got %d", len(providers))
		}
	})

	t.Run("DeleteProvider", func(t *testing.T) {
		if err := storage.DeleteProvider("test-provider-2"); err != nil {
			t.Fatalf("DeleteProvider failed: %v", err)
		}

		_, err := storage.LoadProvider("test-provider-2")
		if err != ErrProviderNotFound {
			t.Errorf("Expected ErrProviderNotFound, got %v", err)
		}
	})

	t.Run("SaveAndLoadModels", func(t *testing.T) {
		models := []*Model{
			{
				ID:          "gpt-4",
				ProviderID:  "test-provider",
				Name:        "gpt-4",
				DisplayName: "GPT-4",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
				},
				ContextWindow: 128000,
				InputPrice:    30.0,
				OutputPrice:   60.0,
			},
		}

		if err := storage.SaveModels("test-provider", models); err != nil {
			t.Fatalf("SaveModels failed: %v", err)
		}

		loaded, err := storage.LoadModels("test-provider")
		if err != nil {
			t.Fatalf("LoadModels failed: %v", err)
		}

		if len(loaded) != 1 {
			t.Fatalf("Expected 1 model, got %d", len(loaded))
		}
		if loaded[0].ID != "gpt-4" {
			t.Errorf("Model ID mismatch: got %s, want gpt-4", loaded[0].ID)
		}
	})

	t.Run("RejectsInvalidProviderIDs", func(t *testing.T) {
		err := storage.SaveProvider(&Provider{
			ID:   "../escape",
			Name: "Bad Provider",
			Type: ProviderTypeCustom,
		})
		if !errors.Is(err, ErrInvalidProviderID) {
			t.Fatalf("SaveProvider error = %v, want ErrInvalidProviderID", err)
		}

		err = storage.SaveModels("../escape", []*Model{})
		if !errors.Is(err, ErrInvalidProviderID) {
			t.Fatalf("SaveModels error = %v, want ErrInvalidProviderID", err)
		}

		if _, err := os.Stat(filepath.Join(tmpDir, "escape.json")); !os.IsNotExist(err) {
			t.Fatalf("expected no traversal output file, stat err=%v", err)
		}
	})

	t.Run("AppendAndLoadUsage", func(t *testing.T) {
		now := time.Now()
		record := &UsageRecord{
			ID:           GenerateID("usage"),
			ProviderID:   "test-provider",
			ModelID:      "gpt-4",
			Timestamp:    now,
			InputTokens:  1000,
			OutputTokens: 500,
			RequestCount: 1,
			LatencyMs:    250,
			Success:      true,
		}

		if err := storage.AppendUsage(record); err != nil {
			t.Fatalf("AppendUsage failed: %v", err)
		}

		// Load usage for today
		start := now.Truncate(24 * time.Hour)
		end := start.Add(24 * time.Hour)
		records, err := storage.LoadUsage("test-provider", start, end)
		if err != nil {
			t.Fatalf("LoadUsage failed: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(records))
		}
		if records[0].InputTokens != 1000 {
			t.Errorf("InputTokens mismatch: got %d, want 1000", records[0].InputTokens)
		}
	})
}

func TestFileStorage_EncryptsProviderSecretsAtRest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "providerpool-file-encrypted-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	enc, err := auth.NewEncryptor(&auth.EncryptionConfig{Key: []byte("01234567890123456789012345678901")})
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}

	storage, err := NewFileStorage(tmpDir, WithStorageEncryptor(enc))
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}

	provider := &Provider{
		ID:      "test-provider",
		Name:    "Test Provider",
		Type:    ProviderTypeCustom,
		Enabled: true,
		Status:  ProviderStatusActive,
		BaseURL: "https://api.test.com/v1",
		APIKeys: []APIKey{{
			ID:      "key-1",
			Key:     "sk-secret-key-12345",
			Enabled: true,
		}},
		OAuth: &OAuthConfig{AccessToken: "oauth-access-token"},
	}
	if err := storage.SaveProvider(provider); err != nil {
		t.Fatalf("SaveProvider failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(tmpDir, "providers.json"))
	if err != nil {
		t.Fatalf("Read providers.json failed: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "sk-secret-key-12345") || strings.Contains(text, "oauth-access-token") {
		t.Fatalf("providers.json should not contain plaintext secrets: %s", text)
	}
	if !strings.Contains(text, encryptedSecretPrefix) {
		t.Fatalf("providers.json should contain encrypted secret prefix, got: %s", text)
	}

	loaded, err := storage.LoadProvider("test-provider")
	if err != nil {
		t.Fatalf("LoadProvider failed: %v", err)
	}
	if len(loaded.APIKeys) != 1 || loaded.APIKeys[0].Key != "sk-secret-key-12345" {
		t.Fatalf("API key restore failed: %+v", loaded.APIKeys)
	}
	if loaded.OAuth == nil || loaded.OAuth.AccessToken != "oauth-access-token" {
		t.Fatalf("OAuth restore failed: %+v", loaded.OAuth)
	}
}

func TestFileStorageDirectoryCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "providerpool-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	basePath := filepath.Join(tmpDir, "nested", "path", "providers")
	_, err = NewFileStorage(basePath)
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}

	// Check directories were created
	dirs := []string{
		basePath,
		filepath.Join(basePath, "models"),
		filepath.Join(basePath, "usage"),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory not created: %s", dir)
		}
	}
}
