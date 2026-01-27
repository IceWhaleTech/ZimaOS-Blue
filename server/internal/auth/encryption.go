package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrKeyNotConfigured  = errors.New("encryption key not configured")
)

// EncryptionConfig holds configuration for API key encryption.
type EncryptionConfig struct {
	// Key is the master encryption key (32 bytes for AES-256).
	// Can be set directly or derived from a passphrase.
	Key []byte

	// KeyPath is the path to a file containing the encryption key.
	KeyPath string

	// Passphrase is used to derive the encryption key using PBKDF2.
	Passphrase string

	// Salt is used with the passphrase for key derivation.
	// If not provided, a default salt is used (not recommended for production).
	Salt []byte
}

// Encryptor handles encryption and decryption of sensitive data.
type Encryptor struct {
	key []byte
	mu  sync.RWMutex
}

// NewEncryptor creates a new Encryptor with the given configuration.
func NewEncryptor(cfg *EncryptionConfig) (*Encryptor, error) {
	if cfg == nil {
		return nil, ErrKeyNotConfigured
	}

	var key []byte
	var err error

	// Priority: Key > KeyPath > Passphrase
	if len(cfg.Key) > 0 {
		key = cfg.Key
	} else if cfg.KeyPath != "" {
		key, err = loadKeyFromFile(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load key from file: %w", err)
		}
	} else if cfg.Passphrase != "" {
		salt := cfg.Salt
		if len(salt) == 0 {
			// Default salt (should be overridden in production)
			salt = []byte("zimaos-echo-default-salt")
		}
		key = deriveKey(cfg.Passphrase, salt)
	} else {
		return nil, ErrKeyNotConfigured
	}

	// Ensure key is 32 bytes for AES-256
	if len(key) != 32 {
		// Hash the key to get exactly 32 bytes
		hash := sha256.Sum256(key)
		key = hash[:]
	}

	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.key) == 0 {
		return "", ErrKeyNotConfigured
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext that was encrypted with Encrypt.
func (e *Encryptor) Decrypt(ciphertext string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.key) == 0 {
		return nil, ErrKeyNotConfigured
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns the encrypted string.
func (e *Encryptor) EncryptString(plaintext string) (string, error) {
	return e.Encrypt([]byte(plaintext))
}

// DecryptString decrypts a string that was encrypted with EncryptString.
func (e *Encryptor) DecryptString(ciphertext string) (string, error) {
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// RotateKey rotates the encryption key. This should be called when re-encrypting data.
func (e *Encryptor) RotateKey(newKey []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(newKey) != 32 {
		hash := sha256.Sum256(newKey)
		newKey = hash[:]
	}

	e.key = newKey
	return nil
}

// loadKeyFromFile loads an encryption key from a file.
func loadKeyFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try to decode as base64 first
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err == nil && len(decoded) >= 16 {
		return decoded, nil
	}

	// Use raw bytes
	return data, nil
}

// deriveKey derives an encryption key from a passphrase using PBKDF2.
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
}

// GenerateKey generates a new random 32-byte encryption key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateKeyBase64 generates a new random encryption key and returns it as base64.
func GenerateKeyBase64() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
