# PRD: API Proxy CC Cache 强化

**Version**: 0.10.25
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-02-13

---

## 1. Overview

### 1.1 Background

ZimaOS-Blue 的 API Proxy 作为 Claude Code 与上游 API 之间的中间层，每次请求都直接转发到上游。大量重复或语义相同的请求（如代码解释、补全）会产生不必要的 API 调用和延迟。通过引入两级缓存（L1 内存 + L2 磁盘）、请求规范化和防穿透机制，可以显著降低延迟、减少 API 消耗、提升用户体验。

### 1.2 Goals

1. 实现 L1（内存）+ L2（磁盘）两级缓存架构，命中时直接返回缓存结果
2. 通过 Sanitizer + Canonicalizer 保证语义相同的请求生成一致的 cache key
3. 使用 Singleflight 防止缓存穿透（多个并发相同请求只调用一次上游 API）
4. 支持启动时 Warmup，将磁盘热点数据预加载到内存
5. 提供缓存命中率、延迟节省等运维指标

### 1.3 Non-Goals

1. 分布式缓存（本版本仅支持单机）
2. Streaming 响应缓存（仅缓存完整响应）
3. 缓存用户会话状态或有副作用的请求

---

## 2. Architecture

### 2.1 总体架构（单机版）

```
Claude Code / VSCode Plugin
        |
        v
   AI Proxy (ZimaOS-Blue)
        |
        |-- Sanitizer (清除 cch/random header)
        |-- Canonicalizer (生成稳定 hash key)
        |-- Memory Cache (L1)
        |-- Disk Cache (L2, 持久化)
        |-- Singleflight (防止穿透)
        |
        v
   Claude Code API
```

- **L1**: 进程内 memory map + LRU（快速命中）
- **L2**: 本地硬盘文件/DB（BoltDB / BadgerDB / LevelDB），持久化
- **Warmup**: 启动时读取 L2 的热点缓存到 L1

### 2.2 完整请求流程

1. 收到请求
2. Sanitizer 去掉随机字段（billing header、cch、cc_version 等）
3. Canonicalizer 生成 SHA256 hash key
4. L1 内存 cache 查找
   - hit → 直接返回
5. L2 Disk cache 查找
   - hit → 写入 L1 → 返回
6. Cache miss
   - → Singleflight 调用 Claude Code API
   - → 存入 L1 + L2 Disk
   - → 返回

---

## 3. Functional Requirements

### 3.1 Cache Key & Canonicalization

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-001 | 使用 SHA256 生成 cache key，输入为 model + canonical(messages) + bucketed parameters | P0 |
| FR-002 | Sanitizer 清理 system message 中的 `x-anthropic-billing-header` / `cch=xxx` / `cc_version` | P0 |
| FR-003 | Canonicalizer 对 messages 排序，保证相同语义请求 hash 不变 | P0 |
| FR-004 | 数值参数量化：temperature / top_p 四舍五入到 0.1 精度 | P1 |
| FR-005 | 删除无意义字段（空字符串、null 值等） | P1 |

**Key 构成**:

```
SHA256(
  model +
  canonical(messages) +
  temperature_bucket +
  top_p_bucket +
  max_tokens
)
```

### 3.2 L1 Memory Cache

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-006 | 使用 LRU 或 Ristretto 实现进程内内存缓存 | P0 |
| FR-007 | 容量可配置（默认 10,000 条，范围 1k~50k） | P0 |
| FR-008 | 支持 auto-evict（LRU 淘汰策略） | P0 |
| FR-009 | 启动时支持 Warmup（从 L2 加载热点数据） | P1 |

### 3.3 L2 Disk Cache

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-010 | 使用嵌入式 KV 数据库实现磁盘持久化缓存 | P0 |
| FR-011 | 支持 TTL 过期（可配置，默认 30min~1h） | P0 |
| FR-012 | 支持按 timestamp 或空间限制清理过期数据 | P1 |
| FR-013 | embedding / reference 类请求支持永久缓存 | P2 |

