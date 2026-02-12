# API Reference

This document provides a comprehensive reference for the ZimaOS Blue REST API.

## Base URL

```
http://localhost:23456/api
```

## Authentication

Most endpoints require authentication. Include the JWT token in the Authorization header:

```
Authorization: Bearer <token>
```

### Obtain Token

```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "your-password"
}
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-01T12:00:00Z"
}
```

---

## Chat API

### Send Message

```http
POST /api/chat/message
Content-Type: application/json
Authorization: Bearer <token>

{
  "message": "Hello, how are you?",
  "session_id": "optional-session-id",
  "attachments": []
}
```

Response:
```json
{
  "id": "msg_123",
  "response": "I'm doing well, thank you for asking!",
  "session_id": "sess_456",
  "created_at": "2024-01-01T12:00:00Z"
}
```

### Stream Message

```http
POST /api/chat/stream
Content-Type: application/json
Authorization: Bearer <token>

{
  "message": "Tell me a story",
  "session_id": "sess_456"
}
```

Response (Server-Sent Events):
```
data: {"type": "start", "id": "msg_789"}

data: {"type": "content", "content": "Once upon a time"}

data: {"type": "content", "content": " there was a..."}

data: {"type": "end", "usage": {"prompt_tokens": 10, "completion_tokens": 50}}
```

### Get Session History

```http
GET /api/chat/sessions/{session_id}/messages
Authorization: Bearer <token>
```

Response:
```json
{
  "messages": [
    {
      "id": "msg_123",
      "role": "user",
      "content": "Hello",
      "created_at": "2024-01-01T12:00:00Z"
    },
    {
      "id": "msg_124",
      "role": "assistant",
      "content": "Hi there!",
      "created_at": "2024-01-01T12:00:01Z"
    }
  ],
  "total": 2
}
```

---

## LLM Configuration

### List Providers

```http
GET /api/llm/providers
Authorization: Bearer <token>
```

Response:
```json
{
  "providers": [
    {
      "id": "openai",
      "name": "OpenAI",
      "models": ["gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo"],
      "status": "healthy"
    },
    {
      "id": "ollama",
      "name": "Ollama",
      "models": ["llama3.2", "mistral"],
      "status": "healthy"
    }
  ]
}
```

### Get Provider Health

```http
GET /api/llm/health
Authorization: Bearer <token>
```

Response:
```json
{
  "providers": {
    "openai": {
      "status": "healthy",
      "latency_ms": 150,
      "last_check": "2024-01-01T12:00:00Z"
    }
  }
}
```

---

## Home Assistant Integration

### Get Entities

```http
GET /api/homeassistant/entities
Authorization: Bearer <token>
```

Response:
```json
{
  "entities": [
    {
      "entity_id": "light.living_room",
      "state": "on",
      "attributes": {
        "brightness": 255,
        "friendly_name": "Living Room Light"
      }
    }
  ]
}
```

### Call Service

```http
POST /api/homeassistant/services/{domain}/{service}
Content-Type: application/json
Authorization: Bearer <token>

{
  "entity_id": "light.living_room",
  "brightness": 128
}
```

Response:
```json
{
  "success": true,
  "result": []
}
```

### Execute Command (Natural Language)

```http
POST /api/homeassistant/command
Content-Type: application/json
Authorization: Bearer <token>

{
  "command": "Turn on the living room lights"
}
```

Response:
```json
{
  "success": true,
  "action": "turn_on",
  "entities": ["light.living_room"],
  "message": "Turned on Living Room Light"
}
```

---

## Plugins

### List Plugins

```http
GET /api/plugins
Authorization: Bearer <token>
```

Response:
```json
{
  "plugins": [
    {
      "id": "weather",
      "name": "Weather Plugin",
      "version": "1.0.0",
      "enabled": true,
      "status": "running"
    }
  ]
}
```

### Enable/Disable Plugin

```http
POST /api/plugins/{plugin_id}/enable
Authorization: Bearer <token>
```

```http
POST /api/plugins/{plugin_id}/disable
Authorization: Bearer <token>
```

### Get Plugin Config

```http
GET /api/plugins/{plugin_id}/config
Authorization: Bearer <token>
```

### Update Plugin Config

