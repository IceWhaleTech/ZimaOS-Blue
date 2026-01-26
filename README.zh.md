# ZimaOS Echo

<p align="center">
  <img src="docs-site/public/logo.svg" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>NAS 原生智能体运行时</strong>
</p>

<p align="center">
  <a href="README.md">English</a> | <strong>中文</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** 是一个专为 NAS 和边缘设备设计的轻量级、高性能 AI 智能体运行时。使用 Go 构建，为在低功耗硬件上运行 AI 助手提供了生产就绪的平台。

[文档](https://echo.zimaos.com) · [快速开始](#快速开始) · [功能特性](#功能特性) · [对比](#与-clawdbot-对比)

## 为什么选择 ZimaOS Echo？

ZimaOS Echo 受 [clawdbot](https://github.com/clawdbot/clawdbot) 启发，但使用 Go 从头重建，具有以下优势：

- **更低资源占用**：可在仅 256MB RAM 的设备上运行
- **更好性能**：原生 Go 二进制文件，基于高效的 goroutine 并发
- **更易部署**：单一二进制文件，无需 Node.js 运行时
- **NAS 优化**：专为低功耗设备 24/7 运行而设计

## 快速开始

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell 管理员权限)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### 从源码构建

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## 功能特性

### 核心功能

- 🚀 **轻量级**：单一二进制文件 < 15MB，内存 < 80MB
- ⚡ **高性能**：基于 Go 和 goroutine 并发
- 🔌 **多提供商**：OpenAI、Anthropic、Ollama 等
- 🛡️ **生产就绪**：熔断器、优雅降级、自动恢复
- 📊 **可观测性**：Prometheus 指标、pprof 性能分析、结构化日志
- 🔄 **热重载**：无需重启即可更改配置
- 💾 **备份/恢复**：自动备份，支持时间点恢复

### 前端

- 🎨 **Vue 3 仪表板**：现代化、响应式 Web 界面
- 💬 **聊天界面**：支持 Markdown 的流式响应
- 📈 **系统监控**：实时资源使用图表
- ⚙️ **设置界面**：便捷的配置管理

### 性能优化 (v0.8.0)

- 🗄️ **ECache**：使用 [orca-zhang/ecache](https://github.com/orca-zhang/ecache) 的高性能 LRU 缓存
- 📦 **Zorm ORM**：使用 [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) 的轻量级数据库层
- 🔀 **分片映射**：无锁并发数据结构
- 🌐 **HTTP/2**：支持压缩的现代协议
- 📊 **基准测试套件**：全面的性能测试

## 与 Clawdbot 对比

ZimaOS Echo 受 clawdbot 启发，但针对 NAS/边缘部署进行了优化：

| 特性 | ZimaOS Echo | Clawdbot |
|------|-------------|----------|
| **语言** | Go | TypeScript/Node.js |
| **二进制大小** | ~15MB | ~200MB+ (包含 node_modules) |
| **内存使用** | ~80MB 空闲 | ~200MB+ 空闲 |
| **启动时间** | < 1s | 3-5s |
| **运行时** | 原生二进制 | 需要 Node.js |
| **目标平台** | NAS/边缘设备 | 桌面/服务器 |

### 功能对比

| 功能 | ZimaOS Echo | Clawdbot |
|------|:-----------:|:--------:|
| **LLM 提供商** | | |
| OpenAI | ✅ | ✅ |
| Anthropic | ✅ | ✅ |
| Ollama (本地) | ✅ | ✅ |
| AWS Bedrock | ✅ | ✅ |
| **渠道** | | |
| Web 聊天 | ✅ | ✅ |
| Telegram | ✅ | ✅ |
| Discord | ✅ | ✅ |
| Slack | ✅ | ✅ |
| WhatsApp | ✅ | ✅ |
| Signal | ✅ | ✅ |
| iMessage | ✅ | ✅ |
| **功能** | | |
| 流式响应 | ✅ | ✅ |
| 工具调用 | ✅ | ✅ |
| 记忆/上下文 | ✅ | ✅ |
| 语音唤醒 | ✅ | ✅ |
| 浏览器控制 | ✅ | ✅ |
| Canvas/A2UI | ✅ | ✅ |
| **运维** | | |
| Prometheus 指标 | ✅ | ❌ |
| pprof 性能分析 | ✅ | ❌ |
| 热重载配置 | ✅ | ❌ |
| 熔断器 | ✅ | ❌ |
| 优雅降级 | ✅ | ❌ |
| 备份/恢复 | ✅ | ❌ |
| **部署** | | |
| 单一二进制 | ✅ | ❌ |
| Docker | ✅ | ✅ |
| systemd 服务 | ✅ | ✅ |
| ZimaOS 应用商店 | 🔜 | ❌ |

### ZimaOS Echo 的新功能

clawdbot 中不可用的功能：

| 功能 | 描述 |
|------|------|
| **Prometheus 指标** | 内置监控指标端点 |
| **pprof 性能分析** | CPU、内存、goroutine 性能分析 |
| **热重载** | 无需重启即可更改配置 |
| **熔断器** | 自动故障隔离 |
| **优雅降级** | 服务失败时的回退策略 |
| **LLM 回退链** | 自动提供商故障转移 |
| **备份/恢复** | 计划备份和时间点恢复 |
| **ECache** | 高性能 LRU 缓存 |
| **Zorm ORM** | 轻量级 SQLite ORM |
| **分片映射** | 无锁并发数据结构 |
| **HTTP/2 支持** | 支持压缩的现代协议 |
| **基准测试套件** | 性能回归检测 |

## 架构

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Vue 3 前端  │  REST API  │  WebSocket          │
├─────────────────────────────────────────────────┤
│              核心运行时 (Go)                      │
│  事件循环 │ 工作池 │ 配置 │ 日志                 │
├─────────────────────────────────────────────────┤
│              智能体运行时                         │
│  LLM 提供商 │ 工具 │ 记忆 │ 上下文              │
├─────────────────────────────────────────────────┤
│              数据层                              │
│  SQLite (Zorm) │ ECache │ 文件                  │
└─────────────────────────────────────────────────┘
```

## 配置

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8080

llm:
  provider: "openai"
  model: "gpt-4"
  api_key: "${OPENAI_API_KEY}"

cache:
  max_size: 1000
  default_ttl: 5m

resilience:
  circuit_breaker:
    enabled: true
    threshold: 5
    timeout_seconds: 30

metrics:
  enabled: true
  endpoint: "/metrics"

profiling:
  enabled: false
  endpoint_prefix: "/debug/pprof"
```

## 路线图

- [x] **v0.1.0** - 核心运行时（事件循环、工作池、配置、日志）
- [x] **v0.2.0** - 智能体运行时（LLM 提供商包括 AWS Bedrock、工具、记忆）
- [x] **v0.3.0** - API 层（REST、WebSocket、流式传输）
- [x] **v0.4.0** - 插件系统（Go 模块、WASM 支持）
- [x] **v0.5.0** - 生产就绪（指标、性能分析、备份）
- [x] **v0.6.0** - 消息渠道（Telegram、Discord、Slack、iMessage）
- [ ] **v0.7.0** - 安全性（OIDC、MFA、审计日志）
- [x] **v0.8.0** - 性能（ECache、Zorm、HTTP/2）
- [x] **v0.9.0** - 未来增强（A2UI、浏览器自动化）
- [ ] **v1.0.0** - RAG 和知识库

## 贡献

欢迎贡献！请阅读我们的[贡献指南](CONTRIBUTING.md)了解详情。

```bash
# 克隆仓库
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# 安装依赖
cd server && go mod download

# 运行测试
go test ./...

# 构建
go build -o zimaos-echo ./cmd/server
```

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE)。

## 致谢

- [clawdbot](https://github.com/clawdbot/clawdbot) - 项目灵感来源
- [orca-zhang/ecache](https://github.com/orca-zhang/ecache) - 高性能缓存
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 轻量级 ORM

---

<p align="center">
  由 <a href="https://github.com/IceWhaleTech">IceWhaleTech</a> 用 ❤️ 制作
</p>
