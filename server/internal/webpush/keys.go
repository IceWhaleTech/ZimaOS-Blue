// Package webpush provides Web Push notification support using VAPID authentication.
package webpush

import (
	"context"
	"fmt"

	wp "github.com/SherClockHolmes/webpush-go"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

const vapidKVKey = "config:webpush_vapid"

type vapidKeys struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// GetOrCreateVAPIDKeys returns VAPID keys, generating and persisting them on first call.
// Keys are stored in the kvstore under "config:webpush_vapid".
func GetOrCreateVAPIDKeys(kv kvstore.Store) (privateKey, publicKey string, err error) {
	ctx := context.Background()

	// Try to load existing keys
	var keys vapidKeys
	if err := kv.GetJSON(ctx, vapidKVKey, &keys); err == nil && keys.PrivateKey != "" && keys.PublicKey != "" {
		return keys.PrivateKey, keys.PublicKey, nil
	}

	// Generate new VAPID key pair
	priv, pub, err := wp.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("generate VAPID keys: %w", err)
	}

	keys = vapidKeys{PrivateKey: priv, PublicKey: pub}
	if err := kv.SetJSON(ctx, vapidKVKey, &keys, 0); err != nil {
		return "", "", fmt.Errorf("persist VAPID keys: %w", err)
	}

	return priv, pub, nil
}
