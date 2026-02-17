// Package cache provides multi-level caching with LRU and LFU eviction policies.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Common errors
var (
	ErrKeyNotFound = errors.New("key not found")
	ErrCacheFull   = errors.New("cache is full")
	ErrKeyExpired  = errors.New("key has expired")
)

// Cache is the interface for cache implementations.
type Cache interface {
	// Get retrieves a value from the cache.
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a value in the cache with optional TTL.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key from the cache.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) bool

	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error

	// Stats returns cache statistics.
	Stats() CacheStats

	// Close closes the cache and releases resources.
	Close() error
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Sets       int64   `json:"sets"`
	Deletes    int64   `json:"deletes"`
	Evictions  int64   `json:"evictions"`
	Size       int64   `json:"size"`
	Capacity   int64   `json:"capacity"`
	HitRate    float64 `json:"hit_rate"`
	MemoryUsed int64   `json:"memory_used,omitempty"`
}

// Entry represents a cache entry.
type Entry struct {
	Key         string
	Value       interface{}
	Size        int64
	CreatedAt   int64 // unix nanos
	ExpiresAt   int64 // unix nanos, 0 = no expiration
	AccessedAt  int64 // unix nanos
	AccessCount int64
}

// IsExpired checks if the entry has expired.
func (e *Entry) IsExpired() bool {
	if e.ExpiresAt == 0 {
		return false
	}
	return timeutil.NowNano() > e.ExpiresAt
}

// TTL returns the remaining time to live.
func (e *Entry) TTL() time.Duration {
	if e.ExpiresAt == 0 {
		return -1 // No expiration
	}
	remaining := e.ExpiresAt - timeutil.NowNano()
	if remaining < 0 {
		return 0
	}
	return time.Duration(remaining)
}

// EvictionPolicy defines the cache eviction policy.
type EvictionPolicy string

const (
	// LRU evicts the least recently used entries.
	LRU EvictionPolicy = "lru"
	// LFU evicts the least frequently used entries.
	LFU EvictionPolicy = "lfu"
	// FIFO evicts the oldest entries first.
	FIFO EvictionPolicy = "fifo"
)

// Config holds cache configuration.
type Config struct {
	// MaxSize is the maximum number of entries.
	MaxSize int `yaml:"max_size"`
	// MaxMemory is the maximum memory usage in bytes (0 = unlimited).
	MaxMemory int64 `yaml:"max_memory"`
	// DefaultTTL is the default TTL for entries (0 = no expiration).
	DefaultTTL time.Duration `yaml:"default_ttl"`
	// EvictionPolicy is the eviction policy to use.
	EvictionPolicy EvictionPolicy `yaml:"eviction_policy"`
	// CleanupInterval is the interval for cleaning up expired entries.
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	// OnEvict is called when an entry is evicted.
	OnEvict func(key string, value interface{})
}

// DefaultConfig returns the default cache configuration.
func DefaultConfig() Config {
	return Config{
		MaxSize:         1000,
		MaxMemory:       0,
		DefaultTTL:      5 * time.Minute,
		EvictionPolicy:  LRU,
		CleanupInterval: time.Minute,
	}
}

// L1Config holds L1 (memory) cache configuration.
type L1Config struct {
	Config
	// ShardCount is the number of shards for concurrent access.
	ShardCount int `yaml:"shard_count"`
}

// DefaultL1Config returns the default L1 cache configuration.
func DefaultL1Config() L1Config {
	return L1Config{
		Config:     DefaultConfig(),
		ShardCount: 16,
	}
}

// L2Config holds L2 (disk) cache configuration.
type L2Config struct {
	Config
	// Path is the directory path for disk cache.
	Path string `yaml:"path"`
	// MaxDiskSize is the maximum disk usage in bytes.
	MaxDiskSize int64 `yaml:"max_disk_size"`
	// Compression enables compression for stored values.
	Compression bool `yaml:"compression"`
	// CompressionLevel is the compression level (1-9).
	CompressionLevel int `yaml:"compression_level"`
}

// DefaultL2Config returns the default L2 cache configuration.
func DefaultL2Config() L2Config {
	return L2Config{
		Config:           DefaultConfig(),
		Path:             "./cache",
		MaxDiskSize:      100 * 1024 * 1024, // 100MB
		Compression:      true,
		CompressionLevel: 6,
	}
}

// MultiLevelConfig holds multi-level cache configuration.
type MultiLevelConfig struct {
	L1 L1Config `yaml:"l1"`
	L2 L2Config `yaml:"l2"`
	// PromoteOnHit promotes entries from L2 to L1 on hit.
	PromoteOnHit bool `yaml:"promote_on_hit"`
	// WriteThrough writes to both L1 and L2 on set.
	WriteThrough bool `yaml:"write_through"`
}

// DefaultMultiLevelConfig returns the default multi-level cache configuration.
func DefaultMultiLevelConfig() MultiLevelConfig {
	return MultiLevelConfig{
		L1:           DefaultL1Config(),
		L2:           DefaultL2Config(),
		PromoteOnHit: true,
		WriteThrough: true,
	}
}
