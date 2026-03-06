package smallmodel

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	llamaCppDefaultTemperature = 0.2
)

type LlamaCppMode string

const (
	LlamaCppModeAuto   LlamaCppMode = "auto"
	LlamaCppModeServer LlamaCppMode = "server"
	LlamaCppModeCGO    LlamaCppMode = "cgo"
	LlamaCppModeFFI    LlamaCppMode = "ffi"
)

// LlamaCppRuntimeOptions controls llama.cpp runtime behavior.
type LlamaCppRuntimeOptions struct {
	Timeout     time.Duration
	MaxParallel int
	CLIPath     string
	Mode        string
}

// LlamaCppRuntime executes fixed GGUF+mmproj inference via llama.cpp CLI.
type LlamaCppRuntime struct {
	manager *Manager
	timeout time.Duration
	mode    LlamaCppMode

	parallelSem chan struct{}

	configuredCLI string
	resolveMu     sync.RWMutex
	resolvedCLI   string
}

func NewLlamaCppRuntime(manager *Manager, opts ...LlamaCppRuntimeOptions) *LlamaCppRuntime {
	var opt LlamaCppRuntimeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxParallel := opt.MaxParallel
	if maxParallel <= 0 {
		maxParallel = defaultMaxParallel
	}
	cliPath := strings.TrimSpace(opt.CLIPath)
	if cliPath == "" {
		cliPath = strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_CPP_CLI"))
	}
	mode := normalizeLlamaCppMode(opt.Mode)
	if mode == LlamaCppModeAuto {
		mode = normalizeLlamaCppMode(os.Getenv("SMALL_MODEL_LLAMA_MODE"))
	}
	if mode == "" {
		mode = LlamaCppModeAuto
	}

	return &LlamaCppRuntime{
		manager:       manager,
		timeout:       timeout,
		mode:          mode,
		parallelSem:   make(chan struct{}, maxParallel),
		configuredCLI: cliPath,
	}
}

func (r *LlamaCppRuntime) Ready() bool {
	reason, _ := r.readinessState()
	return reason == "ready"
}

func (r *LlamaCppRuntime) ReadinessReason() string {
	reason, _ := r.readinessState()
	return reason
}

func (r *LlamaCppRuntime) ReadinessDetail() string {
	_, detail := r.readinessState()
	return detail
}

func (r *LlamaCppRuntime) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	reason, _ := r.readinessState()
	if reason != "ready" {
		return nil, ErrNotReady
	}
	if r.resolveBackend() != LlamaCppModeServer {
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

	temperature := req.Temperature
	if temperature <= 0 {
		temperature = llamaCppDefaultTemperature
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

	cliPath, err := r.resolveCLIPath()
	if err != nil {
		return nil, ErrNotReady
	}

	imagePaths, cleanup, err := prepareLlamaImageFiles(req.Images)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	args := []string{
		"-m", r.manager.ModelPath(),
		"--mmproj", r.manager.MMProjPath(),
		"-n", strconv.Itoa(maxTokens),
		"--temp", strconv.FormatFloat(temperature, 'f', 3, 64),
		"--simple-io",
		"--no-display-prompt",
		"-p", prompt,
	}
	for _, imgPath := range imagePaths {
		args = append(args, "--image", imgPath)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, cliPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := compactSingleLine(stderr.String())
		if detail == "" {
			detail = compactSingleLine(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("llama.cpp generation failed: %s", truncateText(detail, 512))
	}

	text := sanitizeLlamaOutput(prompt, stdout.String())
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty output from llama.cpp runtime")
	}
	return &GenerateResponse{Text: strings.TrimSpace(text)}, nil
}

func (r *LlamaCppRuntime) readinessState() (string, string) {
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
	if _, err := os.Stat(r.manager.ModelPath()); err != nil {
		return "model_file_missing", err.Error()
	}
	if _, err := os.Stat(r.manager.MMProjPath()); err != nil {
		return "mmproj_file_missing", err.Error()
	}
	switch r.resolveBackend() {
	case LlamaCppModeCGO:
		return "llama_cpp_cgo_unavailable", "llama.cpp cgo backend is not linked in this build"
	case LlamaCppModeFFI:
		return "llama_cpp_ffi_unavailable", "llama.cpp ffi backend is not linked in this build"
	}
	if _, err := r.resolveCLIPath(); err != nil {
		return "llama_cpp_cli_not_found", err.Error()
	}
	return "ready", ""
}

func (r *LlamaCppRuntime) resolveCLIPath() (string, error) {
	r.resolveMu.RLock()
	if r.resolvedCLI != "" {
		defer r.resolveMu.RUnlock()
		return r.resolvedCLI, nil
	}
	r.resolveMu.RUnlock()

	r.resolveMu.Lock()
	defer r.resolveMu.Unlock()
	if r.resolvedCLI != "" {
		return r.resolvedCLI, nil
	}

	candidates := make([]string, 0, 4)
	if cli := strings.TrimSpace(r.configuredCLI); cli != "" {
		candidates = append(candidates, cli)
	}
	candidates = append(candidates, "llama-cli", "llama.cpp-cli", "main")

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if p, err := exec.LookPath(candidate); err == nil {
			r.resolvedCLI = p
			return p, nil
		}
	}
	return "", fmt.Errorf("llama.cpp CLI not found in PATH (expected `llama-cli`; optional env SMALL_MODEL_LLAMA_CPP_CLI)")
}

func normalizeLlamaCppMode(raw string) LlamaCppMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "auto":
		return LlamaCppModeAuto
	case "server":
		return LlamaCppModeServer
	case "cgo":
		return LlamaCppModeCGO
	case "ffi":
		return LlamaCppModeFFI
	default:
		return LlamaCppModeAuto
	}
}

