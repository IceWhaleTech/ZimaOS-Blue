package oidc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewKeyManager(t *testing.T) {
	// Create temp directory for test keys
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")

	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	if km == nil {
		t.Fatal("NewKeyManager() returned nil")
	}

	// Verify key file was created
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("NewKeyManager() did not create key file")
	}
}

func TestKeyManager_GetCurrentKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	key, err := km.GetCurrentKey()
	if err != nil {
		t.Fatalf("GetCurrentKey() error = %v", err)
	}

	if key == nil {
		t.Fatal("GetCurrentKey() returned nil")
	}

	if key.ID == "" {
		t.Error("GetCurrentKey() key ID is empty")
	}

	if key.PrivateKey == nil {
		t.Error("GetCurrentKey() PrivateKey is nil")
	}

	if key.CreatedAt.IsZero() {
		t.Error("GetCurrentKey() CreatedAt is zero")
	}

	if key.ExpiresAt.IsZero() {
		t.Error("GetCurrentKey() ExpiresAt is zero")
	}

	if !key.ExpiresAt.After(key.CreatedAt) {
		t.Error("GetCurrentKey() ExpiresAt should be after CreatedAt")
	}
}

func TestKeyManager_GetKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	currentKey, _ := km.GetCurrentKey()

	// Get by valid ID
	key, err := km.GetKey(currentKey.ID)
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	if key.ID != currentKey.ID {
		t.Errorf("GetKey() ID = %v, want %v", key.ID, currentKey.ID)
	}

	// Get by invalid ID
	_, err = km.GetKey("invalid-key-id")
	if err != ErrKeyNotFound {
		t.Errorf("GetKey(invalid) error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestKeyManager_GetJWKS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	jwks := km.GetJWKS()
	if jwks == nil {
		t.Fatal("GetJWKS() returned nil")
	}

	if len(jwks.Keys) == 0 {
		t.Error("GetJWKS() returned empty key set")
	}

	// Verify key properties
	for _, key := range jwks.Keys {
		if key.KeyID == "" {
			t.Error("GetJWKS() key has empty KeyID")
		}
		if key.Algorithm != "RS256" {
			t.Errorf("GetJWKS() key Algorithm = %v, want RS256", key.Algorithm)
		}
		if key.Use != "sig" {
			t.Errorf("GetJWKS() key Use = %v, want sig", key.Use)
		}
	}
}

func TestKeyManager_Signer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	signer, err := km.Signer()
	if err != nil {
		t.Fatalf("Signer() error = %v", err)
	}

	if signer == nil {
		t.Fatal("Signer() returned nil")
	}
}

func TestKeyManager_RotateKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")
	// Use 0 rotation days to force rotation
	km, err := NewKeyManager(keyPath, 0)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	oldKey, _ := km.GetCurrentKey()

	// Wait a moment to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	// Force rotation by setting expiry in the past
	km.mu.Lock()
	km.keys[km.currentKeyID].ExpiresAt = time.Now().Add(-time.Hour)
	km.mu.Unlock()

	err = km.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}

	newKey, _ := km.GetCurrentKey()

	if newKey.ID == oldKey.ID {
		t.Error("RotateKey() did not create a new key")
	}
}

func TestKeyManager_LoadExistingKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-keys-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "test.key")

	// Create first key manager
	km1, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	key1, _ := km1.GetCurrentKey()

	// Create second key manager with same path - should load existing key
	km2, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() second error = %v", err)
	}

	key2, _ := km2.GetCurrentKey()

	// Keys should have the same private key (loaded from file)
	if key1.PrivateKey.N.Cmp(key2.PrivateKey.N) != 0 {
		t.Error("NewKeyManager() did not load existing key")
	}
}

func TestKeyManager_InvalidKeyPath(t *testing.T) {
	// Create a parent path that is a file, so mkdir/write must fail consistently.
	tmpDir := t.TempDir()
	blockedParent := filepath.Join(tmpDir, "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocked parent) error = %v", err)
	}
	keyPath := filepath.Join(blockedParent, "test.key")

	_, err := NewKeyManager(keyPath, 90)
	if err == nil {
		t.Error("NewKeyManager() expected error for invalid path")
	}
}
