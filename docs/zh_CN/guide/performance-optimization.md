# 性能优化指南

本文档描述了 ZimaOS-Blue v0.8.0 中实现的性能优化功能。

## 目录

1. [数据库优化](#数据库优化)
2. [内存优化](#内存优化)
3. [缓存层](#缓存层)
4. [并发优化](#并发优化)
5. [网络优化](#网络优化)
6. [基准测试](#基准测试)
7. [配置](#配置)

---

## 数据库优化

### Zorm ORM

ZimaOS-Blue 使用 [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) 进行轻量级、高性能的数据库操作。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// 使用 zorm 标签定义实体
type User struct {
    ID    int64  `zorm:"id,auto_incr"`
    Name  string `zorm:"name"`
    Email string `zorm:"email"`
    Age   int    `zorm:"age"`
}

// 创建数据库连接
config := database.DefaultZormConfig()
config.DSN = "./data.db"
config.MaxOpenConns = 10

db, err := database.NewZormDB(config)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// 创建仓库
repo := database.NewRepository[User](db, "users")
ctx := context.Background()

// 插入
user := &User{Name: "Alice", Email: "alice@example.com", Age: 25}
id, err := repo.Insert(ctx, user)

// 按 ID 查找
var found User
err = repo.FindByID(ctx, id, &found)

// 按条件查找所有
var users []User
count, err := repo.FindAll(ctx, &users,
    database.Where(database.Gt("age", 18)),
    database.OrderBy("name"),
)

// 分页查找
var page []User
count, err := repo.FindWithPagination(ctx, &page, 1, 10) // 第 1 页，10 条

// 更新
user.Name = "Alice Smith"
affected, err := repo.UpdateByID(ctx, id, user)

// 删除
affected, err := repo.DeleteByID(ctx, id)

// 计数
count, err := repo.Count(ctx, database.Where(database.Gt("age", 18)))

// 检查是否存在
exists, err := repo.Exists(ctx, database.Where(database.Eq("email", "alice@example.com")))
```

### 查询构建器

流畅的查询构建接口。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

repo := database.NewRepository[User](db, "users")
ctx := context.Background()

// 构建复杂查询
var users []User
qb := repo.NewQueryBuilder(ctx)
count, err := qb.
    Where(database.Gt("age", 18)).
    Where(database.Like("name", "A%")).
    OrderBy("created_at DESC").
    Limit(10).
    Select(&users)

// 使用连接（通过原始表访问）
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

### 批量操作

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

repo := database.NewRepository[User](db, "users")
ctx := context.Background()

// 批量插入
users := []User{
    {Name: "Alice", Email: "alice@example.com", Age: 25},
    {Name: "Bob", Email: "bob@example.com", Age: 30},
    {Name: "Charlie", Email: "charlie@example.com", Age: 35},
}
affected, err := repo.InsertBatch(ctx, &users)
```

### 查询优化器

查询优化器分析 SQL 查询并提供优化建议。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// 创建优化器
config := database.DefaultOptimizerConfig()
optimizer := database.NewQueryOptimizer(db, config)

// 分析查询
stats, err := optimizer.Analyze(ctx, "SELECT * FROM users WHERE email = ?", "user@example.com")
if err != nil {
    log.Fatal(err)
}

// 获取建议
suggestions := optimizer.Suggest(stats)
for _, s := range suggestions {
    fmt.Printf("建议: %s\n", s)
}
```

### 批量操作

批量操作可以提高批量插入、更新和删除的性能。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// 创建批量执行器
config := database.DefaultBatchConfig()
executor := database.NewBatchExecutor(db, config)

// 批量插入
items := []database.BatchItem{
    {Query: "INSERT INTO users (name) VALUES (?)", Args: []interface{}{"Alice"}},
    {Query: "INSERT INTO users (name) VALUES (?)", Args: []interface{}{"Bob"}},
}
results, err := executor.ExecuteBatch(ctx, items)
```

### WAL 模式管理器

WAL（预写日志）模式可以提高并发读写性能。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// 创建 WAL 管理器
config := database.DefaultWALConfig()
manager, err := database.NewWALManager(db, config)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// WAL 模式自动启用，检查点自动管理
```

### 连接池管理器

连接池管理器提供健康检查和连接生命周期管理。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"

// 创建连接池管理器
config := database.DefaultPoolConfig()
manager := database.NewPoolManager(db, config)
defer manager.Close()

// 获取连接池统计信息
stats := manager.Stats()
fmt.Printf("活跃: %d, 空闲: %d\n", stats.ActiveConnections, stats.IdleConnections)
```

---

## 内存优化

### 缓冲池

重用字节缓冲区以减少内存分配。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pool"

// 创建缓冲池
bufPool := pool.NewBufferPool()

// 获取并使用缓冲区
buf := bufPool.Get()
buf.WriteString("Hello, World!")
data := buf.String()
bufPool.Put(buf) // 归还到池中
```

### 字节切片池

重用特定大小的字节切片。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pool"

// 创建 4KB 切片池
slicePool := pool.NewByteSlicePool(4096)

// 获取并使用切片
slice := slicePool.Get()
copy(slice, data)
slicePool.Put(slice) // 归还到池中
```

### 对象池

适用于任何类型的通用对象池。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pool"

// 为自定义类型创建池
type MyStruct struct {
    Data []byte
}

objPool := pool.NewObjectPool(
    func() *MyStruct { return &MyStruct{Data: make([]byte, 1024)} },
    func(obj *MyStruct) { obj.Data = obj.Data[:0] },
)

// 获取并使用对象
obj := objPool.Get()
// ... 使用 obj ...
objPool.Put(obj) // 归还到池中
```

---

## 缓存层

### ECache（高性能 LRU 缓存）

ZimaOS-Blue 使用 [orca-zhang/ecache](https://github.com/orca-zhang/ecache) 进行高性能内存缓存，支持 LRU 淘汰策略。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// 创建 ECache（LRU 模式）
config := cache.Config{
    MaxSize:    1000,
    DefaultTTL: 5 * time.Minute,
}
ecache := cache.NewECache(config)
defer ecache.Close()

// 基本操作
ctx := context.Background()
ecache.Set(ctx, "key", "value") // 使用配置中的默认 TTL

value, err := ecache.Get(ctx, "key")

// GetOrSet - 原子性获取或计算
value, err := ecache.GetOrSet(ctx, "key", func() (interface{}, error) {
    return computeExpensiveValue()
})

// SetNX - 仅在不存在时设置
ok := ecache.SetNX(ctx, "key", "value")

// 获取统计信息
stats := ecache.Stats()
fmt.Printf("命中: %d, 未命中: %d, 命中率: %.2f%%\n",
    stats.Hits, stats.Misses, stats.HitRate)
```

### ECache2（LRU-2 模式）

LRU-2 模式可以更好地保护频繁访问的数据免受批量操作的影响。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// 创建 ECache2（LRU-2 模式）
config := cache.Config{
    MaxSize:    1000,
    DefaultTTL: 5 * time.Minute,
}
ecache2 := cache.NewECache2(config)
defer ecache2.Close()

// 使用方式与普通 ECache 相同
ecache2.Set(ctx, "key", "value")
value, err := ecache2.Get(ctx, "key")
```

### LRU 缓存（旧版）

支持 TTL 的最近最少使用缓存。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// 创建 LRU 缓存
config := cache.Config{
    MaxSize:         1000,
    DefaultTTL:      5 * time.Minute,
    CleanupInterval: time.Minute,
}
lruCache := cache.NewLRUCache(config)
defer lruCache.Close()

// 设置和获取值
ctx := context.Background()
lruCache.Set(ctx, "key", "value", 0) // 0 = 使用默认 TTL
value, err := lruCache.Get(ctx, "key")
```

### LFU 缓存

基于频率淘汰的最不经常使用缓存。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// 创建 LFU 缓存
config := cache.Config{
    MaxSize:         1000,
    DefaultTTL:      5 * time.Minute,
    CleanupInterval: time.Minute,
}
lfuCache := cache.NewLFUCache(config)
defer lfuCache.Close()
```

### 磁盘缓存

存储在磁盘上的持久化缓存，支持可选压缩。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// 创建磁盘缓存
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

### 多级缓存

结合 L1（内存）和 L2（磁盘）缓存以获得最佳性能。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"

// 创建多级缓存
config := cache.DefaultMultiLevelConfig()
config.L1.MaxSize = 1000
config.L2.Path = "/var/cache/zimaos-blue"
config.PromoteOnHit = true  // L2 命中时提升到 L1
config.WriteThrough = true  // 同时写入 L1 和 L2

mlCache, err := cache.NewMultiLevelCache(config)
if err != nil {
    log.Fatal(err)
}
defer mlCache.Close()

// 使用方式与其他缓存相同
ctx := context.Background()
mlCache.Set(ctx, "key", "value", time.Hour)
value, err := mlCache.Get(ctx, "key")
```

---

## 并发优化

### 优化的工作池

支持任务队列的自动扩缩容工作池。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"

// 创建优化的工作池
config := worker.DefaultOptimizedPoolConfig()
config.MinWorkers = 4
config.MaxWorkers = 16
config.QueueSize = 1000

pool := worker.NewOptimizedPool(ctx, config)
defer pool.Shutdown()

// 提交任务
pool.SubmitFunc(func(ctx context.Context) error {
    // 执行工作
    return nil
})

// 带超时提交
pool.SubmitFuncWithTimeout(func(ctx context.Context) error {
    // 带超时执行工作
    return nil
}, 5*time.Second)

// 获取统计信息
stats := pool.Stats()
fmt.Printf("活跃: %d, 已完成: %d\n", stats.ActiveWorkers, stats.TasksCompleted)
```

### 分片 Map

减少锁竞争的并发 Map。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sync"

// 创建分片 Map
m := sync.NewShardedMap[string, int](32, sync.StringHashFn)

// 像普通 Map 一样使用
m.Set("key", 42)
value, ok := m.Get("key")
m.Delete("key")

// 遍历
m.Range(func(key string, value int) bool {
    fmt.Printf("%s: %d\n", key, value)
    return true // 继续遍历
})
```

### 原子工具

无锁计数器和标志。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sync"

// 原子计数器
counter := sync.NewAtomicCounter(0)
counter.Inc()
counter.Add(10)
value := counter.Value()

// 原子标志
flag := sync.NewAtomicFlag(false)
flag.Set()
if flag.IsSet() {
    // ...
}
flag.Clear()
```

---

## 网络优化

### HTTP/2 服务器

配置 HTTP/2 以提高性能。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// 创建 HTTP/2 服务器
config := network.DefaultHTTP2Config()
config.MaxConcurrentStreams = 250
config.IdleTimeout = 120 * time.Second

server := network.NewHTTP2Server(handler, config)
server.ListenAndServeTLS(":443", "cert.pem", "key.pem")
```

### 压缩中间件

自动响应压缩。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// 创建压缩中间件
config := network.DefaultCompressionConfig()
config.Level = 6
config.MinSize = 1024 // 仅压缩 > 1KB 的响应

middleware := network.CompressionMiddleware(config)
handler = middleware(handler)
```

### 请求批处理

将多个请求批量合并为单个操作。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// 创建批处理器
config := network.DefaultBatchConfig()
config.MaxBatchSize = 100
config.MaxWaitTime = 10 * time.Millisecond

loader := func(ctx context.Context, keys []string) (map[string]User, error) {
    // 一次加载多个用户
    return db.GetUsersByIDs(ctx, keys)
}

batcher := network.NewBatcher[string, User](ctx, config, loader)
defer batcher.Close()

// 加载单个项目（将与并发请求批量合并）
user, err := batcher.Load(ctx, "user-123")
```

### 请求合并

去重并发的相同请求。

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"

// 创建合并器
loader := func(ctx context.Context, key string) (User, error) {
    return db.GetUser(ctx, key)
}

coalescer := network.NewRequestCoalescer[string, User](loader, 5*time.Second)

// 对同一 key 的多个并发调用只会执行一次 loader
user, err := coalescer.Load(ctx, "user-123")
```

---

## 基准测试

### 运行基准测试

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/benchmark"

// 创建基准测试套件
suite := benchmark.NewSuite("我的基准测试")

// 添加基准测试
suite.Add("cache-get", func(ctx context.Context) error {
    _, err := cache.Get(ctx, "key")
    return err
})

suite.Add("cache-set", func(ctx context.Context) error {
    return cache.Set(ctx, "key", "value", 0)
})

// 运行基准测试
config := benchmark.DefaultConfig()
config.Duration = 5 * time.Second
config.Warmup = 100

err := suite.Run(ctx, config)
if err != nil {
    log.Fatal(err)
}

// 打印报告
fmt.Println(suite.Report())

// 保存结果
suite.SaveJSON("benchmark-results.json")
```

### 比较结果

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/benchmark"

// 比较两个结果
comparison := benchmark.Compare(baseline, current)

// 检查是否有性能回退
if comparison.IsRegression(5.0) { // 5% 阈值
    fmt.Println("检测到性能回退！")
}

// 打印比较结果
fmt.Println(comparison.Report())
```

---

## 配置

### 性能配置

添加到你的 `config.yaml`：

```yaml
performance:
  # 数据库优化
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

  # 内存优化
  memory:
    gogc: 100
    gomemlimit: 512MB
    buffer_pool_size: 1000
    object_pool_size: 500

  # 并发优化
  concurrency:
    worker_pool_size: 100
    max_goroutines: 10000
    channel_buffer_size: 100

  # 网络优化
  network:
    http2_enabled: true
    keep_alive_timeout: 30s
    compression_enabled: true
    compression_level: 6
    request_timeout: 30s
    retry_max_attempts: 3
    retry_backoff_base: 100ms

  # 缓存配置
  cache:
    l1_enabled: true
    l1_size: 1000
    l1_ttl: 5m
    l2_enabled: true
    l2_path: ./cache
    l2_size: 100MB
    l2_ttl: 1h
    eviction_policy: lru

  # 性能分析
  profiling:
    pprof_enabled: true
    pprof_path: /debug/pprof
    metrics_enabled: true
    benchmark_enabled: false
```

---

## 最佳实践

### 1. 对频繁分配的对象使用对象池

```go
// 好：重用缓冲区
buf := bufPool.Get()
defer bufPool.Put(buf)

// 差：每次都分配新缓冲区
buf := new(bytes.Buffer)
```

### 2. 在高并发场景使用分片 Map

```go
// 好：分片 Map 减少锁竞争
m := sync.NewShardedMap[string, int](32, sync.StringHashFn)

// 差：高竞争下使用 sync.Map 或互斥锁保护的 Map
var m sync.Map
```

### 3. 批量数据库操作

```go
// 好：批量插入
executor.ExecuteBatch(ctx, items)

// 差：循环中单独插入
for _, item := range items {
    db.Exec(ctx, "INSERT ...", item)
}
```

### 4. 使用多级缓存

```go
// 好：L1（内存）+ L2（磁盘）缓存
mlCache, _ := cache.NewMultiLevelCache(config)

// 差：仅使用内存缓存（重启后数据丢失）
memCache := cache.NewLRUCache(config)
```

### 5. 启用 HTTP/2 和压缩

```yaml
# 好：启用 HTTP/2 和压缩
network:
  http2_enabled: true
  compression_enabled: true
```

### 6. 监控性能指标

```go
// 启用 Prometheus 指标
metrics := metrics.NewPerformanceMetrics(registry)

// 记录缓存操作
metrics.RecordCacheHit("main", "l1")
metrics.RecordCacheLatency("main", "get", duration)
```

---

## 故障排除

### 内存使用过高

1. 检查 GOGC 设置（值越低 = GC 越频繁）
2. 检查对象池使用情况
3. 检查缓存大小
4. 使用 pprof 识别内存泄漏

### CPU 使用过高

1. 检查 goroutine 数量
2. 使用 pprof 检查锁竞争
3. 检查是否有忙循环
4. 检查数据库查询效率

### 数据库查询慢

1. 启用查询优化器
2. 检查是否缺少索引
3. 使用批量操作
4. 启用 WAL 模式
5. 检查连接池设置

### 网络延迟

1. 启用 HTTP/2
2. 启用压缩
3. 使用请求批处理
4. 检查 keep-alive 设置
