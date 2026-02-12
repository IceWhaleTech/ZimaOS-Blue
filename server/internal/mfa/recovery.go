package mfa

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	// ErrInvalidRecoveryCode is returned when a recovery code is invalid.
	ErrInvalidRecoveryCode = errors.New("invalid recovery code")
	// ErrRecoveryCodeUsed is returned when a recovery code has already been used.
	ErrRecoveryCodeUsed = errors.New("recovery code already used")
	// ErrNoRecoveryCodesLeft is returned when all recovery codes have been used.
	ErrNoRecoveryCodesLeft = errors.New("no recovery codes left")
)

// RecoveryConfig holds recovery code configuration.
type RecoveryConfig struct {
	// Count is the number of recovery codes to generate.
	Count int
	// Length is the length of each recovery code (in characters).
	Length int
}

// DefaultRecoveryConfig returns the default recovery code configuration.
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		Count:  8,
		Length: 8,
	}
}

// Recovery provides recovery code generation and validation.
type Recovery struct {
	config *RecoveryConfig
}

// NewRecovery creates a new Recovery instance.
func NewRecovery(config *RecoveryConfig) *Recovery {
	if config == nil {
		config = DefaultRecoveryConfig()
	}
	return &Recovery{config: config}
}

// RecoveryCodes represents a set of recovery codes.
type RecoveryCodes struct {
	// Codes are the plaintext recovery codes (only available at generation time).
	Codes []string `json:"codes,omitempty"`
	// HashedCodes are the hashed recovery codes for storage.
	HashedCodes []string `json:"-"`
	// UsedCodes tracks which codes have been used (by index).
	UsedCodes []bool `json:"-"`
}

// Generate generates a new set of recovery codes.
func (r *Recovery) Generate() (*RecoveryCodes, error) {
	codes := make([]string, r.config.Count)
	hashedCodes := make([]string, r.config.Count)
	usedCodes := make([]bool, r.config.Count)

	for i := 0; i < r.config.Count; i++ {
		code, err := generateRandomCode(r.config.Length)
		if err != nil {
			return nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		codes[i] = code
		hashedCodes[i] = hashRecoveryCode(code)
	}

	return &RecoveryCodes{
		Codes:       codes,
		HashedCodes: hashedCodes,
		UsedCodes:   usedCodes,
	}, nil
}

// Validate validates a recovery code against the stored hashed codes.
// Returns the index of the matched code, or -1 if not found.
func (r *Recovery) Validate(code string, hashedCodes []string, usedCodes []bool) (int, error) {
	code = normalizeCode(code)

	for i, hashedCode := range hashedCodes {
		if usedCodes != nil && i < len(usedCodes) && usedCodes[i] {
			continue // Skip used codes
		}

		if verifyRecoveryCode(code, hashedCode) {
			return i, nil
		}
	}

	return -1, ErrInvalidRecoveryCode
}

// ValidateAndMark validates a recovery code and marks it as used.
// Returns the updated usedCodes slice.
func (r *Recovery) ValidateAndMark(code string, hashedCodes []string, usedCodes []bool) ([]bool, error) {
	if usedCodes == nil {
		usedCodes = make([]bool, len(hashedCodes))
	}

	// Check if all codes are used
	allUsed := true
	for _, used := range usedCodes {
		if !used {
			allUsed = false
			break
		}
	}
	if allUsed {
		return usedCodes, ErrNoRecoveryCodesLeft
	}

	index, err := r.Validate(code, hashedCodes, usedCodes)
	if err != nil {
		return usedCodes, err
	}

	// Mark as used
	if index >= 0 && index < len(usedCodes) {
		usedCodes[index] = true
	}

	return usedCodes, nil
}

// RemainingCodes returns the number of unused recovery codes.
func (r *Recovery) RemainingCodes(usedCodes []bool) int {
	count := 0
	for _, used := range usedCodes {
		if !used {
			count++
		}
	}
	return count
}

// FormatCodesForDisplay formats recovery codes for display to the user.
// Groups codes in pairs for easier reading.
func (r *Recovery) FormatCodesForDisplay(codes []string) []string {
	formatted := make([]string, len(codes))
	for i, code := range codes {
		// Insert a dash in the middle for readability
		if len(code) >= 4 {
			mid := len(code) / 2
			formatted[i] = code[:mid] + "-" + code[mid:]
		} else {
			formatted[i] = code
		}
	}
	return formatted
}

// GetConfig returns the current recovery configuration.
func (r *Recovery) GetConfig() *RecoveryConfig {
	return r.config
}

// Helper functions

func generateRandomCode(length int) (string, error) {
	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to alphanumeric characters (0-9, A-Z)
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	code := make([]byte, length)
	for i, b := range bytes {
		code[i] = charset[int(b)%len(charset)]
	}

	return string(code), nil
}

func hashRecoveryCode(code string) string {
	// Use Argon2id with fixed parameters for recovery codes
	// We use a fixed salt derived from the code itself for deterministic hashing
	// This is acceptable for recovery codes as they are high-entropy random strings
	salt := []byte("zimaos-blue-recovery-salt")
	hash := argon2.IDKey([]byte(normalizeCode(code)), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(hash)
}

func verifyRecoveryCode(code, hashedCode string) bool {
	expectedHash := hashRecoveryCode(code)
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(hashedCode)) == 1
}

func normalizeCode(code string) string {
	// Remove dashes and spaces, convert to uppercase
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return strings.ToUpper(code)
}
