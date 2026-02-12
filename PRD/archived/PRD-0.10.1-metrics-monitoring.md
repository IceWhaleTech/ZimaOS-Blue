# v0.10.1 用量统计与系统监控

[English Version](../i18n/en/PRD/v0.10-metrics-monitoring.md)

## 概述

本 PRD 定义了 ZimaOS-Blue 的用量统计和系统监控功能。目标是提供全面的 API 调用统计、性能指标、Token 用量追踪以及 CLI 进程的系统资源监控。

## 目标

1. **用量可视化** - 直观展示 API 调用量、Token 消耗
2. **性能监控** - 追踪响应时间、生成速率等关键指标
3. **资源监控** - 监控 CLI 进程的 CPU、内存使用
4. **成本控制** - 帮助用户了解和控制 API 使用成本
5. **问题诊断** - 通过指标快速定位性能问题

## 功能需求

### 1. API 调用统计

#### 1.1 基础统计

```go
// CallStats API 调用统计
type CallStats struct {
    // 调用计数
    TotalCalls      int64 `json:"total_calls"`       // 总调用次数
    SuccessfulCalls int64 `json:"successful_calls"`  // 成功次数
    FailedCalls     int64 `json:"failed_calls"`      // 失败次数

    // 按状态分类
    TimeoutCalls    int64 `json:"timeout_calls"`     // 超时次数
    RateLimitCalls  int64 `json:"rate_limit_calls"`  // 限流次数
    ErrorCalls      int64 `json:"error_calls"`       // 错误次数

    // 成功率
    SuccessRate     float64 `json:"success_rate"`    // 成功率 (%)
}
```

#### 1.2 时间维度统计

| 维度 | 描述 | 保留时间 |
|-----|------|---------|
| 实时 | 最近 1 分钟 | 1 小时 |
| 小时 | 每小时聚合 | 24 小时 |
| 日 | 每日聚合 | 30 天 |
| 月 | 每月聚合 | 12 个月 |

#### 1.3 按模型统计

```go
// ModelStats 按模型统计
type ModelStats struct {
    Model           string  `json:"model"`
    Calls           int64   `json:"calls"`
    SuccessfulCalls int64   `json:"successful_calls"`
    FailedCalls     int64   `json:"failed_calls"`
    SuccessRate     float64 `json:"success_rate"`

    // 延迟统计
    AvgLatency      float64 `json:"avg_latency_ms"`
    P50Latency      float64 `json:"p50_latency_ms"`
    P95Latency      float64 `json:"p95_latency_ms"`
    P99Latency      float64 `json:"p99_latency_ms"`

    // Token 统计
    InputTokens     int64   `json:"input_tokens"`
    OutputTokens    int64   `json:"output_tokens"`
    TotalTokens     int64   `json:"total_tokens"`
    CacheReadTokens int64   `json:"cache_read_tokens"`
    CacheWriteTokens int64  `json:"cache_write_tokens"`

    // 性能统计
    AvgTokensPerSecond float64 `json:"avg_tokens_per_second"`
    AvgTimeToFirstToken float64 `json:"avg_ttft_ms"`

    // 成本统计
    EstimatedCost   float64 `json:"estimated_cost"`
}

// ModelStatsHistory 模型统计历史
type ModelStatsHistory struct {
    Model     string           `json:"model"`
    Period    string           `json:"period"`      // hourly, daily, monthly
    DataPoints []ModelDataPoint `json:"data_points"`
}

// ModelDataPoint 模型数据点
type ModelDataPoint struct {
    Timestamp       time.Time `json:"timestamp"`
    Calls           int64     `json:"calls"`
    SuccessRate     float64   `json:"success_rate"`
    AvgLatency      float64   `json:"avg_latency_ms"`
    TotalTokens     int64     `json:"total_tokens"`
    EstimatedCost   float64   `json:"estimated_cost"`
}
```

#### 1.4 模型对比分析

```go
// ModelComparison 模型对比
type ModelComparison struct {
    Period          string              `json:"period"`
    Models          []ModelStats        `json:"models"`
    Recommendation  *ModelRecommendation `json:"recommendation,omitempty"`
}

// ModelRecommendation 模型推荐
type ModelRecommendation struct {
    BestPerformance string `json:"best_performance"` // 性能最佳
    BestCostValue   string `json:"best_cost_value"`  // 性价比最佳
    MostReliable    string `json:"most_reliable"`    // 最稳定
    Fastest         string `json:"fastest"`          // 最快响应
}
```

### 2. Token 用量统计

#### 2.1 Token 计数

```go
// TokenUsage Token 用量
type TokenUsage struct {
    // 输入输出
    InputTokens     int64 `json:"input_tokens"`      // 输入 Token
    OutputTokens    int64 `json:"output_tokens"`     // 输出 Token
    TotalTokens     int64 `json:"total_tokens"`      // 总 Token

    // 缓存相关
    CacheReadTokens  int64 `json:"cache_read_tokens"`  // 缓存读取
    CacheWriteTokens int64 `json:"cache_write_tokens"` // 缓存写入

    // 成本估算 (USD)
    EstimatedCost   float64 `json:"estimated_cost"`
}
```

