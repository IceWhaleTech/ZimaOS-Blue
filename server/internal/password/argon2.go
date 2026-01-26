// Package password provides secure password hashing using Argon2id.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	// ErrInvalidHash is returned when the hash format is invalid.
	ErrInvalidHash = errors.New("invalid hash format")
	// ErrIncompatibleVersion is returned when the Argon2 version doesn't match.
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
	// ErrMismatchedPassword is returned when password verification fails.
	ErrMismatchedPassword = errors.New("password does not match")
)

// Config holds the Argon2id configuration parameters.
type Config struct {
	// Memory is the amount of memory used by the algorithm (in KB).
	Memory uint32
	// Iterations is the number of iterations (time cost).
	Iterations uint32
	// Parallelism is the number of threads used.
	Parallelism uint8
	// SaltLength is the length of the random salt.
	SaltLength uint32
	// KeyLength is the length of the generated key.
	KeyLength uint32
}

// DefaultConfig returns the recommended Argon2id configuration.
// These parameters follow OWASP recommendations for password hashing.
func DefaultConfig() *Config {
	return &Config{
		Memory:      64 * 1024, // 64 MB
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Hasher provides password hashing and verification using Argon2id.
type Hasher struct {
	config *Config
}

// NewHasher creates a new Hasher with the given configuration.
// If config is nil, DefaultConfig() is used.
func NewHasher(config *Config) *Hasher {
	if config == nil {
		config = DefaultConfig()
	}
	return &Hasher{config: config}
}

// Hash generates an Argon2id hash of the password.
// The returned string is in the format:
// $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.config.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		h.config.Iterations,
		h.config.Memory,
		h.config.Parallelism,
		h.config.KeyLength,
	)

	// Encode to standard format
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.config.Memory,
		h.config.Iterations,
		h.config.Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// Verify checks if the password matches the hash.
// Returns nil if the password matches, ErrMismatchedPassword otherwise.
func (h *Hasher) Verify(password, encodedHash string) error {
	config, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return err
	}

	otherHash := argon2.IDKey(
		[]byte(password),
		salt,
		config.Iterations,
		config.Memory,
		config.Parallelism,
		config.KeyLength,
	)

	if subtle.ConstantTimeCompare(hash, otherHash) != 1 {
		return ErrMismatchedPassword
	}

	return nil
}

// NeedsRehash checks if the hash was created with different parameters
// and should be rehashed with the current configuration.
func (h *Hasher) NeedsRehash(encodedHash string) bool {
	config, _, _, err := decodeHash(encodedHash)
	if err != nil {
		return true
	}

	return config.Memory != h.config.Memory ||
		config.Iterations != h.config.Iterations ||
		config.Parallelism != h.config.Parallelism ||
		config.KeyLength != h.config.KeyLength
}

// decodeHash parses an encoded Argon2id hash string.
func decodeHash(encodedHash string) (*Config, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	config := &Config{}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &config.Memory, &config.Iterations, &config.Parallelism)
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	config.SaltLength = uint32(len(salt))

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	config.KeyLength = uint32(len(hash))

	return config, salt, hash, nil
}
