![](../../docs/assets/bannerX.png)

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
  <strong>日本語</strong> |
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
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
</p>

## はじめに

Clawdbotに触発され、私たちはパーソナルコンピューティングの**未来**は、エッジで動作する**多様なローカルファーストAIエージェント**によって**形作られる**と信じています。

**ZimaOS Blueはその答えです** — 完全に**オープンソースで、監査可能、かつ本番環境対応のエージェントランタイムおよびツールキット**であり、プライベートなセルフホスト型エージェントをゼロフリクションで提供できます。

大胆な開発者が**自分だけのエージェントをバイブコーディングまたは手作り**するために構築されたBlueは、**パフォーマンスを追求した設計**です：**Go**で記述され、メモリフットプリントはわずか10 MB。**あらゆるx86、Raspberry Pi、Windows、macOS**で動作します — 電源さえあればどこでも。

![](../../docs/assets/features.png)

## ハイライト

### ローカルファースト設計と自動モデルアクセス

さらに一歩先へ：**20以上のIMプラットフォーム**のネイティブサポート、自然で文脈を理解した対話のための**音声駆動**インターフェース、IDEスキャンによる**ゼロコンフィグモデル切り替え**、SOULレイヤードパーソナリティを提供します。

### 高速・軽量

Goでネイティブコンパイル — インタプリタなし、VMなし、オーバーヘッドなし。サーバーからデスクトップデバイスまで、あらゆる環境で静かに動作します。

| 指標 | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|--------|-------------------|------------------------|
| `--help` コールド / ウォーム | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` 実行時間 (3回中最速) | **< 0.01 s** | 5.98 s |
| `--help` ピークRSS | **~10 MB** | ~394 MB |
| `status` ピークRSS | **~15 MB** | ~1.52 GB |
| ランタイム依存関係 | **なし** | Node.js 18+ |

> macOS arm64、同一ホスト、3回中最速で計測。2026年2月。

### 純粋なGo、あらゆるデバイス

100% Go、静的バイナリ。**5つのターゲットにクロスコンパイル**可能（![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64）。Nodeランタイム不要、Python不要、コンテナ不要。NAS、Raspberry Pi、古いx86ルーター、Macに置くだけ — そのまま動きます。**その上に独自のUI、ロジック、エージェントスキルを重ねましょう** — 1つのコードベースで、すべてのプラットフォームに対応。

### セキュリティとガバナンス

多層防御を備えた組み込みサイドカーAPIプロキシ：
- **サンドボックス実行** – すべてのツール呼び出しは隔離された環境で実行されます。
- **プロンプトインジェクション防御** – 7つ以上の組み込みインターセプト戦略。
- **セッション監査** – 完全なセッション監視、すべてのインタラクションが追跡可能。
- **RBAC & WebAuthn** – パスワードレス認証によるきめ細かなアクセス制御。

## なぜBlueなのか

私たちは**次世代パーソナルコンピューティング**がLLMを受け入れると信じています — しかし**制御可能で監査可能な**エージェントこそが、個人にもチームにも基盤であり続けます。**Blueが提供するもの**：
- **包括的なコア** – 高度なモデル管理、IM統合、強化されたペルソナ、日常的なインタラクション（ヘッドセット、音声、スマートグラス）に最適化された自然言語インターフェース。
- **ローカルファースト、超軽量、クロスデバイス** – ハイエンドハードウェア不要。計算能力があるものなら何でも動作します。
- **安全で監査可能** – セッション監査、サンドボックス、権限制御、アプリケーション層ファイアウォールとして機能する組み込みAPIプロキシ — すべての入出力バイトが可視化されます。

ボイラープレートを最小限に抑え、**本当に重要なことに集中**できます。**ZimaOSの設計哲学**に忠実に、Blueは以下を提供します：
- **ワンクリックでゼロからイチへ** – 複雑な設定なしで即座にデプロイ。
- **ラピッドプロトタイピング** – シナリオ固有のツール、インタラクション、アプリパッケージをバイブコーディングまたは手作り。
- **グローバル対応** – **世界は広い**、そして英語がデフォルトではありません。**20以上の言語をネイティブサポート**、障壁なし。
- **オープンモデルエコシステム** – ベンダーロックインなし。お好みのモデルをお使いください。

![](../../docs/assets/design_principle.png)

## クイックスタート

### オプション1：デスクトップアプリをダウンロード（macOS & Windows）

ネイティブアプリケーションを入手 — 依存関係なし、コンパイル不要。

- **macOS**: [DMGをダウンロード](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [インストーラーをダウンロード](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### オプション2：インストールスクリプト

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### オプション3：ソースからビルド

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
./dev.sh
```

## アーキテクチャ概要

![](../../docs/assets/architecture.png)

### データフロー

