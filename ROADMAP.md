# ZimaOS-Echo Roadmap

[中文版本](./docs/zh/ROADMAP.md)

## Overview

ZimaOS-Echo is a lightweight, high-performance AI agent runtime designed for NAS and edge devices. This roadmap outlines the development phases and feature milestones.

## Current Status: v0.9.0 (Future Enhancements)

### Completed Features

#### Core Infrastructure
- [x] Single binary deployment (~15MB)
- [x] Low memory footprint (~80MB idle)
- [x] SQLite database with Zorm ORM
- [x] ECache high-performance caching
- [x] Prometheus metrics integration
- [x] Structured logging with Zap

#### Authentication & Security
- [x] JWT and API key authentication
- [x] Multi-factor authentication (MFA)
- [x] Role-based access control (RBAC)
- [x] Audit logging and retention
- [x] External OIDC/OAuth2 integration (Google, GitHub, Microsoft, Keycloak, Auth0, etc.)

#### Communication Channels
- [x] Discord integration
- [x] Telegram integration
- [x] Slack integration
- [x] WhatsApp integration
- [x] Signal integration
- [x] iMessage integration
- [x] Feishu integration
- [x] Matrix integration
- [x] WeChat integration

#### AI/LLM Integration
- [x] Multi-provider LLM support
- [x] Skill system with built-in skills
- [x] Tool execution framework

#### v0.9.0 Features (Backend Complete)
- [x] **A2UI System** - Agent-to-UI dynamic component generation
- [x] **Multi-tenant System** - Full tenant management with isolation
- [x] **Browser Automation** - Rod-based headless browser control
- [x] **Smart Home Integration** - Home Assistant client with natural language control
- [x] **Voice Assistant** - STT/TTS with session management

### In Progress

#### Frontend Development
- [ ] A2UI canvas renderer component
- [ ] Tenant management UI
- [ ] Browser automation dashboard
- [ ] Voice chat interface
- [ ] Home Assistant control panel

#### Testing & Documentation
- [ ] Integration tests for tenant isolation
- [ ] Home Assistant client tests
- [ ] Voice pipeline tests
- [ ] API documentation updates

### Planned Features

#### v0.9.x - Remaining Items
- [ ] **Workflow Automation** - Visual workflow builder (n8n-style)
- [ ] **Mobile App Preparation** - Push notification service, device registration
- [ ] **Multi-node Cluster** - Distributed deployment support

---

## Version History

### v0.9.0 - Future Enhancements (Current)
Focus: Expanding ecosystem with advanced integrations

**Backend Completed:**
| Feature | Status | Files |
|---------|--------|-------|
| A2UI System | ✅ | `server/internal/a2ui/` |
| Multi-tenant | ✅ | `server/internal/tenant/` |
| Browser Automation | ✅ | `server/internal/browser/` |
| External OIDC | ✅ | `server/internal/extauth/`, `server/internal/oidc/` |
| Smart Home | ✅ | `server/internal/homeassistant/` |
| Voice Assistant | ✅ | `server/internal/voice/`, `server/internal/stt/`, `server/internal/tts/` |

**Pending:**
- Workflow Automation Engine
- Push Notification Service
- Cluster Mode

### v0.8.0 - Production Ready
- Performance optimization
- Security hardening
- Monitoring and alerting

### v0.7.0 - Authentication
- JWT authentication
- MFA support
- RBAC implementation
- Audit logging

### v0.6.0 - Plugin System
- Plugin architecture
- Skill system
- Tool framework

### v0.5.0 - Multi-channel
- Channel abstraction
- Multiple platform integrations

### v0.4.0 - LLM Integration
- Provider abstraction
- Streaming support
- Context management

### v0.3.0 - Core Services
- Database layer
- Caching system
- Configuration management

### v0.2.0 - HTTP Server
- Echo framework integration
- API routing
- Middleware stack

### v0.1.0 - Initial Release
- Project structure
- Basic functionality

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      ZimaOS-Echo                            │
├─────────────────────────────────────────────────────────────┤
│  Frontend (Vue 3)                                           │
│  ├── Chat Interface                                         │
│  ├── A2UI Canvas Renderer                                   │
│  ├── Tenant Management                                      │
│  ├── Browser Automation Dashboard                           │
│  └── Voice Chat UI                                          │
├─────────────────────────────────────────────────────────────┤
│  API Layer (Echo v4)                                        │
│  ├── REST Endpoints                                         │
│  ├── WebSocket Support                                      │
│  └── Authentication Middleware                              │
├─────────────────────────────────────────────────────────────┤
│  Core Services                                              │
│  ├── LLM Provider (Multi-provider)                          │
│  ├── Channel Manager (Discord, Telegram, etc.)              │
│  ├── Skill System                                           │
│  ├── A2UI Manager                                           │
│  ├── Tenant Service                                         │
│  ├── Browser Service (Rod)                                  │
│  ├── Home Assistant Client                                  │
│  └── Voice Pipeline (STT/TTS)                               │
├─────────────────────────────────────────────────────────────┤
│  Infrastructure                                             │
│  ├── SQLite + Zorm ORM                                      │
│  ├── ECache (LRU Cache)                                     │
│  ├── Worker Pool                                            │
│  ├── Scheduler                                              │
│  └── Metrics (Prometheus)                                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Feature Details

### A2UI (Agent-to-UI) System
Enables AI agents to generate dynamic, interactive UI components.

**Supported Components:**
- Text, Button, Input, Select, Checkbox, Radio, Slider
- Image, Card, List, Table, Chart
- Form, Progress, Alert, Code, Markdown
- Container, Grid, Tabs, Accordion

**Key Features:**
- Fluent builder API
- Action handlers with form data
- Canvas expiration and cleanup
- JSON serialization

### Multi-tenant System
Full multi-tenant support with data isolation.

**Features:**
- Tenant CRUD operations
- Member management (owner, admin, member roles)
- Invitation system with token-based acceptance
- Resource quotas and limits
- Tenant-specific settings
- Ownership transfer

### Browser Automation
Headless browser control using Rod.

**Capabilities:**
- Screenshot capture (PNG, JPEG, WebP)
- PDF generation
- Web scraping with CSS selectors
- Multi-step automation tasks
- Element interaction (click, type, select, scroll, hover)
- JavaScript execution
- Security controls (URL allowlist/blocklist)

### Smart Home Integration
Home Assistant integration for smart home control.

**Features:**
- Entity discovery and caching
- Real-time state updates via WebSocket
- Service call execution
- Scene and automation control
- Natural language command processing

### Voice Assistant
Voice-based interaction with STT/TTS support.

**Providers:**
- STT: OpenAI Whisper API, Local Whisper
- TTS: OpenAI TTS, Edge TTS

**Features:**
- Voice session management
- Audio transcription
- Speech synthesis
- WebSocket streaming support

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on contributing to ZimaOS-Echo.

## License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.