#### 2.2 Token 价格配置

价格采用通用匹配机制，忽略版本号和日期标识，尽可能匹配模型名称。

```go
// TokenPricing Token 价格配置
type TokenPricing struct {
    // 模型匹配模式 (支持通配符)
    Pattern     string  `json:"pattern"`      // e.g., "opus*", "sonnet*", "haiku*"
    InputPrice  float64 `json:"input_price"`  // 每百万 Token 价格 (USD)
    OutputPrice float64 `json:"output_price"` // 每百万 Token 价格 (USD)
    CacheRead   float64 `json:"cache_read"`   // 缓存读取价格
    CacheWrite  float64 `json:"cache_write"`  // 缓存写入价格
}

// MatchModel 模型匹配函数
// 匹配规则: 忽略版本号、日期、前缀 "claude-"
// 例如: "claude-sonnet-4-5-20250929" 匹配 "sonnet"
func MatchModel(modelName string, pattern string) bool
```

**默认价格配置 (预置)**

```yaml
# Token 价格 (每百万 Token, USD)
# 匹配规则: 忽略 "claude-" 前缀和版本号/日期后缀
token_pricing:
  # Anthropic Claude 系列
  - pattern: "opus*"
    input: 5.00
    output: 25.00
    cache_read: 0.50
    cache_write: 6.25

  - pattern: "sonnet*"
    input: 3.00
    output: 15.00
    cache_read: 0.30
    cache_write: 3.75

  - pattern: "haiku*"
    input: 1.00
    output: 5.00
    cache_read: 0.10
    cache_write: 1.25

  # OpenAI GPT 系列
  - pattern: "gpt-4o*"
    input: 2.50
    output: 10.00

  - pattern: "gpt-4-turbo*"
    input: 10.00
    output: 30.00

  - pattern: "gpt-4*"
    input: 30.00
    output: 60.00

  - pattern: "gpt-3.5*"
    input: 0.50
    output: 1.50

  - pattern: "o1-preview*"
    input: 15.00
    output: 60.00

  - pattern: "o1-mini*"
    input: 3.00
    output: 12.00

  # Google Gemini 系列
  - pattern: "gemini-1.5-pro*"
    input: 1.25
    output: 5.00

  - pattern: "gemini-1.5-flash*"
    input: 0.075
    output: 0.30

  - pattern: "gemini-2.0*"
    input: 0.10
    output: 0.40

  # Meta Llama 系列 (通过 API 提供商)
  - pattern: "llama-3.1-405b*"
    input: 3.00
    output: 3.00

  - pattern: "llama-3.1-70b*"
    input: 0.88
    output: 0.88

  - pattern: "llama-3.1-8b*"
    input: 0.18
    output: 0.18

  # Mistral 系列
  - pattern: "mistral-large*"
    input: 2.00
    output: 6.00

  - pattern: "mistral-medium*"
    input: 2.70
    output: 8.10

  - pattern: "mistral-small*"
    input: 0.20
    output: 0.60

  # DeepSeek 系列
  - pattern: "deepseek-v3*"
    input: 0.27
    output: 1.10

  - pattern: "deepseek-r1*"
    input: 0.55
    output: 2.19

  # 默认 (未匹配时使用)
  - pattern: "*"
    input: 1.00
    output: 5.00
```

**模型匹配示例**

| 输入模型名 | 匹配模式 | 价格 (Input/Output) |
|-----------|---------|---------------------|
| `claude-opus-4-5-20251101` | `opus*` | $5/$25 |
| `opus` | `opus*` | $5/$25 |
| `claude-sonnet-4-5-20250929` | `sonnet*` | $3/$15 |
| `sonnet` | `sonnet*` | $3/$15 |
| `claude-3-5-haiku-20241022` | `haiku*` | $1/$5 |
| `haiku` | `haiku*` | $1/$5 |
| `gpt-4o-2024-08-06` | `gpt-4o*` | $2.50/$10 |
| `gemini-1.5-pro-latest` | `gemini-1.5-pro*` | $1.25/$5 |
| `unknown-model` | `*` (默认) | $1/$5 |

### 3. 性能指标

#### 3.1 响应时间统计

```go
// LatencyStats 延迟统计
type LatencyStats struct {
    // 基础统计
    Min     time.Duration `json:"min_ms"`
    Max     time.Duration `json:"max_ms"`
    Avg     time.Duration `json:"avg_ms"`

    // 百分位数
    P50     time.Duration `json:"p50_ms"`
    P90     time.Duration `json:"p90_ms"`
    P95     time.Duration `json:"p95_ms"`
    P99     time.Duration `json:"p99_ms"`

    // 样本数
    Samples int64         `json:"samples"`
}
```

