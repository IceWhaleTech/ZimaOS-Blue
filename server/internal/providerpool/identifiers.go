package providerpool

import (
	"fmt"
	"regexp"
	"strings"
)

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// ValidateProviderID ensures provider IDs are safe for routing and file-backed storage.
func ValidateProviderID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", fmt.Errorf("%w: provider id is required", ErrInvalidProviderID)
	}
	if !providerIDPattern.MatchString(id) {
		return "", fmt.Errorf("%w: provider id must use only letters, numbers, underscores, or hyphens", ErrInvalidProviderID)
	}
	return id, nil
}
