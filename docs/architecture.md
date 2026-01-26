# ZimaOS-Echo Architecture

[中文版本](./zh/architecture.md)

## Overview

ZimaOS-Echo is a lightweight, self-hosted AI assistant designed for NAS environments. It provides a conversational interface to interact with various LLM providers while maintaining privacy and control over your data.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Layer                              │
├─────────────────────────────────────────────────────────────────┤
│  Web UI (Vue 3)  │  REST API  │  WebSocket  │  CLI              │
└────────┬─────────┴─────┬──────┴──────┬──────┴────────┬──────────┘
         │               │             │               │
         ▼               ▼             ▼               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      API Gateway Layer                           │
├─────────────────────────────────────────────────────────────────┤
│  Echo Server (labstack/echo)                                     │
│  ├── Authentication Middleware (JWT/API Key)                     │
│  ├── RBAC Middleware                                             │
│  ├── Rate Limiting                                               │
│  ├── Request Logging                                             │
│  └── Metrics Collection                                          │
└────────┬─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Business Logic Layer                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   Chat      │  │   Plugin    │  │   Skill     │              │
│  │   Service   │  │   System    │  │   Hub       │              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         │                │                │                      │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐              │
│  │  Context    │  │  Registry   │  │  Executor   │              │
│  │  Manager    │  │  & Loader   │  │  & Registry │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└────────┬─────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   LLM       │  │   Task      │  │   KV        │              │
│  │   Provider  │  │   Scheduler │  │   Store     │              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         │                │                │                      │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐              │
│  │  OpenAI     │  │  Priority   │  │  SQLite     │              │
│  │  Anthropic  │  │  Queue      │  │  BadgerDB   │              │
│  │  Ollama     │  │  Workers    │  │             │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. API Gateway Layer

The API gateway is built on [Echo](https://echo.labstack.com/), a high-performance Go web framework.

**Key Features:**
- RESTful API endpoints
- WebSocket support for streaming responses
- Middleware chain for cross-cutting concerns
- Request/response logging
- Prometheus metrics integration

### 2. Authentication & Authorization

#### JWT Authentication
- Access tokens with configurable expiration
- Refresh token mechanism
- Token blacklist for revocation
- Secure token storage

#### API Key Management
- Scoped API keys for programmatic access
- SHA-256 hashed storage
- Usage tracking and rate limiting

#### RBAC (Role-Based Access Control)
- Predefined roles: admin, user, readonly
- Wildcard permission matching
- Role inheritance support

### 3. Plugin System

The plugin system supports multiple plugin types:

#### Native Go Plugins
- Compile-time integration
- Full access to internal APIs
- Best performance

#### JavaScript Plugins (clawdbot compatible)
- Runtime loading via goja
- clawdbot manifest format support
- Sandboxed execution

**Plugin Lifecycle:**
```
Load → Initialize → Start → Running → Stop → Destroy
```

### 4. Skill Hub

Skills are self-contained units of functionality:

**Built-in Skills:**
- Calculator - arithmetic operations
- DateTime - date/time utilities
- SystemInfo - system information
- Weather - weather data (with API)
- Search - text search utilities

**Skill Interface:**
```go
type Skill interface {
    Manifest() *Manifest
    Execute(ctx context.Context, input map[string]any) (*Result, error)
    Validate(input map[string]any) error
}
```

### 5. Task Scheduler

Priority-based task scheduling with:
- Concurrent execution limits
- Task dependencies
- Timeout handling
- Retry with exponential backoff
- Task cancellation

### 6. LLM Provider Abstraction

Unified interface for multiple LLM providers:

| Provider | Streaming | Function Calling |
|----------|-----------|------------------|
| OpenAI   | ✅        | ✅               |
| Anthropic| ✅        | ✅               |
| Ollama   | ✅        | ⚠️ Limited       |

### 7. Observability

#### Metrics (Prometheus)
- Runtime metrics (goroutines, memory, GC)
- HTTP request metrics
- LLM provider metrics
- Custom business metrics

#### Profiling (pprof)
- CPU profiling
- Memory profiling
- Goroutine analysis
- Block/mutex profiling

### 8. Resilience

- Circuit breaker for external calls
- Rate limiting per client
- Graceful degradation
- Hot configuration reload

## Data Flow

### Chat Request Flow

```
1. Client sends message
         │
         ▼
2. Authentication middleware validates token
         │
         ▼
3. Rate limiter checks quota
         │
         ▼
4. Chat service receives request
         │
         ▼
5. Context manager loads conversation history
         │
         ▼
6. LLM provider sends request to AI
         │
         ▼
7. Response streamed back to client
         │
         ▼
8. Conversation saved to storage
```

### Plugin Execution Flow

```
1. Plugin loaded from directory
         │
         ▼
2. Manifest validated
         │
         ▼
3. Plugin registered in registry
         │
         ▼
4. Initialize called with API
         │
         ▼
5. Start called to begin operation
         │
         ▼
6. Tools/commands available to system
```

## Directory Structure

```
ZimaOS-Echo/
├── server/                 # Go backend
│   ├── cmd/echo/          # Main entry point
│   └── internal/          # Internal packages
│       ├── auth/          # Authentication
│       ├── rbac/          # Authorization
│       ├── plugin/        # Plugin system
│       ├── skill/         # Skill hub
│       ├── scheduler/     # Task scheduler
│       ├── llm/           # LLM providers
│       ├── config/        # Configuration
│       ├── metrics/       # Prometheus metrics
│       ├── profiling/     # pprof integration
│       ├── resilience/    # Circuit breaker
│       └── backup/        # Backup/restore
├── web/                   # Vue 3 frontend
│   ├── src/
│   │   ├── components/    # Vue components
│   │   ├── views/         # Page views
│   │   ├── stores/        # Pinia stores
│   │   └── api/           # API client
│   └── public/            # Static assets
├── docs/                  # Documentation
├── scripts/               # Deployment scripts
└── deploy/                # Deployment configs
```

## Configuration

Configuration is managed via YAML files with support for:
- Environment variable substitution
- Hot reload without restart
- Validation before apply
- Rollback on invalid config

See [Configuration Guide](./configuration.md) for details.

## Security Considerations

1. **Authentication**: All API endpoints require authentication
2. **Authorization**: RBAC enforces least-privilege access
3. **Data Privacy**: All data stored locally, no external telemetry
4. **API Keys**: Hashed storage, scoped permissions
5. **Plugin Isolation**: Sandboxed JavaScript execution

## Performance

- **Concurrent Requests**: Configurable worker pools
- **Memory**: Efficient streaming for large responses
- **Startup Time**: < 2 seconds typical
- **Response Latency**: Dominated by LLM provider latency

## Deployment Options

1. **Standalone Binary**: Single executable, no dependencies
2. **Docker Container**: Official Docker image available
3. **Systemd Service**: Linux service integration
4. **ZimaOS App**: Native ZimaOS application

See [Installation Guide](./installation.md) for deployment instructions.
