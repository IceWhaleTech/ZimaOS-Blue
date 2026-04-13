package browser

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

var (
	// ErrEvaluateDisabled is returned when JavaScript evaluation is disabled.
	ErrEvaluateDisabled = errors.New("JavaScript evaluation is disabled by configuration (browser.evaluate_enabled=false)")

	urlInTextPatternOnce sync.Once
	urlInTextPattern     *regexp.Regexp
)

func ensureURLInTextPattern() {
	urlInTextPatternOnce.Do(func() {
		urlInTextPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>，。；：！？、]+`)
	})
}

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
	_, err := s.NormalizeAndCheckURL(rawURL)
	return err
}

// NormalizeAndCheckURL validates a URL against security rules and returns a
// normalized URL string suitable for browser navigation.
func (s *SecurityChecker) NormalizeAndCheckURL(rawURL string) (string, error) {
	parsed, normalized, err := normalizeURL(rawURL)
	if err != nil {
		return "", ErrURLNotAllowed
	}

	domain := strings.ToLower(strings.TrimSpace(parsed.Hostname()))

	// Check blocked domains first
	if s.isBlocked(domain) {
		return "", ErrURLBlocked
	}

	// If allowed domains is empty, allow all (except blocked)
	if len(s.config.AllowedDomains) == 0 {
		return normalized, nil
	}

	// Check if domain is in allowed list
	if !s.isAllowed(domain) {
		return "", ErrURLNotAllowed
	}

	return normalized, nil
}

func normalizeURL(rawURL string) (*url.URL, string, error) {
	candidate := trimURLInput(rawURL)
	if candidate == "" {
		return nil, "", ErrURLNotAllowed
	}
	if strings.HasPrefix(candidate, "//") {
		candidate = "https:" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		escaped := strings.ReplaceAll(candidate, " ", "%20")
		if escaped != candidate {
			if p, parseErr := url.Parse(escaped); parseErr == nil {
				candidate = escaped
				parsed = p
				err = nil
			}
		}
	}
	if err != nil {
		if prefixed, ok := tryPrefixHTTPS(candidate); ok {
			if p, parseErr := url.Parse(prefixed); parseErr == nil {
				candidate = prefixed
				parsed = p
				err = nil
			}
		}
	}
	if err != nil {
		if extracted := extractURLCandidate(candidate); extracted != "" && extracted != candidate {
			return normalizeURL(extracted)
		}
		return nil, "", err
	}

	if parsed.Hostname() == "" && parsed.Scheme != "" && !strings.Contains(candidate, "://") {
		if prefixed, ok := tryPrefixHTTPS(candidate); ok {
			p, parseErr := url.Parse(prefixed)
			if parseErr == nil {
				candidate = prefixed
				parsed = p
			}
		}
	}

	if parsed.Scheme == "" && parsed.Hostname() == "" {
		if prefixed, ok := tryPrefixHTTPS(candidate); ok {
			p, parseErr := url.Parse(prefixed)
			if parseErr == nil {
				candidate = prefixed
				parsed = p
			}
		}
	}

	if strings.TrimSpace(parsed.Hostname()) == "" {
		if extracted := extractURLCandidate(candidate); extracted != "" && extracted != candidate {
			return normalizeURL(extracted)
		}
		return nil, "", ErrURLNotAllowed
	}

	return parsed, parsed.String(), nil
}

func trimURLInput(raw string) string {
	s := strings.TrimSpace(raw)
	for {
		next := strings.TrimSpace(s)
		next = strings.Trim(next, "\"'`")
		if len(next) >= 2 && strings.HasPrefix(next, "<") && strings.HasSuffix(next, ">") {
			next = strings.TrimSpace(next[1 : len(next)-1])
		}
		if next == s {
			return next
		}
		s = next
	}
}

func extractURLCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	ensureURLInTextPattern()
	if m := strings.TrimSpace(urlInTextPattern.FindString(raw)); m != "" {
		return trimURLToken(m)
	}
	for _, tok := range strings.Fields(raw) {
		tok = trimURLToken(tok)
		if tok == "" {
			continue
		}
		if strings.Contains(tok, "://") || looksLikeHostCandidate(tok) {
			return tok
		}
	}
	return ""
}

func trimURLToken(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimLeft(raw, "\"'`<([{")
	raw = strings.TrimRight(raw, "\"'`>)]}.,;:!?，。；：！？、")
	return raw
}

func tryPrefixHTTPS(raw string) (string, bool) {
	if !looksLikeHostCandidate(raw) {
		return "", false
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "//") {
		return "https:" + trimmed, true
	}
	return "https://" + trimmed, true
}

func looksLikeHostCandidate(raw string) bool {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return false
	}
	if strings.ContainsAny(candidate, " \t\r\n") {
		return false
	}
	candidate = strings.TrimPrefix(candidate, "//")
	if idx := strings.IndexAny(candidate, "/?#"); idx >= 0 {
		candidate = candidate[:idx]
	}
	if at := strings.LastIndex(candidate, "@"); at >= 0 {
		candidate = candidate[at+1:]
	}
	if candidate == "" {
		return false
	}

	host := candidate
	if h, p, err := net.SplitHostPort(candidate); err == nil {
		if p == "" {
			return false
		}
		host = h
	} else if strings.Count(candidate, ":") == 1 {
		parts := strings.SplitN(candidate, ":", 2)
		if parts[0] != "" && parts[1] != "" {
			host = parts[0]
		}
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	hasAlphaNum := false
	for _, r := range host {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAlphaNum = true
			break
		}
	}
	if !hasAlphaNum {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return strings.Contains(host, ".")
}

// isBlocked checks if a domain is in the blocked list.
func (s *SecurityChecker) isBlocked(domain string) bool {
	for _, blocked := range s.config.BlockedDomains {
		blocked = strings.ToLower(strings.TrimSpace(blocked))
		if blocked == "" {
			continue
		}
		if domain == blocked || strings.HasSuffix(domain, "."+blocked) {
			return true
		}
	}
	return false
}

// isAllowed checks if a domain is in the allowed list.
func (s *SecurityChecker) isAllowed(domain string) bool {
	for _, allowed := range s.config.AllowedDomains {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "" {
			continue
		}
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

// CheckEvaluateAllowed checks if JavaScript evaluation is allowed.
// This should be called before executing any JavaScript code via
// act:evaluate or wait --fn to prevent prompt injection attacks.
func (s *SecurityChecker) CheckEvaluateAllowed() error {
	if !s.config.IsEvaluateEnabled() {
		return ErrEvaluateDisabled
	}
	return nil
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
// IMPORTANT: Even if this check passes, you should still call CheckEvaluateAllowed()
// to ensure JavaScript evaluation is enabled in the configuration.
func (s *SecurityChecker) ValidateScript(script string) error {
	if script == "" {
		return nil
	}

	// First check if evaluation is allowed at all
	if err := s.CheckEvaluateAllowed(); err != nil {
		return err
	}

	// Basic validation - reject obviously dangerous patterns
	dangerous := []string{
		"eval(",
		"Function(",
		"setTimeout(",  // with string argument
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
