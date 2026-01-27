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
		time.Sleep(150 * time.Millisecond) // Wait for async update from previous test

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

func TestAPIKeyService_RotateKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer func() {
		time.Sleep(200 * time.Millisecond)
		svc.Close()
	}()

	t.Run("rotate key successfully", func(t *testing.T) {
		// Create original key
		originalKey, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read", "write"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		// Rotate the key
		result, err := svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:       originalKey.ID,
			UserID:      "user-123",
			GracePeriod: 24 * time.Hour,
		})
		if err != nil {
			t.Fatalf("failed to rotate key: %v", err)
		}

		// Check old key info
		if result.OldKey.RotatedTo == nil {
			t.Error("old key should have rotated_to set")
		}
		if result.OldKey.RotatedAt == nil {
			t.Error("old key should have rotated_at set")
		}
		if result.OldKey.GracePeriod == nil {
			t.Error("old key should have grace_period set")
		}

		// Check new key info
		if result.NewKey.Key == "" {
			t.Error("new key should have key value")
		}
		if result.NewKey.RotatedFrom == nil || *result.NewKey.RotatedFrom != originalKey.ID {
			t.Error("new key should have rotated_from set to original key ID")
		}
		if result.NewKey.Name != originalKey.Name {
			t.Error("new key should inherit name from original")
		}
		if len(result.NewKey.Scopes) != len(originalKey.Scopes) {
			t.Error("new key should inherit scopes from original")
		}

		// Both keys should be valid during grace period
		_, err = svc.ValidateKey(context.Background(), originalKey.Key)
		if err != nil {
			t.Errorf("original key should still be valid during grace period: %v", err)
		}

		time.Sleep(150 * time.Millisecond) // Wait for async update

		_, err = svc.ValidateKey(context.Background(), result.NewKey.Key)
		if err != nil {
			t.Errorf("new key should be valid: %v", err)
		}

		time.Sleep(150 * time.Millisecond) // Wait for async update
	})

	t.Run("rotate already rotated key should fail", func(t *testing.T) {
		time.Sleep(100 * time.Millisecond) // Wait for previous test's async operations

		// Create and rotate a key
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-234",
			Name:   "test-key-2",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		_, err = svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  key.ID,
			UserID: "user-234",
		})
		if err != nil {
			t.Fatalf("first rotation should succeed: %v", err)
		}

		// Try to rotate again
		_, err = svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  key.ID,
			UserID: "user-234",
		})
		if err != ErrRotationPending {
			t.Errorf("expected ErrRotationPending, got %v", err)
		}
	})

	t.Run("rotate revoked key should fail", func(t *testing.T) {
		time.Sleep(100 * time.Millisecond) // Wait for previous test's async operations

		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-345",
			Name:   "test-key-3",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		err = svc.RevokeKey(context.Background(), key.ID, "user-345")
		if err != nil {
			t.Fatalf("failed to revoke key: %v", err)
		}

		_, err = svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  key.ID,
			UserID: "user-345",
		})
		if err != ErrAPIKeyRevoked {
			t.Errorf("expected ErrAPIKeyRevoked, got %v", err)
		}
	})

	t.Run("rotate non-existent key should fail", func(t *testing.T) {
		_, err := svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  "non-existent",
			UserID: "user-123",
		})
		if err != ErrAPIKeyNotFound {
			t.Errorf("expected ErrAPIKeyNotFound, got %v", err)
		}
	})

	t.Run("rotate key by different user should fail", func(t *testing.T) {
		time.Sleep(100 * time.Millisecond) // Wait for previous test's async operations

		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-456",
			Name:   "test-key-4",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		_, err = svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  key.ID,
			UserID: "user-789",
		})
		if err != ErrAPIKeyNotFound {
			t.Errorf("expected ErrAPIKeyNotFound, got %v", err)
		}
	})
}

func TestAPIKeyService_CompleteRotation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer func() {
		time.Sleep(200 * time.Millisecond)
		svc.Close()
	}()

	t.Run("complete rotation", func(t *testing.T) {
		// Create and rotate a key
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		result, err := svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  key.ID,
			UserID: "user-123",
		})
		if err != nil {
			t.Fatalf("failed to rotate key: %v", err)
		}

		// Complete the rotation
		err = svc.CompleteRotation(context.Background(), key.ID, "user-123")
		if err != nil {
			t.Fatalf("failed to complete rotation: %v", err)
		}

		// Old key should now be invalid
		_, err = svc.ValidateKey(context.Background(), key.Key)
		if err != ErrAPIKeyRevoked {
			t.Errorf("expected ErrAPIKeyRevoked, got %v", err)
		}

		// New key should still be valid
		time.Sleep(100 * time.Millisecond)
		_, err = svc.ValidateKey(context.Background(), result.NewKey.Key)
		if err != nil {
			t.Errorf("new key should still be valid: %v", err)
		}
	})
}

