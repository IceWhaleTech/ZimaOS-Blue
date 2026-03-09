![](../../docs/assets/banner.png)

<p align="center">
  面向大胆构建者的<strong>本地优先</strong>智能体运行时<br>
  开箱即用 · 开源 · 通用 · 双重监督
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
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
  <strong>简体中文</strong> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <a href="../../docs/assets/wechat_qrcode.png"><img src="../../docs/assets/wechat.png" height="128"/></a>
</p>

## 简介

受 Clawdbot 启发，我们相信个人计算的**未来**将由**多样化的、本地优先的 AI 智能体**在边缘端塑造。

**ZimaOS Blue 是我们的答案** —— 一个完全**开源、可审计、生产就绪的智能体运行时与工具集**，让你零摩擦地交付私有、自托管的智能体。

为那些想要**随心构建或精心打造自己智能体**的大胆开发者而生，Blue **为性能而设计**：使用 **Go** 编写，内存占用低至 10 MB。它可以运行在**任何 x86、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** —— 只要有电的地方。

![](../../docs/assets/features.png)

## 亮点

### 本地优先设计与自动模型接入

更进一步：它原生支持 **20+ 即时通讯平台**、**语音驱动**的自然上下文感知对话界面、**零配置模型切换**（支持 IDE 扫描）以及 SOUL 分层人格系统。

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

### 快速、轻量

Go 原生编译 —— 无解释器、无虚拟机、无额外开销。从服务器到桌面设备，静默运行于一切之上。

| 指标 | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|--------|-------------------|------------------------|
| `help` 冷启动 / 热启动 | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` 运行时间（3 次最优） | **< 0.01 s** | 5.98 s |
| `help` 峰值 RSS | **~10 MB** | ~394 MB |
| `status` 峰值 RSS | **~15 MB** | ~1.52 GB |
| 运行时依赖 | **无** | Node.js 18+ |

> 基准测试环境：macOS arm64（服务器模式，无桌面 UI），同一主机，3 次运行取最优。2026 年 2 月。

### 纯 Go，任意设备

100% Go，静态二进制。**开箱即可交叉编译至 5 个目标平台**（![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64、![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64、![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64）。无需 Node 运行时、无需 Python、无需容器。把它放到 NAS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、旧 x86 路由器或 ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac 上 —— 直接运行。**然后叠加你自己的 UI、逻辑和智能体技能** —— 一套代码，全平台通用。

### 安全与治理

内置 Sidecar API 代理，纵深防御：
- **沙箱执行** – 所有工具调用在隔离环境中运行。
- **提示注入防御** – 7+ 种内置拦截策略。
- **会话审计** – 全量会话监控，每次交互可追溯。
- **RBAC 与 WebAuthn** – 细粒度访问控制，支持无密码认证。

## 为什么选择 Blue

我们相信**下一代个人计算**将拥抱 LLM —— 但**可控、可审计**的智能体仍然是个人和团队的基石。**Blue 提供**：
- **全面的核心能力** – 高级模型管理、即时通讯集成、增强人格，以及为日常交互（耳机、语音、智能眼镜）调优的自然语言界面。
- **本地优先、超轻量、跨设备** – 无需高端硬件。能计算的地方就能运行。
- **安全可审计** – 会话审计、沙箱隔离、权限控制，以及内置的 API 代理充当应用层防火墙 —— 每一个字节的进出都清晰可见。

我们最大限度减少样板代码，让你**专注于真正重要的事**。秉承 <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ZimaOS 的设计哲学**，Blue 提供：
- **一键从零到一** – 即时部署，无需复杂配置。
- **快速原型开发** – 随心或精心打造场景化工具、交互和应用包。
- **全球化就绪** – **世界很大**，不应默认只有英语。**20+ 种语言，原生支持**，零障碍。
- **开放模型生态** – 无供应商锁定。自带模型即可。

![](../../docs/assets/design_principle.png)

## 快速开始

### 方式一：下载桌面应用

获取原生应用 —— 无依赖、无需编译。内置试用配置，秒级上手 —— 通过远程连接即刻开聊，无需配置机器人。真正的开箱即用。

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**：[下载 DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**：[下载安装程序](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 方式二：安装脚本

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 方式三：从源码构建

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **注意：** Windows 构建需要：
> - [MinGW-w64](https://www.mingw-w64.org/)（gcc）和 [CMake](https://cmake.org/) 来编译原生 C 依赖（espeak-ng、whisper.cpp、opus、kokoro、onnx）
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) 用于系统库（winmm 等）
>
> 请确保 `gcc`、`cmake` 已添加到 `PATH` 环境变量中。

## 架构概览

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### 包结构图（`server/internal/`）

| 层级 | 包 |
|-------|----------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Provider | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
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

</details>

### 数据流

**聊天请求（代理热路径）**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**频道消息流**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**语音管线**
```
WebSocket 音频 → STT (Whisper) → LLM 处理 → TTS (eSpeak/Edge) → WebSocket 音频
```

**心跳监控**
```
定时器 (30分钟) → 读取 HEARTBEAT.md → LLM 评估 → 剥离 HEARTBEAT_OK 令牌
  → 去重 (FNV 哈希, 24小时 TTL) → 频道告警 (Telegram/Slack/...)
  → 事件流 → UI 指示器
