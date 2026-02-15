# ZimaOS-Blue Roadmap

This document describes the development direction and milestone planning for the ZimaOS-Blue project.

## Vision

Based on the design philosophy of [clawdbot](https://github.com/clawdbot/clawdbot), build a **A Local-first Agent Runtime for Builders with Bolder Mind** using Golang - lightweight, high-performance, optimized for low-power devices, with one-click deployment and monolithic service architecture.

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
- [x] Complete Metrics (Prometheus)
- [x] Profiling (pprof)
- [x] Grayscale config / hot reload (config level)
- [x] Error isolation & degradation strategy
- [x] Vue 3 frontend project
- [x] One-click installation script
- [x] ZimaOS integration
- [x] Backup/restore functionality

**Stability Assurance**:
- [x] Chaos testing (kill / IO fail)
- [x] Long-term soak test
- [x] Memory / goroutine leak detection

**Documentation**:
- [x] Architecture documentation
- [ ] Plugin development specification
- [ ] NAS integration guide

**Acceptance Criteria**:
- [x] 7-day continuous operation without issues
- [x] Memory stable, no leaks
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

- [x] Database optimization
  - [x] Query optimization and indexing
  - [x] Connection pooling tuning
  - [x] SQLite WAL mode optimization
  - [x] Batch operations for bulk data
- [x] Memory optimization
  - [x] Memory pool for frequent allocations
  - [x] GC tuning (GOGC, GOMEMLIMIT)
  - [x] Object reuse and sync.Pool
  - [x] Memory profiling and leak detection
- [x] Concurrency optimization
  - [x] Goroutine pool sizing
  - [x] Lock contention analysis
  - [x] Channel buffer optimization
  - [x] Context cancellation optimization
- [x] Network optimization
  - [x] HTTP/2 and connection reuse
  - [x] Response compression (gzip/brotli)
  - [x] Request batching
  - [x] Timeout and retry optimization
- [x] Caching layer
  - [x] Multi-level cache (L1 memory, L2 disk)
  - [x] Cache invalidation strategies
  - [x] LRU/LFU cache policies
  - [ ] Distributed cache support
- [x] Benchmarking and profiling
  - [x] Comprehensive benchmark suite
  - [x] Continuous performance monitoring
  - [x] Regression detection
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
- [x] Claude Code CLI bundling and version management
- [x] Download on demand / embedded modes
- [x] System CLI detection and validation
- [x] Auto-update support

**v0.10.1 - Metrics Monitoring**
- [x] API call statistics (by model, by provider)
- [x] Token usage tracking and cost estimation
- [x] Performance metrics (TTFT, latency, throughput)
- [x] System resource monitoring

**v0.10.2 - CLI Reliability**
- [x] Process lifecycle management
- [x] Error recovery and retry logic
- [x] Health monitoring

**v0.10.3 - CLI Integration**
- [x] First-run setup wizard
- [x] Provider auto-detection (Ollama, etc.)
- [x] cc-switch integration
- [x] Limited mode support

**v0.10.4 - Tauri Packaging**
- [x] Cross-platform desktop app (Windows, macOS)
- [x] Network access feature
- [x] System tray integration

**v0.10.5 - API Proxy Sidecar** ★ Major Enhancement
- [x] Local API proxy for all CC CLI requests
- [x] Route selection and high availability
- [x] Session monitoring
- [x] Prompt injection interception
- [x] Usage statistics (tokens, latency, TTFT)
- [x] Dynamic configuration hot-reload
- [x] Simple authentication
- [x] Connection pooling and keep-alive
- [x] Model compatibility layer (tool calling fallback)
- [x] Mock endpoints for development
- [ ] CLI daemon mode (if feasible)

**v0.10.6 - Provider Pool**
- [x] Multi-provider pool with priority routing
- [x] Provider health check and failover
- [x] Streaming refactor with provider ID mapping

**v0.10.7 - Preview Mode**
- [x] Preview mode for unauthenticated access
- [x] Feature gating per mode

**v0.10.8 - Skill Store & Channel Validation**
- [x] Skill store core infrastructure
- [x] Channel connection validation
- [x] Wizard cleanup

**v0.10.9–v0.10.10 - Remote Access & User Management**
- [x] Sub-user creation with page-level permissions
- [x] Default chat-only access for sub-users

**v0.10.13–v0.10.14 - Security & Skill Store Redesign**
- [x] Security page optimization with monitoring integration
- [x] Skill store redesign (new DB schema, search, categories)

**v0.10.15 - Chat Enhancements**
- [x] Chat UX improvements
- [x] Message processing pipeline enhancements

**v0.10.16 - Speech Module**
- [x] Sherpa TTS/ASR integration
- [x] eSpeak-NG language pack management
- [x] TTS provider switching

**v0.10.17 - Remote Access Enhancement**
- [x] Ngrok tunnel integration
- [x] Cloudflare tunnel integration
- [x] ACME / Let's Encrypt certificate support

**v0.10.18–v0.10.20 - Performance Sprint**
- [x] Startup performance optimization
- [x] Chat pipeline performance optimization
- [x] Conversation context cache

**v0.10.21–v0.10.22 - System Prompt & DingTalk**
- [x] System prompt enhancement with context compaction
- [x] DingTalk channel implementation
- [x] Chat pipeline optimization (iteration 1)

**v0.10.23 - OTA Update**
- [x] OTA update system planning and implementation

**v0.10.24 - Channel Upgrade**
- [x] 10 channel native implementations upgraded from stubs

**v0.10.25 - CC Cache** ★ Major Enhancement
- [x] Two-level caching (L1 memory + L2 disk) for API Proxy
- [x] Cache key normalization (FNV-1a)
- [x] Streaming-aware cache bypass

**v0.10.26 - Humanizer**
- [x] Response humanization pipeline

**v0.10.27 - Context Pruner** (In Progress)
- [x] Unified pruner with BM25 scoring
- [x] Chinese/English segmentation
- [x] Benchmark suite (90%+ test coverage)
- [ ] ONNX neural backend integration

**v0.10.28 - Memory Service**
- [x] Progressive search (3-layer: Index/Context/Detail)
- [x] Dual-write backend (SQLite + Markdown)
- [x] Remove Supermemory dependency

---

## Updates

> Development log — key milestones and feature deliveries, sorted chronologically.

| Date | Version | What Shipped |
|------|---------|-------------|
| Jan 26, 2026 | v0.1–v0.9 | Project bootstrapped — Go runtime, LLM providers, plugin system, multi-channel messaging, OIDC/MFA security, perf optimization, browser automation, voice assistant |
| Jan 27, 2026 | v0.9.0 | Browser automation view with task management; roadmap documentation |
| Jan 28, 2026 | v0.9.1–v0.9.2 | Blue Companion real-time monitoring; Smart Form Filler floating widget |
| Jan 29, 2026 | v0.10.0 | Claude Code CLI integration — replace direct API calls with CC CLI |
| Jan 30, 2026 | v0.10.5–v0.10.6 | API Proxy Sidecar core; Provider Pool with streaming refactor |
| Jan 31, 2026 | v0.10.7–v0.10.9 | Preview mode; Typeless card rendering optimization; UI restructure (dashboard + settings consolidation) |
| Feb 1, 2026 | v0.10.1–v0.10.2 | Metrics monitoring (API stats, token tracking); CLI reliability (process lifecycle, error recovery) |
| Feb 2, 2026 | v0.10.15–v0.10.17 | Chat enhancements; Speech module (Sherpa TTS/ASR); Remote access (Ngrok/Cloudflare tunnels, ACME certs) |
| Feb 3, 2026 | v0.10.18 | Startup perf optimization; Chat perf optimization; eSpeak language packs; TTS provider switching; Memory system enhancement; Feishu channel |
| Feb 3, 2026 | v0.10.21–v0.10.22 | System prompt enhancement; Chat pipeline optimization with conversation context cache |
| Feb 5, 2026 | — | Internationalization (i18n) support; message processing enhancements |
| Feb 6, 2026 | v0.10.22 | DingTalk channel; Whisper/GGML build enhancement; OTA update system planning |
| Feb 7, 2026 | — | Route registration refactor; macOS platform startup; TLS cert handling; third-party submodules (espeak-ng, opus, whisper.cpp) |
| Feb 8, 2026 | — | Personality management module (MVC + file storage + UI) |
| Feb 11–12, 2026 | — | Personality refactor; channel enhancement; CLI management commands (config, cron, health, logs, models, plugins) |
| Feb 13, 2026 | — | Project rebrand: Echo → Blue; ACME cert management; service management i18n |
| Feb 14, 2026 | v0.10.25 | CC Cache two-level caching (L1 memory + L2 disk) for API Proxy |
| Feb 15, 2026 | v0.10.27–v0.10.28 | Memory service refactor (progressive search + dual-write backend); Encryption settings; Architecture docs; Context pruner (in progress) |

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

## Milestone Timeline Summary

| Version | Focus | Key Value | Status |
|---------|-------|-----------|--------|
| v0.1 | Go Runtime Core | Stable kernel, 24h running | Done |
| v0.2 | Core Capabilities | Minimal usable, LLM integration | Done |
| v0.3 | NAS Integration | NAS native, systemd support | Done |
| v0.4 | Plugin System | Extensible, security basics | Done |
| v0.5 | Product Baseline | Production ready, documentation | Done |
| v0.6 | Message Channels | Multi-channel support | Done |
| v0.7 | Security | OIDC, MFA, audit | Done |
| v0.8 | Performance | Optimization, caching, benchmarks | Done |
| v0.9 | Ecosystem | Multi-tenant, browser automation, voice | Done |
| v0.10.0 | CLI Bundling | CC CLI bundling, detection, auto-update | Done |
| v0.10.1 | Metrics Monitoring | API stats, token tracking, TTFT | Done |
| v0.10.2 | CLI Reliability | Process lifecycle, error recovery | Done |
| v0.10.3 | CLI Integration | Setup wizard, provider auto-detect | Done |
| v0.10.4 | Tauri Packaging | Desktop app, system tray | Done |
| v0.10.5 | API Proxy Sidecar | Route selection, prompt guard, usage stats | Done |
| v0.10.6 | Provider Pool | Multi-provider routing, health check, failover | Done |
| v0.10.7 | Preview Mode | Unauthenticated access, feature gating | Done |
| v0.10.8 | Skill Store | Skill store infra, channel validation | Done |
| v0.10.9–10 | User Management | Sub-users, page-level permissions | Done |
| v0.10.13–14 | Security & Skills | Security page, skill store redesign | Done |
| v0.10.15 | Chat Enhancements | Chat UX, message pipeline | Done |
| v0.10.16 | Speech Module | Sherpa TTS/ASR, eSpeak, provider switching | Done |
| v0.10.17 | Remote Access | Ngrok, Cloudflare tunnels, ACME certs | Done |
| v0.10.18–20 | Performance Sprint | Startup/chat perf, context cache | Done |
| v0.10.21–22 | Prompt & DingTalk | System prompt, DingTalk channel | Done |
| v0.10.23 | OTA Update | OTA update system | Done |
| v0.10.24 | Channel Upgrade | 10 channels upgraded from stubs | Done |
| v0.10.25 | CC Cache | Two-level cache (L1 mem + L2 disk) | Done |
| v0.10.26 | Humanizer | Response humanization pipeline | Done |
| v0.10.27 | Context Pruner | BM25 scoring, segmentation, benchmarks | Active |
| v0.10.28 | Memory Service | Progressive search, dual-write backend | Done |

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

