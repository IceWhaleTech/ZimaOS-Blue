package password

import (
	"fmt"
	"testing"
)

func TestDefaultPolicyConfig(t *testing.T) {
	config := DefaultPolicyConfig()

	if config.MinLength != 8 {
		t.Errorf("expected MinLength to be 8, got %d", config.MinLength)
	}
	if !config.RequireUppercase {
		t.Error("expected RequireUppercase to be true")
	}
	if !config.RequireLowercase {
		t.Error("expected RequireLowercase to be true")
	}
	if !config.RequireNumber {
		t.Error("expected RequireNumber to be true")
	}
	if !config.RequireSpecial {
		t.Error("expected RequireSpecial to be true")
	}
	if config.HistoryCount != 5 {
		t.Errorf("expected HistoryCount to be 5, got %d", config.HistoryCount)
	}
}

func TestPolicy_Validate(t *testing.T) {
	policy := NewPolicy(nil)

	tests := []struct {
		name     string
		password string
		wantErrs int
	}{
		{
			name:     "valid password",
			password: "SecurePass123!",
			wantErrs: 0,
		},
		{
			name:     "too short",
			password: "Short1!",
			wantErrs: 1, // too short
		},
		{
			name:     "no uppercase",
			password: "securepass123!",
			wantErrs: 1,
		},
		{
			name:     "no lowercase",
			password: "SECUREPASS123!",
			wantErrs: 1,
		},
		{
			name:     "no number",
			password: "SecurePassword!",
			wantErrs: 1,
		},
		{
			name:     "no special",
			password: "SecurePass1234",
			wantErrs: 1,
		},
		{
			name:     "multiple violations",
			password: "short",
			wantErrs: 4, // too short, no uppercase, no number, no special
		},
		{
			name:     "common password base",
			password: "password",
			wantErrs: 4, // no uppercase + no number + no special + common
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := policy.Validate(tt.password)
			if len(errs) != tt.wantErrs {
				t.Errorf("Validate() got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
		})
	}
}

func TestPolicy_Validate_CustomConfig(t *testing.T) {
	// Minimal policy - only length requirement
	policy := NewPolicy(&PolicyConfig{
		MinLength:            8,
		RequireUppercase:     false,
		RequireLowercase:     false,
		RequireNumber:        false,
		RequireSpecial:       false,
		CheckCommonPasswords: false,
	})

	// Simple password should pass with minimal policy
	errs := policy.Validate("simplepassword")
	if len(errs) != 0 {
		t.Errorf("Validate() with minimal policy got errors: %v", errs)
	}

	// Too short should still fail
	errs = policy.Validate("short")
	if len(errs) != 1 {
		t.Errorf("Validate() expected 1 error for short password, got %d", len(errs))
	}
}

func TestPolicy_ValidateWithHistory(t *testing.T) {
	policy := NewPolicy(nil)
	hasher := NewHasher(nil)

	// Create password history
	oldPassword := "OldSecurePass1!"
	oldHash, _ := hasher.Hash(oldPassword)
	history := []string{oldHash}

	// New password should pass
	newPassword := "NewSecurePass2@"
	errs := policy.ValidateWithHistory(newPassword, history, hasher)
	if len(errs) != 0 {
		t.Errorf("ValidateWithHistory() new password got errors: %v", errs)
	}

	// Old password should fail
	errs = policy.ValidateWithHistory(oldPassword, history, hasher)
	found := false
	for _, err := range errs {
		if err == ErrPasswordInHistory {
			found = true
			break
		}
	}
	if !found {
		t.Error("ValidateWithHistory() should return ErrPasswordInHistory for old password")
	}
}

func TestPolicy_ValidateWithHistory_EmptyHistory(t *testing.T) {
	policy := NewPolicy(nil)
	hasher := NewHasher(nil)

	password := "SecurePass123!"
	errs := policy.ValidateWithHistory(password, nil, hasher)
	if len(errs) != 0 {
		t.Errorf("ValidateWithHistory() with empty history got errors: %v", errs)
	}

	errs = policy.ValidateWithHistory(password, []string{}, hasher)
	if len(errs) != 0 {
		t.Errorf("ValidateWithHistory() with empty slice got errors: %v", errs)
	}
}

func TestPolicy_IsValid(t *testing.T) {
	policy := NewPolicy(nil)

	if !policy.IsValid("SecurePass123!") {
		t.Error("IsValid() should return true for valid password")
	}

	if policy.IsValid("weak") {
		t.Error("IsValid() should return false for weak password")
	}
}

func TestPolicy_GetRequirements(t *testing.T) {
	policy := NewPolicy(nil)
	reqs := policy.GetRequirements()

	if len(reqs) != 5 {
		t.Errorf("GetRequirements() expected 5 requirements, got %d", len(reqs))
	}
}

func TestNewPolicy_NilConfig(t *testing.T) {
	policy := NewPolicy(nil)
	if policy.config == nil {
		t.Error("NewPolicy(nil) should use default config")
	}
}

func TestHasUppercase(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ABC", true},
		{"abc", false},
		{"aBc", true},
		{"123", false},
		{"", false},
		{"Hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := hasUppercase(tt.input); got != tt.want {
				t.Errorf("hasUppercase(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasLowercase(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ABC", false},
		{"abc", true},
		{"aBc", true},
		{"123", false},
		{"", false},
		{"Hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := hasLowercase(tt.input); got != tt.want {
				t.Errorf("hasLowercase(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasNumber(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ABC", false},
		{"abc", false},
		{"123", true},
		{"abc123", true},
		{"", false},
		{"Hello1", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := hasNumber(tt.input); got != tt.want {
				t.Errorf("hasNumber(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasSpecial(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ABC", false},
		{"abc", false},
		{"123", false},
		{"abc!", true},
		{"@#$", true},
		{"", false},
		{"Hello!", true},
		{"pass-word", true},
		{"pass_word", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := hasSpecial(tt.input); got != tt.want {
				t.Errorf("hasSpecial(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCommonPassword(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"Password", true},
		{"180", true},
		{"qwerty", true},
		{"uniquepassword", false},
		{"", false},
		{"admin", true},
		{"admin123", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isCommonPassword(tt.input); got != tt.want {
				t.Errorf("isCommonPassword(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPolicy_Validate_SpecificErrors(t *testing.T) {
	policy := NewPolicy(nil)

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "too short",
			password: "Aa1!",
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "no uppercase",
			password: "securepass123!a",
			wantErr:  ErrPasswordNoUppercase,
		},
		{
			name:     "no lowercase",
			password: "SECUREPASS123!A",
			wantErr:  ErrPasswordNoLowercase,
		},
		{
			name:     "no number",
			password: "SecurePassword!a",
			wantErr:  ErrPasswordNoNumber,
		},
		{
			name:     "no special",
			password: "SecurePass12345",
			wantErr:  ErrPasswordNoSpecial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := policy.Validate(tt.password)
			found := false
			for _, err := range errs {
				if err == tt.wantErr {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Validate(%q) expected error %v, got %v", tt.password, tt.wantErr, errs)
			}
		})
	}
}

func ExamplePolicy_Validate() {
	policy := NewPolicy(nil)
	errs := policy.Validate("weak")
	for _, err := range errs {
		fmt.Println(err)
	}
}
