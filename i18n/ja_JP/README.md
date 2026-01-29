# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>セキュア・可観測・ローカルファーストの AI エージェントランタイム</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <strong>日本語</strong> |
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

**ZimaOS Echo** は NAS およびエッジ向けの堅牢な AI エージェントランタイムです。データは自前ハードウェアに保存され、操作はすべて監査可能、AI は隔離サンドボックスで動作します。

[ドキュメント](https://echo.zimaos.com) · [クイックスタート](#クイックスタート) · [機能](#コア原則) · [比較](#clawdbot-との比較)

## なぜ ZimaOS Echo？

ZimaOS Echo は [clawdbot](https://github.com/clawdbot/clawdbot) に触発され、Go で一から構築：

- **低リソース**: 256MB RAM のデバイスで動作
- **高性能**: ネイティブ Go バイナリ、goroutine 並行
- **簡単デプロイ**: 単一バイナリ、Node.js 不要
- **NAS 最適化**: 低消費電力デバイス 24/7 運転向け

## コア原則

### ローカルファースト

- **データ主権**: 全データは NAS にローカル保存、クラウド非依存
- **Ollama 連携**: LLM を完全オンデバイスで実行、外部 API 不要
- **オフライン対応**: コア機能はインターネット不要
- **単一バイナリ**: 約 15MB のネイティブ Go バイナリ、ランタイム依存なし

### 可観測性と監査

- **監査ログ**: 全 AI 操作をコンテキスト・タイムスタンプ付きで記録
- **Prometheus メトリクス**: システム操作のリアルタイム監視
- **pprof プロファイリング**: CPU・メモリ・goroutine の詳細可視化
- **構造化ログ**: JSON ログで解析・アラートが容易

### セキュリティ強化

- **サンドボックス実行**: 全ツール呼び出しを隔離環境で実行
- **RBAC**: きめ細かいロールベースアクセス制御
- **WebAuthn/Passkeys**: パスワードレス FIDO2 認証
- **MFA/TOTP**: 多要素認証
- **OIDC/OAuth 2.0**: エンタープライズ SSO 連携
- **サーキットブレーカー**: 自動障害隔離で連鎖故障を防止

## クイックスタート

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# ソースから
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## セキュリティ強化

### 認証スタック

| 層 | 技術 | 用途 |
|----|------|------|
| プライマリ | WebAuthn/Passkeys | フィッシング耐性のパスワードレス認証 |
| セカンダリ | TOTP/MFA | 時間ベースワンタイムパスワード |
| エンタープライズ | OIDC/OAuth 2.0 | Google、GitHub、Okta 等との SSO |
| 認可 | RBAC | リソース単位の権限制御 |

### ランタイム保護

- **サンドボックス隔離**: ツールは制限環境で実行
- **レート制限**: テナントごとの API スロットリング
- **テナント分離**: データ・リソースの完全分離
- **監査トレイル**: 特権操作の不変ログ

### レジリエンス

- **サーキットブレーカー**: 障害時のサービス自動隔離
- **Graceful Degradation**: プロバイダ障害時のフォールバック
- **LLM フォールバックチェーン**: プロバイダの自動切替
- **ホットリロード**: 設定変更を再起動なしで反映

## 可観測性

```yaml
# フル可観測性スタックを有効化
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

### 公開メトリクス

- リクエストレイテンシ（p50、p95、p99）
- プロバイダ別 LLM トークン使用量
- ツール実行成功率・失敗率
- メモリ・goroutine 数
- サーキットブレーカー状態遷移

## アーキテクチャ

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

## ローカル LLM セットアップ（Ollama）

外部 API なしで完全オフライン AI 実行：

```bash
# Ollama インストール
curl -fsSL https://ollama.com/install.sh | sh

# モデル取得
ollama pull llama3.2

# Echo でローカル LLM を使用するよう設定
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## 開発環境

### 前提

| ツール | バージョン | インストール |
|--------|------------|--------------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux に標準同梱 |

### ワンコマンド起動

```bash
# クローンして起動
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

ダッシュボードは `http://localhost:3000` でアクセス。

### 開発モード（ホットリロード）

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- フロントエンド: `http://localhost:5173`（API はバックエンドにプロキシ）
- バックエンド: `http://localhost:8080`

### ビルドコマンド

```bash
make build              # 単一バイナリビルド（フロントエンド埋め込み）
make build-embedded     # Claude Code CLI 埋め込みビルド
make build-all          # 全プラットフォーム向けクロスビルド
make clean              # ビルド成果物削除
```

### プロジェクト構造

```
ZimaOS-Echo/
├── server/             # Go バックエンド
│   ├── cmd/echo/       # エントリポイント
│   └── internal/       # コアモジュール
├── web/                # Vue 3 フロントエンド
│   └── src/
└── dist/               # ビルド出力
```

## Clawdbot との比較

ZimaOS Echo は clawdbot に触発され、NAS/エッジ向けに最適化：

| 項目 | ZimaOS Echo | Clawdbot |
|------|-------------|----------|
| **言語** | Go | TypeScript/Node.js |
| **バイナリサイズ** | ~15MB | ~200MB+（node_modules 含む）|
| **メモリ** | ~80MB アイドル | ~200MB+ アイドル |
| **起動時間** | < 1s | 3–5s |
| **ランタイム** | ネイティブバイナリ | Node.js 必須 |
| **対象** | NAS/エッジ | デスクトップ/サーバー |

## 謝辞

- [clawdbot](https://github.com/clawdbot/clawdbot) - プロジェクトのインスピレーション
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 軽量 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
