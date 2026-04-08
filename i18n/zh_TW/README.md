![](../../docs/assets/banner.png)

<h2 align="center">ZimaOS Blue：面向大膽構建者的本地優先智能體運行時</h2>

<p align="center"><strong>開箱即用 · 開源 · 通用 · 廠商中立</strong></p>

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

受 OpenClaw 啟發，我們相信，個人運算的未來將由運行在邊緣端、形態多樣且本地優先的 AI Agent 所塑造。

ZimaOS Blue 就是我們的答案：一個完全開源、可稽核、供應商中立、可直接投入生產的 Agent Runtime 與工具套件，讓你幾乎零摩擦地交付私有、自託管的 Agent。

面向想隨心折騰或親手打造 Agent 的開發者，Blue 從一開始就為效能而生：使用 Go 編寫，記憶體占用最低可至 19 MB。無論是 x86、<a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows 還是 ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS，只要能通電的地方，它都能跑起來。

## 示範

### 對話與任務執行

一個快速示範，展示 Blue 中的對話流程與任務執行。

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### LLM 供應商整合

一個快速示範，展示 Blue 的 LLM 供應商整合體驗。

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### 快速總覽 - 總覽、通道與附加配置

一個快速示範，展示產品整體總覽、通道與附加配置。

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## 為什麼 Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### 純 Go，任意裝置

