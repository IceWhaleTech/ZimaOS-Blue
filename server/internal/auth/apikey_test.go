package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAPIKeyService_CreateKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	t.Run("create api key", func(t *testing.T) {
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID:      "user-123",
			Name:        "test-key",
			Scopes:      []string{"read", "write"},
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		if key.ID == "" {
			t.Error("key ID should not be empty")
		}
		if key.Key == "" {
			t.Error("key should not be empty")
		}
		if key.Prefix == "" {
			t.Error("key prefix should not be empty")
		}
		if key.Name != "test-key" {
			t.Errorf("expected name 'test-key', got '%s'", key.Name)
		}
		if key.UserID != "user-123" {
			t.Errorf("expected user_id 'user-123', got '%s'", key.UserID)
		}
	})

	t.Run("create key without expiration", func(t *testing.T) {
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "no-expiry-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		if key.ExpiresAt != nil {
			t.Error("key should not have expiration")
		}
	})
}

func TestAPIKeyService_ValidateKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer func() {
		time.Sleep(200 * time.Millisecond)
		svc.Close()
	}()

	t.Run("validate valid key", func(t *testing.T) {
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read", "write"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		info, err := svc.ValidateKey(context.Background(), key.Key)
		if err != nil {
			t.Fatalf("failed to validate key: %v", err)
		}

		if info.UserID != "user-123" {
			t.Errorf("expected user_id 'user-123', got '%s'", info.UserID)
		}
		if len(info.Scopes) != 2 {
			t.Errorf("expected 2 scopes, got %d", len(info.Scopes))
		}
	})

	t.Run("validate invalid key", func(t *testing.T) {
		_, err := svc.ValidateKey(context.Background(), "invalid-key")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})

	t.Run("validate expired key", func(t *testing.T) {
		expiredTime := time.Now().Add(-time.Hour)
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID:    "user-123",
			Name:      "expired-key",
			Scopes:    []string{"read"},
			ExpiresAt: expiredTime,
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		_, err = svc.ValidateKey(context.Background(), key.Key)
		if err == nil {
			t.Fatal("expected error for expired key")
		}
	})
}

func TestAPIKeyService_ListKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	// Create some keys
	for i := 0; i < 3; i++ {
		_, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}
	}

	// Create key for different user
	_, err = svc.CreateKey(context.Background(), &CreateKeyRequest{
		UserID: "user-456",
		Name:   "other-key",
		Scopes: []string{"read"},
	})
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	t.Run("list keys for user", func(t *testing.T) {
		keys, err := svc.ListKeys(context.Background(), "user-123")
		if err != nil {
			t.Fatalf("failed to list keys: %v", err)
		}

		if len(keys) != 3 {
			t.Errorf("expected 3 keys, got %d", len(keys))
		}

		// Keys should not contain the actual key value
		for _, k := range keys {
			if k.Key != "" {
				t.Error("listed keys should not contain actual key value")
			}
		}
	})
}

func TestAPIKeyService_RevokeKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer func() {
		time.Sleep(200 * time.Millisecond)
		svc.Close()
	}()

	t.Run("revoke key", func(t *testing.T) {
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		// Key should be valid before revocation
		_, err = svc.ValidateKey(context.Background(), key.Key)
		if err != nil {
			t.Fatalf("key should be valid before revocation: %v", err)
		}

		// Wait for async last_used update to complete
		time.Sleep(100 * time.Millisecond)

		// Revoke the key
		err = svc.RevokeKey(context.Background(), key.ID, "user-123")
		if err != nil {
			t.Fatalf("failed to revoke key: %v", err)
		}

		// Key should be invalid after revocation
		_, err = svc.ValidateKey(context.Background(), key.Key)
		if err == nil {
			t.Fatal("expected error for revoked key")
		}
	})

	t.Run("revoke key by different user should fail", func(t *testing.T) {
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		err = svc.RevokeKey(context.Background(), key.ID, "user-456")
		if err == nil {
			t.Fatal("expected error when revoking key by different user")
		}
	})
}

func TestAPIKeyService_HasScope(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer func() {
		time.Sleep(200 * time.Millisecond)
		svc.Close()
	}()

	key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
		UserID: "user-123",
		Name:   "test-key",
		Scopes: []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	info, err := svc.ValidateKey(context.Background(), key.Key)
	if err != nil {
		t.Fatalf("failed to validate key: %v", err)
	}

	t.Run("has scope", func(t *testing.T) {
		if !info.HasScope("read") {
			t.Error("should have 'read' scope")
		}
		if !info.HasScope("write") {
			t.Error("should have 'write' scope")
		}
	})

	t.Run("does not have scope", func(t *testing.T) {
		if info.HasScope("delete") {
			t.Error("should not have 'delete' scope")
		}
	})

	t.Run("wildcard scope", func(t *testing.T) {
		// Wait for any pending async operations
		time.Sleep(100 * time.Millisecond)

		wildcardKey, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "admin-key",
			Scopes: []string{"*"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		wildcardInfo, err := svc.ValidateKey(context.Background(), wildcardKey.Key)
		if err != nil {
			t.Fatalf("failed to validate key: %v", err)
		}

		if !wildcardInfo.HasScope("anything") {
			t.Error("wildcard scope should match any scope")
		}
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
