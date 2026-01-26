package mfa

import (
	"strings"
	"testing"
)

func TestDefaultRecoveryConfig(t *testing.T) {
	config := DefaultRecoveryConfig()

	if config.Count != 8 {
		t.Errorf("Count = %v, want 8", config.Count)
	}
	if config.Length != 8 {
		t.Errorf("Length = %v, want 8", config.Length)
	}
}

func TestNewRecovery(t *testing.T) {
	// Test with nil config
	recovery := NewRecovery(nil)
	if recovery.config == nil {
		t.Error("NewRecovery(nil) should use default config")
	}

	// Test with custom config
	customConfig := &RecoveryConfig{
		Count:  10,
		Length: 12,
	}
	recovery = NewRecovery(customConfig)
	if recovery.config.Count != 10 {
		t.Errorf("Count = %v, want 10", recovery.config.Count)
	}
}

func TestRecovery_Generate(t *testing.T) {
	recovery := NewRecovery(nil)

	codes, err := recovery.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(codes.Codes) != 8 {
		t.Errorf("Generate() returned %d codes, want 8", len(codes.Codes))
	}

	if len(codes.HashedCodes) != 8 {
		t.Errorf("Generate() returned %d hashed codes, want 8", len(codes.HashedCodes))
	}

	if len(codes.UsedCodes) != 8 {
		t.Errorf("Generate() returned %d used codes, want 8", len(codes.UsedCodes))
	}

	// Check code length
	for i, code := range codes.Codes {
		if len(code) != 8 {
			t.Errorf("Code %d length = %d, want 8", i, len(code))
		}
	}

	// Check all codes are unique
	seen := make(map[string]bool)
	for _, code := range codes.Codes {
		if seen[code] {
			t.Error("Generate() returned duplicate codes")
		}
		seen[code] = true
	}

	// Check all used codes are false
	for i, used := range codes.UsedCodes {
		if used {
			t.Errorf("UsedCodes[%d] = true, want false", i)
		}
	}
}

func TestRecovery_Validate(t *testing.T) {
	recovery := NewRecovery(nil)

	codes, _ := recovery.Generate()

	// Validate first code
	index, err := recovery.Validate(codes.Codes[0], codes.HashedCodes, codes.UsedCodes)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if index != 0 {
		t.Errorf("Validate() index = %d, want 0", index)
	}

	// Validate last code
	index, err = recovery.Validate(codes.Codes[7], codes.HashedCodes, codes.UsedCodes)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if index != 7 {
		t.Errorf("Validate() index = %d, want 7", index)
	}

	// Invalid code should fail
	_, err = recovery.Validate("INVALID1", codes.HashedCodes, codes.UsedCodes)
	if err != ErrInvalidRecoveryCode {
		t.Errorf("Validate() error = %v, want %v", err, ErrInvalidRecoveryCode)
	}
}

func TestRecovery_Validate_NormalizeCode(t *testing.T) {
	recovery := NewRecovery(nil)

	codes, _ := recovery.Generate()
	code := codes.Codes[0]

	// Test with lowercase
	index, err := recovery.Validate(strings.ToLower(code), codes.HashedCodes, codes.UsedCodes)
	if err != nil {
		t.Errorf("Validate() with lowercase error = %v", err)
	}
	if index != 0 {
		t.Errorf("Validate() with lowercase index = %d, want 0", index)
	}

	// Test with dashes
	codeWithDash := code[:4] + "-" + code[4:]
	index, err = recovery.Validate(codeWithDash, codes.HashedCodes, codes.UsedCodes)
	if err != nil {
		t.Errorf("Validate() with dashes error = %v", err)
	}
	if index != 0 {
		t.Errorf("Validate() with dashes index = %d, want 0", index)
	}

	// Test with spaces
	codeWithSpaces := code[:4] + " " + code[4:]
	index, err = recovery.Validate(codeWithSpaces, codes.HashedCodes, codes.UsedCodes)
	if err != nil {
		t.Errorf("Validate() with spaces error = %v", err)
	}
	if index != 0 {
		t.Errorf("Validate() with spaces index = %d, want 0", index)
	}
}

func TestRecovery_ValidateAndMark(t *testing.T) {
	recovery := NewRecovery(nil)

	codes, _ := recovery.Generate()

	// Use first code
	usedCodes, err := recovery.ValidateAndMark(codes.Codes[0], codes.HashedCodes, codes.UsedCodes)
	if err != nil {
		t.Fatalf("ValidateAndMark() error = %v", err)
	}
	if !usedCodes[0] {
		t.Error("ValidateAndMark() should mark code as used")
	}

	// Try to use same code again
	_, err = recovery.ValidateAndMark(codes.Codes[0], codes.HashedCodes, usedCodes)
	if err != ErrInvalidRecoveryCode {
		t.Errorf("ValidateAndMark() error = %v, want %v", err, ErrInvalidRecoveryCode)
	}

	// Use another code
	usedCodes, err = recovery.ValidateAndMark(codes.Codes[1], codes.HashedCodes, usedCodes)
	if err != nil {
		t.Fatalf("ValidateAndMark() error = %v", err)
	}
	if !usedCodes[1] {
		t.Error("ValidateAndMark() should mark second code as used")
	}
}

