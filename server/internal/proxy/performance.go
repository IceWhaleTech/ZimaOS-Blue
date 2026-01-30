package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

// PerformanceConfig holds performance optimization settings
type PerformanceConfig struct {
	// Buffer pool settings
	BufferPoolEnabled   bool `json:"buffer_pool_enabled"`
	BufferPoolSize      int  `json:"buffer_pool_size"`       // Number of buffers
	BufferInitialSize   int  `json:"buffer_initial_size"`    // Initial buffer size in bytes
	BufferMaxSize       int  `json:"buffer_max_size"`        // Max buffer size in bytes

	// Response cache settings
	CacheEnabled        bool          `json:"cache_enabled"`
	CacheMaxSize        int           `json:"cache_max_size"`        // Max cache entries
	CacheMaxEntrySize   int           `json:"cache_max_entry_size"`  // Max size per entry in bytes
	CacheTTL            time.Duration `json:"cache_ttl"`
	CacheableStatusCodes []int        `json:"cacheable_status_codes"`

	// Connection metrics
	MetricsEnabled      bool          `json:"metrics_enabled"`
	MetricsInterval     time.Duration `json:"metrics_interval"`
}

// DefaultPerformanceConfig returns default performance configuration
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		BufferPoolEnabled:   true,
		BufferPoolSize:      1000,
		BufferInitialSize:   4096,   // 4KB
		BufferMaxSize:       1 << 20, // 1MB

		CacheEnabled:        false, // Disabled by default for LLM responses
		CacheMaxSize:        1000,
		CacheMaxEntrySize:   1 << 20, // 1MB
		CacheTTL:            5 * time.Minute,
		CacheableStatusCodes: []int{200},

		MetricsEnabled:      true,
		MetricsInterval:     10 * time.Second,
	}
}

// BufferPool provides reusable byte buffers to reduce GC pressure
type BufferPool struct {
	pool        sync.Pool
	config      *PerformanceConfig

	// Stats
	gets        int64
	puts        int64
	news        int64
	oversized   int64
}

// NewBufferPool creates a new buffer pool
func NewBufferPool(config *PerformanceConfig) *BufferPool {
	if config == nil {
		config = DefaultPerformanceConfig()
	}

	bp := &BufferPool{
		config: config,
	}

	bp.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&bp.news, 1)
			return bytes.NewBuffer(make([]byte, 0, config.BufferInitialSize))
		},
	}

	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	atomic.AddInt64(&bp.gets, 1)
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// Don't return oversized buffers to the pool
	if buf.Cap() > bp.config.BufferMaxSize {
		atomic.AddInt64(&bp.oversized, 1)
		return
	}

	atomic.AddInt64(&bp.puts, 1)
	bp.pool.Put(buf)
}

// Stats returns buffer pool statistics
func (bp *BufferPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"gets":      atomic.LoadInt64(&bp.gets),
		"puts":      atomic.LoadInt64(&bp.puts),
		"news":      atomic.LoadInt64(&bp.news),
		"oversized": atomic.LoadInt64(&bp.oversized),
		"reuse_rate": bp.calculateReuseRate(),
	}
}

func (bp *BufferPool) calculateReuseRate() float64 {
	gets := atomic.LoadInt64(&bp.gets)
	news := atomic.LoadInt64(&bp.news)
	if gets == 0 {
		return 0
	}
	reused := gets - news
	if reused < 0 {
		reused = 0
	}
	return float64(reused) / float64(gets) * 100
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Key        string
	Value      []byte
	StatusCode int
	Headers    map[string]string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	HitCount   int64
}

// ResponseCache provides caching for API responses
type ResponseCache struct {
	config  *PerformanceConfig
	entries map[string]*CacheEntry
	mu      sync.RWMutex

	// Stats
	hits    int64
	misses  int64
	evictions int64
}

// NewResponseCache creates a new response cache
func NewResponseCache(config *PerformanceConfig) *ResponseCache {
	if config == nil {
		config = DefaultPerformanceConfig()
	}

	return &ResponseCache{
		config:  config,
		entries: make(map[string]*CacheEntry),
	}
}

