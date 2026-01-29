# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>安全、可觀測、本地優先的 AI 代理運行時</strong>
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** 是專為 NAS 和邊緣設備設計的強化 AI 代理運行時。您的資料保存在您的硬體上，每個操作都可審計，AI 操作在隔離的沙箱中運行。

[文件](https://echo.zimaos.com) · [快速開始](#快速開始) · [功能特性](#核心原則) · [對比](#與-clawdbot-對比)

## 為什麼選擇 ZimaOS Echo？

ZimaOS Echo 受 [clawdbot](https://github.com/clawdbot/clawdbot) 啟發，使用 Go 重建，具備：

- **更低資源佔用**：可在僅 256MB RAM 的裝置上運行
- **更好效能**：原生 Go 二進位檔，基於 goroutine 並行
- **更易部署**：單一二進位檔，無需 Node.js
- **NAS 優化**：為低功耗裝置 24/7 運行設計

## 核心原則

### 本地優先

- **資料主權**：所有資料儲存在您的 NAS 上 - 無雲端依賴
- **Ollama 整合**：完全在設備上運行 LLM，零外部 API 呼叫
- **離線能力**：核心功能無需網路連線即可運作
- **單一二進位檔**：約 15MB 原生 Go 二進位檔，無運行時依賴

### 可觀測與可審計

- **審計日誌**：每個 AI 操作都記錄完整上下文和時間戳
- **Prometheus 指標**：即時監控所有系統操作
- **pprof 效能分析**：深入了解 CPU、記憶體和 goroutine 行為
- **結構化日誌**：JSON 日誌便於解析和告警

### 安全強化

- **沙箱執行**：所有工具呼叫在隔離環境中運行
- **RBAC**：細粒度的角色存取控制
- **WebAuthn/Passkeys**：無密碼 FIDO2 認證
- **MFA/TOTP**：多因素認證支援
- **OIDC/OAuth 2.0**：企業 SSO 整合
- **熔斷器**：自動故障隔離防止級聯故障

## 快速開始

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# 從原始碼
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## 安全強化

### 認證堆疊

| 層級 | 技術 | 用途 |
|------|------|------|
| 主要 | WebAuthn/Passkeys | 防釣魚無密碼認證 |
| 次要 | TOTP/MFA | 基於時間的一次性密碼 |
| 企業 | OIDC/OAuth 2.0 | Google、GitHub、Okta SSO |
| 授權 | RBAC | 每資源權限控制 |

### 運行時保護

- **沙箱隔離**：工具在受限環境中執行
- **速率限制**：每租戶 API 節流
- **租戶隔離**：完整的資料和資源分離
- **審計追蹤**：所有特權操作的不可變日誌

### 韌性

- **熔斷器**：故障時自動服務隔離
- **優雅降級**：提供者失敗時的回退策略
- **LLM 回退鏈**：自動提供者故障轉移
- **熱重載**：無需重啟即可更改配置

## 可觀測性

```yaml
# 啟用完整可觀測性堆疊
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
- 每提供者 LLM 令牌使用量
- 工具執行成功/失敗率
- 記憶體和 goroutine 計數
- 熔斷器狀態轉換

## 架構

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

## 本地 LLM 設定（Ollama）

完全離線運行 AI，無外部 API 呼叫：

```bash
# 安裝 Ollama
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

## 開發設定

### 先決條件

| 工具 | 版本 | 安裝 |
|------|------|------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux 預裝 |

### 一鍵啟動

```bash
# 克隆並啟動所有服務
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

在 `http://localhost:3000` 存取儀表板。

### 開發模式（熱重載）

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- 前端：`http://localhost:5173`（API 代理到後端）
- 後端：`http://localhost:8080`

### 建置指令

```bash
make build              # 建置單一二進位檔（前端內嵌）
make build-embedded     # 建置並內嵌 Claude Code CLI
make build-all          # 全平台交叉編譯
make clean              # 清理建置產物
```

### 專案結構

```
ZimaOS-Echo/
├── server/             # Go 後端
│   ├── cmd/echo/       # 入口
│   └── internal/       # 核心模組
├── web/                # Vue 3 前端
│   └── src/
└── dist/               # 建置輸出
```

## 與 Clawdbot 對比

ZimaOS Echo 受 clawdbot 啟發，針對 NAS/邊緣部署優化：

| 項目 | ZimaOS Echo | Clawdbot |
|------|-------------|----------|
| **語言** | Go | TypeScript/Node.js |
| **二進位大小** | ~15MB | ~200MB+（含 node_modules）|
| **記憶體佔用** | ~80MB 閒置 | ~200MB+ 閒置 |
| **啟動時間** | < 1s | 3–5s |
| **運行時** | 原生二進位檔 | 需 Node.js |
| **目標平台** | NAS/邊緣裝置 | 桌面/伺服器 |

## 致謝

- [clawdbot](https://github.com/clawdbot/clawdbot) - 專案靈感來源
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 輕量 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
