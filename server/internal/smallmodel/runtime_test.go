package smallmodel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	output        string
	err           error
	bin           string
	args          []string
	capturedInput []byte
}

func (r *fakeRunner) Run(_ context.Context, binary string, args []string) (string, error) {
	r.bin = binary
	r.args = append([]string(nil), args...)
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--input" {
			data, _ := os.ReadFile(args[i+1])
			r.capturedInput = data
			break
		}
	}
	if r.err != nil {
		return "", r.err
	}
	return r.output, nil
}

func TestNativeRuntimeReadyRequiresModelAndBinary(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: "python3"})
	if rt.Ready() {
		t.Fatal("expected runtime not ready without model files")
	}
}

func TestNativeRuntimeGenerateBuildsONNXRequest(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	fakePy := filepath.Join(t.TempDir(), "python3")
	if err := os.WriteFile(fakePy, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake python: %v", err)
	}

	runner := &fakeRunner{output: "hello"}
	rt := NewNativeRuntime(m, NativeRuntimeOptions{
		BinaryPath: fakePy,
		Runner:     runner,
	})

	resp, err := rt.Generate(context.Background(), GenerateRequest{
		Prompt:      "hi",
		MaxTokens:   32,
		Temperature: 0.4,
		Images: []ImageInput{
			{MimeType: "image/png", Data: "aGVsbG8="},
		},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Text != "hello" {
		t.Fatalf("response text = %q, want hello", resp.Text)
	}
	if runner.bin != fakePy {
		t.Fatalf("runner binary = %q, want %q", runner.bin, fakePy)
	}
	if len(runner.args) != 3 || runner.args[1] != "--input" {
		t.Fatalf("unexpected args: %v", runner.args)
	}
	if !filepath.IsAbs(runner.args[0]) {
		t.Fatalf("helper script should use absolute path, got %q", runner.args[0])
	}

	var got onnxGenerateInput
	if err := json.Unmarshal(runner.capturedInput, &got); err != nil {
		t.Fatalf("parse request json: %v", err)
	}
	if got.ModelDir != m.ModelDir() {
		t.Fatalf("model_dir = %q, want %q", got.ModelDir, m.ModelDir())
	}
	if got.Prompt != "hi" {
		t.Fatalf("prompt = %q, want hi", got.Prompt)
	}
	if got.MaxTokens != 32 {
		t.Fatalf("max_tokens = %d, want 32", got.MaxTokens)
	}
	if got.Temperature != 0.4 {
		t.Fatalf("temperature = %.2f, want 0.40", got.Temperature)
	}
	if len(got.Images) != 1 {
		t.Fatalf("images = %d, want 1", len(got.Images))
	}
	if got.Images[0].MimeType != "image/png" || got.Images[0].Data != "aGVsbG8=" {
		t.Fatalf("unexpected image payload: %+v", got.Images[0])
	}
}

func TestNativeRuntimeGenerateNotReady(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: "python3"})
	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: "test"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}

func TestNativeRuntimeResolveDevice(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	fakePy := filepath.Join(t.TempDir(), "python3")
	if err := os.WriteFile(fakePy, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake python: %v", err)
	}
	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: fakePy, Runner: &fakeRunner{output: "ok"}})

	t.Setenv("SMALL_MODEL_DEVICE", "cpu")
	if got := rt.resolveDevice(); got != "cpu" {
		t.Fatalf("resolveDevice() = %q, want cpu", got)
	}

	t.Setenv("SMALL_MODEL_DEVICE", "gpu")
	if got := rt.resolveDevice(); got != "gpu" {
		t.Fatalf("resolveDevice() = %q, want gpu", got)
	}

	t.Setenv("SMALL_MODEL_DEVICE", "")
	t.Setenv("SMALL_MODEL_USE_GPU", "false")
	if got := rt.resolveDevice(); got != "cpu" {
		t.Fatalf("resolveDevice() from legacy false = %q, want cpu", got)
	}

	t.Setenv("SMALL_MODEL_USE_GPU", "true")
	if got := rt.resolveDevice(); got != "gpu" {
		t.Fatalf("resolveDevice() from legacy true = %q, want gpu", got)
	}
}

func TestNativeRuntimeReadinessReasonModelFilesMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: "python3"})
	if got := rt.ReadinessReason(); got != "model_files_missing_or_incomplete" {
		t.Fatalf("ReadinessReason() = %q, want model_files_missing_or_incomplete", got)
	}
}

func TestNativeRuntimeReadinessReasonPythonMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	rt := NewNativeRuntime(m, NativeRuntimeOptions{
		BinaryPath: filepath.Join(t.TempDir(), "missing-python3"),
	})
	if got := rt.ReadinessReason(); got != "python_not_found" {
		t.Fatalf("ReadinessReason() = %q, want python_not_found", got)
	}
}

func TestNativeRuntimeReadinessReasonPythonModuleMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	fakePy := filepath.Join(t.TempDir(), "python3")
	script := "#!/bin/sh\necho \"ModuleNotFoundError: No module named 'onnxruntime_genai'\" 1>&2\nexit 1\n"
	if err := os.WriteFile(fakePy, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake python: %v", err)
	}

	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: fakePy})
	if got := rt.ReadinessReason(); got != "python_module_missing_onnxruntime_genai" {
		t.Fatalf("ReadinessReason() = %q, want python_module_missing_onnxruntime_genai", got)
	}
	if got := rt.ReadinessDetail(); !strings.Contains(got, "ModuleNotFoundError") {
		t.Fatalf("ReadinessDetail() = %q, want ModuleNotFoundError", got)
	}
}

func createReadyModelFiles(t *testing.T, m *Manager) {
	t.Helper()
	for _, f := range requiredModelFiles() {
		p := filepath.Join(m.ModelDir(), f.Filename)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}
