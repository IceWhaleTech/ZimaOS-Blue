package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
)

func TestBuildExecCLIRequest_UsesCommandParam(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	req, err := buildExecCLIRequest([]string{`command=printf 'hello'`})
	if err != nil {
		t.Fatalf("buildExecCLIRequest() error = %v", err)
	}
	if req.Command != "printf 'hello'" {
		t.Fatalf("Command = %q, want %q", req.Command, "printf 'hello'")
	}
	if req.UseSandbox {
		t.Fatal("UseSandbox = true, want false")
	}
	if req.SandboxTier != sandbox.TierLight {
		t.Fatalf("SandboxTier = %q, want %q", req.SandboxTier, sandbox.TierLight)
	}
}

func TestBuildExecCLIRequest_RejectsInvalidEnvEntry(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	execEnvValues = []string{"BROKEN"}

	_, err := buildExecCLIRequest([]string{`command=echo hello`})
	if err == nil {
		t.Fatal("buildExecCLIRequest() error = nil, want invalid env error")
	}
	if !strings.Contains(err.Error(), "KEY=VALUE") {
		t.Fatalf("error = %v, want KEY=VALUE guidance", err)
	}
}

func TestBuildExecCLIRequest_QuotesPositionalArgs(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	req, err := buildExecCLIRequest([]string{"printf", "%s", "hello world"})
	if err != nil {
		t.Fatalf("buildExecCLIRequest() error = %v", err)
	}

	want := "printf '%s' 'hello world'"
	if runtime.GOOS == "windows" {
		want = `printf "%s" "hello world"`
	}
	if req.Command != want {
		t.Fatalf("Command = %q, want %q", req.Command, want)
	}
}

func TestExecuteExecCLI_UsesHostRunnerByDefault(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	oldHostRunner := execCLIHostRunner
	oldSandboxRunner := execCLISandboxRunner
	defer func() {
		execCLIHostRunner = oldHostRunner
		execCLISandboxRunner = oldSandboxRunner
	}()

	var captured execCLIRequest
	execCLIHostRunner = func(_ context.Context, req execCLIRequest) (*execCLIResult, error) {
		captured = req
		return &execCLIResult{
			Stdout:   "host\n",
			ExitCode: 0,
		}, nil
	}
	execCLISandboxRunner = func(context.Context, execCLIRequest) (*execCLIResult, error) {
		t.Fatal("sandbox runner should not be called")
		return nil, nil
	}

	result, err := executeExecCLI(context.Background(), []string{`command=echo host`})
	if err != nil {
		t.Fatalf("executeExecCLI() error = %v", err)
	}
	if captured.Command != "echo host" {
		t.Fatalf("captured.Command = %q, want %q", captured.Command, "echo host")
	}
	if result.Stdout != "host\n" {
		t.Fatalf("result.Stdout = %q, want %q", result.Stdout, "host\n")
	}
}

func TestExecuteExecCLI_UsesSandboxRunnerWhenRequested(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	execUseSandbox = true
	execSandboxTier = "strong"

	oldHostRunner := execCLIHostRunner
	oldSandboxRunner := execCLISandboxRunner
	defer func() {
		execCLIHostRunner = oldHostRunner
		execCLISandboxRunner = oldSandboxRunner
	}()

	var captured execCLIRequest
	execCLIHostRunner = func(context.Context, execCLIRequest) (*execCLIResult, error) {
		t.Fatal("host runner should not be called")
		return nil, nil
	}
	execCLISandboxRunner = func(_ context.Context, req execCLIRequest) (*execCLIResult, error) {
		captured = req
		return &execCLIResult{
			Stdout:      "sandbox\n",
			ExitCode:    0,
			Sandbox:     true,
			SandboxTier: string(req.SandboxTier),
		}, nil
	}

	result, err := executeExecCLI(context.Background(), []string{`command=echo sandbox`})
	if err != nil {
		t.Fatalf("executeExecCLI() error = %v", err)
	}
	if captured.SandboxTier != sandbox.TierStrong {
		t.Fatalf("captured.SandboxTier = %q, want %q", captured.SandboxTier, sandbox.TierStrong)
	}
	if !result.Sandbox {
		t.Fatal("result.Sandbox = false, want true")
	}
	if result.SandboxTier != string(sandbox.TierStrong) {
		t.Fatalf("result.SandboxTier = %q, want %q", result.SandboxTier, sandbox.TierStrong)
	}
}

func TestBuildExecCLIRequest_SandboxDefersDefaultTimeoutToManager(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	execUseSandbox = true

	req, err := buildExecCLIRequest([]string{`command=echo sandbox`})
	if err != nil {
		t.Fatalf("buildExecCLIRequest() error = %v", err)
	}
	if req.Timeout != 0 {
		t.Fatalf("Timeout = %v, want 0 so sandbox manager applies configured default", req.Timeout)
	}
}

