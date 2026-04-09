//go:build windows

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unsafe"

	"golang.org/x/sys/windows"
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

	rootPID := uint32(pid)
	childrenByParent, err := snapshotWindowsProcessChildren()
	if err != nil {
		_ = terminateWindowsProcess(rootPID)
		return
	}

	for _, procID := range buildWindowsProcessTerminationOrder(rootPID, childrenByParent) {
		_ = terminateWindowsProcess(procID)
	}
}

func snapshotWindowsProcessChildren() (map[uint32][]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}

	childrenByParent := make(map[uint32][]uint32)
	for {
		childrenByParent[entry.ParentProcessID] = append(childrenByParent[entry.ParentProcessID], entry.ProcessID)

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return childrenByParent, err
		}
	}

	return childrenByParent, nil
}

func buildWindowsProcessTerminationOrder(rootPID uint32, childrenByParent map[uint32][]uint32) []uint32 {
	order := make([]uint32, 0, 8)
	visited := make(map[uint32]struct{}, 8)

	var visit func(uint32)
	visit = func(pid uint32) {
		if pid == 0 {
			return
		}
		if _, seen := visited[pid]; seen {
			return
		}
		visited[pid] = struct{}{}

		for _, childPID := range childrenByParent[pid] {
			visit(childPID)
		}

		order = append(order, pid)
	}

	visit(rootPID)
	return order
}

func terminateWindowsProcess(pid uint32) error {
	if pid == 0 {
		return nil
	}

	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	return windows.TerminateProcess(handle, 1)
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
