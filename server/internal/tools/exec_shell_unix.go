//go:build !windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
)

// GetShellConfig returns the platform-appropriate shell and arguments for
// executing a command string. On Unix it prefers $SHELL (falling back to
// bash → sh).
func GetShellConfig() (shell string, args []string) {
	envShell := strings.TrimSpace(os.Getenv("SHELL"))
	shellName := filepath.Base(envShell)

	// Fish rejects common bashisms — prefer bash when fish is detected.
	if shellName == "fish" {
		if bash := resolveShellFromPath("bash"); bash != "" {
			return bash, []string{"-c"}
		}
		if sh := resolveShellFromPath("sh"); sh != "" {
			return sh, []string{"-c"}
		}
	}

	if envShell != "" {
		return envShell, []string{"-c"}
	}
	return "sh", []string{"-c"}
}

func resolveShellFromPath(name string) string {
	envPath := os.Getenv("PATH")
	if envPath == "" {
		return ""
	}
	for _, dir := range filepath.SplitList(envPath) {
		candidate := filepath.Join(dir, name)
		if isExecutable(candidate) {
			return candidate
		}
	}
	return ""
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}

// SanitizeBinaryOutput strips control characters (except tab, newline, CR)
// and Unicode format/surrogate characters from process output.
func SanitizeBinaryOutput(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		if r < 0x20 {
			continue
		}
		if unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cs, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// KillProcessTree kills a process and all its children on Unix.
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	// Try killing the process group first (negative PID).
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		// Fall back to killing just the process.
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// ResolveWorkdir validates a working directory path and returns the resolved
// path plus any warnings. Falls back to cwd or home if the path is invalid.
func ResolveWorkdir(workdir string) (string, []string) {
	var warnings []string

	if workdir != "" {
		info, err := os.Stat(workdir)
		if err == nil && info.IsDir() {
			return workdir, nil
		}
	}

	cwd, err := os.Getwd()
	if err == nil {
		if workdir != "" {
			warnings = append(warnings, fmt.Sprintf("Warning: workdir %q is unavailable; using %q.", workdir, cwd))
		}
		return cwd, warnings
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.TempDir()
	}
	if workdir != "" {
		warnings = append(warnings, fmt.Sprintf("Warning: workdir %q is unavailable; using %q.", workdir, home))
	}
	return home, warnings
}

// Dangerous environment variables that could alter execution flow or inject
// code when running on the host (non-sandboxed).
var dangerousEnvVars = map[string]struct{}{
	"LD_PRELOAD":            {},
	"LD_LIBRARY_PATH":       {},
	"LD_AUDIT":              {},
	"DYLD_INSERT_LIBRARIES": {},
	"DYLD_LIBRARY_PATH":     {},
	"NODE_OPTIONS":          {},
	"NODE_PATH":             {},
	"PYTHONPATH":            {},
	"PYTHONHOME":            {},
	"RUBYLIB":               {},
	"PERL5LIB":              {},
	"BASH_ENV":              {},
	"ENV":                   {},
	"GCONV_PATH":            {},
	"IFS":                   {},
	"SSLKEYLOGFILE":         {},
}

var dangerousEnvPrefixes = []string{"DYLD_", "LD_"}

// ValidateHostEnv checks that no dangerous environment variables are set.
// It also blocks custom PATH to prevent binary hijacking on the host.
func ValidateHostEnv(env map[string]string) error {
	for key := range env {
		upper := strings.ToUpper(key)

		// Block known dangerous variables.
		if _, ok := dangerousEnvVars[upper]; ok {
			return fmt.Errorf("security violation: environment variable %q is forbidden during host execution", key)
		}

		// Block dangerous prefixes.
		for _, prefix := range dangerousEnvPrefixes {
			if strings.HasPrefix(upper, prefix) {
				return fmt.Errorf("security violation: environment variable %q is forbidden during host execution", key)
			}
		}

		// Block PATH modification on host.
		if upper == "PATH" {
			return fmt.Errorf("security violation: custom 'PATH' variable is forbidden during host execution")
		}
	}
	return nil
}