#### 3.2 生成速率统计

```go
// GenerationSpeed 生成速率
type GenerationSpeed struct {
    // Token 生成速率
    TokensPerSecond     float64 `json:"tokens_per_second"`      // Token/s
    AvgTokensPerSecond  float64 `json:"avg_tokens_per_second"`  // 平均 Token/s
    MaxTokensPerSecond  float64 `json:"max_tokens_per_second"`  // 最大 Token/s

    // Prefill 速率 (首 Token 时间)
    PrefillSpeed        float64 `json:"prefill_speed"`          // 首 Token 速率
    TimeToFirstToken    float64 `json:"time_to_first_token_ms"` // 首 Token 时间 (ms)
    AvgTimeToFirstToken float64 `json:"avg_ttft_ms"`            // 平均首 Token 时间

    // 解码速率
    DecodeSpeed         float64 `json:"decode_speed"`           // 解码速率 Token/s
}
```

#### 3.3 时间分解

```
总响应时间 = 排队时间 + 首 Token 时间 + 生成时间

┌─────────────────────────────────────────────────────────────┐
│                        总响应时间                            │
├──────────┬──────────────────┬───────────────────────────────┤
│ 排队时间  │   首 Token 时间   │          生成时间              │
│ (Queue)  │    (TTFT)        │        (Generation)           │
└──────────┴──────────────────┴───────────────────────────────┘
```

```go
// TimeBreakdown 时间分解
type TimeBreakdown struct {
    QueueTime      time.Duration `json:"queue_time_ms"`      // 排队时间
    TimeToFirstToken time.Duration `json:"ttft_ms"`          // 首 Token 时间
    GenerationTime time.Duration `json:"generation_time_ms"` // 生成时间
    TotalTime      time.Duration `json:"total_time_ms"`      // 总时间
}
```

### 4. 系统资源监控

#### 4.1 CLI 进程监控

```go
// ProcessMetrics CLI 进程指标
type ProcessMetrics struct {
    // 进程信息
    PID         int    `json:"pid"`
    Command     string `json:"command"`
    State       string `json:"state"`       // running, sleeping, etc.

    // CPU 使用
    CPUPercent  float64 `json:"cpu_percent"`  // CPU 使用率 (%)
    CPUTime     float64 `json:"cpu_time_s"`   // CPU 时间 (秒)

    // 内存使用
    MemoryRSS   int64   `json:"memory_rss_bytes"`   // 常驻内存 (bytes)
    MemoryVMS   int64   `json:"memory_vms_bytes"`   // 虚拟内存 (bytes)
    MemoryPercent float64 `json:"memory_percent"`   // 内存使用率 (%)

    // I/O 统计
    IOReadBytes  int64  `json:"io_read_bytes"`
    IOWriteBytes int64  `json:"io_write_bytes"`

    // 线程/协程
    NumThreads  int    `json:"num_threads"`

    // 运行时间
    StartTime   time.Time `json:"start_time"`
    Uptime      time.Duration `json:"uptime"`
}
```

#### 4.2 系统资源概览

```go
// SystemMetrics 系统指标
type SystemMetrics struct {
    // CPU
    CPUCount        int     `json:"cpu_count"`
    CPUUsagePercent float64 `json:"cpu_usage_percent"`
    LoadAvg1        float64 `json:"load_avg_1"`
    LoadAvg5        float64 `json:"load_avg_5"`
    LoadAvg15       float64 `json:"load_avg_15"`

    // 内存
    MemoryTotal     int64   `json:"memory_total_bytes"`
    MemoryUsed      int64   `json:"memory_used_bytes"`
    MemoryFree      int64   `json:"memory_free_bytes"`
    MemoryPercent   float64 `json:"memory_percent"`

    // 磁盘
    DiskTotal       int64   `json:"disk_total_bytes"`
    DiskUsed        int64   `json:"disk_used_bytes"`
    DiskFree        int64   `json:"disk_free_bytes"`
    DiskPercent     float64 `json:"disk_percent"`

    // 网络
    NetworkBytesSent int64  `json:"network_bytes_sent"`
    NetworkBytesRecv int64  `json:"network_bytes_recv"`
}
```

#### 4.3 资源历史记录

```go
// ResourceHistory 资源历史
type ResourceHistory struct {
    Timestamp time.Time       `json:"timestamp"`
    CPU       float64         `json:"cpu_percent"`
    Memory    float64         `json:"memory_percent"`
    Disk      float64         `json:"disk_percent"`
}
```

### 5. 数据聚合与存储

#### 5.1 聚合策略

| 指标类型 | 聚合方式 | 存储粒度 |
|---------|---------|---------|
| 调用次数 | SUM | 分钟/小时/天 |
| 成功率 | AVG | 分钟/小时/天 |
| 延迟 | P50/P95/P99 | 分钟/小时/天 |
| Token 用量 | SUM | 小时/天/月 |
| CPU/内存 | AVG/MAX | 分钟/小时 |