```

## 如何使用

![](../../docs/assets/handcraft.png)

## 里程碑时间线

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| 版本 | 重点 | 核心价值 | 状态 |
|---------|-------|-----------|--------|
| v0.1 | Go 运行时核心 | 稳定内核，24 小时运行 | 已完成 |
| v0.2 | 核心能力 | 最小可用，LLM 集成 | 已完成 |
| v0.3 | NAS 集成 | NAS 原生，systemd 支持 | 已完成 |
| v0.4 | 插件系统 | 可扩展，安全基础 | 已完成 |
| v0.5 | 产品基线 | 生产就绪，文档完善 | 已完成 |
| v0.6 | 消息频道 | 多频道支持 | 已完成 |
| v0.7 | 安全 | OIDC、MFA、审计 | 已完成 |
| v0.8 | 性能 | 优化、缓存、基准测试 | 已完成 |
| v0.9 | 生态系统 | 多租户、浏览器自动化、语音 | 已完成 |
| v0.10.0 | CLI 打包 | CC CLI 打包、检测、自动更新 | 已完成 |
| v0.10.1 | 指标监控 | API 统计、Token 追踪、TTFT | 已完成 |
| v0.10.2 | CLI 可靠性 | 进程生命周期、错误恢复 | 已完成 |
| v0.10.3 | CLI 集成 | 设置向导、提供商自动检测 | 已完成 |
| v0.10.4 | Tauri 打包 | 桌面应用、系统托盘 | 已完成 |
| v0.10.5 | API 代理 Sidecar | 路由选择、提示防护、用量统计 | 已完成 |
| v0.10.6 | Provider Pool | 多提供商路由、健康检查、故障转移 | 已完成 |
| v0.10.7 | 预览模式 | 免认证访问、功能门控 | 已完成 |
| v0.10.8 | 技能商店 | 技能商店基础设施、频道验证 | 已完成 |
| v0.10.9–10 | 用户管理 | 子用户、页面级权限 | 已完成 |
| v0.10.13–14 | 安全与技能 | 安全页面、技能商店重构 | 已完成 |
| v0.10.15 | 聊天增强 | 聊天体验、消息管线 | 已完成 |
| v0.10.16 | 语音模块 | Sherpa TTS/ASR、eSpeak、提供商切换 | 已完成 |
| v0.10.17 | 远程访问 | Ngrok、Cloudflare 隧道、ACME 证书 | 已完成 |
| v0.10.18–20 | 性能冲刺 | 启动/聊天性能、上下文缓存 | 已完成 |
| v0.10.21–22 | 提示词与钉钉 | 系统提示词、钉钉频道 | 已完成 |
| v0.10.23 | OTA 更新 | OTA 更新系统 | 已完成 |
| v0.10.24 | 频道升级 | 10 个频道从桩代码升级 | 已完成 |
| v0.10.25 | CC Cache | 两级缓存（L1 内存 + L2 磁盘） | 已完成 |
| v0.10.26 | Humanizer | 回复人性化管线 | 已完成 |
| v0.10.27 | 上下文裁剪器 | 代码场景节省 54% token（SWE-bench 官方数据），通用文档节省 46–47%（本地 IR），BM25 评分、分段 | 已完成 |
| v0.10.28 | 记忆服务 | 渐进式搜索、双写后端 | 已完成 |

</details>

## 社区与支持

- **问题反馈**：[请在此提交 Bug 和功能请求](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **讨论交流**：[Discord](https://discord.gg/b3AgFDxe9v)
- **关注我们**：[GitHub](https://github.com/IceWhaleTech)

## 许可证

本项目基于 MIT 许可证开源 - 详见 [LICENSE](../../LICENSE) 文件。我们信仰开源，致力于回馈社区。

## 贡献者

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
