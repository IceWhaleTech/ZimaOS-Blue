// Package security provides security utilities for ZimaOS Blue.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

// TimingSafeCompare performs a constant-time comparison of two strings.
// This prevents timing attacks where an attacker could determine the
// correct value by measuring response times.
func TimingSafeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// TimingSafeCompareBytes performs a constant-time comparison of two byte slices.
func TimingSafeCompareBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

// ValidateHMACSignature validates an HMAC-SHA256 signature using constant-time comparison.
// This is used for webhook signature validation to prevent timing attacks.
func ValidateHMACSignature(message, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expectedMAC := mac.Sum(nil)

	// Try to decode signature as base64 first
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		// Try hex decoding
		signatureBytes, err = hex.DecodeString(signature)
		if err != nil {
			return false
		}
	}

	return hmac.Equal(expectedMAC, signatureBytes)
}

// ValidateHMACSHA256Base64 validates an HMAC-SHA256 signature encoded as base64.
// Used for LINE webhook signature validation.
func ValidateHMACSHA256Base64(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison to prevent timing attacks
	expectedBytes := []byte(expectedMAC)
	signatureBytes := []byte(signature)

	if len(expectedBytes) != len(signatureBytes) {
		return false
	}

	return subtle.ConstantTimeCompare(expectedBytes, signatureBytes) == 1
}

// ValidateHMACSHA256Hex validates an HMAC-SHA256 signature encoded as hex.
// Used for various webhook signature validations.
func ValidateHMACSHA256Hex(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison to prevent timing attacks
	return TimingSafeCompare(expectedMAC, signature)
}
