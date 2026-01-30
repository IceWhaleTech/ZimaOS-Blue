# API Proxy Sidecar - API Reference

> Version: 0.10.5

## Overview

The API Proxy Sidecar provides intelligent routing, failover, security monitoring, and usage statistics for LLM API calls.

## Base URL

```
http://localhost:{port}/api/v1/proxy
```

The port is dynamically allocated and can be found in the port file or environment variable.

---

## Core Endpoints

### GET /status

Get proxy server status.

**Response:**
```json
{
  "status": "running",
  "port": 8080,
  "bind_address": "127.0.0.1",
  "endpoint": "http://127.0.0.1:8080",
  "uptime": "1h30m45s",
  "version": "0.10.5.1"
}
```

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### GET /providers

List all configured providers.

**Response:**
```json
{
  "providers": [
    {
      "name": "openai",
      "endpoint": "https://api.openai.com/v1",
      "priority": 1,
      "enabled": true,
      "healthy": true,
      "last_check": "2024-01-15T10:30:00Z",
      "last_latency_ms": 150,
      "circuit_state": "closed"
    }
  ],
  "default_provider": "openai",
  "load_balancing": "priority"
}
```

---

## Session Endpoints

### GET /sessions

List active sessions.

**Query Parameters:**
- `active` (boolean): Filter to active sessions only

**Response:**
```json
{
  "sessions": [
    {
      "id": "sess_abc123",
      "start_time": "2024-01-15T10:30:00Z",
      "last_activity": "2024-01-15T10:31:00Z",
      "provider": "openai",
      "model": "gpt-4",
      "tokens_in": 1500,
      "tokens_out": 500,
      "status": "active",
      "request_count": 3,
      "error_count": 0,
      "client_ip": "127.0.0.1"
    }
  ],
  "stats": {
    "enabled": true,
    "total_sessions": 100,
    "active_sessions": 5,
    "completed": 90,
    "failed": 5,
    "total_tokens_in": 150000,
    "total_tokens_out": 50000,
    "total_requests": 500
  }
}
```

### GET /sessions/:id

Get session details by ID.

**Response:**
```json
{
  "id": "sess_abc123",
  "start_time": "2024-01-15T10:30:00Z",
  "last_activity": "2024-01-15T10:31:00Z",
  "provider": "openai",
  "model": "gpt-4",
  "tokens_in": 1500,
  "tokens_out": 500,
  "status": "active",
  "request_count": 3,
  "error_count": 0,
  "client_ip": "127.0.0.1",
  "user_agent": "Mozilla/5.0..."
}
```

---

## Metrics Endpoints

### GET /metrics

Get usage metrics summary.

**Response:**
```json
{
  "enabled": true,
  "total_requests": 1000,
  "total_success": 950,
  "total_errors": 50,
  "success_rate": 95.0,
  "avg_latency_ms": 250,
  "avg_ttft_ms": 100,
  "avg_proxy_overhead": 5,
  "p95_latency_ms": 500,
  "p99_latency_ms": 800,
  "tokens_per_sec": 150.5,
  "total_tokens_in": 500000,
  "total_tokens_out": 200000,
  "total_bytes": 10485760,
  "provider_count": 3,
  "bucket_count": 24
}
```

### GET /metrics/providers

Get per-provider metrics.

**Query Parameters:**
- `provider` (string): Filter to specific provider

**Response:**
```json
{
  "providers": {
    "openai": {
      "name": "openai",
      "request_count": 500,
      "success_count": 480,
      "error_count": 20,
      "total_latency_ms": 125000,
      "avg_latency_ms": 250,
      "total_ttft_ms": 50000,
      "avg_ttft_ms": 100,
      "total_tokens_in": 250000,
      "total_tokens_out": 100000,
      "tokens_per_sec": 75.5
    }
  }
}
```

### GET /metrics/latency

Get latency statistics.

**Response:**
```json
{
  "avg_ms": 250,
  "min_ms": 50,
  "max_ms": 2000,
  "p50_ms": 200,
  "p90_ms": 400,
  "p95_ms": 500,
  "p99_ms": 800,
  "sample_count": 1000
}
```

### GET /metrics/timeseries

Get time-series metrics data.

**Query Parameters:**
- `start` (RFC3339): Start time (default: 24 hours ago)
- `end` (RFC3339): End time (default: now)

**Response:**
```json
{
  "buckets": [
    {
      "timestamp": "2024-01-15T10:00:00Z",
      "requests": 50,
      "errors": 2,
      "avg_latency_ms": 230
    }
  ],
  "start": "2024-01-14T10:00:00Z",
  "end": "2024-01-15T10:00:00Z"
}
```

---

## Guard Endpoints

### GET /guard/stats

Get prompt guard statistics.

**Response:**
```json
{
  "enabled": true,
  "block_on_detect": false,
  "pattern_count": 15,
  "whitelist_count": 2,
  "detection_count": 25,
  "blocked_count": 5,
  "max_prompt_length": 100000
}
```