func (r *LlamaCppRuntime) resolveBackend() LlamaCppMode {
	switch r.mode {
	case LlamaCppModeCGO:
		return LlamaCppModeCGO
	case LlamaCppModeFFI:
		return LlamaCppModeFFI
	case LlamaCppModeServer:
		return LlamaCppModeServer
	default:
		// Auto mode: reserve CGO/FFI priority for future linked backends.
		return LlamaCppModeServer
	}
}

func prepareLlamaImageFiles(images []ImageInput) ([]string, func(), error) {
	if len(images) == 0 {
		return nil, func() {}, nil
	}

	dir, err := os.MkdirTemp("", "smallmodel-llama-images-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir for images: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	paths := make([]string, 0, len(images))
	for i, img := range images {
		payload, decodeErr := decodeBase64PayloadLlama(img.Data)
		if decodeErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("decode image payload: %w", decodeErr)
		}
		ext := imageExtForMime(img.MimeType)
		path := filepath.Join(dir, fmt.Sprintf("image-%02d%s", i, ext))
		if writeErr := os.WriteFile(path, payload, 0o600); writeErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("write temp image: %w", writeErr)
		}
		paths = append(paths, path)
	}
	return paths, cleanup, nil
}

func decodeBase64PayloadLlama(raw string) ([]byte, error) {
	payload := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(payload), "data:") {
		if idx := strings.Index(payload, ","); idx >= 0 {
			payload = payload[idx+1:]
		}
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, fmt.Errorf("empty image payload")
	}
	if b, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return b, nil
	}
	b, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func imageExtForMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func sanitizeLlamaOutput(prompt, raw string) string {
	out := strings.TrimSpace(raw)
	if out == "" {
		return ""
	}
	trimPrompt := strings.TrimSpace(prompt)
	if trimPrompt != "" && strings.HasPrefix(out, trimPrompt) {
		out = strings.TrimSpace(strings.TrimPrefix(out, trimPrompt))
	}
	if idx := strings.Index(strings.ToLower(out), "\nassistant:"); idx >= 0 {
		out = strings.TrimSpace(out[idx+len("\nassistant:"):])
	}
	if strings.HasPrefix(strings.ToLower(out), "assistant:") {
		out = strings.TrimSpace(out[len("assistant:"):])
	}
	return out
}

func compactSingleLine(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func truncateText(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
