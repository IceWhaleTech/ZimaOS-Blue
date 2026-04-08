![](./docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: A Local-First Agent Runtime for Bold Builders</h2>

<p align="center"><strong>Out-of-the-Box · Open-Source · Universal · Vendor-Neutral</strong></p>

<p align="center">
  <strong>English</strong> |
  <a href="./i18n/ca_ES/README.md">Català</a> |
  <a href="./i18n/cs_CZ/README.md">Čeština</a> |
  <a href="./i18n/da_DK/README.md">Dansk</a> |
  <a href="./i18n/de_DE/README.md">Deutsch</a> |
  <a href="./i18n/el_GR/README.md">Ελληνικά</a> |
  <a href="./i18n/en_GB/README.md">English (UK)</a> |
  <a href="./i18n/es_ES/README.md">Español</a> |
  <a href="./i18n/fr_FR/README.md">Français</a> |
  <a href="./i18n/ga_IE/README.md">Gaeilge</a> |
  <a href="./i18n/hr_HR/README.md">Hrvatski</a> |
  <a href="./i18n/hu_HU/README.md">Magyar</a> |
  <a href="./i18n/it_IT/README.md">Italiano</a> |
  <a href="./i18n/ja_JP/README.md">日本語</a> |
  <a href="./i18n/ko_KR/README.md">한국어</a> |
  <a href="./i18n/ml_IN/README.md">മലയാളം</a> |
  <a href="./i18n/nb_NO/README.md">Norsk Bokmål</a> |
  <a href="./i18n/nl_NL/README.md">Nederlands</a> |
  <a href="./i18n/pl_PL/README.md">Polski</a> |
  <a href="./i18n/pt_BR/README.md">Português (BR)</a> |
  <a href="./i18n/pt_PT/README.md">Português (PT)</a> |
  <a href="./i18n/ro_RO/README.md">Română</a> |
  <a href="./i18n/ru_RU/README.md">Русский</a> |
  <a href="./i18n/sk_SK/README.md">Slovenčina</a> |
  <a href="./i18n/sv_SE/README.md">Svenska</a> |
  <a href="./i18n/zh_CN/README.md">简体中文</a> |
  <a href="./i18n/zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

<p align="center">
  <a href="./docs-site/README.md"><strong>Docs</strong></a> ·
  <a href="https://deepwiki.com/IceWhaleTech/ZimaOS-Blue"><strong>DeepWiki</strong></a> ·
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><strong>Releases</strong></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="./docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="./docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="./docs/assets/x.png" alt="X" height="128" /></a>

</p>

## Introduction

Inspired by Clawdbot, we believe the future of personal computing will be shaped by diverse, local-first AI agents running at the edge.

ZimaOS Blue is our answer — a fully open-source, auditable, vendor-neutral, and production-ready agent runtime and toolkit that lets you ship private, self-hosted agents with zero friction.

Built for bold developers who want to vibe or handcraft their own agents, Blue is engineered for performance: written in Go, with a memory footprint as low as 19 MB. It runs on any x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS — anywhere you plug in power.

## Demos

### Conversation & Task Execution

A quick demo of conversation flow and task execution in Blue.

[Watch the demo](<./docs/assets/demo.mp4>)

### LLM Providers Integration

A quick demo of Blue's LLM providers integration experience.

[Watch the demo](<./docs/assets/demo Provider.mp4>)

### Quick Overview - Overview, Channels & Additional Configuration

A quick demo covering the overall product overview, channels, and additional configuration.

[Watch the demo](<./docs/assets/demo quickv4.mp4>)

## Why Blue

<p align="center">
  <img src="./docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, Any Device

100% Go, static binary. Cross-compiles to 5 targets out of the box (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). No Node runtime, no Python, no containers required. Drop it on a NAS, a ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, an old x86 router, or a ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — it just runs. Then layer on your own UI, logic, and agent skills — one codebase, every platform.

### Out of the Box, Ready to Work

Everyone wants tools that are simple, reliable, and scale when you need them. Tools that just work, so you can focus on what you're actually building.

This isn't a new philosophy. It's the same one that built <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: simple, reliable, and built to stay out of your way. Blue is that philosophy, extended to the agent stack.

### Designed for Your Life, Built to Stay Local

From deep research that delivers a full HTML report, to OCR, PDF, browser automation, and document conversion, Blue handles complex, real-world workflows without sending your data to the cloud. Voice wake, STT/TTS, Talk Mode, and support for local inference make everyday interactions instant, private, and always available.

## Quick Start

### Option 1: Download Desktop App

Get the native application — no dependencies, no compilation. Built-in trial configuration with onboarding in seconds — start chatting instantly via remote connection, no bot setup required. True out-of-the-box experience.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Download DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Download Installer](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Option 2: Install Script

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Option 3: Build from Source

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

> **Note:** Windows builds require:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) and [CMake](https://cmake.org/) for native C dependencies (espeak-ng, whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) for system libraries (winmm, etc.)
>
> Make sure `gcc`, `cmake` are in your `PATH`.

## Architecture Overview

<p align="center">
  <img src="./docs/assets/architecture.png" alt="architecture" />
</p>

Take it further: it delivers native support for **20+ IM platforms**, **voice‑driven** interfaces for natural, context‑aware dialogue, **zero‑config model switching** with IDE scanning.

<p align="center">
  <img src="./docs/assets/providers.png" alt="Supported Providers" />
</p>

## How to Build

<p align="center">
  <img src="./docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> If you plan to keep tuning or vibe coding on top of Blue, do not treat a few good-looking chats as release evidence. Any change that affects routing, execution behavior, tool surface, budget control, model selection, or the execution framework should be validated with Blue Harness, not with ad hoc spot checks.
>
> Blue should follow one simple rule here: data first, gates first, cut over last. In practice, that means updating the relevant Harness dataset / eval spec before judging a change, then keeping one stable `candidate_id` across the whole attempt so selector, execution, budget, and readiness reports all describe the same candidate instead of four unrelated runs.

### Recommended Harness Workflow

1. Run `blue harness selector verify`
2. Run `blue harness execution verify`
3. Reuse the selector eval run for `blue harness budget gate`
4. Finish with `blue harness cutover-readiness`

For local iteration, nightly validation, or CI evidence collection, prefer `python3 scripts/cutover_candidate_pipeline.py`. It runs the full selector -> execution -> budget -> readiness sequence under one shared candidate, which makes the result easier to compare, review, and cut over from.

### Extra Guardrails

| Area | What to Watch |
|------|----------------|
| Baseline stability | Keep the baseline, dataset version, and `candidate_id` stable, or the comparison will drift and the result will not be trustworthy. |
| Real build output | Rebuild the affected binary or frontend bundle before running Harness, otherwise you may end up validating stale behavior instead of the current change. |
| Route registration | If frontend and backend change together, confirm that all new backend routes are actually registered before judging the feature through UI behavior, because missing registration often looks like a logic bug but is really a `404`. |
| Release judgment | A tuning pass is only ready when Harness shows no meaningful regression and cutover-readiness confirms that the candidate is actually ready to cut over. |

In short, tuning on top of Blue is not about "it feels better in a few chats." It is about putting the candidate into Harness, collecting comparable evidence, and letting the gate and readiness results decide whether the change is truly safe to keep.

## Features

| Feature | What It Delivers |
|---------|-------------------|
| High-Availability Web Retrieval and Browser Runtime | One of Blue's **sharpest differentiators**. Blue unifies **four web access paths** for search, read, extract, and crawl; keeps **three fallback layers** across HTTP, proxy extraction, and browser sessions; handles **anti-bot pages** with challenge detection, cookie/session reuse, stealth, and browser handoff; and routes across **three browser engines**: `lightpanda`, managed Chromium, and relay/local Chromium. |
| Three-in-One Research Runtime | **One public research entry** can route into `deep_research`, `analyze`, and `ui_review`. The same discovery and evidence stack then produces **citation-first research**, **bounded reports**, and **structured UI/UX/accessibility reviews**. |
| Harness Runtime, Evaluation, and Evolution Framework | Makes evaluation a **runtime primitive** across development, training, and production. Harness covers **regression and smoke checks**, scoring, baselines, reports, and runtime validation, then carries the same evidence into **skill evolution**, follow-up evaluation, promotion or rollback, and `AGENTS.md` or instruction proposal review. |
| Multimodal Native-Capability-First Runtime | Keeps **voice, OCR, PDF, browser tasks, document conversion, structured form filling, media processing, and media generation** on **native and local paths first**, with **model routing only when it is actually needed**. |
| Security and Governance | Includes **sandbox execution**, **prompt-injection defense**, **session auditing**, permissions, **RBAC**, **WebAuthn**, operational guardrails, and **skill security scanning**. |
| LLM Wiki and Knowledge Space | Turns memory, research, and runtime outputs into a **wiki-like knowledge surface** with **summary pages**, indexes, **backlinks**, **freshness**, and **archive workflows**. |
| Skill Store and Marketplace | Ships **built-in skill discovery**, curation, sync, and **local scanning** so extensibility is available **from day one**. |
| Production-Grade Provider Pool | Provides a real provider pool with **health checks**, **automatic failover**, **circuit breakers**, and **provider racing** for long-running workloads. |
| Built-In Local Small Model Runtime | Ships a built-in **`Qwen3.5-0.8B` + `llama.cpp`** runtime for **local short Q&A**, image recognition, tool routing, summarization, **context compression**, and **document preprocessing**. |
| Long-Running Reliability | Treats **OTA updates**, **backup and restore**, **config hot reload**, and **post-failure recovery** as **built-in operating concerns**. |

## Milestone Timeline

<p align="center">
  <img src="./docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Date | Version | Keywords / Features |
|------|---------|---------------------|
| Jan 26, 2026 | `v0.1–v0.9` | Go runtime, plugin system, browser automation |
| Jan 27–28, 2026 | `v0.9.0–v0.9.2` | Browser task view, Blue Companion, Smart Form Filler |
| Jan 29–31, 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, UI restructure |
| Feb 1–3, 2026 | `v0.10.1–v0.10.22` | Metrics, remote access, context cache |
| Feb 5–18, 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, release pipeline |
| Feb 20–25, 2026 | `v0.10.28–v0.10.29` | Desktop loader, mobile UX, memory redesign |
| Feb 28–Mar 2, 2026 | `v0.10.30` | Deep Research, skill reranker, security scan |
| Mar 9–18, 2026 | `v0.10.31` | Dashboard overhaul, VoiceChat refactor, approved sites |
| Mar 19–22, 2026 | `v0.10.32` | Harness rollout, transcript audit, web search |
| Mar 23–25, 2026 | `v0.10.33` | Harness groups, browser approvals, skill market |
| Mar 29–30, 2026 | `v0.10.35` | Harness v3, browser relay, context compression |
| Mar 31–Apr 1, 2026 | `v0.10.36` | Transcript audit, Harness overlays, tool parsing |
| Apr 1, 2026 | `v0.10.37` | Runtime hardening, Skill+Exec cutover, recovery polish |
| Apr 2–5, 2026 | `v0.10.38` | GitHub support, marketplace refinement, reliability improvements |
| Apr 6–7, 2026 | `v0.10.39` | Research unification, evolution surfaces, memory footprint reduction |

## Community & Support

- **Issues**: [Please file bugs and feature requests here](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussions**: [Discord](https://discord.gg/zwWbKA4S2)
- **Follow us** on [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. We believe in open source and giving back to the community.

## Contributors

Thanks to all Blue contributors:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## References

1. **OpenClaw** — Local-first open source agent. Pioneered connecting LLMs to local devices through channel adapters and tool calling, directly inspiring Blue's agent runtime architecture. https://github.com/openclaw/openclaw
2. **MiroMind** — Deep research mode with evidence-backed synthesis. Shaped Blue's built-in deep research pipeline: planning, parallel retrieval, evidence deduplication, and HTML report generation. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM as knowledge compiler. Reframes LLMs to build persistent, evolving knowledge spaces, moving beyond RAG's accumulation trap.
4. **OpenSpace (HKUDS)** — Self-evolving skill engine. A DAG-based framework where agents learn from failures and derive specialized skills. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Versioned API documentation registry for coding agents. Addresses agent hallucinations and forgotten session knowledge. Provides curated, versioned docs with annotation and feedback loops, turning documentation into a self-improving knowledge layer. https://github.com/andrewyng/context-hub
6. **Notion** — Simple, human, and intentionally quiet. Inspired by the minimalist ethos of Notion, Blue brings warmth back to the grid. Where refined serif meets thoughtful design, crafting a space that feels like home. https://www.notion.com/about
7. **Matrix** — Visual inspiration from the iconic digital rain aesthetic. The aesthetic direction for Blue's technical diagrams.
8. **IceWhale** — Love, Death & Robots S2E2 "Ice". A collective that gathers worldwide to break through internet giants' walls and resist data concentration. The ice whale symbolizes a community building sovereign tools together at the edge.
9. **ZimaOS Blue** — Love, Death & Robots S1E14 "Zima Blue". A metaphor: intelligence that begins in service and evolves to explore the world. Blue is an agent for wisdom, rooted in simplicity and reaching for depth.
10. **ZimaOS** — Simplified, Focused, Open design principles. Both ZimaOS and Blue share the belief that technology should serve the user — deploy in 30 seconds, run anywhere, stay vendor-neutral. https://www.zimaspace.com/zimaos
