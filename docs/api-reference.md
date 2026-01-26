# API Reference

[中文版本](./zh/api-reference.md)

This document provides a complete reference for the ZimaOS-Echo REST API.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Most endpoints require authentication. ZimaOS-Echo supports two authentication methods:

### JWT Token

Include the token in the Authorization header:

```
Authorization: Bearer <token>
```

### API Key

Include the API key in the X-API-Key header:

```
X-API-Key: <api-key>
```

---

## Authentication Endpoints

### Login

Authenticate a user and receive JWT tokens.

```http
POST /auth/login
```

**Request Body:**

```json
{
  "username": "string",
  "password": "string"
}
```

**Response:**

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

**Status Codes:**
- `200` - Success
- `401` - Invalid credentials
- `429` - Too many attempts

---

### Logout

Invalidate the current token.

```http
POST /auth/logout
```

**Headers:**
- `Authorization: Bearer <token>` (required)

**Response:**

```json
{
  "message": "Successfully logged out"
}
```

---

### Refresh Token

Get a new access token using a refresh token.

```http
POST /auth/refresh
```

**Request Body:**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-15T12:00:00Z"
}
```

---

### Get Current User

Get information about the authenticated user.

```http
GET /auth/me
```

**Response:**

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

## API Key Endpoints

### List API Keys

Get all API keys for the current user.

```http
GET /apikeys
```

**Response:**

```json
{
  "keys": [
    {
      "id": "uuid",
      "name": "My API Key",
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

### Create API Key

Create a new API key.

```http
POST /apikeys
```

**Request Body:**

```json
{
  "name": "My API Key",
  "scopes": ["chat", "skills.execute"],
  "expires_in": "8760h"
}
```

**Response:**

```json
{
  "id": "uuid",
  "name": "My API Key",
  "key": "ek_abc123...",
  "scopes": ["chat", "skills.execute"],
  "created_at": "2024-01-01T00:00:00Z",
  "expires_at": "2025-01-01T00:00:00Z"
}
```

> **Note:** The full API key is only returned once at creation time. Store it securely.

---

### Delete API Key

Delete an API key.

```http
DELETE /apikeys/:id
```

**Response:**

```json
{
  "message": "API key deleted"
}
```

---

## Chat Endpoints

### Send Message

Send a message and receive a response.

```http
POST /chat
```

**Request Body:**

```json
{
  "message": "Hello, how are you?",
  "conversation_id": "uuid",
  "provider": "openai",
  "model": "gpt-4",
  "options": {
    "temperature": 0.7,
    "max_tokens": 1000
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| message | string | Yes | The user's message |
| conversation_id | string | No | Continue existing conversation |
| provider | string | No | LLM provider override |
| model | string | No | Model override |
| options | object | No | Generation options |

**Response:**

```json
{
  "id": "uuid",
  "conversation_id": "uuid",
  "message": "I'm doing well, thank you for asking!",
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

### Stream Message

Send a message and receive a streaming response.

```http
POST /chat/stream
```

**Request Body:** Same as `/chat`

**Response:** Server-Sent Events (SSE)

```
data: {"type":"start","conversation_id":"uuid"}

data: {"type":"content","content":"I'm"}

data: {"type":"content","content":" doing"}

data: {"type":"content","content":" well"}

data: {"type":"done","usage":{"prompt_tokens":10,"completion_tokens":15}}
```

---

### List Conversations

Get all conversations for the current user.

```http
GET /conversations
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| limit | int | 20 | Max results |
| offset | int | 0 | Pagination offset |

**Response:**

```json
{
  "conversations": [
    {
      "id": "uuid",
      "title": "Chat about weather",
      "message_count": 10,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T11:00:00Z"
    }
  ],
  "total": 50
}
```

---

### Get Conversation

Get a specific conversation with messages.

```http
GET /conversations/:id
```

**Response:**

```json
{
  "id": "uuid",
  "title": "Chat about weather",
  "messages": [
    {
      "id": "uuid",
      "role": "user",
      "content": "What's the weather like?",
      "created_at": "2024-01-15T10:00:00Z"
    },
    {
      "id": "uuid",
      "role": "assistant",
      "content": "I can help you check the weather...",
      "created_at": "2024-01-15T10:00:01Z"
    }
  ],
  "created_at": "2024-01-15T10:00:00Z"
}
```

---

### Delete Conversation

Delete a conversation and all its messages.

```http
DELETE /conversations/:id
```

**Response:**

```json
{
  "message": "Conversation deleted"
}
```

---

## Plugin Endpoints

### List Plugins

Get all available plugins.

```http
GET /plugins
```

**Response:**

```json
{
  "plugins": [
    {
      "id": "example-plugin",
      "name": "Example Plugin",
      "version": "1.0.0",
      "description": "An example plugin",
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

### Get Plugin Details

Get detailed information about a plugin.

```http
GET /plugins/:id
```

**Response:**

```json
{
  "id": "example-plugin",
  "name": "Example Plugin",
  "version": "1.0.0",
  "description": "An example plugin",
  "author": "ZimaOS",
  "enabled": true,
  "type": "javascript",
  "tools": [
    {
      "name": "example_tool",
      "description": "Does something useful",
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
      "description": "Run example command"
    }
  ],
  "config": {
    "api_key": "***"
  }
}
```

---

### Enable Plugin

Enable a plugin.

```http
POST /plugins/:id/enable
```

**Response:**

```json
{
  "message": "Plugin enabled",
  "plugin": {
    "id": "example-plugin",
    "enabled": true
  }
}
```

---

### Disable Plugin

Disable a plugin.

```http
POST /plugins/:id/disable
```

**Response:**

```json
{
  "message": "Plugin disabled",
  "plugin": {
    "id": "example-plugin",
    "enabled": false
  }
}
```

---

### Update Plugin Config

Update plugin configuration.

```http
PUT /plugins/:id/config
```

**Request Body:**

```json
{
  "api_key": "new-api-key",
  "setting": "value"
}
```

**Response:**

```json
{
  "message": "Configuration updated"
}
```

---

## Skill Endpoints

### List Skills

Get all available skills.

```http
GET /skills
```

**Response:**

```json
{
  "skills": [
    {
      "id": "calculator",
      "name": "Calculator",
      "version": "1.0.0",
      "description": "Perform arithmetic calculations",
      "category": "utility",
      "enabled": true
    },
    {
      "id": "weather",
      "name": "Weather",
      "version": "1.0.0",
      "description": "Get weather information",
      "category": "information",
      "enabled": true
    }
  ]
}
```

---

### Get Skill Details

Get detailed information about a skill.

```http
GET /skills/:id
```

**Response:**

```json
{
  "id": "calculator",
  "name": "Calculator",
  "version": "1.0.0",
  "description": "Perform arithmetic calculations",
  "category": "utility",
  "enabled": true,
  "parameters": {
    "expression": {
      "type": "string",
      "description": "Mathematical expression to evaluate",
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

### Execute Skill

Execute a skill with given parameters.

```http
POST /skills/:id/execute
```

**Request Body:**

```json
{
  "expression": "2 + 2 * 3"
}
```

**Response:**

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

## Task Endpoints

### List Tasks

Get all scheduled tasks.

```http
GET /tasks
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| status | string | all | Filter by status |
| limit | int | 50 | Max results |

**Response:**

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

### Get Task

Get details of a specific task.

```http
GET /tasks/:id
```

**Response:**

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

### Cancel Task

Cancel a pending or running task.

```http
POST /tasks/:id/cancel
```

**Response:**

```json
{
  "message": "Task cancelled",
  "task": {
    "id": "uuid",
    "status": "cancelled"
  }
}
```

---

## System Endpoints

### Health Check

Check if the service is healthy.

```http
GET /health
```

**Response:**

```json
{
  "status": "ok",
  "version": "0.5.0",
  "uptime": "24h30m15s"
}
```

---

### Get Metrics

Get Prometheus metrics (if enabled).

```http
GET /metrics
```

**Response:** Prometheus text format

```
# HELP echo_http_requests_total Total HTTP requests
# TYPE echo_http_requests_total counter
echo_http_requests_total{method="GET",path="/api/v1/chat",status="200"} 1234
...
```

---

### Reload Configuration

Trigger a configuration reload.

```http
POST /config/reload
```

**Required Permission:** `admin`

**Response:**

```json
{
  "message": "Configuration reloaded",
  "changes": ["llm.providers.openai.model"]
}
```

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request body",
    "details": {
      "field": "message",
      "reason": "required"
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `VALIDATION_ERROR` | 400 | Invalid request data |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

---

## Rate Limiting

API requests are rate limited per client:

- **Default:** 10 requests/second, burst of 20
- **Authenticated:** Configurable per user/role

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1705312800
```

---

## WebSocket API

### Chat Stream

Connect to receive real-time chat updates.

```
ws://localhost:8080/api/v1/ws/chat
```

**Authentication:** Include token as query parameter:

```
ws://localhost:8080/api/v1/ws/chat?token=<jwt-token>
```

**Message Format:**

```json
{
  "type": "message",
  "data": {
    "conversation_id": "uuid",
    "message": "Hello!"
  }
}
```

**Event Types:**

| Type | Description |
|------|-------------|
| `message` | Send a chat message |
| `typing` | User is typing indicator |
| `response_start` | Assistant started responding |
| `response_chunk` | Partial response content |
| `response_end` | Response complete |
| `error` | Error occurred |

---

## SDK Examples

### Go

```go
package main

import (
    "github.com/IceWhaleTech/ZimaOS-Echo/sdk/go/echo"
)

func main() {
    client := echo.NewClient("http://localhost:8080", "your-api-key")

    resp, err := client.Chat("Hello, Echo!")
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

response = client.chat("Hello, Echo!")
print(response.message)
```

### JavaScript

```javascript
import { EchoClient } from '@zimaos/echo-sdk';

const client = new EchoClient('http://localhost:8080', {
  apiKey: 'your-api-key'
});

const response = await client.chat('Hello, Echo!');
console.log(response.message);
```

---

## Changelog

### v0.5.0
- Added task scheduling endpoints
- Added WebSocket support
- Added rate limiting headers

### v0.4.0
- Added plugin management endpoints
- Added skill execution endpoints
- Added API key management
- Added RBAC permissions
