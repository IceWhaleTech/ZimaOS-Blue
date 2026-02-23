// Package password provides password policy validation.
package password

import (
	"errors"
	"strings"
	"unicode"
)

var (
	// ErrPasswordTooShort is returned when password is shorter than minimum length.
	ErrPasswordTooShort = errors.New("password is too short")
	// ErrPasswordNoUppercase is returned when password lacks uppercase letters.
	ErrPasswordNoUppercase = errors.New("password must contain at least one uppercase letter")
	// ErrPasswordNoLowercase is returned when password lacks lowercase letters.
	ErrPasswordNoLowercase = errors.New("password must contain at least one lowercase letter")
	// ErrPasswordNoLetter is returned when password lacks any letters.
	ErrPasswordNoLetter = errors.New("password must contain at least one letter")
	// ErrPasswordNoNumber is returned when password lacks numbers.
	ErrPasswordNoNumber = errors.New("password must contain at least one number")
	// ErrPasswordNoSpecial is returned when password lacks special characters.
	ErrPasswordNoSpecial = errors.New("password must contain at least one special character")
	// ErrPasswordInHistory is returned when password was recently used.
	ErrPasswordInHistory = errors.New("password was recently used")
	// ErrPasswordCommonlyUsed is returned when password is in common passwords list.
	ErrPasswordCommonlyUsed = errors.New("password is too common")
)

// PolicyConfig holds password policy configuration.
type PolicyConfig struct {
	// MinLength is the minimum password length.
	MinLength int
	// RequireUppercase requires at least one uppercase letter.
	RequireUppercase bool
	// RequireLowercase requires at least one lowercase letter.
	RequireLowercase bool
	// RequireLetter requires at least one letter (upper or lower).
	RequireLetter bool
	// RequireNumber requires at least one number.
	RequireNumber bool
	// RequireSpecial requires at least one special character.
	RequireSpecial bool
	// HistoryCount is the number of previous passwords to check against.
	HistoryCount int
	// CheckCommonPasswords enables checking against common passwords.
	CheckCommonPasswords bool
}

// DefaultPolicyConfig returns the recommended password policy configuration.
func DefaultPolicyConfig() *PolicyConfig {
	return &PolicyConfig{
		MinLength:            6,
		RequireUppercase:     false,
		RequireLowercase:     false,
		RequireLetter:        true,
		RequireNumber:        true,
		RequireSpecial:       true,
		HistoryCount:         5,
		CheckCommonPasswords: true,
	}
}

// Policy validates passwords against configured rules.
type Policy struct {
	config *PolicyConfig
}

// NewPolicy creates a new Policy with the given configuration.
// If config is nil, DefaultPolicyConfig() is used.
func NewPolicy(config *PolicyConfig) *Policy {
	if config == nil {
		config = DefaultPolicyConfig()
	}
	return &Policy{config: config}
}

// Config returns the policy configuration.
func (p *Policy) Config() *PolicyConfig {
	return p.config
}

// Validate checks if the password meets all policy requirements.
// Returns a slice of all validation errors, or nil if valid.
func (p *Policy) Validate(password string) []error {
	var errs []error

	if len(password) < p.config.MinLength {
		errs = append(errs, ErrPasswordTooShort)
	}

	if p.config.RequireUppercase && !hasUppercase(password) {
		errs = append(errs, ErrPasswordNoUppercase)
	}

	if p.config.RequireLowercase && !hasLowercase(password) {
		errs = append(errs, ErrPasswordNoLowercase)
	}

	if p.config.RequireLetter && !hasLetter(password) {
		errs = append(errs, ErrPasswordNoLetter)
	}

	if p.config.RequireNumber && !hasNumber(password) {
		errs = append(errs, ErrPasswordNoNumber)
	}

	if p.config.RequireSpecial && !hasSpecial(password) {
		errs = append(errs, ErrPasswordNoSpecial)
	}

	if p.config.CheckCommonPasswords && isCommonPassword(password) {
		errs = append(errs, ErrPasswordCommonlyUsed)
	}

	return errs
}

// ValidateWithHistory checks password against policy and password history.
// passwordHashes should be the hashed versions of previous passwords.
func (p *Policy) ValidateWithHistory(password string, passwordHashes []string, hasher *Hasher) []error {
	errs := p.Validate(password)

	if p.config.HistoryCount > 0 && len(passwordHashes) > 0 {
		// Check against recent password history
		checkCount := p.config.HistoryCount
		if checkCount > len(passwordHashes) {
			checkCount = len(passwordHashes)
		}

		for i := 0; i < checkCount; i++ {
			if hasher.Verify(password, passwordHashes[i]) == nil {
				errs = append(errs, ErrPasswordInHistory)
				break
			}
		}
	}

	return errs
}

// IsValid returns true if the password meets all policy requirements.
func (p *Policy) IsValid(password string) bool {
	return len(p.Validate(password)) == 0
}

// GetRequirements returns a human-readable list of password requirements.
func (p *Policy) GetRequirements() []string {
	var reqs []string

	reqs = append(reqs, "At least "+string(rune('0'+p.config.MinLength/10))+string(rune('0'+p.config.MinLength%10))+" characters")

	if p.config.RequireUppercase {
		reqs = append(reqs, "At least one uppercase letter (A-Z)")
	}
	if p.config.RequireLowercase {
		reqs = append(reqs, "At least one lowercase letter (a-z)")
	}
	if p.config.RequireLetter {
		reqs = append(reqs, "At least one letter (a-z, A-Z)")
	}
	if p.config.RequireNumber {
		reqs = append(reqs, "At least one number (0-9)")
	}
	if p.config.RequireSpecial {
		reqs = append(reqs, "At least one special character (!@#$%^&*...)")
	}

	return reqs
}

func hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func hasLowercase(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func hasNumber(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func hasSpecial(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// commonPasswords is a small set of very common passwords.
// In production, this should be loaded from a larger database.
var commonPasswords = map[string]bool{
	"password":       true,
	"180":         true,
	"18078":       true,
	"qwerty":         true,
	"abc123":         true,
	"monkey":         true,
	"1807":        true,
	"letmein":        true,
	"trustno1":       true,
	"dragon":         true,
	"baseball":       true,
	"iloveyou":       true,
	"master":         true,
	"sunshine":       true,
	"ashley":         true,
	"bailey":         true,
	"passw0rd":       true,
	"shadow":         true,
	"123123":         true,
	"654321":         true,
	"superman":       true,
	"qazwsx":         true,
	"michael":        true,
	"football":       true,
	"password1":      true,
	"password123":    true,
	"welcome":        true,
	"welcome1":       true,
	"admin":          true,
	"admin123":       true,
	"root":           true,
	"toor":           true,
	"pass":           true,
	"test":           true,
	"guest":          true,
	"master123":      true,
	"changeme":       true,
	"180789":      true,
	"18078901":    true,
	"1807890":     true,
	"0987654321":     true,
	"password12":     true,
	"password1234":   true,
	"qwerty123":      true,
	"qwertyuiop":     true,
	"asdfghjkl":      true,
	"zxcvbnm":        true,
	"1q2w3e4r":       true,
	"1qaz2wsx":       true,
}

func isCommonPassword(password string) bool {
	lower := strings.ToLower(password)
	return commonPasswords[lower]
}
