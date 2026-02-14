# ZimaOS Blue

<p align="center">
  <img src="docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Secure, Observable AI Agent Runtime</strong>
</p>

<p align="center">
  <strong>English</strong> |
  <a href="./i18n/zh_CN/README.md">简体中文</a> |
  <a href="./i18n/zh_TW/README.md">繁體中文</a> |
  <a href="./i18n/ja_JP/README.md">日本語</a> |
  <a href="./i18n/ko_KR/README.md">한국어</a> |
  <a href="./i18n/de_DE/README.md">Deutsch</a> |
  <a href="./i18n/fr_FR/README.md">Français</a> |
  <a href="./i18n/es_ES/README.md">Español</a> |
  <a href="./i18n/it_IT/README.md">Italiano</a> |
  <a href="./i18n/pt_BR/README.md">Português</a> |
  <a href="./i18n/ru_RU/README.md">Русский</a> |
  <a href="./i18n/ar_SA/README.md">العربية</a> |
  <a href="./i18n/hi_IN/README.md">हिन्दी</a> |
  <a href="./i18n/th_TH/README.md">ไทย</a> |
  <a href="./i18n/vi_VN/README.md">Tiếng Việt</a> |
  <a href="./i18n/id_ID/README.md">Bahasa Indonesia</a> |
  <a href="./i18n/tr_TR/README.md">Türkçe</a> |
  <a href="./i18n/pl_PL/README.md">Polski</a> |
  <a href="./i18n/nl_NL/README.md">Nederlands</a> |
  <a href="./i18n/sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** is a lightweight, high-performance AI agent runtime designed for NAS and edge devices. Built with Go, it provides a production-ready platform with zero-config deployment, session monitoring, and comprehensive usage analytics.

[Quick Start](#quick-start) · [Features](#core-features)

## Highlights

| Spec | Value |
|------|-------|
| **Binary Size** | ~40MB (single executable) |
| **Memory (Idle)** | ~4MB |
| **Startup Time** | < 1s |
| **Dependencies** | None (zero-config deployment) |

## Core Features

### Zero-Config Deployment

- **Single Binary**: Download and run - no runtime dependencies required
- **Configuration on Demand**: Works out of the box, customize when needed
- **Cross-Platform**: Windows, macOS, Linux - same binary, same experience
- **Daemon Support**: Run as a persistent background service

### Session Monitoring

- **Real-time Session Tracking**: Monitor all active AI sessions with live status
- **Conversation History**: Full audit trail of all interactions
- **Session Replay**: Review and analyze past conversations
- **Multi-tenant Isolation**: Complete session separation between users

### Call Chain Optimization

- **Request Tracing**: End-to-end visibility of every API call
- **Latency Analysis**: Identify bottlenecks in the request pipeline
- **Provider Routing**: Intelligent routing to optimal LLM providers
- **Circuit Breaker**: Automatic failover on provider failures

### Usage Analytics

- **Token Consumption**: Track usage per user, session, and provider
- **Cost Attribution**: Detailed cost breakdown by operation
- **Rate Limiting**: Per-tenant quota management
- **Export Reports**: Generate usage reports in multiple formats

### Security Hardening

- **Sandbox Execution**: All tool calls run in isolated environments
- **RBAC**: Fine-grained role-based access control
- **WebAuthn/Passkeys**: Passwordless FIDO2 authentication
- **MFA/TOTP**: Multi-factor authentication support
- **Audit Trail**: Immutable logs of all privileged operations

## Quick Start

```bash
# From Source
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

Access the dashboard at `http://localhost:3000`

## LLM Provider Configuration

ZimaOS Blue supports multiple LLM providers including local LLM services:

```yaml
llm:
  # Cloud providers
  provider: "openai"  # or "anthropic", "azure", etc.
  api_key: "your-api-key"

  # Local LLM (optional)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
├─────────────────────────────────────────────────────┤
│  Session Monitor │ Usage Analytics │ Call Tracing  │
├─────────────────────────────────────────────────────┤
│  Audit Log  │  Metrics  │  RBAC  │  Rate Limiter   │
├─────────────────────────────────────────────────────┤
│              Sandbox Execution Layer                 │
│         Tool Isolation │ Resource Limits            │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Tools │ Memory │ Circuit Breaker   │
├─────────────────────────────────────────────────────┤
│              Local Data Layer                        │
│  SQLite │ ECache │ Encrypted Storage                │
└─────────────────────────────────────────────────────┘
```

## Observability

```yaml
# Enable full observability stack
metrics:
  enabled: true
  endpoint: "/metrics"

profiling:
  enabled: true
  endpoint_prefix: "/debug/pprof"

audit:
  enabled: true
  retention_days: 90
```

### Metrics Exposed

- Request latency (p50, p95, p99)
- LLM token usage per provider
- Tool execution success/failure rates
- Memory and goroutine counts
- Circuit breaker state transitions

## Development Setup

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Pre-installed on macOS/Linux |

### Development Mode (Hot Reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:23456`

### Build Commands

```bash
make build              # Build single binary (frontend embedded)
make build-embedded     # Build with Claude Code CLI embedded
make build-all          # Cross-compile for all platforms
make clean              # Clean build artifacts
```

### Project Structure

```
ZimaOS-Blue/
├── server/             # Go backend
│   ├── cmd/blue/       # Entry point
│   └── internal/       # Core modules
├── web/                # Vue 3 frontend
│   └── src/
└── dist/               # Build output
```

## Acknowledgments

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration for the project
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Lightweight ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
