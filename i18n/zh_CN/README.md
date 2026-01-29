# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>安全、可观测、本地优先的 AI 智能体运行时</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <strong>简体中文</strong> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_BR/README.md">Português</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** 是面向 NAS 与边缘设备的加固型 AI 智能体运行时。数据留存于你的硬件，行为可审计，AI 运行于隔离沙箱。

[文档](https://echo.zimaos.com) · [快速开始](#快速开始) · [功能特性](#核心原则) · [对比](#与-clawdbot-对比)

## 为什么选择 ZimaOS Echo？

ZimaOS Echo 受 [clawdbot](https://github.com/clawdbot/clawdbot) 启发，使用 Go 重建，具备：

- **更低资源占用**：可在仅 256MB RAM 的设备上运行
- **更好性能**：原生 Go 二进制，基于 goroutine 并发
- **更易部署**：单一二进制，无需 Node.js
- **NAS 优化**：为低功耗设备 24/7 运行设计

## 核心原则

### 本地优先

- **数据主权**：所有数据本地存储在 NAS，无云依赖
- **Ollama 集成**：LLM 完全本机运行，零外部 API 调用
- **离线可用**：核心功能无需联网
- **单一二进制**：约 15MB 原生 Go 二进制，无运行时依赖

### 可观测与可审计

- **审计日志**：每次 AI 操作带完整上下文与时间戳记录
- **Prometheus 指标**：系统操作实时监控
- **pprof 分析**：CPU、内存、goroutine 深度可见性
- **结构化日志**：JSON 日志便于解析与告警

### 安全加固

- **沙箱执行**：所有工具调用在隔离环境中运行
- **RBAC**：细粒度基于角色的访问控制
- **WebAuthn/Passkeys**：无密码 FIDO2 认证
- **MFA/TOTP**：多因素认证
- **OIDC/OAuth 2.0**：企业 SSO 集成
- **熔断器**：自动故障隔离，防止级联失败

## 快速开始

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# 从源码
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## 安全加固

### 认证体系

| 层级 | 技术 | 用途 |
|------|------|------|
| 主要 | WebAuthn/Passkeys | 防钓鱼无密码认证 |
| 次要 | TOTP/MFA | 基于时间的一次性密码 |
| 企业 | OIDC/OAuth 2.0 | 与 Google、GitHub、Okta 等 SSO |
| 授权 | RBAC | 按资源的权限控制 |

### 运行时保护

- **沙箱隔离**：工具在受限环境中执行
- **限流**：按租户的 API 节流
- **租户隔离**：数据与资源完全分离
- **审计链**：所有特权操作不可变日志

### 韧性

- **熔断器**：故障时自动隔离服务
- **优雅降级**：提供商不可用时的回退策略
- **LLM 回退链**：自动切换提供商
- **热重载**：配置变更无需重启

## 可观测性

```yaml
# 启用完整可观测性栈
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

### 暴露的指标

- 请求延迟（p50、p95、p99）
- 各提供商 LLM 的 token 使用量
- 工具执行成功/失败率
- 内存与 goroutine 计数
- 熔断器状态转换

## 架构

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

## 本地 LLM 配置（Ollama）

完全离线运行 AI，无需外部 API 调用：

```bash
# 安装 Ollama
curl -fsSL https://ollama.com/install.sh | sh

# 拉取模型
ollama pull llama3.2

# 配置 Echo 使用本地 LLM
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## 开发环境

### 前置条件

| 工具 | 版本 | 安装 |
|------|------|------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux 通常已预装 |

### 一键启动

```bash
# 克隆并启动
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

在 `http://localhost:3000` 访问控制台。

### 开发模式（热重载）

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- 前端：`http://localhost:5173`（API 代理到后端）
- 后端：`http://localhost:8080`

### 构建命令

```bash
make build              # 构建单一二进制（前端内嵌）
make build-embedded     # 构建并内嵌 Claude Code CLI
make build-all          # 全平台交叉编译
make clean              # 清理构建产物
```

### 项目结构

```
ZimaOS-Echo/
├── server/             # Go 后端
│   ├── cmd/echo/       # 入口
│   └── internal/       # 核心模块
├── web/                # Vue 3 前端
│   └── src/
└── dist/               # 构建输出
```

## 与 Clawdbot 对比

ZimaOS Echo 受 clawdbot 启发，针对 NAS/边缘部署优化：

| 特性 | ZimaOS Echo | Clawdbot |
|------|-------------|----------|
| **语言** | Go | TypeScript/Node.js |
| **二进制大小** | ~15MB | ~200MB+（含 node_modules）|
| **内存占用** | ~80MB 空闲 | ~200MB+ 空闲 |
| **启动时间** | < 1s | 3–5s |
| **运行时** | 原生二进制 | 需 Node.js |
| **目标平台** | NAS/边缘设备 | 桌面/服务器 |

## 致谢

- [clawdbot](https://github.com/clawdbot/clawdbot) - 项目灵感来源
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 轻量级 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
