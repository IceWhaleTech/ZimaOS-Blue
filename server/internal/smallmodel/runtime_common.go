package smallmodel

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotReady    = errors.New("small model runtime not ready")
	ErrCircuitOpen = errors.New("small model circuit breaker open")
)

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxParallel = 2
	defaultMaxTokens   = 128
)

// GenerateRequest contains normalized inference input for small-model tasks.
type GenerateRequest struct {
	Prompt      string
	MaxTokens   int
	Temperature float64
	Images      []ImageInput
}

// ImageInput carries a user-supplied image for multimodal generation.
type ImageInput struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64 payload
}

// GenerateResponse carries generated text and optional debug metadata.
type GenerateResponse struct {
	Text     string
	Fallback string
}

// Runtime is intentionally narrow so routing/orchestration can swap engines.
type Runtime interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	Ready() bool
}
