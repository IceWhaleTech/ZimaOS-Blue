package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/go-jose/go-jose/v3"
	"github.com/google/uuid"
)

var (
	// ErrKeyNotFound is returned when a key is not found.
	ErrKeyNotFound = errors.New("key not found")
	// ErrInvalidKey is returned when a key is invalid.
	ErrInvalidKey = errors.New("invalid key")
)

// KeyManager manages RSA signing keys for OIDC.
type KeyManager struct {
	mu           sync.RWMutex
	keys         map[string]*SigningKey
	currentKeyID string
	keyPath      string
	rotationDays int
}

// SigningKey represents an RSA signing key.
type SigningKey struct {
	ID         string
	PrivateKey *rsa.PrivateKey
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// NewKeyManager creates a new key manager.
func NewKeyManager(keyPath string, rotationDays int) (*KeyManager, error) {
	km := &KeyManager{
		keys:         make(map[string]*SigningKey),
		keyPath:      keyPath,
		rotationDays: rotationDays,
	}

	// Try to load existing key
	if err := km.loadOrGenerateKey(); err != nil {
		return nil, err
	}

	return km, nil
}

// loadOrGenerateKey loads an existing key or generates a new one.
func (km *KeyManager) loadOrGenerateKey() error {
	// Try to load existing key
	if _, err := os.Stat(km.keyPath); err == nil {
		if err := km.loadKey(); err != nil {
			return err
		}
		return nil
	}

	// Generate new key
	return km.generateKey()
}

// loadKey loads a key from disk.
func (km *KeyManager) loadKey() error {
	data, err := os.ReadFile(km.keyPath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return ErrInvalidKey
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return ErrInvalidKey
		}
	}

	keyID := uuid.New().String()[:8]
	now := timeutil.NowTime()

	km.mu.Lock()
	defer km.mu.Unlock()

	km.keys[keyID] = &SigningKey{
		ID:         keyID,
		PrivateKey: privateKey,
		CreatedAt:  now,
		ExpiresAt:  now.AddDate(0, 0, km.rotationDays),
	}
	km.currentKeyID = keyID

	return nil
}

// generateKey generates a new RSA key.
func (km *KeyManager) generateKey() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(km.keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Save key to disk
	keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}

	if err := os.WriteFile(km.keyPath, pem.EncodeToMemory(block), 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	keyID := uuid.New().String()[:8]
	now := timeutil.NowTime()

	km.mu.Lock()
	defer km.mu.Unlock()

	km.keys[keyID] = &SigningKey{
		ID:         keyID,
		PrivateKey: privateKey,
		CreatedAt:  now,
		ExpiresAt:  now.AddDate(0, 0, km.rotationDays),
	}
	km.currentKeyID = keyID

	return nil
}

// GetCurrentKey returns the current signing key.
func (km *KeyManager) GetCurrentKey() (*SigningKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, ok := km.keys[km.currentKeyID]
	if !ok {
		return nil, ErrKeyNotFound
	}

	return key, nil
}

// GetKey returns a key by ID.
func (km *KeyManager) GetKey(keyID string) (*SigningKey, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	key, ok := km.keys[keyID]
	if !ok {
		return nil, ErrKeyNotFound
	}

	return key, nil
}

// GetJWKS returns the JSON Web Key Set.
func (km *KeyManager) GetJWKS() *jose.JSONWebKeySet {
	km.mu.RLock()
	defer km.mu.RUnlock()

	var keys []jose.JSONWebKey
	for _, key := range km.keys {
		jwk := jose.JSONWebKey{
			Key:       &key.PrivateKey.PublicKey,
			KeyID:     key.ID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}
		keys = append(keys, jwk)
	}

	return &jose.JSONWebKeySet{Keys: keys}
}

// RotateKey rotates the signing key if needed.
func (km *KeyManager) RotateKey() error {
	km.mu.RLock()
	currentKey, ok := km.keys[km.currentKeyID]
	km.mu.RUnlock()

	if !ok || timeutil.NowNano() > currentKey.ExpiresAt.UnixNano() {
		return km.generateKey()
	}

	return nil
}

// Signer returns a jose.Signer for the current key.
func (km *KeyManager) Signer() (jose.Signer, error) {
	key, err := km.GetCurrentKey()
	if err != nil {
		return nil, err
	}

	signingKey := jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       key.PrivateKey,
	}

	opts := &jose.SignerOptions{}
	opts.WithHeader("kid", key.ID)

	return jose.NewSigner(signingKey, opts)
}
