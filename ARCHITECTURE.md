# ZimaOS-Blue Architecture

> A Local-first Agent Runtime for Builders with Bolder Mind
> Single-binary Go monolith · ~40MB · Idle ~4MB · Startup < 1s

```
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
  🎨 Vue 3 Frontend  (web/)  ── Vibe it / DIY it / 随便换
│                                                                                                         │
  Vue 3 + Vite + TypeScript + Tailwind CSS + i18n (18 langs)
│ Build 后 embed 进 Go binary，也可以独立部署，也可以整个换掉                                                  │
│                                                                                                         │
  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ │ChatView  │ │Providers │ │Channels  │ │Security  │ │Metrics   │ │Plugins   │ │Settings  │ │
  │          │ │PoolMgmt  │ │Mgmt      │ │Dashboard │ │Dashboard │ │Store     │ │Panels    │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ │Memory    │ │Companion │ │Heartbeat │ │Personality│ │Voice/TTS │ │Automation│ │
  │Browser   │ │Monitor   │ │Settings  │ │Editor    │ │Controls  │ │Workflows │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ │
│                                                                                                         │
  API: api/*.ts  (45 modules: chat, proxy, metrics, auth, memory, companion, voice, tts, ...)
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
         │ HTTP / WebSocket / SSE
         │ (Frontend fully decoupled — swap with any SPA, mobile app, or curl)
         ▼
╔════════════════════════════════════════════════════════════════════════════════════════════════════════════╗
║  Go Backend  (server/)                                                                                   ║
╠════════════════════════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                                          ║
║  ┌───────────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                        Gateway & Routing  (bootstrap/routes.go)                                   │    ║
║  │                                                                                                   │    ║
║  │  Echo Router ──► Middleware: Auth → RateLimit → CORS → PromptGuard → Logging → Metrics            │    ║
║  │                                                                                                   │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  OpenAI-Compatible       │    ║
║  │  │ /v1/messages  │ │ /v1/chat/*    │ │ /v1/models    │ │ /v1/embeddings│  Proxy Endpoints         │    ║
║  │  │ /v1/complete  │ │ /api/chat     │ │ /v1/providers │ │               │                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  Management API          │    ║
║  │  │ /api/memory/* │ │ /api/auth/*   │ │ /api/metrics/*│ │/api/channels/*│                          │    ║
║  │  │ /api/v2/mem/* │ │ /api/security │ │ /api/system/* │ │/api/heartbeat │                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  Feature API             │    ║
║  │  │/api/companion │ │ /api/plugins  │ │ /api/skills   │ │ /api/voice    │                          │    ║
║  │  │/api/personal  │ │ /api/cron     │ │ /api/workflow │ │ /api/tts      │                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  WebSocket & Debug       │    ║
║  │  │ /ws/chat      │ │ /ws/companion │ │ /ws/voice     │ │/debug/pprof/* │                          │    ║
║  │  │ (SSE stream)  │ │ (events)      │ │ (audio)       │ │(CPU/mem/gortn)│                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║         ▼                                                                                                ║
║  ┌───────────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  ★ API Proxy Sidecar  (proxy/)  ── 核心热路径，所有 LLM 请求必经                     │    ║
║  │                                                                                                   │    ║
║  │  Claude Code CLI / Any OpenAI Client                                                              │    ║
║  │       │                                                                                           │    ║
║  │       ▼                                                                                           │    ║
║  │  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌────────────────────┐              │    ║
║  │  │  Auth    │──▶│  Prompt  │──▶│  Context │──▶│  Route   │──▶│  Upstream LLM      │              │    ║
║  │  │  Gate    │   │  Guard   │   │  Pruner  │   │  Select  │   │  Provider          │              │    ║
║  │  │(api key) │   │(security)│   │(pruner/) │   │(pool/)   │   │(Anthropic/OpenAI/..)│             │    ║
║  │  └──────────┘   └──────────┘   └──────────┘   └──────────┘   └────────────────────┘              │    ║
║  │                                     │              │              │                               │    ║
║  │                                     ▼              ▼              ▼                               │    ║
║  │                               ┌──────────┐  ┌──────────┐  ┌──────────┐                           │    ║
║  │                               │  Score   │  │route:auto│  │ Failover │                           │    ║
║  │                               │  Cache   │  │route:cloud  │ Circuit  │                           │    ║
║  │                               │(ecache2) │  │route:local  │ Breaker  │                           │    ║
║  │                               └──────────┘  └──────────┘  └──────────┘                           │    ║
║  │                                                                                                   │    ║
║  │  ┌─────────────────────────────────────────────────────────────────────────────────────┐          │    ║
║  │  │  Two-Level Cache (CC Cache)                                                         │          │    ║
║  │  │  ┌───────────────────────┐          ┌───────────────────────────────┐                │          │    ║
║  │  │  │  L1: In-Memory LRU   │  miss──▶ │  L2: Disk SQLite Cache        │                │          │    ║
║  │  │  │  (ecache2 sharded)   │          │  (cache_disk.go)              │                │          │    ║
║  │  │  │  Fast path, ~μs      │          │  Persistent, survives restart │                │          │    ║
║  │  │  └───────────────────────┘          └───────────────────────────────┘                │          │    ║
║  │  └─────────────────────────────────────────────────────────────────────────────────────┘          │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                                          │    ║
║  │  │  Conn    │  │  Config  │  │  Model   │  │  Metrics │                                          │    ║
║  │  │  Pool    │  │  Watch   │  │  Compat  │  │  Writer  │                                          │    ║
║  │  │  HTTP/2  │  │  HotLoad │  │  Layer   │  │  (stats) │                                          │    ║
║  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘                                          │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Provider Pool  (providerpool/)  ── 统一 LLM 提供商管理                             │    ║
║  │                                                                                                   │    ║
║  │  Provider Types:                                                                                  │    ║
║  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │    ║
║  │  │ OpenAI  │ │Anthropic│ │  Ollama │ │  Grok   │ │  Qwen   │ │ Custom  │ │  Trial  │            │    ║
║  │  │ (cloud) │ │ (cloud) │ │ (local) │ │ (cloud) │ │ (cloud) │ │(OpenAI) │ │ (quota) │            │    ║
║  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘            │    ║
║  │  ┌─────────┐ ┌─────────┐                                                                         │    ║
║  │  │  ACP    │ │  IDE    │  (LM Studio auto-discover)                                               │    ║
║  │  │(Agent)  │ │(detect) │                                                                          │    ║
║  │  └─────────┘ └─────────┘                                                                          │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                                │    ║
║  │  │  Health  │ │ Priority │ │  Rate    │ │  API Key │ │  Model   │                                │    ║
║  │  │  Check   │ │  Route   │ │  Limit   │ │ Rotation │ │ Discover │                                │    ║
║  │  │(circuit) │ │  Engine  │ │(per-prov)│ │  & Usage │ │(auto)    │                                │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘                                │    ║
║  │                                                                                                   │    ║
║  │  API Formats: OpenAI (default) │ Anthropic │ Ollama │ Google AI                                   │    ║
║  │  Storage: {dataDir}/providerpool/*.json                                                           │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Context Pruner  (pruner/)  ── 智能上下文裁剪，降低 token 开销                       │    ║
║  │                                                                                                   │    ║
║  │  Request ──▶ DetectType ──▶ Segmentize ──▶ Score (BM25) ──▶ SelectTopK ──▶ Pruned Output          │    ║
║  │                  │              │              │                │                                  │    ║
║  │                  ▼              ▼              ▼                ▼                                  │    ║
║  │            ┌──────────┐  ┌──────────┐   ┌──────────┐    ┌──────────┐                              │    ║
║  │            │  Code    │  │ Function │   │ IDF Pre- │    │  Token   │                              │    ║
║  │            │  Log     │  │ Paragraph│   │ Compute  │    │  Budget  │                              │    ║
║  │            │  Doc     │  │ Data     │   │ ecache2  │    │  Greedy  │                              │    ║
║  │            │  Data    │  │ Import   │   │ LRU Cache│    │  Select  │                              │    ║
║  │            └──────────┘  └──────────┘   └──────────┘    └──────────┘                              │    ║
║  │                                                                                                   │    ║
║  │  Backends: local (line) │ bm25 (segment) │ ir (paragraph) │ remote (SWE-Pruner HTTP)             │    ║
║  │  Middleware: plugs into proxy, processes tool/user/system messages                                │    ║
║  │  Config: enabled=false, threshold=0.5, min_lines=50, cache=256 slots                             │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Agent Runtime  ── LLM 交互核心                                                    │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │    ║
║  │  │  LLM Client  │  │  Tool/Func   │  │  Context     │  │  Streaming   │  │  Personality │        │    ║
║  │  │  (llm/)      │  │  Calling     │  │  Manager     │  │  Response    │  │  (SOUL.md)   │        │    ║
║  │  │  OpenAI fmt  │  │  (tools/)    │  │  (context/)  │  │  (streaming/)│  │  (personal/) │        │    ║
║  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘        │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Memory & Intelligence                                                             │    ║
║  │                                                                                                   │    ║
║  │  ┌─────────────────────────────┐  ┌─────────────────────────────┐  ┌────────────────────────┐     │    ║
║  │  │  Conversation Memory        │  │  Vector Memory (optional)   │  │  Memory Service v2     │     │    ║
║  │  │  (memory/)                  │  │  (memory/sqlite_vec_store)  │  │  (memory/)             │     │    ║
║  │  │  ┌────────┐ ┌────────┐     │  │  ┌────────┐ ┌────────┐     │  │  ┌────────┐ ┌────────┐ │     │    ║
║  │  │  │Sessions│ │Messages│     │  │  │FTS5    │ │sqlite- │     │  │  │Version │ │Namespace│ │     │    ║
║  │  │  │        │ │History │     │  │  │Keyword │ │vec     │     │  │  │Entries │ │Isolate │ │     │    ║
║  │  │  └────────┘ └────────┘     │  │  │Search  │ │Hybrid  │     │  │  └────────┘ └────────┘ │     │    ║
║  │  │  ┌────────┐                │  │  └────────┘ └────────┘     │  │  Purge: every 6h       │     │    ║
║  │  │  │Compact │ (auto-summary) │  │  Embedding: text-embed-3   │  │  API: /api/v2/memory/* │     │    ║
║  │  │  └────────┘                │  │  AES-256-GCM encryption    │  └────────────────────────┘     │    ║
║  │  └─────────────────────────────┘  └─────────────────────────────┘                                │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Channel Adapters  (channel/)  ── 多渠道消息                                       │    ║
║  │                                                                                                   │    ║
║  │  ┌────────┐ ┌──────────┐ ┌─────────┐ ┌───────┐ ┌──────────┐ ┌────────┐ ┌──────────┐ ┌────────┐  │    ║
║  │  │  Web   │ │ Telegram │ │ Discord │ │ Slack │ │  WeChat  │ │ Feishu │ │  Matrix  │ │iMessage│  │    ║
║  │  │(built) │ │   Bot    │ │   Bot   │ │       │ │  Work    │ │ /Lark  │ │          │ │(exper.)│  │    ║
║  │  └────────┘ └──────────┘ └─────────┘ └───────┘ └──────────┘ └────────┘ └──────────┘ └────────┘  │    ║
║  │                                                                                                   │    ║
║  │  Manager: central registry, auto-reconnect, message queue                                         │    ║
║  │  Humanizer (humanizer/): Markdown → natural text (IM mode / Voice mode)                           │    ║
║  │  AutoReply (autoreply/): rule-based auto-response                                                 │    ║
║  │  Storage: {dataDir}/channels/*.json                                                               │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Security Layer  (security/ + auth/ + permission/)                                 │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │    ║
║  │  │  JWT     │ │ API Keys │ │ Password │ │  MFA     │ │ WebAuthn │ │  OIDC    │ │  RBAC    │     │    ║
║  │  │  Auth    │ │ Scoped   │ │ Argon2id │ │  TOTP    │ │ Passkeys │ │ Provider │ │ Roles    │     │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘     │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                                             │    ║
║  │  │  Threat  │ │  Sandbox │ │  Audit   │ │  TLS     │                                             │    ║
║  │  │ Detector │ │ Executor │ │  Trail   │ │ Manager  │                                             │    ║
║  │  │(SQLi/XSS│ │(isolated)│ │(immutable│ │(ACME/LE) │                                             │    ║
║  │  │PromptInj│ │ resource │ │ append)  │ │ auto-    │                                             │    ║
║  │  │ CmdInj) │ │ limits)  │ │          │ │ renew    │                                             │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘                                             │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Voice & Speech Pipeline  (voice/ + tts/ + stt/ + speech/)                         │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────────────────────────────────────────────────────────────────────────────┐         │    ║
║  │  │  STT (Whisper)  ──▶  LLM Processing  ──▶  TTS (eSpeak-NG / Edge TTS)               │         │    ║
║  │  │  ┌──────────┐       ┌──────────┐         ┌──────────┐ ┌──────────┐                  │         │    ║
║  │  │  │  Whisper │       │  Agent   │         │ eSpeak-NG│ │ Edge TTS │                  │         │    ║
║  │  │  │  (local) │       │  Runtime │         │  (local) │ │ (cloud)  │                  │         │    ║
║  │  │  │  lazy    │       │          │         │  CGO,lazy│ │ MS Edge  │                  │         │    ║
║  │  │  └──────────┘       └──────────┘         └──────────┘ └──────────┘                  │         │    ║
║  │  └──────────────────────────────────────────────────────────────────────────────────────┘         │    ║
║  │  Session: per-user │ States: Idle→Listening→Processing→Speaking │ WebSocket audio stream          │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Observability & Agents                                                             │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────────────────┐  ┌──────────────────────────┐  ┌──────────────────────────┐        │    ║
║  │  │  Metrics  (metrics/)     │  │  Heartbeat (heartbeat/)  │  │  Companion (companion/)  │        │    ║
║  │  │  ┌────────┐ ┌────────┐  │  │  ┌────────┐ ┌────────┐  │  │  ┌────────┐ ┌────────┐  │        │    ║
║  │  │  │CPU/Mem │ │API Call│  │  │  │Periodic│ │HEARTBEAT  │  │  │  │Realtime│ │Session │  │        │    ║
║  │  │  │GC/Gortn│ │Token $ │  │  │  │Poll    │ │.md Prompt│  │  │  │Events  │ │Monitor │  │        │    ║
║  │  │  └────────┘ └────────┘  │  │  └────────┘ └────────┘  │  │  └────────┘ └────────┘  │        │    ║
║  │  │  SQLite: metrics.db     │  │  Dedup: 24h cache        │  │  WebSocket + JSONL      │        │    ║
║  │  │  Interval: 10s, 30pts   │  │  Active hours window     │  │  Retention: 7d/30d/90d  │        │    ║
║  │  └──────────────────────────┘  └──────────────────────────┘  └──────────────────────────┘        │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Plugin & Skill System  (plugin/ + skill/ + skillstore/)                            │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────────────────┐  ┌──────────────────────────┐  ┌──────────────────────────┐        │    ║
║  │  │  Plugin Runtime          │  │  Skill Registry          │  │  Skill Store             │        │    ║
║  │  │  ┌────────┐ ┌────────┐  │  │  ┌────────┐ ┌────────┐  │  │  ┌────────┐ ┌────────┐  │        │    ║
║  │  │  │Go      │ │  JS    │  │  │  │Weather │ │Search  │  │  │  │Featured│ │Install │  │        │    ║
║  │  │  │Module  │ │ (goja) │  │  │  │Calc    │ │SysInfo │  │  │  │Skills  │ │& Sync  │  │        │    ║
║  │  │  └────────┘ └────────┘  │  │  │DateTime│ │Custom  │  │  │  └────────┘ └────────┘  │        │    ║
║  │  │  Isolation + limits     │  │  └────────┘ └────────┘  │  │  SQLite: skills.db      │        │    ║
║  │  └──────────────────────────┘  └──────────────────────────┘  └──────────────────────────┘        │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Integrations & Automation                                                         │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │    ║
║  │  │ Browser  │ │  Home    │ │  Cron    │ │ Workflow │ │  Form   │ │  Ngrok   │ │  Backup  │     │    ║
║  │  │ (Rod)    │ │Assistant │ │ Schedule │ │ (n8n)   │ │ Filler  │ │ Tunnel   │ │ Restore  │     │    ║
║  │  │ lazy init│ │  REST    │ │          │ │ nodes   │ │ template│ │ SDK      │ │ 7d retain│     │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘     │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Scheduler  (scheduler/)  ── 优先级任务调度                                         │    ║
║  │                                                                                                   │    ║
║  │  Priority: Low → Normal → High → Critical                                                        │    ║
║  │  States: Pending → Running → Completed/Failed/Cancelled                                           │    ║
║  │  Retry: exponential backoff │ Deps: DAG resolution │ Max concurrent: 10                           │    ║
║  │  Persistence: SQLite (scheduler/persistence/)                                                     │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Core Runtime  (v0.1)  ── 稳定内核                                                 │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                  │    ║
║  │  │Lifecycle │ │  Worker  │ │  Config  │ │  Logger  │ │  Update  │ │  SysInfo │                  │    ║
║  │  │ Manager  │ │  Pool    │ │  (Viper) │ │  (Zap)   │ │  OTA     │ │  (mem/cpu│                  │    ║
║  │  │ graceful │ │conc/pool │ │ hot-load │ │ struct   │ │ channels │ │  process)│                  │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘                  │    ║
║  │                                                                                                   │    ║
║  │  GC: GOGC=30, GOMEMLIMIT=48MB │ DB pool: 5 open, 2 idle │ Argon2: 32MB                          │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Data Layer                                                                        │    ║
║  │                                                                                                   │    ║
║  │  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐  │    ║
║  │  │  SQLite (WAL)  │ │  memory.db     │ │  metrics.db    │ │  skills.db     │ │  Files/JSON    │  │    ║
║  │  │  blue.db       │ │  vector_mem.db │ │  memory_svc.db │ │  cache (disk)  │ │  {dataDir}/*   │  │    ║
║  │  │  users/roles   │ │  conversations │ │  API stats     │ │  skill meta    │ │  providers     │  │    ║
║  │  │  audit/keys    │ │  embeddings    │ │  token usage   │ │  featured list │ │  channels      │  │    ║
║  │  └────────────────┘ └────────────────┘ └────────────────┘ └────────────────┘ └────────────────┘  │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║                                                                                                          ║
╚════════════════════════════════════════════════════════════════════════════════════════════════════════════╝
         │
         ▼
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  External Services                                                                                        │
│                                                                                                            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │ Anthropic│ │  OpenAI  │ │  Ollama  │ │ Telegram │ │ Discord  │ │  Home    │ │  Let's   │              │
│  │  API     │ │  API     │ │  (local) │ │  Bot API │ │  Bot API │ │ Assist   │ │ Encrypt  │              │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘              │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### Chat Request (Proxy Hot Path)
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

### Channel Message Flow
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

### Voice Pipeline
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

## Package Map (server/internal/)

| Layer | Packages |
|-------|----------|
| Gateway | `bootstrap`, `server`, `gateway` |
| Proxy | `proxy`, `connection`, `streaming`, `resilience` |
| Provider | `providerpool`, `providers`, `llm` |
| Pruner | `pruner` (detector, segmenter, bm25, pipeline, cache) |
| Agent | `context`, `tools`, `personality`, `humanizer` |
| Memory | `memory`, `embedding`, `kvstore` |
| Channel | `channel`, `autoreply`, `i18n` |
| Security | `security`, `auth`, `permission`, `rbac`, `mfa`, `password`, `oidc`, `extauth`, `sandbox`, `promptguard`, `audit` |
| Voice | `voice`, `tts`, `stt`, `speech` |
| Observe | `metrics`, `heartbeat`, `companion`, `profiling`, `leakdetect` |
| Plugin | `plugin`, `skill`, `skillstore` |
| Integrate | `browser`, `homeassistant`, `cron`, `workflow`, `formfiller`, `ngrok`, `crawler` |
| Scheduler | `scheduler`, `worker`, `workerpool`, `pool` |
| Core | `lifecycle`, `config`, `logger`, `database`, `cache`, `ratelimit`, `retry`, `timeutil`, `sync` |
| System | `sysinfo`, `cgroup`, `iotask`, `watcher`, `resources`, `backup`, `update` |
| Multi-tenant | `tenant`, `user`, `session`, `preview` |