#### 5.2 存储方案

```go
// MetricsStore 指标存储接口
type MetricsStore interface {
    // 写入
    Record(metric Metric) error
    RecordBatch(metrics []Metric) error

    // 查询
    Query(query MetricsQuery) ([]Metric, error)
    Aggregate(query AggregateQuery) (*AggregateResult, error)

    // 管理
    Cleanup(before time.Time) error
    Export(format string) ([]byte, error)
}
```

## API 端点

### 调用统计

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/metrics/calls` | 获取调用统计 |
| GET | `/api/v1/metrics/calls/history` | 获取调用历史 |
| GET | `/api/v1/metrics/calls/by-model` | 按模型统计 |
| GET | `/api/v1/metrics/calls/by-model/:model` | 获取指定模型统计 |
| GET | `/api/v1/metrics/calls/by-model/:model/history` | 获取指定模型历史 |
| GET | `/api/v1/metrics/models/compare` | 模型对比分析 |

### Token 用量

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/metrics/tokens` | 获取 Token 用量 |
| GET | `/api/v1/metrics/tokens/history` | 获取用量历史 |
| GET | `/api/v1/metrics/tokens/cost` | 获取成本估算 |
| GET | `/api/v1/metrics/tokens/by-model` | 按模型 Token 统计 |
| GET | `/api/v1/metrics/tokens/by-model/:model` | 指定模型 Token 统计 |

### 性能指标

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/metrics/latency` | 获取延迟统计 |
| GET | `/api/v1/metrics/latency/by-model` | 按模型延迟统计 |
| GET | `/api/v1/metrics/speed` | 获取生成速率 |
| GET | `/api/v1/metrics/speed/by-model` | 按模型生成速率 |
| GET | `/api/v1/metrics/breakdown` | 获取时间分解 |
| GET | `/api/v1/metrics/breakdown/by-model` | 按模型时间分解 |

### 系统资源

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/metrics/system` | 获取系统资源 |
| GET | `/api/v1/metrics/process` | 获取 CLI 进程指标 |
| GET | `/api/v1/metrics/process/history` | 获取进程历史 |

### 导出

| 方法 | 路径 | 描述 |
|-----|------|------|
| GET | `/api/v1/metrics/export` | 导出指标数据 |
| GET | `/metrics` | Prometheus 格式导出 |

## API 响应示例

### GET /api/v1/metrics/calls/by-model

```json
{
  "period": "last_24h",
  "models": [
    {
      "model": "opus",
      "calls": 523,
      "successful_calls": 512,
      "failed_calls": 11,
      "success_rate": 97.9,
      "avg_latency_ms": 3245,
      "p50_latency_ms": 2890,
      "p95_latency_ms": 5670,
      "p99_latency_ms": 8900,
      "input_tokens": 1000000,
      "output_tokens": 500000,
      "total_tokens": 1500000,
      "cache_read_tokens": 200000,
      "cache_write_tokens": 50000,
      "avg_tokens_per_second": 38.5,
      "avg_ttft_ms": 345,
      "estimated_cost": 8.25
    },
    {
      "model": "sonnet",
      "calls": 856,
      "successful_calls": 842,
      "failed_calls": 14,
      "success_rate": 98.4,
      "avg_latency_ms": 1856,
      "p50_latency_ms": 1650,
      "p95_latency_ms": 3200,
      "p99_latency_ms": 4500,
      "input_tokens": 1456789,
      "output_tokens": 734567,
      "total_tokens": 2191356,
      "cache_read_tokens": 256789,
      "cache_write_tokens": 73456,
      "avg_tokens_per_second": 52.3,
      "avg_ttft_ms": 234,
      "estimated_cost": 4.20
    },
    {
      "model": "haiku",
      "calls": 144,
      "successful_calls": 143,
      "failed_calls": 1,
      "success_rate": 99.3,
      "avg_latency_ms": 856,
      "p50_latency_ms": 780,
      "p95_latency_ms": 1200,
      "p99_latency_ms": 1800,
      "input_tokens": 234567,
      "output_tokens": 123456,
      "total_tokens": 358023,
      "cache_read_tokens": 45678,
      "cache_write_tokens": 12345,
      "avg_tokens_per_second": 78.9,
      "avg_ttft_ms": 123,
      "estimated_cost": 0.15
    }
  ],
  "summary": {
    "total_calls": 1523,
    "total_tokens": 4049379,
    "total_cost": 12.60
  }
}
```

### GET /api/v1/metrics/calls/by-model/:model/history

