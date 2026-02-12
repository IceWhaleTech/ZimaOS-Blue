# 配置指南

[English Version](../../../DEV/configuration.md)

本指南涵盖 ZimaOS-Blue 的所有配置选项。

## 配置文件

主配置文件是位于应用程序目录中的 `config.yaml`。

### 基本结构

```yaml
# 服务器配置
server:
  host: "0.0.0.0"
  port: 23456
  read_timeout: 30s
  write_timeout: 30s

# LLM 提供商配置
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

# 认证
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

# 插件系统
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

# 任务调度器
scheduler:
  enabled: true
  max_concurrent: 10
  default_timeout: "5m"
  retry:
    max_attempts: 3
    backoff_base: "1s"
    backoff_max: "1m"

# 指标
metrics:
  enabled: true
  endpoint: "/metrics"
  include_runtime: true
  include_http: true
  include_llm: true

# 性能分析
profiling:
  enabled: false
  endpoint_prefix: "/debug/pprof"
  auth_required: true

# 热重载
config:
  hot_reload: true
  watch_interval_ms: 1000
  validate_before_apply: true

# 弹性
resilience:
  circuit_breaker:
    enabled: true
    threshold: 5
    timeout_seconds: 30
  rate_limit:
    enabled: true
    requests_per_second: 10
    burst: 20

# 备份
backup:
  enabled: true
  schedule: "0 2 * * *"
  retention_days: 7
  path: "./backups"

# 日志
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

# 数据库
database:
  type: "sqlite"
  path: "./data/echo.db"
  max_connections: 10
```

## 环境变量

所有配置值都可以使用环境变量覆盖：

| 变量 | 描述 | 默认值 |
|------|------|--------|
| `BLUE_HOST` | 服务器主机 | `0.0.0.0` |
| `BLUE_PORT` | 服务器端口 | `23456` |
| `JWT_SECRET` | JWT 签名密钥 | (必需) |
| `OPENAI_API_KEY` | OpenAI API 密钥 | - |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | - |
| `OLLAMA_BASE_URL` | Ollama 服务器 URL | `http://localhost:11434` |

### 在配置中使用环境变量

使用 `${VAR_NAME}` 语法引用环境变量：

```yaml
llm:
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
```

## 配置部分

### 服务器

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `host` | string | `0.0.0.0` | 监听地址 |
| `port` | int | `23456` | 监听端口 |
| `read_timeout` | duration | `30s` | 请求读取超时 |
| `write_timeout` | duration | `30s` | 响应写入超时 |

### LLM 提供商

#### OpenAI

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `api_key` | string | - | OpenAI API 密钥 |
| `model` | string | `gpt-4` | 使用的模型 |
| `base_url` | string | `https://api.openai.com/v1` | API 基础 URL |
| `max_tokens` | int | `4096` | 最大 token 数 |
| `temperature` | float | `0.7` | 采样温度 |

#### Anthropic

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `api_key` | string | - | Anthropic API 密钥 |
| `model` | string | `claude-3-opus-20240229` | 使用的模型 |
| `max_tokens` | int | `4096` | 最大 token 数 |

#### Ollama

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `base_url` | string | `http://localhost:11434` | Ollama 服务器 URL |
| `model` | string | `llama2` | 使用的模型 |

### 认证

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用认证 |
| `jwt.secret` | string | - | JWT 签名密钥（必需）|
| `jwt.expiration` | duration | `24h` | 访问令牌过期时间 |
| `jwt.refresh_expiration` | duration | `168h` | 刷新令牌过期时间 |
| `api_keys.enabled` | bool | `true` | 启用 API 密钥认证 |
| `api_keys.max_per_user` | int | `10` | 每用户最大 API 密钥数 |

### RBAC

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用 RBAC |
| `default_role` | string | `user` | 新用户默认角色 |
| `roles` | array | - | 角色定义 |

### 调度器

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用调度器 |
| `max_concurrent` | int | `10` | 最大并发任务数 |
| `default_timeout` | duration | `5m` | 默认任务超时 |
| `retry.max_attempts` | int | `3` | 最大重试次数 |
| `retry.backoff_base` | duration | `1s` | 基础退避时间 |
| `retry.backoff_max` | duration | `1m` | 最大退避时间 |

### 指标

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用 Prometheus 指标 |
| `endpoint` | string | `/metrics` | 指标端点路径 |
| `include_runtime` | bool | `true` | 包含 Go 运行时指标 |
| `include_http` | bool | `true` | 包含 HTTP 指标 |
| `include_llm` | bool | `true` | 包含 LLM 指标 |

### 备份

| 选项 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `enabled` | bool | `true` | 启用备份 |
| `schedule` | string | `0 2 * * *` | Cron 调度 |
| `retention_days` | int | `7` | 备份保留天数 |
| `path` | string | `./backups` | 备份目录 |

## 热重载

配置更改可以在不重启的情况下应用：

1. **自动**：自动检测文件更改
2. **信号**：发送 `SIGHUP` 重载
3. **API**：POST 到 `/api/v1/config/reload`

### 可重载选项

并非所有选项都支持热重载：

| 部分 | 热重载 |
|------|--------|
| 服务器端口 | ❌ |
| LLM 提供商 | ✅ |
| 认证设置 | ✅ |
| RBAC 角色 | ✅ |
| 调度器 | ✅ |
| 指标 | ✅ |
| 日志级别 | ✅ |

## 验证

配置在加载和重载时进行验证：

- 检查必需字段
- 类型验证
- 范围验证
- 跨字段验证

无效配置将被拒绝并显示详细错误消息。

## 示例

### 最小配置

```yaml
server:
  port: 23456

llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"

auth:
  enabled: false
```

### 生产配置

```yaml
server:
  host: "0.0.0.0"
  port: 23456
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

### 开发配置

```yaml
server:
  port: 23456

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
