# Configuration Guide

[中文版本](./zh/configuration.md)

This guide covers all configuration options for ZimaOS-Echo.

## Configuration File

The main configuration file is `config.yaml` located in the application directory.

### Basic Structure

```yaml
# Server configuration
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

# LLM Provider configuration
llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      model: "gpt-4"
      base_url: "https://api.openai.com/v1"
    anthropic:
      api_key: "${ANTHROPIC_API_KEY}"
      model: "claude-3-opus-20240229"
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"

# Authentication
auth:
  enabled: true
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "24h"
    refresh_expiration: "168h"
  api_keys:
    enabled: true
    max_per_user: 10

# RBAC
rbac:
  enabled: true
  default_role: "user"
  roles:
    - name: admin
      permissions: ["*"]
    - name: user
      permissions: ["chat", "skills.execute"]
    - name: readonly
      permissions: ["chat.read", "skills.list"]

# Plugin System
plugins:
  enabled: true
  directory: "./plugins"
  auto_load: true

# Skill Hub
skills:
  enabled: true
  builtin:
    calculator: true
    datetime: true
    systeminfo: true
    weather: true
    search: true

# Task Scheduler
scheduler:
  enabled: true
  max_concurrent: 10
  default_timeout: "5m"
  retry:
    max_attempts: 3
    backoff_base: "1s"
    backoff_max: "1m"

# Metrics
metrics:
  enabled: true
  endpoint: "/metrics"
  include_runtime: true
  include_http: true
  include_llm: true

# Profiling
profiling:
  enabled: false
  endpoint_prefix: "/debug/pprof"
  auth_required: true

# Hot Reload
config:
  hot_reload: true
  watch_interval_ms: 1000
  validate_before_apply: true

# Resilience
resilience:
  circuit_breaker:
    enabled: true
    threshold: 5
    timeout_seconds: 30
  rate_limit:
    enabled: true
    requests_per_second: 10
    burst: 20

# Backup
backup:
  enabled: true
  schedule: "0 2 * * *"
  retention_days: 7
  path: "./backups"

# Logging
logging:
  level: "info"
  format: "json"
  output: "stdout"
  file:
    enabled: false
    path: "./logs/echo.log"
    max_size_mb: 100
    max_backups: 3
    max_age_days: 7

# Database
database:
  type: "sqlite"
  path: "./data/echo.db"
  max_connections: 10
```

## Environment Variables

All configuration values can be overridden using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `ECHO_HOST` | Server host | `0.0.0.0` |
| `ECHO_PORT` | Server port | `8080` |
| `JWT_SECRET` | JWT signing secret | (required) |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `ANTHROPIC_API_KEY` | Anthropic API key | - |
| `OLLAMA_BASE_URL` | Ollama server URL | `http://localhost:11434` |

### Using Environment Variables in Config

Use `${VAR_NAME}` syntax to reference environment variables:

```yaml
llm:
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
```

## Configuration Sections

### Server

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | string | `0.0.0.0` | Listen address |
| `port` | int | `8080` | Listen port |
| `read_timeout` | duration | `30s` | Request read timeout |
| `write_timeout` | duration | `30s` | Response write timeout |

### LLM Providers

#### OpenAI

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `api_key` | string | - | OpenAI API key |
| `model` | string | `gpt-4` | Model to use |
| `base_url` | string | `https://api.openai.com/v1` | API base URL |
| `max_tokens` | int | `4096` | Maximum tokens |
| `temperature` | float | `0.7` | Sampling temperature |

#### Anthropic

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `api_key` | string | - | Anthropic API key |
| `model` | string | `claude-3-opus-20240229` | Model to use |
| `max_tokens` | int | `4096` | Maximum tokens |

#### Ollama

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `base_url` | string | `http://localhost:11434` | Ollama server URL |
| `model` | string | `llama2` | Model to use |

### Authentication

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable authentication |
| `jwt.secret` | string | - | JWT signing secret (required) |
| `jwt.expiration` | duration | `24h` | Access token expiration |
| `jwt.refresh_expiration` | duration | `168h` | Refresh token expiration |
| `api_keys.enabled` | bool | `true` | Enable API key auth |
| `api_keys.max_per_user` | int | `10` | Max API keys per user |

### RBAC

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable RBAC |
| `default_role` | string | `user` | Default role for new users |
| `roles` | array | - | Role definitions |

### Scheduler

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable scheduler |
| `max_concurrent` | int | `10` | Max concurrent tasks |
| `default_timeout` | duration | `5m` | Default task timeout |
| `retry.max_attempts` | int | `3` | Max retry attempts |
| `retry.backoff_base` | duration | `1s` | Base backoff duration |
| `retry.backoff_max` | duration | `1m` | Max backoff duration |

### Metrics

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Prometheus metrics |
| `endpoint` | string | `/metrics` | Metrics endpoint path |
| `include_runtime` | bool | `true` | Include Go runtime metrics |
| `include_http` | bool | `true` | Include HTTP metrics |
| `include_llm` | bool | `true` | Include LLM metrics |

### Backup

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable backups |
| `schedule` | string | `0 2 * * *` | Cron schedule |
| `retention_days` | int | `7` | Days to keep backups |
| `path` | string | `./backups` | Backup directory |

## Hot Reload

Configuration changes can be applied without restart:

1. **Automatic**: File changes detected automatically
2. **Signal**: Send `SIGHUP` to reload
3. **API**: POST to `/api/v1/config/reload`

### Reloadable Options

Not all options support hot reload:

| Section | Hot Reload |
|---------|------------|
| Server port | ❌ |
| LLM providers | ✅ |
| Auth settings | ✅ |
| RBAC roles | ✅ |
| Scheduler | ✅ |
| Metrics | ✅ |
| Logging level | ✅ |

## Validation

Configuration is validated on load and reload:

- Required fields checked
- Type validation
- Range validation
- Cross-field validation

Invalid configuration is rejected with detailed error messages.

## Examples

### Minimal Configuration

```yaml
server:
  port: 8080

llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"

auth:
  enabled: false
```

### Production Configuration

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 60s
  write_timeout: 120s

llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      model: "gpt-4"

auth:
  enabled: true
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "1h"

metrics:
  enabled: true

backup:
  enabled: true
  schedule: "0 */6 * * *"
  retention_days: 30

logging:
  level: "info"
  format: "json"
```

### Development Configuration

```yaml
server:
  port: 8080

llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"

auth:
  enabled: false

profiling:
  enabled: true
  auth_required: false

logging:
  level: "debug"
  format: "text"
```