```json
{
  "model": "sonnet",
  "period": "last_24h",
  "granularity": "hourly",
  "data_points": [
    {
      "timestamp": "2026-01-28T00:00:00Z",
      "calls": 32,
      "success_rate": 96.9,
      "avg_latency_ms": 1923,
      "total_tokens": 89234,
      "estimated_cost": 0.18
    },
    {
      "timestamp": "2026-01-28T01:00:00Z",
      "calls": 28,
      "success_rate": 100.0,
      "avg_latency_ms": 1756,
      "total_tokens": 76543,
      "estimated_cost": 0.15
    }
  ]
}
```

### GET /api/v1/metrics/models/compare

```json
{
  "period": "last_7d",
  "models": [
    {
      "model": "opus",
      "calls": 3521,
      "success_rate": 97.8,
      "avg_latency_ms": 3456,
      "avg_tokens_per_second": 36.2,
      "avg_ttft_ms": 378,
      "total_tokens": 12500000,
      "estimated_cost": 68.75
    },
    {
      "model": "sonnet",
      "calls": 5892,
      "success_rate": 98.5,
      "avg_latency_ms": 1923,
      "avg_tokens_per_second": 48.7,
      "avg_ttft_ms": 256,
      "total_tokens": 18900000,
      "estimated_cost": 36.20
    },
    {
      "model": "haiku",
      "calls": 1245,
      "success_rate": 99.1,
      "avg_latency_ms": 923,
      "avg_tokens_per_second": 72.3,
      "avg_ttft_ms": 134,
      "total_tokens": 3200000,
      "estimated_cost": 1.28
    }
  ],
  "recommendation": {
    "best_performance": "opus",
    "best_cost_value": "haiku",
    "most_reliable": "haiku",
    "fastest": "haiku"
  },
  "insights": [
    "Sonnet 提供了性能和成本的最佳平衡",
    "Haiku 在简单任务上响应最快且最稳定",
    "Opus 适合需要最高质量输出的复杂任务"
  ]
}
```

### GET /api/v1/metrics/speed/by-model

```json
{
  "period": "last_24h",
  "models": [
    {
      "model": "opus",
      "current": {
        "tokens_per_second": 35.2,
        "time_to_first_token_ms": 389,
        "decode_speed": 42.1
      },
      "average": {
        "tokens_per_second": 36.2,
        "time_to_first_token_ms": 378,
        "decode_speed": 43.5
      },
      "percentiles": {
        "tps_p50": 37.1,
        "tps_p95": 45.6,
        "tps_p99": 52.3,
        "ttft_p50_ms": 356,
        "ttft_p95_ms": 567,
        "ttft_p99_ms": 890
      }
    },
    {
      "model": "sonnet",
      "current": {
        "tokens_per_second": 48.9,
        "time_to_first_token_ms": 245,
        "decode_speed": 56.7
      },
      "average": {
        "tokens_per_second": 48.7,
        "time_to_first_token_ms": 256,
        "decode_speed": 55.2
      },
      "percentiles": {
        "tps_p50": 49.2,
        "tps_p95": 62.3,
        "tps_p99": 71.5,
        "ttft_p50_ms": 234,
        "ttft_p95_ms": 389,
        "ttft_p99_ms": 567
      }
    },
    {
      "model": "haiku",
      "current": {
        "tokens_per_second": 75.6,
        "time_to_first_token_ms": 128,
        "decode_speed": 82.3
      },
      "average": {
        "tokens_per_second": 72.3,
        "time_to_first_token_ms": 134,
        "decode_speed": 78.9
      },
      "percentiles": {
        "tps_p50": 73.5,
        "tps_p95": 89.2,
        "tps_p99": 98.7,
        "ttft_p50_ms": 123,
        "ttft_p95_ms": 198,
        "ttft_p99_ms": 289
      }
    }
  ]
}
```

### GET /api/v1/metrics/calls

```json
{
  "period": "last_24h",
  "stats": {
    "total_calls": 1523,
    "successful_calls": 1489,
    "failed_calls": 34,
    "timeout_calls": 12,
    "rate_limit_calls": 8,
    "error_calls": 14,
    "success_rate": 97.77
  },
  "by_hour": [
    {"hour": "2026-01-28T00:00:00Z", "calls": 45, "success_rate": 97.8},
    {"hour": "2026-01-28T01:00:00Z", "calls": 52, "success_rate": 98.1}
  ]
}
```

### GET /api/v1/metrics/tokens

```json
{
  "period": "last_24h",
  "usage": {
    "input_tokens": 2456789,
    "output_tokens": 1234567,
    "total_tokens": 3691356,
    "cache_read_tokens": 456789,
    "cache_write_tokens": 123456,
    "estimated_cost": 12.45
  },
  "by_model": [
    {
      "model": "opus",
      "input_tokens": 1000000,
      "output_tokens": 500000,
      "estimated_cost": 8.25
    },
    {
      "model": "sonnet",
      "input_tokens": 1456789,
      "output_tokens": 734567,
      "estimated_cost": 4.20
    }
  ]
}
```

### GET /api/v1/metrics/speed

