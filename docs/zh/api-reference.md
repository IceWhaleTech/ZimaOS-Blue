# API 参考

[English Version](../api-reference.md)

本文档提供 ZimaOS-Echo REST API 的完整参考。

## 基础 URL

```
http://localhost:8080/api/v1
```

## 认证

大多数端点需要认证。ZimaOS-Echo 支持两种认证方式：

### JWT 令牌

在 Authorization 头中包含令牌：

```
Authorization: Bearer <token>
```

### API 密钥

在 X-API-Key 头中包含 API 密钥：

```
X-API-Key: <api-key>
```

---

## 认证端点

### 登录

认证用户并接收 JWT 令牌。

```http
POST /auth/login
```

**请求体：**

```json
{
  "username": "string",
  "password": "string"
}
```

**响应：**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-15T12:00:00Z",
  "user": {
    "id": "uuid",
    "username": "admin",
    "role": "admin"
  }
}
```

**状态码：**
- `200` - 成功
- `401` - 凭据无效
- `429` - 尝试次数过多

---

### 登出

使当前令牌失效。

```http
POST /auth/logout
```

**请求头：**
- `Authorization: Bearer <token>`（必需）

**响应：**

```json
{
  "message": "成功登出"
}
```

---

### 刷新令牌

使用刷新令牌获取新的访问令牌。

```http
POST /auth/refresh
```

**请求体：**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**响应：**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-15T12:00:00Z"
}
```

---

### 获取当前用户

获取已认证用户的信息。

```http
GET /auth/me
```

**响应：**

