# API Proxy Sidecar - Configuration Guide

> Version: 0.10.5

## Overview

The API Proxy Sidecar provides intelligent routing, failover, security monitoring, and usage statistics for LLM API calls. This guide covers all configuration options.

## Configuration File

The proxy configuration is part of the main Echo configuration file, typically located at:
- `~/.config/zimaos-echo/config.yaml`
- `/etc/zimaos-echo/config.yaml`

## Configuration Structure

```yaml
proxy:
  enabled: true
  port:
    value: 0              # 0 = auto-allocate from range
    range: "19000-19100"  # Port range for auto-allocation
    bind_address: "127.0.0.1"

  routing:
    default_provider: "openai"
    load_balancing: "priority"  # priority, round-robin, weighted
    providers:
      - name: "openai"
        endpoint: "https://api.openai.com/v1"
        api_key: "${OPENAI_API_KEY}"
        priority: 1
        enabled: true
        weight: 100
        health_check: "/models"
      - name: "anthropic"
        endpoint: "https://api.anthropic.com/v1"
        api_key: "${ANTHROPIC_API_KEY}"
        priority: 2
        enabled: true
    failover:
      enabled: true
      max_retries: 3
      retry_delay: "1s"
      circuit_breaker: true
      failure_threshold: 5
      recovery_timeout: "30s"

  connection:
    max_idle_conns: 100
    max_idle_conns_per_host: 10
    max_conns_per_host: 100
    idle_conn_timeout: "90s"
    keep_alive: true
    keep_alive_interval: "30s"
    dial_timeout: "30s"
    tls_handshake_timeout: "10s"
    response_header_timeout: "60s"
    force_http2: true

  health_check:
    enabled: true
    interval: "30s"
    timeout: "10s"

  auth:
    enabled: false
    type: "api_key"       # api_key, bearer, basic
    api_keys: []
    allowed_ips: ["127.0.0.1"]
    header_name: "X-API-Key"
    skip_paths: ["/health", "/api/v1/proxy/status"]

  rate_limit:
    enabled: false
    requests_per_min: 60
    burst_size: 10
    per_ip: true

  guard:
    enabled: true
    block_on_detection: false
    max_prompt_length: 100000
    whitelist: []

  session:
    enabled: true
    idle_timeout: "30m"
    max_sessions: 1000
    cleanup_period: "5m"
    retain_complete: "1h"

  metrics:
    enabled: true
    retention_period: "24h"
    bucket_size: "1h"

  performance:
    buffer_pool_enabled: true
    buffer_pool_size: 1000
    buffer_initial_size: 4096
    buffer_max_size: 1048576
    cache_enabled: false
    cache_max_size: 1000
    cache_max_entry_size: 1048576
    cache_ttl: "5m"
```

---

## Port Configuration

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `value` | int | 0 | Fixed port number. 0 = auto-allocate |
| `range` | string | "19000-19100" | Port range for auto-allocation |
| `bind_address` | string | "127.0.0.1" | Address to bind to |

### Examples

**Fixed Port:**
```yaml
port:
  value: 8080
  bind_address: "0.0.0.0"
```

**Auto-Allocate:**
```yaml
port:
  value: 0
  range: "19000-19100"
  bind_address: "127.0.0.1"
```

### Port Discovery

When using auto-allocation, the port is written to:
- File: `~/.config/zimaos-echo/proxy.port`
- Environment: `ECHO_PROXY_PORT`

---

## Provider Configuration

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `name` | string | required | Unique provider identifier |
| `endpoint` | string | required | Base URL for the provider API |
| `api_key` | string | "" | API key (supports env vars) |
| `priority` | int | 1 | Selection priority (lower = higher priority) |
| `enabled` | bool | true | Whether provider is active |
| `weight` | int | 100 | Weight for weighted load balancing |
| `health_check` | string | "" | Health check endpoint path |
| `models` | []string | [] | Supported models (optional) |

### Environment Variables

API keys can reference environment variables:
```yaml
api_key: "${OPENAI_API_KEY}"
```

### Provider Examples

**OpenAI:**
```yaml
- name: "openai"
  endpoint: "https://api.openai.com/v1"
  api_key: "${OPENAI_API_KEY}"
  priority: 1
  health_check: "/models"
```

**Anthropic:**
```yaml
- name: "anthropic"
  endpoint: "https://api.anthropic.com/v1"
  api_key: "${ANTHROPIC_API_KEY}"
  priority: 2
```

