# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>安全、可观测的 AI 智能体运行时</strong>
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** 是面向 NAS 与边缘设备的轻量、高性能 AI 智能体运行时。使用 Go 构建，提供零配置部署、会话监控与全面使用分析的生产级平台。

[快速开始](#快速开始) · [功能特性](#核心功能)

## 亮点

| 规格 | 数值 |
|------|------|
| **二进制大小** | ~40MB（单可执行文件） |
| **内存（空闲）** | ~4MB |
| **启动时间** | < 1s |
| **依赖** | 无（零配置部署） |

## 核心功能

### 零配置部署

- **单一二进制**：下载即用，无需运行时依赖
- **按需配置**：开箱可用，需要时再自定义
- **跨平台**：Windows、macOS、Linux 同一二进制、同一体验
- **守护进程**：可作为常驻后台服务运行

### 会话监控

- **实时会话追踪**：监控所有活跃 AI 会话及实时状态
- **对话历史**：完整交互审计记录
- **会话回放**：回顾与分析历史对话
- **多租户隔离**：用户间会话完全分离

### 调用链优化

- **请求追踪**：每次 API 调用的端到端可见性
- **延迟分析**：定位请求链路中的瓶颈
- **提供商路由**：智能路由至最优 LLM 提供商
- **熔断器**：提供商故障时自动切换

### 使用分析

- **Token 消耗**：按用户、会话、提供商统计使用量
- **成本归属**：按操作详细成本拆分
- **限流**：按租户的配额管理
- **导出报告**：多种格式生成使用报告

### 安全加固

- **沙箱执行**：所有工具调用在隔离环境中运行
- **RBAC**：细粒度基于角色的访问控制
- **WebAuthn/Passkeys**：无密码 FIDO2 认证
- **MFA/TOTP**：多因素认证
- **审计链**：所有特权操作不可变日志

## 快速开始

```bash
# 从源码
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

在 `http://localhost:3000` 访问控制台。

## LLM 提供商配置

ZimaOS Blue 支持多种 LLM 提供商，包括本地 LLM 服务：

```yaml
llm:
  # 云提供商
  provider: "openai"  # 或 "anthropic", "azure" 等
  api_key: "your-api-key"

  # 本地 LLM（可选）
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## 架构

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

## 开发环境

### 前置条件

| 工具 | 版本 | 安装 |
|------|------|------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux 通常已预装 |

### 开发模式（热重载）

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- 前端：`http://localhost:3000`
- 后端：`http://localhost:23456`

### 构建命令

```bash
make build              # 构建单一二进制（前端内嵌）
make build-embedded     # 构建并内嵌 Claude Code CLI
make build-all          # 全平台交叉编译
make clean              # 清理构建产物
```

### 项目结构

```
ZimaOS-Blue/
├── server/             # Go 后端
│   ├── cmd/blue/       # 入口
│   └── internal/       # 核心模块
├── web/                # Vue 3 前端
│   └── src/
└── dist/               # 构建输出
```

## 致谢

- [clawdbot](https://github.com/clawdbot/clawdbot) - 项目灵感来源
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 轻量级 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