func TestAPIKeyService_CleanupExpiredRotations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	t.Run("cleanup expired rotations", func(t *testing.T) {
		// Create and rotate a key with very short grace period
		key, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		_, err = svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:       key.ID,
			UserID:      "user-123",
			GracePeriod: 1 * time.Millisecond, // Very short grace period
		})
		if err != nil {
			t.Fatalf("failed to rotate key: %v", err)
		}

		// Wait for grace period to expire
		time.Sleep(10 * time.Millisecond)

		// Cleanup should revoke the old key
		count, err := svc.CleanupExpiredRotations(context.Background())
		if err != nil {
			t.Fatalf("failed to cleanup: %v", err)
		}

		if count != 1 {
			t.Errorf("expected 1 key to be cleaned up, got %d", count)
		}

		// Old key should now be revoked
		_, err = svc.ValidateKey(context.Background(), key.Key)
		if err != ErrAPIKeyRevoked {
			t.Errorf("expected ErrAPIKeyRevoked, got %v", err)
		}
	})
}

func TestAPIKeyService_GetRotationHistory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	svc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	t.Run("get rotation history", func(t *testing.T) {
		// Create original key
		key1, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		// Rotate twice
		result1, err := svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  key1.ID,
			UserID: "user-123",
		})
		if err != nil {
			t.Fatalf("failed to rotate key: %v", err)
		}

		result2, err := svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  result1.NewKey.ID,
			UserID: "user-123",
		})
		if err != nil {
			t.Fatalf("failed to rotate key: %v", err)
		}

		// Get history from any key in the chain
		history, err := svc.GetRotationHistory(context.Background(), result2.NewKey.ID, "user-123")
		if err != nil {
			t.Fatalf("failed to get history: %v", err)
		}

		if len(history) != 3 {
			t.Errorf("expected 3 keys in history, got %d", len(history))
		}

		// First should be original
		if history[0].ID != key1.ID {
			t.Error("first key in history should be original")
		}

		// Last should be newest
		if history[2].ID != result2.NewKey.ID {
			t.Error("last key in history should be newest")
		}
	})
}

func TestAPIKeyService_WithEncryption(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_encrypted_test.db")

	// Create encryptor
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	enc, err := NewEncryptor(&EncryptionConfig{Key: key})
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	svc, err := NewAPIKeyService(dbPath, WithEncryption(enc))
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	t.Run("create key with encryption", func(t *testing.T) {
		apiKey, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "encrypted-key",
			Scopes: []string{"read", "write"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		// Key should be returned on creation
		if apiKey.Key == "" {
			t.Error("key should not be empty on creation")
		}

		// Validate the key works
		info, err := svc.ValidateKey(context.Background(), apiKey.Key)
		if err != nil {
			t.Fatalf("failed to validate key: %v", err)
		}

		if info.UserID != "user-123" {
			t.Errorf("expected user_id 'user-123', got '%s'", info.UserID)
		}

		// Get decrypted key
		decrypted, err := svc.GetDecryptedKey(context.Background(), apiKey.ID, "user-123")
		if err != nil {
			t.Fatalf("failed to get decrypted key: %v", err)
		}

		if decrypted != apiKey.Key {
			t.Error("decrypted key should match original")
		}
	})

	t.Run("rotate key with encryption", func(t *testing.T) {
		apiKey, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-456",
			Name:   "rotate-test",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		result, err := svc.RotateKey(context.Background(), &RotateKeyRequest{
			KeyID:  apiKey.ID,
			UserID: "user-456",
		})
		if err != nil {
			t.Fatalf("failed to rotate key: %v", err)
		}

		// New key should be returned
		if result.NewKey.Key == "" {
			t.Error("new key should not be empty")
		}

		// Get decrypted new key
		decrypted, err := svc.GetDecryptedKey(context.Background(), result.NewKey.ID, "user-456")
		if err != nil {
			t.Fatalf("failed to get decrypted key: %v", err)
		}

		if decrypted != result.NewKey.Key {
			t.Error("decrypted key should match new key")
		}
	})
}

func TestAPIKeyService_ReEncryptAllKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_reencrypt_test.db")

	// Create old encryptor
	oldKey, _ := GenerateKey()
	oldEnc, _ := NewEncryptor(&EncryptionConfig{Key: oldKey})

	svc, err := NewAPIKeyService(dbPath, WithEncryption(oldEnc))
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	defer svc.Close()

	// Create some keys
	var createdKeys []string
	for i := 0; i < 3; i++ {
		apiKey, err := svc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}
		createdKeys = append(createdKeys, apiKey.Key)
	}

	// Create new encryptor
	newKey, _ := GenerateKey()
	newEnc, _ := NewEncryptor(&EncryptionConfig{Key: newKey})

	// Re-encrypt all keys
	count, err := svc.ReEncryptAllKeys(context.Background(), oldEnc, newEnc)
	if err != nil {
		t.Fatalf("failed to re-encrypt keys: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 keys re-encrypted, got %d", count)
	}

	// Update service to use new encryptor
	svc.encryptor = newEnc

	// Verify keys can still be decrypted
	keys, _ := svc.ListKeys(context.Background(), "user-123")
	for i, k := range keys {
		decrypted, err := svc.GetDecryptedKey(context.Background(), k.ID, "user-123")
		if err != nil {
			t.Fatalf("failed to decrypt key %d: %v", i, err)
		}

		if decrypted != createdKeys[2-i] { // Keys are returned in reverse order (newest first)
			t.Errorf("decrypted key %d does not match original", i)
		}
	}
}