```http
PUT /api/plugins/{plugin_id}/config
Content-Type: application/json
Authorization: Bearer <token>

{
  "api_key": "your-api-key",
  "location": "New York"
}
```

---

## Backup & Restore

### Create Backup

```http
POST /api/backup/create
Content-Type: application/json
Authorization: Bearer <token>

{
  "type": "full"  // full, config, data
}
```

Response:
```json
{
  "id": "backup_123",
  "type": "full",
  "size_bytes": 1234567,
  "created_at": "2024-01-01T12:00:00Z",
  "path": "/backups/backup_full_20240101_120000.tar.gz"
}
```

### List Backups

```http
GET /api/backup/list
Authorization: Bearer <token>
```

### Restore Backup

```http
POST /api/backup/restore/{backup_id}
Authorization: Bearer <token>
```

### Delete Backup

```http
DELETE /api/backup/{backup_id}
Authorization: Bearer <token>
```

---

## System

### Health Check

```http
GET /health
```

Response:
```json
{
  "status": "healthy",
  "version": "0.5.0",
  "uptime": "24h30m15s"
}
```

### System Info

```http
GET /api/system/info
Authorization: Bearer <token>
```

Response:
```json
{
  "version": "0.5.0",
  "go_version": "1.21.0",
  "os": "linux",
  "arch": "amd64",
  "goroutines": 42,
  "memory": {
    "alloc": 12345678,
    "total_alloc": 123456789,
    "sys": 234567890
  }
}
```

### Get Metrics

```http
GET /metrics
```

Returns Prometheus-format metrics.

---

## Setup Wizard

### Get Setup Status

```http
GET /api/setup/status
```

Response:
```json
{
  "completed": false,
  "current_step": 1,
  "total_steps": 5,
  "version": "0.5.0"
}
```

### Get Defaults

```http
GET /api/setup/defaults
```

### Validate Step

```http
POST /api/setup/validate
Content-Type: application/json

{
  "step": 1,
  "config": {
    "language": "en",
    "timezone": "UTC"
  }
}
```

### Complete Setup

```http
POST /api/setup/complete
Content-Type: application/json

{
  "language": "en",
  "timezone": "UTC",
  "llm_provider": "openai",
  "llm_api_key": "sk-...",
  "llm_model": "gpt-4o-mini",
  "admin_username": "admin",
  "admin_password": "secure-password"
}
```

---

## Grayscale / Feature Flags

### List Flags

```http
GET /api/grayscale/flags
Authorization: Bearer <token>
```

### Evaluate Flag

```http
POST /api/grayscale/flags/evaluate
Content-Type: application/json
Authorization: Bearer <token>

{
  "flag_name": "new_feature",
  "user_id": "user_123",
  "attributes": {
    "plan": "premium"
  }
}
```

Response:
```json
{
  "enabled": true,
  "variant": "treatment_a"
}
```

---

## Error Responses

All endpoints return errors in a consistent format:

```json
{
  "error": "error_code",
  "message": "Human-readable error message",
  "details": {}
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `unauthorized` | 401 | Missing or invalid authentication |
| `forbidden` | 403 | Insufficient permissions |
| `not_found` | 404 | Resource not found |
| `validation_error` | 400 | Invalid request parameters |
| `rate_limited` | 429 | Too many requests |
| `internal_error` | 500 | Server error |

---

## Rate Limiting

API requests are rate limited:

- **Default**: 100 requests per minute per user
- **Chat**: 20 messages per minute per user
- **Streaming**: 5 concurrent streams per user

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1704067200
```

---

## Pagination

List endpoints support pagination:

```http
GET /api/resource?page=1&per_page=20
```

Response includes pagination metadata:
```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

---

## WebSocket API

### Chat WebSocket

```
ws://localhost:23456/ws/chat
```

Connect with token:
```javascript
const ws = new WebSocket('ws://localhost:23456/ws/chat?token=<jwt-token>');
```

Message format:
```json
{
  "type": "message",
  "content": "Hello",
  "session_id": "sess_123"
}
```

Response events:
```json
{"type": "connected", "session_id": "sess_123"}
{"type": "typing"}
{"type": "message", "content": "Hi there!", "id": "msg_456"}
{"type": "error", "message": "Error description"}
```
