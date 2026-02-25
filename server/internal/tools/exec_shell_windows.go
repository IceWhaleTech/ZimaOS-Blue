//go:build windows

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
// executing a command string. On Windows it uses PowerShell.
func GetShellConfig() (shell string, args []string) {
	ps := resolveWindowsPowerShell()
	return ps, []string{"-NoProfile", "-NonInteractive", "-Command"}
}

func resolveWindowsPowerShell() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = os.Getenv("WINDIR")
	}
	if systemRoot != "" {
		candidate := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "powershell.exe"
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return true
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

// KillProcessTree kills a process and all its children on Windows.
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	// taskkill /F /T /PID <pid>
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	_ = cmd.Run()
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
	"NODE_OPTIONS":  {},
	"NODE_PATH":     {},
	"PYTHONPATH":    {},
	"PYTHONHOME":    {},
	"RUBYLIB":       {},
	"PERL5LIB":      {},
	"SSLKEYLOGFILE": {},
}

// ValidateHostEnv checks that no dangerous environment variables are set.
// It also blocks custom PATH to prevent binary hijacking on the host.
func ValidateHostEnv(env map[string]string) error {
	for key := range env {
		upper := strings.ToUpper(key)

		// Block known dangerous variables.
		if _, ok := dangerousEnvVars[upper]; ok {
			return fmt.Errorf("security violation: environment variable %q is forbidden during host execution", key)
		}

		// Block PATH modification on host.
		if upper == "PATH" {
			return fmt.Errorf("security violation: custom 'PATH' variable is forbidden during host execution")
		}
	}
	return nil
}
