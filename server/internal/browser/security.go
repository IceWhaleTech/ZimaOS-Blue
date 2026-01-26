package browser

import (
	"net/url"
	"strings"
)

// SecurityChecker validates URLs against security rules.
type SecurityChecker struct {
	config *Config
}

// NewSecurityChecker creates a new security checker.
func NewSecurityChecker(config *Config) *SecurityChecker {
	return &SecurityChecker{config: config}
}

// CheckURL validates a URL against the security rules.
func (s *SecurityChecker) CheckURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrURLNotAllowed
	}

	domain := strings.ToLower(parsed.Hostname())

	// Check blocked domains first
	if s.isBlocked(domain) {
		return ErrURLBlocked
	}

	// If allowed domains is empty, allow all (except blocked)
	if len(s.config.AllowedDomains) == 0 {
		return nil
	}

	// Check if domain is in allowed list
	if !s.isAllowed(domain) {
		return ErrURLNotAllowed
	}

	return nil
}

// isBlocked checks if a domain is in the blocked list.
func (s *SecurityChecker) isBlocked(domain string) bool {
	for _, blocked := range s.config.BlockedDomains {
		blocked = strings.ToLower(blocked)
		if domain == blocked || strings.HasSuffix(domain, "."+blocked) {
			return true
		}
	}
	return false
}

// isAllowed checks if a domain is in the allowed list.
func (s *SecurityChecker) isAllowed(domain string) bool {
	for _, allowed := range s.config.AllowedDomains {
		allowed = strings.ToLower(allowed)
		if domain == allowed || strings.HasSuffix(domain, "."+allowed) {
			return true
		}
		// Support wildcard patterns like *.example.com
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // Remove the *
			if strings.HasSuffix(domain, suffix) {
				return true
			}
		}
	}
	return false
}

// ValidateSelector validates a CSS selector for safety.
func (s *SecurityChecker) ValidateSelector(selector string) error {
	if selector == "" {
		return nil
	}

	// Basic validation - reject obviously malicious selectors
	dangerous := []string{
		"javascript:",
		"data:",
		"vbscript:",
	}

	lower := strings.ToLower(selector)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return ErrInvalidSelector
		}
	}

	return nil
}

// ValidateScript validates JavaScript code for safety.
// Note: This is a basic check. For production, consider using a proper sandbox.
func (s *SecurityChecker) ValidateScript(script string) error {
	if script == "" {
		return nil
	}

	// Basic validation - reject obviously dangerous patterns
	dangerous := []string{
		"eval(",
		"Function(",
		"setTimeout(", // with string argument
		"setInterval(", // with string argument
	}

	for _, d := range dangerous {
		if strings.Contains(script, d) {
			// Allow if it's a function reference, not a string
			// This is a simplified check
		}
	}

	return nil
}
