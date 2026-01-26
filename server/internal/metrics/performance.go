package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// PerformanceMetrics holds performance-related prometheus metrics.
type PerformanceMetrics struct {
	registry *prometheus.Registry

	// Database metrics
	dbQueryDuration     *prometheus.HistogramVec
	dbQueryTotal        *prometheus.CounterVec
	dbSlowQueries       prometheus.Counter
	dbConnectionsActive prometheus.Gauge
	dbConnectionsIdle   prometheus.Gauge
	dbPoolWaitTime      prometheus.Histogram

	// Cache metrics
	cacheHits       *prometheus.CounterVec
	cacheMisses     *prometheus.CounterVec
	cacheEvictions  *prometheus.CounterVec
	cacheSize       *prometheus.GaugeVec
	cacheHitRate    *prometheus.GaugeVec
	cacheLatency    *prometheus.HistogramVec

	// Memory pool metrics
	poolGets     *prometheus.CounterVec
	poolPuts     *prometheus.CounterVec
	poolNews     *prometheus.CounterVec
	poolHitRate  *prometheus.GaugeVec

	// GC metrics
	gcPauseTime    prometheus.Histogram
	gcCycles       prometheus.Counter
	heapAlloc      prometheus.Gauge
	heapInuse      prometheus.Gauge
	heapObjects    prometheus.Gauge

	// Goroutine metrics
	goroutinesActive prometheus.Gauge
	goroutinesCreated prometheus.Counter
}

// NewPerformanceMetrics creates a new PerformanceMetrics instance.
func NewPerformanceMetrics(registry *prometheus.Registry) *PerformanceMetrics {
	pm := &PerformanceMetrics{
		registry: registry,
	}

	pm.initDatabaseMetrics()
	pm.initCacheMetrics()
	pm.initPoolMetrics()
	pm.initGCMetrics()
	pm.initGoroutineMetrics()

	return pm
}

func (pm *PerformanceMetrics) initDatabaseMetrics() {
	pm.dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "table"},
	)

	pm.dbQueryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation", "table", "status"},
	)

	pm.dbSlowQueries = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "db_slow_queries_total",
			Help: "Total number of slow database queries",
		},
	)

	pm.dbConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
	)

	pm.dbConnectionsIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	pm.dbPoolWaitTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "db_pool_wait_seconds",
			Help:    "Time spent waiting for a database connection",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
		},
	)

	pm.registry.MustRegister(
		pm.dbQueryDuration,
		pm.dbQueryTotal,
		pm.dbSlowQueries,
		pm.dbConnectionsActive,
		pm.dbConnectionsIdle,
		pm.dbPoolWaitTime,
	)
}

func (pm *PerformanceMetrics) initCacheMetrics() {
	pm.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache", "level"},
	)

	pm.cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache", "level"},
	)

	pm.cacheEvictions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_evictions_total",
			Help: "Total number of cache evictions",
		},
		[]string{"cache", "level"},
	)

	pm.cacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_size",
			Help: "Current cache size (number of entries)",
		},
		[]string{"cache", "level"},
	)

	pm.cacheHitRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_hit_rate",
			Help: "Cache hit rate percentage",
		},
		[]string{"cache", "level"},
	)

	pm.cacheLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Cache operation duration in seconds",
			Buckets: []float64{.00001, .00005, .0001, .0005, .001, .005, .01},
		},
		[]string{"cache", "operation"},
	)

	pm.registry.MustRegister(
		pm.cacheHits,
		pm.cacheMisses,
		pm.cacheEvictions,
		pm.cacheSize,
		pm.cacheHitRate,
		pm.cacheLatency,
	)
}

func (pm *PerformanceMetrics) initPoolMetrics() {
	pm.poolGets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pool_gets_total",
			Help: "Total number of pool gets",
		},
		[]string{"pool"},
	)

	pm.poolPuts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pool_puts_total",
			Help: "Total number of pool puts",
		},
		[]string{"pool"},
	)

	pm.poolNews = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pool_news_total",
			Help: "Total number of new allocations from pool",
		},
		[]string{"pool"},
	)

	pm.poolHitRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pool_hit_rate",
			Help: "Pool hit rate percentage",
		},
		[]string{"pool"},
	)

	pm.registry.MustRegister(
		pm.poolGets,
		pm.poolPuts,
		pm.poolNews,
		pm.poolHitRate,
	)
}

func (pm *PerformanceMetrics) initGCMetrics() {
	pm.gcPauseTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gc_pause_seconds",
			Help:    "GC pause time in seconds",
			Buckets: []float64{.00001, .00005, .0001, .0005, .001, .005, .01, .05},
		},
	)

	pm.gcCycles = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gc_cycles_total",
			Help: "Total number of GC cycles",
		},
	)

	pm.heapAlloc = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "heap_alloc_bytes",
			Help: "Heap allocation in bytes",
		},
	)

	pm.heapInuse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "heap_inuse_bytes",
			Help: "Heap in use in bytes",
		},
	)

	pm.heapObjects = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "heap_objects",
			Help: "Number of heap objects",
		},
	)

	pm.registry.MustRegister(
		pm.gcPauseTime,
		pm.gcCycles,
		pm.heapAlloc,
		pm.heapInuse,
		pm.heapObjects,
	)
}