```json
{
  "id": "uuid",
  "username": "admin",
  "email": "admin@example.com",
  "role": "admin",
  "permissions": ["*"],
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## API 密钥端点

### 列出 API 密钥

获取当前用户的所有 API 密钥。

```http
GET /apikeys
```

**响应：**

```json
{
  "keys": [
    {
      "id": "uuid",
      "name": "我的 API 密钥",
      "prefix": "ek_abc...",
      "scopes": ["chat", "skills.execute"],
      "last_used_at": "2024-01-10T10:00:00Z",
      "created_at": "2024-01-01T00:00:00Z",
      "expires_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

---

### 创建 API 密钥

创建新的 API 密钥。

```http
POST /apikeys
```

**请求体：**

```json
{
  "name": "我的 API 密钥",
  "scopes": ["chat", "skills.execute"],
  "expires_in": "8760h"
}
```

**响应：**

```json
{
  "id": "uuid",
  "name": "我的 API 密钥",
  "key": "ek_abc123...",
  "scopes": ["chat", "skills.execute"],
  "created_at": "2024-01-01T00:00:00Z",
  "expires_at": "2025-01-01T00:00:00Z"
}
```

> **注意：** 完整的 API 密钥仅在创建时返回一次。请安全存储。

---

### 删除 API 密钥

删除 API 密钥。

```http
DELETE /apikeys/:id
```

**响应：**

```json
{
  "message": "API 密钥已删除"
}
```

---

## 聊天端点

### 发送消息

发送消息并接收响应。

```http
POST /chat
```

**请求体：**

```json
{
  "message": "你好，最近怎么样？",
  "conversation_id": "uuid",
  "provider": "openai",
  "model": "gpt-4",
  "options": {
    "temperature": 0.7,
    "max_tokens": 1000
  }
}
```

| 字段 | 类型 | 必需 | 描述 |
|------|------|------|------|
| message | string | 是 | 用户消息 |
| conversation_id | string | 否 | 继续现有对话 |
| provider | string | 否 | LLM 提供商覆盖 |
| model | string | 否 | 模型覆盖 |
| options | object | 否 | 生成选项 |

**响应：**

```json
{
  "id": "uuid",
  "conversation_id": "uuid",
  "message": "我很好，谢谢你的关心！",
  "provider": "openai",
  "model": "gpt-4",
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 15,
    "total_tokens": 25
  },
  "created_at": "2024-01-15T10:00:00Z"
}
```

---

### 流式消息

发送消息并接收流式响应。

```http
POST /chat/stream
```

**请求体：** 与 `/chat` 相同

**响应：** Server-Sent Events (SSE)

```
data: {"type":"start","conversation_id":"uuid"}

data: {"type":"content","content":"我"}

data: {"type":"content","content":"很好"}

data: {"type":"content","content":"，谢谢"}

data: {"type":"done","usage":{"prompt_tokens":10,"completion_tokens":15}}
```

---

### 列出对话

获取当前用户的所有对话。

```http
GET /conversations
```

**查询参数：**

| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| limit | int | 20 | 最大结果数 |
| offset | int | 0 | 分页偏移 |

**响应：**

```json
{
  "conversations": [
    {
      "id": "uuid",
      "title": "关于天气的聊天",
      "message_count": 10,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T11:00:00Z"
    }
  ],
  "total": 50
}
```

---

### 获取对话

获取特定对话及其消息。

```http
GET /conversations/:id
```

**响应：**

```json
{
  "id": "uuid",
  "title": "关于天气的聊天",
  "messages": [
    {
      "id": "uuid",
      "role": "user",
      "content": "今天天气怎么样？",
      "created_at": "2024-01-15T10:00:00Z"
    },
    {
      "id": "uuid",
      "role": "assistant",
      "content": "我可以帮你查看天气...",
      "created_at": "2024-01-15T10:00:01Z"
    }
  ],
  "created_at": "2024-01-15T10:00:00Z"
}
```

---

### 删除对话

删除对话及其所有消息。

```http
DELETE /conversations/:id
```

**响应：**

```json
{
  "message": "对话已删除"
}
```

---

## 插件端点

### 列出插件

获取所有可用插件。

```http
GET /plugins
```

**响应：**

```json
{
  "plugins": [
    {
      "id": "example-plugin",
      "name": "示例插件",
      "version": "1.0.0",
      "description": "一个示例插件",
      "author": "ZimaOS",
      "enabled": true,
      "type": "javascript",
      "tools": ["example_tool"],
      "commands": ["/example"]
    }
  ]
}
```

---

### 获取插件详情

获取插件的详细信息。

```http
GET /plugins/:id
```

**响应：**

```json
{
  "id": "example-plugin",
  "name": "示例插件",
  "version": "1.0.0",
  "description": "一个示例插件",
  "author": "ZimaOS",
  "enabled": true,
  "type": "javascript",
  "tools": [
    {
      "name": "example_tool",
      "description": "做一些有用的事情",
      "parameters": {
        "input": {
          "type": "string",
          "required": true
        }
      }
    }
  ],
  "commands": [
    {
      "name": "/example",
      "description": "运行示例命令"
    }
  ],
  "config": {
    "api_key": "***"
  }
}
```

---

### 启用插件

启用插件。

```http
POST /plugins/:id/enable
```

**响应：**

```json
{
  "message": "插件已启用",
  "plugin": {
    "id": "example-plugin",
    "enabled": true
  }
}
```

---

### 禁用插件

禁用插件。

```http
POST /plugins/:id/disable
```

**响应：**

```json
{
  "message": "插件已禁用",
  "plugin": {
    "id": "example-plugin",
    "enabled": false
  }
}
```

---

### 更新插件配置

更新插件配置。

```http
PUT /plugins/:id/config
```

**请求体：**

```json
{
  "api_key": "new-api-key",
  "setting": "value"
}
```

**响应：**

```json
{
  "message": "配置已更新"
}
```

---

## Skill 端点

### 列出 Skills

获取所有可用的 Skills。

```http
GET /skills
```

**响应：**

```json
{
  "skills": [
    {
      "id": "calculator",
      "name": "计算器",
      "version": "1.0.0",
      "description": "执行算术计算",
      "category": "utility",
      "enabled": true
    },
    {
      "id": "weather",
      "name": "天气",
      "version": "1.0.0",
      "description": "获取天气信息",
      "category": "information",
      "enabled": true
    }
  ]
}
```

---

### 获取 Skill 详情

获取 Skill 的详细信息。

```http
GET /skills/:id
```

**响应：**

```json
{
  "id": "calculator",
  "name": "计算器",
  "version": "1.0.0",
  "description": "执行算术计算",
  "category": "utility",
  "enabled": true,
  "parameters": {
    "expression": {
      "type": "string",
      "description": "要计算的数学表达式",
      "required": true
    }
  },
  "examples": [
    {
      "input": {"expression": "2 + 2"},
      "output": {"result": 4}
    }
  ]
}
```

---

### 执行 Skill

使用给定参数执行 Skill。

```http
POST /skills/:id/execute
```

**请求体：**

```json
{
  "expression": "2 + 2 * 3"
}
```

**响应：**

```json
{
  "success": true,
  "result": {
    "value": 8,
    "expression": "2 + 2 * 3"
  },
  "execution_time_ms": 5
}
```

---

## 任务端点

### 列出任务

获取所有调度的任务。

```http
GET /tasks
```

**查询参数：**

| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| status | string | all | 按状态过滤 |
| limit | int | 50 | 最大结果数 |

**响应：**

```json
{
  "tasks": [
    {
      "id": "uuid",
      "name": "backup-task",
      "status": "completed",
      "priority": 1,
      "scheduled_at": "2024-01-15T02:00:00Z",
      "started_at": "2024-01-15T02:00:01Z",
      "completed_at": "2024-01-15T02:05:00Z"
    }
  ],
  "stats": {
    "total": 100,
    "pending": 5,
    "running": 2,
    "completed": 90,
    "failed": 3
  }
}
```

---

### 获取任务

获取特定任务的详情。

```http
GET /tasks/:id
```

**响应：**

```json
{
  "id": "uuid",
  "name": "backup-task",
  "status": "completed",
  "priority": 1,
  "metadata": {
    "type": "backup",
    "target": "/data"
  },
  "scheduled_at": "2024-01-15T02:00:00Z",
  "started_at": "2024-01-15T02:00:01Z",
  "completed_at": "2024-01-15T02:05:00Z",
  "retry_count": 0,
  "max_retries": 3
}
```

---

### 取消任务

取消待处理或正在运行的任务。

```http
POST /tasks/:id/cancel
```

**响应：**

```json
{
  "message": "任务已取消",
  "task": {
    "id": "uuid",
    "status": "cancelled"
  }
}
```

---

## 系统端点

### 健康检查

检查服务是否健康。

```http
GET /health
```

**响应：**

```json
{
  "status": "ok",
  "version": "0.5.0",
  "uptime": "24h30m15s"
}
```

---

### 获取指标

获取 Prometheus 指标（如果启用）。

```http
GET /metrics
```

**响应：** Prometheus 文本格式

```
# HELP echo_http_requests_total Total HTTP requests
# TYPE echo_http_requests_total counter
echo_http_requests_total{method="GET",path="/api/v1/chat",status="200"} 1234
...
```

---

### 重载配置

触发配置重载。

```http
POST /config/reload
```

**所需权限：** `admin`

**响应：**

```json
{
  "message": "配置已重载",
  "changes": ["llm.providers.openai.model"]
}
```

---

## 错误响应

所有错误遵循一致的格式：

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "无效的请求体",
    "details": {
      "field": "message",
      "reason": "required"
    }
  }
}
```

### 错误码

| 错误码 | HTTP 状态 | 描述 |
|--------|-----------|------|
| `UNAUTHORIZED` | 401 | 缺少或无效的认证 |
| `FORBIDDEN` | 403 | 权限不足 |
| `NOT_FOUND` | 404 | 资源未找到 |
| `VALIDATION_ERROR` | 400 | 无效的请求数据 |
| `RATE_LIMITED` | 429 | 请求过多 |
| `INTERNAL_ERROR` | 500 | 服务器错误 |
| `SERVICE_UNAVAILABLE` | 503 | 服务暂时不可用 |

---

## 速率限制

API 请求按客户端进行速率限制：

- **默认：** 10 请求/秒，突发 20
- **已认证：** 可按用户/角色配置

响应中包含速率限制头：

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1705312800
```

---

## WebSocket API

### 聊天流

连接以接收实时聊天更新。

```
ws://localhost:8080/api/v1/ws/chat
```

**认证：** 将令牌作为查询参数包含：

```
ws://localhost:8080/api/v1/ws/chat?token=<jwt-token>
```

**消息格式：**

```json
{
  "type": "message",
  "data": {
    "conversation_id": "uuid",
    "message": "你好！"
  }
}
```

**事件类型：**

| 类型 | 描述 |
|------|------|
| `message` | 发送聊天消息 |
| `typing` | 用户正在输入指示器 |
| `response_start` | 助手开始响应 |
| `response_chunk` | 部分响应内容 |
| `response_end` | 响应完成 |
| `error` | 发生错误 |

---

## SDK 示例

### Go

```go
package main

import (
    "github.com/IceWhaleTech/ZimaOS-Echo/sdk/go/echo"
)

func main() {
    client := echo.NewClient("http://localhost:8080", "your-api-key")

    resp, err := client.Chat("你好，Echo！")
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Message)
}
```

### Python

```python
from echo_sdk import EchoClient

client = EchoClient("http://localhost:8080", api_key="your-api-key")

response = client.chat("你好，Echo！")
print(response.message)
```

### JavaScript

```javascript
import { EchoClient } from '@zimaos/echo-sdk';

const client = new EchoClient('http://localhost:8080', {
  apiKey: 'your-api-key'
});

const response = await client.chat('你好，Echo！');
console.log(response.message);
```

---

## 更新日志

### v0.5.0
- 添加任务调度端点
- 添加 WebSocket 支持
- 添加速率限制头

### v0.4.0
- 添加插件管理端点
- 添加 Skill 执行端点
- 添加 API 密钥管理
- 添加 RBAC 权限