**Disk Cache 选型对比**:

| 库 | 优点 | 备注 |
|----|------|------|
| BadgerDB | 高性能，支持 TTL | Go 原生，支持事务 |
| BoltDB | 简单、稳定 | 单文件，但大文件写入慢 |
| LevelDB | 快速、压缩好 | C++ 原生，Go 绑定 |

**存储格式**:

```json
{
  "key": "<sha256-hash>",
  "value": {
    "response": { "..." },
    "usage": { "..." },
    "created_at": "<timestamp>"
  }
}
```

### 3.4 Singleflight 防穿透

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-014 | 使用 `golang.org/x/sync/singleflight` 合并并发相同请求 | P0 |
| FR-015 | 相同 cache key 的并发请求只调用一次上游 API，其余等待结果 | P0 |
| FR-016 | Singleflight 超时保护，避免长时间阻塞 | P1 |

**参考实现**:

```go
var group singleflight.Group

val, _, _ := group.Do(key, func() (interface{}, error) {
    resp := callClaudeCodeAPI()
    cache.Store(key, resp)
    return resp, nil
})
```

### 3.5 Warmup 机制

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-017 | 启动时扫描 disk cache，按访问频率/timestamp 排序 | P1 |
| FR-018 | 载入 Top-N 热点数据到 L1 内存 cache（默认 N=1000） | P1 |
| FR-019 | 支持定时 warmup（如每天更新热点） | P2 |

**参考实现**:

```go
for _, kv := range diskCache.TopN(1000) {
    l1Cache.Set(kv.Key, kv.Value)
}
```

### 3.6 Cache 生命周期

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-020 | TTL 可配置：code explain 类 30min~1h | P0 |
| FR-021 | embedding / reference 类支持永久缓存 | P2 |
| FR-022 | L1 使用 LRU auto-evict | P0 |
| FR-023 | L2 Disk 支持按 timestamp 或空间限制清理 | P1 |

### 3.7 运维指标

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-024 | 统计并暴露 `cache_hit_ratio`（总命中率） | P0 |
| FR-025 | 统计并暴露 `l1_hit_ratio`（内存命中率） | P0 |
| FR-026 | 统计并暴露 `disk_hit_ratio`（磁盘命中率） | P0 |
| FR-027 | 统计并暴露 `miss_count`（未命中次数） | P0 |
| FR-028 | 统计并暴露 `latency_saved_ms`（缓存节省的延迟） | P1 |
| FR-029 | 支持 Prometheus / expvar 方式收集指标 | P2 |

---

## 4. Technical Design

### 4.1 Sanitizer & Canonicalizer

```go
type Message struct {
    Role    string
    Content string
}

func SanitizeMessages(msgs []Message) []Message {
    out := []Message{}
    for _, m := range msgs {
        if m.Role == "system" &&
           strings.HasPrefix(m.Content, "x-anthropic-billing-header:") {
            continue
        }
        out = append(out, m)
    }
    return out
}

func CanonicalKey(model string, msgs []Message, temp float64, topP float64, maxTokens int) string {
    msgsJson, _ := json.Marshal(msgs)
    keyData := fmt.Sprintf("%s|%s|%.1f|%.1f|%d", model, msgsJson, temp, topP, maxTokens)
    sum := sha256.Sum256([]byte(keyData))
    return hex.EncodeToString(sum[:])
}
```

### 4.2 Package Structure

```
server/internal/providerpool/cache/
├── cache.go          // Cache interface & CacheManager (L1+L2 orchestration)
├── l1_memory.go      // LRU/Ristretto memory cache
├── l2_disk.go        // BadgerDB/BoltDB disk cache
├── canonicalizer.go  // Sanitizer + Canonicalizer + key generation
├── singleflight.go   // Singleflight wrapper
├── warmup.go         // Warmup logic
└── metrics.go        // Cache metrics collection
```

