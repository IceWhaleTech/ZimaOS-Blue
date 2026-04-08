![](../../docs/assets/banner.png)

<h2 align="center">ZimaOS Blue: 大胆なビルダーのためのローカルファーストエージェントランタイム</h2>

<p align="center"><strong>すぐに使える · オープンソース · ユニバーサル · ベンダーニュートラル</strong></p>

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
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## はじめに

OpenClaw からインスピレーションを受け、パーソナル コンピューティングの未来は、エッジで実行されるローカルファーストの多様な AI エージェントによって形作られると私たちは考えています。

ZimaOS Blue が私たちの答えです。完全にオープンソースで監査可能、ベンダー中立、本番環境に対応したエージェント ランタイムとツールキットで、プライベートの自己ホスト型エージェントをスムーズに配布できます。

Blue は、独自のエージェントを動かしたり手作りしたりしたい大胆な開発者向けに構築されており、パフォーマンスを重視して設計されています。Go で書かれており、メモリ使用量は 19 MB と低いです。 x86、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS など、電源を接続すればどこでも動作します。

## デモ

### 会話とタスク実行

Blue における会話フローとタスク実行を手早く紹介するデモです。

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### LLM プロバイダー統合

Blue の LLM プロバイダー統合体験を手早く紹介するデモです。

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### クイック概要 - 概要、チャネル、追加設定

製品全体の概要、チャネル、追加設定を手早く紹介するデモです。

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## なぜ Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go、どんなデバイスでも