**チャットリクエスト（プロキシホットパス）**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**チャネルメッセージフロー**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**音声パイプライン**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### パッケージマップ (`server/internal/`)

| レイヤー | パッケージ |
|-------|----------|
| ゲートウェイ | bootstrap, server, gateway |
| プロキシ | proxy, connection, streaming, resilience |
| プロバイダー | providerpool, providers, llm |
| プルーナー | pruner (detector, segmenter, bm25, pipeline, cache) |
| エージェント | context, tools, personality, humanizer |
| メモリ | memory, embedding, kvstore |
| チャネル | channel, autoreply, i18n |
| セキュリティ | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| 音声 | voice, tts, stt, speech |
| 監視 | metrics, heartbeat, companion, profiling, leakdetect |
| プラグイン | plugin, skill, skillstore |
| 統合 | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| スケジューラー | scheduler, worker, workerpool, pool |
| コア | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| システム | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| マルチテナント | tenant, user, session, preview |

## 使い方

![](../../docs/assets/handcraft.png)

## マイルストーンタイムライン

![](../../docs/assets/timeline.png)

| バージョン | フォーカス | 主な価値 | ステータス |
|---------|-------|-----------|--------|
| v0.1 | Goランタイムコア | 安定したカーネル、24時間稼働 | Done |
| v0.2 | コア機能 | 最小限の使用可能状態、LLM統合 | Done |
| v0.3 | NAS統合 | NASネイティブ、systemdサポート | Done |
| v0.4 | プラグインシステム | 拡張可能、セキュリティ基盤 | Done |
| v0.5 | 製品ベースライン | 本番環境対応、ドキュメント | Done |
| v0.6 | メッセージチャネル | マルチチャネルサポート | Done |
| v0.7 | セキュリティ | OIDC、MFA、監査 | Done |
| v0.8 | パフォーマンス | 最適化、キャッシュ、ベンチマーク | Done |
| v0.9 | エコシステム | マルチテナント、ブラウザ自動化、音声 | Done |
| v0.10.0 | CLIバンドル | CC CLIバンドル、検出、自動更新 | Done |
| v0.10.1 | メトリクス監視 | API統計、トークン追跡、TTFT | Done |
| v0.10.2 | CLI信頼性 | プロセスライフサイクル、エラー回復 | Done |
| v0.10.3 | CLI統合 | セットアップウィザード、プロバイダー自動検出 | Done |
| v0.10.4 | Tauriパッケージング | デスクトップアプリ、システムトレイ | Done |
| v0.10.5 | APIプロキシサイドカー | ルート選択、プロンプトガード、使用統計 | Done |
| v0.10.6 | プロバイダープール | マルチプロバイダールーティング、ヘルスチェック、フェイルオーバー | Done |
| v0.10.7 | プレビューモード | 未認証アクセス、機能ゲーティング | Done |
| v0.10.8 | スキルストア | スキルストア基盤、チャネルバリデーション | Done |
| v0.10.9–10 | ユーザー管理 | サブユーザー、ページレベル権限 | Done |
| v0.10.13–14 | セキュリティ & スキル | セキュリティページ、スキルストア再設計 | Done |
| v0.10.15 | チャット強化 | チャットUX、メッセージパイプライン | Done |
| v0.10.16 | 音声モジュール | Sherpa TTS/ASR、eSpeak、プロバイダー切り替え | Done |
| v0.10.17 | リモートアクセス | Ngrok、Cloudflareトンネル、ACME証明書 | Done |
| v0.10.18–20 | パフォーマンススプリント | 起動/チャット性能、コンテキストキャッシュ | Done |
| v0.10.21–22 | プロンプト & DingTalk | システムプロンプト、DingTalkチャネル | Done |
| v0.10.23 | OTAアップデート | OTAアップデートシステム | Done |
| v0.10.24 | チャネルアップグレード | 10チャネルをスタブからアップグレード | Done |
| v0.10.25 | CCキャッシュ | 2層キャッシュ（L1メモリ + L2ディスク） | Done |
| v0.10.26 | ヒューマナイザー | レスポンスヒューマナイゼーションパイプライン | Done |
| v0.10.27 | コンテキストプルーナー | BM25スコアリング、セグメンテーション、ベンチマーク | Done |
| v0.10.28 | メモリサービス | プログレッシブ検索、デュアルライトバックエンド | Done |

## コミュニティ & サポート

- **Issues**: [バグ報告や機能リクエストはこちら](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **ディスカッション**: [Discord](https://discord.gg/SrCYvumF)
- **フォロー**: [GitHub](https://github.com/IceWhaleTech)

## ライセンス

このプロジェクトはMITライセンスの下で公開されています — 詳細は[LICENSE](../../LICENSE)ファイルをご覧ください。私たちはオープンソースとコミュニティへの貢献を信じています。

## コントリビューター

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