```json
{
  "current": {
    "tokens_per_second": 45.6,
    "time_to_first_token_ms": 234,
    "decode_speed": 52.3
  },
  "average": {
    "tokens_per_second": 42.1,
    "time_to_first_token_ms": 287,
    "decode_speed": 48.7
  },
  "percentiles": {
    "ttft_p50_ms": 245,
    "ttft_p95_ms": 456,
    "ttft_p99_ms": 789,
    "tps_p50": 43.2,
    "tps_p95": 56.7,
    "tps_p99": 62.1
  }
}
```

### GET /api/v1/metrics/process

```json
{
  "processes": [
    {
      "pid": 12345,
      "command": "claude",
      "state": "running",
      "cpu_percent": 15.3,
      "cpu_time_s": 123.45,
      "memory_rss_bytes": 268435456,
      "memory_vms_bytes": 536870912,
      "memory_percent": 3.2,
      "num_threads": 8,
      "uptime": "2h15m30s"
    }
  ],
  "summary": {
    "total_processes": 1,
    "total_cpu_percent": 15.3,
    "total_memory_bytes": 268435456
  }
}
```

### GET /api/v1/metrics/system

```json
{
  "cpu": {
    "count": 8,
    "usage_percent": 23.5,
    "load_avg_1": 1.23,
    "load_avg_5": 1.45,
    "load_avg_15": 1.67
  },
  "memory": {
    "total_bytes": 17179869184,
    "used_bytes": 8589934592,
    "free_bytes": 8589934592,
    "percent": 50.0
  },
  "disk": {
    "total_bytes": 512110190592,
    "used_bytes": 256055095296,
    "free_bytes": 256055095296,
    "percent": 50.0
  }
}
```

## UI 设计

### 仪表盘概览

```
┌─────────────────────────────────────────────────────────────────────┐
│  ZimaOS Blue - 监控仪表盘                                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │ 总调用次数    │  │ 成功率       │  │ Token 用量   │  │ 预估成本  │ │
│  │    1,523     │  │   97.8%      │  │   3.69M      │  │  $12.45  │ │
│  │  ↑ 12.3%     │  │  ↑ 0.5%      │  │  ↑ 8.2%      │  │  ↑ 5.1%  │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────┘ │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  调用趋势 (24h)                                                  ││
│  │  ▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁   ││
│  │  00:00        06:00        12:00        18:00        24:00      ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌────────────────────────────┐  ┌────────────────────────────────┐ │
│  │  性能指标                   │  │  系统资源                       │ │
│  │                            │  │                                │ │
│  │  平均延迟:     2.3s        │  │  CPU:    ████░░░░░░  23.5%     │ │
│  │  首Token时间:  287ms       │  │  内存:   █████░░░░░  50.0%     │ │
│  │  生成速率:     42.1 tok/s  │  │  磁盘:   █████░░░░░  50.0%     │ │
│  │  P95延迟:      4.5s        │  │                                │ │
│  └────────────────────────────┘  └────────────────────────────────┘ │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  CLI 进程监控                                                    ││
│  │  ┌─────────┬────────┬─────────┬─────────┬─────────┬──────────┐ ││
│  │  │ PID     │ 状态   │ CPU %   │ 内存    │ 线程    │ 运行时间  │ ││
│  │  ├─────────┼────────┼─────────┼─────────┼─────────┼──────────┤ ││
│  │  │ 12345   │ 运行中 │ 15.3%   │ 256MB   │ 8       │ 2h15m    │ ││
│  │  └─────────┴────────┴─────────┴─────────┴─────────┴──────────┘ ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 模型统计页面

```
┌─────────────────────────────────────────────────────────────────────┐
│  模型统计                                           [日] [周] [月]   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  模型对比                                                        ││
│  │                                                                  ││
│  │  ┌─────────┬────────┬─────────┬─────────┬─────────┬──────────┐ ││
│  │  │ 模型    │ 调用数 │ 成功率  │ 平均延迟 │ Token/s │ 成本     │ ││
│  │  ├─────────┼────────┼─────────┼─────────┼─────────┼──────────┤ ││
│  │  │ opus    │ 523    │ 97.9%   │ 3.2s    │ 38.5    │ $8.25    │ ││
│  │  │ sonnet  │ 856    │ 98.4%   │ 1.9s    │ 52.3    │ $4.20    │ ││
│  │  │ haiku   │ 144    │ 99.3%   │ 0.9s    │ 78.9    │ $0.15    │ ││
│  │  └─────────┴────────┴─────────┴─────────┴─────────┴──────────┘ ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌────────────────────────────────┐  ┌──────────────────────────────┐│
│  │  调用分布 (饼图)               │  │  Token 用量分布 (饼图)        ││
│  │                                │  │                              ││
│  │       ┌───────┐                │  │       ┌───────┐              ││
│  │      /  opus  \                │  │      /  opus  \              ││
│  │     │  34.3%   │               │  │     │  37.0%   │             ││
│  │      \        /                │  │      \        /              ││
│  │  haiku└──────┘sonnet           │  │  haiku└──────┘sonnet         ││
│  │   9.5%        56.2%            │  │   8.8%        54.2%          ││
│  │                                │  │                              ││
│  └────────────────────────────────┘  └──────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  模型性能趋势 (24h)                                              ││
│  │                                                                  ││
│  │  延迟 (ms)                                                       ││
│  │  4000 ┤                                                          ││
│  │  3000 ┤ ─────────────────────────────────────── opus             ││
│  │  2000 ┤ ═══════════════════════════════════════ sonnet           ││
│  │  1000 ┤ ▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬▬ haiku            ││
│  │     0 ┼────────────────────────────────────────────────────────  ││
│  │       00:00    06:00    12:00    18:00    24:00                  ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  推荐                                                            ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐││
│  │  │ 最佳性能    │ │ 最佳性价比  │ │ 最稳定      │ │ 最快响应    │││
│  │  │   opus     │ │   haiku    │ │   haiku    │ │   haiku    │││
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 单模型详情页面

