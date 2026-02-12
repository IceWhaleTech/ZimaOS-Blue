# ZimaOS-Blue Roadmap

[中文版本](../i18n/zh_CN/DEV/ROADMAP.md)

This document describes the development direction and milestone planning for the ZimaOS-Blue project.

## Vision

Based on the design philosophy of [clawdbot](https://github.com/clawdbot/clawdbot), build a **NAS-native Agent Runtime** using Golang - lightweight, high-performance, optimized for low-power trusted NAS devices, with one-click deployment and monolithic service architecture.

> **This is NOT "clawdbot in Go", but "NAS Agent Runtime with clawdbot as the requirements sample".**

## Project Positioning

| Feature | clawdbot (Original) | ZimaOS-Blue (Target) |
|---------|---------------------|----------------------|
| Language | TypeScript/Node.js | Golang |
| Architecture | Multi-process/Microservices | Monolithic |
| Deployment | Multiple methods | One-click |
| Resource Usage | High (Node.js) | Minimal (<80MB) |
| Target Devices | General | Low-power NAS/Edge |
| Frontend | Various | Vue 3 |

## Core Principles

1. **Stability First**: Replace runtime first, then add features
2. **Resource Efficient**: Control resources first, then pursue ecosystem
3. **NAS Native**: Can be resident first, then extensible
4. **Performance First**: Leverage Golang concurrency for high throughput and low latency
5. **Extensibility**: Plugin-based design, load features on demand

## Version Semantics

| Version Range | Meaning |
|---------------|---------|
| 0.1 - 0.2 | Technical validation / Core stability |
| 0.3 - 0.4 | Feature usable / NAS integration |
| 0.5 | Product-level baseline (external release) |
| 0.6+ | Advanced features / Ecosystem |

---

## Phase Planning

### v0.1: Go Runtime Core

**Goal**: Stable and long-running

**Must Complete**:
- [x] Go single-process daemon model
- [x] Main event loop (explicit scheduling)
- [x] Controlled goroutine / worker pool
- [x] Global Context / lifecycle management
- [x] Structured logging (Zap/Zerolog)
- [x] Basic configuration (file + env) with Viper
- [x] Health check endpoints
- [x] Basic HTTP service framework

**Explicitly NOT Doing**:
- Plugin system
- Skill Hub
- Complex command system
- Frontend UI

**Acceptance Criteria**:
- [x] Can run on NAS for **24h without restart**
- [x] Idle CPU ≈ 0%
- [x] Resident memory < 50-80MB

**Tech Stack**:
- Web Framework: Gin / Fiber / Echo
- Configuration: Viper
- Logging: Zap / Zerolog
- Goroutine Pool: sourcegraph/conc/pool

---

### v0.2: Core Capabilities & Minimal Functions

**Goal**: Can work, but with converged features

**New Content**:
- [x] Core command execution framework (static registration)
- [x] Basic state management (KV / memory + persistence)
- [x] External API call wrapper (rate limiting / timeout)
- [x] Basic metrics API (health / metrics)
- [x] LLM Provider interface (OpenAI, Claude, Ollama)
- [x] Basic tool/function calling framework
- [x] Streaming response support
- [x] Context management
- [x] Memory/conversation history (SQLite)
- [x] Chat API endpoints

**Technical Focus**:
- No runtime dynamic loading
- No goroutine leaks
- All IO must be interruptible

**Acceptance Criteria**:
- [x] Concurrent commands controllable
- [x] P99 latency stable
- [x] Safe recovery after force kill

---

### v0.3: NAS Integration & System Integration

**Goal**: Like a NAS built-in service

**New Content**:
- [x] systemd / launchd support
- [x] cgroup / rlimit resource limits
- [x] File system capabilities (watch / IO task)
- [x] Basic Web management API (system status)
- [x] Docker build configuration
- [x] CI/CD pipeline
- [x] Basic Web Chat channel

**Architecture Changes**:
- [x] Task scheduler module independent
- [ ] IO / compute separation
- [ ] Optional NUMA / CPU pin (if needed)

**Acceptance Criteria**:
- [x] Coexist with other NAS services
- [x] No obvious resource contention
- [x] Auto-recovery after system restart

---

### v0.4: Plugin & Extension Model

**Goal**: Extensible without sacrificing performance

**Plugin Strategy** (Recommended):
- [x] Compile-time plugins (Go module)
- [x] JavaScript plugins (goja runtime, clawdbot compatible)
- [ ] WASM plugins (limited capabilities) - Optional
- [x] Plugin interface definition
- [x] Plugin configuration management
- [x] Plugin dependency resolution
- [x] Plugin isolation and restrictions

**Plugin Restrictions** (Enforced):
- Plugins CANNOT:
  - Directly access global state
  - Start goroutines
  - Bypass scheduler
- [x] Resource usage limits per plugin

**Skill Hub Strategy**:
- [x] NOT reusing Node Skill Hub
- [x] New Skill Manifest definition (YAML/JSON)
- [x] Skill description and execution separation
- [x] Built-in skills: weather, search, calculator, system info, datetime

**Security & Identity**:
- [x] Basic authentication (JWT)
- [x] Permission control (RBAC)
- [x] API key management
- [x] API key rotation
- [x] User-role assignment

**Frontend**:
- [x] Plugin management UI
- [x] Authentication UI (login, token management)
- [x] Web chat improvements (dark mode, mobile-friendly)
- [x] System status page
- [x] Backup/restore UI

**Acceptance Criteria**:
- [x] Plugins can load/unload
- [x] Plugin exceptions don't affect main process
- [x] Performance regression acceptable
- [x] Basic authentication working
- [x] RBAC permissions enforced

---

### v0.5: Product-Level Baseline (First Stable Release)

**Goal**: Can be used externally, can be delivered

**New Content**:
- [ ] Complete Metrics (Prometheus)
- [ ] Profiling (pprof)
- [ ] Grayscale config / hot reload (config level)
- [ ] Error isolation & degradation strategy
- [ ] Vue 3 frontend project
- [ ] One-click installation script
- [ ] ZimaOS integration
- [ ] Backup/restore functionality

**Stability Assurance**:
- [ ] Chaos testing (kill / IO fail)
- [ ] Long-term soak test
- [ ] Memory / goroutine leak detection

**Documentation**:
- [ ] Architecture documentation
- [ ] Plugin development specification
- [ ] NAS integration guide

**Acceptance Criteria**:
- [ ] 7-day continuous operation without issues
- [ ] Memory stable, no leaks
- [ ] All core features documented

---

## Long-term Goals (v0.6.0+)

### v0.6: Message Channels & More Integrations

**Goal**: Multi-channel message support

- [x] Telegram Bot
- [x] Discord Bot
- [x] Slack
- [x] WeChat Work
- [x] Feishu/Lark
- [x] Matrix
- [x] Channel abstraction interface

---

### v0.7: Security Enhancement

**Goal**: Complete security mechanisms

- [x] Built-in OIDC Provider (OpenID Connect)
- [x] Local user management
- [x] Password authentication (Argon2)
- [x] Multi-factor authentication (TOTP)
- [x] WebAuthn/Passkeys support
- [x] Audit logging
- [x] Sandbox execution environment
- [ ] Unit tests for all security modules
- [ ] Security documentation

**Tech Stack**:
- OIDC: ory/fosite
- Password: Argon2id
- MFA: pquerna/otp
- WebAuthn: go-webauthn/webauthn

---

### v0.8: Performance Optimization

**Goal**: Comprehensive performance tuning and optimization

- [ ] Database optimization
  - [ ] Query optimization and indexing
  - [ ] Connection pooling tuning
  - [ ] SQLite WAL mode optimization
  - [ ] Batch operations for bulk data
- [ ] Memory optimization
  - [ ] Memory pool for frequent allocations
  - [ ] GC tuning (GOGC, GOMEMLIMIT)
  - [ ] Object reuse and sync.Pool
  - [ ] Memory profiling and leak detection
- [ ] Concurrency optimization
  - [ ] Goroutine pool sizing
  - [ ] Lock contention analysis
  - [ ] Channel buffer optimization
  - [ ] Context cancellation optimization
- [ ] Network optimization
  - [ ] HTTP/2 and connection reuse
  - [ ] Response compression (gzip/brotli)
  - [ ] Request batching
  - [ ] Timeout and retry optimization
- [ ] Caching layer
  - [ ] Multi-level cache (L1 memory, L2 disk)
  - [ ] Cache invalidation strategies
  - [ ] LRU/LFU cache policies
  - [ ] Distributed cache support
- [ ] Benchmarking and profiling
  - [ ] Comprehensive benchmark suite
  - [ ] Continuous performance monitoring
  - [ ] Regression detection
  - [ ] Load testing framework

---

### v0.9: Future Enhancements

- [x] Multi-user/multi-tenant
- [ ] Mobile App (Native iOS/Android)
- [x] Browser automation (Playwright/Rod)
- [x] Smart home integration (Home Assistant API)
- [x] Voice assistant mode
- [ ] Workflow automation (n8n-style)
- [ ] Multi-node cluster mode
- [x] External OIDC provider integration

---

### v0.10: Claude Code CLI Integration

**Goal**: Seamless Claude Code CLI integration with enterprise-grade features

**v0.10.0 - CLI Bundling**
- [ ] Claude Code CLI bundling and version management
- [ ] Download on demand / embedded modes
- [ ] System CLI detection and validation
- [ ] Auto-update support

**v0.10.1 - Metrics Monitoring**
- [ ] API call statistics (by model, by provider)
- [ ] Token usage tracking and cost estimation
- [ ] Performance metrics (TTFT, latency, throughput)
- [ ] System resource monitoring

**v0.10.2 - CLI Reliability**
- [ ] Process lifecycle management
- [ ] Error recovery and retry logic
- [ ] Health monitoring

**v0.10.3 - CLI Integration**
- [ ] First-run setup wizard
- [ ] Provider auto-detection (Ollama, etc.)
- [ ] cc-switch integration
- [ ] Limited mode support

**v0.10.4 - Tauri Packaging**
- [ ] Cross-platform desktop app (Windows, macOS)
- [ ] Network access feature
- [ ] System tray integration

**v0.10.5 - API Proxy Sidecar** ★ Major Enhancement
- [ ] Local API proxy for all CC CLI requests
- [ ] Route selection and high availability
- [ ] Session monitoring
- [ ] Prompt injection interception
- [ ] Usage statistics (tokens, latency, TTFT)
- [ ] Dynamic configuration hot-reload
- [ ] Simple authentication
- [ ] Connection pooling and keep-alive
- [ ] Model compatibility layer (tool calling fallback)
- [ ] Mock endpoints for development
- [ ] CLI daemon mode (if feasible)

---

## Explicit "NOT Doing" List (Before v0.5)

> The following are **intentionally NOT included in v0.5**:

- Node plugin compatibility layer
- ~~JS runtime embedding~~ (Completed in v0.4 via goja)
- Complex UI
- Hot code updates
- Full Skill Hub ecosystem

**These will immediately break performance and maintainability if added too early.**

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              ZimaOS-Blue                                          │
├──────────────────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────────┐              │
│  │    Vue 3     │  │   REST API   │  │       WebSocket            │              │
│  │   Frontend   │  │   Endpoints  │  │       Gateway              │              │
│  │   (v0.5+)    │  │              │  │                            │              │
│  └──────┬───────┘  └──────┬───────┘  └────────────┬───────────────┘              │
│         │                 │                       │                              │
│  ┌──────┴─────────────────┴───────────────────────┴───────────────┐              │
│  │                     Core Runtime (v0.1)                        │              │
│  │  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌─────────┐ ┌─────────┐  │              │
│  │  │ Event   │ │ Worker  │ │ Context  │ │ Config  │ │ Logger  │  │              │
│  │  │  Loop   │ │  Pool   │ │ Manager  │ │         │ │         │  │              │
│  │  └─────────┘ └─────────┘ └──────────┘ └─────────┘ └─────────┘  │              │
│  └────────────────────────────────────────────────────────────────┘              │
│                               │                                                  │
│  ┌────────────────────────────┴───────────────────────────────────┐              │
│  │                    Agent Runtime (v0.2)                        │              │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐            │              │
│  │  │   LLM    │ │  Tools   │ │  Memory  │ │ Context │            │              │
│  │  │ Provider │ │ Executor │ │  Store   │ │ Manager │            │              │
│  │  └──────────┘ └──────────┘ └──────────┘ └─────────┘            │              │
│  └────────────────────────────────────────────────────────────────┘              │
│                               │                                                  │
│  ┌────────────────────────────┴───────────────────────────────────┐              │
│  │              ★ API Proxy Sidecar (v0.10.5) ★                   │              │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │              │
│  │  │  Route   │ │  Session │ │  Prompt  │ │  Usage   │           │              │
│  │  │  Select  │ │  Monitor │ │  Guard   │ │  Stats   │           │              │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │              │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │              │
│  │  │  Config  │ │  Conn    │ │  Model   │ │  Auth    │           │              │
│  │  │  Watch   │ │  Pool    │ │  Compat  │ │  Gate    │           │              │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │              │
│  └────────────────────────────────────────────────────────────────┘              │
│                               │                                                  │
│  ┌────────────────────────────┴───────────────────────────────────┐              │
│  │              Claude Code CLI (Daemon Mode) (v0.10.5)           │              │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │              │
│  │  │  Daemon  │ │  Session │ │  Tool    │ │  MCP     │           │              │
│  │  │  Process │ │  Reuse   │ │  Execute │ │  Server  │           │              │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │              │
│  └────────────────────────────────────────────────────────────────┘              │
│                               │                                                  │
│  ┌────────────────────────────┴───────────────────────────────────┐              │
│  │                  Channel Adapters (v0.6)                       │              │
│  │  ┌────────┐ ┌──────────┐ ┌─────────┐ ┌───────┐ ┌────────────┐  │              │
│  │  │  Web   │ │ Telegram │ │ Discord │ │ Slack │ │ WeChat/... │  │              │
│  │  │(v0.3)  │ │          │ │         │ │       │ │            │  │              │
│  │  └────────┘ └──────────┘ └─────────┘ └───────┘ └────────────┘  │              │
│  └────────────────────────────────────────────────────────────────┘              │
│                               │                                                  │
│  ┌────────────────────────────┴───────────────────────────────────┐              │
│  │                   Plugin System (v0.4)                         │              │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │              │
│  │  │ Skills │ │  Cron  │ │ Media  │ │Webhook │ │Custom Plugins│  │              │
│  │  └────────┘ └────────┘ └────────┘ └────────┘ └──────────────┘  │              │
│  └────────────────────────────────────────────────────────────────┘              │
│                               │                                                  │
│  ┌────────────────────────────┴───────────────────────────────────┐              │
│  │                      Data Layer                                │              │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────┐  │              │
│  │  │  SQLite    │ │  BoltDB    │ │ SQLite-vec │ │    Files    │  │              │
│  │  │ (Primary)  │ │ (KV Cache) │ │ (Future)   │ │   (Media)   │  │              │
│  │  └────────────┘ └────────────┘ └────────────┘ └─────────────┘  │              │
│  └────────────────────────────────────────────────────────────────┘              │
└──────────────────────────────────────────────────────────────────────────────────┘
```

---

## Milestone Timeline Summary

| Version | Focus | Key Value |
|---------|-------|-----------|
| v0.1 | Go Runtime Core | Stable kernel, 24h running |
| v0.2 | Core Capabilities | Minimal usable, LLM integration |
| v0.3 | NAS Integration | NAS native, systemd support |
| v0.4 | Plugin System | Extensible, security basics |
| v0.5 | Product Baseline | Production ready, documentation |
| v0.6 | Message Channels | Multi-channel support |
| v0.7 | Security | OIDC, MFA, audit |
| v0.8 | Performance | Optimization, caching, benchmarks |
| v0.9 | Ecosystem | Multi-tenant, mobile, automation |
| v0.10 | CC CLI Integration | API Proxy, metrics, desktop app |

---

## References

- Original Project: [clawdbot/clawdbot](https://github.com/clawdbot/clawdbot)
- Tech Stack References:
  - [Gin Web Framework](https://github.com/gin-gonic/gin)
  - [Gorilla WebSocket](https://github.com/gorilla/websocket)
  - [Vue 3](https://vuejs.org/)
  - [Vite](https://vitejs.dev/)
  - [SQLite-vec](https://github.com/asg017/sqlite-vec)
  - [sourcegraph/conc](https://github.com/sourcegraph/conc)

