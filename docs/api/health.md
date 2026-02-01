# Health API

Health check endpoints for monitoring service status.

## GET /health

Returns comprehensive health information about the service.

### Request

```bash
curl http://localhost:23456/health
```

### Response

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

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Service status (`ok` or `error`) |
| `timestamp` | string | Current server time (ISO 8601) |
| `uptime` | string | Time since service started |
| `version` | string | Service version |
| `go_version` | string | Go runtime version |
| `num_cpu` | integer | Number of CPUs available |
| `goroutines` | integer | Current number of goroutines |
| `mem_alloc_bytes` | integer | Allocated memory in bytes |

---

## GET /health/live

Kubernetes liveness probe endpoint. Returns 200 if the service is alive.

### Request

```bash
curl http://localhost:23456/health/live
```

### Response

**Success (200 OK)**
```json
{
  "status": "alive"
}
```

### Usage in Kubernetes

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

Kubernetes readiness probe endpoint. Returns 200 if the service is ready to accept traffic.

### Request

```bash
curl http://localhost:23456/health/ready
```

### Response

**Ready (200 OK)**
```json
{
  "status": "ready"
}
```

**Not Ready (503 Service Unavailable)**
```json
{
  "status": "not ready"
}
```

### Usage in Kubernetes

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

Returns worker pool statistics.

### Request

```bash
curl http://localhost:23456/api/v1/workers/stats
```

### Response

```json
{
  "pool_size": 10,
  "running": 2,
  "total": 150
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `pool_size` | integer | Maximum concurrent workers |
| `running` | integer | Currently running workers |
| `total` | integer | Total tasks processed |
