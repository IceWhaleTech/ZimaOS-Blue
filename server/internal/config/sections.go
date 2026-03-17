package config

import (
	"errors"
	"strings"
)

var ErrInvalidSection = errors.New("invalid config section")

var configSectionSet = func() map[string]struct{} {
	allowed := make(map[string]struct{}, len(configSections))
	for _, section := range configSections {
		allowed[section] = struct{}{}
	}
	return allowed
}()

// ValidateSectionName normalizes and validates an externally supplied config section name.
func ValidateSectionName(section string) (string, error) {
	normalized := strings.TrimSpace(section)
	if normalized == "" {
		return "", ErrInvalidSection
	}
	if _, ok := configSectionSet[normalized]; !ok {
		return "", ErrInvalidSection
	}
	return normalized, nil
}
