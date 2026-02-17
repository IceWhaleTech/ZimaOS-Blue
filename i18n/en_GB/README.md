![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <strong>English (UK)</strong> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../hr_HR/README.md">Hrvatski</a> |
  <a href="../hu_HU/README.md">Magyar</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../ml_IN/README.md">മലയാളം</a> |
  <a href="../nb_NO/README.md">Norsk Bokmål</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT Licence"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introduction

Inspired by Clawdbot, we believe the **future** of personal computing will be **shaped by diverse, local-first AI agents** running at the edge.

**ZimaOS Blue is our answer** — a fully **open-source, auditable, and production-ready agent runtime and toolkit** that lets you ship private, self-hosted agents with zero friction.

Built for bold developers who want to **vibe or handcraft their own agents**, Blue is **engineered for performance**: written in **Go**, with a memory footprint as low as 10 MB. It runs on **any x86, Raspberry Pi, Windows, macOS** — anywhere you plug in power.

![](../../docs/assets/features.png)

## Highlights

### Local-First Design & Automatic Model Access

Take it further: it delivers native support for **20+ IM platforms**, **voice-driven** interfaces for natural, context-aware dialogue, **zero-config model switching** with IDE scanning, and SOUL-layered personalities.

### Fast, Light

Compiled natively in Go — no interpreter, no VM, no overhead. Runs silently on everything from servers to your desktop devices.

| Metric | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|--------|-------------------|------------------------|
| `--help` cold / warm | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` runtime (best of 3) | **< 0.01 s** | 5.98 s |
| `--help` peak RSS | **~10 MB** | ~394 MB |
| `status` peak RSS | **~15 MB** | ~1.52 GB |
| Runtime dependencies | **None** | Node.js 18+ |

> Benchmarked on macOS arm64, same host, best of 3 runs. Feb 2026.

### Pure Go, Any Device

100% Go, static binary. **Cross-compiles to 5 targets** out of the box (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). No Node runtime, no Python, no containers required. Drop it on a NAS, a Raspberry Pi, an old x86 router, or a Mac — it just runs. **Then layer on your own UI, logic, and agent skills** — one codebase, every platform.

### Security & Governance

Built-in sidecar API proxy with defence in depth:
- **Sandbox Execution** – All tool calls run in isolated environments.
- **Prompt Injection Defence** – 7+ built-in interception strategies.
- **Session Auditing** – Full session monitoring, every interaction traceable.
- **RBAC & WebAuthn** – Fine-grained access control with passwordless auth.

## Why Blue

We believe **next-gen personal computing** embraces LLMs — but **controllable, auditable** agents remain the bedrock for both individuals and teams. **Blue delivers**:
- **Comprehensive Core** – Advanced model management, IM integration, enhanced persona, and natural language interfaces tuned for daily interactions (headsets, voice, smart glasses).
- **Local-First, Ultra-Lightweight, Cross-Device** – No high-end hardware required. Runs on anything that can compute.
- **Secure & Auditable** – Session auditing, sandboxing, permission controls, and a built-in API proxy that acts as an application-layer firewall — every byte in/out is visible.

We minimise boilerplate so you **focus on what matters**. Staying true to **ZimaOS's design philosophy**, Blue delivers:
- **Zero-to-One in One Click** – Deploy instantly, no complex config.
- **Rapid Prototyping** – Vibe or handcraft scenario-specific tools, interactions, and app packages.
- **Global-Ready** – **The world is huge**, and it doesn't default to English. **20+ languages, native**, no barriers.
- **Open Model Ecosystem** – No vendor lock-in. Bring your own models.

![](../../docs/assets/design_principle.png)

## Quick Start

### Option 1: Download Desktop App (macOS & Windows)

Get the native application — no dependencies, no compilation.

- **macOS**: [Download DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Download Installer](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Option 2: Install Script

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Option 3: Build from Source

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

**macOS / Linux**
```bash
sh dev.sh
```

**Windows (PowerShell)**
```powershell
.\dev.bat
```

## Architecture Overview

![](../../docs/assets/architecture.png)

### Data Flow

**Chat Request (Proxy Hot Path)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Channel Message Flow**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humaniser (MD→text) → Channel → User
```

**Voice Pipeline**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Package Map (`server/internal/`)

| Layer | Packages |
|-------|----------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Provider | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humaniser |
| Memory | memory, embedding, kvstore |
| Channel | channel, autoreply, i18n |
| Security | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voice | voice, tts, stt, speech |
| Observe | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrate | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Scheduler | scheduler, worker, workerpool, pool |
| Core | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

## How to Use

![](../../docs/assets/handcraft.png)

## Milestone Timeline

![](../../docs/assets/timeline.png)

| Version | Focus | Key Value | Status |
|---------|-------|-----------|--------|
| v0.1 | Go Runtime Core | Stable kernel, 24h running | Done |
| v0.2 | Core Capabilities | Minimal usable, LLM integration | Done |
| v0.3 | NAS Integration | NAS native, systemd support | Done |
| v0.4 | Plugin System | Extensible, security basics | Done |
| v0.5 | Product Baseline | Production ready, documentation | Done |
| v0.6 | Message Channels | Multi-channel support | Done |
| v0.7 | Security | OIDC, MFA, audit | Done |
| v0.8 | Performance | Optimisation, caching, benchmarks | Done |
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
| v0.10.26 | Humaniser | Response humanisation pipeline | Done |
| v0.10.27 | Context Pruner | 54% token savings on code (SWE-bench official), 46–47% on general docs (local IR), BM25 scoring, segmentation | Done |
| v0.10.28 | Memory Service | Progressive search, dual-write backend | Done |

## Community & Support

- **Issues**: [Please file bugs and feature requests here](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussions**: [Discord](https://discord.gg/SrCYvumF)
- **Follow us** on [GitHub](https://github.com/IceWhaleTech)

## Licence

This project is licenced under the MIT Licence - see the [LICENCE](../../LICENSE) file for details. We believe in open source and giving back to the community.

## Contributors

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
