//go:build !onnx

package pruner

import (
	"fmt"
)

// NewOnnxBackend is a stub when built without the onnx tag.
func NewOnnxBackend(cfg Config, modelDir string) (Backend, error) {
	return nil, fmt.Errorf("onnx backend not available: build with -tags onnx")
}