**Azure OpenAI:**
```yaml
- name: "azure-openai"
  endpoint: "https://your-resource.openai.azure.com"
  api_key: "${AZURE_OPENAI_KEY}"
  priority: 3
```

**Local LLM (Ollama):**
```yaml
- name: "ollama"
  endpoint: "http://localhost:11434/v1"
  priority: 10
  enabled: true
```

---

## Load Balancing

### Strategies

| Strategy | Description |
|----------|-------------|
| `priority` | Select highest priority healthy provider |
| `round-robin` | Rotate through healthy providers |
| `weighted` | Select based on weight distribution |

### Priority Selection

Providers are selected by priority (lowest number = highest priority). Unhealthy providers are skipped.

```yaml
routing:
  load_balancing: "priority"
  providers:
    - name: "primary"
      priority: 1
    - name: "secondary"
      priority: 2
    - name: "fallback"
      priority: 3
```

### Weighted Selection

Distribute requests based on weight:

```yaml
routing:
  load_balancing: "weighted"
  providers:
    - name: "fast"
      weight: 70
    - name: "cheap"
      weight: 30
```

---

## Failover Configuration

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable failover |
| `max_retries` | int | 3 | Maximum retry attempts |
| `retry_delay` | duration | "1s" | Delay between retries |
| `circuit_breaker` | bool | true | Enable circuit breaker |
| `failure_threshold` | int | 5 | Failures before circuit opens |
| `recovery_timeout` | duration | "30s" | Time before half-open state |

### Circuit Breaker States

1. **Closed**: Normal operation, requests pass through
2. **Open**: Provider marked unhealthy, requests fail fast
3. **Half-Open**: Testing if provider recovered

### Example

```yaml
failover:
  enabled: true
  max_retries: 3
  retry_delay: "1s"
  circuit_breaker: true
  failure_threshold: 5
  recovery_timeout: "30s"
```

---

## Connection Pool

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_idle_conns` | int | 100 | Max idle connections total |
| `max_idle_conns_per_host` | int | 10 | Max idle connections per host |
| `max_conns_per_host` | int | 100 | Max connections per host |
| `idle_conn_timeout` | duration | "90s" | Idle connection timeout |
| `keep_alive` | bool | true | Enable keep-alive |
| `keep_alive_interval` | duration | "30s" | Keep-alive probe interval |
| `dial_timeout` | duration | "30s" | Connection dial timeout |
| `tls_handshake_timeout` | duration | "10s" | TLS handshake timeout |
| `response_header_timeout` | duration | "60s" | Response header timeout |
| `force_http2` | bool | true | Force HTTP/2 |

### Performance Tuning

For high-throughput scenarios:
```yaml
connection:
  max_idle_conns: 500
  max_idle_conns_per_host: 50
  max_conns_per_host: 200
  force_http2: true
```

---

## Health Check

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable health checks |
| `interval` | duration | "30s" | Check interval |
| `timeout` | duration | "10s" | Check timeout |

### Example

```yaml
health_check:
  enabled: true
  interval: "30s"
  timeout: "10s"
```

---

## Authentication

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable authentication |
| `type` | string | "api_key" | Auth type: api_key, bearer, basic |
| `api_keys` | []string | [] | Valid API keys |
| `allowed_ips` | []string | [] | IPs that bypass auth |
| `header_name` | string | "X-API-Key" | Header for API key |
| `skip_paths` | []string | [] | Paths that skip auth |

### API Key Authentication

```yaml
auth:
  enabled: true
  type: "api_key"
  api_keys:
    - "sk-your-key-1"
    - "sk-your-key-2"
  header_name: "X-API-Key"
  skip_paths:
    - "/health"
    - "/api/v1/proxy/status"
```

### IP Allowlist

```yaml
auth:
  enabled: true
  allowed_ips:
    - "127.0.0.1"
    - "192.168.1.0/24"
```

---

## Rate Limiting

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable rate limiting |
| `requests_per_min` | int | 60 | Requests per minute |
| `burst_size` | int | 10 | Burst allowance |
| `per_ip` | bool | true | Rate limit per IP |

### Example

```yaml
rate_limit:
  enabled: true
  requests_per_min: 100
  burst_size: 20
  per_ip: true
```

---

## Prompt Guard

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable prompt guard |
| `block_on_detection` | bool | false | Block detected injections |
| `max_prompt_length` | int | 100000 | Max prompt length |
| `whitelist` | []string | [] | Whitelisted patterns |

### Detection Patterns

The guard detects:
- Instruction override attempts
- Role manipulation
- System prompt extraction
- Jailbreak attempts

### Example

```yaml
guard:
  enabled: true
  block_on_detection: true
  max_prompt_length: 50000
  whitelist:
    - "ignore.*test"
