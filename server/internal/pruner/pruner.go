package pruner

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Backend defines the inference backend interface for context pruning.
type Backend interface {
	Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error)
	Health(ctx context.Context) error
	Close() error
}

// PruneRequest is the input to the pruning engine.
type PruneRequest struct {
	Code        string      `json:"code"`                   // Backward compat alias
	Content     string      `json:"content"`                // Preferred: raw content (code or text)
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
	Enabled       bool    `mapstructure:"enabled"`
	Backend       string  `mapstructure:"backend"`        // "local", "bm25", "ir" (default), "code", "remote", "onnx"
	RemoteURL     string  `mapstructure:"remote_url"`     // only used when backend=remote/code
	ModelDir      string  `mapstructure:"model_dir"`      // directory for ONNX model files (onnx backend)
	Threshold     float64 `mapstructure:"threshold"`
	MinLines      int     `mapstructure:"min_lines"`
	TimeoutMs     int     `mapstructure:"timeout_ms"`
	CacheCapacity int     `mapstructure:"cache_capacity"` // LRU cache slots (default 256)
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
		return NewRemoteBackend(cfg.RemoteURL, &http.Client{
			Timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond,
		}), nil
	case "onnx":
		return NewOnnxBackend(cfg, cfg.ModelDir)
	default:
		return nil, fmt.Errorf("unknown pruner backend: %s", cfg.Backend)
	}
}
