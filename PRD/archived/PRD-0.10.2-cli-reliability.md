# v0.10.2 Claude Code CLI 调用可靠性提升

[English Version](../i18n/en/PRD/v0.10-cli-reliability.md)

## 概述

本 PRD 定义了提升 Claude Code CLI 调用成功率的策略和实现方案。目标是通过智能重试、响应缓存、错误检测与恢复等机制，将 CLI 调用成功率从当前水平提升至 99%+。

## 背景

### 当前问题

1. **无重试机制** - 瞬时错误（网络抖动、API 限流）直接导致失败
2. **无响应缓存** - 相同请求重复调用 CLI，浪费资源
3. **错误分类不足** - 无法区分可重试错误和永久性错误
4. **缺乏健康检查** - 无法提前发现 CLI 不可用
5. **无熔断保护** - 连续失败时继续尝试，加剧问题
6. **监控不足** - 缺乏调用成功率、延迟等指标

### 现有错误类型

```go
// 当前已定义的错误类型
ErrInvalidConfig    // 配置验证错误
ErrCliExecution     // CLI 执行错误（含 exit code 和 stderr）
ErrTimeout          // 超时错误
ErrSessionNotFound  // 会话未找到
ErrParseOutput      // 输出解析错误
```

## 目标

1. **提升成功率** - CLI 调用成功率达到 99%+
2. **减少延迟** - 通过缓存减少重复调用
3. **快速恢复** - 自动从瞬时错误中恢复
4. **优雅降级** - 在 CLI 不可用时提供备选方案
5. **可观测性** - 提供完整的监控和告警能力

## 技术设计

### 1. 智能重试机制

#### 1.1 错误分类

```go
// ErrorCategory 错误分类
type ErrorCategory int

const (
    // CategoryRetryable 可重试错误
    CategoryRetryable ErrorCategory = iota
    // CategoryNonRetryable 不可重试错误
    CategoryNonRetryable
    // CategoryRateLimited 限流错误
    CategoryRateLimited
    // CategoryTimeout 超时错误
    CategoryTimeout
)

// ClassifyError 对错误进行分类
func ClassifyError(err error) ErrorCategory {
    // 根据错误类型和内容判断分类
}
```

#### 1.2 重试策略

| 错误类型 | 是否重试 | 重试策略 | 最大重试次数 |
|---------|---------|---------|-------------|
| 网络错误 | ✓ | 指数退避 | 3 |
| 超时错误 | ✓ | 线性退避 | 2 |
| 限流 (429) | ✓ | 固定延迟 | 5 |
| 解析错误 | ✗ | - | 0 |
| 配置错误 | ✗ | - | 0 |
| 认证错误 | ✗ | - | 0 |

#### 1.3 退避算法

```go
// RetryConfig 重试配置
type RetryConfig struct {
    MaxRetries      int           // 最大重试次数
    InitialDelay    time.Duration // 初始延迟
    MaxDelay        time.Duration // 最大延迟
    Multiplier      float64       // 退避乘数
    Jitter          float64       // 抖动因子 (0-1)
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
    MaxRetries:   3,
    InitialDelay: 500 * time.Millisecond,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1,
}
```

### 2. 响应缓存

#### 2.1 缓存策略

```go
// CacheConfig 缓存配置
type CacheConfig struct {
    Enabled     bool          // 是否启用缓存
    MaxSize     int           // 最大缓存条目数
    TTL         time.Duration // 缓存过期时间
    KeyStrategy CacheKeyStrategy // 缓存键策略
}

// CacheKeyStrategy 缓存键生成策略
type CacheKeyStrategy int

const (
    // KeyByPromptHash 按 prompt 哈希
    KeyByPromptHash CacheKeyStrategy = iota
    // KeyByPromptAndModel 按 prompt + model
    KeyByPromptAndModel
    // KeyByFullRequest 按完整请求
    KeyByFullRequest
)
```

#### 2.2 缓存键生成

```go
// GenerateCacheKey 生成缓存键
func GenerateCacheKey(params *RunParams, strategy CacheKeyStrategy) string {
    switch strategy {
    case KeyByPromptHash:
        return sha256(params.Prompt)[:16]
    case KeyByPromptAndModel:
        return sha256(params.Prompt + params.Model)[:16]
    case KeyByFullRequest:
        return sha256(serialize(params))[:16]
    }
}
```

#### 2.3 缓存存储

