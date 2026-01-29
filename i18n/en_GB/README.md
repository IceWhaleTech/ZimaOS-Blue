# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <a href="../README.md"><strong>English</strong></a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <strong>English (UK)</strong> |
  <a href="../es_ES/README.md">Español (ES)</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../fi_FI/README.md">Suomi</a> |
  <a href="../no_NO/README.md">Norsk</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a>
</p>

> This is the UK English version of the README for ZimaOS Echo.

For full documentation, please visit: https://echo.zimaos.com

The rest of this document follows the main English README. Please refer to `../README.md` for the latest and most complete information.

# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>NAS-Native Agent Runtime</strong>
</p>

<p align="center">
  <a href="../../README.md">English (US)</a> | <a href="../zh_CN/README.md">中文</a> | <strong>English (UK)</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/Licence-MIT-blue.svg?style=for-the-badge" alt="MIT Licence"></a>
</p>

**ZimaOS Echo** is a lightweight, high-performance AI agent runtime designed specifically for NAS and edge devices. Built with Go, it provides a production-ready platform for running AI assistants on low-power hardware.

[Documentation](https://echo.zimaos.com) · [Quick Start](#quick-start) · [Features](#features) · [Comparison](#comparison-with-clawdbot)

## Why ZimaOS Echo?

ZimaOS Echo is inspired by [clawdbot](https://github.com/clawdbot/clawdbot) but rebuilt from the ground up in Go for:

- **Lower Resource Usage**: Runs on devices with as little as 256MB RAM
- **Better Performance**: Native Go binary with efficient goroutine-based concurrency
- **Easier Deployment**: Single binary, no Node.js runtime required
- **NAS Optimisation**: Designed for 24/7 operation on low-power devices

## Quick Start

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell as Admin)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### From Source

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Features

### Core Features

- 🚀 **Lightweight**: Single binary < 15MB, memory < 80MB
- ⚡ **High Performance**: Go-based with goroutine concurrency
- 🔌 **Multi-Provider**: OpenAI, Anthropic, Ollama, and more
- 🛡️ **Production Ready**: Circuit breaker, graceful degradation, auto-recovery
- 📊 **Observable**: Prometheus metrics, pprof profiling, structured logging
- 🔄 **Hot Reload**: Configuration changes without restart
- 💾 **Backup/Restore**: Automated backup with point-in-time restore

### Smart Home Integration

- 🏠 **Home Assistant**: Native integration with Home Assistant API
- 💡 **Device Control**: Lights, switches, sensors, climate, and more
- 🤖 **AI Automation**: Natural language commands for smart home control
- 📡 **Real-time Events**: Subscribe to device state changes via WebSocket

### Voice Capabilities

- 🎤 **Speech Recognition**: Whisper-based speech-to-text
- 🔊 **Text-to-Speech**: Multiple TTS engines support
- 👂 **Voice Wake**: Customisable wake word detection
- 🗣️ **Voice Commands**: Hands-free AI assistant interaction

### Multi-Tenant Architecture

- 👥 **Tenant Isolation**: Complete data and resource isolation
- 🔐 **Per-Tenant Auth**: Independent authentication per tenant
- 📊 **Resource Quotas**: CPU, memory, and API rate limits per tenant
- 🎛️ **Tenant Dashboard**: Self-service management portal

### Communication Channels

- 💬 **Matrix Protocol**: Decentralised, end-to-end encrypted messaging
- 📱 **Telegram/Discord/Slack**: Popular messaging platform support
- 📞 **Signal/WhatsApp**: Secure messaging integration
- 🍎 **iMessage**: Native macOS iMessage support

### Security & Authentication

- 🔑 **WebAuthn/Passkeys**: Passwordless authentication with FIDO2
- 🔐 **OIDC/OAuth 2.0**: Enterprise SSO (Google, GitHub, Okta, etc.)
- 📲 **MFA/TOTP**: Multi-factor authentication support
- 🛡️ **RBAC**: Fine-grained role-based access control
- 📝 **Audit Logging**: Comprehensive security audit trail
- 🔒 **Sandbox**: Isolated execution environment for tools

### Frontend

- 🎨 **Vue 3 Dashboard**: Modern, responsive web interface
- 💬 **Chat Interface**: Streaming responses with markdown support
- 📈 **System Monitor**: Real-time resource usage charts
- ⚙️ **Settings UI**: Easy configuration management

## Comparison with Clawdbot

ZimaOS Echo is inspired by clawdbot but optimised for NAS/edge deployment:

| Feature | ZimaOS Echo | Clawdbot |
|---------|-------------|----------|
| **Language** | Go | TypeScript/Node.js |
| **Binary Size** | ~15MB | ~200MB+ (with node_modules) |
| **Memory Usage** | ~80MB idle | ~200MB+ idle |
| **Startup Time** | < 1s | 3-5s |
| **Runtime** | Native binary | Node.js required |
| **Target Platform** | NAS/Edge devices | Desktop/Server |

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Vue 3 Frontend  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Core Runtime (Go)                   │
│  Event Loop │ Worker Pool │ Config │ Logger     │
├─────────────────────────────────────────────────┤
│              Agent Runtime                       │
│  LLM Provider │ Tools │ Memory │ Context        │
├─────────────────────────────────────────────────┤
│              Data Layer                          │
│  SQLite (Zorm) │ ECache │ Files                 │
└─────────────────────────────────────────────────┘
```

## Roadmap

- [x] **v0.1.0** - Core Runtime (Event loop, Worker pool, Config, Logger)
- [x] **v0.2.0** - Agent Runtime (LLM providers incl. AWS Bedrock, Tools, Memory)
- [x] **v0.3.0** - API Layer (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Plugin System (Go modules, WASM support)
- [x] **v0.5.0** - Production Ready (Metrics, Profiling, Backup)
- [x] **v0.6.0** - Message Channels (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Security (OIDC, MFA, WebAuthn, Audit logging, Sandbox)
- [x] **v0.8.0** - Performance (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Future Enhancements (A2UI, Browser Automation)
- [ ] **v1.0.0** - RAG & Knowledge Base

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

```bash
# Clone the repository
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Install dependencies
cd server && go mod download

# Run tests
go test ./...

# Build
go build -o zimaos-echo ./cmd/server
```

## Licence

MIT Licence - see [LICENSE](LICENSE) for details.

## Acknowledgements

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiration for the project
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - Lightweight ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
