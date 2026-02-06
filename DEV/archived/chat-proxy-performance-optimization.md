# Chat → Proxy Handler 性能优化计划

## 当前调用链路分析

```
ChatHandler.SendMessage()
  ↓
getDefaultProvider() / getProviderFromPool()
  ↓
ProxyHandler.HandleRequest()
  ↓
CacheHandler (检查缓存)
  ↓
APIHandler (调用 LLM API)
  ↓
ResponseHandler (处理响应)
```

## 性能瓶颈识别

### 1. **Provider 查询开销**
- 每次请求都需要从 ProviderRegistry 查询
- 可能涉及多次 map 查询和锁操作
- **优化方案**: 缓存 Provider 引用，减少查询次数

### 2. **缓存检查开销**
- 每次请求都要检查缓存
- 缓存 key 生成可能涉及复杂计算
- **优化方案**: 预计算缓存 key，使用快速哈希

### 3. **内存分配**
- 每次请求创建新的 Request/Response 对象
- 字符串拼接和 JSON 序列化
- **优化方案**: 对象池复用，减少 GC 压力

### 4. **并发锁竞争**
- 多个 goroutine 竞争 Provider 锁
- 缓存锁竞争
- **优化方案**: 使用 RWMutex，减少写锁时间

### 5. **上下文传递**
- 每层都需要传递 context
- 可能涉及多次 context 检查
- **优化方案**: 使用 context 值缓存

## 优化策略

### Phase 1: 快速优化（立即实施）
1. **Provider 缓存层**
   - 在 ChatHandler 中缓存最近使用的 Provider
   - 使用 LRU 缓存，容量 10-20

2. **缓存 Key 预计算**
   - 在请求开始时计算一次
   - 避免重复计算

3. **对象池**
   - 为 Request/Response 创建对象池
   - 使用 sync.Pool 管理

### Phase 2: 中期优化（1-2 周）
1. **批量请求处理**
   - 支持批量 API 调用
   - 减少网络往返

2. **流式响应优化**
   - 优化 SSE 流式传输
   - 减少缓冲区复制

3. **并发优化**
   - 使用 sync.Map 替代某些 map
   - 优化锁粒度

### Phase 3: 长期优化（2-4 周）
1. **请求管道化**
   - 预加载下一个请求
   - 并行处理多个请求

2. **智能缓存策略**
   - 基于访问模式的缓存预热
   - 自适应缓存大小

3. **性能监控**
   - 添加详细的性能指标
   - 实时性能告警

## 实施步骤

### 步骤 1: 添加 Provider 缓存
```go
type ChatHandler struct {
    // ... 现有字段
    providerCache *lru.Cache  // LRU 缓存最近使用的 Provider
    providerCacheMu sync.RWMutex
}
```

### 步骤 2: 实现对象池
```go
var requestPool = sync.Pool{
    New: func() interface{} {
        return &ChatRequest{}
    },
}
```

### 步骤 3: 优化缓存 Key 生成
```go
// 预计算缓存 key
cacheKey := generateCacheKey(userID, conversationID, messageHash)
```

### 步骤 4: 性能测试
- 基准测试对比
- 负载测试（1000+ 并发）
- 内存使用监控

## 预期性能提升

| 优化项 | 预期提升 | 优先级 |
|------|--------|------|
| Provider 缓存 | 10-15% | 高 |
| 对象池 | 20-30% | 高 |
| 缓存 Key 优化 | 5-10% | 中 |
| 并发优化 | 15-25% | 中 |
| 流式优化 | 10-20% | 低 |

**总体预期**: 50-100% 性能提升

## 监控指标

1. **响应时间**
   - P50, P95, P99 延迟
   - 平均响应时间

2. **吞吐量**
   - 每秒请求数 (RPS)
   - 并发连接数

3. **资源使用**
   - 内存使用量
   - CPU 使用率
   - GC 暂停时间

4. **缓存效率**
   - 缓存命中率
   - 缓存大小

## 风险评估

- **兼容性**: 确保优化不破坏现有 API
- **稳定性**: 充分测试，避免引入 bug
- **可维护性**: 保持代码清晰，添加注释

## 时间表

- **Week 1**: Phase 1 实施 + 测试
- **Week 2-3**: Phase 2 实施 + 性能验证
- **Week 4+**: Phase 3 实施 + 持续优化
