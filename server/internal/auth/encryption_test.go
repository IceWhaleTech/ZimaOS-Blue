package auth

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"
)

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	enc, err := NewEncryptor(&EncryptionConfig{Key: key})
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	t.Run("encrypt and decrypt bytes", func(t *testing.T) {
		plaintext := []byte("secret api key data")

		ciphertext, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}

		if ciphertext == string(plaintext) {
			t.Error("ciphertext should not equal plaintext")
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("failed to decrypt: %v", err)
		}

		if !bytes.Equal(decrypted, plaintext) {
			t.Errorf("decrypted data does not match original: got %s, want %s", decrypted, plaintext)
		}
	})

	t.Run("encrypt and decrypt string", func(t *testing.T) {
		plaintext := "ek_abc123def456"

		ciphertext, err := enc.EncryptString(plaintext)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}

		decrypted, err := enc.DecryptString(ciphertext)
		if err != nil {
			t.Fatalf("failed to decrypt: %v", err)
		}

		if decrypted != plaintext {
			t.Errorf("decrypted string does not match original: got %s, want %s", decrypted, plaintext)
		}
	})

	t.Run("different encryptions produce different ciphertexts", func(t *testing.T) {
		plaintext := []byte("same data")

		ct1, _ := enc.Encrypt(plaintext)
		ct2, _ := enc.Encrypt(plaintext)

		if ct1 == ct2 {
			t.Error("encrypting same data twice should produce different ciphertexts (due to random nonce)")
		}
	})
}

func TestEncryptor_WithPassphrase(t *testing.T) {
	enc, err := NewEncryptor(&EncryptionConfig{
		Passphrase: "my-secure-passphrase",
		Salt:       []byte("custom-salt-value"),
	})
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := "sensitive data"
	ciphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	decrypted, err := enc.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted does not match: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptor_WithKeyPath(t *testing.T) {
	// Create a temporary key file
	key, _ := GenerateKey()
	keyPath := t.TempDir() + "/test.key"

	// Write key as base64
	keyBase64 := base64.StdEncoding.EncodeToString(key)
	if err := writeTestFile(keyPath, []byte(keyBase64)); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	enc, err := NewEncryptor(&EncryptionConfig{KeyPath: keyPath})
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := "test data"
	ciphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	decrypted, err := enc.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted does not match: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptor_InvalidCiphertext(t *testing.T) {
	key, _ := GenerateKey()
	enc, _ := NewEncryptor(&EncryptionConfig{Key: key})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := enc.Decrypt("not-valid-base64!!!")
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})

	t.Run("too short ciphertext", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := enc.Decrypt(short)
		if err == nil {
			t.Error("expected error for too short ciphertext")
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		ciphertext, _ := enc.EncryptString("original")
		// Tamper with the ciphertext
		data, _ := base64.StdEncoding.DecodeString(ciphertext)
		data[len(data)-1] ^= 0xFF // Flip bits in last byte
		tampered := base64.StdEncoding.EncodeToString(data)

		_, err := enc.DecryptString(tampered)
		if err == nil {
			t.Error("expected error for tampered ciphertext")
		}
	})
}

func TestEncryptor_KeyRotation(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	enc, _ := NewEncryptor(&EncryptionConfig{Key: key1})

	// Encrypt with original key
	plaintext := "secret data"
	ciphertext, _ := enc.EncryptString(plaintext)

	// Rotate to new key
	if err := enc.RotateKey(key2); err != nil {
		t.Fatalf("failed to rotate key: %v", err)
	}

	// Old ciphertext should fail to decrypt with new key
	_, err := enc.DecryptString(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with rotated key")
	}

	// New encryption should work
	newCiphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt with new key: %v", err)
	}

	decrypted, err := enc.DecryptString(newCiphertext)
	if err != nil {
		t.Fatalf("failed to decrypt with new key: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted does not match: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptor_NoKey(t *testing.T) {
	_, err := NewEncryptor(nil)
	if err != ErrKeyNotConfigured {
		t.Errorf("expected ErrKeyNotConfigured, got %v", err)
	}

	_, err = NewEncryptor(&EncryptionConfig{})
	if err != ErrKeyNotConfigured {
		t.Errorf("expected ErrKeyNotConfigured, got %v", err)
	}
}

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("expected 32 byte key, got %d bytes", len(key1))
	}

	key2, _ := GenerateKey()
	if bytes.Equal(key1, key2) {
		t.Error("generated keys should be unique")
	}
}

func TestGenerateKeyBase64(t *testing.T) {
	keyStr, err := GenerateKeyBase64()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		t.Fatalf("failed to decode key: %v", err)
	}

	if len(decoded) != 32 {
		t.Errorf("expected 32 byte key, got %d bytes", len(decoded))
	}
}

func TestDeriveKey(t *testing.T) {
	passphrase := "my-passphrase"
	salt := []byte("my-salt")

	key1 := deriveKey(passphrase, salt)
	key2 := deriveKey(passphrase, salt)

	if !bytes.Equal(key1, key2) {
		t.Error("same passphrase and salt should produce same key")
	}

	key3 := deriveKey(passphrase, []byte("different-salt"))
	if bytes.Equal(key1, key3) {
		t.Error("different salt should produce different key")
	}

	key4 := deriveKey("different-passphrase", salt)
	if bytes.Equal(key1, key4) {
		t.Error("different passphrase should produce different key")
	}
}

func writeTestFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
