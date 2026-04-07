package skillmarket

import (
	"fmt"
	"strings"
)

// NormalizeSkillID converts a human-provided skill identifier into a safe slug.
func NormalizeSkillID(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	normalized := normalizeSkillID(raw)
	if normalized == "skill" && strings.TrimSpace(raw) == "" {
		return ""
	}
	return normalized
}

// ValidateSkillID rejects identifiers that could escape installation roots or create ambiguous paths.
func ValidateSkillID(raw string) (string, error) {
	ensureSkillMarketIdentifierRegex()
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("skill id is required")
	}
	if trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("invalid skill id")
	}
	if strings.ContainsAny(trimmed, `/\`) || strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("invalid skill id")
	}
	if !skillIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid skill id")
	}
	return trimmed, nil
}