```
┌─────────────────────────────────────────────────────────────────────┐
│  模型详情: Sonnet                                   [日] [周] [月]   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │ 调用次数      │  │ 成功率       │  │ Token 用量   │  │ 成本      │ │
│  │    856       │  │   98.4%      │  │   2.19M      │  │  $4.20   │ │
│  │  ↑ 15.2%     │  │  ↑ 0.8%      │  │  ↑ 12.3%     │  │  ↑ 8.7%  │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────┘ │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  性能指标                                                        ││
│  │                                                                  ││
│  │  ┌─────────────────────┐  ┌─────────────────────┐               ││
│  │  │ 延迟统计            │  │ 生成速率            │               ││
│  │  │                     │  │                     │               ││
│  │  │ 平均:    1,856 ms   │  │ 平均:    52.3 tok/s │               ││
│  │  │ P50:     1,650 ms   │  │ P50:     49.2 tok/s │               ││
│  │  │ P95:     3,200 ms   │  │ P95:     62.3 tok/s │               ││
│  │  │ P99:     4,500 ms   │  │ P99:     71.5 tok/s │               ││
│  │  │                     │  │                     │               ││
│  │  │ 首Token: 234 ms     │  │ 解码:    55.2 tok/s │               ││
│  │  └─────────────────────┘  └─────────────────────┘               ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  Token 用量明细                                                  ││
│  │                                                                  ││
│  │  输入 Token:    1,456,789  ████████████████████████  66.5%      ││
│  │  输出 Token:      734,567  ████████████              33.5%      ││
│  │  缓存读取:        256,789  ████████                  11.7%      ││
│  │  缓存写入:         73,456  ███                        3.4%      ││
│  │                                                                  ││
│  │  成本明细:                                                       ││
│  │  输入:  $4.37 (1.46M × $3.00/M)                                 ││
│  │  输出: $11.02 (0.73M × $15.00/M)                                ││
│  │  缓存:  -$0.69 (节省)                                           ││
│  │  ─────────────────────────                                      ││
│  │  总计:  $4.20                                                   ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  调用趋势 (24h)                                                  ││
│  │  ▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂▁   ││
│  │  00:00        06:00        12:00        18:00        24:00      ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  错误分析                                                        ││
│  │                                                                  ││
│  │  失败调用: 14 (1.6%)                                            ││
│  │  ┌─────────────┬────────┬─────────────────────────────────────┐││
│  │  │ 错误类型    │ 次数   │ 占比                                │││
│  │  ├─────────────┼────────┼─────────────────────────────────────┤││
│  │  │ 超时        │ 8      │ ████████████████████████  57.1%    │││
│  │  │ 限流        │ 4      │ ████████████              28.6%    │││
│  │  │ 其他错误    │ 2      │ ██████                    14.3%    │││
│  │  └─────────────┴────────┴─────────────────────────────────────┘││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 详细统计页面

```
┌─────────────────────────────────────────────────────────────────────┐
│  Token 用量详情                                     [日] [周] [月]   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  用量趋势                                                        ││
│  │                                                                  ││
│  │  输入 ████████████████████████████████████████  2.46M           ││
│  │  输出 ████████████████████                      1.23M           ││
│  │  缓存 ████████                                  0.58M           ││
│  │                                                                  ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  按模型分布                                                      ││
│  │                                                                  ││
│  │  ┌─────────┬───────────┬───────────┬───────────┬──────────────┐││
│  │  │ 模型    │ 输入Token │ 输出Token │ 总Token   │ 成本 (USD)   │││
│  │  ├─────────┼───────────┼───────────┼───────────┼──────────────┤││
│  │  │ opus    │ 1,000,000 │ 500,000   │ 1,500,000 │ $8.25        │││
│  │  │ sonnet  │ 1,456,789 │ 734,567   │ 2,191,356 │ $4.20        │││
│  │  │ haiku   │ 0         │ 0         │ 0         │ $0.00        │││
│  │  ├─────────┼───────────┼───────────┼───────────┼──────────────┤││
│  │  │ 总计    │ 2,456,789 │ 1,234,567 │ 3,691,356 │ $12.45       │││
│  │  └─────────┴───────────┴───────────┴───────────┴──────────────┘││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## 配置

