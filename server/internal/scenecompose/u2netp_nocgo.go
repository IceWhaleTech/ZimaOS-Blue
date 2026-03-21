//go:build !cgo

package scenecompose

import (
	"context"
	"fmt"
	"image"
)

func (m *U2NetPModelManager) inferMask(_ context.Context, _ image.Image) (*CutoutResult, error) {
	return nil, fmt.Errorf("u2netp requires cgo-enabled onnx runtime")
}
