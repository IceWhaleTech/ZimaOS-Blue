---
layout: home

hero:
  name: ZimaOS Echo
  text: NAS-Native Agent Runtime
  tagline: Lightweight, high-performance AI agent runtime for low-power NAS devices
  image:
    src: /logo.svg
    alt: ZimaOS Echo
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/IceWhaleTech/ZimaOS-Echo/server

features:
  - icon: 🚀
    title: Lightweight
    details: Single binary under 15MB, memory usage < 80MB, optimized for NAS devices
  - icon: ⚡
    title: High Performance
    details: Built with Go for high throughput and low latency, leveraging goroutines
  - icon: 🔌
    title: Extensible
    details: Plugin-based architecture with Go modules or WASM support
  - icon: 🛡️
    title: Stable
    details: Designed for 24/7 operation with graceful shutdown and auto-recovery
  - icon: 🎯
    title: One-Click Deploy
    details: Simple installation script for Linux, macOS, and Windows
  - icon: 🌐
    title: Multi-Channel
    details: Support for Web, Telegram, Discord, Slack, and more (coming soon)
---

## Install from Source

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

See [Installation](guide/installation.md) for more options.

## What is ZimaOS Echo?

ZimaOS Echo is a **NAS-native Agent Runtime** built with Go, inspired by [clawdbot](https://github.com/clawdbot/clawdbot). It's designed specifically for low-power NAS and edge devices, providing:

- **Minimal Resource Usage**: Runs efficiently on devices with limited CPU and memory
- **Long-term Stability**: Built for 24/7 operation without restarts
- **Easy Deployment**: One-click installation with systemd integration
- **Modern Frontend**: Vue 3 dashboard for monitoring and management

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
│  SQLite │ BoltDB │ Files                        │
└─────────────────────────────────────────────────┘
```
