# PRD: Memory Service for LLM Agents

**Version**: 0.10.28
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-02-15
**License**: MIT

---

## 1. Overview

### 1.1 Problem Statement

LLM agents are stateless by default. Each conversation starts from zero context, forcing users to repeat preferences, re-explain project structures, and re-establish working context. This leads to:

1. **Token waste** — agents re-discover information that was already established in prior sessions
2. **Inconsistent behavior** — without persistent memory, agents cannot learn user preferences or adapt over time
3. **Context window pressure** — stuffing all relevant history into a single prompt is expensive and hits token limits
4. **Poor continuity** — multi-session tasks lose coherence when the agent forgets prior decisions

A dedicated memory service solves this by providing persistent, queryable, semantically searchable memory that agents can read from and write to across sessions.

### 1.2 Goals

1. Provide long-term persistent memory storage for LLM agents
2. Support short-term / working memory with automatic expiration
3. Enable semantic retrieval via embedding-based vector search
4. Store structured metadata alongside memory content
5. Support versioned memory entries with full history
6. Remain local-first and offline-capable with no mandatory cloud dependency
7. Expose a clean HTTP and gRPC API
8. Allow pluggable storage backends (vector DB, KV store)
9. Implement in Go for consistency with the existing ZimaOS-Blue server

### 1.3 Non-Goals

- This is NOT a general-purpose database or document store
- This does NOT replace conversation history storage (chat logs remain separate)
- This does NOT perform inference or reasoning — it only stores and retrieves
- This does NOT manage agent orchestration or tool routing
- No built-in UI in this version (API-only)

### 1.4 Target Users

| User | Need |
|------|------|
| LLM agent runtime | Read/write memory during conversations |
| Agent developers | Configure memory behavior, inspect stored data |
| System administrators | Monitor storage usage, manage retention policies |
| End users (indirect) | Benefit from agents that remember preferences and context |

---

## 2. Core Use Cases

### UC-1: Agent Stores a Learned Fact
An agent discovers that the user prefers TypeScript over JavaScript. The agent writes this as a long-term memory entry with metadata tags `["preference", "language"]`. On future sessions, the agent queries for user preferences and retrieves this fact.

### UC-2: Working Memory During a Task
An agent is debugging a multi-file issue. It stores intermediate findings (stack traces, file paths, hypotheses) as short-term memory entries with a 24-hour TTL. These entries are automatically cleaned up after the task window closes.

### UC-3: Semantic Search Across Memories
An agent needs to recall anything related to "database migration." It performs a semantic search using an embedding of the query. The memory service returns the top-K most relevant entries ranked by cosine similarity, regardless of exact keyword matches.

### UC-4: Memory Versioning
An agent updates a previously stored fact — the user's preferred framework changed from React to Svelte. The memory service creates a new version of the entry, preserving the old version in history. The agent can query version history if needed.

### UC-5: Namespace Isolation
Multiple agents (or multiple users) share the same memory service instance. Each operates within its own namespace, ensuring complete isolation. Agent A cannot read Agent B's memories unless explicitly granted cross-namespace access.

### UC-6: Bulk Context Loading
At the start of a session, an agent loads its top-N most relevant memories based on the current conversation topic. This pre-populates the agent's context window with useful background, reducing the need for the user to re-explain.

---

## 3. Functional Requirements

### 3.1 Memory Entry Model

A memory entry is the atomic unit of storage.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string (ULID) | Unique identifier, time-sortable |
| `namespace` | string | Isolation boundary (e.g., agent ID, user ID) |
| `content` | string | The memory text content |
| `content_type` | enum | `text`, `json`, `markdown` |
| `category` | string | Classification (e.g., `fact`, `preference`, `task_note`, `episode`) |
| `tags` | []string | Freeform labels for filtering |
| `metadata` | map[string]any | Arbitrary structured data |
| `embedding` | []float32 | Vector embedding of content (auto-generated or provided) |
| `importance` | float32 | 0.0–1.0 score indicating retrieval priority |
| `ttl` | duration | Time-to-live; 0 = permanent |
| `version` | int | Auto-incrementing version number |
| `parent_id` | string | Previous version's ID (for version chain) |
| `created_at` | timestamp | Creation time |
| `updated_at` | timestamp | Last modification time |
| `expires_at` | timestamp | Computed from TTL; null if permanent |
| `source` | string | Origin identifier (e.g., `agent:chat`, `agent:tool`, `user:manual`) |

### 3.2 Core Operations