100% Go，靜態二進位，開箱即可交叉編譯到 5 個目標（![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`、`linux/arm64`、![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`、`darwin/arm64`、![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`）。不需要 Node 執行環境、不需要 Python，也不需要容器。把它丟到 NAS、<a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、老舊 x86 路由器，或 ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac 上，它就能直接跑。之後你再疊加自己的 UI、邏輯與 Agent 技能就好：一套程式碼庫，覆蓋每個平台。

### 開箱即用，立即可用

大家都想要簡單、可靠，且在需要時能夠擴展的工具。最好是開箱就能用，讓你把注意力放回真正想打造的東西。

這不是什麼新哲學。打造 <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS 的，正是同樣的理念：簡單、可靠、不擋路。Blue 只是把這套理念延伸到了 Agent 技術棧。

### 為真實生活而設計，堅持資料留在本地

從可輸出完整 HTML 報告的深度研究，到 OCR、PDF、瀏覽器自動化與文件轉換，Blue 都能在不把資料送上雲端的前提下處理複雜的真實工作流。語音喚醒、STT/TTS、Talk Mode，以及對本地推理的支援，讓日常互動更即時、更私密，也始終可用。

## 快速入門

### 選項 1：下載桌面應用程式

取得原生應用程式，無需依賴、無需編譯。內建試用配置，幾秒就能完成上手；透過遠端連線即可立刻開始聊天，不需要額外設定 bot。真正的開箱即用體驗。

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [在 ZimaOS 上執行](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**：[下載DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**：[下載安裝程式](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### 選項 2：安裝腳本

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### 選項 3：從原始碼構建

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
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) 和 [CMake](https://cmake.org/) 用於本機 C 依賴項（espeak-ng、whisper.cpp、opus、kokoro、onnx）
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) 用於系統函式庫（winmm 等）
>
> 確保`gcc`、`cmake` 位於您的`PATH`。

## 架構概述

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

更進一步：它為 **20 多個 IM 平台**提供本機支持，**語音驅動**介面可實現自然的上下文感知對話，透過 IDE 掃描進行**零配置模型切換**。

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## 如何建構

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> 如果你打算繼續在 Blue 之上做調優或 vibe coding，不要把幾次看起來不錯的聊天當成發版依據。任何會影響路由、執行行為、工具介面、預算控制、模型選擇或執行框架的改動，都應該透過 Blue Harness 驗證，而不是靠零散抽查。
>
> 在這件事上，Blue 只該遵循一條簡單規則：先看資料，先過 gate，最後再 cut over。實際操作中，這意味著先更新對應的 Harness 資料集或評測規格，再在整個驗證過程中維持同一個穩定的 `candidate_id`，這樣 selector、execution、budget 和 readiness 報告描述的才會是同一個候選版本，而不是四次互不相干的執行。

### 推薦的 Harness 工作流

1. 執行 `blue harness selector verify`
2. 執行 `blue harness execution verify`
3. 重用 selector 的 eval 結果執行 `blue harness budget gate`
4. 最後執行 `blue harness cutover-readiness`

對於本地迭代、夜間驗證或 CI 證據收集，優先使用 `python3 scripts/cutover_candidate_pipeline.py`。它會在同一個共享 candidate 下依序執行 selector -> execution -> budget -> readiness 全流程，讓結果更容易比較、審閱與 cut over。

### 額外護欄

| 關注點 | 需要留意什麼 |
|------|----------------|
| 基線穩定性 | 保持 baseline、資料集版本和 `candidate_id` 穩定，否則對比會漂移，結果也不可信。 |
| 真實建置產物 | 在執行 Harness 前，先重新建置受影響的二進位或前端產物，否則你驗證的可能是舊行為，而不是目前的改動。 |
| 路由註冊 | 如果前後端一起改動，在透過 UI 行為判斷功能前，先確認所有新的後端路由都已正確註冊，因為路由沒註冊常常看起來像邏輯 bug，實際上只是 `404`。 |
| 發版判斷 | 只有當 Harness 沒有顯示出明顯回歸，且 cutover-readiness 確認候選版本確實可以切換時，這輪調優才算真正準備好。 |

簡而言之，在 Blue 之上做調優，不能只靠「幾次聊天感覺更好了」。你需要把候選版本放進 Harness，收集可比較的證據，再由 gate 和 readiness 的結果來決定這次改動是否真的安全可留。

## 功能特性

| 功能 | 提供能力 |
|--------------------|--------------------|
| 高可用 Web 擷取與瀏覽器運行時 | Blue **最鮮明的差異化能力**之一。它整合了用於搜尋、讀取、擷取與爬取的 **四條 Web 存取路徑**；在 HTTP、代理擷取與瀏覽器工作階段之間保留 **三層回退機制**；透過挑戰偵測、Cookie/工作階段重用、隱身與瀏覽器接管處理 **反機器人頁面**；並可在 **三種瀏覽器引擎** 之間路由：`lightpanda`、託管 Chromium，以及中繼/本地 Chrome。 |
| 三合一研究運行時 | **一個公開研究入口** 可路由到 `deep_research`、`analyze` 和 `ui_review`。同一套發現與證據棧隨後可產出 **引用優先的研究結果**、**邊界清晰的報告**，以及 **結構化的 UI/UX/無障礙評審**。 |
| Harness 運行時、評估與演化框架 | 讓評估成為貫穿開發、訓練與生產的 **運行時原語**。Harness 覆蓋 **回歸與 smoke 檢查**、評分、基線、報告與運行時驗證，並把同一份證據繼續用於 **技能演化**、後續評估、升級或回滾，以及 `AGENTS.md` 或指令提案審查。 |
| 多模態原生能力優先運行時 | 讓 **語音、OCR、PDF、瀏覽器任務、文件轉換、結構化表單填寫、媒體處理與本地媒體生成** 優先走 **原生與本地路徑**，僅在確有必要時才做 **模型路由**。 |
| 安全與治理 | 包含 **沙箱執行**、**提示注入防禦**、**工作階段審計**、權限、**RBAC**、**WebAuthn**、運行護欄，以及 **技能安全掃描**。 |
| LLM Wiki 與知識空間 | 將記憶、研究和運行時輸出整理成 **類 Wiki 的知識介面**，包含 **摘要頁**、索引、**反向連結**、**新鮮度** 與 **封存工作流**。 |
| 技能商店與市場 | 提供 **內建技能發現**、策展、同步與 **本地掃描**，讓擴充能力 **從第一天起就可用**。 |
| 生產級 Provider Pool | 提供具備 **健康檢查**、**自動故障切換**、**熔斷器** 與 **Provider 競速** 的真實 Provider 池，用於支撐長時運行工作負載。 |
| 內建本地小模型運行時 | 內建 **`Qwen3.5-0.8B` + `llama.cpp`** 運行時，用於 **本地短問答**、圖像辨識、工具路由、摘要、**上下文壓縮** 與 **文件預處理**。 |
| 長時運行可靠性 | 將 **OTA 更新**、**備份與還原**、**設定熱重載** 與 **故障後恢復** 視為 **內建運行能力**。 |

## 里程碑時間表

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

|日期 |版本 |關鍵字/特點|
|------|---------|---------------------|
| 2026 年 1 月 26 日 | `v0.1–v0.9` | Go 運行時間、插件系統、瀏覽器自動化 |
| 2026 年 1 月 27 日至 28 日 | `v0.9.0–v0.9.2` |瀏覽器任務視圖，Blue Companion，Smart Form Filler |
| 2026 年 1 月 29 日至 31 日 | `v0.10.0–v0.10.9` | Claude Code CLI、API Proxy、UI重構 |
| 2026 年 2 月 1 日至 3 日 | `v0.10.1–v0.10.22` |指標、遠端存取、上下文快取 |
| 2026 年 2 月 5 日至 18 日 | `v0.10.25–v0.10.29` | i18n、CC 快取、發布管道 |
| 2026 年 2 月 20 日至 25 日 | `v0.10.28–v0.10.29` |桌面載入程式、行動用戶體驗、記憶體重新設計 |
| 2026 年 2 月 28 日至 3 月 2 日 | `v0.10.30` | Deep Research、技能重新排序、安全掃描 |
| 2026 年 3 月 9 日至 18 日 | `v0.10.31` |儀表板大修、VoiceChat 重構、核准網站 |
| 2026 年 3 月 19 日至 22 日 | `v0.10.32` | Harness 推出、成績單審核、網路搜尋 |
| 2026 年 3 月 23 日至 25 日 | `v0.10.33` | Harness 群組、瀏覽器批准、技能市場 |
| 2026 年 3 月 29 日至 30 日 | `v0.10.35` | Harness v3，瀏覽器中繼，上下文壓縮 |
| 2026 年 3 月 31 日至 4 月 1 日 | `v0.10.36` |轉錄審核、Harness 覆蓋、工具解析 |
| 2026 年 4 月 1 日 | `v0.10.37` |運行時強化、Skill+Exec 切換、恢復拋光 |
| 2026 年 4 月 2 日至 5 日 | `v0.10.38` | GitHub 支援、市場完善、可靠性改善 |
| 2026 年 4 月 6 日至 7 日 | `v0.10.39` | 研究統一、演化表面、記憶體占用降低 |

## 社區與支持

- **問題**：[請在此提交錯誤和功能請求](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **討論**：[Discord](https://discord.gg/zwWbKA4S2)
- **在 [GitHub](https://github.com/IceWhaleTech) 上關注我們**

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## 許可證

該項目根據 MIT 許可證獲得許可 - 有關詳細信息，請參閱 [LICENSE](../../LICENSE) 文件。我們相信開源並回饋社區。

## 貢獻者

感謝所有Blue 貢獻者：

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## 參考文獻

1. **OpenClaw** — 本地優先的開源代理。率先透過通道適配器和工具呼叫將LLM連接到本機設備，直接啟發了Blue的代理程式執行時間架構。 https://github.com/openclaw/openclaw
2. **MiroMind** — 具有證據支持的綜合的深度研究模式。塑造Blue的內建深度研究管道：規劃、並行檢索、重複證據刪除和HTML報告生成。 https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM 作為知識編譯器。重新建構法學碩士以建立持久的、不斷發展的知識空間，超越 RAG 的累積陷阱。
4. **OpenSpace (HKUDS)** — 自我進化的技能引擎。基於 DAG 的框架，代理商可以從失敗中學習並獲得專業技能。 https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — 用於編碼代理程式的版本化 API 文件註冊表。解決座席幻覺和遺忘的會話知識。提供帶有註釋和回饋循環的精選、版本化文檔，將文檔轉變為自我改進的知識層。 https://github.com/andrewyng/context-hub
6. **Notion** — 簡單、人性化且有意保持安靜。受到Notion 極簡主義精神的啟發，Blue 為網格帶來了溫暖。精緻的襯線與貼心的設計相結合，打造出一個有家的感覺的空間。 https://www.notion.com/about
7. **Matrix** — 視覺靈感來自標誌性的數位雨美學。 Blue技術圖表的美學方向。
8. **IceWhale** — 愛、死亡與機器人 S2E2「冰」。一個聚集在世界各地的集體，旨在突破網路巨頭的圍牆，抵制資料集中。冰鯨象徵著一個在邊緣共同建構主權工具的社群。
9. **ZimaOS Blue** — 愛、死亡與機器人 S1E14「Zima Blue」。一個比喻：智能始於服務，並不斷發展以探索世界。 Blue 是智慧的代理人，根植於簡單，觸及深度。
10. **ZimaOS** — 簡化、專注、開放的設計原則。 ZimaOS 和Blue 都堅信科技應該為用戶服務——30 秒內部署、在任何地方運作、保持供應商中立。 https://www.zimaspace.com/zimaos
