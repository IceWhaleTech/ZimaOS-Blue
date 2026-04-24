// Package mfa provides multi-factor authentication functionality.
package mfa

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

var (
	// ErrInvalidCode is returned when the TOTP code is invalid.
	ErrInvalidCode = errors.New("invalid TOTP code")
	// ErrSecretTooShort is returned when the secret is too short.
	ErrSecretTooShort = errors.New("secret is too short")
)

// Algorithm represents the hashing function used for OTP generation.
type Algorithm int

const (
	// AlgorithmSHA1 should be used for compatibility with Google Authenticator.
	AlgorithmSHA1 Algorithm = iota
	AlgorithmSHA256
	AlgorithmSHA512
	AlgorithmMD5
)

func (a Algorithm) String() string {
	switch effectiveAlgorithm(a) {
	case AlgorithmSHA1:
		return "SHA1"
	case AlgorithmSHA256:
		return "SHA256"
	case AlgorithmSHA512:
		return "SHA512"
	case AlgorithmMD5:
		return "MD5"
	default:
		return "SHA1"
	}
}

func (a Algorithm) hashFunc() func() hash.Hash {
	switch effectiveAlgorithm(a) {
	case AlgorithmSHA1:
		return sha1.New
	case AlgorithmSHA256:
		return sha256.New
	case AlgorithmSHA512:
		return sha512.New
	case AlgorithmMD5:
		return md5.New
	default:
		return sha1.New
	}
}

// Digits represents the number of digits in a TOTP code.
type Digits int

const (
	DigitsSix   Digits = 6
	DigitsEight Digits = 8
)

// Format converts an integer into the zero-filled size for this Digits.
func (d Digits) Format(in int32) string {
	return fmt.Sprintf("%0*d", d.Length(), in)
}

// Length returns the number of characters for this Digits.
func (d Digits) Length() int {
	if d == 0 {
		return int(DigitsSix)
	}
	return int(d)
}

func (d Digits) String() string {
	return fmt.Sprintf("%d", d.Length())
}

// Key contains the provisioning data for a TOTP account.
type Key struct {
	url         string
	secret      string
	issuer      string
	accountName string
}

func newKey(issuer, accountName, secret string, period uint, digits Digits, algorithm Algorithm) *Key {
	return &Key{
		url:         buildProvisioningURI(issuer, accountName, secret, period, digits, algorithm),
		secret:      sanitizeProvisioningSecret(secret),
		issuer:      strings.TrimSpace(issuer),
		accountName: strings.TrimSpace(accountName),
	}
}

func (k *Key) String() string {
	return k.url
}

func (k *Key) URL() string {
	return k.url
}

func (k *Key) Secret() string {
	return k.secret
}

func (k *Key) Issuer() string {
	return k.issuer
}

func (k *Key) AccountName() string {
	return k.accountName
}

// TOTPConfig holds TOTP configuration.
type TOTPConfig struct {
	// Issuer is the name shown in authenticator apps.
	Issuer string
	// Algorithm is the hash algorithm (SHA1, SHA256, SHA512).
	Algorithm Algorithm
	// Digits is the number of digits in the code.
	Digits Digits
	// Period is the time step in seconds.
	Period uint
	// SecretSize is the size of the secret in bytes.
	SecretSize uint
	// Skew is the number of periods to allow for clock drift.
	Skew uint
}

// DefaultTOTPConfig returns the default TOTP configuration.
func DefaultTOTPConfig() *TOTPConfig {
	return &TOTPConfig{
		Issuer:     "ZimaOS-Blue",
		Algorithm:  AlgorithmSHA1, // Standard for most authenticator apps
		Digits:     DigitsSix,
		Period:     30,
		SecretSize: 20, // 160 bits
		Skew:       1,  // Allow 1 period before/after
	}
}

// TOTP provides TOTP generation and validation.
type TOTP struct {
	config *TOTPConfig
}

// NewTOTP creates a new TOTP instance.
func NewTOTP(config *TOTPConfig) *TOTP {
	if config == nil {
		config = DefaultTOTPConfig()
	}
	return &TOTP{config: config}
}

