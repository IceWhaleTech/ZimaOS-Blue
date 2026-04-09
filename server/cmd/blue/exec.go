package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/spf13/cobra"
)

var (
	execCommandText string
	execWorkdir     string
	execTimeout     time.Duration
	execEnvValues   []string
	execStdin       string
	execUseSandbox  bool
	execSandboxTier string
)

var execCmd = &cobra.Command{
	Use:   "exec [command]",
	Short: "Run a command locally or through Blue's sandbox",
	Long: `Run a shell command from the Blue CLI.

Examples:
  blue exec command='echo hello'
  blue exec --sandbox --sandbox-tier strong command='python -V'
  blue exec echo hello`,
	Args: cobra.ArbitraryArgs,
	Run:  runExec,
}

type execCLIRequest struct {
	Command     string
	Workdir     string
	Timeout     time.Duration
	Env         map[string]string
	Stdin       string
	UseSandbox  bool
	SandboxTier sandbox.Tier
}

type execCLIResult struct {
	Stdout      string   `json:"stdout,omitempty"`
	Stderr      string   `json:"stderr,omitempty"`
	ExitCode    int      `json:"exit_code"`
	Sandbox     bool     `json:"sandbox"`
	SandboxTier string   `json:"sandbox_tier,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

var (
	execCLIExit                 = os.Exit
	execCLIHostRunner           = runExecCLIOnHost
	execCLISandboxRunner        = runExecCLIInSandbox
	execCLISandboxManagerLoader = loadExecCLISandboxManager
)

func init() {
	execCmd.Flags().StringVar(&execCommandText, "command", "", "command string to execute")
	execCmd.Flags().StringVar(&execWorkdir, "workdir", "", "working directory")
	execCmd.Flags().DurationVar(&execTimeout, "timeout", 0, "execution timeout")
	execCmd.Flags().StringArrayVar(&execEnvValues, "env", nil, "environment variable in KEY=VALUE form")
	execCmd.Flags().StringVar(&execStdin, "stdin", "", "stdin text to pass to the command")
	execCmd.Flags().BoolVar(&execUseSandbox, "sandbox", false, "run the command inside Blue's sandbox")
	execCmd.Flags().StringVar(&execSandboxTier, "sandbox-tier", "light", "sandbox tier: light or strong")

	rootCmd.AddCommand(execCmd)
}

func runExec(_ *cobra.Command, args []string) {
	result, err := executeExecCLI(context.Background(), args)
	if err != nil {
		if jsonOutput {
			printJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		execCLIExit(1)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "%s\n", warning)
		}
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
	}

	if result.ExitCode != 0 {
		execCLIExit(result.ExitCode)
	}
}

func executeExecCLI(ctx context.Context, args []string) (*execCLIResult, error) {
	req, err := buildExecCLIRequest(args)
	if err != nil {
		return nil, err
	}
	if req.UseSandbox {
		return execCLISandboxRunner(ctx, req)
	}
	return execCLIHostRunner(ctx, req)
}

func buildExecCLIRequest(args []string) (execCLIRequest, error) {
	params, positional := parseIPCArgs(args)

	command := strings.TrimSpace(execCommandText)
	if command == "" {
		command = strings.TrimSpace(params["command"])
	}
	if command == "" && len(positional) > 0 {
		command = buildExecCLICommandFromPositional(positional)
	}
	if command == "" {
		return execCLIRequest{}, fmt.Errorf("command is required; use --command, command='...', or positional arguments")
	}

	envMap, err := parseExecCLIEnv(execEnvValues)
	if err != nil {
		return execCLIRequest{}, err
	}

	tier, err := parseExecCLISandboxTier(execSandboxTier)
	if err != nil {
		return execCLIRequest{}, err
	}
	if !execUseSandbox && tier != sandbox.TierLight {
		return execCLIRequest{}, fmt.Errorf("--sandbox-tier requires --sandbox")
	}

	timeout := execTimeout
	if timeout < 0 {
		return execCLIRequest{}, fmt.Errorf("--timeout must be non-negative")
	}
	if !execUseSandbox {
		cfg := bootstrap.NewRuntimeExecConfig(getDataDir(), nil)
		if timeout == 0 {
			timeout = cfg.DefaultTimeout
		}
		if cfg.MaxTimeout > 0 && timeout > cfg.MaxTimeout {
			timeout = cfg.MaxTimeout
		}
	}

	return execCLIRequest{
		Command:     command,
		Workdir:     execWorkdir,
		Timeout:     timeout,
		Env:         envMap,
		Stdin:       execStdin,
		UseSandbox:  execUseSandbox,
		SandboxTier: tier,
	}, nil
}

func parseExecCLIEnv(values []string) (map[string]string, error) {
	env := make(map[string]string, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		idx := strings.IndexByte(raw, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("invalid --env value %q: expected KEY=VALUE", raw)
		}
		key := strings.TrimSpace(raw[:idx])
		if key == "" {
			return nil, fmt.Errorf("invalid --env value %q: expected KEY=VALUE", raw)
		}
		env[key] = raw[idx+1:]
	}
	return env, nil
}

func parseExecCLISandboxTier(raw string) (sandbox.Tier, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(sandbox.TierLight):
		return sandbox.TierLight, nil
	case string(sandbox.TierStrong):
		return sandbox.TierStrong, nil
	default:
		return "", fmt.Errorf("invalid --sandbox-tier %q: expected light or strong", raw)
	}
}

func buildExecCLICommandFromPositional(args []string) string {
	if len(args) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteExecCLIPositionalArg(arg))
	}
	return strings.Join(quoted, " ")
}

func quoteExecCLIPositionalArg(arg string) string {
	if arg == "" {
		if runtime.GOOS == "windows" {
			return `""`
		}
		return "''"
	}
	if isSimpleExecCLIToken(arg) {
		return arg
	}
	if runtime.GOOS == "windows" {
		return strconv.Quote(arg)
	}
	return "'" + strings.ReplaceAll(arg, "'", `'"'"'`) + "'"
}

