# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>安全、可觀測的 AI 智能體執行環境</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <strong>繁體中文</strong> |
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

**ZimaOS Blue** 是專為 NAS 與邊緣裝置設計的輕量、高效能 AI 智能體執行環境。以 Go 建構，提供零設定部署、會話監控與完整使用分析的生產就緒平台。

[快速開始](#快速開始) · [功能](#核心功能)

## 亮點

| 規格 | 數值 |
|------|------|
| **二進位大小** | ~40MB（單一執行檔） |
| **記憶體（閒置）** | ~4MB |
| **啟動時間** | < 1s |
| **依賴** | 無（零設定部署） |

## 核心功能

### 零設定部署

- **單一二進位**：下載即用，無需執行時依賴
- **按需設定**：開箱可用，需要時再自訂
- **跨平台**：Windows、macOS、Linux 同一二進位、同一體驗
- **常駐程式**：可作為背景服務持續執行

### 會話監控

- **即時會話追蹤**：監控所有活躍 AI 會話與即時狀態
- **對話歷史**：完整互動審計記錄
- **會話重播**：檢視與分析過往對話
- **多租戶隔離**：使用者間會話完全分離

### 呼叫鏈優化

- **請求追蹤**：每次 API 呼叫的端到端可見性
- **延遲分析**：找出請求管線中的瓶頸
- **提供商路由**：智慧路由至最佳 LLM 提供商
- **熔斷器**：提供商故障時自動切換

### 使用分析

- **Token 消耗**：按使用者、會話、提供商統計使用量
- **成本歸屬**：依操作詳細成本拆分
- **限流**：按租戶的配額管理
- **匯出報告**：多種格式產生使用報告

### 安全加固

- **沙箱執行**：所有工具呼叫在隔離環境中執行
- **RBAC**：細粒度角色型存取控制
- **WebAuthn/Passkeys**：無密碼 FIDO2 認證
- **MFA/TOTP**：多因素認證
- **審計鏈**：所有特權操作不可變日誌

## 快速開始

```bash
# 從原始碼
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

於 `http://localhost:3000` 存取儀表板。

## LLM 提供商設定

ZimaOS Blue 支援多種 LLM 提供商，包含本地 LLM 服務：

```yaml
llm:
  # 雲端提供商
  provider: "openai"  # 或 "anthropic", "azure" 等
  api_key: "your-api-key"

  # 本地 LLM（可選）
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## 架構

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

## 可觀測性

```yaml
# 啟用完整可觀測性棧
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

### 暴露的指標

- 請求延遲（p50、p95、p99）
- 各提供商 LLM 的 token 使用量
- 工具執行成功/失敗率
- 記憶體與 goroutine 計數
- 熔斷器狀態轉換

## 開發環境

### 前置條件

| 工具 | 版本 | 安裝 |
|------|------|------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux 通常已預裝 |

### 開發模式（熱重載）

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- 前端：`http://localhost:3000`
- 後端：`http://localhost:23456`

### 建置指令

```bash
make build              # 建置單一二進位（前端內嵌）
make build-embedded     # 建置並內嵌 Claude Code CLI
make build-all          # 全平台交叉編譯
make clean              # 清理建置產物
```

### 專案結構

```
ZimaOS-Blue/
├── server/             # Go 後端
│   ├── cmd/blue/       # 進入點
│   └── internal/       # 核心模組
├── web/                # Vue 3 前端
│   └── src/
└── dist/               # 建置輸出
```

## 致謝

- [clawdbot](https://github.com/clawdbot/clawdbot) - 專案靈感來源
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 輕量級 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
