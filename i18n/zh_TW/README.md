![](../../docs/assets/banner.png)

<p align="center">
  面向大膽構建者的<strong>本地優先</strong>智能體運行時<br>
  開箱即用 · 開源 · 通用 · 廠商中立
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
  <a href="../zh_CN/README.md">简体中文</a> |
  <strong>繁體中文</strong>
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

## 簡介

受 Clawdbot 啟發，我們相信**個人運算的未來**將由**多元化、本地優先的 AI 代理**在邊緣端塑造。

**ZimaOS Blue 是我們的答案** — 一個完全**開源、可審計、生產就緒的代理執行環境與工具包**，讓你零摩擦地部署私有、自託管的代理。

專為勇於**自由創造或精心打造自己代理**的開發者而生，Blue **為效能而設計**：以 **Go** 編寫，記憶體佔用低至 10 MB。可在**任何 x86、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、Windows、macOS** 上運行 — 只要有電源就能啟動。

![](../../docs/assets/features.png)

## 亮點

### 本地優先設計與自動模型存取

更進一步：原生支援 **20+ 即時通訊平台**、**語音驅動**介面實現自然的上下文感知對話、搭配 IDE 掃描的**零配置模型切換**，以及 SOUL 分層人格系統。

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### 快速、輕量

以 Go 原生編譯 — 無直譯器、無虛擬機、無額外開銷。從伺服器到桌面裝置，靜默運行於一切設備上。

| 指標 | ZimaOS Blue (Go) | Reference Agent (Node + dist) |
|--------|-------------------|------------------------|
| `help` 冷啟動 / 熱啟動 | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` 執行時間（最佳 3 次） | **< 0.01 s** | 5.98 s |
| `help` 峰值 RSS | **~10 MB** | ~394 MB |
| `status` 峰值 RSS | **~15 MB** | ~1.52 GB |
| 執行期依賴 | **無** | Node.js 18+ |

> 基準測試環境：macOS arm64（伺服器模式，無桌面 UI），同一主機，3 次運行取最佳結果。2026 年 2 月。

### 純 Go，任何裝置

100% Go，靜態二進位檔。**開箱即可交叉編譯至 5 個目標平台**（![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64、![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64、![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64）。無需 Node 執行環境、無需 Python、無需容器。放到 NAS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、舊的 x86 路由器或 Mac 上 — 直接運行。**然後疊加你自己的 UI、邏輯和代理技能** — 一套程式碼，所有平台。

### 安全與治理

內建側車 API 代理，具備縱深防禦：
- **沙箱執行** – 所有工具呼叫在隔離環境中運行。
- **提示注入防禦** – 7+ 種內建攔截策略。
- **會話審計** – 完整的會話監控，每次互動皆可追溯。
- **RBAC 與 WebAuthn** – 細粒度存取控制搭配無密碼認證。

## 為何選擇 Blue

我們相信**下一代個人運算**擁抱 LLM — 但**可控、可審計**的代理仍是個人與團隊的基石。**Blue 提供**：
- **全面的核心** – 進階模型管理、即時通訊整合、增強人格，以及為日常互動（耳機、語音、智慧眼鏡）調校的自然語言介面。
- **本地優先、超輕量、跨裝置** – 無需高階硬體。任何能運算的裝置都能運行。
- **安全且可審計** – 會話審計、沙箱、權限控制，以及作為應用層防火牆的內建 API 代理 — 每一個位元組的進出皆可見。

![](../../docs/assets/design_principle.png)

我們最小化樣板程式碼，讓你**專注於真正重要的事**。秉持 <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ZimaOS 的設計哲學**，Blue 提供：
- **一鍵從零到一** – 即時部署，無需複雜配置。
- **快速原型開發** – 自由創造或精心打造場景專屬的工具、互動和應用套件。
- **全球就緒** – **世界很大**，不以英語為預設。**20+ 種語言，原生支援**，無障礙。
- **開放模型生態** – 無供應商鎖定。自帶模型。

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| 供應商 | 模型 | 類型 |
|----------|--------|------|
| OpenAI | GPT-4o, GPT-4, o1, o3 | 雲端 |
| Anthropic | Claude 4.5, Claude 4 | 雲端 |
| Google | Gemini 2.5, Gemini 2.0 | 雲端 |
| Ollama | Llama, Qwen, Gemma, Phi 等 | 本地 |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | 雲端 |
| Grok | Grok-3, Grok-3-mini | 雲端 |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | 雲端 |
| GLM | GLM-4, GLM-4-Flash | 雲端 |
| Moonshot | Moonshot-v1 | 雲端 |
| MiniMax | abab6.5, abab5.5 | 雲端 |
| Venice | Llama, Mistral（隱私優先） | 雲端 |
| AWS Bedrock | Claude, Llama, Titan | 雲端 |
| Azure | 透過 Azure 使用 OpenAI 模型 | 雲端 |
| OpenRouter | 100+ 聚合模型 | 雲端 |
| AIHubMix | 多供應商聚合器 | 雲端 |
| Codex | OpenAI Codex | 雲端 |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | 雲端 |
| 自訂 | 任何 OpenAI / Anthropic / Gemini 相容 API | 雲端 / 本地 |

</details>

### 支援的 IDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## 快速開始

### 選項 1：下載桌面應用程式

取得原生應用程式 — 無需依賴、無需編譯。內建試用配置，秒級上手 — 透過遠端連線即刻開聊，無需配置機器人。真正的開箱即用。

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**：[下載 DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**：[下載安裝程式](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 選項 2：安裝腳本

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 選項 3：從原始碼建置

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **注意：** Windows 建置需要 [MinGW-w64](https://www.mingw-w64.org/)（gcc）和 [CMake](https://cmake.org/) 來編譯原生 C 相依套件（espeak-ng、whisper.cpp、opus）。請確保 `gcc` 和 `cmake` 已加入 `PATH` 環境變數中。

## 架構概覽

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### 套件地圖（`server/internal/`）

| 層級 | 套件 |
|-------|----------|
| 閘道層 | bootstrap, server, gateway |
| 代理層 | proxy, connection, streaming, resilience |
| 供應商層 | providerpool, providers, llm |
| 裁剪層 | pruner (detector, segmenter, bm25, pipeline, cache) |
| 智能體層 | context, tools, personality, humanizer |
| 記憶層 | memory, embedding, kvstore |
| 頻道層 | channel, autoreply, i18n |
| 安全層 | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| 語音層 | voice, tts, stt, speech |
| 觀測層 | metrics, companion, profiling, leakdetect |
| 外掛層 | plugin, skill, skillstore |
| 整合層 | browser, cron, workflow, formfiller, tunnel, crawler |
| 排程層 | scheduler, worker, workerpool, pool |
| 核心層 | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| 系統層 | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| 多租戶層 | tenant, user, session, preview |

</details>

### 資料流

**聊天請求（代理熱路徑）**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**頻道訊息流**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**語音管線**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```