// GenerateCacheKey generates a cache key from request parameters
func (rc *ResponseCache) GenerateCacheKey(provider, model, prompt string) string {
	h := sha256.New()
	h.Write([]byte(provider))
	h.Write([]byte(model))
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// Get retrieves a cached response
func (rc *ResponseCache) Get(key string) (*CacheEntry, bool) {
	if !rc.config.CacheEnabled {
		return nil, false
	}

	rc.mu.RLock()
	entry, ok := rc.entries[key]
	rc.mu.RUnlock()

	if !ok {
		atomic.AddInt64(&rc.misses, 1)
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		rc.mu.Lock()
		delete(rc.entries, key)
		rc.mu.Unlock()
		atomic.AddInt64(&rc.misses, 1)
		atomic.AddInt64(&rc.evictions, 1)
		return nil, false
	}

	atomic.AddInt64(&rc.hits, 1)
	atomic.AddInt64(&entry.HitCount, 1)
	return entry, true
}

// Set stores a response in the cache
func (rc *ResponseCache) Set(key string, value []byte, statusCode int, headers map[string]string) {
	if !rc.config.CacheEnabled {
		return
	}

	// Check if status code is cacheable
	cacheable := false
	for _, code := range rc.config.CacheableStatusCodes {
		if code == statusCode {
			cacheable = true
			break
		}
	}
	if !cacheable {
		return
	}

	// Check entry size
	if len(value) > rc.config.CacheMaxEntrySize {
		return
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Evict if at capacity
	if len(rc.entries) >= rc.config.CacheMaxSize {
		rc.evictOldest()
	}

	now := time.Now()
	rc.entries[key] = &CacheEntry{
		Key:        key,
		Value:      value,
		StatusCode: statusCode,
		Headers:    headers,
		CreatedAt:  now,
		ExpiresAt:  now.Add(rc.config.CacheTTL),
	}
}

// evictOldest removes the oldest entry (must be called with lock held)
func (rc *ResponseCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range rc.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(rc.entries, oldestKey)
		atomic.AddInt64(&rc.evictions, 1)
	}
}

// Clear clears all cache entries
func (rc *ResponseCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries = make(map[string]*CacheEntry)
}

// Stats returns cache statistics
func (rc *ResponseCache) Stats() map[string]interface{} {
	rc.mu.RLock()
	entryCount := len(rc.entries)
	rc.mu.RUnlock()

	hits := atomic.LoadInt64(&rc.hits)
	misses := atomic.LoadInt64(&rc.misses)
	total := hits + misses

	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"enabled":     rc.config.CacheEnabled,
		"entries":     entryCount,
		"max_entries": rc.config.CacheMaxSize,
		"hits":        hits,
		"misses":      misses,
		"evictions":   atomic.LoadInt64(&rc.evictions),
		"hit_rate":    hitRate,
	}
}

// ConnectionMetrics tracks connection pool performance
type ConnectionMetrics struct {
	config *PerformanceConfig
	mu     sync.RWMutex

	// Request metrics
	totalRequests     int64
	activeRequests    int64
	completedRequests int64
	failedRequests    int64

	// Connection metrics
	connectionsOpened int64
	connectionsClosed int64
	connectionsReused int64
	connectionErrors  int64

	// Latency tracking
	totalLatencyNs    int64
	minLatencyNs      int64
	maxLatencyNs      int64
	latencySamples    int64

	// Per-provider metrics
	providerMetrics map[string]*ProviderConnectionMetrics
}

// ProviderConnectionMetrics tracks per-provider connection stats
type ProviderConnectionMetrics struct {
	Requests      int64
	Successes     int64
	Failures      int64
	TotalLatency  int64
	Reused        int64
}

// NewConnectionMetrics creates a new connection metrics tracker
func NewConnectionMetrics(config *PerformanceConfig) *ConnectionMetrics {
	if config == nil {
		config = DefaultPerformanceConfig()
	}

	return &ConnectionMetrics{
		config:          config,
		providerMetrics: make(map[string]*ProviderConnectionMetrics),
	}
}

// RecordRequest records a request start
func (cm *ConnectionMetrics) RecordRequest(provider string) {
	atomic.AddInt64(&cm.totalRequests, 1)
	atomic.AddInt64(&cm.activeRequests, 1)

	cm.mu.Lock()
	if _, ok := cm.providerMetrics[provider]; !ok {
		cm.providerMetrics[provider] = &ProviderConnectionMetrics{}
	}
	cm.providerMetrics[provider].Requests++
	cm.mu.Unlock()
}

// RecordRequestComplete records a request completion
func (cm *ConnectionMetrics) RecordRequestComplete(provider string, success bool, latencyNs int64, reused bool) {
	atomic.AddInt64(&cm.activeRequests, -1)
	atomic.AddInt64(&cm.completedRequests, 1)

	if !success {
		atomic.AddInt64(&cm.failedRequests, 1)
	}

	if reused {
		atomic.AddInt64(&cm.connectionsReused, 1)
	}

	// Update latency stats
	atomic.AddInt64(&cm.totalLatencyNs, latencyNs)
	atomic.AddInt64(&cm.latencySamples, 1)

	cm.mu.Lock()
	// Update min/max
	if cm.minLatencyNs == 0 || latencyNs < cm.minLatencyNs {
		cm.minLatencyNs = latencyNs
	}
	if latencyNs > cm.maxLatencyNs {
		cm.maxLatencyNs = latencyNs
	}

	// Update provider metrics
	if pm, ok := cm.providerMetrics[provider]; ok {
		pm.TotalLatency += latencyNs
		if success {
			pm.Successes++
		} else {
			pm.Failures++
		}
		if reused {
			pm.Reused++
		}
	}
	cm.mu.Unlock()
}

