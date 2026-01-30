package providerpool

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashAPIKey creates a display-safe hash of an API key
// Returns first 8 chars + "..." + last 4 chars
func HashAPIKey(key string) string {
	if len(key) <= 12 {
		return strings.Repeat("*", len(key))
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// HashAPIKeyFull creates a full SHA-256 hash of an API key for identification
func HashAPIKeyFull(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// GenerateID generates a random ID with the given prefix
func GenerateID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
