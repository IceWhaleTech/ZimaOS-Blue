# Performance Optimization Guide

This document describes the performance optimization features implemented in ZimaOS-Blue v0.8.0.

## Table of Contents

1. [Database Optimization](#database-optimization)
2. [Memory Optimization](#memory-optimization)
3. [Caching Layer](#caching-layer)
4. [Concurrency Optimization](#concurrency-optimization)
5. [Network Optimization](#network-optimization)
6. [Benchmarking](#benchmarking)
7. [Configuration](#configuration)

---

## Database Optimization

### Zorm ORM

ZimaOS-Blue uses [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) for lightweight, high-performance database operations.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// Define entity with zorm tags
type User struct {
    ID    int64  `zorm:"id,auto_incr"`
    Name  string `zorm:"name"`
    Email string `zorm:"email"`
    Age   int    `zorm:"age"`
}

// Create database connection
config := database.DefaultZormConfig()
config.DSN = "./data.db"
config.MaxOpenConns = 10

db, err := database.NewZormDB(config)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create repository
repo := database.NewRepository[User](db, "users")
ctx := context.Background()

// Insert
user := &User{Name: "Alice", Email: "alice@example.com", Age: 25}
id, err := repo.Insert(ctx, user)

// Find by ID
var found User
err = repo.FindByID(ctx, id, &found)

// Find all with conditions
var users []User
count, err := repo.FindAll(ctx, &users,
    database.Where(database.Gt("age", 18)),
    database.OrderBy("name"),
)

// Find with pagination
var page []User
count, err := repo.FindWithPagination(ctx, &page, 1, 10) // page 1, 10 items

// Update
user.Name = "Alice Smith"
affected, err := repo.UpdateByID(ctx, id, user)

// Delete
affected, err := repo.DeleteByID(ctx, id)

// Count
count, err := repo.Count(ctx, database.Where(database.Gt("age", 18)))

// Check existence
exists, err := repo.Exists(ctx, database.Where(database.Eq("email", "alice@example.com")))
```

### Query Builder

Fluent query building interface.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

repo := database.NewRepository[User](db, "users")
ctx := context.Background()

// Build complex queries
var users []User
qb := repo.NewQueryBuilder(ctx)
count, err := qb.
    Where(database.Gt("age", 18)).
    Where(database.Like("name", "A%")).
    OrderBy("created_at DESC").
    Limit(10).
    Select(&users)

// With joins (using raw table access)
t := db.TableContext(ctx, "users")
var results []struct {
    UserName  string `zorm:"users.name"`
    OrderID   int64  `zorm:"orders.id"`
}
t.Select(&results,
    database.LeftJoin("orders", database.Cond("users.id = orders.user_id")),
    database.Where(database.Gt("orders.total", 100)),
)
```

### Batch Operations

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

repo := database.NewRepository[User](db, "users")
ctx := context.Background()

// Batch insert
users := []User{
    {Name: "Alice", Email: "alice@example.com", Age: 25},
    {Name: "Bob", Email: "bob@example.com", Age: 30},
    {Name: "Charlie", Email: "charlie@example.com", Age: 35},
}
affected, err := repo.InsertBatch(ctx, &users)
```

### Query Optimizer

The query optimizer analyzes SQL queries and provides optimization suggestions.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// Create optimizer
config := database.DefaultOptimizerConfig()
optimizer := database.NewQueryOptimizer(db, config)

// Analyze a query
stats, err := optimizer.Analyze(ctx, "SELECT * FROM users WHERE email = ?", "user@example.com")
if err != nil {
    log.Fatal(err)
}

// Get suggestions
suggestions := optimizer.Suggest(stats)
for _, s := range suggestions {
    fmt.Printf("Suggestion: %s\n", s)
}
```

### Batch Operations

Batch operations improve performance for bulk inserts, updates, and deletes.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// Create batch executor
config := database.DefaultBatchConfig()
executor := database.NewBatchExecutor(db, config)

// Batch insert
items := []database.BatchItem{
    {Query: "INSERT INTO users (name) VALUES (?)", Args: []interface{}{"Alice"}},
    {Query: "INSERT INTO users (name) VALUES (?)", Args: []interface{}{"Bob"}},
}
results, err := executor.ExecuteBatch(ctx, items)
```

### WAL Mode Manager

WAL (Write-Ahead Logging) mode improves concurrent read/write performance.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// Create WAL manager
config := database.DefaultWALConfig()
manager, err := database.NewWALManager(db, config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// WAL mode is automatically enabled and checkpoints are managed
```

### Connection Pool Manager

The connection pool manager provides health checks and connection lifecycle management.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// Create pool manager
config := database.DefaultPoolConfig()
manager := database.NewPoolManager(db, config)
defer manager.Close()

// Get pool statistics
stats := manager.Stats()
fmt.Printf("Active: %d, Idle: %d\n", stats.ActiveConnections, stats.IdleConnections)
```

---

## Memory Optimization

### Buffer Pool

Reuse byte buffers to reduce allocations.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pool"

// Create buffer pool
bufPool := pool.NewBufferPool()

// Get and use buffer
buf := bufPool.Get()
buf.WriteString("Hello, World!")
data := buf.String()
bufPool.Put(buf) // Return to pool
```

### Byte Slice Pool

Reuse byte slices of a specific size.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pool"

// Create pool for 4KB slices
slicePool := pool.NewByteSlicePool(4096)

// Get and use slice
slice := slicePool.Get()
copy(slice, data)
slicePool.Put(slice) // Return to pool
```

### Object Pool

Generic object pool for any type.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pool"

// Create pool for custom type
type MyStruct struct {
    Data []byte
}

objPool := pool.NewObjectPool(
    func() *MyStruct { return &MyStruct{Data: make([]byte, 1024)} },
    func(obj *MyStruct) { obj.Data = obj.Data[:0] },
)

// Get and use object
obj := objPool.Get()
// ... use obj ...
objPool.Put(obj) // Return to pool
```

---

## Caching Layer

### ECache (High-Performance LRU Cache)

ZimaOS-Blue uses [orca-zhang/ecache](https://github.com/orca-zhang/ecache) for high-performance in-memory caching with LRU eviction.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// Create ECache (LRU mode)
config := cache.Config{
    MaxSize:    1000,
    DefaultTTL: 5 * time.Minute,
}
ecache := cache.NewECache(config)
defer ecache.Close()

// Basic operations
ctx := context.Background()
ecache.Set(ctx, "key", "value") // Uses default TTL from config
value, err := ecache.Get(ctx, "key")

// GetOrSet - atomic get-or-compute
value, err := ecache.GetOrSet(ctx, "key", func() (interface{}, error) {
    return computeExpensiveValue()
})

// SetNX - set only if not exists
ok := ecache.SetNX(ctx, "key", "value")

// Get statistics
stats := ecache.Stats()
fmt.Printf("Hits: %d, Misses: %d, Hit Rate: %.2f%%\n",
    stats.Hits, stats.Misses, stats.HitRate)
```

### ECache2 (LRU-2 Mode)

LRU-2 mode provides better protection for frequently accessed data against bulk operations.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// Create ECache2 (LRU-2 mode)
config := cache.Config{
    MaxSize:    1000,
    DefaultTTL: 5 * time.Minute,
}
ecache2 := cache.NewECache2(config)
defer ecache2.Close()

// Use like regular ECache
ecache2.Set(ctx, "key", "value")
value, err := ecache2.Get(ctx, "key")
```

### LRU Cache (Legacy)

Least Recently Used cache with TTL support.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// Create LRU cache
config := cache.Config{
    MaxSize:         1000,
    DefaultTTL:      5 * time.Minute,
    CleanupInterval: time.Minute,
}
lruCache := cache.NewLRUCache(config)
defer lruCache.Close()

// Set and get values
ctx := context.Background()
lruCache.Set(ctx, "key", "value", 0) // 0 = use default TTL
value, err := lruCache.Get(ctx, "key")
```

### LFU Cache

Least Frequently Used cache for frequency-based eviction.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// Create LFU cache
config := cache.Config{
    MaxSize:         1000,
    DefaultTTL:      5 * time.Minute,
    CleanupInterval: time.Minute,
}
lfuCache := cache.NewLFUCache(config)
defer lfuCache.Close()
```

### Disk Cache

Persistent cache stored on disk with optional compression.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// Create disk cache
config := cache.DefaultL2Config()
config.Path = "/var/cache/zimaos-blue"
config.Compression = true
config.MaxDiskSize = 100 * 1024 * 1024 // 100MB

diskCache, err := cache.NewDiskCache(config)
if err != nil {
    log.Fatal(err)
}
defer diskCache.Close()
```

### Multi-Level Cache

Combines L1 (memory) and L2 (disk) caches for optimal performance.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// Create multi-level cache
config := cache.DefaultMultiLevelConfig()
config.L1.MaxSize = 1000
config.L2.Path = "/var/cache/zimaos-blue"
config.PromoteOnHit = true  // Promote L2 hits to L1
config.WriteThrough = true  // Write to both L1 and L2

mlCache, err := cache.NewMultiLevelCache(config)
if err != nil {
    log.Fatal(err)
}
defer mlCache.Close()

// Use like any other cache
ctx := context.Background()
mlCache.Set(ctx, "key", "value", time.Hour)
value, err := mlCache.Get(ctx, "key")
```

---

## Concurrency Optimization

### Optimized Worker Pool

Auto-scaling worker pool with task queuing.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"

// Create optimized pool
config := worker.DefaultOptimizedPoolConfig()
config.MinWorkers = 4
config.MaxWorkers = 16
config.QueueSize = 1000

pool := worker.NewOptimizedPool(ctx, config)
defer pool.Shutdown()

// Submit tasks
pool.SubmitFunc(func(ctx context.Context) error {
    // Do work
    return nil
})

// Submit with timeout
pool.SubmitFuncWithTimeout(func(ctx context.Context) error {
    // Do work with timeout
    return nil
}, 5*time.Second)

// Get statistics
stats := pool.Stats()
fmt.Printf("Active: %d, Completed: %d\n", stats.ActiveWorkers, stats.TasksCompleted)
```

### Sharded Map

Concurrent map with reduced lock contention.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sync"

// Create sharded map
m := sync.NewShardedMap[string, int](32, sync.StringHashFn)

// Use like a regular map
m.Set("key", 42)
value, ok := m.Get("key")
m.Delete("key")

// Iterate
m.Range(func(key string, value int) bool {
    fmt.Printf("%s: %d\n", key, value)
    return true // continue iteration
})
```

### Atomic Utilities

Lock-free counters and flags.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sync"

// Atomic counter
counter := sync.NewAtomicCounter(0)
counter.Inc()
counter.Add(10)
value := counter.Value()

// Atomic flag
flag := sync.NewAtomicFlag(false)
flag.Set()
if flag.IsSet() {
    // ...
}
flag.Clear()
```

---

## Network Optimization

### HTTP/2 Server

Configure HTTP/2 for improved performance.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// Create HTTP/2 server
config := network.DefaultHTTP2Config()
config.MaxConcurrentStreams = 250
config.IdleTimeout = 120 * time.Second

server := network.NewHTTP2Server(handler, config)
server.ListenAndServeTLS(":443", "cert.pem", "key.pem")
```

### Compression Middleware

Automatic response compression.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// Create compression middleware
config := network.DefaultCompressionConfig()
config.Level = 6
config.MinSize = 1024 // Only compress responses > 1KB

middleware := network.CompressionMiddleware(config)
handler = middleware(handler)
```

### Request Batching

Batch multiple requests into single operations.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// Create batcher
config := network.DefaultBatchConfig()
config.MaxBatchSize = 100
config.MaxWaitTime = 10 * time.Millisecond

loader := func(ctx context.Context, keys []string) (map[string]User, error) {
    // Load multiple users at once
    return db.GetUsersByIDs(ctx, keys)
}

batcher := network.NewBatcher[string, User](ctx, config, loader)
defer batcher.Close()

// Load single item (will be batched with concurrent requests)
user, err := batcher.Load(ctx, "user-123")
```

### Request Coalescing

Deduplicate concurrent identical requests.

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// Create coalescer
loader := func(ctx context.Context, key string) (User, error) {
    return db.GetUser(ctx, key)
}

coalescer := network.NewRequestCoalescer[string, User](loader, 5*time.Second)

// Multiple concurrent calls for same key will only execute loader once
user, err := coalescer.Load(ctx, "user-123")
```

---

## Benchmarking

### Running Benchmarks

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/benchmark"

// Create benchmark suite
suite := benchmark.NewSuite("My Benchmarks")

// Add benchmarks
suite.Add("cache-get", func(ctx context.Context) error {
    _, err := cache.Get(ctx, "key")
    return err
})

suite.Add("cache-set", func(ctx context.Context) error {
    return cache.Set(ctx, "key", "value", 0)
})

// Run benchmarks
config := benchmark.DefaultConfig()
config.Duration = 5 * time.Second
config.Warmup = 100

err := suite.Run(ctx, config)
if err != nil {
    log.Fatal(err)
}

// Print report
fmt.Println(suite.Report())

// Save results
suite.SaveJSON("benchmark-results.json")
```

### Comparing Results

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/benchmark"

// Compare two results
comparison := benchmark.Compare(baseline, current)

// Check for regression
if comparison.IsRegression(5.0) { // 5% threshold
    fmt.Println("Performance regression detected!")
}

// Print comparison
fmt.Println(comparison.Report())
```

---

## Configuration

### Performance Configuration

Add to your `config.yaml`:

```yaml
performance:
  # Database optimization
  database:
    pool_size: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h
    wal_mode: true
    cache_size: 10000
    page_size: 4096
    checkpoint_interval: 5m
    batch_size: 1000
    slow_query_threshold: 100ms
    enable_query_cache: true

  # Memory optimization
  memory:
    gogc: 100
    gomemlimit: 512MB
    buffer_pool_size: 1000
    object_pool_size: 500

  # Concurrency optimization
  concurrency:
    worker_pool_size: 100
    max_goroutines: 10000
    channel_buffer_size: 100

  # Network optimization
  network:
    http2_enabled: true
    keep_alive_timeout: 30s
    compression_enabled: true
    compression_level: 6
    request_timeout: 30s
    retry_max_attempts: 3
    retry_backoff_base: 100ms

  # Cache configuration
  cache:
    l1_enabled: true
    l1_size: 1000
    l1_ttl: 5m
    l2_enabled: true
    l2_path: ./cache
    l2_size: 100MB
    l2_ttl: 1h
    eviction_policy: lru

  # Profiling
  profiling:
    pprof_enabled: true
    pprof_path: /debug/pprof
    metrics_enabled: true
    benchmark_enabled: false
```

---

## Best Practices

### 1. Use Object Pools for Frequently Allocated Objects

```go
// Good: Reuse buffers
buf := bufPool.Get()
defer bufPool.Put(buf)

// Bad: Allocate new buffer each time
buf := new(bytes.Buffer)
```

### 2. Use Sharded Maps for High-Concurrency Scenarios

```go
// Good: Sharded map reduces lock contention
m := sync.NewShardedMap[string, int](32, sync.StringHashFn)

// Bad: sync.Map or mutex-protected map under high contention
var m sync.Map
```

### 3. Batch Database Operations

```go
// Good: Batch insert
executor.ExecuteBatch(ctx, items)

// Bad: Individual inserts in a loop
for _, item := range items {
    db.Exec(ctx, "INSERT ...", item)
}
```

### 4. Use Multi-Level Caching

```go
// Good: L1 (memory) + L2 (disk) cache
mlCache, _ := cache.NewMultiLevelCache(config)

// Bad: Only memory cache (loses data on restart)
memCache := cache.NewLRUCache(config)
```

### 5. Enable HTTP/2 and Compression

```yaml
# Good: Enable HTTP/2 and compression
network:
  http2_enabled: true
  compression_enabled: true
```

### 6. Monitor Performance Metrics

```go
// Enable Prometheus metrics
metrics := metrics.NewPerformanceMetrics(registry)

// Record cache operations
metrics.RecordCacheHit("main", "l1")
metrics.RecordCacheLatency("main", "get", duration)
```

---

## Troubleshooting

### High Memory Usage

1. Check GOGC setting (lower = more frequent GC)
2. Review object pool usage
3. Check cache sizes
4. Use pprof to identify memory leaks

### High CPU Usage

1. Check goroutine count
2. Review lock contention with pprof
3. Check for busy loops
4. Review database query efficiency

### Slow Database Queries

1. Enable query optimizer
2. Check for missing indexes
3. Use batch operations
4. Enable WAL mode
5. Review connection pool settings

### Network Latency

1. Enable HTTP/2
2. Enable compression
3. Use request batching
4. Review keep-alive settings