### GET /guard/rules

List guard rules.

**Response:**
```json
{
  "rules": [
    {
      "id": "custom-0",
      "name": "Custom Rule 1",
      "pattern": "(?i)ignore.*instructions",
      "enabled": true,
      "risk_level": "medium",
      "action": "log"
    }
  ]
}
```

### POST /guard/rules

Add a guard rule.

**Request:**
```json
{
  "name": "Block jailbreak",
  "pattern": "(?i)jailbreak",
  "description": "Blocks jailbreak attempts",
  "enabled": true,
  "risk_level": "high",
  "action": "block"
}
```

**Response:**
```json
{
  "message": "Rule added successfully",
  "rule": {
    "id": "custom-1",
    "name": "Block jailbreak",
    "pattern": "(?i)jailbreak",
    "enabled": true,
    "risk_level": "high",
    "action": "block"
  }
}
```

---

## Auth Endpoints

### GET /auth/stats

Get authentication statistics.

**Response:**
```json
{
  "auth_enabled": true,
  "auth_type": "api_key",
  "api_key_count": 3,
  "allowed_ip_count": 5,
  "auth_failures": 10,
  "rate_limit_enabled": true,
  "requests_per_min": 60,
  "burst_size": 10,
  "rate_limit_hits": 5,
  "active_limiters": 3
}
```

### GET /auth/keys

List API keys (masked).

**Response:**
```json
{
  "keys": [
    {
      "id": 0,
      "key": "sk-a****xyz",
      "active": true
    }
  ]
}
```

### POST /auth/keys

Add an API key.

**Request:**
```json
{
  "key": "sk-your-api-key-here"
}
```

**Response:**
```json
{
  "message": "API key added successfully"
}
```

### DELETE /auth/keys

Remove an API key.

**Query Parameters:**
- `key` (string): The API key to remove

**Response:**
```json
{
  "message": "API key removed successfully"
}
```

---

## Model Endpoints

### GET /models

List model compatibility information.

**Query Parameters:**
- `model` (string): Get features for specific model

**Response:**
```json
{
  "models": {
    "gpt-4": {
      "name": "gpt-4",
      "provider": "openai",
      "max_context": 128000,
      "max_output": 4096,
      "supports_streaming": true,
      "supports_functions": true,
      "supports_vision": true,
      "tool_calling": true
    }
  }
}
```

---

## Mock Endpoints

### GET /mock

List mock endpoints.

**Response:**
```json
{
  "endpoints": [
    {
      "path": "/v1/chat/completions",
      "method": "POST",
      "enabled": true
    }
  ],
  "enabled": false
}
```

### POST /mock

Add a mock endpoint.

**Request:**
```json
{
  "path": "/v1/chat/completions",
  "method": "POST",
  "response": {
    "choices": [{"message": {"content": "Mock response"}}]
  },
  "status_code": 200,
  "enabled": true
}
```

**Response:**
```json
{
  "message": "Mock endpoint added successfully",
  "endpoint": {...}
}
```

---

## Data Masking Endpoints

### GET /masking/stats

Get data masking statistics.

**Response:**
```json
{
  "enabled": false,
  "rule_count": 0,
  "total_masks": 0,
  "mask_counts": {},
  "status": "stub"
}
```

### GET /masking/rules

List masking rules.

**Response:**
```json
{
  "rules": [],
  "default_rules": [
    {
      "id": "email",
      "name": "Email Address",
      "category": "pii",
      "pattern": "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}",
      "replacement": "[EMAIL]",
      "direction": "both",
      "enabled": false
    }
  ]
}
```

### POST /masking/rules

Add a masking rule.

**Request:**
```json
{
  "id": "custom-ssn",
  "name": "SSN Pattern",
  "category": "pii",
  "pattern": "\\d{3}-\\d{2}-\\d{4}",
  "replacement": "[SSN]",
  "direction": "both",
  "enabled": true
}
```

### DELETE /masking/rules

Remove a masking rule.

**Query Parameters:**
- `id` (string): Rule ID to remove

---

## Config Endpoints

### POST /config/reload

Force configuration reload.

**Response:**
```json
{
  "message": "Configuration reloaded successfully",
  "reloaded_at": "2024-01-15T10:30:00Z"
}
```

---

## Failover Endpoints

### GET /failover/metrics

Get failover metrics.

**Response:**
```json
{
  "total_failovers": 10,
  "successful_failovers": 8,
  "failed_failovers": 2,
  "avg_failover_time_ms": 150
}
```

### GET /failover/config

Get failover configuration.

### PUT /failover/config

Update failover configuration.

### POST /failover/reset

Reset circuit breakers.

### GET /failover/breakers

Get circuit breaker status.

---

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": "Error message description"
}
```

Common HTTP status codes:
- `400` - Bad Request
- `401` - Unauthorized
- `404` - Not Found
- `405` - Method Not Allowed
- `500` - Internal Server Error
