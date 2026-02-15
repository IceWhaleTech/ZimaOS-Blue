# PRD: Unified Pruning Module — Code & Non-Code Context Compression

**Version**: 0.10.27-1
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-02-15
**Supersedes**: PRD-0.10.27-context-pruner.md (code-only pruner)

**References**:
- Paper: [SWE-Pruner: Self-Adaptive Context Pruning for Coding Agents](https://arxiv.org/abs/2601.16746)
- Prior Implementation: `server/internal/pruner/` (v0.10.27 — code-only, BM25 + TF-IDF)
- Model: [ayanami-kitasan/code-pruner](https://huggingface.co/ayanami-kitasan/code-pruner) (Qwen3-Reranker-0.6B)

---

## Phase 0 — Scope & Applicability

### 0.1 Overview

The pruning module reduces context size sent to the LLM by removing irrelevant content from tool outputs. Unlike v0.10.27 (code-only), this version supports **both code and non-code content** with strategy selection per scenario.

### 0.2 Applicability by Scenario

| Scenario | Recommended Pruner | Rationale |
|----------|--------------------|-----------|
| Code patch / Bug fix / Feature implementation | CodePruner (SWE-Pruner neural model) | Identifies code lines causally relevant to modifications. High precision for coding tasks. |
| General code understanding / code search | IRPruner (BM25 / TF-IDF) | Lightweight lexical-semantic scoring is sufficient. Neural pruner optional. |
| Non-code documents (text, logs, docs, KB) | IRPruner (BM25 / TF-IDF) | Neural code pruner is ineffective on prose. IR methods are fast, interpretable, low-latency. |
| RAG / Knowledge retrieval / Agent context | IRPruner (BM25 / TF-IDF) | Reliable token reduction without adding latency. |

### 0.3 Key Principles

1. **Code vs Non-Code Differentiation**: Neural pruners like SWE-Pruner are vertical models effective only on coding scenarios. Non-code content must use IR-based methods.
2. **Unified Architecture**: Single pruning API automatically selects strategy:
   - Default: **IRPruner** (BM25 + TF-IDF, CPU-only, deterministic)
   - Optional: **CodePruner** (neural, coding tasks only, GPU recommended)
3. **Performance Guarantees**:
   - IRPruner: ≤20ms per 10k LOC equivalent, CPU-only
   - CodePruner: variable latency, GPU recommended
4. **Extensibility**: Future scenario-specific pruners can be added without affecting the IR baseline.

### 0.4 Implications

- Online agents / low-latency apps → IRPruner as primary engine
- Neural code pruner → optional, scenario-specific, disabled by default
- IRPruner ensures determinism, caching, and explainability
- Non-code content (logs, docs, knowledge base) always uses IRPruner

---

## Phase 1 — Product Requirements Document

### 1.1 Background & Motivation

LLM-based agent systems accumulate large context windows through tool outputs — file reads, search results, logs, documentation. The SWE-Pruner paper shows that read operations account for **76% of total tokens** in typical SWE-bench tasks.

**Why neural pruners are insufficient as the sole solution:**
- Neural models (SWE-Pruner, 0.6B params) add 50–200ms latency per request
- They are trained on code and perform poorly on non-code content (logs, docs, prose)
- GPU dependency increases infrastructure cost
- Non-deterministic outputs complicate caching and debugging

**Why IR-based pruning is preferred for general content:**
- BM25/TF-IDF scoring is deterministic, cacheable, and explainable
- Sub-20ms latency on 10k LOC equivalent text, CPU-only
- Effective on both code and non-code content
- Zero external dependencies — compiles into the Go binary
- Rule-based boosting can approximate 80–90% of neural pruning quality on code

### 1.2 Goals

| ID | Goal | Target |
|----|------|--------|
| G-001 | Reduce tokens sent to LLM | ≥50% on code, ≥40% on non-code |
| G-002 | Added latency per request (IRPruner) | ≤20ms on 10k LOC equivalent |
| G-003 | Recall@K vs full-context baseline | ≥0.85 |
| G-004 | Memory footprint (IRPruner) | <50MB for scorer + cache |
| G-005 | Support both code and non-code content | Automatic detection + strategy selection |
| G-006 | Unified API | Single `Backend.Prune()` interface for all content types |
| G-007 | Optional neural code pruner | CodePruner disabled by default, opt-in for coding tasks |

### 1.3 Non-Goals

1. Deep semantic reasoning outside coding (e.g., summarization, paraphrasing)
2. GPU dependency for non-code pruning
3. Training or fine-tuning neural models
4. Replacing the existing CCCache — pruning is complementary
5. Modifying the agent's tool-calling behavior

### 1.4 User Stories

**Story 1: Automatic Token Savings (Code)**
- **As a** developer using ZimaOS Blue as a coding agent proxy
- **I want** irrelevant code lines automatically pruned from tool outputs
- **So that** I spend fewer tokens on context that doesn't help the LLM solve my task
- **Acceptance**: ≥50% token reduction on code-heavy conversations, `(filtered N lines)` markers

**Story 2: Non-Code Document Pruning**
- **As a** user querying documentation or logs through the agent
- **I want** irrelevant paragraphs/sections pruned from large documents
- **So that** the LLM receives only the relevant portions, reducing cost and improving focus
- **Acceptance**: ≥40% token reduction on non-code content, paragraph-level granularity

**Story 3: Streaming & Non-Streaming Support**
- **As a** developer
- **I want** pruning to work in both streaming and non-streaming response paths
- **So that** I get consistent token savings regardless of request mode

**Story 4: Pruning Statistics**
- **As an** administrator
- **I want** to see pruning metrics (tokens saved, compression ratio, latency) in the dashboard
- **So that** I can quantify cost reduction

**Story 5: Configurable Strategy**
- **As an** administrator
- **I want** to choose between IRPruner (default) and CodePruner (optional) per scenario
- **So that** I can balance latency vs accuracy for different workloads

### 1.5 Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Token reduction (code) | ≥50% | Benchmark on synthetic 1k/5k/10k LOC files |
| Token reduction (non-code) | ≥40% | Benchmark on synthetic docs/logs |
| Recall@K (code) | ≥0.85 | Against full-context baseline |
| Recall@K (non-code) | ≥0.80 | Against full-context baseline |
| IRPruner latency (10k LOC) | ≤20ms | Benchmark on Apple M2 |
| Memory footprint | <50MB | Runtime profiling |
| Cache hit rate | >60% | Multi-turn conversation simulation |
| Test coverage | >85% | `go test -cover` |

---

## Phase 2 — Architecture Design

### 2.1 Component Diagram

```
Client Request (with tool outputs)
        |
        v
   ProxyHandler
        |
        v
┌───────────────────────────────────────────────────────┐
│                  Unified Pruner Middleware              │
│                                                        │
│  ┌──────────────┐                                      │
│  │ ContentDetect │  Classify: code vs non-code          │
│  └──────┬───────┘                                      │
│         v                                              │
│  ┌──────────────┐    ┌──────────────┐                  │
│  │  Segmenter    │    │  Paragraph   │                  │
│  │  (code)       │    │  Splitter    │                  │
│  │  func/class/  │    │  (non-code)  │                  │
│  │  block/lines  │    │  para/sent/  │                  │
│  └──────┬───────┘    │  heading     │                  │
│         │            └──────┬───────┘                  │
│         └───────┬───────────┘                          │
│                 v                                      │
│  ┌──────────────────────┐                              │
│  │  Tokenizer &          │  camelCase/snake_case split  │
│  │  Normalizer           │  lowercase, stopword removal │
│  └──────────┬───────────┘                              │
│             v                                          │
│  ┌──────────────────────┐                              │
│  │  IR Scorer            │  BM25 (default) or TF-IDF   │
│  │  (IRPruner)           │  k1=1.2, b=0.75             │
│  └──────────┬───────────┘                              │
│             v                                          │
│  ┌──────────────────────┐                              │
│  │  Rule-Based Weights   │  Structural boost, import    │
│  │                       │  boost, name-match boost     │
│  └──────────┬───────────┘                              │
│             v                                          │
│  ┌──────────────────────┐                              │
│  │  Top-K Selector       │  Token budget constraint     │
│  └──────────┬───────────┘                              │
│             v                                          │
│  ┌──────────────────────┐                              │
│  │  Cache Layer          │  LRU in-memory + disk        │
│  └──────────┬───────────┘                              │
│             v                                          │
│  ┌──────────────────────┐  (optional, disabled default) │
│  │  CodePruner Hook      │  Neural SWE-Pruner for code  │
│  │  (second-stage)       │  POST /prune to external svc │
│  └──────────┬───────────┘                              │
│             v                                          │
│  ┌──────────────────────┐                              │
│  │  Output Builder       │  Reassemble + filtered       │
│  │                       │  markers + metrics           │
│  └──────────────────────┘                              │
└───────────────────────────────────────────────────────┘
        |
        v
   Route to Provider (reduced token count)
        |
        v
   Response → Client
```

### 2.2 Data Flow

```
1. Request arrives at ProxyHandler
2. Middleware extracts tool-output messages
3. ContentDetector classifies each message:
   - Code → Segmenter (function/class/block/lines)
   - Non-code → ParagraphSplitter (paragraph/sentence/heading)
4. Segments are tokenized and normalized
5. IR Scorer (BM25) scores segments against query
6. Rule-based weights boost structural/import/name-match segments
7. Top-K selector picks segments within token budget
8. Cache stores scored results (LRU in-memory, optional disk)
9. [Optional] CodePruner neural hook for coding tasks
10. Output builder reassembles content with filtered markers
11. Metrics recorded (tokens saved, latency, compression ratio)
```

### 2.3 Segmentation Strategy

| Content Type | Segmentation | Granularity |
|-------------|-------------|-------------|
| Code (Go, Python, JS/TS, Rust, Java) | Function / Class / Block / Lines | Regex-based boundary detection, brace-depth / indentation tracking |
| Markdown / Documentation | Heading / Paragraph | Split on `##` headings, double newlines |
| Logs | Line groups | Group by timestamp pattern or blank-line separation |
| Plain text | Paragraph / Sentence | Split on double newlines, fallback to sentence boundaries |
| JSON / YAML | Top-level keys | Split on top-level object/array boundaries |

### 2.4 Scoring Pipeline

```
Segments → Tokenize → BM25 Score → Rule Boost → Top-K Select → Output
                                      │
                                      ├── Structural: func 1.3x, class 1.25x, block 1.1x
                                      ├── Import: 1.15x for import/require/include
                                      ├── Name-match: 1.5x when segment name matches query
                                      ├── Position: 1.1x for first/last 10% of content
                                      └── Heading: 1.2x for section headings (non-code)
```

### 2.5 Cache Architecture

```
┌─────────────────────────────────────┐
│         Cache Layer                  │
│                                      │
│  ┌─────────────┐  ┌──────────────┐  │
│  │ LRU Memory  │  │ Disk Cache   │  │
│  │ (256 slots) │  │ (optional)   │  │
│  │ SHA256 key  │  │ gob-encoded  │  │
│  │ O(1) lookup │  │ file-backed  │  │
│  └─────────────┘  └──────────────┘  │
│                                      │
│  Key = SHA256(content + query)[:16]  │
│  TTL = configurable (default 5min)   │
│  Eviction = LRU                      │
└─────────────────────────────────────┘
```

### 2.6 Package Structure

```
server/internal/pruner/
├── pruner.go              // Backend interface, Config, factory, ContentType enum
├── detector.go            // ContentDetector: code vs non-code classification
├── segmenter.go           // Code segmenter (function/class/block/lines)
├── paragraph.go           // Non-code segmenter (paragraph/heading/sentence)
├── tokenizer.go           // Unified tokenizer: camelCase/snake_case split, stopwords
├── scorer.go              // LocalBackend: TF-IDF + structural line scoring
├── bm25.go                // BM25Scorer, Scorer interface, ScoredSegment types
├── pipeline.go            // IRPruner pipeline: BM25 → Boost → TopK → Output
├── cache.go               // LRU in-memory cache
├── cache_disk.go          // Optional disk persistence for cache
├── remote_backend.go      // CodePruner: HTTP client for SWE-Pruner neural service
├── middleware.go          // Proxy middleware integration
├── token_estimator.go     // Token count estimation (EN/CJK)
├── metrics.go             // Pruning statistics (atomic counters)
├── handler.go             // API handler (/stats, /config, /health)
└── *_test.go              // Tests for each component
```

---

## Phase 3 — Data Structures & APIs

### 3.1 Content Classification

```go
// ContentType identifies the type of content being pruned.
type ContentType int

const (
    ContentCode    ContentType = iota // Source code (Go, Python, JS, etc.)
    ContentDoc                        // Documentation, markdown, prose
    ContentLog                        // Log output, structured text
    ContentData                       // JSON, YAML, structured data
    ContentUnknown                    // Fallback — treated as non-code
)

// DetectContentType classifies content and returns the detected type.
func DetectContentType(content string, minLines int) ContentType
```

### 3.2 Unified Segment Types

```go
// SegmentKind identifies the granularity of a segment.
type SegmentKind int

const (
    // Code segments
    SegmentFunction SegmentKind = iota
    SegmentClass
    SegmentBlock
    SegmentLines

    // Non-code segments
    SegmentParagraph
    SegmentHeading
    SegmentSentence
    SegmentLogGroup
    SegmentDataKey
)

// Segment represents a content unit (function, paragraph, heading, etc.).
type Segment struct {
    StartLine int         `json:"start_line"`
    EndLine   int         `json:"end_line"`
    Kind      SegmentKind `json:"kind"`
    Name      string      `json:"name"`      // e.g., "func main", "## Section Title"
    Content   string      `json:"content"`
    Tokens    []string    `json:"-"`          // pre-tokenized (not serialized)
}

// ScoredSegment pairs a segment with its relevance score.
type ScoredSegment struct {
    Segment Segment `json:"segment"`
    Score   float64 `json:"score"`
}
```

### 3.3 Scorer Interface

```go
// Scorer is the pluggable scoring interface.
type Scorer interface {
    // Score computes relevance scores for segments against a query.
    Score(query string, segments []Segment) []ScoredSegment
}

// BM25Scorer implements Scorer using Okapi BM25.
type BM25Scorer struct {
    K1 float64 // Term frequency saturation (default 1.2)
    B  float64 // Length normalization (default 0.75)
}

func NewBM25Scorer(k1, b float64) *BM25Scorer
func (s *BM25Scorer) Score(query string, segments []Segment) []ScoredSegment
```

### 3.4 Backend Interface

```go
// Backend defines the pruning backend interface.
type Backend interface {
    // Prune scores and removes low-relevance content.
    Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error)
    // Health returns nil if the backend is available.
    Health(ctx context.Context) error
    // Close releases backend resources.
    Close() error
}

// PruneRequest is the input to the pruning engine.
type PruneRequest struct {
    Content     string      `json:"content"`               // Raw content (code or text)
    Query       string      `json:"query,omitempty"`        // What the agent is looking for
    Threshold   float64     `json:"threshold,omitempty"`    // Score threshold (0.0–1.0)
    ContentType ContentType `json:"content_type,omitempty"` // Hint (auto-detected if 0)
}

// PruneResponse is the output from the pruning engine.
type PruneResponse struct {
    PrunedContent   string      `json:"pruned_content"`
    ContentType     ContentType `json:"content_type"`
    Score           float64     `json:"score"`
    OriginalLines   int         `json:"original_lines"`
    KeptLines       int         `json:"kept_lines"`
    PrunedLines     int         `json:"pruned_lines"`
    OriginalTokens  int         `json:"original_tokens"`
    PrunedTokens    int         `json:"pruned_tokens"`
    CompressionRate float64     `json:"compression_rate"`
    LatencyMs       float64     `json:"latency_ms"`
}
```

### 3.5 IRPruner (Primary Backend)

```go
// IRPruner implements Backend using BM25 scoring with unified segmentation.
// Handles both code and non-code content.
type IRPruner struct {
    config Config
    scorer *BM25Scorer
    cache  *LRUScoreCache
}

func NewIRPruner(cfg Config) *IRPruner
func (p *IRPruner) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error)
func (p *IRPruner) Health(ctx context.Context) error
func (p *IRPruner) Close() error
```

### 3.6 CodePruner (Optional Neural Backend)

```go
// CodePruner implements Backend by calling an external SWE-Pruner neural service.
// Only effective for coding tasks. Disabled by default.
type CodePruner struct {
    client  *http.Client
    baseURL string
}

func NewCodePruner(url string, client *http.Client) *CodePruner
func (c *CodePruner) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error)
func (c *CodePruner) Health(ctx context.Context) error
func (c *CodePruner) Close() error
```

### 3.7 Cache Interface

```go
// ScoreCache provides caching for scored results.
type ScoreCache interface {
    Get(key string) ([]ScoredSegment, bool)
    Put(key string, segments []ScoredSegment)
}

// LRUScoreCache is a thread-safe LRU cache for scored segments.
type LRUScoreCache struct {
    mu       sync.Mutex
    capacity int
    items    map[string]*list.Element
    order    *list.List
}

func NewScoreCache(capacity int) *LRUScoreCache
func CacheKey(content, query string) string // SHA256[:16]

// DiskCache provides optional disk persistence.
type DiskCache struct {
    dir string
    ttl time.Duration
}

func NewDiskCache(dir string, ttl time.Duration) *DiskCache
func (d *DiskCache) Get(key string) ([]ScoredSegment, bool)
func (d *DiskCache) Put(key string, segments []ScoredSegment)
```

### 3.8 Segmentation APIs

```go
// Code segmentation
func Segmentize(code string) []Segment                    // Multi-language code segmenter
func SegmentizeWithLang(code string, lang string) []Segment // Language-specific

// Non-code segmentation
func SegmentizeParagraphs(text string) []Segment           // Paragraph/heading splitter
func SegmentizeLogs(text string) []Segment                 // Log group splitter
func SegmentizeData(text string) []Segment                 // JSON/YAML key splitter

// Unified entry point
func AutoSegmentize(content string, ct ContentType) []Segment
```

### 3.9 Configuration

```go
// Config holds unified pruner configuration.
type Config struct {
    Enabled       bool    `mapstructure:"enabled"`
    Backend       string  `mapstructure:"backend"`        // "ir" (default), "code", "auto"
    RemoteURL     string  `mapstructure:"remote_url"`     // CodePruner service URL
    Threshold     float64 `mapstructure:"threshold"`      // 0.0–1.0, default 0.5
    MinLines      int     `mapstructure:"min_lines"`      // Minimum lines to trigger pruning
    TimeoutMs     int     `mapstructure:"timeout_ms"`     // Remote backend timeout
    CacheCapacity int     `mapstructure:"cache_capacity"` // LRU cache slots (default 256)
    DiskCacheDir  string  `mapstructure:"disk_cache_dir"` // Optional disk cache directory
    DiskCacheTTL  int     `mapstructure:"disk_cache_ttl"` // Disk cache TTL in seconds
}

func DefaultConfig() Config {
    return Config{
        Enabled:       false,
        Backend:       "ir",
        Threshold:     0.5,
        MinLines:      50,   // Lower than v0.10.27 (was 200) for non-code support
        TimeoutMs:     5000,
        CacheCapacity: 256,
    }
}
```

```yaml
# config.yaml
pruner:
  enabled: false
  backend: "ir"              # "ir" (default), "code" (neural), "auto" (detect + select)
  threshold: 0.5
  min_lines: 50
  cache_capacity: 256
  # disk_cache_dir: "/tmp/pruner-cache"  # Optional
  # disk_cache_ttl: 300                  # 5 minutes
  # remote_url: "http://localhost:8000"  # Only for backend=code
  # timeout_ms: 5000                     # Only for backend=code
```

### 3.10 Metrics

```go
// Stats tracks cumulative pruning metrics with atomic counters.
type Stats struct {
    TotalRequests       atomic.Int64
    PrunedRequests      atomic.Int64
    PassthroughRequests atomic.Int64
    CodeRequests        atomic.Int64   // Requests classified as code
    NonCodeRequests     atomic.Int64   // Requests classified as non-code
    TotalTokensBefore   atomic.Int64
    TotalTokensAfter    atomic.Int64
    TotalTokensSaved    atomic.Int64
    TotalLatencyUs      atomic.Int64
}

func (s *Stats) Record(resp *PruneResponse, ct ContentType)
func (s *Stats) Snapshot() StatsSnapshot
```

---

## Phase 4 — Implementation Checklist

### 4.1 Core Scoring (P0)

- [ ] 4.1.1 Refactor `BM25Scorer` to accept unified `Segment` types (code + non-code)
- [ ] 4.1.2 Implement unified `codeTokenize()` + `textTokenize()` in `tokenizer.go`
- [ ] 4.1.3 Add stopword list for non-code content (English common words)
- [ ] 4.1.4 Implement `ContentType` enum and `DetectContentType()` in `detector.go`
- [ ] 4.1.5 Add non-code detection heuristics (no code fences, low syntax density)

### 4.2 Indexing / Segmentation (P0)

- [ ] 4.2.1 Refactor `Segmenter` to support new `SegmentKind` values (Paragraph, Heading, etc.)
- [ ] 4.2.2 Implement `SegmentizeParagraphs()` — split on headings + double newlines
- [ ] 4.2.3 Implement `SegmentizeLogs()` — group by timestamp or blank-line separation
- [ ] 4.2.4 Implement `SegmentizeData()` — split JSON/YAML on top-level keys
- [ ] 4.2.5 Implement `AutoSegmentize()` — dispatch to correct segmenter by ContentType
- [ ] 4.2.6 Add heading boost (1.2x) for non-code heading segments

### 4.3 Caching (P0)

- [ ] 4.3.1 Existing LRU in-memory cache — verify works with new Segment types
- [ ] 4.3.2 Implement `DiskCache` with gob encoding + file-backed storage
- [ ] 4.3.3 Add TTL-based expiration for disk cache entries
- [ ] 4.3.4 Implement two-tier cache lookup: memory → disk → compute

### 4.4 Pruning Pipeline (P0)

- [ ] 4.4.1 Implement `IRPruner` backend wrapping BM25 + segmentation + cache
- [ ] 4.4.2 Wire `ContentType` detection into `IRPruner.Prune()`
- [ ] 4.4.3 Update `PruneRequest` to include `ContentType` field (auto-detect if unset)
- [ ] 4.4.4 Update `PruneResponse` to include `ContentType` field
- [ ] 4.4.5 Update `NewBackend()` factory: `"ir"` → IRPruner, `"code"` → CodePruner, `"auto"` → detect + select
- [ ] 4.4.6 Implement `"auto"` backend mode: detect content type, use IRPruner for non-code, CodePruner for code (if available, else fallback to IRPruner)

### 4.5 Non-Code Boost Rules (P1)

- [ ] 4.5.1 Add heading boost (1.2x) for `SegmentHeading` kind
- [ ] 4.5.2 Add position boost (1.1x) for first/last 10% of non-code content
- [ ] 4.5.3 Add keyword density boost for non-code segments matching query terms
- [ ] 4.5.4 Tune BM25 parameters for non-code content (potentially different k1/b)

### 4.6 Testing (P0)

- [ ] 4.6.1 Unit tests for `DetectContentType()` — code, markdown, logs, JSON, plain text
- [ ] 4.6.2 Unit tests for `SegmentizeParagraphs()` — headings, paragraphs, mixed
- [ ] 4.6.3 Unit tests for `SegmentizeLogs()` — timestamped, blank-separated
- [ ] 4.6.4 Unit tests for `SegmentizeData()` — JSON objects, YAML documents
- [ ] 4.6.5 Unit tests for `AutoSegmentize()` — dispatch correctness
- [ ] 4.6.6 Unit tests for `IRPruner.Prune()` — code content, non-code content, mixed
- [ ] 4.6.7 Unit tests for `DiskCache` — put/get, TTL expiration, concurrent access
- [ ] 4.6.8 Integration test: full pipeline code → detect → segment → score → prune
- [ ] 4.6.9 Integration test: full pipeline non-code → detect → segment → score → prune
- [ ] 4.6.10 Verify existing tests still pass (backward compatibility)

### 4.7 Benchmarking (P1)

- [ ] 4.7.1 Benchmark IRPruner on 1k/5k/10k LOC code files
- [ ] 4.7.2 Benchmark IRPruner on 1k/5k/10k line non-code documents
- [ ] 4.7.3 Recall@K computation for code content
- [ ] 4.7.4 Recall@K computation for non-code content
- [ ] 4.7.5 Token reduction measurement (code vs non-code)
- [ ] 4.7.6 Memory profiling with disk cache enabled
- [ ] 4.7.7 Cache hit rate measurement in simulated multi-turn conversations

---

## Phase 5 — Implementation Notes

### 5.1 Migration from v0.10.27

The existing `server/internal/pruner/` package (v0.10.27) provides the foundation. Key changes:

| Existing Component | Change Required |
|-------------------|-----------------|
| `pruner.go` (Backend, Config, factory) | Add `ContentType`, rename `Code` → `Content` in PruneRequest, add `"ir"` and `"auto"` backend modes |
| `bm25.go` (BM25Scorer, Segment types) | Add non-code SegmentKind values, no scoring changes needed |
| `segmenter.go` (code segmenter) | Keep as-is, add `SegmentizeWithLang()` variant |
| `pipeline.go` (BM25Backend) | Rename to `IRPruner`, add content-type-aware segmentation dispatch |
| `scorer.go` (LocalBackend) | Keep as legacy fallback, IRPruner supersedes |
| `cache.go` (LRU cache) | Keep as-is, works with new Segment types |
| `detector.go` (IsCodeContent) | Extend to `DetectContentType()` returning ContentType enum |
| `remote_backend.go` | Rename to CodePruner, keep HTTP client logic |
| `middleware.go` | Update to use unified PruneRequest with ContentType |

### 5.2 New Files

| File | Purpose |
|------|---------|
| `paragraph.go` | Non-code segmenter: paragraphs, headings, sentences |
| `tokenizer.go` | Unified tokenizer: `codeTokenize()` + `textTokenize()` + stopwords |
| `cache_disk.go` | Disk-backed cache with gob encoding and TTL |
| `ir_pruner.go` | IRPruner backend (replaces BM25Backend as primary) |

### 5.3 Backward Compatibility

- Existing `"local"` and `"bm25"` backend values continue to work (mapped to IRPruner)
- `PruneRequest.Code` field kept as alias for `Content` via JSON tag
- `"remote"` backend value mapped to CodePruner
- All existing tests must continue to pass

### 5.4 Key Implementation Decisions

1. **BM25 parameters**: k1=1.2, b=0.75 for code; k1=1.5, b=0.5 for non-code (longer documents benefit from less length normalization)
2. **Token budget**: `keepRatio = 1.0 - threshold` (threshold=0.5 → keep 50%)
3. **Minimum segment size**: 1 line for code, 1 sentence for non-code
4. **Disk cache format**: gob-encoded `[]ScoredSegment` with SHA256 filename
5. **Thread safety**: `sync.Mutex` for LRU cache, `sync.RWMutex` for disk cache directory listing

### 5.5 Constraints

- Go 1.22+, no ML frameworks for IRPruner
- No Redis, no GPU for non-code pruning
- Single-node, CPU-only, deterministic
- Favor slices over maps for memory locality
- Avoid reflection
- No external heavy dependencies

---

## Phase 6 — Validation & Evaluation Plan

### 6.1 Benchmark Harness

```go
// benchmark_test.go

func BenchmarkIRPruner_Code_1k(b *testing.B)   { benchIRPruner(b, generateGoCode(1000)) }
func BenchmarkIRPruner_Code_5k(b *testing.B)   { benchIRPruner(b, generateGoCode(5000)) }
func BenchmarkIRPruner_Code_10k(b *testing.B)  { benchIRPruner(b, generateGoCode(10000)) }
func BenchmarkIRPruner_Doc_1k(b *testing.B)    { benchIRPruner(b, generateMarkdown(1000)) }
func BenchmarkIRPruner_Doc_5k(b *testing.B)    { benchIRPruner(b, generateMarkdown(5000)) }
func BenchmarkIRPruner_Doc_10k(b *testing.B)   { benchIRPruner(b, generateMarkdown(10000)) }
func BenchmarkIRPruner_Log_10k(b *testing.B)   { benchIRPruner(b, generateLogOutput(10000)) }
```

### 6.2 Dataset Format

Test datasets are generated programmatically in `benchmark_test.go`:

| Generator | Content Type | Structure |
|-----------|-------------|-----------|
| `generateGoCode(n)` | Code | Realistic Go functions with imports, types, methods |
| `generateMarkdown(n)` | Documentation | Headings, paragraphs, code blocks, lists |
| `generateLogOutput(n)` | Logs | Timestamped entries with varying severity levels |
| `generateJSON(n)` | Data | Nested JSON objects with realistic keys/values |

### 6.3 Recall@K Computation

```go
// computeRecallAtK measures how many "relevant" segments are retained after pruning.
// Relevant segments are those containing query terms in the original content.
func computeRecallAtK(original, pruned, query string) float64 {
    relevantLines := findRelevantLines(original, query)
    retainedLines := findRetainedLines(pruned, relevantLines)
    if len(relevantLines) == 0 {
        return 1.0
    }
    return float64(len(retainedLines)) / float64(len(relevantLines))
}
```

### 6.4 Token Reduction Calculation

```go
func measureTokenReduction(original, pruned string) float64 {
    origTokens := EstimateTokens(original)
    prunedTokens := EstimateTokens(pruned)
    if origTokens == 0 {
        return 0
    }
    return 1.0 - float64(prunedTokens)/float64(origTokens)
}
```

### 6.5 Scenario-Specific Evaluation

| Scenario | Content | Metrics | Target |
|----------|---------|---------|--------|
| Code pruning (IRPruner) | Go/Python/JS 10k LOC | Token reduction, Recall@K, latency | ≥50%, ≥0.85, ≤20ms |
| Non-code pruning (IRPruner) | Markdown docs 10k lines | Token reduction, Recall@K, latency | ≥40%, ≥0.80, ≤20ms |
| Log pruning (IRPruner) | Server logs 10k lines | Token reduction, Recall@K, latency | ≥60%, ≥0.75, ≤15ms |
| Code pruning (CodePruner) | Go/Python/JS 10k LOC | Token reduction, Recall@K, latency | ≥60%, ≥0.90, ≤200ms |
| Cache effectiveness | Multi-turn conversation | Cache hit rate, latency improvement | >60%, >50% speedup |
| Memory footprint | 10k LOC + 256-slot cache | Peak RSS delta | <50MB |

### 6.6 Evaluation Workflow

```
1. Generate test datasets (code, docs, logs, JSON)
2. Run IRPruner benchmarks → record latency, allocs, memory
3. Compute Recall@K for each content type
4. Compute token reduction for each content type
5. Run cache hit rate simulation (repeated queries on same content)
6. [Optional] Run CodePruner benchmarks if neural service available
7. Compare results against v0.10.27 baseline
8. Update checklist with actual metrics
```

### 6.7 v0.10.27 Baseline (for comparison)

| Backend | Token Reduction | Recall@K | Latency (10k LOC) | Allocs/op |
|---------|----------------|----------|-------------------|-----------|
| Local (TF-IDF) | 59% | 1.00 | 15ms | 200,017 |
| BM25 | 47% | 0.51 | 6.8ms | 827 |

---

## Appendix A — Constraints Summary

| Constraint | Value |
|-----------|-------|
| Language | Go 1.22+ |
| ML frameworks | None for IRPruner |
| GPU | None for IRPruner |
| Redis | Not used |
| Deployment | Single-node, CPU-only |
| Determinism | Required for IRPruner |
| External dependencies | Minimal (stdlib + existing project deps) |

## Appendix B — Glossary

| Term | Definition |
|------|-----------|
| IRPruner | Information Retrieval Pruner — BM25/TF-IDF based, handles all content types |
| CodePruner | Neural code pruner — SWE-Pruner based, coding tasks only |
| BM25 | Okapi BM25 — probabilistic relevance scoring with TF saturation and length normalization |
| Recall@K | Fraction of relevant segments retained after pruning |
| Token budget | Maximum tokens allowed in pruned output, derived from threshold |
| Segment | Atomic unit of content: function, paragraph, heading, log group, etc. |
| ContentType | Classification of input: code, documentation, logs, data, unknown |
