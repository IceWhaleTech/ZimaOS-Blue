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

func TestGoRuntimeReadinessReasonRuntimeNil(t *testing.T) {
	var rt *GoRuntime
	if got := rt.ReadinessReason(); got != "runtime_nil" {
		t.Fatalf("ReadinessReason() = %q, want runtime_nil", got)
	}
	if got := rt.ReadinessDetail(); !strings.Contains(got, "runtime") {
		t.Fatalf("ReadinessDetail() = %q, want runtime-related detail", got)
	}
}

func TestGoRuntimeReadinessReasonManagerNil(t *testing.T) {
	rt := NewGoRuntime(nil)
	if got := rt.ReadinessReason(); got != "manager_nil" {
		t.Fatalf("ReadinessReason() = %q, want manager_nil", got)
	}
}

func TestGoRuntimeReadinessReasonModelFilesMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewGoRuntime(m)
	if got := rt.ReadinessReason(); got != "model_files_missing_or_incomplete" {
		t.Fatalf("ReadinessReason() = %q, want model_files_missing_or_incomplete", got)
	}
	if rt.Ready() {
		t.Fatal("Ready() = true, want false")
	}
}

func TestGoRuntimeReadinessReasonUsesCachedLoadFailure(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	rt := NewGoRuntime(m)
	rt.lastLoadErr = errors.New("tokenizer parse failed")
	rt.lastLoadCode = "tokenizer_load_failed"
	rt.lastLoadAt = time.Now()

	if got := rt.ReadinessReason(); got != "tokenizer_load_failed" {
		t.Fatalf("ReadinessReason() = %q, want tokenizer_load_failed", got)
	}
	if got := rt.ReadinessDetail(); !strings.Contains(got, "tokenizer parse failed") {
		t.Fatalf("ReadinessDetail() = %q, want tokenizer parse failed", got)
	}
}

func TestGoRuntimeGenerateNotReadyWhenModelMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewGoRuntime(m)

	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: "hello"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}

func TestGoRuntimeGenerateRejectsEmptyPrompt(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := &GoRuntime{
		manager:     m,
		timeout:     time.Second,
		parallelSem: make(chan struct{}, 1),
		engine:      &goEngine{},
	}

	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: " \n\t "})
	if err == nil || !strings.Contains(err.Error(), "empty prompt") {
		t.Fatalf("Generate() error = %v, want empty prompt", err)
	}
}

func createReadyModelFiles(t *testing.T, m *Manager) {
	t.Helper()
	for _, f := range requiredModelFiles(m.assets) {
		p := filepath.Join(m.ModelDir(), f.Filename)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}