| Operation | Method | Description |
|-----------|--------|-------------|
| Create | `POST /memories` | Store a new memory entry |
| Get | `GET /memories/:id` | Retrieve by ID |
| Update | `PUT /memories/:id` | Create new version of existing entry |
| Delete | `DELETE /memories/:id` | Soft-delete (mark as deleted, retain for audit) |
| List | `GET /memories` | List with filtering, pagination, sorting |
| Search | `POST /memories/search` | Semantic vector search + metadata filters |
| Batch Create | `POST /memories/batch` | Store multiple entries atomically |
| History | `GET /memories/:id/history` | Retrieve version history |
| Purge | `DELETE /memories/expired` | Remove expired short-term entries |
| Stats | `GET /memories/stats` | Namespace-level storage statistics |

### 3.3 Search Capabilities

The search endpoint supports hybrid retrieval:

1. **Vector search** — cosine similarity against stored embeddings
2. **Metadata filter** — exact match on tags, category, source, custom metadata fields
3. **Time range** — filter by created_at or updated_at windows
4. **Importance threshold** — minimum importance score
5. **Full-text keyword** — optional substring/keyword match on content
6. **Combined ranking** — weighted combination of vector similarity and importance score

Search request shape:
```
{
  "query": "string or embedding vector",
  "namespace": "required",
  "top_k": 10,
  "min_similarity": 0.7,
  "min_importance": 0.0,
  "filters": {
    "categories": ["fact", "preference"],
    "tags": ["language"],
    "source": "agent:chat",
    "created_after": "2026-01-01T00:00:00Z",
    "metadata": { "project": "zima-blue" }
  }
}
```

### 3.4 Embedding Generation

- The service can auto-generate embeddings using a configurable embedding provider
- Supported providers: local (built-in lightweight model), OpenAI-compatible API, Ollama
- Clients may also supply pre-computed embeddings directly
- Default embedding dimension: 384 (configurable per namespace)
- Embedding model is configurable at the namespace level

### 3.5 Memory Lifecycle

```
Created → Active → [Updated (new version)] → [Expired | Deleted]
                                                    ↓
                                                 Purged
```

- **Active**: retrievable via search and list
- **Expired**: TTL exceeded; excluded from search, pending purge
- **Deleted**: soft-deleted; excluded from search, retained for audit
- **Purged**: permanently removed from storage

Automatic purge runs on a configurable schedule (default: every 6 hours).

### 3.6 Namespace Management

| Operation | Description |
|-----------|-------------|
| Create namespace | Initialize with config (embedding dim, default TTL, quota) |
| List namespaces | Enumerate all namespaces with stats |
| Delete namespace | Remove all entries in a namespace |
| Namespace config | Per-namespace settings (embedding model, retention, quotas) |

---

## 4. Non-Functional Requirements

### 4.1 Performance

| Metric | Target |
|--------|--------|
| Write latency (single entry) | < 10ms (excluding embedding generation) |
| Read latency (by ID) | < 5ms |
| Vector search latency (10K entries) | < 50ms |
| Vector search latency (100K entries) | < 200ms |
| Batch write throughput | > 500 entries/sec |
| Concurrent namespace support | > 100 |

### 4.2 Storage

- Default backend: embedded (SQLite + flat-file vectors)
- Optional backends: PostgreSQL + pgvector, custom KV stores
- Storage quota per namespace: configurable (default: 100MB)
- Maximum entry content size: 64KB
- Maximum entries per namespace: configurable (default: 100,000)

### 4.3 Reliability

- All writes are durable (fsync before acknowledgment)
- Crash recovery via WAL (write-ahead log)
- Backup/restore via snapshot export (JSON or binary)
- No data loss on unclean shutdown

### 4.4 Compatibility

- Go 1.22+
- Linux, macOS, Windows (amd64, arm64)
- HTTP/1.1 and HTTP/2
- gRPC with reflection enabled
- OpenAPI 3.0 spec auto-generated

---

## 5. API Principles

1. **Namespace-scoped** — every request requires a namespace header or path parameter
2. **Idempotent writes** — PUT with the same content produces no new version
3. **Pagination** — all list endpoints use cursor-based pagination
4. **Consistent error format** — `{ "error": { "code": "...", "message": "..." } }`
5. **Versioned API** — `/api/v1/memories/...`
6. **Content negotiation** — JSON by default, Protobuf for gRPC
7. **Rate limiting** — per-namespace, configurable
8. **Request tracing** — X-Request-ID propagation

