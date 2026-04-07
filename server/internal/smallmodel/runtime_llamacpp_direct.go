package smallmodel

import (
	"context"
	"errors"
)

var errLlamaCppDirectUnsupported = errors.New("llama direct backend unsupported")

type llamaCppDirectGenerateRequest struct {
	ModelPath   string
	MMProjPath  string
	Prompt      string
	MaxTokens   int
	Temperature float64
	Images      []ImageInput
}

type llamaCppDirectBackend interface {
	EnsureLoaded() error
	Generate(ctx context.Context, req llamaCppDirectGenerateRequest) (string, error)
}
