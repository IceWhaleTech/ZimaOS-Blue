package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func tempKeyPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.key")
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewContentEncryptor("test-passphrase", tempKeyPath(t), true)
	if err != nil {
		t.Fatal(err)
	}
	original := "Hello, this is a secret memory about API keys and passwords."
	ciphertext, err := enc.Encrypt(original)
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == original {
		t.Fatal("ciphertext should differ from plaintext")
	}
	if !enc.IsEncrypted(ciphertext) {
		t.Fatal("should detect encrypted content")
	}
	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != original {
		t.Fatalf("got %q, want %q", plaintext, original)
	}
}

func TestDecryptPlaintext(t *testing.T) {
	enc, err := NewContentEncryptor("pass", tempKeyPath(t), true)
	if err != nil {
		t.Fatal(err)
	}
	plain := "just a normal string"
	result, err := enc.Decrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if result != plain {
		t.Fatalf("plaintext passthrough failed: got %q", result)
	}
}

func TestDisabledEncryptor(t *testing.T) {
	enc, err := NewContentEncryptor("pass", tempKeyPath(t), false)
	if err != nil {
		t.Fatal(err)
	}
	if enc.IsEnabled() {
		t.Fatal("should be disabled")
	}
	text := "should not be encrypted"
	result, err := enc.Encrypt(text)
	if err != nil {
		t.Fatal(err)
	}
	if result != text {
		t.Fatal("disabled encryptor should return plaintext")
	}
}

func TestNilEncryptor(t *testing.T) {
	var enc *ContentEncryptor
	if enc.IsEnabled() {
		t.Fatal("nil encryptor should not be enabled")
	}
	text := "test"
	result, err := enc.Encrypt(text)
	if err != nil {
		t.Fatal(err)
	}
	if result != text {
		t.Fatal("nil encryptor should passthrough")
	}
	result, err = enc.Decrypt(text)
	if err != nil {
		t.Fatal(err)
	}
	if result != text {
		t.Fatal("nil encryptor should passthrough decrypt")
	}
}

func TestDifferentNonces(t *testing.T) {
	enc, err := NewContentEncryptor("pass", tempKeyPath(t), true)
	if err != nil {
		t.Fatal(err)
	}
	text := "same input"
	c1, _ := enc.Encrypt(text)
	c2, _ := enc.Encrypt(text)
	if c1 == c2 {
		t.Fatal("two encryptions of same plaintext should produce different ciphertexts")
	}
	// Both should decrypt to the same value
	p1, _ := enc.Decrypt(c1)
	p2, _ := enc.Decrypt(c2)
	if p1 != text || p2 != text {
		t.Fatal("both should decrypt correctly")
	}
}

func TestWrongPassphrase(t *testing.T) {
	keyPath := tempKeyPath(t)
	enc1, _ := NewContentEncryptor("correct-pass", keyPath, true)
	ciphertext, _ := enc1.Encrypt("secret data")

	// Create a new encryptor with wrong passphrase but same salt file
	enc2, _ := NewContentEncryptor("wrong-pass", keyPath, true)
	_, err := enc2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("should fail with wrong passphrase")
	}
}

func TestEmptyContent(t *testing.T) {
	enc, err := NewContentEncryptor("pass", tempKeyPath(t), true)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := enc.Encrypt("")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "" {
		t.Fatalf("expected empty string, got %q", plaintext)
	}
}

func TestSaltPersistence(t *testing.T) {
	keyPath := tempKeyPath(t)
	enc1, _ := NewContentEncryptor("pass", keyPath, true)
	ciphertext, _ := enc1.Encrypt("persistent test")

	// Create new encryptor with same passphrase and key path — should load same salt
	enc2, _ := NewContentEncryptor("pass", keyPath, true)
	plaintext, err := enc2.Decrypt(ciphertext)
	if err != nil {
		t.Fatal("should decrypt with same passphrase + persisted salt")
	}
	if plaintext != "persistent test" {
		t.Fatalf("got %q", plaintext)
	}
}

func TestIsEncrypted(t *testing.T) {
	enc, _ := NewContentEncryptor("pass", tempKeyPath(t), true)
	if enc.IsEncrypted("hello") {
		t.Fatal("plain text should not be detected as encrypted")
	}
	if !enc.IsEncrypted("ENC:v1:abc:def:ghi") {
		t.Fatal("prefixed content should be detected as encrypted")
	}
}

func TestKeyRotation(t *testing.T) {
	keyPath := tempKeyPath(t)
	enc, _ := NewContentEncryptor("old-pass", keyPath, true)
	ciphertext, _ := enc.Encrypt("rotate me")

	// Read old salt for verification
	oldSalt, _ := os.ReadFile(keyPath)

	oldKey, err := enc.RotateKey("new-pass")
	if err != nil {
		t.Fatal(err)
	}
	if oldKey == nil {
		t.Fatal("should return old key")
	}

	// Salt file should have changed
	newSalt, _ := os.ReadFile(keyPath)
	if string(oldSalt) == string(newSalt) {
		t.Fatal("salt should change after rotation")
	}

	// Old ciphertext can't be decrypted with new key
	_, err = enc.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("old ciphertext should not decrypt with new key")
	}
}