| 存储类型 | 适用场景 | 容量 | 持久化 |
|---------|---------|------|--------|
| 内存缓存 | 短期缓存 | 1000 条 | ✗ |
| 文件缓存 | 长期缓存 | 10GB | ✓ |
| Redis | 分布式 | 无限 | ✓ |

### 3. 健康检查

#### 3.1 检查类型

```go
// HealthCheck 健康检查
type HealthCheck struct {
    // Binary 检查 CLI 二进制是否可用
    Binary bool
    // Version 检查版本是否匹配
    Version bool
    // Connectivity 检查网络连通性
    Connectivity bool
    // Authentication 检查认证是否有效
    Authentication bool
}

// HealthStatus 健康状态
type HealthStatus struct {
    Healthy     bool
    LastCheck   time.Time
    Checks      map[string]CheckResult
    Message     string
}
```

#### 3.2 检查频率

| 检查项 | 频率 | 超时 | 失败阈值 |
|-------|------|------|---------|
| Binary | 启动时 | 5s | 1 |
| Version | 每小时 | 10s | 3 |
| Connectivity | 每分钟 | 5s | 3 |
| Authentication | 每 5 分钟 | 10s | 2 |

### 4. 熔断器

#### 4.1 熔断状态

```
┌─────────┐     失败率 > 阈值     ┌─────────┐
│  Closed │ ──────────────────▶ │  Open   │
│ (正常)  │                      │ (熔断)  │
└─────────┘                      └─────────┘
     ▲                                │
     │                                │ 等待超时
     │         ┌─────────────┐        │
     │         │ Half-Open   │ ◀──────┘
     └─────────│ (半开)      │
    成功       └─────────────┘
```

#### 4.2 熔断配置

```go
// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
    FailureThreshold   int           // 失败阈值
    SuccessThreshold   int           // 成功阈值（半开状态）
    Timeout            time.Duration // 熔断超时
    HalfOpenMaxCalls   int           // 半开状态最大调用数
}

// DefaultCircuitBreakerConfig 默认熔断配置
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
    FailureThreshold: 5,
    SuccessThreshold: 2,
    Timeout:          30 * time.Second,
    HalfOpenMaxCalls: 3,
}
```

### 5. 错误检测与恢复

#### 5.1 常见错误模式

| 错误模式 | 检测方式 | 恢复策略 |
|---------|---------|---------|
| CLI 崩溃 | exit code != 0 | 重启 CLI |
| 输出截断 | JSON 解析失败 | 增加超时重试 |
| 会话丢失 | session not found | 创建新会话 |
| 内存溢出 | OOM 信号 | 限制输入大小 |
| 死锁 | 超时无响应 | 强制终止进程 |

#### 5.2 自动恢复流程

```
错误发生
    │
    ▼
┌─────────────┐
│ 错误分类    │
└─────────────┘
    │
    ├─── 可重试 ───▶ 执行重试策略
    │
    ├─── 限流 ─────▶ 等待后重试
    │
    ├─── 超时 ─────▶ 增加超时重试
    │
    └─── 不可重试 ─▶ 返回错误
```

### 6. 监控与指标

#### 6.1 核心指标

```go
// Metrics CLI 调用指标
type Metrics struct {
    // 调用统计
    TotalCalls      int64   // 总调用次数
    SuccessfulCalls int64   // 成功次数
    FailedCalls     int64   // 失败次数
    RetriedCalls    int64   // 重试次数

    // 延迟统计
    AvgLatency      time.Duration // 平均延迟
    P50Latency      time.Duration // P50 延迟
    P95Latency      time.Duration // P95 延迟
    P99Latency      time.Duration // P99 延迟

    // 缓存统计
    CacheHits       int64   // 缓存命中
    CacheMisses     int64   // 缓存未命中

    // 熔断统计
    CircuitOpens    int64   // 熔断次数
}
```

#### 6.2 告警规则

| 指标 | 阈值 | 告警级别 |
|-----|------|---------|
| 成功率 | < 95% | Warning |
| 成功率 | < 90% | Critical |
| P95 延迟 | > 30s | Warning |
| P99 延迟 | > 60s | Critical |
| 熔断状态 | Open | Critical |

## 配置

### config.yaml

