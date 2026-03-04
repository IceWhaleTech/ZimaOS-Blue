package smallmodel

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotReady    = errors.New("small model runtime not ready")
	ErrCircuitOpen = errors.New("small model circuit breaker open")
)

const (
	defaultPythonPath  = "python3"
	defaultTimeout     = 30 * time.Second
	defaultMaxParallel = 2
	defaultMaxTokens   = 128
	defaultTemperature = 0.2
)

//go:embed onnx_generate.py
var onnxGenerateScript string

// GenerateRequest contains normalized inference input for small-model tasks.
type GenerateRequest struct {
	Prompt      string
	MaxTokens   int
	Temperature float64
	Images      []ImageInput
}

// ImageInput carries a user-supplied image for multimodal generation.
type ImageInput struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64 payload
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

// NativeRuntime executes small-model inference via Python onnxruntime-genai.
type NativeRuntime struct {
	manager          *Manager
	binaryPath       string
	timeout          time.Duration
	runner           commandRunner
	parallelSem      chan struct{}
	helperScriptPath string
	probeMu          sync.Mutex
	probeAt          time.Time
	probeOK          bool
	probeMsg         string
}

type onnxGenerateInput struct {
	ModelDir    string       `json:"model_dir"`
	Prompt      string       `json:"prompt"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature"`
	Device      string       `json:"device"` // cpu|gpu
	Images      []ImageInput `json:"images,omitempty"`
}

func NewNativeRuntime(manager *Manager, opts ...NativeRuntimeOptions) *NativeRuntime {
	var opt NativeRuntimeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	bin := resolvePythonBinaryPath(strings.TrimSpace(opt.BinaryPath))
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
	helperPath := ""
	if manager != nil {
		helperPath = filepath.Join(manager.ModelDir(), ".onnx_generate.py")
	}
	return &NativeRuntime{
		manager:          manager,
		binaryPath:       bin,
		timeout:          timeout,
		runner:           r,
		parallelSem:      make(chan struct{}, maxParallel),
		helperScriptPath: helperPath,
	}
}

func (r *NativeRuntime) Ready() bool {
	reason, _ := r.readinessState()
	return reason == "ready"
}

// ReadinessReason returns a stable reason code for current runtime readiness.
func (r *NativeRuntime) ReadinessReason() string {
	reason, _ := r.readinessState()
	return reason
}

// ReadinessDetail returns human-readable context for current readiness state.
func (r *NativeRuntime) ReadinessDetail() string {
	_, detail := r.readinessState()
	return detail
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

	reqFile, err := os.CreateTemp(r.manager.ModelDir(), "onnx-generate-*.json")
	if err != nil {
		return nil, fmt.Errorf("create onnx input file: %w", err)
	}
	reqPath := reqFile.Name()
	defer func() {
		_ = os.Remove(reqPath)
	}()
	payload := onnxGenerateInput{
		ModelDir:    r.manager.ModelDir(),
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: temp,
		Device:      r.resolveDevice(),
		Images:      req.Images,
	}
	if err := json.NewEncoder(reqFile).Encode(payload); err != nil {
		_ = reqFile.Close()
		return nil, fmt.Errorf("write onnx input file: %w", err)
	}
	if err := reqFile.Close(); err != nil {
		return nil, fmt.Errorf("close onnx input file: %w", err)
	}

	args := []string{
		r.helperScriptPath,
		"--input",
		reqPath,
	}
	out, err := r.runner.Run(runCtx, r.binaryPath, args)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(out)
	if text == "" {
		return nil, fmt.Errorf("empty output from onnx runtime")
	}
	return &GenerateResponse{Text: text}, nil
}

