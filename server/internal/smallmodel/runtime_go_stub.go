//go:build !linux || !cgo

package smallmodel

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// GoRuntimeOptions controls pure-Go ONNX runtime behavior.
type GoRuntimeOptions struct {
	Timeout     time.Duration
	MaxParallel int
}

// GoRuntime is a compatibility stub outside linux+cgo builds.
type GoRuntime struct {
	reason string
	detail string
}

func NewGoRuntime(_ *Manager, _ ...GoRuntimeOptions) *GoRuntime {
	return &GoRuntime{
		reason: "onnx_runtime_unsupported_platform",
		detail: fmt.Sprintf(
			"go onnx runtime is only supported on linux with cgo; current platform %s/%s",
			runtime.GOOS,
			runtime.GOARCH,
		),
	}
}

func (r *GoRuntime) Ready() bool {
	_ = r
	return false
}

func (r *GoRuntime) ReadinessReason() string {
	if r == nil {
		return "runtime_nil"
	}
	return r.reason
}

func (r *GoRuntime) ReadinessDetail() string {
	if r == nil {
		return "small model runtime is nil"
	}
	return r.detail
}

func (r *GoRuntime) Generate(_ context.Context, _ GenerateRequest) (*GenerateResponse, error) {
	return nil, ErrNotReady
}