---

## 6. Security & Privacy Considerations

### 6.1 Data Protection

- Memory content may contain sensitive user information
- All data at rest encrypted using AES-256-GCM (key derived from user-provided passphrase or system key)
- API keys or bearer tokens for authentication
- Namespace isolation enforced at the storage layer, not just the API layer

### 6.2 Access Control

- Per-namespace API keys with scoped permissions (read, write, admin)
- Optional: cross-namespace read grants for shared knowledge bases
- Audit log for all write operations (who, when, what changed)

### 6.3 Data Retention

- Configurable retention policies per namespace
- Automatic purge of expired entries
- Manual purge API for compliance (GDPR right-to-erasure)
- Export API for data portability

### 6.4 Embedding Privacy

- When using local embedding models, no data leaves the device
- When using external embedding APIs, content is sent to the configured provider
- Configuration clearly documents which mode is active

---

## 7. Extensibility Strategy

### 7.1 Pluggable Storage Backends

```go
type VectorStore interface {
    Insert(ctx context.Context, id string, embedding []float32, metadata map[string]any) error
    Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]SearchResult, error)
    Delete(ctx context.Context, id string) error
    Count(ctx context.Context) (int64, error)
}

type KVStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string, cursor string, limit int) ([]KVEntry, string, error)
    Close() error
}
```

Built-in implementations:
- `flatfile` — JSON files + brute-force vector search (development/small scale)
- `sqlite` — SQLite with custom vector extension (default production)
- `pgvector` — PostgreSQL with pgvector (optional, for large deployments)

