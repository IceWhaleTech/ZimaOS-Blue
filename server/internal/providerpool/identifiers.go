package providerpool

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	providerIDPattern     *regexp.Regexp
	providerIDPatternOnce sync.Once
)

func providerIDRegexp() *regexp.Regexp {
	providerIDPatternOnce.Do(func() {
		providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
	})
	return providerIDPattern
}

// ValidateProviderID ensures provider IDs are safe for routing and file-backed storage.
func ValidateProviderID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", fmt.Errorf("%w: provider id is required", ErrInvalidProviderID)
	}
	if !providerIDRegexp().MatchString(id) {
		return "", fmt.Errorf("%w: provider id must use only letters, numbers, underscores, or hyphens", ErrInvalidProviderID)
	}
	return id, nil
}
