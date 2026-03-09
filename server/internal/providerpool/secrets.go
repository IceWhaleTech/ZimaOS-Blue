package providerpool

import (
	"fmt"
	"strings"
)

const encryptedSecretPrefix = "enc:v1:"

// SecretEncryptor encrypts/decrypts provider secrets at rest.
// *auth.Encryptor satisfies this interface.
type SecretEncryptor interface {
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
}

type storageOptions struct {
	encryptor SecretEncryptor
}

// StorageOption configures provider storage behavior.
type StorageOption func(*storageOptions)

// WithStorageEncryptor enables encryption for persisted provider secrets.
func WithStorageEncryptor(enc SecretEncryptor) StorageOption {
	return func(o *storageOptions) {
		o.encryptor = enc
	}
}

func applyStorageOptions(opts ...StorageOption) storageOptions {
	var cfg storageOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func normalizeProviderAPIKeys(provider *Provider) bool {
	if provider == nil {
		return false
	}
	changed := false
	for i := range provider.APIKeys {
		if normalizeAPIKeyMetadata(&provider.APIKeys[i]) {
			changed = true
		}
	}
	return changed
}

func normalizeAPIKeyMetadata(key *APIKey) bool {
	if key == nil || strings.TrimSpace(key.Key) == "" {
		return false
	}
	changed := false
	if key.KeyHash == "" {
		key.KeyHash = HashAPIKey(key.Key)
		changed = true
	}
	if key.KeyDigest == "" {
		key.KeyDigest = HashAPIKeyFull(key.Key)
		changed = true
	}
	return changed
}

func apiKeyFingerprint(key *APIKey) string {
	if key == nil {
		return ""
	}
	if key.KeyDigest != "" {
		return "digest:" + key.KeyDigest
	}
	if strings.TrimSpace(key.Key) != "" {
		return "digest:" + HashAPIKeyFull(key.Key)
	}
	if key.KeyHash != "" {
		return "mask:" + key.KeyHash
	}
	if key.ID != "" {
		return "id:" + key.ID
	}
	return ""
}

func encryptStoredSecret(enc SecretEncryptor, value string) (string, error) {
	if enc == nil || strings.TrimSpace(value) == "" {
		return value, nil
	}
	if strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	ciphertext, err := enc.EncryptString(value)
	if err != nil {
		return "", fmt.Errorf("%w: encrypt provider secret: %v", ErrEncryptionFailed, err)
	}
	return encryptedSecretPrefix + ciphertext, nil
}

func decryptStoredSecret(enc SecretEncryptor, value string) (string, error) {
	if !strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if enc == nil {
		return "", fmt.Errorf("%w: encrypted provider secret present but decryptor is unavailable", ErrEncryptionFailed)
	}
	plaintext, err := enc.DecryptString(strings.TrimPrefix(value, encryptedSecretPrefix))
	if err != nil {
		return "", fmt.Errorf("%w: decrypt provider secret: %v", ErrEncryptionFailed, err)
	}
	return plaintext, nil
}

func prepareStoredAPIKeys(provider *Provider, enc SecretEncryptor) ([]string, error) {
	if provider == nil || provider.Type == ProviderTypeTrial || len(provider.APIKeys) == 0 {
		return nil, nil
	}
	normalizeProviderAPIKeys(provider)
	keys := make([]string, len(provider.APIKeys))
	for i, apiKey := range provider.APIKeys {
		encrypted, err := encryptStoredSecret(enc, apiKey.Key)
		if err != nil {
			return nil, err
		}
		keys[i] = encrypted
	}
	return keys, nil
}

func restoreStoredAPIKeys(provider *Provider, stored []string, enc SecretEncryptor) error {
	if provider == nil || len(stored) == 0 {
		return nil
	}
	if len(provider.APIKeys) < len(stored) {
		provider.APIKeys = append(provider.APIKeys, make([]APIKey, len(stored)-len(provider.APIKeys))...)
	}
	for i, raw := range stored {
		plaintext, err := decryptStoredSecret(enc, raw)
		if err != nil {
			return err
		}
		provider.APIKeys[i].Key = plaintext
		normalizeAPIKeyMetadata(&provider.APIKeys[i])
	}
	return nil
}

func prepareStoredOAuthSecrets(provider *Provider, enc SecretEncryptor) (*oauthSecrets, error) {
	secrets := extractOAuthSecrets(provider)
	if secrets == nil {
		return nil, nil
	}
	copy := *secrets
	var err error
	if copy.AccessToken, err = encryptStoredSecret(enc, copy.AccessToken); err != nil {
		return nil, err
	}
	if copy.RefreshToken, err = encryptStoredSecret(enc, copy.RefreshToken); err != nil {
		return nil, err
	}
	if copy.ClientSecret, err = encryptStoredSecret(enc, copy.ClientSecret); err != nil {
		return nil, err
	}
	return &copy, nil
}

func restoreStoredOAuthSecrets(provider *Provider, secrets *oauthSecrets, enc SecretEncryptor) error {
	if provider == nil || secrets == nil {
		return nil
	}
	copy := *secrets
	var err error
	if copy.AccessToken, err = decryptStoredSecret(enc, copy.AccessToken); err != nil {
		return err
	}
	if copy.RefreshToken, err = decryptStoredSecret(enc, copy.RefreshToken); err != nil {
		return err
	}
	if copy.ClientSecret, err = decryptStoredSecret(enc, copy.ClientSecret); err != nil {
		return err
	}
	restoreOAuthSecrets(provider, &copy)
	return nil
}

func storedAPIKeysNeedEncryption(stored []string, enc SecretEncryptor) bool {
	if enc == nil {
		return false
	}
	for _, raw := range stored {
		if strings.TrimSpace(raw) != "" && !strings.HasPrefix(raw, encryptedSecretPrefix) {
			return true
		}
	}
	return false
}

func storedOAuthSecretsNeedEncryption(secrets *oauthSecrets, enc SecretEncryptor) bool {
	if enc == nil || secrets == nil {
		return false
	}
	for _, raw := range []string{secrets.AccessToken, secrets.RefreshToken, secrets.ClientSecret} {
		if strings.TrimSpace(raw) != "" && !strings.HasPrefix(raw, encryptedSecretPrefix) {
			return true
		}
	}
	return false
}