### 7.2 Pluggable Embedding Providers

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
    ModelName() string
}
```

Built-in implementations:
- `local` — lightweight model bundled with the binary (e.g., all-MiniLM-L6-v2 via ONNX)
- `openai` — OpenAI-compatible embedding API
- `ollama` — Local Ollama instance

### 7.3 Event Hooks

```go
type EventHook interface {
    OnMemoryCreated(ctx context.Context, entry *MemoryEntry) error
    OnMemoryUpdated(ctx context.Context, prev, updated *MemoryEntry) error
    OnMemoryDeleted(ctx context.Context, entry *MemoryEntry) error
    OnSearchPerformed(ctx context.Context, query *SearchQuery, results []SearchResult) error
}
```

Hooks enable external integrations: logging, analytics, replication, cache invalidation.

---

## 8. Implementation Checklist

### Phase 1: Core Domain (P0)

#### 8.1 Module Layer

- [ ] 1.1 Define `MemoryEntry` struct with all fields from Section 3.1
- [ ] 1.2 Define `Namespace` struct with config, quotas, stats
- [ ] 1.3 Define `SearchQuery` and `SearchResult` types
- [ ] 1.4 Define `MemoryStatus` enum (`active`, `expired`, `deleted`)
- [ ] 1.5 Implement ULID generation for entry IDs
- [ ] 1.6 Implement TTL → `expires_at` computation
- [ ] 1.7 Implement version chain logic (parent_id linking)

#### 8.2 Interface Layer

- [ ] 2.1 Define `VectorStore` interface (Insert, Search, Delete, Count)
- [ ] 2.2 Define `KVStore` interface (Get, Set, Delete, List, Close)
- [ ] 2.3 Define `EmbeddingProvider` interface (Embed, Dimension, ModelName)
- [ ] 2.4 Define `EventHook` interface (OnCreated, OnUpdated, OnDeleted, OnSearch)
- [ ] 2.5 Define `MemoryRepository` interface (CRUD + Search + History + Stats)
- [ ] 2.6 Define `NamespaceRepository` interface (Create, Get, List, Delete, UpdateConfig)

#### 8.3 Storage Layer

- [ ] 3.1 Implement `sqlite` KVStore backend (default)
- [ ] 3.2 Implement `sqlite` VectorStore backend with brute-force cosine similarity
- [ ] 3.3 Implement `flatfile` KVStore backend (JSON files, for development)
- [ ] 3.4 Implement `flatfile` VectorStore backend (in-memory vectors)
- [ ] 3.5 Implement `MemoryRepository` using KVStore + VectorStore composition
- [ ] 3.6 Implement `NamespaceRepository` using KVStore
- [ ] 3.7 Implement WAL for crash recovery
- [ ] 3.8 Implement storage encryption (AES-256-GCM wrapper around KVStore)
- [ ] 3.9 *(Optional)* Implement `pgvector` VectorStore backend

### Phase 2: Service Layer (P0)

- [ ] 4.1 Implement `MemoryService` — orchestrates repository, embedding, hooks
- [ ] 4.2 Implement Create flow: validate → embed (if needed) → store → hook
- [ ] 4.3 Implement Update flow: load existing → version chain → embed → store → hook
- [ ] 4.4 Implement Delete flow: soft-delete → hook
- [ ] 4.5 Implement Search flow: embed query → vector search → metadata filter → rank → return
- [ ] 4.6 Implement List flow: filter → paginate → return
- [ ] 4.7 Implement History flow: walk version chain → return ordered
- [ ] 4.8 Implement Purge flow: scan expired → hard delete → hook
- [ ] 4.9 Implement Stats aggregation (count, size, category breakdown)
- [ ] 4.10 Implement `NamespaceService` — namespace CRUD, quota enforcement
- [ ] 4.11 Implement background purge scheduler (configurable interval)

### Phase 3: Embedding Layer (P1)

- [ ] 5.1 Implement `local` EmbeddingProvider (ONNX runtime, all-MiniLM-L6-v2 or equivalent)
- [ ] 5.2 Implement `openai` EmbeddingProvider (OpenAI-compatible API client)
- [ ] 5.3 Implement `ollama` EmbeddingProvider (Ollama API client)
- [ ] 5.4 Implement embedding cache (avoid re-embedding identical content)
- [ ] 5.5 Implement batch embedding with configurable concurrency

### Phase 4: API Layer (P0)

#### HTTP (Echo)

- [ ] 6.1 `POST /api/v1/memories` — Create entry
- [ ] 6.2 `GET /api/v1/memories/:id` — Get entry
- [ ] 6.3 `PUT /api/v1/memories/:id` — Update entry (new version)
- [ ] 6.4 `DELETE /api/v1/memories/:id` — Soft-delete entry
- [ ] 6.5 `GET /api/v1/memories` — List entries (cursor pagination, filters)
- [ ] 6.6 `POST /api/v1/memories/search` — Semantic search
- [ ] 6.7 `POST /api/v1/memories/batch` — Batch create
- [ ] 6.8 `GET /api/v1/memories/:id/history` — Version history
- [ ] 6.9 `DELETE /api/v1/memories/expired` — Purge expired
- [ ] 6.10 `GET /api/v1/memories/stats` — Namespace stats
- [ ] 6.11 `POST /api/v1/namespaces` — Create namespace
- [ ] 6.12 `GET /api/v1/namespaces` — List namespaces
- [ ] 6.13 `DELETE /api/v1/namespaces/:id` — Delete namespace
- [ ] 6.14 Middleware: namespace extraction (header or path)
- [ ] 6.15 Middleware: authentication (API key / bearer token)
- [ ] 6.16 Middleware: rate limiting (per-namespace)
- [ ] 6.17 Middleware: request ID propagation

#### gRPC

- [ ] 7.1 Define `memory.proto` — service definition, message types
- [ ] 7.2 Generate Go code from proto
- [ ] 7.3 Implement gRPC server wrapping MemoryService
- [ ] 7.4 Enable gRPC reflection

### Phase 5: Configuration & Bootstrap (P1)

- [ ] 8.1 Define `MemoryConfig` struct (storage backend, embedding provider, purge interval, encryption)
- [ ] 8.2 Wire into existing ZimaOS-Blue config system
- [ ] 8.3 Implement service initialization and graceful shutdown
- [ ] 8.4 Register routes in existing bootstrap/routes.go

### Phase 6: Testing (P0)

- [ ] 9.1 Unit tests for `MemoryEntry` domain logic (ULID, TTL, versioning)
- [ ] 9.2 Unit tests for each `VectorStore` implementation
- [ ] 9.3 Unit tests for each `KVStore` implementation
- [ ] 9.4 Unit tests for `MemoryService` (mock repository + mock embedder)
- [ ] 9.5 Unit tests for `NamespaceService`
- [ ] 9.6 Integration tests for HTTP API endpoints (httptest)
- [ ] 9.7 Integration tests for gRPC endpoints
- [ ] 9.8 Integration tests for search accuracy (known embeddings, verify ranking)
- [ ] 9.9 Benchmark tests for vector search at 1K, 10K, 100K entries
- [ ] 9.10 Benchmark tests for write throughput
- [ ] 9.11 Test encryption round-trip (encrypt → store → load → decrypt)
- [ ] 9.12 Test purge scheduler (mock clock, verify expired entries removed)
- [ ] 9.13 Test namespace isolation (cross-namespace access denied)

### Phase 7: Observability (P1)

- [ ] 10.1 Structured logging (slog) for all service operations
- [ ] 10.2 Prometheus metrics: request count, latency histogram, storage size gauge
- [ ] 10.3 Health check endpoint (`GET /api/v1/memories/health`)
- [ ] 10.4 Request tracing via X-Request-ID

### Phase 8: Documentation (P2)

- [ ] 11.1 OpenAPI 3.0 spec (auto-generated from Echo routes)
- [ ] 11.2 gRPC service documentation
- [ ] 11.3 Configuration reference
- [ ] 11.4 Storage backend comparison guide
- [ ] 11.5 Embedding provider setup guide

---

## 9. Repository Layout

```
server/internal/memory/
├── memory.go                  # MemoryEntry, Namespace, SearchQuery types
├── service.go                 # MemoryService (core business logic)
├── namespace.go               # NamespaceService
├── handler.go                 # HTTP handlers (Echo)
├── handler_test.go            # HTTP handler integration tests
├── config.go                  # MemoryConfig
│
├── store/                     # Storage interfaces and implementations
│   ├── interfaces.go          # VectorStore, KVStore interfaces
│   ├── sqlite/
│   │   ├── kv.go              # SQLite KVStore implementation
│   │   ├── kv_test.go
│   │   ├── vector.go          # SQLite VectorStore (brute-force cosine)
│   │   ├── vector_test.go
│   │   └── migrations.go      # Schema migrations
│   ├── flatfile/
│   │   ├── kv.go              # JSON file KVStore
│   │   ├── kv_test.go
│   │   ├── vector.go          # In-memory VectorStore
│   │   └── vector_test.go
│   └── pgvector/              # Optional PostgreSQL backend
│       ├── kv.go
│       ├── vector.go
│       └── vector_test.go
│
├── repository.go              # MemoryRepository (composes KV + Vector)
├── repository_test.go
│
├── embedding/                 # Embedding provider interface and implementations
│   ├── provider.go            # EmbeddingProvider interface
│   ├── local.go               # ONNX-based local embedder
│   ├── local_test.go
│   ├── openai.go              # OpenAI-compatible API embedder
│   ├── openai_test.go
│   ├── ollama.go              # Ollama embedder
│   ├── ollama_test.go
│   └── cache.go               # Embedding cache (content hash → vector)
│
├── encryption/                # At-rest encryption
│   ├── aes.go                 # AES-256-GCM encrypt/decrypt
│   └── aes_test.go
│
├── hooks/                     # Event hook system
│   ├── hooks.go               # EventHook interface, dispatcher
│   └── hooks_test.go
│
├── purge/                     # Background purge scheduler
│   ├── scheduler.go
│   └── scheduler_test.go
│
├── proto/                     # gRPC definitions
│   ├── memory.proto
│   └── memory_grpc.pb.go      # Generated
│
└── benchmark/                 # Benchmark suite
    ├── bench_test.go          # Go benchmarks (search, write)
    └── token_reduction/       # Token reduction benchmark
        ├── runner.go
        ├── workloads.go
        └── report.go
