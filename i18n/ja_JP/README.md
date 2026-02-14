# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>セキュア・可観測な AI エージェントランタイム</strong>
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** は NAS およびエッジデバイス向けの軽量・高性能 AI エージェントランタイムです。Go で構築され、ゼロ設定デプロイ、セッション監視、包括的な利用分析を備えた本番用プラットフォームを提供します。

[クイックスタート](#クイックスタート) · [機能](#コア機能)

## ハイライト

| 項目 | 値 |
|------|------|
| **バイナリサイズ** | ~40MB（単一実行ファイル） |
| **メモリ（アイドル）** | ~4MB |
| **起動時間** | < 1s |
| **依存関係** | なし（ゼロ設定デプロイ） |

## コア機能

### ゼロ設定デプロイ

- **単一バイナリ**：ダウンロードして実行、ランタイム依存不要
- **必要に応じて設定**：そのまま動作、必要時にカスタマイズ
- **クロスプラットフォーム**：Windows、macOS、Linux 同一バイナリ・同一体験
- **デーモン対応**：常駐バックグラウンドサービスとして実行可能

### セッション監視

- **リアルタイムセッション追跡**：全アクティブ AI セッションをライブ状態で監視
- **会話履歴**：全インタラクションの完全な監査証跡
- **セッションリプレイ**：過去の会話のレビューと分析
- **マルチテナント分離**：ユーザー間の完全なセッション分離

### コールチェーン最適化

- **リクエストトレース**：全 API コールのエンドツーエンド可視性
- **レイテンシ分析**：リクエストパイプラインのボトルネック特定
- **プロバイダールーティング**：最適 LLM プロバイダーへのインテリジェントルーティング
- **サーキットブレーカー**：プロバイダー障害時の自動フェイルオーバー

### 利用分析

- **トークン消費**：ユーザー・セッション・プロバイダー別の利用追跡
- **コスト帰属**：操作別の詳細コスト内訳
- **レート制限**：テナント別クォータ管理
- **レポートエクスポート**：複数形式で利用レポート生成

### セキュリティ強化

- **サンドボックス実行**：全ツール呼び出しを隔離環境で実行
- **RBAC**：きめ細かいロールベースアクセス制御
- **WebAuthn/Passkeys**：パスワードレス FIDO2 認証
- **MFA/TOTP**：多要素認証
- **監査証跡**：特権操作の不変ログ

## クイックスタート

```bash
# ソースから
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

ダッシュボードは `http://localhost:3000` でアクセスできます。

## LLM プロバイダー設定

ZimaOS Blue はローカル LLM サービスを含む複数の LLM プロバイダーをサポートします：

```yaml
llm:
  # クラウドプロバイダー
  provider: "openai"  # または "anthropic", "azure" など
  api_key: "your-api-key"

  # ローカル LLM（オプション）
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## アーキテクチャ

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
- プロバイダー別 LLM トークン使用量
- ツール実行成功/失敗率
- メモリ・goroutine 数
- サーキットブレーカー状態遷移

## 開発環境

### 前提条件

| ツール | バージョン | インストール |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | macOS/Linux にプリインストール |

### 開発モード（ホットリロード）

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- フロントエンド：`http://localhost:3000`
- バックエンド：`http://localhost:23456`

### ビルドコマンド

```bash
make build              # 単一バイナリビルド（フロントエンド埋め込み）
make build-embedded     # Claude Code CLI 埋め込みでビルド
make build-all          # 全プラットフォーム向けクロスコンパイル
make clean              # ビルド成果物のクリーン
```

### プロジェクト構造

```
ZimaOS-Blue/
├── server/             # Go バックエンド
│   ├── cmd/blue/       # エントリーポイント
│   └── internal/       # コアモジュール
├── web/                # Vue 3 フロントエンド
│   └── src/
└── dist/               # ビルド出力
```

## 謝辞

- [clawdbot](https://github.com/clawdbot/clawdbot) - プロジェクトのインスピレーション
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - 軽量 ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