func (pm *PerformanceMetrics) initGoroutineMetrics() {
	pm.goroutinesActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "goroutines_active",
			Help: "Number of active goroutines",
		},
	)

	pm.goroutinesCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "goroutines_created_total",
			Help: "Total number of goroutines created",
		},
	)

	pm.registry.MustRegister(
		pm.goroutinesActive,
		pm.goroutinesCreated,
	)
}

// RecordDBQuery records a database query.
func (pm *PerformanceMetrics) RecordDBQuery(operation, table string, duration float64, success bool) {
	pm.dbQueryDuration.WithLabelValues(operation, table).Observe(duration)

	status := "success"
	if !success {
		status = "error"
	}
	pm.dbQueryTotal.WithLabelValues(operation, table, status).Inc()
}

// RecordSlowQuery records a slow query.
func (pm *PerformanceMetrics) RecordSlowQuery() {
	pm.dbSlowQueries.Inc()
}

// UpdateDBConnections updates database connection metrics.
func (pm *PerformanceMetrics) UpdateDBConnections(active, idle int) {
	pm.dbConnectionsActive.Set(float64(active))
	pm.dbConnectionsIdle.Set(float64(idle))
}

// RecordDBPoolWait records time spent waiting for a connection.
func (pm *PerformanceMetrics) RecordDBPoolWait(duration float64) {
	pm.dbPoolWaitTime.Observe(duration)
}

// RecordCacheHit records a cache hit.
func (pm *PerformanceMetrics) RecordCacheHit(cache, level string) {
	pm.cacheHits.WithLabelValues(cache, level).Inc()
}

// RecordCacheMiss records a cache miss.
func (pm *PerformanceMetrics) RecordCacheMiss(cache, level string) {
	pm.cacheMisses.WithLabelValues(cache, level).Inc()
}

// RecordCacheEviction records a cache eviction.
func (pm *PerformanceMetrics) RecordCacheEviction(cache, level string) {
	pm.cacheEvictions.WithLabelValues(cache, level).Inc()
}

// UpdateCacheSize updates the cache size metric.
func (pm *PerformanceMetrics) UpdateCacheSize(cache, level string, size int64) {
	pm.cacheSize.WithLabelValues(cache, level).Set(float64(size))
}

// UpdateCacheHitRate updates the cache hit rate metric.
func (pm *PerformanceMetrics) UpdateCacheHitRate(cache, level string, rate float64) {
	pm.cacheHitRate.WithLabelValues(cache, level).Set(rate)
}

// RecordCacheLatency records cache operation latency.
func (pm *PerformanceMetrics) RecordCacheLatency(cache, operation string, duration float64) {
	pm.cacheLatency.WithLabelValues(cache, operation).Observe(duration)
}

// RecordPoolGet records a pool get operation.
func (pm *PerformanceMetrics) RecordPoolGet(pool string) {
	pm.poolGets.WithLabelValues(pool).Inc()
}

// RecordPoolPut records a pool put operation.
func (pm *PerformanceMetrics) RecordPoolPut(pool string) {
	pm.poolPuts.WithLabelValues(pool).Inc()
}

// RecordPoolNew records a new allocation from pool.
func (pm *PerformanceMetrics) RecordPoolNew(pool string) {
	pm.poolNews.WithLabelValues(pool).Inc()
}

// UpdatePoolHitRate updates the pool hit rate metric.
func (pm *PerformanceMetrics) UpdatePoolHitRate(pool string, rate float64) {
	pm.poolHitRate.WithLabelValues(pool).Set(rate)
}

// RecordGCPause records a GC pause time.
func (pm *PerformanceMetrics) RecordGCPause(duration float64) {
	pm.gcPauseTime.Observe(duration)
}

// RecordGCCycle records a GC cycle.
func (pm *PerformanceMetrics) RecordGCCycle() {
	pm.gcCycles.Inc()
}

// UpdateHeapStats updates heap statistics.
func (pm *PerformanceMetrics) UpdateHeapStats(alloc, inuse int64, objects int64) {
	pm.heapAlloc.Set(float64(alloc))
	pm.heapInuse.Set(float64(inuse))
	pm.heapObjects.Set(float64(objects))
}

// UpdateGoroutines updates goroutine count.
func (pm *PerformanceMetrics) UpdateGoroutines(count int) {
	pm.goroutinesActive.Set(float64(count))
}

// RecordGoroutineCreated records a new goroutine creation.
func (pm *PerformanceMetrics) RecordGoroutineCreated() {
	pm.goroutinesCreated.Inc()
}
