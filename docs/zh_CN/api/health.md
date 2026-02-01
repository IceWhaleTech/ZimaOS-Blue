# 健康检查 API

用于监控服务状态的健康检查端点。

## GET /health

返回服务的综合健康信息。

### 请求

```bash
curl http://localhost:23456/health
```

### 响应

```json
{
  "status": "ok",
  "timestamp": "2024-01-01T12:00:00Z",
  "uptime": "24h30m15s",
  "version": "0.1.0",
  "go_version": "go1.21.5",
  "num_cpu": 4,
  "goroutines": 12,
  "mem_alloc_bytes": 5242880
}
```

### 字段说明

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `status` | string | 服务状态（`ok` 或 `error`） |
| `timestamp` | string | 当前服务器时间（ISO 8601） |
| `uptime` | string | 服务启动后的运行时间 |
| `version` | string | 服务版本 |
| `go_version` | string | Go 运行时版本 |
| `num_cpu` | integer | 可用 CPU 数量 |
| `goroutines` | integer | 当前 goroutine 数量 |
| `mem_alloc_bytes` | integer | 已分配内存（字节） |

---

## GET /health/live

Kubernetes 存活探针端点。如果服务存活则返回 200。

### 请求

```bash
curl http://localhost:23456/health/live
```

### 响应

**成功 (200 OK)**
```json
{
  "status": "alive"
}
```

### Kubernetes 中的使用

```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 23456
  initialDelaySeconds: 5
  periodSeconds: 10
```

---

## GET /health/ready

Kubernetes 就绪探针端点。如果服务准备好接受流量则返回 200。

### 请求

```bash
curl http://localhost:23456/health/ready
```

### 响应

**就绪 (200 OK)**
```json
{
  "status": "ready"
}
```

**未就绪 (503 Service Unavailable)**
```json
{
  "status": "not ready"
}
```

### Kubernetes 中的使用

```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 23456
  initialDelaySeconds: 5
  periodSeconds: 5
```

---

## GET /api/v1/workers/stats

返回工作池统计信息。

### 请求

```bash
curl http://localhost:23456/api/v1/workers/stats
```

### 响应

```json
{
  "pool_size": 10,
  "running": 2,
  "total": 150
}
```

### 字段说明

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `pool_size` | integer | 最大并发工作数 |
| `running` | integer | 当前运行中的工作数 |
| `total` | integer | 已处理的总任务数 |