// GenerateSecret generates a new TOTP secret.
func (t *TOTP) GenerateSecret() (string, error) {
	secret := make([]byte, effectiveSecretSize(t.config.SecretSize))
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateKey generates a new TOTP key for a user.
func (t *TOTP) GenerateKey(accountName string) (*Key, error) {
	issuer := strings.TrimSpace(t.config.Issuer)
	accountName = strings.TrimSpace(accountName)
	if issuer == "" {
		return nil, errors.New("issuer is required")
	}
	if accountName == "" {
		return nil, errors.New("account name is required")
	}

	secret, err := t.GenerateSecret()
	if err != nil {
		return nil, err
	}

	return newKey(
		issuer,
		accountName,
		secret,
		effectivePeriod(t.config.Period),
		effectiveDigits(t.config.Digits),
		effectiveAlgorithm(t.config.Algorithm),
	), nil
}

// Validate validates a TOTP code against a secret.
func (t *TOTP) Validate(code, secret string) bool {
	valid, err := validateCodeAt(code, secret, timeutil.NowTime().UTC(), 30, 1, DigitsSix, AlgorithmSHA1)
	return err == nil && valid
}

// ValidateWithSkew validates a TOTP code with clock skew tolerance.
func (t *TOTP) ValidateWithSkew(code, secret string) bool {
	valid, err := validateCodeAt(
		code,
		secret,
		timeutil.NowTime().UTC(),
		effectivePeriod(t.config.Period),
		t.config.Skew,
		effectiveDigits(t.config.Digits),
		effectiveAlgorithm(t.config.Algorithm),
	)
	return err == nil && valid
}

// GenerateCode generates a TOTP code for the current time.
func (t *TOTP) GenerateCode(secret string) (string, error) {
	return generateCodeAt(
		secret,
		timeutil.NowTime().UTC(),
		effectivePeriod(t.config.Period),
		effectiveDigits(t.config.Digits),
		effectiveAlgorithm(t.config.Algorithm),
	)
}

// GetProvisioningURI returns the provisioning URI for authenticator apps.
func (t *TOTP) GetProvisioningURI(accountName, secret string) string {
	return buildProvisioningURI(
		t.config.Issuer,
		accountName,
		secret,
		effectivePeriod(t.config.Period),
		effectiveDigits(t.config.Digits),
		effectiveAlgorithm(t.config.Algorithm),
	)
}

// SetupResponse contains the data needed for MFA setup.
type SetupResponse struct {
	// Secret is the TOTP secret (base32 encoded).
	Secret string `json:"secret"`
	// URI is the provisioning URI for authenticator apps.
	URI string `json:"uri"`
}

// Setup generates the provisioning secret and URI for MFA setup.
func (t *TOTP) Setup(accountName string) (*SetupResponse, error) {
	key, err := t.GenerateKey(accountName)
	if err != nil {
		return nil, err
	}

	resp := &SetupResponse{
		Secret: key.Secret(),
		URI:    key.URL(),
	}

	return resp, nil
}

// GetConfig returns the current TOTP configuration.
func (t *TOTP) GetConfig() *TOTPConfig {
	return t.config
}

func effectiveAlgorithm(algorithm Algorithm) Algorithm {
	switch algorithm {
	case AlgorithmSHA1, AlgorithmSHA256, AlgorithmSHA512, AlgorithmMD5:
		return algorithm
	default:
		return AlgorithmSHA1
	}
}

func effectiveDigits(digits Digits) Digits {
	if digits == 0 {
		return DigitsSix
	}
	return digits
}

func effectivePeriod(period uint) uint {
	if period == 0 {
		return 30
	}
	return period
}

func effectiveSecretSize(secretSize uint) uint {
	if secretSize == 0 {
		return 20
	}
	return secretSize
}

func sanitizeProvisioningSecret(secret string) string {
	return strings.TrimRight(strings.ToUpper(strings.TrimSpace(secret)), "=")
}

func buildProvisioningURI(issuer, accountName, secret string, period uint, digits Digits, algorithm Algorithm) string {
	values := url.Values{}
	values.Set("secret", sanitizeProvisioningSecret(secret))
	values.Set("issuer", strings.TrimSpace(issuer))
	values.Set("algorithm", effectiveAlgorithm(algorithm).String())
	values.Set("digits", effectiveDigits(digits).String())
	values.Set("period", fmt.Sprintf("%d", effectivePeriod(period)))

	return (&url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + strings.TrimSpace(issuer) + ":" + strings.TrimSpace(accountName),
		RawQuery: values.Encode(),
	}).String()
}

func generateCodeAt(secret string, now time.Time, period uint, digits Digits, algorithm Algorithm) (string, error) {
	return generateCodeForCounter(secret, counterAt(now, period), digits, algorithm)
}

func validateCodeAt(code, secret string, now time.Time, period uint, skew uint, digits Digits, algorithm Algorithm) (bool, error) {
	code = strings.TrimSpace(code)
	digits = effectiveDigits(digits)
	if len(code) != digits.Length() {
		return false, nil
	}

	baseCounter := counterAt(now, period)
	for delta := int64(0); delta <= int64(skew); delta++ {
		counters := []uint64{baseCounter}
		if delta > 0 {
			counters = []uint64{baseCounter + uint64(delta)}
			if baseCounter >= uint64(delta) {
				counters = append(counters, baseCounter-uint64(delta))
			}
		}

		for _, counter := range counters {
			expected, err := generateCodeForCounter(secret, counter, digits, algorithm)
			if err != nil {
				return false, err
			}
			if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
				return true, nil
			}
		}
	}

	return false, nil
}

func counterAt(now time.Time, period uint) uint64 {
	period = effectivePeriod(period)
	unix := now.UTC().Unix()
	if unix <= 0 {
		return 0
	}
	return uint64(unix) / uint64(period)
}

func generateCodeForCounter(secret string, counter uint64, digits Digits, algorithm Algorithm) (string, error) {
	decodedSecret, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(effectiveAlgorithm(algorithm).hashFunc(), decodedSecret)
	if _, err := mac.Write(buf); err != nil {
		return "", fmt.Errorf("failed to write counter to HMAC: %w", err)
	}
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := int64(((int(sum[offset]) & 0x7f) << 24) |
		((int(sum[offset+1] & 0xff)) << 16) |
		((int(sum[offset+2] & 0xff)) << 8) |
		(int(sum[offset+3]) & 0xff))

	mod := int32(value % int64(math.Pow10(effectiveDigits(digits).Length())))
	return effectiveDigits(digits).Format(mod), nil
}

func decodeSecret(secret string) ([]byte, error) {
	normalizedSecret := strings.ToUpper(strings.TrimSpace(secret))
	if normalizedSecret == "" {
		return nil, ErrSecretTooShort
	}
	if n := len(normalizedSecret) % 8; n != 0 {
		normalizedSecret += strings.Repeat("=", 8-n)
	}

	decodedSecret, err := base32.StdEncoding.DecodeString(normalizedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}
	return decodedSecret, nil
}