100% Go、静的バイナリ。すぐに使用できる 5 つのターゲット (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`、`linux/arm64`、![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`、`darwin/arm64`、![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`) にクロスコンパイルします。 Node ランタイム、Python、コンテナーは必要ありません。 NAS、<a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS、![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi、古い x86 ルーター、または ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac にドロップするだけで実行できます。次に、独自の UI、ロジック、エージェント スキルを 1 つのコードベース、すべてのプラットフォームに重ねていきます。

### 箱から出してすぐに使える

誰もが、シンプルで信頼性が高く、必要なときに拡張できるツールを望んでいます。機能するツールなので、実際に構築しているものに集中できます。

これは新しい哲学ではありません。 <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS を構築したものと同じです。シンプルで信頼性が高く、邪魔にならないように構築されています。 Blue はその哲学であり、エージェント スタックにも拡張されています。

### あなたの生活に合わせてデザインされ、地元に留まるように作られています

完全な HTML レポートを提供する詳細な調査から、OCR、PDF、ブラウザの自動化、ドキュメント変換まで、Blue はデータをクラウドに送信せずに、複雑な現実世界のワークフローを処理します。音声ウェイク、STT/TTS、Talk Mode、およびローカル推論のサポートにより、日常の対話が瞬時に行われ、プライベートになり、いつでも利用できるようになります。

## クイックスタート

### オプション 1: デスクトップ アプリをダウンロードする

ネイティブ アプリケーションを入手します。依存関係やコンパイルは必要ありません。数秒でオンボーディングできる組み込みのトライアル構成 — ボットのセットアップは必要なく、リモート接続経由で即座にチャットを開始できます。すぐに使える真の体験。

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [ZimaOS で実行](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [DMGをダウンロード](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [インストーラーをダウンロード](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### オプション 2: スクリプトのインストール

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### オプション 3: ソースからビルドする

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

> **注:** Windows ビルドには以下が必要です。
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) および [CMake](https://cmake.org/) ネイティブ C 依存関係 (espeak-ng、whisper.cpp、opus、kokoro、onnx) 用
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) システム ライブラリ (winmm など)
>
> `gcc`、`cmake` が `PATH` に含まれていることを確認してください。

## アーキテクチャの概要

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

さらに言えば、**20 以上の IM プラットフォーム** のネイティブ サポート、自然でコンテキストを意識した対話のための **音声駆動** インターフェイス、IDE スキャンによる **ゼロ設定モデルの切り替え** を提供します。

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## 構築方法

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Blue に基づいてチューニングやバイブ コーディングを継続する予定がある場合は、いくつかの見栄えの良いチャットをリリースの証拠として扱わないでください。ルーティング、実行動作、ツール サーフェス、予算管理、モデル選択、または実行フレームワークに影響を与える変更は、その場限りのスポット チェックではなく、Blue Harness で検証する必要があります。
>
> Blue は、ここでは 1 つの単純なルールに従う必要があります。つまり、データが最初、ゲートが最初、カットオーバーが最後です。実際には、これは、変更を判断する前に関連する Harness データセット / 評価仕様を更新し、試行全体にわたって 1 つの安定した `candidate_id` を維持することを意味します。これにより、セレクター、実行、予算、準備状況のレポートはすべて、無関係な 4 つの実行ではなく、同じ候補を記述します。

### 推奨される Harness ワークフロー

1. `blue harness selector verify` を実行します。
2. `blue harness execution verify` を実行します。
3. `blue harness budget gate` のセレクター評価実行を再利用します。
4. `blue harness cutover-readiness` で終了します

ローカルでの反復、夜間の検証、または CI 証拠収集の場合は、`python3 scripts/cutover_candidate_pipeline.py` を優先します。 1 つの共有候補の下で完全なセレクター -> 実行 -> 予算 -> 準備完了シーケンスを実行するため、結果の比較、レビュー、カットオーバーが容易になります。

### 追加のガードレール

|エリア |何を見るべきか |
|------|----------------|
|ベースラインの安定性 |ベースライン、データセットのバージョン、`candidate_id` を安定したままにしておかないと、比較がずれて結果が信頼できなくなります。 |
|実際のビルド出力 | Harness を実行する前に、影響を受けるバイナリまたはフロントエンド バンドルを再構築してください。そうしないと、現在の変更ではなく古い動作を検証することになる可能性があります。 |
|ルート登録 |フロントエンドとバックエンドが一緒に変更される場合は、UI 動作を通じて機能を判断する前に、すべての新しいバックエンド ルートが実際に登録されていることを確認してください。登録の欠落はロジックのバグのように見えることがよくありますが、実際には `404` であるためです。 |
|リリース判定 |調整パスは、Harness が意味のある回帰を示さず、カットオーバー準備状況で候補が実際にカットオーバーの準備ができていることを確認した場合にのみ準備が整っています。 |

つまり、Blue をベースにしたチューニングは、「数回のチャットで気分が良くなる」ということではありません。候補者を Harness に投入し、比較可能な証拠を収集し、ゲートと準備の結果に基づいて変更を保持しても本当に安全かどうかを判断させることが重要です。

## 特徴

|特集 |提供するもの |
|----------|--------|
|高可用性 Web 検索とブラウザ ランタイム | Blue の **最も鋭い差別化要因** の 1 つ。 Blue は、検索、読み取り、抽出、クロールのための **4 つの Web アクセス パス**を統合します。 HTTP、プロキシ抽出、ブラウザ セッション全体で **3 つのフォールバック層**を維持します。チャレンジ検出、Cookie/セッションの再利用、ステルス、ブラウザーハンドオフを使用して**アンチボットページ**を処理します。 **3 つのブラウザ エンジン** (`lightpanda`、マネージド Chromium、リレー/ローカル Chrome) 間でルーティングします。 |
| 3 つの機能を 1 つにまとめたリサーチ ランタイム | **1 つの公開研究エントリ** は、`deep_research`、`analyze`、および `ui_review` にルーティングできます。同じ発見と証拠の積み重ねから、**引用優先調査**、**制限付きレポート**、**構造化された UI/UX/アクセシビリティ レビュー**が生成されます。 |
| Harness ランタイム、評価、進化フレームワーク |開発、トレーニング、運用全体にわたる評価を**実行時のプリミティブ**にします。 Harness は、**回帰チェックとスモーク チェック**、スコアリング、ベースライン、レポート、実行時検証をカバーし、同じ証拠を **スキル進化**、フォローアップ評価、昇進またはロールバック、`AGENTS.md` または指示提案のレビューに取り入れます。 |
|マルチモーダル ネイティブ機能ファースト ランタイム | **音声、OCR、PDF、ブラウザ タスク、ドキュメント変換、構造化フォーム入力、メディア処理、およびローカル メディア生成**を **最初にネイティブおよびローカル パス**で維持し、**実際に必要な場合にのみモデル ルーティング**を行います。 |
|セキュリティとガバナンス | **サンドボックス実行**、**プロンプトインジェクション防御**、**セッション監査**、権限、**RBAC**、**WebAuthn**、運用上のガードレール、**スキル セキュリティ スキャン**が含まれます。 |
| LLM Wiki とナレッジ スペース |メモリ、リサーチ、およびランタイムの出力を、**概要ページ**、インデックス、**バックリンク**、**鮮度**、**アーカイブ ワークフロー**を備えた **Wiki のようなナレッジ サーフェス**に変換します。 |
|スキルストアとマーケットプレイス | **組み込みのスキル検出**、キュレーション、同期、**ローカル スキャン**が搭載されているため、**初日から**拡張性を利用できます。 |
|実稼働グレードのプロバイダー プール |長時間実行されるワークロード向けに、**ヘルスチェック**、**自動フェイルオーバー**、**サーキットブレーカー**、**プロバイダーレーシング**を備えた実際のプロバイダープールを提供します。 |
|組み込みのローカル小規模モデル ランタイム | **ローカルの短い Q&A**、画像認識、ツール ルーティング、要約、**コンテキスト圧縮**、**ドキュメント前処理**用の組み込み **`Qwen3.5-0.8B` + `llama.cpp`** ランタイムを同梱します。 |
|長期にわたる信頼性 | **OTA 更新**、**バックアップと復元**、**構成ホット リロード**、**障害後の回復**を **組み込みの運用上の懸念事項**として扱います。 |

## マイルストーンのタイムライン

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

|日付 |バージョン |キーワード・特徴 |
|-----|----------|----------|
| 2026 年 1 月 26 日 | `v0.1–v0.9` | Go ランタイム、プラグイン システム、ブラウザ自動化 |
| 2026 年 1 月 27 ～ 28 日 | `v0.9.0–v0.9.2` |ブラウザタスクビュー、Blue Companion、Smart Form Filler |
| 2026 年 1 月 29 ～ 31 日 | `v0.10.0–v0.10.9` | Claude Code CLI、API Proxy、UI の再構築 |
| 2026 年 2 月 1 ～ 3 日 | `v0.10.1–v0.10.22` |メトリクス、リモート アクセス、コンテキスト キャッシュ |
| 2026 年 2 月 5 ～ 18 日 | `v0.10.25–v0.10.29` | i18n、CC キャッシュ、リリース パイプライン |
| 2026 年 2 月 20 ～ 25 日 | `v0.10.28–v0.10.29` |デスクトップローダー、モバイルUX、メモリの再設計 |
| 2026 年 2 月 28 日～3 月 2 日 | `v0.10.30` | Deep Research、スキル リランカー、セキュリティ スキャン |
| 2026 年 3 月 9 ～ 18 日 | `v0.10.31` |ダッシュボードのオーバーホール、VoiceChat リファクタリング、承認済みサイト |
| 2026 年 3 月 19 ～ 22 日 | `v0.10.32` | Harness ロールアウト、トランスクリプト監査、Web 検索 |
| 2026 年 3 月 23 ～ 25 日 | `v0.10.33` | Harness グループ、ブラウザ承認、スキル マーケット |
| 2026 年 3 月 29 ～ 30 日 | `v0.10.35` | Harness v3、ブラウザリレー、コンテキスト圧縮 |
| 2026 年 3 月 31 日～4 月 1 日 | `v0.10.36` |トランスクリプト監査、Harness オーバーレイ、ツール解析 |
| 2026 年 4 月 1 日 | `v0.10.37` |ランタイム強化、Skill+Exec カットオーバー、リカバリ磨き |
| 2026 年 4 月 2 ～ 5 日 | `v0.10.38` | GitHub サポート、マーケットプレイスの改善、信頼性の向上 |
| 2026 年 4 月 6 ～ 7 日 | `v0.10.39` | 研究の統合、進化サーフェス、メモリ使用量の削減 |

## コミュニティとサポート

- **問題**: [バグや機能リクエストはここに提出してください](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **ディスカッション**: [Discord](https://discord.gg/zwWbKA4S2)
- **[GitHub](https://github.com/IceWhaleTech) で **フォローしてください**

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## ライセンス

このプロジェクトは MIT ライセンスに基づいてライセンスされています。詳細については、[LICENSE](../../LICENSE) ファイルを参照してください。私たちはオープンソースとコミュニティへの還元を信じています。

## 貢献者

Blue の寄稿者全員に感謝します。

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## 参考文献

1. **OpenClaw** — ローカルファーストのオープンソース エージェント。チャネル アダプタとツール呼び出しを介して LLM をローカル デバイスに接続する先駆者であり、Blue のエージェント ランタイム アーキテクチャに直接影響を与えました。 https://github.com/openclaw/openclaw
2. **MiroMind** — 証拠に裏付けられた合成による詳細な調査モード。 Blue の組み込みの詳細な調査パイプラインを形成: 計画、並行検索、証拠の重複排除、HTML レポートの生成。 https://www.miromind.ai
3. **Karpathy's LLM Wiki** — ナレッジ コンパイラとしての LLM。 LLM を再フレーム化して、永続的で進化する知識空間を構築し、RAG の蓄積トラップを超えます。
4. **OpenSpace (HKUDS)** — 自己進化するスキル エンジン。エージェントが失敗から学習し、専門的なスキルを引き出す DAG ベースのフレームワーク。 https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — コーディング エージェント用のバージョン付き API ドキュメント レジストリ。エージェントの幻覚と忘れられたセッション知識に対処します。注釈とフィードバック ループを備えた厳選され、バージョン管理されたドキュメントを提供し、ドキュメントを自己改善型のナレッジ レイヤーに変えます。 https://github.com/andrewyng/context-hub
6. **Notion** — シンプルで、人間的で、意図的に静かです。 Notion のミニマリスト精神にインスピレーションを受けて、Blue はグリッドに暖かさを取り戻します。洗練されたセリフと思慮深いデザインが融合し、まるで自宅にいるような空間が生まれます。 https://www.notion.com/about
7. **Matrix** — 象徴的なデジタル雨の美学からの視覚的なインスピレーション。 Blue の技術図の美的方向性。
8. **IceWhale** — ラブ、デス&ロボット S2E2「アイス」。インターネット巨人の壁を突破し、データ集中に抵抗するために世界中から集まる集団。氷のクジラは、エッジで主権ツールを一緒に構築するコミュニティを象徴しています。
9. **ZimaOS Blue** — 愛と死とロボット S1E14「ジーマ Blue」。比喩: サービスに始まり、世界を探索するために進化する知性。 Blue は、シンプルさを根幹にしながら奥深さを追求する知恵のエージェントです。
10. **ZimaOS** — シンプル、焦点を絞った、オープンな設計原則。 ZimaOS と Blue はどちらも、テクノロジーはユーザーに役立つべきである、つまり 30 秒で展開し、どこでも実行でき、ベンダー中立を保つべきであるという信念を共有しています。 https://www.zimaspace.com/zimaos
