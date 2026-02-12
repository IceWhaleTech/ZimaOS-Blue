# Version Plan

This document describes the version planning and release strategy for the ZimaOS-Blue project.

## Versioning Standard

The project follows [Semantic Versioning 2.0.0](https://semver.org/):

```
MAJOR.MINOR.PATCH
```

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible new features
- **PATCH**: Backward-compatible bug fixes

## Release Cycle

| Version Type | Frequency | Description |
|--------------|-----------|-------------|
| Major | As needed | Major architecture changes or breaking updates |
| Minor | As needed | New feature releases |
| Patch | As needed | Bug fixes and minor improvements |

## Version Roadmap

### Current Version

**v0.0.0** - Project initialization phase

### Planned Versions (v0.1.0 - v0.5.0)

| Version | Target | Status | Key Features |
|---------|--------|--------|--------------|
| v0.1.0 | Go Runtime Core | Planned | Daemon, event loop, worker pool, logging, config, health check |
| v0.2.0 | Core Capabilities | Planned | LLM providers, tools, memory, streaming, context management |
| v0.3.0 | NAS Integration | Planned | systemd, cgroup, file watch, Docker, CI/CD, Web Chat |
| v0.4.0 | Plugin System | Planned | Go/WASM plugins, Skill Hub, JWT, RBAC, API keys |
| **v0.5.0** | **First Stable Release** | Planned | **Metrics, profiling, Vue 3 frontend, documentation** |

### Post-Stable Versions (v0.6.0+)

| Version | Target | Key Features |
|---------|--------|--------------|
| v0.6.0 | Message Channels | Telegram, Discord, Slack, WeChat, Feishu, Matrix |
| v0.7.0 | Security Enhancement | OIDC Provider, local users, MFA, audit logging |
| v0.8.0 | RAG & Knowledge Base | Vector DB, document ingestion, semantic search |
| v0.9.0 | Mesh Network | Tailscale/EasyTier integration, P2P, NAT traversal |
| v1.0.0+ | Ecosystem | Multi-tenant, mobile app, browser automation, smart home |

## Branch Strategy

| Branch | Purpose |
|--------|---------|
| `main` | Stable releases, production ready |
| `develop` | Development branch, latest features |
| `feature/*` | Feature development branches |
| `hotfix/*` | Emergency fix branches |
| `release/*` | Release preparation branches |

## Release Process

1. Feature development on `feature/*` branches
2. Merge to `develop` after completion
3. Create `release/*` branch when preparing release
4. Merge to `main` and tag after testing
5. Update CHANGELOG.md

## Pre-release Versions

- **Alpha** (`x.x.x-alpha.n`): Early development, unstable
- **Beta** (`x.x.x-beta.n`): Feature complete, testing phase
- **RC** (`x.x.x-rc.n`): Release candidate, final testing

## Compatibility Policy

- **v0.x.x**: API may change between minor versions
- **v0.5.0+**: Stable API, backward compatibility within patch versions