func TestRunExec_JSONOutputIncludesSandboxMetadata(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	jsonOutput = true
	execUseSandbox = true
	execSandboxTier = "strong"

	oldSandboxRunner := execCLISandboxRunner
	defer func() {
		execCLISandboxRunner = oldSandboxRunner
	}()

	execCLISandboxRunner = func(context.Context, execCLIRequest) (*execCLIResult, error) {
		return &execCLIResult{
			Stdout:      "hello\n",
			Stderr:      "warn\n",
			ExitCode:    0,
			Sandbox:     true,
			SandboxTier: string(sandbox.TierStrong),
		}, nil
	}

	exitCode, stdout, stderr := runExecForTest(func() {
		runExec(nil, []string{`command=echo hello`})
	})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stderr=%q)", exitCode, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, stdout)
	}
	if got, _ := payload["sandbox"].(bool); !got {
		t.Fatalf("sandbox = %#v, want true", payload["sandbox"])
	}
	if got, _ := payload["sandbox_tier"].(string); got != "strong" {
		t.Fatalf("sandbox_tier = %#v, want %q", payload["sandbox_tier"], "strong")
	}
	if got, _ := payload["stdout"].(string); got != "hello\n" {
		t.Fatalf("stdout = %#v, want %q", payload["stdout"], "hello\n")
	}
}

func TestRunExec_ExitsWithCommandExitCode(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	oldHostRunner := execCLIHostRunner
	defer func() {
		execCLIHostRunner = oldHostRunner
	}()

	execCLIHostRunner = func(context.Context, execCLIRequest) (*execCLIResult, error) {
		return &execCLIResult{
			Stdout:   "partial\n",
			ExitCode: 7,
		}, nil
	}

	exitCode, stdout, stderr := runExecForTest(func() {
		runExec(nil, []string{`command=exit 7`})
	})
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7 (stdout=%q stderr=%q)", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "partial") {
		t.Fatalf("stdout = %q, want command output", stdout)
	}
}

func TestRunExecCLIOnHost_ExecutesCommandWithEnv(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	command := `printf '%s' "$BLUE_EXEC_TEST"`
	if runtime.GOOS == "windows" {
		command = `Write-Output $env:BLUE_EXEC_TEST`
	}

	result, err := runExecCLIOnHost(context.Background(), execCLIRequest{
		Command: command,
		Env: map[string]string{
			"BLUE_EXEC_TEST": "host-value",
		},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("runExecCLIOnHost() error = %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "host-value" {
		t.Fatalf("Stdout = %q, want %q", result.Stdout, "host-value")
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func resetExecCLIGlobals(t *testing.T) func() {
	t.Helper()

	oldCommand := execCommandText
	oldWorkdir := execWorkdir
	oldTimeout := execTimeout
	oldEnvValues := append([]string(nil), execEnvValues...)
	oldStdin := execStdin
	oldUseSandbox := execUseSandbox
	oldSandboxTier := execSandboxTier
	oldJSONOutput := jsonOutput

	execCommandText = ""
	execWorkdir = ""
	execTimeout = 0
	execEnvValues = nil
	execStdin = ""
	execUseSandbox = false
	execSandboxTier = "light"
	jsonOutput = false

	return func() {
		execCommandText = oldCommand
		execWorkdir = oldWorkdir
		execTimeout = oldTimeout
		execEnvValues = oldEnvValues
		execStdin = oldStdin
		execUseSandbox = oldUseSandbox
		execSandboxTier = oldSandboxTier
		jsonOutput = oldJSONOutput
	}
}

type execCommandExitPanic struct {
	code int
}

func runExecForTest(fn func()) (exitCode int, stdout string, stderr string) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldExit := execCLIExit

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	execCLIExit = func(code int) { panic(execCommandExitPanic{code: code}) }

	defer func() {
		execCLIExit = oldExit
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()

		var stdoutBuf bytes.Buffer
		var stderrBuf bytes.Buffer
		_, _ = io.Copy(&stdoutBuf, stdoutR)
		_, _ = io.Copy(&stderrBuf, stderrR)
		_ = stdoutR.Close()
		_ = stderrR.Close()
		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()

		if rec := recover(); rec != nil {
			if exit, ok := rec.(execCommandExitPanic); ok {
				exitCode = exit.code
				return
			}
			panic(rec)
		}
	}()

	fn()
	return 0, "", ""
}

func TestRunExec_DefaultTimeoutIsAcceptableForTests(t *testing.T) {
	reset := resetExecCLIGlobals(t)
	defer reset()

	req, err := buildExecCLIRequest([]string{`command=echo hello`})
	if err != nil {
		t.Fatalf("buildExecCLIRequest() error = %v", err)
	}
	if req.Timeout < 0 || req.Timeout > 30*time.Minute {
		t.Fatalf("Timeout = %v, expected bounded default timeout", req.Timeout)
	}
}

func TestRootCommand_RegistersExecCommand(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"exec"})
	if err != nil {
		t.Fatalf("rootCmd.Find(exec): %v", err)
	}
	if cmd == nil || cmd.Name() != "exec" {
		t.Fatalf("expected exec command to be registered, got %#v", cmd)
	}
}
