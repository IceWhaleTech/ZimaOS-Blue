// Package webpush provides Web Push notification support using VAPID authentication.
package webpush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	wp "github.com/SherClockHolmes/webpush-go"
)

type vapidKeys struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// GetOrCreateVAPIDKeys returns VAPID keys, generating and persisting them on first call.
// Keys are stored in {dataDir}/webpush_vapid.json.
func GetOrCreateVAPIDKeys(dataDir string) (privateKey, publicKey string, err error) {
	path := filepath.Join(dataDir, "webpush_vapid.json")

	// Try to load existing keys
	if data, err := os.ReadFile(path); err == nil {
		var keys vapidKeys
		if err := json.Unmarshal(data, &keys); err == nil && keys.PrivateKey != "" && keys.PublicKey != "" {
			return keys.PrivateKey, keys.PublicKey, nil
		}
	}

	// Generate new VAPID key pair
	priv, pub, err := wp.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("generate VAPID keys: %w", err)
	}

	keys := vapidKeys{PrivateKey: priv, PublicKey: pub}
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal VAPID keys: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", "", fmt.Errorf("write VAPID keys: %w", err)
	}

	return priv, pub, nil
}