```

---

## 10. Token Reduction Benchmark Suite

### 10.1 Objective

Measure how much memory-augmented agents reduce token consumption compared to agents without persistent memory. This quantifies the practical value of the memory service.

### 10.2 Methodology

**A/B comparison** across identical task sequences:

| Condition | Description |
|-----------|-------------|
| **Baseline** (no memory) | Agent starts each session with zero context. All information must be re-established via conversation. |
| **Memory-augmented** | Agent pre-loads relevant memories at session start and writes new learnings during the session. |

Each condition runs the same sequence of tasks. We measure total tokens consumed (input + output) across all sessions.

### 10.3 Workload Definitions

#### W1: Repeated Preference Recall (5 sessions)
- Session 1: User states 8 preferences (language, framework, style, etc.)
- Sessions 2–5: Agent must apply those preferences to new tasks
- Measures: how many tokens spent re-establishing preferences

#### W2: Multi-Session Project (10 sessions)
- Simulates a 10-session software project (design → implement → test → deploy)
- Each session builds on prior decisions
- Measures: context re-establishment overhead

#### W3: Fact Accumulation (20 sessions)
- Each session introduces 3 new facts about a domain
- Later sessions require recalling facts from earlier sessions
- Measures: scaling behavior as knowledge grows

#### W4: Mixed Workload (15 sessions)
- Combination of preference recall, project continuity, and fact retrieval
- Includes sessions with no memory relevance (control)
- Measures: real-world-like usage pattern

### 10.4 Metrics

| Metric | Formula | Unit |
|--------|---------|------|
| Total Input Tokens | Sum of all prompt tokens across sessions | tokens |
| Total Output Tokens | Sum of all completion tokens across sessions | tokens |
| Total Tokens | Input + Output | tokens |
| Token Reduction Ratio | `1 - (memory_total / baseline_total)` | percentage |
| Memory Retrieval Accuracy | Correct recalls / Total recall attempts | percentage |
| Memory Write Overhead | Tokens spent on memory write operations | tokens |
| Net Token Savings | Baseline total - Memory total - Write overhead | tokens |
| Latency Overhead | Additional time for memory operations per session | ms |

### 10.5 Data Collection Process

1. **Prepare workloads** — define exact task scripts with expected outputs
2. **Run baseline** — execute all workloads without memory service; log all API calls and token counts
3. **Run memory-augmented** — execute same workloads with memory service enabled; log all API calls, token counts, and memory operations
4. **Extract metrics** — parse logs, compute per-session and aggregate metrics
5. **Generate report** — markdown report with tables, per-workload breakdown, and summary

All runs use the same LLM model, temperature=0, and fixed random seeds for reproducibility.

### 10.6 Pass/Good/Excellent Thresholds

| Rating | Token Reduction | Retrieval Accuracy | Net Savings |
|--------|----------------|-------------------|-------------|
| **Pass** | ≥ 15% | ≥ 80% | Positive (savings > overhead) |
| **Good** | ≥ 30% | ≥ 90% | ≥ 20% net reduction |
| **Excellent** | ≥ 50% | ≥ 95% | ≥ 40% net reduction |

Additional quality gates:
- Memory write latency must not exceed 100ms p99
- Search latency must not exceed 200ms p99
- No memory corruption or data loss across all workloads

### 10.7 Benchmark Script Outline

```go
// benchmark/token_reduction/runner.go