### config.yaml

```yaml
metrics:
  enabled: true

  # 存储配置
  storage:
    type: "sqlite"           # sqlite | postgres | memory
    path: "data/metrics.db"  # SQLite 路径
    retention:
      raw: "7d"              # 原始数据保留时间
      hourly: "30d"          # 小时聚合保留时间
      daily: "365d"          # 日聚合保留时间

  # 采集配置
  collection:
    interval: "10s"          # 采集间隔
    process_interval: "5s"   # 进程监控间隔
    system_interval: "30s"   # 系统资源采集间隔

  # 导出配置
  export:
    prometheus:
      enabled: true
      path: "/metrics"
    json:
      enabled: true
      path: "/api/v1/metrics/export"

  # Token 价格配置 (使用通用匹配机制)
  # 匹配规则: 忽略 "claude-" 前缀和版本号/日期后缀
  token_pricing:
    # Anthropic Claude 系列
    - pattern: "opus*"
      input: 5.00
      output: 25.00
      cache_read: 0.50
      cache_write: 6.25
    - pattern: "sonnet*"
      input: 3.00
      output: 15.00
      cache_read: 0.30
      cache_write: 3.75
    - pattern: "haiku*"
      input: 1.00
      output: 5.00
      cache_read: 0.10
      cache_write: 1.25
    # OpenAI GPT 系列
    - pattern: "gpt-4o*"
      input: 2.50
      output: 10.00
    - pattern: "gpt-4-turbo*"
      input: 10.00
      output: 30.00
    - pattern: "gpt-4*"
      input: 30.00
      output: 60.00
    - pattern: "gpt-3.5*"
      input: 0.50
      output: 1.50
    # Google Gemini 系列
    - pattern: "gemini-1.5-pro*"
      input: 1.25
      output: 5.00
    - pattern: "gemini-1.5-flash*"
      input: 0.075
      output: 0.30
    # DeepSeek 系列
    - pattern: "deepseek-v3*"
      input: 0.27
      output: 1.10
    - pattern: "deepseek-r1*"
      input: 0.55
      output: 2.19
    # 默认 (未匹配时使用)
    - pattern: "*"
      input: 1.00
      output: 5.00
```

## 实现清单

### Phase 1: 基础统计 (Week 1)

- [ ] 实现 `CallStats` 数据结构
- [ ] 实现调用计数器
- [ ] 添加成功/失败统计
- [ ] 实现基础 API 端点
- [ ] 编写单元测试

### Phase 2: Token 用量 (Week 2)

- [ ] 实现 `TokenUsage` 数据结构
- [ ] 集成 CLI 输出解析
- [ ] 实现成本计算
- [ ] 添加 Token 统计 API
- [ ] 编写单元测试

### Phase 3: 性能指标 (Week 3)

- [ ] 实现延迟统计
- [ ] 实现百分位数计算
- [ ] 实现生成速率计算
- [ ] 实现时间分解
- [ ] 添加性能指标 API

### Phase 4: 系统监控 (Week 4)

- [ ] 实现进程监控
- [ ] 实现系统资源采集
- [ ] 添加历史记录
- [ ] 添加系统监控 API
- [ ] 跨平台适配 (Linux/macOS/Windows)

### Phase 5: 存储与聚合 (Week 5)

- [ ] 实现 SQLite 存储
- [ ] 实现数据聚合
- [ ] 实现数据清理
- [ ] 实现数据导出
- [ ] 编写集成测试

### Phase 6: UI 集成 (Week 6)

- [ ] 实现仪表盘组件
- [ ] 实现图表组件
- [ ] 实现详情页面
- [ ] 实现实时更新
- [ ] 端到端测试

## 成功指标

| 指标 | 目标 |
|-----|------|
| 指标采集延迟 | < 100ms |
| 存储写入延迟 | < 50ms |
| API 响应时间 | < 200ms |
| 数据准确性 | 99.9% |
| UI 刷新频率 | 5s |

## 非功能需求

| 需求 | 目标 |
|-----|------|
| 存储空间 | < 100MB/月 |
| CPU 开销 | < 1% |
| 内存开销 | < 50MB |
| 数据保留 | 最长 1 年 |

## 参考

- [Prometheus Metrics](https://prometheus.io/docs/concepts/metric_types/)
- [OpenTelemetry](https://opentelemetry.io/)
- [v0.10 CLI Reliability](v0.10-cli-reliability.md)
