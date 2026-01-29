# ZimaOS Echo

<p align="center">
  <img src="docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Secure, Observable, Local-First AI Agent Runtime</strong>
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** is a lightweight, high-performance AI agent runtime designed specifically for NAS and edge devices. Built with Go, it provides a production-ready platform for running AI assistants on low-power hardware.

[Documentation](https://echo.zimaos.com) · [Quick Start](#quick-start) · [Features](#core-principles) · [Comparison](#comparison-with-clawdbot)

## Why ZimaOS Echo?

ZimaOS Echo is inspired by [clawdbot](https://github.com/clawdbot/clawdbot) but rebuilt from the ground up in Go for:

- **Lower Resource Usage**: Runs on devices with as little as 256MB RAM
- **Better Performance**: Native Go binary with efficient goroutine-based concurrency
- **Easier Deployment**: Single binary, no Node.js runtime required
- **NAS Optimization**: Designed for 24/7 operation on low-power devices

## Core Principles

### Local-First

- **Data Sovereignty**: All data stored locally on your NAS - no cloud dependency
- **Ollama Integration**: Run LLMs entirely on-device with zero external API calls
- **Offline Capable**: Core functionality works without internet connectivity
- **Single Binary**: ~15MB native Go binary, no runtime dependencies

### Observable & Auditable

- **Audit Logging**: Every AI action logged with full context and timestamps
- **Prometheus Metrics**: Real-time monitoring of all system operations
- **pprof Profiling**: Deep visibility into CPU, memory, and goroutine behavior
- **Structured Logging**: JSON logs for easy parsing and alerting

### Security Hardening

- **Sandbox Execution**: All tool calls run in isolated environments
- **RBAC**: Fine-grained role-based access control
- **WebAuthn/Passkeys**: Passwordless FIDO2 authentication
- **MFA/TOTP**: Multi-factor authentication support
- **OIDC/OAuth 2.0**: Enterprise SSO integration
- **Circuit Breaker**: Automatic failure isolation prevents cascade failures

## Quick Start

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# From Source
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Security Hardening

### Authentication Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| Primary | WebAuthn/Passkeys | Phishing-resistant passwordless auth |
| Secondary | TOTP/MFA | Time-based one-time passwords |
| Enterprise | OIDC/OAuth 2.0 | SSO with Google, GitHub, Okta |
| Authorization | RBAC | Per-resource permission control |

### Runtime Protection

- **Sandbox Isolation**: Tool execution in restricted environments
- **Rate Limiting**: Per-tenant API throttling
- **Tenant Isolation**: Complete data and resource separation
- **Audit Trail**: Immutable logs of all privileged operations

### Resilience

- **Circuit Breaker**: Automatic service isolation on failure
- **Graceful Degradation**: Fallback strategies when providers fail
- **LLM Fallback Chain**: Automatic provider failover
- **Hot Reload**: Configuration changes without restart

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

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
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

## Local LLM Setup (Ollama)

Run AI completely offline with no external API calls:

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model
ollama pull llama3.2

# Configure Echo to use local LLM
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## Development Setup

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Pre-installed on macOS/Linux |

### One-Command Start

```bash
# Clone and start everything
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

Access the dashboard at `http://localhost:3000`

### Development Mode (Hot Reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:5173` (proxies API to backend)
- Backend: `http://localhost:8080`

### Build Commands

```bash
make build              # Build single binary (frontend embedded)
make build-embedded     # Build with Claude Code CLI embedded
make build-all          # Cross-compile for all platforms
make clean              # Clean build artifacts
```

### Project Structure

```
ZimaOS-Echo/
├── server/             # Go backend
│   ├── cmd/echo/       # Entry point
│   └── internal/       # Core modules
├── web/                # Vue 3 frontend
│   └── src/
└── dist/               # Build output
```

## Comparison with Clawdbot

ZimaOS Echo is inspired by clawdbot but optimized for NAS/edge deployment:

| Feature | ZimaOS Echo | Clawdbot |
|---------|-------------|----------|
| **Language** | Go | TypeScript/Node.js |
| **Binary Size** | ~15MB | ~200MB+ (with node_modules) |
| **Memory Usage** | ~80MB idle | ~200MB+ idle |
| **Startup Time** | < 1s | 3-5s |
| **Runtime** | Native binary | Node.js required |
| **Target Platform** | NAS/Edge devices | Desktop/Server |

## Acknowledgments

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration for the project
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Lightweight ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
