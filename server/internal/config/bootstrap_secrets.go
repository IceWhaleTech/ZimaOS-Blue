package config

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const defaultJWTSecretPlaceholder = "change-me-in-production-use-a-strong-secret-key"

func ensureFirstRunSecrets(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if usesDefaultJWTSecret(cfg.Security.JWT.Secret) {
		secret, err := generateSecretHex(32)
		if err != nil {
			return err
		}
		cfg.Security.JWT.Secret = secret
	}
	return nil
}

func usesDefaultJWTSecret(secret string) bool {
	s := strings.TrimSpace(secret)
	return s == "" || s == defaultJWTSecretPlaceholder
}

func generateSecretHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
