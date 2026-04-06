//go:build !linux

package smallmodel

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestGoRuntimeNonLinuxReportsUnsupportedPlatform(t *testing.T) {
	rt := NewGoRuntime(nil)

	if got := rt.ReadinessReason(); got != "onnx_runtime_unsupported_platform" {
		t.Fatalf("ReadinessReason() = %q, want onnx_runtime_unsupported_platform", got)
	}
	if got := rt.ReadinessDetail(); !strings.Contains(got, runtime.GOOS) {
		t.Fatalf("ReadinessDetail() = %q, want mention %s", got, runtime.GOOS)
	}
	if rt.Ready() {
		t.Fatal("Ready() = true, want false")
	}

	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: "hello"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("Generate() error = %v, want ErrNotReady", err)
	}
}
