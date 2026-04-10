//go:build !windows

package sandbox

import (
	"os"
	"runtime"
	"strings"
)

func defaultSandboxEnvMap(requestEnv map[string]string) map[string]string {
	env := map[string]string{
		"PATH":   defaultSandboxPATH(),
		"HOME":   "/tmp",
		"TMPDIR": "/tmp",
	}
	for key, value := range requestEnv {
		if key == "PATH" {
			env["PATH"] = prependSandboxPATH(value, env["PATH"])
			continue
		}
		env[key] = value
	}
	return env
}

func defaultSandboxPATH() string {
	entries := make([]string, 0, 8)
	if runtime.GOOS == "darwin" {
		entries = append(entries, "/opt/homebrew/bin", "/opt/homebrew/sbin")
	}
	entries = append(entries,
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/bin",
		"/usr/sbin",
		"/bin",
		"/usr/libexec",
	)
	return strings.Join(entries, string(os.PathListSeparator))
}

func prependSandboxPATH(prefix, fallback string) string {
	prefix = strings.TrimSpace(prefix)
	fallback = strings.TrimSpace(fallback)
	switch {
	case prefix == "":
		return fallback
	case fallback == "":
		return prefix
	default:
		return prefix + string(os.PathListSeparator) + fallback
	}
}