func TestRecovery_ValidateAndMark_AllUsed(t *testing.T) {
	recovery := NewRecovery(&RecoveryConfig{Count: 2, Length: 8})

	codes, _ := recovery.Generate()

	// Use all codes
	usedCodes := codes.UsedCodes
	usedCodes, _ = recovery.ValidateAndMark(codes.Codes[0], codes.HashedCodes, usedCodes)
	usedCodes, _ = recovery.ValidateAndMark(codes.Codes[1], codes.HashedCodes, usedCodes)

	// Try to use another code
	_, err := recovery.ValidateAndMark("ANYCODE1", codes.HashedCodes, usedCodes)
	if err != ErrNoRecoveryCodesLeft {
		t.Errorf("ValidateAndMark() error = %v, want %v", err, ErrNoRecoveryCodesLeft)
	}
}

func TestRecovery_RemainingCodes(t *testing.T) {
	recovery := NewRecovery(nil)

	codes, _ := recovery.Generate()

	// All codes should be remaining
	remaining := recovery.RemainingCodes(codes.UsedCodes)
	if remaining != 8 {
		t.Errorf("RemainingCodes() = %d, want 8", remaining)
	}

	// Use some codes
	codes.UsedCodes[0] = true
	codes.UsedCodes[1] = true
	codes.UsedCodes[2] = true

	remaining = recovery.RemainingCodes(codes.UsedCodes)
	if remaining != 5 {
		t.Errorf("RemainingCodes() = %d, want 5", remaining)
	}

	// Use all codes
	for i := range codes.UsedCodes {
		codes.UsedCodes[i] = true
	}

	remaining = recovery.RemainingCodes(codes.UsedCodes)
	if remaining != 0 {
		t.Errorf("RemainingCodes() = %d, want 0", remaining)
	}
}

func TestRecovery_FormatCodesForDisplay(t *testing.T) {
	recovery := NewRecovery(nil)

	codes := []string{"ABCD1234", "EFGH5678"}
	formatted := recovery.FormatCodesForDisplay(codes)

	if formatted[0] != "ABCD-1234" {
		t.Errorf("FormatCodesForDisplay()[0] = %v, want ABCD-1234", formatted[0])
	}
	if formatted[1] != "EFGH-5678" {
		t.Errorf("FormatCodesForDisplay()[1] = %v, want EFGH-5678", formatted[1])
	}
}

func TestRecovery_GetConfig(t *testing.T) {
	config := &RecoveryConfig{Count: 10}
	recovery := NewRecovery(config)

	if recovery.GetConfig().Count != 10 {
		t.Error("GetConfig() should return the config")
	}
}

func TestGenerateRandomCode(t *testing.T) {
	code1, err := generateRandomCode(8)
	if err != nil {
		t.Fatalf("generateRandomCode() error = %v", err)
	}

	if len(code1) != 8 {
		t.Errorf("generateRandomCode() length = %d, want 8", len(code1))
	}

	// Check characters are alphanumeric
	for _, c := range code1 {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			t.Errorf("generateRandomCode() contains invalid character: %c", c)
		}
	}

	// Generate another code - should be different
	code2, _ := generateRandomCode(8)
	if code1 == code2 {
		t.Error("generateRandomCode() should return unique codes")
	}
}

func TestHashRecoveryCode(t *testing.T) {
	hash1 := hashRecoveryCode("ABCD1234")
	hash2 := hashRecoveryCode("ABCD1234")

	// Same code should produce same hash
	if hash1 != hash2 {
		t.Error("hashRecoveryCode() should be deterministic")
	}

	// Different codes should produce different hashes
	hash3 := hashRecoveryCode("EFGH5678")
	if hash1 == hash3 {
		t.Error("hashRecoveryCode() should produce different hashes for different codes")
	}
}

func TestVerifyRecoveryCode(t *testing.T) {
	code := "ABCD1234"
	hash := hashRecoveryCode(code)

	if !verifyRecoveryCode(code, hash) {
		t.Error("verifyRecoveryCode() should return true for matching code")
	}

	if verifyRecoveryCode("WRONG123", hash) {
		t.Error("verifyRecoveryCode() should return false for non-matching code")
	}
}

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcd1234", "ABCD1234"},
		{"ABCD-1234", "ABCD1234"},
		{"abcd 1234", "ABCD1234"},
		{"abcd-1234", "ABCD1234"},
		{"  ABCD1234  ", "ABCD1234"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeCode(tt.input)
			if got != tt.want {
				t.Errorf("normalizeCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func BenchmarkRecovery_Generate(b *testing.B) {
	recovery := NewRecovery(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = recovery.Generate()
	}
}

func BenchmarkRecovery_Validate(b *testing.B) {
	recovery := NewRecovery(nil)
	codes, _ := recovery.Generate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = recovery.Validate(codes.Codes[0], codes.HashedCodes, codes.UsedCodes)
	}
}
