package smallmodel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotReady    = errors.New("small model runtime not ready")
	ErrCircuitOpen = errors.New("small model circuit breaker open")
)

const (
	defaultLlamaCLIPath = "llama-cli"
	defaultTimeout      = 30 * time.Second
	defaultMaxParallel  = 2
	defaultMaxTokens    = 128
	defaultTemperature  = 0.2
)

// GenerateRequest contains normalized inference input for small-model tasks.
type GenerateRequest struct {
	Prompt      string
	MaxTokens   int
	Temperature float64
}

// GenerateResponse carries generated text and optional debug metadata.
type GenerateResponse struct {
	Text     string
	Fallback string
}

// Runtime is intentionally narrow so routing/orchestration can swap engines.
type Runtime interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	Ready() bool
}

type commandRunner interface {
	Run(ctx context.Context, binary string, args []string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, binary string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			return "", fmt.Errorf("%w: %s", err, s)
		}
		return "", err
	}
	return string(out), nil
}

type NativeRuntimeOptions struct {
	BinaryPath  string
	Timeout     time.Duration
	MaxParallel int
	Runner      commandRunner
}

// NativeRuntime executes in-process native llama.cpp CLI calls (llama-cli).
type NativeRuntime struct {
	manager     *Manager
	binaryPath  string
	timeout     time.Duration
	runner      commandRunner
	parallelSem chan struct{}
}

func NewNativeRuntime(manager *Manager, opts ...NativeRuntimeOptions) *NativeRuntime {
	var opt NativeRuntimeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	bin := strings.TrimSpace(opt.BinaryPath)
	if bin == "" {
		bin = strings.TrimSpace(os.Getenv("LLAMA_CPP_CLI_PATH"))
	}
	if bin == "" {
		bin = defaultLlamaCLIPath
	}
	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxParallel := opt.MaxParallel
	if maxParallel <= 0 {
		maxParallel = defaultMaxParallel
	}
	r := opt.Runner
	if r == nil {
		r = execRunner{}
	}
	return &NativeRuntime{
		manager:     manager,
		binaryPath:  bin,
		timeout:     timeout,
		runner:      r,
		parallelSem: make(chan struct{}, maxParallel),
	}
}

func (r *NativeRuntime) Ready() bool {
	if r == nil || r.manager == nil || !r.manager.IsReady() {
		return false
	}
	_, err := exec.LookPath(r.binaryPath)
	return err == nil
}

func (r *NativeRuntime) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if !r.Ready() {
		return nil, ErrNotReady
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("empty prompt")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	if maxTokens > 1024 {
		maxTokens = 1024
	}
	temp := req.Temperature
	if temp <= 0 {
		temp = defaultTemperature
	}
	if temp > 2 {
		temp = 2
	}

	runCtx := ctx
	cancel := func() {}
	if _, ok := runCtx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(runCtx, r.timeout)
	}
	defer cancel()

	select {
	case r.parallelSem <- struct{}{}:
		defer func() { <-r.parallelSem }()
	case <-runCtx.Done():
		return nil, runCtx.Err()
	}

	args := []string{
		"-m", r.manager.ModelPath(),
		"-p", prompt,
		"-n", strconv.Itoa(maxTokens),
		"--temp", fmt.Sprintf("%.2f", temp),
		"--simple-io",
		"--no-display-prompt",
	}

	out, err := r.runner.Run(runCtx, r.binaryPath, args)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(out)
	if text == "" {
		return nil, fmt.Errorf("empty output from llama runtime")
	}
	return &GenerateResponse{Text: text}, nil
}
