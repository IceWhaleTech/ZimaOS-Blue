package mfa

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestDefaultTOTPConfig(t *testing.T) {
	config := DefaultTOTPConfig()

	if config.Issuer != "ZimaOS-Blue" {
		t.Errorf("Issuer = %v, want ZimaOS-Blue", config.Issuer)
	}
	if config.Algorithm != otp.AlgorithmSHA1 {
		t.Errorf("Algorithm = %v, want SHA1", config.Algorithm)
	}
	if config.Digits != otp.DigitsSix {
		t.Errorf("Digits = %v, want 6", config.Digits)
	}
	if config.Period != 30 {
		t.Errorf("Period = %v, want 30", config.Period)
	}
	if config.SecretSize != 20 {
		t.Errorf("SecretSize = %v, want 20", config.SecretSize)
	}
}

func TestNewTOTP(t *testing.T) {
	// Test with nil config
	totp := NewTOTP(nil)
	if totp.config == nil {
		t.Error("NewTOTP(nil) should use default config")
	}

	// Test with custom config
	customConfig := &TOTPConfig{
		Issuer:     "CustomIssuer",
		Algorithm:  otp.AlgorithmSHA256,
		Digits:     otp.DigitsEight,
		Period:     60,
		SecretSize: 32,
		Skew:       2,
	}
	totp = NewTOTP(customConfig)
	if totp.config.Issuer != "CustomIssuer" {
		t.Errorf("Issuer = %v, want CustomIssuer", totp.config.Issuer)
	}
}

func TestTOTP_GenerateSecret(t *testing.T) {
	totpInstance := NewTOTP(nil)

	secret, err := totpInstance.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	if secret == "" {
		t.Error("GenerateSecret() returned empty secret")
	}

	// Secret should be base32 encoded
	if !isValidBase32(secret) {
		t.Error("GenerateSecret() returned invalid base32 string")
	}

	// Generate another secret - should be different
	secret2, _ := totpInstance.GenerateSecret()
	if secret == secret2 {
		t.Error("GenerateSecret() should return unique secrets")
	}
}

func TestTOTP_GenerateKey(t *testing.T) {
	totpInstance := NewTOTP(nil)

	key, err := totpInstance.GenerateKey("testuser@example.com")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	if key == nil {
		t.Fatal("GenerateKey() returned nil key")
	}

	if key.Secret() == "" {
		t.Error("GenerateKey() returned key with empty secret")
	}

	if key.Issuer() != "ZimaOS-Blue" {
		t.Errorf("Issuer = %v, want ZimaOS-Blue", key.Issuer())
	}

	if key.AccountName() != "testuser@example.com" {
		t.Errorf("AccountName = %v, want testuser@example.com", key.AccountName())
	}
}

func TestTOTP_Validate(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Generate a key
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	// Generate a valid code
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("Failed to generate code: %v", err)
	}

	// Validate the code
	if !totpInstance.Validate(code, secret) {
		t.Error("Validate() should return true for valid code")
	}

	// Invalid code should fail
	if totpInstance.Validate("000000", secret) {
		t.Error("Validate() should return false for invalid code")
	}
}

func TestTOTP_ValidateWithSkew(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Generate a key
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	// Generate a valid code
	code, _ := totp.GenerateCode(secret, time.Now())

	// Validate with skew
	if !totpInstance.ValidateWithSkew(code, secret) {
		t.Error("ValidateWithSkew() should return true for valid code")
	}

	// Invalid code should fail
	if totpInstance.ValidateWithSkew("000000", secret) {
		t.Error("ValidateWithSkew() should return false for invalid code")
	}
}

func TestTOTP_GenerateCode(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Generate a key
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	// Generate a code
	code, err := totpInstance.GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if len(code) != 6 {
		t.Errorf("GenerateCode() returned code with length %d, want 6", len(code))
	}

	// Code should be valid
	if !totpInstance.Validate(code, secret) {
		t.Error("Generated code should be valid")
	}
}

func TestTOTP_GetProvisioningURI(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Generate a key
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	uri := totpInstance.GetProvisioningURI("testuser@example.com", secret)

	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("GetProvisioningURI() should start with otpauth://totp/, got %s", uri)
	}

	if !strings.Contains(uri, "ZimaOS-Blue") {
		t.Error("GetProvisioningURI() should contain issuer")
	}

	if !strings.Contains(uri, "testuser@example.com") {
		t.Error("GetProvisioningURI() should contain account name")
	}
}

func TestTOTP_GenerateQRCode(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Generate a key
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	// Generate QR code
	qrData, err := totpInstance.GenerateQRCode("testuser@example.com", secret, 256)
	if err != nil {
		t.Fatalf("GenerateQRCode() error = %v", err)
	}

	if len(qrData) == 0 {
		t.Error("GenerateQRCode() returned empty data")
	}

	// Check PNG header
	if len(qrData) < 8 || string(qrData[1:4]) != "PNG" {
		t.Error("GenerateQRCode() should return PNG data")
	}
}

func TestTOTP_Setup(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Setup without QR code
	resp, err := totpInstance.Setup("testuser@example.com", false)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if resp.Secret == "" {
		t.Error("Setup() returned empty secret")
	}
	if resp.URI == "" {
		t.Error("Setup() returned empty URI")
	}
	if resp.QRCode != "" {
		t.Error("Setup() should not include QR code when not requested")
	}

	// Setup with QR code
	resp, err = totpInstance.Setup("testuser@example.com", true)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if resp.QRCode == "" {
		t.Error("Setup() should include QR code when requested")
	}
	if !strings.HasPrefix(resp.QRCode, "data:image/png;base64,") {
		t.Error("Setup() QR code should be base64 data URI")
	}
}

func TestTOTP_GetConfig(t *testing.T) {
	config := &TOTPConfig{
		Issuer: "TestIssuer",
	}
	totpInstance := NewTOTP(config)

	if totpInstance.GetConfig().Issuer != "TestIssuer" {
		t.Error("GetConfig() should return the config")
	}
}

func TestTOTP_Validate_NormalizeSecret(t *testing.T) {
	totpInstance := NewTOTP(nil)

	// Generate a key
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	// Generate a valid code
	code, _ := totp.GenerateCode(secret, time.Now())

	// Test with lowercase secret
	if !totpInstance.Validate(code, strings.ToLower(secret)) {
		t.Error("Validate() should normalize lowercase secret")
	}

	// Test with spaces
	if !totpInstance.Validate(code, " "+secret+" ") {
		t.Error("Validate() should trim spaces from secret")
	}
}

// Helper function to check if a string is valid base32
func isValidBase32(s string) bool {
	// Base32 characters
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ807"
	for _, c := range strings.ToUpper(s) {
		if !strings.ContainsRune(validChars, c) && c != '=' {
			return false
		}
	}
	return true
}

func BenchmarkTOTP_Validate(b *testing.B) {
	totpInstance := NewTOTP(nil)
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()
	code, _ := totp.GenerateCode(secret, time.Now())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		totpInstance.Validate(code, secret)
	}
}

func BenchmarkTOTP_GenerateCode(b *testing.B) {
	totpInstance := NewTOTP(nil)
	key, _ := totpInstance.GenerateKey("testuser@example.com")
	secret := key.Secret()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = totpInstance.GenerateCode(secret)
	}
}
