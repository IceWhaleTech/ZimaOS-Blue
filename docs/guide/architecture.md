# Architecture

This document describes the architecture of ZimaOS Echo.

## Overview

ZimaOS Echo is a NAS-native Agent Runtime built with Go, designed for low-power devices. It provides a lightweight, high-performance platform for running AI agents.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           ZimaOS-Echo                                    │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Frontend Layer                                │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ Vue 3 UI │ │ REST API │ │WebSocket │ │   Prometheus     │   │    │
│  │  │Dashboard │ │ Handlers │ │  Server  │ │    Metrics       │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    Core Runtime                                  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │  Event   │ │  Worker  │ │  Config  │ │     Logger       │   │    │
│  │  │   Loop   │ │   Pool   │ │ Manager  │ │   (Structured)   │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    Agent Runtime                                 │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │   LLM    │ │  Tools   │ │  Memory  │ │    Context       │   │    │
│  │  │ Provider │ │ Registry │ │  Store   │ │   Management     │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    Data Layer                                    │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │  SQLite  │ │  ECache  │ │  Files   │ │     Backup       │   │    │
│  │  │  (Zorm)  │ │  (LRU)   │ │ Storage  │ │    Manager       │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Frontend Layer

- **Vue 3 Dashboard**: Modern web interface for monitoring and management
- **REST API**: RESTful endpoints for all operations
- **WebSocket Server**: Real-time communication for streaming responses
- **Prometheus Metrics**: Observability and monitoring

### 2. Core Runtime

- **Event Loop**: Asynchronous event processing
- **Worker Pool**: Auto-scaling goroutine pool for concurrent tasks
- **Config Manager**: Hot-reloadable configuration with validation
- **Logger**: Structured logging with multiple outputs

### 3. Agent Runtime

- **LLM Provider**: Multi-provider support (OpenAI, Anthropic, Ollama, etc.)
- **Tools Registry**: Extensible tool system
- **Memory Store**: Conversation history and context
- **Context Management**: Token counting and context window management

### 4. Data Layer

- **SQLite (Zorm)**: Lightweight ORM for persistent storage
- **ECache (LRU)**: High-performance in-memory caching
- **File Storage**: Local file management
- **Backup Manager**: Automated backup and restore

## Key Design Principles

### 1. Lightweight

- Single binary under 15MB
- Memory usage < 80MB at idle
- Minimal dependencies

### 2. High Performance

- Goroutine-based concurrency
- Lock-free data structures where possible
- Efficient memory pooling

### 3. Stability

- Graceful shutdown handling
- Panic recovery middleware
- Circuit breaker pattern for external calls
- Automatic retry with exponential backoff

### 4. Extensibility

- Plugin-based architecture
- Go modules or WASM support
- Hook system for customization

## Data Flow

```
User Request
     │
     ▼
┌─────────────┐
│  HTTP/WS    │
│   Server    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Middleware │ ← Rate Limiting, Auth, Logging
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Router    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Handler   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Service   │ ← Business Logic
└──────┬──────┘
       │
       ▼
┌─────────────┐
│    Agent    │ ← LLM Interaction
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Provider   │ ← OpenAI/Anthropic/Ollama
└─────────────┘
```

## Resilience Patterns

### Circuit Breaker

Protects against cascading failures when external services are unavailable.

```go
// Circuit breaker states
CLOSED → OPEN → HALF_OPEN → CLOSED
```

### Bulkhead

Isolates resources to prevent one component from affecting others.

### Graceful Degradation

Falls back to cached responses or static content when services are degraded.

## Performance Optimizations

### Caching Strategy

- **L1 Cache (ECache)**: In-memory LRU cache for hot data
- **L2 Cache (Disk)**: Persistent cache for larger datasets
- **Multi-level**: Automatic promotion/demotion between levels

### Connection Pooling

- Database connection pool with health checks
- HTTP client connection reuse
- WebSocket connection management

### Memory Management

- Object pooling for frequent allocations
- Buffer reuse with sync.Pool
- Efficient JSON serialization

## Security

### Authentication

- API key authentication
- JWT token support
- Session management

### Authorization

- Role-based access control
- Resource-level permissions

### Data Protection

- Encrypted storage for sensitive data
- Secure configuration handling
- Audit logging

## Monitoring

### Metrics

- Request latency (P50, P95, P99)
- Error rates
- Resource usage (CPU, memory, goroutines)
- LLM token usage and costs

### Profiling

- CPU profiling via pprof
- Memory profiling
- Goroutine analysis
- Block profiling

### Logging

- Structured JSON logging
- Log levels (debug, info, warn, error)
- Request tracing