## 使用方式

![](../../docs/assets/handcraft.png)

## 里程碑時間線

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| 版本 | 重點 | 核心價值 | 狀態 |
|---------|-------|-----------|--------|
| v0.1 | Go 執行環境核心 | 穩定核心，24 小時運行 | Done |
| v0.2 | 核心能力 | 最小可用，LLM 整合 | Done |
| v0.3 | NAS 整合 | NAS 原生，systemd 支援 | Done |
| v0.4 | 外掛系統 | 可擴展，安全基礎 | Done |
| v0.5 | 產品基線 | 生產就緒，文件完善 | Done |
| v0.6 | 訊息頻道 | 多頻道支援 | Done |
| v0.7 | 安全性 | OIDC、MFA、審計 | Done |
| v0.8 | 效能 | 最佳化、快取、基準測試 | Done |
| v0.9 | 生態系統 | 多租戶、瀏覽器自動化、語音 | Done |
| v0.10.0 | CLI 整合 | CC CLI 整合、偵測、自動更新 | Done |
| v0.10.1 | 指標監控 | API 統計、Token 追蹤、TTFT | Done |
| v0.10.2 | CLI 可靠性 | 程序生命週期、錯誤恢復 | Done |
| v0.10.3 | CLI 整合 | 設定精靈、供應商自動偵測 | Done |
| v0.10.4 | Tauri 打包 | 桌面應用、系統匣 | Done |
| v0.10.5 | API 代理側車 | 路由選擇、提示防護、用量統計 | Done |
| v0.10.6 | 供應商池 | 多供應商路由、健康檢查、故障轉移 | Done |
| v0.10.7 | 預覽模式 | 免認證存取、功能閘控 | Done |
| v0.10.8 | 技能商店 | 技能商店基礎設施、頻道驗證 | Done |
| v0.10.9–10 | 使用者管理 | 子使用者、頁面級權限 | Done |
| v0.10.13–14 | 安全與技能 | 安全頁面、技能商店重新設計 | Done |
| v0.10.15 | 聊天增強 | 聊天體驗、訊息管線 | Done |
| v0.10.16 | 語音模組 | Sherpa TTS/ASR、eSpeak、供應商切換 | Done |
| v0.10.17 | 遠端存取 | Ngrok、Cloudflare 隧道、ACME 憑證 | Done |
| v0.10.18–20 | 效能衝刺 | 啟動/聊天效能、上下文快取 | Done |
| v0.10.21–22 | 提示與 DingTalk | 系統提示、DingTalk 頻道 | Done |
| v0.10.23 | OTA 更新 | OTA 更新系統 | Done |
| v0.10.24 | 頻道升級 | 10 個頻道從存根升級 | Done |
| v0.10.25 | CC Cache | 兩級快取（L1 記憶體 + L2 磁碟） | Done |
| v0.10.26 | Humanizer | 回應人性化管線 | Done |
| v0.10.27 | 上下文裁剪器 | 代碼場景節省 54% token（SWE-bench 官方數據），通用文檔節省 46–47%（本地 IR），BM25 評分、分段 | Done |
| v0.10.28 | 記憶服務 | 漸進式搜尋、雙寫後端 | Done |

</details>

## 社群與支援

- **問題回報**：[請在此提交錯誤與功能請求](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **討論交流**：[Discord](https://discord.gg/b3AgFDxe9v)
- **關注我們**：[GitHub](https://github.com/IceWhaleTech)

## 授權條款

本專案採用 MIT 授權條款 — 詳見 [LICENSE](../../LICENSE) 檔案。我們信仰開源，並致力於回饋社群。

## 貢獻者

<p align="center">
  由 <a href="https://github.com/IceWhaleTech">IceWhaleTech</a> 用心打造
</p>
