package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CCCache is the main cache implementation for cc-cache.
// Supports L1 (memory) + L2 (disk) two-level caching with canonicalization and singleflight.
type CCCache struct {
	config        *CacheConfig
	entries       sync.Map // map[string]*CCCacheEntry — L1 memory
	size          int64    // Current number of L1 entries
	canonicalizer *Canonicalizer
	singleflight  *CacheSingleflight
	disk          *DiskCache // L2 disk cache (nil if disabled)

	// Stats
	hits      int64
	misses    int64
	evictions int64
	bypasses  int64 // Requests that bypassed cache (streaming, etc.)

	// L1/L2 split stats
	l1Hits       int64
	diskHits     int64
	latencySaved int64 // Total latency saved in milliseconds

	// Persistence
	statsStore StatsStore

	// Cleanup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// StatsStore interface for persisting cache statistics
type StatsStore interface {
	SaveCacheStats(stats *CacheStats) error
	LoadCacheStats() (*CacheStats, error)
}

// CacheStats represents persistable cache statistics
type CacheStats struct {
	Hits      int64     `json:"hits"`
	Misses    int64     `json:"misses"`
	Evictions int64     `json:"evictions"`
	Bypasses  int64     `json:"bypasses"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CCCacheEntry represents a cached response
type CCCacheEntry struct {
	Key         string            `json:"key"`
	Body        []byte            `json:"body"`
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	HitCount    int64             `json:"hit_count"`
	ContentHash string            `json:"content_hash"` // For deduplication
}

// NewCCCache creates a new cc-cache instance with L1 memory + optional L2 disk.
func NewCCCache(config *CacheConfig) *CCCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &CCCache{
		config:        config,
		canonicalizer: NewCanonicalizer(),
		singleflight:  NewCacheSingleflight(),
		stopCleanup:   make(chan struct{}),
	}

	// Initialize L2 disk cache if configured
	if config.StorageType == "disk" || config.StorageType == "multilevel" {
		path := config.StoragePath
		if path == "" {
			path = "./data/cache.db"
		}
		if dc, err := NewDiskCache(path); err == nil {
			cache.disk = dc
			fmt.Printf("[CCCache] L2 disk cache initialized at %s\n", path)
		} else {
			fmt.Printf("[CCCache] L2 disk cache init failed: %v (L1-only mode)\n", err)
		}
	}

	// Start cleanup goroutine
	if config.Enabled {
		cache.cleanupTicker = time.NewTicker(time.Minute)
		go cache.cleanupLoop()
	}

	return cache
}

// SetStatsStore sets the stats persistence store and loads existing stats
func (c *CCCache) SetStatsStore(store StatsStore) {
	c.statsStore = store
	// Load existing stats
	if stats, err := store.LoadCacheStats(); err == nil && stats != nil {
		atomic.StoreInt64(&c.hits, stats.Hits)
		atomic.StoreInt64(&c.misses, stats.Misses)
		atomic.StoreInt64(&c.evictions, stats.Evictions)
		atomic.StoreInt64(&c.bypasses, stats.Bypasses)
	}
}

// SaveStats persists current stats to the store
func (c *CCCache) SaveStats() error {
	if c.statsStore == nil {
		return nil
	}
	stats := &CacheStats{
		Hits:      atomic.LoadInt64(&c.hits),
		Misses:    atomic.LoadInt64(&c.misses),
		Evictions: atomic.LoadInt64(&c.evictions),
		Bypasses:  atomic.LoadInt64(&c.bypasses),
		UpdatedAt: time.Now(),
	}
	return c.statsStore.SaveCacheStats(stats)
}

// cleanupLoop periodically removes expired entries and saves stats
func (c *CCCache) cleanupLoop() {
	statsTicker := time.NewTicker(5 * time.Minute) // Save stats every 5 minutes
	defer statsTicker.Stop()

	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanup()
		case <-statsTicker.C:
			c.SaveStats()
		case <-c.stopCleanup:
			c.cleanupTicker.Stop()
			c.SaveStats() // Save stats on shutdown
			return
		}
	}
}

// cleanup removes expired entries from L1 and L2
func (c *CCCache) cleanup() {
	now := time.Now()
	c.entries.Range(func(key, value interface{}) bool {
		entry := value.(*CCCacheEntry)
		if now.After(entry.ExpiresAt) {
			c.entries.Delete(key)
			atomic.AddInt64(&c.size, -1)
			atomic.AddInt64(&c.evictions, 1)
		}
		return true
	})
	// Cleanup L2 disk
	if c.disk != nil {
		c.disk.Cleanup()
	}
}

// Stop stops the cache cleanup goroutine and closes L2 disk.
func (c *CCCache) Stop() {
	if c.cleanupTicker != nil {
		close(c.stopCleanup)
	}
	if c.disk != nil {
		c.disk.Close()
	}
}

// GenerateKey generates a cache key from request using FNV hash (faster than SHA256)
func (c *CCCache) GenerateKey(r *http.Request, body []byte) string {
	// Use FNV-1a hash for better performance
	const (
		fnvOffset uint64 = 14695981039346656037
		fnvPrime  uint64 = 1099511628211
	)
	hash := fnvOffset

	// Method
	for _, b := range r.Method {
		hash ^= uint64(b)
		hash *= fnvPrime
	}

	// Path
	for _, b := range r.URL.Path {
		hash ^= uint64(b)
		hash *= fnvPrime
	}

	// Query params (sorted, excluding ignored)
	if r.URL.RawQuery != "" {
		params := r.URL.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			// Skip ignored params
			skip := false
			for _, ignored := range c.config.KeyIgnoreParams {
				if k == ignored {
					skip = true
					break
				}
			}
			if !skip {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, b := range k {
				hash ^= uint64(b)
				hash *= fnvPrime
			}
			for _, v := range params[k] {
				for _, b := range v {
					hash ^= uint64(b)
					hash *= fnvPrime
				}
			}
		}
	}

	// Include specified headers
	for _, header := range c.config.KeyIncludeHeaders {
		if v := r.Header.Get(header); v != "" {
			for _, b := range header {
				hash ^= uint64(b)
				hash *= fnvPrime
			}
			for _, b := range v {
				hash ^= uint64(b)
				hash *= fnvPrime
			}
		}
	}

	// Body hash (for POST requests)
	if len(body) > 0 {
		for _, b := range body {
			hash ^= uint64(b)
			hash *= fnvPrime
		}
	}

	return fmt.Sprintf("%016x", hash)
}

// ShouldCache determines if a request should be cached
func (c *CCCache) ShouldCache(r *http.Request) bool {
	if !c.config.Enabled {
		return false
	}

	// Check method
	methodAllowed := false
	for _, m := range c.config.CacheableMethods {
		if r.Method == m {
			methodAllowed = true
			break
		}
	}
	if !methodAllowed {
		return false
	}

	// Check for streaming request
	if c.config.SkipStreaming {
		// Check Accept header for streaming
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/event-stream") {
			return false
		}

		// Check body for stream: true
		// This is done in the middleware after reading body
	}

	return true
}

// IsStreamingRequest checks if the request body indicates streaming
func (c *CCCache) IsStreamingRequest(body []byte) bool {
	if !c.config.SkipStreaming {
		return false
	}

	// Parse JSON body to check for stream field
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}

	if stream, ok := req["stream"].(bool); ok && stream {
		return true
	}

	return false
}

// Get retrieves a cached response. Checks L1 memory first, then L2 disk.
func (c *CCCache) Get(key string) (*CCCacheEntry, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	// L1 memory lookup
	value, ok := c.entries.Load(key)
	if ok {
		entry := value.(*CCCacheEntry)
		if time.Now().After(entry.ExpiresAt) {
			c.entries.Delete(key)
			atomic.AddInt64(&c.size, -1)
			atomic.AddInt64(&c.evictions, 1)
		} else {
			atomic.AddInt64(&c.hits, 1)
			atomic.AddInt64(&c.l1Hits, 1)
			atomic.AddInt64(&entry.HitCount, 1)
			return entry, true
		}
	}

	// L2 disk lookup
	if c.disk != nil {
		if entry, found := c.disk.Get(key); found {
			atomic.AddInt64(&c.hits, 1)
			atomic.AddInt64(&c.diskHits, 1)
			// Promote to L1
			c.entries.Store(key, entry)
			atomic.AddInt64(&c.size, 1)
			return entry, true
		}
	}

	atomic.AddInt64(&c.misses, 1)
	return nil, false
}

// Set stores a response in L1 memory cache and L2 disk cache (write-through).
func (c *CCCache) Set(key string, body []byte, statusCode int, headers http.Header, provider, model string) {
	if !c.config.Enabled {
		return
	}

	// Check if status code is cacheable
	cacheable := false
	for _, code := range c.config.CacheableStatusCodes {
		if code == statusCode {
			cacheable = true
			break
		}
	}
	if !cacheable {
		return
	}

	// Check entry size
	if len(body) > c.config.MaxEntrySize {
		return
	}

	// Check max size and evict if needed
	currentSize := atomic.LoadInt64(&c.size)
	if int(currentSize) >= c.config.MaxSize {
		c.evictOldest()
	}

	// Convert headers
	headerMap := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			headerMap[k] = v[0]
		}
	}

	// Calculate content hash for deduplication
	contentHash := sha256.Sum256(body)

	now := time.Now()
	entry := &CCCacheEntry{
		Key:         key,
		Body:        body,
		StatusCode:  statusCode,
		Headers:     headerMap,
		Provider:    provider,
		Model:       model,
		CreatedAt:   now,
		ExpiresAt:   now.Add(c.config.TTL),
		ContentHash: hex.EncodeToString(contentHash[:16]),
	}

	c.entries.Store(key, entry)
	atomic.AddInt64(&c.size, 1)

	// Write-through to L2 disk
	if c.disk != nil {
		go c.disk.Set(entry)
	}
}

// evictOldest removes the oldest entry
func (c *CCCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	c.entries.Range(func(key, value interface{}) bool {
		entry := value.(*CCCacheEntry)
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key.(string)
			oldestTime = entry.CreatedAt
		}
		return true
	})

	if oldestKey != "" {
		c.entries.Delete(oldestKey)
		atomic.AddInt64(&c.size, -1)
		atomic.AddInt64(&c.evictions, 1)
	}
}

// Clear clears all cache entries (L1 + L2)
func (c *CCCache) Clear() {
	c.entries.Range(func(key, _ interface{}) bool {
		c.entries.Delete(key)
		return true
	})
	atomic.StoreInt64(&c.size, 0)
	if c.disk != nil {
		c.disk.Clear()
	}
}

// Stats returns cache statistics with L1/L2 breakdown.
func (c *CCCache) Stats() map[string]interface{} {
	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)
	l1Hits := atomic.LoadInt64(&c.l1Hits)
	diskHits := atomic.LoadInt64(&c.diskHits)
	total := hits + misses

	hitRate := float64(0)
	l1HitRate := float64(0)
	diskHitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
		l1HitRate = float64(l1Hits) / float64(total) * 100
		diskHitRate = float64(diskHits) / float64(total) * 100
	}

	stats := map[string]interface{}{
		"enabled":        c.config.Enabled,
		"storage_type":   c.config.StorageType,
		"l1_entries":     atomic.LoadInt64(&c.size),
		"max_entries":    c.config.MaxSize,
		"hits":           hits,
		"misses":         misses,
		"evictions":      atomic.LoadInt64(&c.evictions),
		"bypasses":       atomic.LoadInt64(&c.bypasses),
		"hit_rate":       hitRate,
		"l1_hits":        l1Hits,
		"l1_hit_rate":    l1HitRate,
		"disk_hits":      diskHits,
		"disk_hit_rate":  diskHitRate,
		"latency_saved_ms": atomic.LoadInt64(&c.latencySaved),
		"ttl_seconds":    c.config.TTL.Seconds(),
	}

	if c.disk != nil {
		stats["disk_entries"] = c.disk.Count()
	}

	return stats
}

// RecordBypass records a cache bypass (streaming, etc.)
func (c *CCCache) RecordBypass() {
	atomic.AddInt64(&c.bypasses, 1)
}

// RecordLatencySaved adds saved latency in milliseconds.
func (c *CCCache) RecordLatencySaved(ms int64) {
	atomic.AddInt64(&c.latencySaved, ms)
}

// GenerateCanonicalKey generates a semantic cache key using the Canonicalizer.
// This produces stable keys for semantically identical requests.
func (c *CCCache) GenerateCanonicalKey(body []byte) string {
	if c.canonicalizer != nil {
		return c.canonicalizer.CanonicalKey(body)
	}
	// Fallback to raw body hash
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// GetSingleflight returns the singleflight instance.
func (c *CCCache) GetSingleflight() *CacheSingleflight {
	return c.singleflight
}

// GetDisk returns the L2 disk cache (nil if not configured).
func (c *CCCache) GetDisk() *DiskCache {
	return c.disk
}

// Warmup loads top-N entries from L2 disk into L1 memory.
func (c *CCCache) Warmup(topN int) int {
	if c.disk == nil {
		return 0
	}
	entries := c.disk.TopN(topN)
	loaded := 0
	for _, entry := range entries {
		if _, exists := c.entries.Load(entry.Key); !exists {
			c.entries.Store(entry.Key, entry)
			atomic.AddInt64(&c.size, 1)
			loaded++
		}
	}
	fmt.Printf("[CCCache] Warmup: loaded %d entries from L2 disk\n", loaded)
	return loaded
}

// CacheMiddleware creates HTTP middleware for caching
type CacheMiddleware struct {
	cache *CCCache
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(cache *CCCache) *CacheMiddleware {
	return &CacheMiddleware{cache: cache}
}

// Wrap wraps an http.Handler with caching
func (cm *CacheMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if caching is enabled and request is cacheable
		if !cm.cache.ShouldCache(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Read and buffer the request body
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Check if this is a streaming request
		if cm.cache.IsStreamingRequest(bodyBytes) {
			cm.cache.RecordBypass()
			// Restore body for next handler
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			next.ServeHTTP(w, r)
			return
		}

		// Generate cache key
		cacheKey := cm.cache.GenerateKey(r, bodyBytes)

		// Try to get from cache
		if entry, ok := cm.cache.Get(cacheKey); ok {
			// Cache hit - return cached response
			for k, v := range entry.Headers {
				w.Header().Set(k, v)
			}
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("X-Cache-Key", cacheKey[:16])
			w.WriteHeader(entry.StatusCode)
			w.Write(entry.Body)
			return
		}

		// Cache miss - capture response
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}

		// Restore body for next handler
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Call next handler
		next.ServeHTTP(recorder, r)

		// Store in cache if response is cacheable
		if !recorder.isStreaming {
			// Extract provider and model from request body
			var reqBody map[string]interface{}
			provider := ""
			model := ""
			if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
				if m, ok := reqBody["model"].(string); ok {
					model = m
				}
			}

			cm.cache.Set(
				cacheKey,
				recorder.body.Bytes(),
				recorder.statusCode,
				recorder.Header(),
				provider,
				model,
			)
		}

		// Add cache headers
		w.Header().Set("X-Cache", "MISS")
		w.Header().Set("X-Cache-Key", cacheKey[:16])
	})
}

// responseRecorder captures the response for caching
type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	body        *bytes.Buffer
	isStreaming bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code

	// Check if this is a streaming response
	contentType := rr.Header().Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/x-ndjson") {
		rr.isStreaming = true
	}

	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	// Only buffer non-streaming responses
	if !rr.isStreaming {
		rr.body.Write(b)
	}
	return rr.ResponseWriter.Write(b)
}

// Flush implements http.Flusher
func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
