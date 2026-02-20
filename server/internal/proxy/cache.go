package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ecache2 "github.com/orca-zhang/ecache2"
	"github.com/tidwall/gjson"
)

// CCCache is the main cache implementation for cc-cache.
// L1 uses ecache2 (LRU-2, sharded, lock-free reads) for O(1) get/set/evict.
// L2 uses SQLite disk cache for persistence across restarts.
type CCCache struct {
	config        *CacheConfig
	l1            *ecache2.Cache[string] // L1 memory (LRU-2, handles eviction + TTL)
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

	// Token savings (parsed from cached response usage)
	inputTokensSaved  int64
	outputTokensSaved int64

	// Persistence
	statsStore StatsStore

	// Cleanup (disk only — L1 TTL handled by ecache2)
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}

	// Lazy initialization
	initOnce sync.Once
}

// StatsStore interface for persisting cache statistics
type StatsStore interface {
	SaveCacheStats(stats *CacheStats) error
	LoadCacheStats() (*CacheStats, error)
}

// CacheStats represents persistable cache statistics
type CacheStats struct {
	Hits              int64     `json:"hits"`
	Misses            int64     `json:"misses"`
	Evictions         int64     `json:"evictions"`
	Bypasses          int64     `json:"bypasses"`
	InputTokensSaved  int64     `json:"input_tokens_saved"`
	OutputTokensSaved int64     `json:"output_tokens_saved"`
	UpdatedAt         time.Time `json:"updated_at"`
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
	ContentHash string            `json:"content_hash"`
}

// NewCCCache creates a new cc-cache instance. The expensive L1 memory cache,
// L2 disk cache, and cleanup goroutine are lazily initialized on first use
// to reduce startup RSS.
func NewCCCache(config *CacheConfig) *CCCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	return &CCCache{
		config:        config,
		canonicalizer: NewCanonicalizer(),
		singleflight:  NewCacheSingleflight(),
		stopCleanup:   make(chan struct{}),
	}
}

// ensureInit lazily initializes L1 memory cache, L2 disk cache, and cleanup goroutine.
func (c *CCCache) ensureInit() {
	c.initOnce.Do(func() {
		bucketCount := 16
		bucketSize := c.config.MaxSize / bucketCount
		if bucketSize < 10 {
			bucketSize = 10
		}
		ttl := c.config.TTL
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}

		c.l1 = ecache2.NewLRUCache[string](uint16(bucketCount), uint16(bucketSize), ttl).LRU2(uint16(bucketSize / 4))

		if c.config.StorageType == "disk" || c.config.StorageType == "multilevel" {
			path := c.config.StoragePath
			if path == "" {
				path = "./data/cache.db"
			}
			if dc, err := NewDiskCache(path); err == nil {
				c.disk = dc
			}
		}

		if c.config.Enabled {
			c.cleanupTicker = time.NewTicker(time.Minute)
			go c.cleanupLoop()
		}
	})
}

// SetStatsStore sets the stats persistence store and loads existing stats
func (c *CCCache) SetStatsStore(store StatsStore) {
	c.statsStore = store
	if stats, err := store.LoadCacheStats(); err == nil && stats != nil {
		atomic.StoreInt64(&c.hits, stats.Hits)
		atomic.StoreInt64(&c.misses, stats.Misses)
		atomic.StoreInt64(&c.evictions, stats.Evictions)
		atomic.StoreInt64(&c.bypasses, stats.Bypasses)
		atomic.StoreInt64(&c.inputTokensSaved, stats.InputTokensSaved)
		atomic.StoreInt64(&c.outputTokensSaved, stats.OutputTokensSaved)
	}
}

// SaveStats persists current stats to the store
func (c *CCCache) SaveStats() error {
	if c.statsStore == nil {
		return nil
	}
	return c.statsStore.SaveCacheStats(&CacheStats{
		Hits:              atomic.LoadInt64(&c.hits),
		Misses:            atomic.LoadInt64(&c.misses),
		Evictions:         atomic.LoadInt64(&c.evictions),
		Bypasses:          atomic.LoadInt64(&c.bypasses),
		InputTokensSaved:  atomic.LoadInt64(&c.inputTokensSaved),
		OutputTokensSaved: atomic.LoadInt64(&c.outputTokensSaved),
		UpdatedAt:         time.Now(),
	})
}

