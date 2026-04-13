package plugin

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	pluginIDPatternOnce sync.Once
	pluginIDPattern     *regexp.Regexp
)

func ensurePluginIDPattern() {
	pluginIDPatternOnce.Do(func() {
		pluginIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	})
}

func ValidatePluginID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	if trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("invalid plugin id")
	}
	if strings.ContainsAny(trimmed, `/\`) || strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("invalid plugin id")
	}
	ensurePluginIDPattern()
	if !pluginIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid plugin id")
	}
	return trimmed, nil
}
