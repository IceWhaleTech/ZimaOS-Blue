package smallmodel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	output string
	err    error
	bin    string
	args   []string
}

func (r *fakeRunner) Run(_ context.Context, binary string, args []string) (string, error) {
	r.bin = binary
	r.args = append([]string(nil), args...)
	if r.err != nil {
		return "", r.err
	}
	return r.output, nil
}

func TestNativeRuntimeReadyRequiresModelAndBinary(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: "echo"})
	if rt.Ready() {
		t.Fatal("expected runtime not ready without model file")
	}
}

func TestNativeRuntimeGenerateBuildsLlamaCLIArgs(t *testing.T) {
	m := NewManager(t.TempDir())
	modelPath := m.ModelPath()
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}

	runner := &fakeRunner{output: "hello"}
	rt := NewNativeRuntime(m, NativeRuntimeOptions{
		BinaryPath:  "echo",
		Timeout:     time.Second,
		MaxParallel: 1,
		Runner:      runner,
	})

	resp, err := rt.Generate(context.Background(), GenerateRequest{
		Prompt:      "hi",
		MaxTokens:   32,
		Temperature: 0.4,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Text != "hello" {
		t.Fatalf("response text = %q, want hello", resp.Text)
	}
	argsJoined := strings.Join(runner.args, " ")
	if !strings.Contains(argsJoined, "-m "+modelPath) {
		t.Fatalf("missing model arg, args=%v", runner.args)
	}
	if !strings.Contains(argsJoined, "-n 32") {
		t.Fatalf("missing max token arg, args=%v", runner.args)
	}
	if !strings.Contains(argsJoined, "--temp 0.40") {
		t.Fatalf("missing temperature arg, args=%v", runner.args)
	}
}

func TestNativeRuntimeGenerateNotReady(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewNativeRuntime(m, NativeRuntimeOptions{BinaryPath: "echo"})
	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: "test"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}
