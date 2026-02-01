# API Reference

ZimaOS Echo provides a RESTful API for monitoring and management.

## Base URL

```
http://localhost:23456
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Full health status |
| GET | `/health/live` | Liveness probe |
| GET | `/health/ready` | Readiness probe |
| GET | `/api/v1/workers/stats` | Worker pool statistics |

## Authentication

Currently, the API does not require authentication. Authentication will be added in v0.4.0.

## Response Format

All responses are in JSON format:

```json
{
  "status": "ok",
  "data": { ... }
}
```

## Error Responses

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message"
  }
}
```

## Rate Limiting

No rate limiting is currently implemented. This will be added in future versions.
