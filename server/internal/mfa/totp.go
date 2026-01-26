// Package mfa provides multi-factor authentication functionality.
package mfa

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"io"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

var (
	// ErrInvalidCode is returned when the TOTP code is invalid.
	ErrInvalidCode = errors.New("invalid TOTP code")
	// ErrSecretTooShort is returned when the secret is too short.
	ErrSecretTooShort = errors.New("secret is too short")
)

// TOTPConfig holds TOTP configuration.
type TOTPConfig struct {
	// Issuer is the name shown in authenticator apps.
	Issuer string
	// Algorithm is the hash algorithm (SHA1, SHA256, SHA512).
	Algorithm otp.Algorithm
	// Digits is the number of digits in the code.
	Digits otp.Digits
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
		Issuer:     "ZimaOS-Echo",
		Algorithm:  otp.AlgorithmSHA1, // Standard for most authenticator apps
		Digits:     otp.DigitsSix,
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
	secret := make([]byte, t.config.SecretSize)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateKey generates a new TOTP key for a user.
func (t *TOTP) GenerateKey(accountName string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      t.config.Issuer,
		AccountName: accountName,
		Period:      t.config.Period,
		SecretSize:  t.config.SecretSize,
		Digits:      t.config.Digits,
		Algorithm:   t.config.Algorithm,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// Validate validates a TOTP code against a secret.
func (t *TOTP) Validate(code, secret string) bool {
	// Normalize the secret
	secret = strings.ToUpper(strings.TrimSpace(secret))

	return totp.Validate(code, secret)
}

// ValidateWithSkew validates a TOTP code with clock skew tolerance.
func (t *TOTP) ValidateWithSkew(code, secret string) bool {
	secret = strings.ToUpper(strings.TrimSpace(secret))

	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    t.config.Period,
		Skew:      t.config.Skew,
		Digits:    t.config.Digits,
		Algorithm: t.config.Algorithm,
	})

	return err == nil && valid
}

// GenerateCode generates a TOTP code for the current time.
func (t *TOTP) GenerateCode(secret string) (string, error) {
	secret = strings.ToUpper(strings.TrimSpace(secret))

	code, err := totp.GenerateCodeCustom(secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    t.config.Period,
		Digits:    t.config.Digits,
		Algorithm: t.config.Algorithm,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate code: %w", err)
	}
	return code, nil
}

// GetProvisioningURI returns the provisioning URI for authenticator apps.
func (t *TOTP) GetProvisioningURI(accountName, secret string) string {
	secret = strings.ToUpper(strings.TrimSpace(secret))

	key, _ := otp.NewKeyFromURL(fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=%s&digits=%d&period=%d",
		t.config.Issuer,
		accountName,
		secret,
		t.config.Issuer,
		t.config.Algorithm.String(),
		t.config.Digits,
		t.config.Period,
	))

	return key.URL()
}

// GenerateQRCode generates a QR code PNG for the provisioning URI.
func (t *TOTP) GenerateQRCode(accountName, secret string, size int) ([]byte, error) {
	uri := t.GetProvisioningURI(accountName, secret)

	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	return qr.PNG(size)
}

// GenerateQRCodeWriter writes a QR code PNG to the provided writer.
func (t *TOTP) GenerateQRCodeWriter(w io.Writer, accountName, secret string, size int) error {
	uri := t.GetProvisioningURI(accountName, secret)

	qr, err := qrcode.New(uri, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("failed to create QR code: %w", err)
	}

	img := qr.Image(size)
	return png.Encode(w, img)
}

// SetupResponse contains the data needed for MFA setup.
type SetupResponse struct {
	// Secret is the TOTP secret (base32 encoded).
	Secret string `json:"secret"`
	// URI is the provisioning URI for authenticator apps.
	URI string `json:"uri"`
	// QRCode is the base64-encoded QR code PNG (optional).
	QRCode string `json:"qr_code,omitempty"`
}

// Setup generates all data needed for MFA setup.
func (t *TOTP) Setup(accountName string, includeQRCode bool) (*SetupResponse, error) {
	key, err := t.GenerateKey(accountName)
	if err != nil {
		return nil, err
	}

	resp := &SetupResponse{
		Secret: key.Secret(),
		URI:    key.URL(),
	}

	if includeQRCode {
		qrData, err := t.GenerateQRCode(accountName, key.Secret(), 256)
		if err != nil {
			return nil, err
		}
		resp.QRCode = "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrData)
	}

	return resp, nil
}

// GetConfig returns the current TOTP configuration.
func (t *TOTP) GetConfig() *TOTPConfig {
	return t.config
}
