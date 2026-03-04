//go:build !cgo

package pruner

import (
	"context"
	"errors"
)

// ErrOnnxNotReady is returned when ONNX runtime is not available in !cgo builds.
var ErrOnnxNotReady = errors.New("onnx pruner model not ready")

// OnnxBackend is a no-cgo compatibility stub.
type OnnxBackend struct {
	config   Config
	modelDir string
}

// NewOnnxBackend creates a stub backend in !cgo builds.
func NewOnnxBackend(cfg Config, modelDir string) (*OnnxBackend, error) {
	return &OnnxBackend{
		config:   cfg,
		modelDir: modelDir,
	}, nil
}

// StartAsync is a no-op in !cgo builds.
func (b *OnnxBackend) StartAsync() {
	_ = b
}

// IsReady always returns false in !cgo builds.
func (b *OnnxBackend) IsReady() bool {
	_ = b
	return false
}

// Prune always reports model not ready in !cgo builds.
func (b *OnnxBackend) Prune(_ context.Context, _ PruneRequest) (*PruneResponse, error) {
	return nil, ErrOnnxNotReady
}

// Health always reports model not ready in !cgo builds.
func (b *OnnxBackend) Health(_ context.Context) error {
	return ErrOnnxNotReady
}

// Close is a no-op in !cgo builds.
func (b *OnnxBackend) Close() error {
	_ = b
	return nil
}
