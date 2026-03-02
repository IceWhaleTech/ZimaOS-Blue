package pruner

import (
	"context"
	"fmt"
)

// Backend defines the inference backend interface for context pruning.
type Backend interface {
	Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error)
	Health(ctx context.Context) error
	Close() error
}

// PruneRequest is the input to the pruning engine.
type PruneRequest struct {
	Code        string      `json:"code"`    // Backward compat alias
	Content     string      `json:"content"` // Preferred: raw content (code or text)
	Query       string      `json:"query,omitempty"`
	Threshold   float64     `json:"threshold,omitempty"`
	ContentType ContentType `json:"content_type,omitempty"` // Hint (auto-detected if 0)
}

// GetContent returns the content to prune, preferring Content over Code for backward compat.
func (r PruneRequest) GetContent() string {
	if r.Content != "" {
		return r.Content
	}
	return r.Code
}

// PruneResponse is the output from the pruning engine.
type PruneResponse struct {
	PrunedCode      string      `json:"pruned_code"`
	PrunedContent   string      `json:"pruned_content"`
	ContentType     ContentType `json:"content_type"`
	Score           float64     `json:"score"`
	OriginalLines   int         `json:"original_lines"`
	KeptLines       int         `json:"kept_lines"`
	PrunedLines     int         `json:"pruned_lines"`
	OriginalTokens  int         `json:"original_tokens"`
	PrunedTokens    int         `json:"pruned_tokens"`
	CompressionRate float64     `json:"compression_rate"`
	LatencyMs       float64     `json:"latency_ms"`
}

// Config holds pruner configuration.
type Config struct {
	Enabled       bool    `yaml:"enabled"`
	Backend       string  `yaml:"backend"`    // public: "local" only; internal tests may still use bm25/ir
	RemoteURL     string  `yaml:"remote_url"` // deprecated (SWE remote backend removed)
	ModelDir      string  `yaml:"model_dir"`  // deprecated (ONNX cross-encoder removed)
	Threshold     float64 `yaml:"threshold"`
	MinLines      int     `yaml:"min_lines"`
	TimeoutMs     int     `yaml:"timeout_ms"`
	CacheCapacity int     `yaml:"cache_capacity"` // LRU cache slots (default 256)
}

// DefaultConfig returns the default pruner configuration (disabled by default).
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		Backend:       "local",
		Threshold:     0.5,
		MinLines:      50,
		TimeoutMs:     5000,
		CacheCapacity: 256,
	}
}

func resolveThreshold(requestThreshold, fallback float64) float64 {
	threshold := requestThreshold
	if threshold <= 0 {
		threshold = fallback
	}
	if threshold < 0 {
		return 0
	}
	if threshold > 1 {
		return 1
	}
	return threshold
}

// NormalizeBackendName canonicalizes backend selection for public APIs.
// Pruner backend switching is hidden; only local backend is exposed.
func NormalizeBackendName(_ string) string {
	return "local"
}

// NewBackend creates a Backend based on the config.
func NewBackend(cfg Config) (Backend, error) {
	switch cfg.Backend {
	case "local", "":
		return NewLocalBackend(cfg), nil
	case "bm25":
		return NewBM25Backend(cfg), nil
	case "ir":
		return NewIRPruner(cfg), nil
	case "remote", "code":
		return nil, fmt.Errorf("remote SWE pruner backend has been removed; local backend only")
	case "onnx", "hybrid":
		return nil, fmt.Errorf("onnx cross-encoder backend has been removed; local backend only")
	default:
		return nil, fmt.Errorf("unknown pruner backend: %s", cfg.Backend)
	}
}
