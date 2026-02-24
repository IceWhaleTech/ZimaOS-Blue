// Command sign_trial_license signs a trial license with an Ed25519 private key.
//
// Usage:
//
//	go run tools/sign_trial_license.go \
//	  --key <base64-DER-private-key-or-PEM-file> \
//	  --kid 1 \
//	  --api-key sk-xxx \
//	  --url https://xxxxx/ \
//	  --limit 10000 \
//	  [--exp 2026-12-31]
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type claims struct {
	KID       string `json:"kid"`
	Key       string `json:"key"`
	URL       string `json:"url"`
	Limit     int64  `json:"lim"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Format    string `json:"fmt"`
	Model     string `json:"model"`
}

func main() {
	keyFlag := flag.String("key", "", "Base64 DER private key or path to PEM file")
	kid := flag.String("kid", "1", "Key ID (1-5)")
	apiKey := flag.String("api-key", "", "Trial API key")
	urlFlag := flag.String("url", "https://xxxxx/", "Trial base URL")
	limit := flag.Int64("limit", 10000, "Token limit")
	expFlag := flag.String("exp", "", "Expiry date (YYYY-MM-DD), empty=no expiry")
	fmtFlag := flag.String("fmt", "anthropic", "API format")
	model := flag.String("model", "claude-haiku-4-5", "Default model")
	flag.Parse()

	if *keyFlag == "" || *apiKey == "" {
		fmt.Fprintln(os.Stderr, "Usage: sign_trial_license --key <key> --api-key <key> [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load private key
	var der []byte
	if data, err := os.ReadFile(*keyFlag); err == nil {
		// It's a file — try PEM or raw base64
		der, _ = base64.StdEncoding.DecodeString(string(data))
		if der == nil {
			der = data
		}
	} else {
		// Try as inline base64
		var err2 error
		der, err2 = base64.StdEncoding.DecodeString(*keyFlag)
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "Cannot decode private key: %v\n", err2)
			os.Exit(1)
		}
	}

	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse private key: %v\n", err)
		os.Exit(1)
	}
	privKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		fmt.Fprintln(os.Stderr, "Not an Ed25519 private key")
		os.Exit(1)
	}

	c := &claims{
		KID:      *kid,
		Key:      *apiKey,
		URL:      *urlFlag,
		Limit:    *limit,
		IssuedAt: time.Now().Unix(),
		Format:   *fmtFlag,
		Model:    *model,
	}

	if *expFlag != "" {
		t, err := time.Parse("2006-01-02", *expFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid expiry date: %v\n", err)
			os.Exit(1)
		}
		c.ExpiresAt = t.Unix()
	}

	payloadJSON, _ := json.Marshal(c)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig := ed25519.Sign(privKey, []byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	license := payloadB64 + "." + sigB64
	fmt.Println(license)
}
