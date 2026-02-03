package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// CachePrefetcher learns access patterns and prefetches likely-to-be-needed data
type CachePrefetcher struct {
	// Access pattern tracking
	accessPatterns map[string]*AccessPattern
	patternMu      sync.RWMutex

	// Prefetch queue
	prefetchQueue chan PrefetchTask
	stopChan      chan struct{}

	// Metrics
	prefetched int64
	hits       int64
	misses     int64

	// Configuration
	maxPatterns    int
	minAccessCount int
	prefetchWorkers int
}

// AccessPattern tracks how often a key is accessed
type AccessPattern struct {
	Key          string
	AccessCount  int64
	LastAccess   time.Time
	Confidence   float64 // 0.0-1.0, higher = more likely to be accessed again
	RelatedKeys  []string
	AccessTimes  []time.Time // Last N access times for pattern analysis
	maxTimes     int
}

// PrefetchTask represents a prefetch operation
type PrefetchTask struct {
	Key      string
	Priority int
	Deadline time.Time
}

// NewCachePrefetcher creates a new cache prefetcher
func NewCachePrefetcher(maxPatterns, minAccessCount, prefetchWorkers int) *CachePrefetcher {
	cp := &CachePrefetcher{
		accessPatterns:  make(map[string]*AccessPattern),
		prefetchQueue:   make(chan PrefetchTask, 1000),
		stopChan:        make(chan struct{}),
		maxPatterns:     maxPatterns,
		minAccessCount:  minAccessCount,
		prefetchWorkers: prefetchWorkers,
	}

	// Start prefetch workers
	for i := 0; i < prefetchWorkers; i++ {
		go cp.prefetchWorker()
	}

	return cp
}

// RecordAccess records an access to a key
func (cp *CachePrefetcher) RecordAccess(key string) {
	cp.patternMu.Lock()
	defer cp.patternMu.Unlock()

	pattern, exists := cp.accessPatterns[key]
	if !exists {
		if len(cp.accessPatterns) >= cp.maxPatterns {
			// Evict least confident pattern
			cp.evictLeastConfident()
		}
		pattern = &AccessPattern{
			Key:         key,
			AccessTimes: make([]time.Time, 0, 10),
			maxTimes:    10,
		}
		cp.accessPatterns[key] = pattern
	}

	pattern.AccessCount++
	pattern.LastAccess = time.Now()

	// Track access times for pattern analysis
	if len(pattern.AccessTimes) >= pattern.maxTimes {
		pattern.AccessTimes = pattern.AccessTimes[1:]
	}
	pattern.AccessTimes = append(pattern.AccessTimes, time.Now())

	// Update confidence based on access frequency
	cp.updateConfidence(pattern)
}

// updateConfidence updates the confidence score based on access patterns
func (cp *CachePrefetcher) updateConfidence(pattern *AccessPattern) {
	if pattern.AccessCount < int64(cp.minAccessCount) {
		pattern.Confidence = 0.0
		return
	}

	// Base confidence on access count
	confidence := float64(pattern.AccessCount) / float64(cp.minAccessCount*10)
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Boost confidence if accesses are recent and frequent
	if len(pattern.AccessTimes) >= 2 {
		timeDiff := pattern.AccessTimes[len(pattern.AccessTimes)-1].Sub(pattern.AccessTimes[0])
		if timeDiff > 0 {
			frequency := float64(len(pattern.AccessTimes)) / timeDiff.Seconds()
			if frequency > 0.1 { // More than 0.1 accesses per second
				confidence = confidence * 1.2
			}
		}
	}

	if confidence > 1.0 {
		confidence = 1.0
	}
	pattern.Confidence = confidence
}

// GetPrefetchCandidates returns keys that should be prefetched
func (cp *CachePrefetcher) GetPrefetchCandidates(limit int) []string {
	cp.patternMu.RLock()
	defer cp.patternMu.RUnlock()

	candidates := make([]string, 0, limit)
	for _, pattern := range cp.accessPatterns {
		if pattern.Confidence > 0.5 && time.Since(pattern.LastAccess) < 5*time.Minute {
			candidates = append(candidates, pattern.Key)
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates
}

// Prefetch schedules a key for prefetching
func (cp *CachePrefetcher) Prefetch(key string, priority int) {
	select {
	case cp.prefetchQueue <- PrefetchTask{
		Key:      key,
		Priority: priority,
		Deadline: time.Now().Add(30 * time.Second),
	}:
		atomic.AddInt64(&cp.prefetched, 1)
	default:
		// Queue full, skip
	}
}

// prefetchWorker processes prefetch tasks
func (cp *CachePrefetcher) prefetchWorker() {
	for {
		select {
		case <-cp.stopChan:
			return
		case task := <-cp.prefetchQueue:
			if time.Now().Before(task.Deadline) {
				// Prefetch logic would go here
				// This is a placeholder for actual prefetch implementation
			}
		}
	}
}

// evictLeastConfident removes the pattern with lowest confidence
func (cp *CachePrefetcher) evictLeastConfident() {
	var leastKey string
	var leastConfidence float64 = 2.0

	for key, pattern := range cp.accessPatterns {
		if pattern.Confidence < leastConfidence {
			leastConfidence = pattern.Confidence
			leastKey = key
		}
	}

	if leastKey != "" {
		delete(cp.accessPatterns, leastKey)
	}
}

// Stop stops the prefetcher
func (cp *CachePrefetcher) Stop() {
	close(cp.stopChan)
}

// GetMetrics returns prefetcher metrics
func (cp *CachePrefetcher) GetMetrics() map[string]interface{} {
	cp.patternMu.RLock()
	defer cp.patternMu.RUnlock()

	return map[string]interface{}{
		"prefetched":     atomic.LoadInt64(&cp.prefetched),
		"hits":           atomic.LoadInt64(&cp.hits),
		"misses":         atomic.LoadInt64(&cp.misses),
		"patterns":       len(cp.accessPatterns),
		"queue_size":     len(cp.prefetchQueue),
		"max_patterns":   cp.maxPatterns,
	}
}
