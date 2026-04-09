package sandbox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCodexSandboxExecutor_UnsupportedWhenNetworkEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NetworkEnabled = true

	executor, err := NewCodexSandboxExecutor(cfg, CodexSandboxExecutorOptions{
		Subcommand: "windows",
		LookPath: func(string) (string, error) {
			return "/tmp/codex", nil
		},
	})
	if err != nil {
		t.Fatalf("NewCodexSandboxExecutor() error = %v", err)
	}
	if executor.IsSupported() {
		t.Fatal("executor should be unsupported when network_enabled=true")
	}
	if executor.SupportReason() == "" || !strings.Contains(executor.SupportReason(), "network_enabled=false") {
		t.Fatalf("SupportReason() = %q, want mention of network_enabled=false", executor.SupportReason())
	}
	if executor.SupportsNetworkEnabled() {
		t.Fatal("SupportsNetworkEnabled() = true, want false")
	}
}

func TestNewCodexSandboxExecutor_UnsupportedWhenBinaryMissing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WindowsCodexExecutable = ""
	cfg.WindowsCodexAutoDownload = false

	executor, err := NewCodexSandboxExecutor(cfg, CodexSandboxExecutorOptions{
		Subcommand: "windows",
		LookPath: func(string) (string, error) {
			return "", errors.New("not found")
		},
	})
	if err != nil {
		t.Fatalf("NewCodexSandboxExecutor() error = %v", err)
	}
	if executor.IsSupported() {
		t.Fatal("executor should be unsupported when codex binary is missing")
	}
	if executor.SupportReason() == "" || !strings.Contains(strings.ToLower(executor.SupportReason()), "not found") {
		t.Fatalf("SupportReason() = %q, want not-found detail", executor.SupportReason())
	}
}

func TestNewCodexSandboxExecutor_SupportedWhenBinaryCanBeDownloadedOnDemand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WindowsCodexExecutable = ""
	cfg.WindowsCodexAutoDownload = true

	manager := NewCodexBinaryManager(cfg, CodexBinaryManagerOptions{
		GOOS:         "windows",
		GOARCH:       "amd64",
		UserCacheDir: func() (string, error) { return t.TempDir(), nil },
		LookPath:     func(string) (string, error) { return "", os.ErrNotExist },
	})

	executor, err := NewCodexSandboxExecutor(cfg, CodexSandboxExecutorOptions{
		Subcommand:     "windows",
		BinaryResolver: manager,
	})
	if err != nil {
		t.Fatalf("NewCodexSandboxExecutor() error = %v", err)
	}
	if !executor.IsSupported() {
		t.Fatalf("executor should remain supported when auto-download can provision the binary, got reason %q", executor.SupportReason())
	}
}

func TestNewCodexSandboxExecutor_ExecuteWrapsCommand(t *testing.T) {
	binary := writeCodexSandboxTestBinary(t, `#!/bin/sh
if [ "$1" = "sandbox" ] && [ "$2" = "windows" ] && [ "$3" = "--help" ]; then
  echo "help ok"
  exit 0
fi
printf 'pwd=%s\n' "$PWD"
index=1
for arg in "$@"; do
  printf 'arg%d=%s\n' "$index" "$arg"
  index=$((index+1))
done
printf 'env_BLUE_TEST=%s\n' "$BLUE_TEST"
stdin_contents=$(cat)
printf 'stdin=%s\n' "$stdin_contents"
`)

	workdir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	resolvedWorkdir, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		resolvedWorkdir = workdir
	}

	cfg := DefaultConfig()
	cfg.NetworkEnabled = false

	executor, err := NewCodexSandboxExecutor(cfg, CodexSandboxExecutorOptions{
		Binary:     binary,
		Subcommand: "windows",
	})
	if err != nil {
		t.Fatalf("NewCodexSandboxExecutor() error = %v", err)
	}
	if !executor.IsSupported() {
		t.Fatalf("executor should be supported, got reason %q", executor.SupportReason())
	}

	req := NewExecutionRequest("powershell.exe", "-NoProfile", "Write-Output", "ok")
	req.Timeout = 5 * time.Second
	req.WorkDir = workdir
	req.Env = map[string]string{"BLUE_TEST": "codex-value"}
	req.Stdin = "hello codex"

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Status != StatusCompleted {
		t.Fatalf("Execute() status = %s, want %s (stderr=%q)", result.Status, StatusCompleted, result.Stderr)
	}

	for _, want := range []string{
		"pwd=" + resolvedWorkdir,
		"arg1=sandbox",
		"arg2=windows",
		"arg3=--full-auto",
		"arg4=powershell.exe",
		"arg5=-NoProfile",
		"arg6=Write-Output",
		"arg7=ok",
		"env_BLUE_TEST=codex-value",
		"stdin=hello codex",
	} {
		if !strings.Contains(result.Stdout, want) {
			t.Fatalf("stdout = %q, want substring %q", result.Stdout, want)
		}
	}
}

