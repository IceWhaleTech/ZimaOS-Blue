![](../../docs/assets/banner.png)

<h2 align="center">ZimaOS Blue：面向大胆构建者的本地优先智能体运行时</h2>

<p align="center"><strong>开箱即用 · 开源 · 通用 · 厂商中立</strong></p>

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

受 OpenClaw 启发，我们相信，个人计算的未来将由运行在边缘侧、形态多样且本地优先的 AI Agent 所塑造。

ZimaOS Blue 就是我们的答案：一个完全开源、可审计、厂商中立、可直接投入生产的 Agent Runtime 与工具包，让你几乎零门槛地交付私有、自托管的 Agent。

面向想随心折腾或亲手打磨 Agent 的开发者，Blue 从一开始就为性能而生：使用 Go 编写，内存占用最低可至 19 MB。无论是 x86、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows 还是 ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS，只要能通电的地方，它都能跑起来。

## 演示

### 对话与任务执行

一个快速演示，展示 Blue 中的对话流程与任务执行。

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### LLM 提供商集成

一个快速演示，展示 Blue 的 LLM 提供商集成体验。

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### 快速总览 - 总览、通道与附加配置

一个快速演示，展示产品整体总览、通道和附加配置。

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## 为什么 Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### 纯 Go，任意设备

100% Go，静态二进制，开箱即可交叉编译到 5 个目标（![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`、`linux/arm64`、![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`、`darwin/arm64`、![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`）。不需要 Node 运行时，不需要 Python，也不需要容器。把它丢到 NAS、<a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、老旧 x86 路由器，或 ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac 上，它就能直接跑。之后你再叠加自己的 UI、逻辑和 Agent 技能就好：一套代码库，覆盖每个平台。

### 开箱即用，立即可用

大家都希望工具简单、可靠，并且在需要时能够扩展。最好是开箱就能用，这样你才能把精力放在真正想构建的东西上。

这并不是什么新理念。打造 <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS 的，正是同样的哲学：简单、可靠、不打扰。Blue 只是把这套哲学延伸到了 Agent 技术栈。

### 为真实生活而设计，坚持本地优先

从可输出完整 HTML 报告的深度研究，到 OCR、PDF、浏览器自动化和文档转换，Blue 都能在不把数据送上云端的前提下处理复杂的真实工作流。语音唤醒、STT/TTS、Talk Mode，以及对本地推理的支持，让日常交互更即时、更私密，也始终可用。

## 快速入门

### 选项 1：下载桌面应用程序

获取原生应用，无需依赖、无需编译。内置试用配置，几秒即可完成上手；通过远程连接即可立即开始聊天，不需要额外配置 bot。真正的开箱即用体验。

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [在 ZimaOS 上运行](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**：[下载DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**：[下载安装程序](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 选项 2：安装脚本

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 选项 3：从源代码构建

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **注意：** Windows 版本需要：
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) 和 [CMake](https://cmake.org/) 用于本机 C 依赖项（espeak-ng、whisper.cpp、opus、kokoro、onnx）
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) 用于系统库（winmm 等）
>
> 确保`gcc`、`cmake` 位于您的`PATH` 中。

## 架构概述

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

更进一步：它为 **20 多个 IM 平台**提供本机支持，**语音驱动**界面可实现自然的上下文感知对话，通过 IDE 扫描进行**零配置模型切换**。

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## 如何构建

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> 如果你计划继续在 Blue 之上做调优或 vibe coding，不要把几次看起来不错的聊天当作发布依据。任何会影响路由、执行行为、工具表面、预算控制、模型选择或执行框架的改动，都应该通过 Blue Harness 验证，而不是靠零散抽查。
>
> 在这件事上，Blue 只该遵循一条简单规则：先看数据，先过 gate，最后再 cut over。实际操作中，这意味着先更新对应的 Harness 数据集或评测规范，再在整个验证过程中保持同一个稳定的 `candidate_id`，这样 selector、execution、budget 和 readiness 报告描述的才会是同一个候选版本，而不是四次互不相关的运行。

### 推荐的 Harness 工作流

1. 运行 `blue harness selector verify`
2. 运行 `blue harness execution verify`
3. 复用 selector 的 eval 结果运行 `blue harness budget gate`
4. 最后运行 `blue harness cutover-readiness`

对于本地迭代、夜间验证或 CI 证据收集，优先使用 `python3 scripts/cutover_candidate_pipeline.py`。它会在同一个共享 candidate 下依次执行 selector -> execution -> budget -> readiness 全流程，让结果更容易比较、审阅和执行 cut over。

### 额外护栏

| 关注点 | 需要留意什么 |
|------|----------------|
| 基线稳定性 | 保持 baseline、数据集版本和 `candidate_id` 稳定，否则对比会漂移，结果也不可信。 |
| 真实构建产物 | 在运行 Harness 前，先重新构建受影响的二进制或前端产物，否则你验证的可能是旧行为，而不是当前改动。 |
| 路由注册 | 如果前后端一起改动，在通过 UI 行为判断功能前，先确认所有新的后端路由都已经正确注册，因为路由没注册常常看起来像逻辑 bug，实际上只是 `404`。 |
| 发布判断 | 只有当 Harness 没有显示出明显回归，且 cutover-readiness 确认候选版本确实可以切换时，这轮调优才算真正准备好。 |

简而言之，在 Blue 之上做调优，不能只靠“几次聊天感觉更好了”。你需要把候选版本放进 Harness，收集可比较的证据，再由 gate 和 readiness 的结果来决定这次改动是否真的安全可留。

## 功能特性

| 功能 | 提供能力 |
|--------------------|--------------------|
| 高可用 Web 检索与浏览器运行时 | Blue **最鲜明的差异化能力**之一。它统一了用于搜索、读取、提取与抓取的 **四条 Web 访问路径**；在 HTTP、代理提取与浏览器会话之间保留 **三层回退机制**；通过挑战检测、Cookie/会话复用、隐身与浏览器接管处理 **反机器人页面**；并可在 **三种浏览器引擎** 之间路由：`lightpanda`、托管 Chromium，以及中继/本地 Chrome。 |
| 三合一研究运行时 | **一个公开研究入口** 可路由到 `deep_research`、`analyze` 和 `ui_review`。同一套发现与证据栈随后可产出 **引用优先的研究结果**、**边界清晰的报告**，以及 **结构化的 UI/UX/无障碍评审**。 |
| Harness 运行时、评估与演化框架 | 让评估成为贯穿开发、训练与生产的 **运行时原语**。Harness 覆盖 **回归与 smoke 检查**、评分、基线、报告与运行时验证，并把同一份证据继续用于 **技能演化**、后续评估、晋级或回滚，以及 `AGENTS.md` 或指令提案审查。 |
| 多模态原生能力优先运行时 | 让 **语音、OCR、PDF、浏览器任务、文档转换、结构化表单填写、媒体处理与本地媒体生成** 优先走 **原生与本地路径**，仅在确有必要时才做 **模型路由**。 |
| 安全与治理 | 包含 **沙箱执行**、**提示注入防御**、**会话审计**、权限、**RBAC**、**WebAuthn**、运行护栏，以及 **技能安全扫描**。 |
| LLM Wiki 与知识空间 | 将记忆、研究和运行时输出组织成 **类 Wiki 的知识界面**，包含 **摘要页**、索引、**反向链接**、**新鲜度** 与 **归档工作流**。 |
| 技能商店与市场 | 提供 **内置技能发现**、策展、同步与 **本地扫描**，让扩展能力 **从第一天起就可用**。 |
| 生产级 Provider Pool | 提供具备 **健康检查**、**自动故障切换**、**熔断器** 与 **Provider 竞速** 的真实 Provider 池，用于支撑长时运行工作负载。 |
| 内置本地小模型运行时 | 内置 **`Qwen3.5-0.8B` + `llama.cpp`** 运行时，用于 **本地短问答**、图像识别、工具路由、摘要、**上下文压缩** 与 **文档预处理**。 |
| 长时运行可靠性 | 将 **OTA 更新**、**备份与恢复**、**配置热重载** 与 **故障后恢复** 视为 **内建运行能力**。 |

## 里程碑时间表

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

|日期 |版本 |关键词/特点|
|------|---------|---------------------|
| 2026 年 1 月 26 日 | `v0.1–v0.9` | Go 运行时、插件系统、浏览器自动化 |
| 2026 年 1 月 27 日至 28 日 | `v0.9.0–v0.9.2` |浏览器任务视图，Blue Companion，Smart Form Filler |
| 2026 年 1 月 29 日至 31 日 | `v0.10.0–v0.10.9` | Claude Code CLI、API Proxy、UI重构 |
| 2026 年 2 月 1 日至 3 日 | `v0.10.1–v0.10.22` |指标、远程访问、上下文缓存 |
| 2026 年 2 月 5 日至 18 日 | `v0.10.25–v0.10.29` | i18n、CC 缓存、发布管道 |
| 2026 年 2 月 20 日至 25 日 | `v0.10.28–v0.10.29` |桌面加载程序、移动用户体验、内存重新设计 |
| 2026 年 2 月 28 日至 3 月 2 日 | `v0.10.30` | Deep Research、技能重新排序、安全扫描 |
| 2026 年 3 月 9 日至 18 日 | `v0.10.31` |仪表板大修、VoiceChat 重构、批准站点 |
| 2026 年 3 月 19 日至 22 日 | `v0.10.32` | Harness 推出、成绩单审核、网络搜索 |
| 2026 年 3 月 23 日至 25 日 | `v0.10.33` | Harness 群组、浏览器批准、技能市场 |
| 2026 年 3 月 29 日至 30 日 | `v0.10.35` | Harness v3，浏览器中继，上下文压缩 |
| 2026 年 3 月 31 日至 4 月 1 日 | `v0.10.36` |转录审核、Harness 覆盖、工具解析 |
| 2026 年 4 月 1 日 | `v0.10.37` |运行时强化、Skill+Exec 切换、恢复抛光 |
| 2026 年 4 月 2 日至 5 日 | `v0.10.38` | GitHub 支持、市场完善、可靠性改进 |
| 2026 年 4 月 6 日至 7 日 | `v0.10.39` | 研究统一、进化表面、内存占用降低 |

## 社区与支持

- **问题**：[请在此处提交错误和功能请求](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **讨论**：[Discord](https://discord.gg/zwWbKA4S2)
- **在 [GitHub](https://github.com/IceWhaleTech) 上关注我们**

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## 许可证

该项目根据 MIT 许可证获得许可 - 有关详细信息，请参阅 [LICENSE](../../LICENSE) 文件。我们相信开源并回馈社区。

## 贡献者

感谢所有Blue 贡献者：

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## 参考文献

1. **OpenClaw** — 本地优先的开源代理。率先通过通道适配器和工具调用将LLM连接到本地设备，直接启发了Blue的代理运行时架构。 https://github.com/openclaw/openclaw
2. **MiroMind** — 具有证据支持的综合的深度研究模式。塑造Blue的内置深度研究管道：规划、并行检索、重复证据删除和HTML报告生成。 https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM 作为知识编译器。重新构建法学硕士以构建持久的、不断发展的知识空间，超越 RAG 的积累陷阱。
4. **OpenSpace (HKUDS)** — 自我进化的技能引擎。基于 DAG 的框架，代理可以从失败中学习并获得专业技能。 https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — 用于编码代理的版本化 API 文档注册表。解决座席幻觉和遗忘的会话知识。提供带有注释和反馈循环的精选、版本化文档，将文档转变为自我改进的知识层。 https://github.com/andrewyng/context-hub
6. **Notion** — 简单、人性化且有意保持安静。受到Notion 极简主义精神的启发，Blue 为网格带来了温暖。精致的衬线与贴心的设计相结合，打造出一个有家的感觉的空间。 https://www.notion.com/about
7. **Matrix** — 视觉灵感来自标志性的数字雨美学。 Blue技术图表的美学方向。
8. **IceWhale** — 爱、死亡与机器人 S2E2“冰”。一个聚集在世界各地的集体，旨在突破互联网巨头的围墙，抵制数据集中。冰鲸象征着一个在边缘共同构建主权工具的社区。
9. **ZimaOS Blue** — 爱、死亡与机器人 S1E14“Zima Blue”。一个比喻：智能始于服务，并不断发展以探索世界。 Blue 是智慧的代理人，植根于简单，触及深度。
10. **ZimaOS** — 简化、专注、开放的设计原则。 ZimaOS 和Blue 都坚信技术应该为用户服务——30 秒内部署、在任何地方运行、保持供应商中立。 https://www.zimaspace.com/zimaos
