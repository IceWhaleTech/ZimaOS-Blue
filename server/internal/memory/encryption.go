package memory

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	encPrefix    = "ENC:v1:"
	saltLen      = 16
	nonceLen     = 12
	keyLen       = 32 // AES-256
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
)

// ContentEncryptor provides AES-256-GCM encryption/decryption for memory content.
type ContentEncryptor struct {
	mu      sync.RWMutex
	key     []byte
	salt    []byte
	keyPath string
	enabled bool
}

// NewContentEncryptor creates an encryptor. If enabled and passphrase is non-empty,
// it derives a key via Argon2id. The salt is loaded from keyPath or generated fresh.
func NewContentEncryptor(passphrase, keyPath string, enabled bool) (*ContentEncryptor, error) {
	e := &ContentEncryptor{
		keyPath: keyPath,
		enabled: enabled,
	}
	if !enabled || passphrase == "" {
		return e, nil
	}
	salt, err := e.loadOrCreateSalt(keyPath)
	if err != nil {
		return nil, fmt.Errorf("encryption salt: %w", err)
	}
	e.salt = salt
	e.key = deriveKey(passphrase, salt)
	return e, nil
}

// IsEnabled returns whether encryption is active.
func (e *ContentEncryptor) IsEnabled() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// IsEncrypted checks if content has the encryption prefix.
func (e *ContentEncryptor) IsEncrypted(content string) bool {
	return strings.HasPrefix(content, encPrefix)
}

// Encrypt encrypts plaintext. Returns plaintext unchanged if disabled.
func (e *ContentEncryptor) Encrypt(plaintext string) (string, error) {
	if e == nil || !e.IsEnabled() || len(e.key) == 0 {
		return plaintext, nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Format: ENC:v1:<b64salt>:<b64nonce>:<b64ciphertext>
	return encPrefix +
		base64.RawStdEncoding.EncodeToString(e.salt) + ":" +
		base64.RawStdEncoding.EncodeToString(nonce) + ":" +
		base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts content. If content lacks the ENC prefix, returns it as-is.
func (e *ContentEncryptor) Decrypt(content string) (string, error) {
	if e == nil || !strings.HasPrefix(content, encPrefix) {
		return content, nil
	}
	if len(e.key) == 0 {
		return "", fmt.Errorf("encryption key not initialized")
	}

	parts := strings.SplitN(content[len(encPrefix):], ":", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed encrypted content")
	}

	nonce, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// SetEnabled toggles encryption on/off at runtime.
func (e *ContentEncryptor) SetEnabled(enabled bool) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = enabled
}

// InitKey initializes the key from a passphrase (used when enabling at runtime).
func (e *ContentEncryptor) InitKey(passphrase string) error {
	if e == nil {
		return fmt.Errorf("nil encryptor")
	}
	salt, err := e.loadOrCreateSalt(e.keyPath)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.salt = salt
	e.key = deriveKey(passphrase, salt)
	return nil
}

// RotateKey derives a new key from a new passphrase with a fresh salt.
func (e *ContentEncryptor) RotateKey(newPassphrase string) ([]byte, error) {
	if e == nil {
		return nil, fmt.Errorf("nil encryptor")
	}
	newSalt := make([]byte, saltLen)
	if _, err := rand.Read(newSalt); err != nil {
		return nil, err
	}
	if err := e.persistSalt(e.keyPath, newSalt); err != nil {
		return nil, err
	}
	newKey := deriveKey(newPassphrase, newSalt)
	e.mu.Lock()
	defer e.mu.Unlock()
	oldKey := e.key
	e.salt = newSalt
	e.key = newKey
	return oldKey, nil
}

func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, keyLen)
}

func (e *ContentEncryptor) loadOrCreateSalt(keyPath string) ([]byte, error) {
	if keyPath != "" {
		data, err := os.ReadFile(keyPath)
		if err == nil && len(data) == saltLen {
			return data, nil
		}
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if keyPath != "" {
		if err := e.persistSalt(keyPath, salt); err != nil {
			return nil, err
		}
	}
	return salt, nil
}

func (e *ContentEncryptor) persistSalt(keyPath string, salt []byte) error {
	if keyPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return err
	}
	return os.WriteFile(keyPath, salt, 0600)
}
