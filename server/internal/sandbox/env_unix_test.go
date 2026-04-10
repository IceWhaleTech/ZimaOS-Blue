//go:build !windows

package sandbox

import (
	"runtime"
	"strings"
	"testing"
)

func TestDefaultSandboxPATHIncludesSystemDirectories(t *testing.T) {
	pathValue := defaultSandboxPATH()

	for _, want := range []string{
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/bin",
		"/usr/sbin",
		"/bin",
		"/usr/libexec",
	} {
		if !strings.Contains(pathValue, want) {
			t.Fatalf("defaultSandboxPATH() = %q, want to contain %q", pathValue, want)
		}
	}

	if runtime.GOOS == "darwin" {
		for _, want := range []string{"/opt/homebrew/bin", "/opt/homebrew/sbin"} {
			if !strings.Contains(pathValue, want) {
				t.Fatalf("defaultSandboxPATH() = %q, want to contain %q on darwin", pathValue, want)
			}
		}
	}
}

func TestDefaultSandboxEnvMapPrependsRequestPATH(t *testing.T) {
	env := defaultSandboxEnvMap(map[string]string{
		"PATH": "/custom/bin",
		"LANG": "en_US.UTF-8",
	})

	if got := env["PATH"]; !strings.HasPrefix(got, "/custom/bin:") {
		t.Fatalf("PATH = %q, want request path to be prepended", got)
	}
	if got := env["HOME"]; got != "/tmp" {
		t.Fatalf("HOME = %q, want /tmp", got)
	}
	if got := env["TMPDIR"]; got != "/tmp" {
		t.Fatalf("TMPDIR = %q, want /tmp", got)
	}
	if got := env["LANG"]; got != "en_US.UTF-8" {
		t.Fatalf("LANG = %q, want request value preserved", got)
	}
}