func TestNewCodexSandboxExecutor_UnsupportedWhenProbeFails(t *testing.T) {
	binary := writeCodexSandboxTestBinary(t, `#!/bin/sh
if [ "$1" = "sandbox" ] && [ "$2" = "windows" ] && [ "$3" = "--help" ]; then
  echo "probe failed" >&2
  exit 12
fi
exit 0
`)

	cfg := DefaultConfig()
	cfg.WindowsCodexAutoDownload = false
	executor, err := NewCodexSandboxExecutor(cfg, CodexSandboxExecutorOptions{
		Binary:     binary,
		Subcommand: "windows",
	})
	if err != nil {
		t.Fatalf("NewCodexSandboxExecutor() error = %v", err)
	}
	if executor.IsSupported() {
		t.Fatal("executor should be unsupported when help probe fails")
	}
	if executor.SupportReason() == "" || !strings.Contains(strings.ToLower(executor.SupportReason()), "probe") {
		t.Fatalf("SupportReason() = %q, want probe failure detail", executor.SupportReason())
	}
}

func TestCodexSandboxExecutor_ExecuteDownloadsManagedBinaryWhenReadyProbeFails(t *testing.T) {
	badBinary := writeCodexSandboxTestBinary(t, `#!/bin/sh
if [ "$1" = "sandbox" ] && [ "$2" = "windows" ] && [ "$3" = "--help" ]; then
  echo "bad probe" >&2
  exit 9
fi
echo "bad binary should not execute"
exit 99
`)

	downloadedScript := `#!/bin/sh
if [ "$1" = "sandbox" ] && [ "$2" = "windows" ] && [ "$3" = "--help" ]; then
  echo "help ok"
  exit 0
fi
echo "binary=downloaded"
index=1
for arg in "$@"; do
  printf 'arg%d=%s\n' "$index" "$arg"
  index=$((index+1))
done
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/codex-x86_64-pc-windows-msvc.exe") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(downloadedScript))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.WindowsCodexExecutable = ""
	cfg.WindowsCodexAutoDownload = true

	manager := NewCodexBinaryManager(cfg, CodexBinaryManagerOptions{
		GOOS:         "windows",
		GOARCH:       "amd64",
		UserCacheDir: func() (string, error) { return t.TempDir(), nil },
		LookPath: func(string) (string, error) {
			return badBinary, nil
		},
		HTTPClient: server.Client(),
		DownloadSources: []string{
			server.URL + "/blue",
		},
	})

	executor, err := NewCodexSandboxExecutor(cfg, CodexSandboxExecutorOptions{
		Subcommand:     "windows",
		BinaryResolver: manager,
	})
	if err != nil {
		t.Fatalf("NewCodexSandboxExecutor() error = %v", err)
	}
	if !executor.IsSupported() {
		t.Fatalf("executor should stay supported when download fallback is enabled, got reason %q", executor.SupportReason())
	}

	req := NewExecutionRequest("powershell.exe", "-NoProfile", "Write-Output", "ok")
	req.Timeout = 5 * time.Second

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("Execute() status = %s, want %s (stderr=%q)", result.Status, StatusCompleted, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "binary=downloaded") {
		t.Fatalf("stdout = %q, want managed downloaded binary output", result.Stdout)
	}
	if strings.Contains(result.Stdout, "bad binary should not execute") {
		t.Fatalf("stdout = %q, unexpected output from bad ready binary", result.Stdout)
	}
}

func writeCodexSandboxTestBinary(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "codex-test")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
