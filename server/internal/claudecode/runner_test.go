package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRunnerRun_UsesPerRequestBackendEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test is unix-only")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "claude")
	script := "#!/bin/sh\nprintf '%s' \"$BLUE_USER_ID\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cfg := &ClaudeCodeConfig{
		Backend: CliBackendConfig{
			Command:          "claude",
			Output:           "text",
			Input:            "stdin",
			SessionMode:      "none",
			SystemPromptWhen: "never",
		},
		Timeout:      5 * time.Second,
		WorkspaceDir: tmpDir,
	}
	runner := NewRunner(cfg)
	// Ensure command resolution picks our temp binary.
	runner.binaryManager = NewBinaryManager(tmpDir)

	params := &RunParams{
		Prompt:       "",
		Model:        "",
		WorkspaceDir: tmpDir,
		Timeout:      3 * time.Second,
		Backend: &CliBackendConfig{
			Command:          "claude",
			Output:           "text",
			Input:            "stdin",
			SessionMode:      "none",
			SystemPromptWhen: "never",
			Env: map[string]string{
				"BLUE_USER_ID": "user-from-params",
			},
		},
	}

	result, err := runner.Run(context.Background(), params)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil || result.Output == nil {
		t.Fatalf("Run() returned nil result/output")
	}
	if got := result.Output.Text; got != "user-from-params" {
		t.Fatalf("stdout text = %q, want %q", got, "user-from-params")
	}
}
