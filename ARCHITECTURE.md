# ZimaOS-Blue Architecture

> A Local-first Agent Runtime for Builders with Bolder Mind
> Single-binary Go monolith · ~40MB · Idle ~4MB · Startup < 1s

```
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
│ 🎨 Vue 3 Frontend  (web/)  ── Vibe it / DIY it / Swap it                                      │
│                                                                                               │
│ Vue 3 + Vite + TypeScript + Tailwind CSS + i18n (18 langs)                                    │
│ Build output embedded into Go binary; can also deploy standalone or swap entirely             │
│                                                                                               │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│ │ChatView  │ │Providers │ │Channels  │ │Security   │ │Metrics   │ │Plugins   │ │Settings  │   │
│ │          │ │PoolMgmt  │ │Mgmt      │ │Dashboard  │ │Dashboard │ │Store     │ │Panels    │   │
│ └──────────┘ └──────────┘ └──────────┘ └───────────┘ └──────────┘ └──────────┘ └──────────┘   │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐                │
│ │Memory    │ │Companion │ │Heartbeat │ │Personality│ │Voice/TTS │ │Automation│                │
│ │Browser   │ │Monitor   │ │Settings  │ │Editor     │ │Controls  │ │Workflows │                │
│ └──────────┘ └──────────┘ └──────────┘ └───────────┘ └──────────┘ └──────────┘                │
│                                                                                               │
│ API: api/*.ts  (45 modules: chat, proxy, metrics, auth, memory, companion, voice, tts, ...)   │
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘
         │ HTTP / WebSocket / SSE
         │ (Frontend fully decoupled — swap with any SPA, mobile app, or curl)
         ▼
╔═══════════════════════════════════════════════════════════════════════════════════════════════════════════╗
║  Go Backend  (server/)                                                                                    ║
╠═══════════════════════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                                           ║
║  ┌───────────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                        Gateway & Routing  (bootstrap/routes.go)                                   │    ║
║  │                                                                                                   │    ║
║  │  Echo Router ──► Middleware: Auth → RateLimit → CORS → PromptGuard → Logging → Metrics            │    ║
║  │                                                                                                   │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  OpenAI-Compatible       │    ║
║  │  │  Messages     │ │  Chat         │ │  Models       │ │  Embeddings   │  Proxy Endpoints         │    ║
║  │  │  Completions  │ │  Streaming    │ │  Providers    │ │               │                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  Management API          │    ║
║  │  │  Memory       │ │  Auth         │ │  Metrics      │ │  Channels     │                          │    ║
║  │  │  MemoryV2     │ │  Security     │ │  System       │ │  Heartbeat    │                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  Feature API             │    ║
║  │  │  Companion    │ │  Plugins      │ │  Skills       │ │  Voice        │                          │    ║
║  │  │  Personality  │ │  Cron         │ │  Workflow     │ │  TTS          │                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  │  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  WebSocket & Debug       │    ║
║  │  │  ChatStream   │ │  Companion    │ │  VoiceStream  │ │  Profiling    │                          │    ║
║  │  │  (SSE)        │ │  (events)     │ │  (audio)      │ │(CPU/mem/gortn)│                          │    ║
║  │  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘                          │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║         ▼                                                                                                 ║
║  ┌───────────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  ★ API Proxy Sidecar  (proxy/)  ── Core hot path, all LLM requests pass through   │    ║
║  │                                                                                                   │    ║
║  │  Claude Code CLI / Any OpenAI Client / Anthropic SDK / Gemini SDK / OAuth Client                  │    ║
║  │       │                                                                                           │    ║
║  │       ▼                                                                                           │    ║
║  │  Inbound API Formats (4 upstream protocols)                                                       │    ║
║  │  ┌──────────────┐ ┌──────────────┐ ┌───────────────┐ ┌──────────────┐                             │    ║
║  │  │OpenAI Compat │ │Anthropic API │ │  Gemini API   │ │  OAuth API   │                             │    ║
║  │  │chat/responses│ │  messages    │ │generateContent│ │ token-based  │                             │    ║
║  │  └──────────────┘ └──────────────┘ └───────────────┘ └──────────────┘                             │    ║
║  │       │                                                                                           │    ║
║  │       ▼                                                                                           │    ║
║  │  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐                         │    ║
║  │  │  Auth    │──▶│  Prompt  │──▶│  Context │──▶│  Route   │──▶│ Upstream │                         │    ║
║  │  │  Gate    │   │  Guard   │   │  Pruner  │   │  Select  │   │ Provider │                         │    ║
║  │  │(api key) │   │(security)│   │(pruner/) │   │(pool/)   │   │          │                         │    ║
║  │  └──────────┘   └──────────┘   └──────────┘   └──────────┘   └──────────┘                         │    ║
║  │                                     │              │              │                               │    ║
║  │                                     ▼              ▼              ▼                               │    ║
║  │                               ┌──────────┐  ┌────────────┐  ┌──────────┐                          │    ║
║  │                               │  Score   │  │ route:auto │  │ Failover │                          │    ║
║  │                               │  Cache   │  │ route:cloud│  │ Circuit  │                          │    ║
║  │                               │(ecache2) │  │ route:local│  │ Breaker  │                          │    ║
║  │                               └──────────┘  └────────────┘  └──────────┘                          │    ║
║  │                                                                                                   │    ║
║  │  ┌─────────────────────────────────────────────────────────────────────────────────────┐          │    ║
║  │  │  Two-Level Cache (CC Cache)                                                         │          │    ║
║  │  │  ┌──────────────────────┐          ┌───────────────────────────────┐                │          │    ║
║  │  │  │  L1: In-Memory LRU   │  miss──▶ │  L2: Disk SQLite Cache        │                │          │    ║
║  │  │  │  (ecache2 sharded)   │          │  (cache_disk.go)              │                │          │    ║
║  │  │  │  Fast path, ~μs      │          │  Persistent, survives restart │                │          │    ║
║  │  │  └──────────────────────┘          └───────────────────────────────┘                │          │    ║
║  │  └─────────────────────────────────────────────────────────────────────────────────────┘          │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                                           │    ║
║  │  │  Conn    │  │  Config  │  │  Model   │  │  Metrics │                                           │    ║
║  │  │  Pool    │  │  Watch   │  │  Compat  │  │  Writer  │                                           │    ║
║  │  │  HTTP/2  │  │  HotLoad │  │  Layer   │  │  (stats) │                                           │    ║
║  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘                                           │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Provider Pool  (providerpool/)  ── Unified LLM provider management               │    ║
║  │                                                                                                   │    ║
║  │  Provider Types:                                                                                  │    ║
║  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │    ║
║  │  │ OpenAI  │ │Anthropic│ │ Google  │ │DeepSeek │ │Moonshot │ │  Azure  │ │  Open   │              │    ║
║  │  │ (cloud) │ │ (cloud) │ │ Gemini  │ │ (cloud) │ │ (Kimi)  │ │ OpenAI  │ │ Router  │              │    ║
║  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘              │    ║
║  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │    ║
║  │  │AiHubMix │ │  Ollama │ │ MiniMax │ │  Codex  │ │  Grok   │ │  Qwen   │ │ Venice  │              │    ║
║  │  │ (cloud) │ │ (local) │ │ (cloud) │ │ (cloud) │ │  (xAI)  │ │(Alibaba)│ │   AI    │              │    ║
║  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘              │    ║
║  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                                      │    ║
║  │  │ Amazon  │ │  GLM    │ │ Custom  │ │  Trial  │ │  IDE    │                                      │    ║
║  │  │ Bedrock │ │ (Zhipu) │ │(OpenAI) │ │ (quota) │ │(detect) │                                      │    ║
║  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘                                      │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                                 │    ║
║  │  │  Health  │ │ Priority │ │  Rate    │ │  API Key │ │  Model   │                                 │    ║
║  │  │  Check   │ │  Route   │ │  Limit   │ │ Rotation │ │ Discover │                                 │    ║
║  │  │(circuit) │ │  Engine  │ │(per-prov)│ │  & Usage │ │(auto)    │                                 │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘                                 │    ║
║  │                                                                                                   │    ║
║  │  API Formats: OpenAI Compat │ Anthropic │ Gemini │ OAuth                                          │    ║
║  │  Storage: {dataDir}/providerpool/*.json                                                           │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Context Pruner  (pruner/)  ── Smart context pruning, reduce token cost           │    ║
║  │                                                                                                   │    ║
║  │  Request ──▶ DetectType ──▶ Segmentize ──▶ Score (BM25) ──▶ SelectTopK ──▶ Pruned Output          │    ║
║  │                  │              │              │                │                                 │    ║
║  │                  ▼              ▼              ▼                ▼                                 │    ║
║  │            ┌──────────┐  ┌──────────┐   ┌──────────┐    ┌──────────┐                              │    ║
║  │            │  Code    │  │ Function │   │ IDF Pre- │    │  Token   │                              │    ║
║  │            │  Log     │  │ Paragraph│   │ Compute  │    │  Budget  │                              │    ║
║  │            │  Doc     │  │ Data     │   │ ecache2  │    │  Greedy  │                              │    ║
║  │            │  Data    │  │ Import   │   │ LRU Cache│    │  Select  │                              │    ║
║  │            └──────────┘  └──────────┘   └──────────┘    └──────────┘                              │    ║
║  │                                                                                                   │    ║
║  │  Backends: local (line) │ bm25 (segment) │ ir (paragraph) │ swe-pruner (Go native)                │    ║
║  │  Middleware: plugs into proxy, processes tool/user/system messages                                │    ║
║  │  Config: enabled=false, threshold=0.5, min_lines=50, cache=256 slots                              │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Agent Runtime  ── LLM interaction core                                           │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │    ║
║  │  │  LLM Client  │  │  Tool/Func   │  │  Context     │  │  Streaming   │  │  Personality │         │    ║
║  │  │  (llm/)      │  │  Calling     │  │  Manager     │  │  Response    │  │  (SOUL.md)   │         │    ║
║  │  │  OpenAI fmt  │  │  (tools/)    │  │  (context/)  │  │  (streaming/)│  │  (personal/) │         │    ║
║  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘         │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Memory & Intelligence                                                            │    ║
║  │                                                                                                   │    ║
║  │  ┌─────────────────────────────┐  ┌─────────────────────────────┐  ┌─────────────────────────┐    │    ║
║  │  │  Conversation Memory        │  │  Vector Memory (optional)   │  │  Memory Service v2      │    │    ║
║  │  │  (memory/)                  │  │  (memory/sqlite_vec_store)  │  │  (memory/)              │    │    ║
║  │  │  ┌────────┐ ┌────────┐      │  │  ┌────────┐ ┌────────┐      │  │  ┌────────┐ ┌─────────┐ │    │    ║
║  │  │  │Sessions│ │Messages│      │  │  │FTS5    │ │sqlite- │      │  │  │Version │ │Namespace│ │    │    ║
║  │  │  │        │ │History │      │  │  │Keyword │ │vec     │      │  │  │Entries │ │Isolate  │ │    │    ║
║  │  │  └────────┘ └────────┘      │  │  │Search  │ │Hybrid  │      │  │  └────────┘ └─────────┘ │    │    ║
║  │  │  ┌────────┐                 │  │  └────────┘ └────────┘      │  │  Purge: every 6h        │    │    ║
║  │  │  │Compact │ (auto-summary)  │  │  Embedding: text-embed-3    │  │  API: MemoryV2 module   │    │    ║
║  │  │  └────────┘                 │  │  AES-256-GCM encryption     │  └─────────────────────────┘    │    ║
║  │  └─────────────────────────────┘  └─────────────────────────────┘                                 │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Channel Adapters  (channel/)  ── Multi-channel messaging                         │    ║
║  │                                                                                                   │    ║
║  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐          │    ║
║  │  │  Web   │ │Telegram│ │Discord │ │ Slack  │ │ WeChat │ │ Feishu │ │ Matrix │ │iMessage│          │    ║
║  │  │(built) │ │        │ │        │ │        │ │  Work  │ │ /Lark  │ │        │ │        │          │    ║
║  │  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘          │    ║
║  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐          │    ║
║  │  │WhatsApp│ │ Signal │ │ Teams  │ │Matter- │ │  Blue  │ │  Zalo  │ │DingTalk│ │ Google │          │    ║
║  │  │        │ │        │ │        │ │ most   │ │Bubbles │ │        │ │        │ │  Chat  │          │    ║
║  │  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘          │    ║
║  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐          │    ║
║  │  │  LINE  │ │Messengr│ │  Next  │ │ Nostr  │ │   QQ   │ │ Twitch │ │Twitter │ │ Viber  │          │    ║
║  │  │        │ │        │ │  cloud │ │        │ │        │ │        │ │        │ │        │          │    ║
║  │  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘          │    ║
║  │  ┌────────┐                                                                                       │    ║
║  │  │Instagrm│                                                                                       │    ║
║  │  │        │                                                                                       │    ║
║  │  └────────┘                                                                                       │    ║
║  │                                                                                                   │    ║
║  │  Manager: central registry, auto-reconnect, message queue                                         │    ║
║  │  Humanizer (humanizer/): Markdown → natural text (IM mode / Voice mode)                           │    ║
║  │  AutoReply (autoreply/): rule-based auto-response                                                 │    ║
║  │  Storage: {dataDir}/channels/*.json                                                               │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Security Layer  (security/ + auth/ + permission/)                                │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │    ║
║  │  │  JWT     │ │ API Keys │ │ Password │ │  MFA     │ │ WebAuthn │ │  OIDC    │ │  RBAC    │       │    ║
║  │  │  Auth    │ │ Scoped   │ │ Argon2id │ │  TOTP    │ │ Passkeys │ │ Provider │ │ Roles    │       │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘       │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                                              │    ║
║  │  │  Threat  │ │  Sandbox │ │  Audit   │ │  TLS     │                                              │    ║
║  │  │ Detector │ │ Executor │ │  Trail   │ │ Manager  │                                              │    ║
║  │  │(SQLi/XSS │ │(isolated │ │(immutable│ │(ACME/LE) │                                              │    ║
║  │  │PromptInj │ │ resource │ │ append)  │ │ auto-    │                                              │    ║
║  │  │ CmdInj)  │ │ limits)  │ │          │ │ renew    │                                              │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘                                              │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Voice & Speech Pipeline  (voice/ + tts/ + stt/ + speech/)                        │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────────────────────────────────────────────────────────────────────────────┐         │    ║
║  │  │  STT (Whisper)  ──▶  LLM Processing  ──▶  TTS (eSpeak-NG / Edge TTS)                 │         │    ║
║  │  │  ┌──────────┐       ┌──────────┐         ┌──────────┐ ┌──────────┐                   │         │    ║
║  │  │  │  Whisper │       │  Agent   │         │ eSpeak-NG│ │ Edge TTS │                   │         │    ║
║  │  │  │  (local) │       │  Runtime │         │  (local) │ │ (cloud)  │                   │         │    ║
║  │  │  │  lazy    │       │          │         │  CGO,lazy│ │ MS Edge  │                   │         │    ║
║  │  │  └──────────┘       └──────────┘         └──────────┘ └──────────┘                   │         │    ║
║  │  └──────────────────────────────────────────────────────────────────────────────────────┘         │    ║
║  │  Session: per-user │ States: Idle→Listening→Processing→Speaking │ WebSocket audio stream          │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Observability & Agents                                                           │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────────────────┐  ┌──────────────────────────┐  ┌──────────────────────────┐         │    ║
║  │  │  Metrics  (metrics/)     │  │  Heartbeat (heartbeat/)  │  │  Companion (companion/)  │         │    ║
║  │  │  ┌────────┐ ┌────────┐   │  │  ┌────────┐ ┌────────┐   │  │  ┌────────┐ ┌────────┐   │         │    ║
║  │  │  │CPU/Mem │ │API Call│   │  │  │Periodic│ │HEART-  │   │  │  │Realtime│ │Session │   │         │    ║
║  │  │  │GC/Gortn│ │Token $ │   │  │  │Poll    │ │BEAT.md │   │  │  │Events  │ │Monitor │   │         │    ║
║  │  │  └────────┘ └────────┘   │  │  └────────┘ └────────┘   │  │  └────────┘ └────────┘   │         │    ║
║  │  │  SQLite: blue.db         │  │  Dedup: 24h cache        │  │  WebSocket + JSONL       │         │    ║
║  │  │  Interval: 10s, 30pts    │  │  Active hours window     │  │  Retention: 7d/30d/90d   │         │    ║
║  │  └──────────────────────────┘  └──────────────────────────┘  └──────────────────────────┘         │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Plugin & Skill System  (plugin/ + skill/ + skillstore/)                          │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────────────────────┐  ┌──────────────────────────┐  ┌──────────────────────────┐         │    ║
║  │  │  Plugin Runtime          │  │  Skill Registry          │  │  Skill Store             │         │    ║
║  │  │  ┌────────┐ ┌────────┐   │  │  ┌────────┐ ┌────────┐   │  │  ┌────────┐ ┌────────┐   │         │    ║
║  │  │  │Go      │ │  JS    │   │  │  │Weather │ │Search  │   │  │  │Featured│ │Install │   │         │    ║
║  │  │  │Module  │ │ (goja) │   │  │  │Calc    │ │SysInfo │   │  │  │Skills  │ │& Sync  │   │         │    ║
║  │  │  └────────┘ └────────┘   │  │  │DateTime│ │Custom  │   │  │  └────────┘ └────────┘   │         │    ║
║  │  │  Isolation + limits      │  │  └────────┘ └────────┘   │  │  SQLite: skills.db       │         │    ║
║  │  └──────────────────────────┘  └──────────────────────────┘  └──────────────────────────┘         │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Integrations & Automation                                                        │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │    ║
║  │  │ Browser  │ │  Home    │ │  Cron    │ │ Workflow │ │  Form    │ │  Tunnel  │ │  Backup  │       │    ║
║  │  │ (Rod)    │ │Assistant │ │ Schedule │ │ (n8n)    │ │ Filler   │ │ Manager  │ │ Restore  │       │    ║
║  │  │ lazy init│ │  REST    │ │          │ │ nodes    │ │ template │ │ (4 provs)│ │ 7d retain│       │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘       │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Scheduler  (scheduler/)  ── Priority task scheduling                             │    ║
║  │                                                                                                   │    ║
║  │  Priority: Low → Normal → High → Critical                                                         │    ║
║  │  States: Pending → Running → Completed/Failed/Cancelled                                           │    ║
║  │  Retry: exponential backoff │ Deps: DAG resolution │ Max concurrent: 10                           │    ║
║  │  Persistence: SQLite (scheduler/persistence/)                                                     │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Core Runtime  (v0.1)  ── Stable kernel                                           │    ║
║  │                                                                                                   │    ║
║  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                    │    ║
║  │  │Lifecycle │ │  Worker  │ │  Config  │ │  Logger  │ │  Update  │ │  SysInfo │                    │    ║
║  │  │ Manager  │ │  Pool    │ │  (Viper) │ │  (Zap)   │ │  OTA     │ │  (mem/cpu│                    │    ║
║  │  │ graceful │ │conc/pool │ │ hot-load │ │ struct   │ │ channels │ │  process)│                    │    ║
║  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘                    │    ║
║  │                                                                                                   │    ║
║  │  GC: GOGC=30, GOMEMLIMIT=48MB │ DB pool: 8 open, 3 idle │ Argon2: 32MB                            │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║         │                                                                                                 ║
║  ┌──────┴────────────────────────────────────────────────────────────────────────────────────────────┐    ║
║  │                  Data Layer                                                                       │    ║
║  │                                                                                                   │    ║
║  │  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐   │    ║
║  │  │  SQLite (WAL)  │ │  skills.db     │ │  cache.db      │ │  Files/JSON    │ │  memory.db     │   │    ║
║  │  │  blue.db       │ │  skill meta    │ │  L2 disk cache │ │  {dataDir}/*   │ │  vector search │   │    ║
║  │  │  users/roles   │ │  featured list │ │  (proxy)       │ │  providers     │ │  sqlite-vec    │   │    ║
║  │  │  audit/keys    │ │                │ │                │ │  channels      │ │  FTS/hybrid    │   │    ║
║  │  │  conversations │ │                │ │                │ │                │ │                │   │    ║
║  │  │  api_keys      │ │                │ │                │ │                │ │                │   │    ║
║  │  └────────────────┘ └────────────────┘ └────────────────┘ └────────────────┘ └────────────────┘   │    ║
║  └───────────────────────────────────────────────────────────────────────────────────────────────────┘    ║
║                                                                                                           ║
╚═══════════════════════════════════════════════════════════════════════════════════════════════════════════╝
         │
         ▼
┌───────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  External Services                                                                                        │
│                                                                                                           │
│  LLM Providers                                                                                            │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │ OpenAI  │ │Anthropic│ │ Google  │ │DeepSeek │ │Moonshot │ │  Azure  │ │  Open   │ │AiHubMix │          │
│  │         │ │         │ │ Gemini  │ │         │ │ (Kimi)  │ │ OpenAI  │ │ Router  │ │         │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │  Ollama │ │ MiniMax │ │  Codex  │ │  Grok   │ │  Qwen   │ │ Venice  │ │ Amazon  │ │  GLM    │          │
│  │ (local) │ │         │ │         │ │  (xAI)  │ │(Alibaba)│ │   AI    │ │ Bedrock │ │ (Zhipu) │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
│                                                                                                           │
│  Channels (IM / Social)                                                                                   │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐                  │
│  │Telegram│ │Discord │ │ Slack  │ │ WeChat │ │ Feishu │ │ Matrix │ │iMessage│ │WhatsApp│                  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘                  │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐                  │
│  │ Signal │ │ Teams  │ │Matter- │ │  Zalo  │ │DingTalk│ │ Google │ │  LINE  │ │Messengr│                  │
│  │        │ │        │ │ most   │ │        │ │        │ │  Chat  │ │        │ │        │                  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘                  │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐                                        │
│  │  Next  │ │ Nostr  │ │   QQ   │ │ Twitch │ │Twitter │ │ Viber  │                                        │
│  │  cloud │ │        │ │        │ │        │ │        │ │        │                                        │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘                                        │
│                                                                                                           │
│  Smart Home & IoT                                                                                         │
│  ┌──────────┐                                                                                             │
│  │  Home    │                                                                                             │
│  │Assistant │                                                                                             │
│  └──────────┘                                                                                             │
│                                                                                                           │
│  Remote Access (tunnel/)                                                                                  │
│  ┌──────────┐ ┌──────────┐                                                                                │
│  │  Ngrok   │ │Cloudflare│                                                                                │
│  │          │ │  Tunnel  │                                                                                │
│  └──────────┘ └──────────┘                                                                                │
│                                                                                                           │
│  Infrastructure                                                                                           │
│  ┌──────────┐ ┌──────────┐                                                                                │
│  │  Let's   │ │ Edge TTS │                                                                                │
│  │ Encrypt  │ │ (MS TTS) │                                                                                │
│  └──────────┘ └──────────┘                                                                                │
└───────────────────────────────────────────────────────────────────────────────────────────────────────────┘
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
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
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
| Integrate | `browser`, `homeassistant`, `cron`, `workflow`, `formfiller`, `tunnel`, `crawler` |
| Scheduler | `scheduler`, `worker`, `workerpool`, `pool` |
| Core | `lifecycle`, `config`, `logger`, `database`, `cache`, `ratelimit`, `retry`, `timeutil`, `sync` |
| System | `sysinfo`, `cgroup`, `iotask`, `watcher`, `resources`, `backup`, `update` |
| Multi-tenant | `tenant`, `user`, `session`, `preview` |