// cleanupLoop periodically cleans L2 disk and saves stats.
func (c *CCCache) cleanupLoop() {
	statsTicker := time.NewTicker(5 * time.Minute)
	defer statsTicker.Stop()
	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanup()
		case <-statsTicker.C:
			c.SaveStats()
		case <-c.stopCleanup:
			c.cleanupTicker.Stop()
			c.SaveStats()
			return
		}
	}
}

// cleanup removes expired entries from L2 disk.
// L1 cleanup is not needed — ecache2 handles TTL and eviction internally.
func (c *CCCache) cleanup() {
	if c.disk != nil {
		c.disk.Cleanup()
	}
}

// Stop stops the cache cleanup goroutine and closes L2 disk.
func (c *CCCache) Stop() {
	// Only stop if actually initialized
	if c.l1 == nil {
		return
	}
	if c.cleanupTicker != nil {
		close(c.stopCleanup)
	}
	if c.disk != nil {
		c.disk.Close()
	}
}

// GenerateKey generates a cache key from request using FNV hash.
func (c *CCCache) GenerateKey(r *http.Request, body []byte) string {
	const (
		fnvOffset uint64 = 14695981039346656037
		fnvPrime  uint64 = 1099511628211
	)
	hash := fnvOffset
	for _, b := range r.Method {
		hash ^= uint64(b)
		hash *= fnvPrime
	}
	for _, b := range r.URL.Path {
		hash ^= uint64(b)
		hash *= fnvPrime
	}
	if r.URL.RawQuery != "" {
		params := r.URL.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
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
	if c.config.SkipStreaming {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			return false
		}
	}
	return true
}

// IsStreamingRequest checks if the request body indicates streaming
func (c *CCCache) IsStreamingRequest(body []byte) bool {
	if !c.config.SkipStreaming {
		return false
	}
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	if stream, ok := req["stream"].(bool); ok && stream {
		return true
	}
	return false
}

// IsStreamingRequestFromParsed checks streaming flag from pre-parsed request.
func (c *CCCache) IsStreamingRequestFromParsed(reqMap map[string]interface{}) bool {
	if !c.config.SkipStreaming {
		return false
	}
	if stream, ok := reqMap["stream"].(bool); ok && stream {
		return true
	}
	return false
}

// Get retrieves a cached response. Checks L1 memory first, then L2 disk.
func (c *CCCache) Get(key string) (*CCCacheEntry, bool) {
	if !c.config.Enabled {
		return nil, false
	}
	c.ensureInit()
	// L1 memory lookup (ecache2 handles TTL internally)
	if val, ok := c.l1.Get(key); ok {
		entry := val.(*CCCacheEntry)
		atomic.AddInt64(&c.hits, 1)
		atomic.AddInt64(&c.l1Hits, 1)
		atomic.AddInt64(&entry.HitCount, 1)
		c.accumulateTokens(entry.Body)
		return entry, true
	}
	// L2 disk lookup
	if c.disk != nil {
		if entry, found := c.disk.Get(key); found {
			atomic.AddInt64(&c.hits, 1)
			atomic.AddInt64(&c.diskHits, 1)
			c.l1.Put(key, entry) // Promote to L1
			c.accumulateTokens(entry.Body)
			return entry, true
		}
	}
	atomic.AddInt64(&c.misses, 1)
	return nil, false
}

// accumulateTokens parses usage from a cached response body and adds to token counters.
// Supports OpenAI (prompt_tokens/completion_tokens) and Anthropic (input_tokens/output_tokens).
func (c *CCCache) accumulateTokens(body []byte) {
	if len(body) == 0 {
		return
	}
	// OpenAI format: usage.prompt_tokens / usage.completion_tokens
	if pt := gjson.GetBytes(body, "usage.prompt_tokens"); pt.Exists() {
		atomic.AddInt64(&c.inputTokensSaved, pt.Int())
		atomic.AddInt64(&c.outputTokensSaved, gjson.GetBytes(body, "usage.completion_tokens").Int())
		return
	}
	// Anthropic format: usage.input_tokens / usage.output_tokens
	if it := gjson.GetBytes(body, "usage.input_tokens"); it.Exists() {
		atomic.AddInt64(&c.inputTokensSaved, it.Int())
		atomic.AddInt64(&c.outputTokensSaved, gjson.GetBytes(body, "usage.output_tokens").Int())
	}
}

// Set stores a response in L1 memory and L2 disk (write-through).
func (c *CCCache) Set(key string, body []byte, statusCode int, headers http.Header, provider, model string) {
	if !c.config.Enabled {
		return
	}
	c.ensureInit()
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
	if len(body) > c.config.MaxEntrySize {
		return
	}
	headerMap := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			headerMap[k] = v[0]
		}
	}
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
		ContentHash: fnvHashBytes(body),
	}
	c.l1.Put(key, entry)
	if c.disk != nil {
		go c.disk.Set(entry)
	}
}