func isSimpleExecCLIToken(arg string) bool {
	for _, r := range arg {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("._/:=-+", r):
		default:
			return false
		}
	}
	return true
}

func runExecCLIOnHost(ctx context.Context, req execCLIRequest) (*execCLIResult, error) {
	if err := tools.ValidateHostEnv(req.Env); err != nil {
		return nil, err
	}

	workdir, warnings := tools.ResolveWorkdir(req.Workdir)
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	shell, shellArgs := tools.GetShellConfig()
	args := append(append([]string{}, shellArgs...), req.Command)
	cmd := osexec.CommandContext(ctx, shell, args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), flattenExecCLIEnv(req.Env)...)
	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("execution timed out after %s", req.Timeout)
	}

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		var exitErr *osexec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, err
		}
	}

	return &execCLIResult{
		Stdout:   tools.SanitizeBinaryOutput(stdoutBuf.String()),
		Stderr:   tools.SanitizeBinaryOutput(stderrBuf.String()),
		ExitCode: exitCode,
		Sandbox:  false,
		Warnings: warnings,
	}, nil
}

func runExecCLIInSandbox(ctx context.Context, req execCLIRequest) (*execCLIResult, error) {
	manager, err := execCLISandboxManagerLoader()
	if err != nil {
		return nil, err
	}
	if manager == nil {
		return nil, fmt.Errorf("sandbox is disabled by configuration")
	}

	workdir, warnings := tools.ResolveWorkdir(req.Workdir)
	shell, shellArgs := tools.GetShellConfig()
	execReq := sandbox.NewExecutionRequest(shell, append(shellArgs, req.Command)...)
	execReq.Tier = req.SandboxTier
	execReq.WorkDir = workdir
	execReq.Timeout = req.Timeout
	execReq.Env = req.Env
	execReq.Stdin = req.Stdin

	result, err := manager.Execute(ctx, execReq)
	if err != nil {
		return nil, err
	}

	return &execCLIResult{
		Stdout:      result.Stdout,
		Stderr:      result.Stderr,
		ExitCode:    result.ExitCode,
		Sandbox:     true,
		SandboxTier: string(req.SandboxTier),
		Warnings:    warnings,
	}, nil
}

func loadExecCLISandboxManager() (*sandbox.Manager, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return nil, err
	}
	return bootstrap.NewSandboxManagerFromConfig(cfg)
}

func flattenExecCLIEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}
