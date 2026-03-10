package smallmodel

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotReady                 = errors.New("small model runtime not ready")
	ErrCircuitOpen              = errors.New("small model circuit breaker open")
	ErrPrefixCachingUnsupported = errors.New("small model prefix caching not supported")
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

// PrefillRequest describes a cacheable prompt prefix that should be pre-computed.
type PrefillRequest struct {
	CacheKey string
	Prefix   string
	Images   []ImageInput
	TTL      time.Duration
	Priority int
}

// PrefillResult describes a cached prefix entry returned by Prefill.
type PrefillResult struct {
	PrefixID     string
	PrefixTokens int
	KVBytes      int64
	CacheHit     bool
	Tier         string
	ExpiresAt    time.Time
}

// GenerateFromPrefixRequest continues generation from a previously prefetched prefix.
type GenerateFromPrefixRequest struct {
	PrefixID     string
	Suffix       string
	MaxTokens    int
	Temperature  float64
	Images       []ImageInput
	CacheKeyHint string
}

// PrefixCacheStats reports runtime prefix-cache capability and usage.
type PrefixCacheStats struct {
	Supported   bool
	Entries     int
	Hits        uint64
	Misses      uint64
	Evictions   uint64
	MemoryBytes int64
	DiskBytes   int64
}

// BatchingStats reports request-window alignment used to improve backend-side
// continuous batching opportunities.
type BatchingStats struct {
	Supported    bool
	Window       time.Duration
	MaxBatchSize int
	Pending      int64
	Submitted    uint64
	Executed     uint64
	BatchCount   uint64
	LargestBatch int
}

// Runtime is intentionally narrow so routing/orchestration can swap engines.
type Runtime interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	Ready() bool
}

// PrefixCachingRuntime is an optional extension for runtimes that can reuse
// prefilled prompt prefixes and expose cache stats.
type PrefixCachingRuntime interface {
	Runtime
	Prefill(ctx context.Context, req PrefillRequest) (*PrefillResult, error)
	GenerateFromPrefix(ctx context.Context, req GenerateFromPrefixRequest) (*GenerateResponse, error)
	EvictPrefix(prefixID string) bool
	PrefixCacheStats() PrefixCacheStats
}

// BatchingRuntime is an optional extension for runtimes that expose request
// alignment / scheduling statistics.
type BatchingRuntime interface {
	Runtime
	BatchingStats() BatchingStats
}