```

---

## Session Monitoring

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable session tracking |
| `idle_timeout` | duration | "30m" | Session idle timeout |
| `max_sessions` | int | 1000 | Max concurrent sessions |
| `cleanup_period` | duration | "5m" | Cleanup interval |
| `retain_complete` | duration | "1h" | Retain completed sessions |

### Example

```yaml
session:
  enabled: true
  idle_timeout: "30m"
  max_sessions: 1000
  cleanup_period: "5m"
  retain_complete: "1h"
```

---

## Metrics Collection

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable metrics |
| `retention_period` | duration | "24h" | Data retention period |
| `bucket_size` | duration | "1h" | Time bucket size |

### Example

```yaml
metrics:
  enabled: true
  retention_period: "24h"
  bucket_size: "1h"
```

---

## Performance Optimization

### Buffer Pool

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `buffer_pool_enabled` | bool | true | Enable buffer pooling |
| `buffer_pool_size` | int | 1000 | Pool size |
| `buffer_initial_size` | int | 4096 | Initial buffer size |
| `buffer_max_size` | int | 1048576 | Max buffer size |

### Response Cache

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `cache_enabled` | bool | false | Enable response caching |
| `cache_max_size` | int | 1000 | Max cache entries |
| `cache_max_entry_size` | int | 1048576 | Max entry size |
| `cache_ttl` | duration | "5m" | Cache TTL |

### Example

```yaml
performance:
  buffer_pool_enabled: true
  buffer_pool_size: 1000
  buffer_initial_size: 4096
  buffer_max_size: 1048576
  cache_enabled: false
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ECHO_PROXY_PORT` | Override proxy port |
| `ECHO_PROXY_BIND` | Override bind address |
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `AZURE_OPENAI_KEY` | Azure OpenAI API key |

---

## Hot Reload

Configuration can be reloaded without restart:

```bash
# Via API
curl -X POST http://localhost:PORT/api/v1/proxy/config/reload

# Via signal
kill -HUP <pid>
```

---

## Troubleshooting

### Common Issues

**Port Already in Use:**
```yaml
port:
  value: 0  # Use auto-allocation
  range: "19000-19100"
```

**Provider Connection Timeout:**
```yaml
connection:
  dial_timeout: "60s"
  response_header_timeout: "120s"
```

**High Memory Usage:**
```yaml
performance:
  buffer_max_size: 524288  # Reduce to 512KB
  cache_enabled: false
```

**Rate Limit Too Strict:**
```yaml
rate_limit:
  requests_per_min: 120
  burst_size: 30
```

### Debug Mode

Enable debug logging:
```yaml
logging:
  level: "debug"
  proxy_requests: true
```

---

## Security Best Practices

1. **Enable Authentication** for production deployments
2. **Use HTTPS** for provider endpoints
3. **Restrict Bind Address** to localhost for local use
4. **Enable Prompt Guard** to detect injection attempts
5. **Set Rate Limits** to prevent abuse
6. **Monitor Metrics** for anomalies
7. **Rotate API Keys** regularly
8. **Use Environment Variables** for sensitive data

---

## Example Configurations

### Development

```yaml
proxy:
  enabled: true
  port:
    value: 8080
    bind_address: "127.0.0.1"
  auth:
    enabled: false
  rate_limit:
    enabled: false
  guard:
    enabled: true
    block_on_detection: false
```

### Production

```yaml
proxy:
  enabled: true
  port:
    value: 0
    range: "19000-19100"
    bind_address: "127.0.0.1"
  auth:
    enabled: true
    type: "api_key"
    api_keys:
      - "${PROXY_API_KEY}"
  rate_limit:
    enabled: true
    requests_per_min: 100
    burst_size: 20
  guard:
    enabled: true
    block_on_detection: true
  health_check:
    enabled: true
    interval: "30s"
```

### High Availability

```yaml
proxy:
  enabled: true
  routing:
    load_balancing: "priority"
    providers:
      - name: "primary"
        endpoint: "https://api.openai.com/v1"
        priority: 1
      - name: "secondary"
        endpoint: "https://api.anthropic.com/v1"
        priority: 2
      - name: "fallback"
        endpoint: "http://localhost:11434/v1"
        priority: 10
    failover:
      enabled: true
      max_retries: 3
      circuit_breaker: true
      failure_threshold: 3
      recovery_timeout: "60s"
  connection:
    max_idle_conns: 200
    max_conns_per_host: 100
    force_http2: true
```
