![](../../docs/assets/banner.png)

<p align="center">
  大胆なビルダーのための<strong>ローカルファースト</strong>エージェントランタイム<br>
  すぐに使える · オープンソース · ユニバーサル · デュアル監視
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
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## はじめに

Clawdbotに触発され、私たちはパーソナルコンピューティングの**未来**は、エッジで動作する**多様なローカルファーストAIエージェント**によって**形作られる**と信じています。

**ZimaOS Blueはその答えです** — 完全に**オープンソースで、監査可能、かつ本番環境対応のエージェントランタイムおよびツールキット**であり、プライベートなセルフホスト型エージェントをゼロフリクションで提供できます。

大胆な開発者が**自分だけのエージェントをバイブコーディングまたは手作り**するために構築されたBlueは、**パフォーマンスを追求した設計**です：**Go**で記述され、メモリフットプリントはわずか10 MB。**あらゆるx86、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、Windows、macOS**で動作します — 電源さえあればどこでも。

![](../../docs/assets/features.png)

## ハイライト

### ローカルファースト設計と自動モデルアクセス

さらに一歩先へ：**20以上のIMプラットフォーム**のネイティブサポート、自然で文脈を理解した対話のための**音声駆動**インターフェース、IDEスキャンによる**ゼロコンフィグモデル切り替え**、SOULレイヤードパーソナリティを提供します。

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### 高速・軽量

Goでネイティブコンパイル — インタプリタなし、VMなし、オーバーヘッドなし。サーバーからデスクトップデバイスまで、あらゆる環境で静かに動作します。

| 指標 | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|--------|-------------------|------------------------|
| `--help` コールド / ウォーム | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` 実行時間 (3回中最速) | **< 0.01 s** | 5.98 s |
| `--help` ピークRSS | **~10 MB** | ~394 MB |
| `status` ピークRSS | **~15 MB** | ~1.52 GB |
| ランタイム依存関係 | **なし** | Node.js 18+ |

> ベンチマーク環境：macOS arm64（サーバーモード、デスクトップUIなし）、同一ホスト、3回中最速で計測。2026年2月。

### 純粋なGo、あらゆるデバイス

100% Go、静的バイナリ。**5つのターゲットにクロスコンパイル**可能（![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64、![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64、![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64）。Nodeランタイム不要、Python不要、コンテナ不要。NAS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、古いx86ルーター、Macに置くだけ — そのまま動きます。**その上に独自のUI、ロジック、エージェントスキルを重ねましょう** — 1つのコードベースで、すべてのプラットフォームに対応。

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

![](../../docs/assets/design_principle.png)

ボイラープレートを最小限に抑え、**本当に重要なことに集中**できます。<a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **ZimaOSの設計哲学**に忠実に、Blueは以下を提供します：
- **ワンクリックでゼロからイチへ** – 複雑な設定なしで即座にデプロイ。
- **ラピッドプロトタイピング** – シナリオ固有のツール、インタラクション、アプリパッケージをバイブコーディングまたは手作り。
- **グローバル対応** – **世界は広い**、そして英語がデフォルトではありません。**20以上の言語をネイティブサポート**、障壁なし。
- **オープンモデルエコシステム** – ベンダーロックインなし。お好みのモデルをお使いください。

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| プロバイダー | モデル | タイプ |
|----------|--------|------|
| OpenAI | GPT-4o, GPT-4, o1, o3 | クラウド |
| Anthropic | Claude 4.5, Claude 4 | クラウド |
| Google | Gemini 2.5, Gemini 2.0 | クラウド |
| Ollama | Llama, Qwen, Gemma, Phi 等 | ローカル |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | クラウド |
| Grok | Grok-3, Grok-3-mini | クラウド |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | クラウド |
| GLM | GLM-4, GLM-4-Flash | クラウド |
| Moonshot | Moonshot-v1 | クラウド |
| MiniMax | abab6.5, abab5.5 | クラウド |
| Venice | Llama, Mistral（プライバシー優先） | クラウド |
| AWS Bedrock | Claude, Llama, Titan | クラウド |
| Azure | Azure経由のOpenAIモデル | クラウド |
| OpenRouter | 100+ 集約モデル | クラウド |
| AIHubMix | マルチプロバイダーアグリゲーター | クラウド |
| Codex | OpenAI Codex | クラウド |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | クラウド |
| カスタム | 任意のOpenAI / Anthropic / Gemini互換API | クラウド / ローカル |

</details>

### サポートされるIDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## クイックスタート

### オプション1：デスクトップアプリをダウンロード

ネイティブアプリケーションを入手 — 依存関係なし、コンパイル不要。内蔵トライアル設定で数秒でオンボーディング — リモート接続で即座にチャット開始、ボット設定不要。真のすぐ使える体験。

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMGをダウンロード](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [インストーラーをダウンロード](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### オプション2：インストールスクリプト

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### オプション3：ソースからビルド

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

> **注意:** Windowsでのビルドには、ネイティブC依存関係（espeak-ng、whisper.cpp、opus）のために[MinGW-w64](https://www.mingw-w64.org/)（gcc）と[CMake](https://cmake.org/)が必要です。`gcc`と`cmake`が`PATH`に含まれていることを確認してください。

## アーキテクチャ概要

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

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

</details>

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
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**音声パイプライン**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

## 使い方

![](../../docs/assets/handcraft.png)

## マイルストーンタイムライン

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

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
| v0.10.27 | コンテキストプルーナー | コードで54%トークン削減（SWE-bench公式）、一般文書で46–47%（ローカルIR）、BM25スコアリング、セグメンテーション | Done |
| v0.10.28 | メモリサービス | プログレッシブ検索、デュアルライトバックエンド | Done |

</details>

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