// RecordConnectionOpened records a new connection
func (cm *ConnectionMetrics) RecordConnectionOpened() {
	atomic.AddInt64(&cm.connectionsOpened, 1)
}

// RecordConnectionClosed records a closed connection
func (cm *ConnectionMetrics) RecordConnectionClosed() {
	atomic.AddInt64(&cm.connectionsClosed, 1)
}

// RecordConnectionError records a connection error
func (cm *ConnectionMetrics) RecordConnectionError() {
	atomic.AddInt64(&cm.connectionErrors, 1)
}

// Stats returns connection metrics
func (cm *ConnectionMetrics) Stats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	samples := atomic.LoadInt64(&cm.latencySamples)
	avgLatency := int64(0)
	if samples > 0 {
		avgLatency = atomic.LoadInt64(&cm.totalLatencyNs) / samples
	}

	reused := atomic.LoadInt64(&cm.connectionsReused)
	completed := atomic.LoadInt64(&cm.completedRequests)
	reuseRate := float64(0)
	if completed > 0 {
		reuseRate = float64(reused) / float64(completed) * 100
	}

	// Copy provider metrics
	providerStats := make(map[string]interface{})
	for name, pm := range cm.providerMetrics {
		avgProviderLatency := int64(0)
		if pm.Requests > 0 {
			avgProviderLatency = pm.TotalLatency / pm.Requests
		}
		providerStats[name] = map[string]interface{}{
			"requests":    pm.Requests,
			"successes":   pm.Successes,
			"failures":    pm.Failures,
			"reused":      pm.Reused,
			"avg_latency_ns": avgProviderLatency,
		}
	}

	return map[string]interface{}{
		"total_requests":      atomic.LoadInt64(&cm.totalRequests),
		"active_requests":     atomic.LoadInt64(&cm.activeRequests),
		"completed_requests":  atomic.LoadInt64(&cm.completedRequests),
		"failed_requests":     atomic.LoadInt64(&cm.failedRequests),
		"connections_opened":  atomic.LoadInt64(&cm.connectionsOpened),
		"connections_closed":  atomic.LoadInt64(&cm.connectionsClosed),
		"connections_reused":  reused,
		"connection_errors":   atomic.LoadInt64(&cm.connectionErrors),
		"reuse_rate":          reuseRate,
		"avg_latency_ns":      avgLatency,
		"min_latency_ns":      cm.minLatencyNs,
		"max_latency_ns":      cm.maxLatencyNs,
		"latency_samples":     samples,
		"providers":           providerStats,
	}
}

// Reset resets all metrics
func (cm *ConnectionMetrics) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	atomic.StoreInt64(&cm.totalRequests, 0)
	atomic.StoreInt64(&cm.activeRequests, 0)
	atomic.StoreInt64(&cm.completedRequests, 0)
	atomic.StoreInt64(&cm.failedRequests, 0)
	atomic.StoreInt64(&cm.connectionsOpened, 0)
	atomic.StoreInt64(&cm.connectionsClosed, 0)
	atomic.StoreInt64(&cm.connectionsReused, 0)
	atomic.StoreInt64(&cm.connectionErrors, 0)
	atomic.StoreInt64(&cm.totalLatencyNs, 0)
	atomic.StoreInt64(&cm.latencySamples, 0)
	cm.minLatencyNs = 0
	cm.maxLatencyNs = 0
	cm.providerMetrics = make(map[string]*ProviderConnectionMetrics)
}

// PerformanceManager coordinates all performance optimizations
type PerformanceManager struct {
	config      *PerformanceConfig
	bufferPool  *BufferPool
	cache       *ResponseCache
	connMetrics *ConnectionMetrics
}

// NewPerformanceManager creates a new performance manager
func NewPerformanceManager(config *PerformanceConfig) *PerformanceManager {
	if config == nil {
		config = DefaultPerformanceConfig()
	}

	return &PerformanceManager{
		config:      config,
		bufferPool:  NewBufferPool(config),
		cache:       NewResponseCache(config),
		connMetrics: NewConnectionMetrics(config),
	}
}

// GetBufferPool returns the buffer pool
func (pm *PerformanceManager) GetBufferPool() *BufferPool {
	return pm.bufferPool
}

// GetCache returns the response cache
func (pm *PerformanceManager) GetCache() *ResponseCache {
	return pm.cache
}

// GetConnectionMetrics returns the connection metrics
func (pm *PerformanceManager) GetConnectionMetrics() *ConnectionMetrics {
	return pm.connMetrics
}

// Stats returns all performance statistics
func (pm *PerformanceManager) Stats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_pool":        pm.bufferPool.Stats(),
		"cache":              pm.cache.Stats(),
		"connection_metrics": pm.connMetrics.Stats(),
	}
}