func (r *NativeRuntime) ensureHelperScript() error {
	if r == nil {
		return fmt.Errorf("nil runtime")
	}
	if strings.TrimSpace(r.helperScriptPath) == "" {
		return fmt.Errorf("empty helper script path")
	}
	if err := os.MkdirAll(filepath.Dir(r.helperScriptPath), 0o755); err != nil {
		return fmt.Errorf("create helper script dir: %w", err)
	}
	existing, _ := os.ReadFile(r.helperScriptPath)
	if string(existing) == onnxGenerateScript {
		// Keep script executable.
		_ = os.Chmod(r.helperScriptPath, 0o755)
		return nil
	}
	if err := os.WriteFile(r.helperScriptPath, []byte(onnxGenerateScript), 0o755); err != nil {
		return fmt.Errorf("write helper script: %w", err)
	}
	return nil
}

func (r *NativeRuntime) pythonModulesReady() bool {
	ok, _ := r.pythonModulesReadyDetailed()
	return ok
}

func (r *NativeRuntime) pythonModulesReadyDetailed() (bool, string) {
	if r == nil {
		return false, "runtime is nil"
	}
	r.probeMu.Lock()
	defer r.probeMu.Unlock()

	// Keep probe cheap under frequent readiness checks.
	if !r.probeAt.IsZero() && time.Since(r.probeAt) < 30*time.Second {
		return r.probeOK, r.probeMsg
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.binaryPath, "-c", "import onnxruntime_genai")
	out, err := cmd.CombinedOutput()
	r.probeAt = time.Now()
	r.probeOK = err == nil
	r.probeMsg = strings.TrimSpace(string(out))
	if err != nil && r.probeMsg == "" {
		r.probeMsg = err.Error()
	}
	return r.probeOK, r.probeMsg
}

func resolvePythonBinaryPath(optPath string) string {
	if optPath != "" {
		return optPath
	}
	if envPath := strings.TrimSpace(os.Getenv("SMALL_MODEL_PYTHON_PATH")); envPath != "" {
		return envPath
	}
	if envPath := strings.TrimSpace(os.Getenv("PYTHON")); envPath != "" {
		return envPath
	}
	return defaultPythonPath
}

func (r *NativeRuntime) resolveDevice() string {
	if v := parseSmallModelDevice(strings.TrimSpace(os.Getenv("SMALL_MODEL_DEVICE"))); v != "" {
		return v
	}
	// Backward-compatible toggle.
	if b, set := parseLegacySmallModelUseGPU(strings.TrimSpace(os.Getenv("SMALL_MODEL_USE_GPU"))); set {
		if b {
			return "gpu"
		}
		return "cpu"
	}
	// macOS default: prefer GPU path.
	if runtime.GOOS == "darwin" {
		return "gpu"
	}
	return "cpu"
}

func parseSmallModelDevice(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "gpu", "metal", "cuda", "coreml":
		return "gpu"
	case "cpu":
		return "cpu"
	default:
		return ""
	}
}

func parseLegacySmallModelUseGPU(raw string) (bool, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false, false
	}
	switch raw {
	case "1", "true", "yes", "on", "gpu", "metal":
		return true, true
	case "0", "false", "no", "off", "cpu":
		return false, true
	default:
		if b, err := strconv.ParseBool(raw); err == nil {
			return b, true
		}
		return false, false
	}
}

func (r *NativeRuntime) readinessState() (string, string) {
	if r == nil {
		return "runtime_nil", "small model runtime is nil"
	}
	if r.manager == nil {
		return "manager_nil", "small model manager is nil"
	}
	st := r.manager.GetStatus()
	if !st.Ready {
		if st.Downloading {
			return "model_downloading", "small model files are downloading"
		}
		if strings.TrimSpace(st.Error) != "" {
			return "model_unready", st.Error
		}
		return "model_files_missing_or_incomplete", "required small model files are missing"
	}
	if _, err := exec.LookPath(r.binaryPath); err != nil {
		return "python_not_found", err.Error()
	}
	if err := r.ensureHelperScript(); err != nil {
		return "helper_script_unavailable", err.Error()
	}
	if ok, detail := r.pythonModulesReadyDetailed(); !ok {
		if strings.TrimSpace(detail) == "" {
			detail = "onnxruntime_genai import failed"
		}
		return "python_module_missing_onnxruntime_genai", detail
	}
	return "ready", ""
}