```yaml
claudecode:
  enabled: true

  # 可靠性配置
  reliability:
    # 重试配置
    retry:
      enabled: true
      max_retries: 3
      initial_delay: "500ms"
      max_delay: "30s"
      multiplier: 2.0
      jitter: 0.1

    # 缓存配置
    cache:
      enabled: true
      max_size: 1000
      ttl: "1h"
      key_strategy: "prompt_and_model"
      storage: "memory"  # memory | file | redis

    # 健康检查配置
    health_check:
      enabled: true
      interval: "1m"
      timeout: "5s"

    # 熔断器配置
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      success_threshold: 2
      timeout: "30s"

    # 监控配置
    metrics:
      enabled: true
      export_interval: "10s"
      prometheus_endpoint: "/metrics"
```

### 环境变量

| 变量 | 描述 | 默认值 |
|-----|------|--------|
| `BLUE_CC_RETRY_ENABLED` | 启用重试 | `true` |
| `BLUE_CC_RETRY_MAX` | 最大重试次数 | `3` |
| `BLUE_CC_CACHE_ENABLED` | 启用缓存 | `true` |
| `BLUE_CC_CACHE_TTL` | 缓存 TTL | `1h` |
| `BLUE_CC_CB_ENABLED` | 启用熔断器 | `true` |
| `BLUE_CC_CB_THRESHOLD` | 熔断阈值 | `5` |

## API 端点

### 健康检查

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/claudecode/health` | 获取健康状态 |
| POST | `/api/v1/claudecode/health/check` | 触发健康检查 |

### 指标

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/claudecode/metrics` | 获取调用指标 |
| GET | `/api/v1/claudecode/metrics/history` | 获取历史指标 |
| POST | `/api/v1/claudecode/metrics/reset` | 重置指标 |

### 缓存管理

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/claudecode/cache/stats` | 获取缓存统计 |
| POST | `/api/v1/claudecode/cache/clear` | 清空缓存 |
| DELETE | `/api/v1/claudecode/cache/:key` | 删除指定缓存 |

### 熔断器

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/claudecode/circuit` | 获取熔断状态 |
| POST | `/api/v1/claudecode/circuit/reset` | 重置熔断器 |

## 实现清单

### Phase 1: 错误分类与重试 (Week 1)

- [ ] 实现错误分类器 `ErrorClassifier`
- [ ] 实现重试策略 `RetryPolicy`
- [ ] 实现指数退避算法
- [ ] 添加重试相关配置
- [ ] 编写单元测试

### Phase 2: 响应缓存 (Week 2)

- [ ] 实现缓存键生成器
- [ ] 实现内存缓存 `MemoryCache`
- [ ] 实现文件缓存 `FileCache`
- [ ] 添加缓存统计
- [ ] 编写单元测试

### Phase 3: 健康检查 (Week 3)

- [ ] 实现健康检查器 `HealthChecker`
- [ ] 实现各类检查（Binary, Version, Connectivity）
- [ ] 添加健康检查 API
- [ ] 实现定期检查调度
- [ ] 编写单元测试

### Phase 4: 熔断器 (Week 4)

- [ ] 实现熔断器 `CircuitBreaker`
- [ ] 实现状态转换逻辑
- [ ] 添加熔断器 API
- [ ] 集成到 Runner
- [ ] 编写单元测试

### Phase 5: 监控与告警 (Week 5)

- [ ] 实现指标收集器 `MetricsCollector`
- [ ] 添加 Prometheus 导出
- [ ] 实现告警规则
- [ ] 添加监控 API
- [ ] 编写集成测试

### Phase 6: 集成与优化 (Week 6)

- [ ] 集成所有组件到 Runner
- [ ] 性能测试与优化
- [ ] 文档更新
- [ ] 端到端测试

## 成功指标

| 指标 | 当前值 | 目标值 |
|-----|--------|--------|
| 调用成功率 | ~95% | 99%+ |
| 平均延迟 | ~5s | < 3s |
| P99 延迟 | ~30s | < 15s |
| 缓存命中率 | 0% | > 30% |
| MTTR | 手动 | < 30s |

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|-----|------|---------|
| 缓存一致性 | 返回过期数据 | 设置合理 TTL，支持手动清除 |
| 重试风暴 | 加剧 API 压力 | 使用抖动，限制并发重试 |
| 熔断误触发 | 服务不可用 | 调整阈值，支持手动重置 |
| 内存占用 | OOM | 限制缓存大小，使用 LRU |

## 参考

- [v0.10 Claude Code CLI Bundling](v0.10-claude-code-bundling.md)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Exponential Backoff](https://en.wikipedia.org/wiki/Exponential_backoff)