// fnvHashBytes computes FNV-1a hash of bytes, returning a hex string.
func fnvHashBytes(data []byte) string {
	hash := fnvOffset64
	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnvPrime64
	}
	return fmt.Sprintf("%016x", hash)
}

// Clear clears all cache entries. Recreates L1 ecache2 instance.
func (c *CCCache) Clear() {
	c.ensureInit()
	bucketCount := 16
	bucketSize := c.config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}
	ttl := c.config.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.l1 = ecache2.NewLRUCache[string](uint16(bucketCount), uint16(bucketSize), ttl).LRU2(uint16(bucketSize / 4))
	if c.disk != nil {
		c.disk.Clear()
	}
}

// Stats returns cache statistics with L1/L2 breakdown.
func (c *CCCache) Stats() map[string]interface{} {
	c.ensureInit()
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
		"enabled":              c.config.Enabled,
		"storage_type":         c.config.StorageType,
		"max_entries":          c.config.MaxSize,
		"hits":                 hits,
		"misses":               misses,
		"evictions":            atomic.LoadInt64(&c.evictions),
		"bypasses":             atomic.LoadInt64(&c.bypasses),
		"hit_rate":             hitRate,
		"l1_hits":              l1Hits,
		"l1_hit_rate":          l1HitRate,
		"disk_hits":            diskHits,
		"disk_hit_rate":        diskHitRate,
		"latency_saved_ms":     atomic.LoadInt64(&c.latencySaved),
		"input_tokens_saved":   atomic.LoadInt64(&c.inputTokensSaved),
		"output_tokens_saved":  atomic.LoadInt64(&c.outputTokensSaved),
		"ttl_seconds":          c.config.TTL.Seconds(),
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
func (c *CCCache) GenerateCanonicalKey(body []byte) string {
	if c.canonicalizer != nil {
		return c.canonicalizer.CanonicalKey(body)
	}
	return fnvHashBytes(body)
}

// GenerateCanonicalKeyFromParsed generates a semantic cache key from pre-parsed request.
func (c *CCCache) GenerateCanonicalKeyFromParsed(reqMap map[string]interface{}) string {
	if c.canonicalizer != nil {
		return c.canonicalizer.CanonicalKeyFromParsed(reqMap)
	}
	data, _ := json.Marshal(reqMap)
	return fnvHashBytes(data)
}

// GetSingleflight returns the singleflight instance.
func (c *CCCache) GetSingleflight() *CacheSingleflight {
	return c.singleflight
}

// GetDisk returns the L2 disk cache (nil if not configured).
func (c *CCCache) GetDisk() *DiskCache {
	c.ensureInit()
	return c.disk
}

// Warmup loads top-N entries from L2 disk into L1 memory.
func (c *CCCache) Warmup(topN int) int {
	c.ensureInit()
	if c.disk == nil {
		return 0
	}
	entries := c.disk.TopN(topN)
	loaded := 0
	for _, entry := range entries {
		if _, exists := c.l1.Get(entry.Key); !exists {
			c.l1.Put(entry.Key, entry)
			loaded++
		}
	}
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
		if !cm.cache.ShouldCache(r) {
			next.ServeHTTP(w, r)
			return
		}

		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		if cm.cache.IsStreamingRequest(bodyBytes) {
			cm.cache.RecordBypass()
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			next.ServeHTTP(w, r)
			return
		}

		cacheKey := cm.cache.GenerateKey(r, bodyBytes)

		if entry, ok := cm.cache.Get(cacheKey); ok {
			for k, v := range entry.Headers {
				w.Header().Set(k, v)
			}
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("X-Cache-Key", cacheKey[:16])
			w.WriteHeader(entry.StatusCode)
			w.Write(entry.Body)
			return
		}

		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		next.ServeHTTP(recorder, r)

		if !recorder.isStreaming {
			var reqBody map[string]interface{}
			provider := ""
			model := ""
			if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
				if m, ok := reqBody["model"].(string); ok {
					model = m
				}
			}
			cm.cache.Set(cacheKey, recorder.body.Bytes(), recorder.statusCode, recorder.Header(), provider, model)
		}

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
	contentType := rr.Header().Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/x-ndjson") {
		rr.isStreaming = true
	}
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
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
