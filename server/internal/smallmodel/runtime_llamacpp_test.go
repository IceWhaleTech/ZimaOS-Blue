package smallmodel

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLlamaCppRuntimeReadinessReasonRuntimeNil(t *testing.T) {
	var rt *LlamaCppRuntime
	if got := rt.ReadinessReason(); got != "runtime_nil" {
		t.Fatalf("ReadinessReason() = %q, want runtime_nil", got)
	}
	if got := rt.ReadinessDetail(); !strings.Contains(got, "runtime") {
		t.Fatalf("ReadinessDetail() = %q, want runtime-related detail", got)
	}
}

func TestLlamaCppRuntimeReadinessReasonManagerNil(t *testing.T) {
	rt := NewLlamaCppRuntime(nil)
	if got := rt.ReadinessReason(); got != "manager_nil" {
		t.Fatalf("ReadinessReason() = %q, want manager_nil", got)
	}
}

func TestLlamaCppRuntimeReadinessReasonModelFilesMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewLlamaCppRuntime(m)
	if got := rt.ReadinessReason(); got != "model_files_missing_or_incomplete" {
		t.Fatalf("ReadinessReason() = %q, want model_files_missing_or_incomplete", got)
	}
	if rt.Ready() {
		t.Fatal("Ready() = true, want false")
	}
}

func TestLlamaCppRuntimeReadinessReasonCGOModeUnavailable(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)
	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{Mode: "cgo"})
	if got := rt.ReadinessReason(); got != "llama_cpp_cgo_unavailable" {
		t.Fatalf("ReadinessReason() = %q, want llama_cpp_cgo_unavailable", got)
	}
}

func TestLlamaCppRuntimeReadinessReasonFFIModeUnavailable(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)
	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{Mode: "ffi"})
	if got := rt.ReadinessReason(); got != "llama_cpp_ffi_unavailable" {
		t.Fatalf("ReadinessReason() = %q, want llama_cpp_ffi_unavailable", got)
	}
}

func TestLlamaCppRuntimeGenerateNotReadyWhenModelMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewLlamaCppRuntime(m)

	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: "hello"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}