### 4.3 Configuration

```yaml
cache:
  enabled: true
  l1:
    max_entries: 10000        # L1 最大条目数
    algorithm: "lru"          # lru | ristretto
  l2:
    engine: "badger"          # badger | bolt | leveldb
    path: "./data/cache"      # 磁盘缓存路径
    max_size_mb: 1024         # 最大磁盘空间
  ttl:
    default: "30m"            # 默认 TTL
    embedding: "0"            # 0 = 永久
  warmup:
    enabled: true
    top_n: 1000               # 启动时加载 Top-N 热点
    schedule: "0 3 * * *"     # 定时 warmup cron（可选）
  singleflight:
    enabled: true
    timeout: "30s"            # 最大等待时间
```

---

## 5. API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/cache/stats` | 获取缓存统计信息（命中率、条目数等） |
| POST | `/api/v1/cache/clear` | 清空所有缓存 |
| POST | `/api/v1/cache/warmup` | 手动触发 warmup |
| DELETE | `/api/v1/cache/key/:hash` | 删除指定缓存条目 |
| GET | `/api/v1/cache/config` | 获取当前缓存配置 |
| PUT | `/api/v1/cache/config` | 更新缓存配置 |

---

## 6. Implementation Checklist

> 基于现有代码分析：L1 memory cache (sync.Map + LRU eviction) 已存在于 `proxy/cache.go`，
> 需要增强 canonicalization、添加 L2 disk、singleflight、warmup 和分层指标。

- [x] Phase 1: Canonicalizer & Singleflight (P0)
  - [x] 1.1 新建 `proxy/canonicalizer.go` — Sanitizer + Canonicalizer + SHA256 key 生成
  - [x] 1.2 新建 `proxy/singleflight.go` — Singleflight wrapper 防穿透
  - [x] 1.3 修改 `proxy/cache.go` — GenerateKey 改用 CanonicalKey，添加 L1/L2 分层 hit 统计
  - [x] 1.4 修改 `proxy/handler.go` — ServeHTTP 集成 canonicalizer + singleflight + latency tracking

- [x] Phase 2: L2 Disk Cache (P0)
  - [x] 2.1 新建 `proxy/cache_disk.go` — SQLite 磁盘缓存实现（复用已有依赖，无需新增 BadgerDB）
  - [x] 2.2 修改 `proxy/cache.go` — CCCache 增加 L2 disk 字段，Get/Set 实现两级查找
  - [x] 2.3 修改 `proxy/config.go` — DefaultCacheConfig 改为 multilevel 模式
  - [x] 2.4 无需新增依赖（复用 mattn/go-sqlite3）

- [x] Phase 3: Warmup & Metrics (P1)
  - [x] 3.1 Warmup 集成到 `proxy/cache.go` Warmup() 方法（从 L2 加载 Top-N 到 L1）
  - [x] 3.2 修改 `proxy/cache.go` — 增强 Stats() 返回 l1_hit_ratio / disk_hit_ratio / latency_saved_ms
  - [x] 3.3 修改 `proxy/cache_handler.go` — 添加 POST /warmup endpoint + TriggerWarmup handler
  - [x] 3.4 warmup route 已通过 RegisterRoutes 自动注册
  - [x] 3.5 修改 `cmd/blue/main.go` — 启动时自动 warmup + disk cache 路径使用 dataDir
  - [x] 3.6 Singleflight 完整集成到 handler（cache miss 时合并并发请求）
  - [x] 3.7 Cache HIT 时记录 latency_saved_ms

- [ ] Phase 4: Advanced (P2) — 后续版本
  - [ ] 4.1 定时 Warmup (cron)
  - [ ] 4.2 永久缓存（embedding/reference 类请求）
  - [ ] 4.3 Prometheus / expvar 指标导出
  - [ ] 4.4 前端缓存管理 UI