type BenchmarkRunner struct {
    LLMClient      LLMClient           // Calls the LLM API
    MemoryService  *memory.MemoryService // nil for baseline runs
    Workloads      []Workload
    Results        []SessionResult
}

type Workload struct {
    Name     string
    Sessions []Session
}

type Session struct {
    ID       string
    Tasks    []Task           // Scripted user messages
    Expected []string         // Expected key facts in responses
}

type Task struct {
    UserMessage string
    RequiresMemory bool       // Whether this task benefits from memory
}

type SessionResult struct {
    WorkloadName    string
    SessionID       string
    InputTokens     int
    OutputTokens    int
    MemoryReads     int
    MemoryWrites    int
    MemoryLatencyMs float64
    RecallHits      int       // Correct memory recalls
    RecallMisses    int       // Failed memory recalls
}

// Run executes all workloads and collects results
func (r *BenchmarkRunner) Run(ctx context.Context) error {
    for _, w := range r.Workloads {
        for _, s := range w.Sessions {
            result := r.runSession(ctx, w.Name, s)
            r.Results = append(r.Results, result)
        }
    }
    return nil
}

// Report generates a markdown report from collected results
func (r *BenchmarkRunner) Report() string {
    // Compute per-workload and aggregate metrics
    // Format as markdown tables
    // Include pass/good/excellent rating
}
```

```bash
#!/bin/bash
# scripts/run-token-benchmark.sh

# Step 1: Run baseline (no memory)
go run ./benchmark/token_reduction/cmd \
  --mode=baseline \
  --model=claude-sonnet-4-5-20250929 \
  --output=results/baseline.json

# Step 2: Run memory-augmented
go run ./benchmark/token_reduction/cmd \
  --mode=memory \
  --model=claude-sonnet-4-5-20250929 \
  --memory-backend=sqlite \
  --output=results/memory.json

# Step 3: Generate comparison report
go run ./benchmark/token_reduction/cmd \
  --mode=report \
  --baseline=results/baseline.json \
  --memory=results/memory.json \
  --output=results/report.md
```

---

## Self-Review Checklist

- [x] No external project names mentioned
- [x] All naming is generic and descriptive
- [x] Architecture follows standard patterns (repository, service, handler layers)
- [x] No code copied from any existing implementation
- [x] All interfaces designed from first principles based on product goals
- [x] Benchmark methodology is independent and reproducible
