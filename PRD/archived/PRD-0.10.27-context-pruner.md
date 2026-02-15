# PRD: Context Pruner — Intelligent LLM Context Compression

**Version**: 0.10.27
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-02-15

**References**:
- Paper: [SWE-Pruner: Self-Adaptive Context Pruning for Coding Agents](https://arxiv.org/abs/2601.16746)
- Original Implementation: [Ayanami1314/swe-pruner](https://github.com/Ayanami1314/swe-pruner) (Python/PyTorch)
- Model: [ayanami-kitasan/code-pruner](https://huggingface.co/ayanami-kitasan/code-pruner) (Qwen3-Reranker-0.6B backbone)

---

## 1. Overview

### 1.1 Background

ZimaOS Blue proxies LLM API requests through its OpenAI-compatible gateway (`/v1/chat/completions`). In multi-turn coding agent workflows, file-read tool outputs dominate token consumption — the SWE-Pruner paper demonstrates that read operations account for **76% of total tokens** in typical SWE-bench tasks. Most of this content is irrelevant to the agent's current step.

The existing proxy layer already implements two-level caching (CCCache) to reduce redundant API calls. However, caching does not help when the same file is read with different surrounding context, or when large files are read but only a few lines matter. A complementary approach is **context pruning** — intelligently removing irrelevant lines from tool outputs before they reach the LLM, reducing input tokens without degrading task performance.

SWE-Pruner achieves 23–54% token savings on SWE-Bench Verified with near-identical resolve rates, using a 0.6B parameter neural skimmer with CRF-based line-level pruning. The original implementation is Python/PyTorch. This PRD proposes a **Go-native reimplementation** integrated into the ZimaOS Blue proxy pipeline, with an optional toggle and pruning statistics tracked in the existing metrics system.

### 1.2 Goals

1. Implement a Go-native context pruning service inspired by SWE-Pruner's line-level approach
2. Integrate as an optional middleware in the proxy pipeline, intercepting tool-call responses containing code/file content
3. Provide a configurable on/off toggle with threshold tuning
4. Track pruning statistics (tokens saved, compression ratio, lines pruned) in the existing metrics system
5. Support two backends: **local** (pure Go, TF-IDF + structural scoring, zero dependencies) and **remote** (optional, calls external SWE-Pruner neural service for higher accuracy)

### 1.3 Non-Goals

1. Training or fine-tuning neural models
2. Modifying the agent's tool-calling behavior or adding `context_focus_question` parameters (Phase 1)
3. Pruning non-code content (natural language responses, images, etc.)
4. Replacing the existing CCCache — this is complementary

---

## 2. User Stories

### Story 1: Automatic Token Savings
**As a** developer using ZimaOS Blue as a coding agent proxy
**I want** irrelevant code lines automatically pruned from tool outputs
**So that** I spend fewer tokens on context that doesn't help the LLM solve my task

**Acceptance Criteria:**
- [ ] Tool-call responses containing code are automatically pruned before reaching the LLM
- [ ] Pruning preserves syntactically valid code structure (line-level granularity)
- [ ] Pruned sections are replaced with `(filtered N lines)` markers so the agent knows content was removed
- [ ] Token savings of 20%+ on code-heavy multi-turn conversations

### Story 2: Pruning Statistics in Usage Dashboard
**As an** administrator
**I want** to see how much the pruner is saving in the usage/metrics dashboard
**So that** I can quantify the cost reduction and justify keeping it enabled

**Acceptance Criteria:**
- [ ] Metrics track: total tokens pruned, compression ratio, number of pruned requests
- [ ] Per-model breakdown of token savings
- [ ] Statistics accessible via the existing `/api/v1/metrics/stats` endpoint
- [ ] Pruning stats included in the usage data export

### Story 3: Configurable Pruning
**As an** administrator
**I want** to enable/disable pruning and tune the aggressiveness
**So that** I can balance token savings against potential information loss

**Acceptance Criteria:**
- [ ] Global enable/disable toggle in `config.yaml`
- [ ] Configurable pruning threshold (0.0–1.0, default 0.5)
- [ ] Configurable minimum file size to trigger pruning (skip small files)
- [ ] Choice of backend: `local` (ONNX) or `remote` (external service URL)

---

## 3. Functional Requirements

### 3.1 Pruning Engine

| ID | Requirement | Priority |
|----|-------------|----------|
| PR-001 | Implement line-level scoring for code content | P0 |
| PR-002 | Support CRF-based token-to-line score aggregation | P0 |
| PR-003 | Replace pruned sections with `(filtered N lines)` markers | P0 |
| PR-004 | Chunk long files (>8192 tokens) with configurable overlap | P0 |
| PR-005 | Preserve code structure — never split mid-statement | P1 |
| PR-006 | Support query/goal hint for context-aware pruning | P1 |
| PR-007 | Document-level relevance scoring (reranking head) | P2 |

### 3.2 Inference Backends

| ID | Requirement | Priority |
|----|-------------|----------|
| PR-008 | Local backend: pure Go TF-IDF + structural scoring (zero external dependencies) | P0 |
| PR-009 | Remote backend: HTTP client to external SWE-Pruner FastAPI service (optional, for higher accuracy) | P1 |
| PR-010 | Backend selection via config (`local` default / `remote`) | P0 |
| PR-011 | Health check with graceful fallback (pass-through if unavailable) | P0 |
| PR-012 | Connection pooling and timeout configuration for remote backend | P2 |

### 3.3 Proxy Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| PR-013 | Intercept tool-call responses in the proxy pipeline | P0 |
| PR-014 | Detect code/file content in tool outputs (heuristic: language markers, file paths, code fences) | P0 |
| PR-015 | Apply pruning only to tool-output messages, not user messages or system prompts | P0 |
| PR-016 | Pass-through when pruner is disabled or backend is unavailable | P0 |
| PR-017 | Support both streaming and non-streaming response paths | P1 |
| PR-018 | Extract query context from the conversation's last user message or tool-call description | P1 |

### 3.4 Metrics Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| PR-019 | Track total input tokens before and after pruning per request | P0 |
| PR-020 | Track cumulative tokens saved (input_tokens_original - input_tokens_pruned) | P0 |
| PR-021 | Track compression ratio per request and rolling average | P0 |
| PR-022 | Track number of requests pruned vs passed-through | P1 |
| PR-023 | Track pruning latency (time spent in pruner) | P1 |
| PR-024 | Expose pruning stats in `/api/v1/metrics/stats` response | P0 |
| PR-025 | Persist pruning stats to SQLite via existing MetricsWriter | P1 |
| PR-026 | Include pruning stats in user data export | P2 |

### 3.5 Configuration

| ID | Requirement | Priority |
|----|-------------|----------|
| PR-027 | Global enable/disable toggle | P0 |
| PR-028 | Pruning threshold (float, 0.0–1.0, default 0.5) | P0 |
| PR-029 | Minimum content length to trigger pruning (default 200 lines) | P1 |
| PR-030 | Backend mode: `local` (default) or `remote` | P0 |
| PR-031 | Remote backend URL configuration (only when backend=remote) | P1 |
| PR-032 | Request timeout for remote backend (default 5s) | P2 |

---

## 4. Technical Design

### 4.1 Architecture

```
Client Request (with tool outputs)
        |
        v
   ProxyHandler
        |
        v
   ┌──────────────────┐
   │  Context Pruner   │  ← NEW middleware in proxy pipeline
   │  (optional)       │
   │                   │
   │  Detect code in   │
   │  tool outputs     │──→ Skip if non-code or below min size
   │                   │
   │  Score lines      │──→ Local:  TF-IDF + structural scoring (default)
   │                   │    Remote: POST /prune to SWE-Pruner service (optional)
   │                   │
   │  Prune low-score  │
   │  lines            │──→ Replace with "(filtered N lines)"
   │                   │
   │  Record metrics   │──→ Stats.Record()
   └──────────────────┘
        |
        v
   Route to Provider (with reduced token count)
        |
        v
   Response → Client
```

### 4.2 Package Structure

```
server/internal/pruner/
├── pruner.go              // Backend interface, Config, factory
├── scorer.go              // LocalBackend: TF-IDF + structural line scoring (default)
├── remote_backend.go      // RemoteBackend: HTTP client for SWE-Pruner service (optional)
├── detector.go            // Code content detection heuristics
├── token_estimator.go     // Token count estimation
├── middleware.go          // Proxy middleware integration
├── metrics.go             // Pruning statistics (atomic counters)
├── handler.go             // API handler (/stats, /config, /health)
├── pruner_test.go         // Config + factory tests
├── scorer_test.go         // Local backend + scoring tests
├── remote_backend_test.go // Remote backend tests
├── detector_test.go       // Detection heuristic tests
├── middleware_test.go     // Middleware tests
└── metrics_test.go        // Token estimator + stats tests
```

### 4.3 Core API

```go
package pruner

// Backend defines the inference backend interface.
type Backend interface {
    // Prune scores and removes low-relevance lines from code content.
    Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error)
    // Health returns nil if the backend is available.
    Health(ctx context.Context) error
    // Close releases backend resources.
    Close() error
}

// PruneRequest is the input to the pruning engine.
type PruneRequest struct {
    Code      string  // Source code content
    Query     string  // What the agent is looking for (optional)
    Threshold float64 // Score threshold for keeping lines (0.0–1.0)
}

// PruneResponse is the output from the pruning engine.
type PruneResponse struct {
    PrunedCode      string    // Code with low-score lines replaced by markers
    Score           float64   // Document-level relevance score
    OriginalLines   int       // Total lines in original
    KeptLines       int       // Lines retained
    PrunedLines     int       // Lines removed
    OriginalTokens  int       // Estimated token count before pruning
    PrunedTokens    int       // Estimated token count after pruning
    CompressionRate float64   // PrunedTokens / OriginalTokens
    LatencyMs       float64   // Time spent in pruner
}

// Config holds pruner configuration.
type Config struct {
    Enabled   bool    `mapstructure:"enabled"`
    Backend   string  `mapstructure:"backend"`    // "local" (default) or "remote"
    RemoteURL string  `mapstructure:"remote_url"` // only used when backend=remote
    Threshold float64 `mapstructure:"threshold"`  // 0.0–1.0, default 0.5
    MinLines  int     `mapstructure:"min_lines"`  // Minimum lines to trigger pruning
    TimeoutMs int     `mapstructure:"timeout_ms"` // Remote backend timeout
}

func DefaultConfig() Config {
    return Config{
        Enabled:   false, // Opt-in by default
        Backend:   "local",
        Threshold: 0.5,
        MinLines:  200,
        TimeoutMs: 5000,
    }
}
```

### 4.4 Local Backend (Default)

The local backend uses a pure-Go scoring algorithm with zero external dependencies:

1. **Structural keyword scoring**: Lines starting with `func`, `class`, `import`, `type`, `package`, etc. get high scores (0.8–1.0)
2. **Query relevance via TF-IDF with query expansion**: Tokenize the query with camelCase/snake_case splitting (`authenticateUser` → `authenticate` + `user`), compute term overlap with IDF-like weighting
3. **Position bias**: First/last 10% of file lines get a boost (headers, imports, exports)
4. **Bracket preservation**: Closing braces `}` always kept to maintain structure
5. **Threshold pruning**: Lines below threshold are grouped into `(filtered N lines)` markers

```go
// LocalBackend implements Backend using pure-Go TF-IDF + structural scoring.
type LocalBackend struct {
    config Config
}

func (b *LocalBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
    // 1. Split code into lines
    // 2. Score each line (structural + query relevance + position)
    // 3. Group consecutive low-score lines into "(filtered N lines)" markers
    // 4. Return pruned code with metrics
}
```

### 4.5 Remote Backend (Optional)

For higher accuracy, an optional remote backend can call the SWE-Pruner neural service (0.6B CRF model):

```go
// RemoteBackend calls an external SWE-Pruner FastAPI service.
type RemoteBackend struct {
    client  *http.Client
    baseURL string
}

func (r *RemoteBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
    // POST /prune with JSON body {code, query, threshold}
    // Parse response {score, pruned_code, kept_frags}
    // Calculate metrics (original vs pruned token counts)
}

func (r *RemoteBackend) Health(ctx context.Context) error {
    // GET /health
}
```

### 4.5 Code Detection Heuristics

The pruner should only process content that looks like source code. Detection heuristics:

1. **Code fence markers**: Content wrapped in `` ```language ... ``` ``
2. **File path indicators**: Tool output referencing file paths (e.g., `cat src/main.go`)
3. **Syntax patterns**: High density of programming tokens (`{`, `}`, `func`, `class`, `import`, `def`, etc.)
4. **Line count threshold**: Only prune content exceeding `min_lines` configuration

```go
// IsCodeContent returns true if the content appears to be source code.
func IsCodeContent(content string, minLines int) bool {
    lines := strings.Count(content, "\n")
    if lines < minLines {
        return false
    }
    // Check for code fences, syntax patterns, etc.
}
```

### 4.6 Proxy Middleware Integration

The pruner integrates into the proxy pipeline by processing the request body before forwarding:

```go
func (p *PrunerMiddleware) ProcessRequest(body []byte) ([]byte, *PruneStats, error) {
    var req openaiRequest
    json.Unmarshal(body, &req)

    modified := false
    var stats PruneStats

    for i, msg := range req.Messages {
        // Only process tool-output messages
        if msg.Role != "tool" {
            continue
        }
        if !IsCodeContent(msg.Content, p.config.MinLines) {
            continue
        }

        // Extract query context from preceding messages
        query := extractQueryContext(req.Messages, i)

        result, err := p.backend.Prune(ctx, PruneRequest{
            Code:      msg.Content,
            Query:     query,
            Threshold: p.config.Threshold,
        })
        if err != nil {
            continue // Pass-through on error
        }

        req.Messages[i].Content = result.PrunedCode
        stats.Add(result)
        modified = true
    }

    if !modified {
        return body, nil, nil
    }
    return json.Marshal(req), &stats, nil
}
```

### 4.7 Metrics Integration

Extend the existing `MetricsWriter` with pruning statistics:

```go
// PruningStats tracks cumulative pruning metrics.
type PruningStats struct {
    TotalRequests      int64   // Requests that went through pruner
    PrunedRequests     int64   // Requests where pruning was applied
    PassthroughRequests int64  // Requests skipped (non-code, too small, error)
    TotalTokensBefore  int64   // Sum of original token counts
    TotalTokensAfter   int64   // Sum of pruned token counts
    TotalTokensSaved   int64   // TotalTokensBefore - TotalTokensAfter
    AvgCompressionRate float64 // Rolling average compression ratio
    TotalLatencyMs     float64 // Cumulative pruning latency
    AvgLatencyMs       float64 // Average pruning latency per request
}
```

Exposed in the `/api/v1/metrics/stats` response:

```json
{
  "token_usage": { ... },
  "model_stats": [ ... ],
  "pruning": {
    "enabled": true,
    "total_requests": 1542,
    "pruned_requests": 876,
    "tokens_saved": 2450000,
    "avg_compression_rate": 0.62,
    "avg_latency_ms": 45.2,
    "estimated_cost_saved": 1.23
  }
}
```

### 4.8 Configuration

```yaml
pruner:
  enabled: false              # Opt-in, disabled by default
  backend: "local"            # "local" (default, pure Go) or "remote" (SWE-Pruner service)
  threshold: 0.5              # Line score threshold (0.0–1.0)
  min_lines: 200              # Minimum lines to trigger pruning
  # remote_url: "http://localhost:8000"  # Only needed when backend=remote
  # timeout_ms: 5000                     # Only needed when backend=remote
```

---

## 5. Implementation Phases

| Phase | Scope | Priority | Status |
|-------|-------|----------|--------|
| Phase 1 | Core `pruner` package: Backend interface, LocalBackend (TF-IDF + structural scoring), code detection, config | P0 | ✅ Done |
| Phase 2 | Proxy middleware integration: intercept tool outputs, apply pruning, pass-through fallback | P0 | ✅ Done |
| Phase 3 | Metrics integration: track tokens saved, compression ratio, expose in stats API | P0 | ✅ Done |
| Phase 4 | Configuration wiring: config.yaml defaults, bootstrap route registration | P0 | ✅ Done |
| Phase 5 | RemoteBackend: optional HTTP client to SWE-Pruner neural service for higher accuracy | P1 | ✅ Done |
| Phase 6 | Query context extraction: derive pruning hints from conversation context | P1 | ✅ Done |
| Phase 7 | BM25 + Segment Indexing: BM25Scorer, Segmenter, LRU cache, Top-K selector, weight boosting | P1 | ✅ Done |
| Phase 8 | Validation & Benchmarking: latency, Recall@K, token reduction, memory profiling | P1 | ✅ Done |
| Phase 8b | Scoring Optimization: query expansion, name-match boost, budget tuning | P1 | ✅ Done |
| Phase 9 | Streaming support: prune tool outputs in streaming response path | P2 | Planned |
| Phase 10 | Admin UI: pruning toggle, stats visualization in dashboard | P2 | Planned |

---

## 6. Test Strategy

### 6.1 Unit Tests

Table-driven tests for each component:

```go
func TestIsCodeContent(t *testing.T) {
    tests := []struct {
        name     string
        content  string
        minLines int
        expected bool
    }{
        {"go source", goFileContent, 10, true},
        {"short snippet", "x := 1", 10, false},
        {"natural language", longParagraph, 10, false},
        {"python with fences", fencedPython, 10, true},
    }
}

func TestRemoteBackend_Prune(t *testing.T) {
    // Mock HTTP server returning known PruneResponse
    // Verify request format, threshold passing, response parsing
}

func TestPrunerMiddleware_ProcessRequest(t *testing.T) {
    // Full request body with tool messages
    // Verify only tool messages are pruned
    // Verify non-code tool messages are skipped
    // Verify metrics are recorded
}
```

### 6.2 Integration Tests

- End-to-end: send a multi-turn conversation through the proxy with pruner enabled, verify token reduction
- Failover: kill the remote backend mid-request, verify graceful pass-through
- Metrics: verify pruning stats appear in `/api/v1/metrics/stats` after pruned requests

### 6.3 Benchmarks

- Latency overhead: measure added latency per request with pruner enabled vs disabled
- Token savings: replay real conversation traces, measure actual compression ratios
- Throughput: concurrent requests through pruner middleware

---

## 7. Success Metrics

| Metric | Target | Actual |
|--------|--------|--------|
| Token savings on code-heavy conversations | 20–50% input token reduction | Local: 59%, BM25: 47% ✅ |
| Pruning latency overhead | < 20ms per request (local), < 100ms (remote) | Local: 15ms, BM25: 6.8ms (10k LOC) ✅ |
| Task performance degradation | < 2% on coding benchmarks | TBD (A/B testing) |
| Pass-through reliability | 100% when backend unavailable | ✅ (integration tests) |
| Test coverage | > 85% for pruner package | 90.6% ✅ |
| Recall@K | >= 0.85 against full-context baseline | Local: 1.00 ✅, BM25: 0.51 |
| Memory footprint | < 50MB for local backend | BM25: 1.4MB/10k LOC ✅ |

---

## 8. Deployment Considerations

### 8.1 Local Backend (Default)

Zero-dependency deployment — the local backend is compiled into the Go binary:
- No external services, no model downloads, no GPU
- CPU-only, deterministic, cacheable
- Latency: < 20ms on 10k LOC files
- Memory: < 50MB overhead

### 8.2 Remote Backend (Optional)

For higher accuracy, deploy the SWE-Pruner neural service as a sidecar:

```yaml
# docker-compose addition (optional)
swe-pruner:
  image: ghcr.io/ayanami1314/swe-pruner:latest
  ports:
    - "8000:8000"
  volumes:
    - ./models/code-pruner:/app/model
  deploy:
    resources:
      limits:
        memory: 2G
```

The 0.6B model requires ~1.2GB RAM. GPU is optional but recommended for throughput.

---

## 9. Advanced Local Scorer — BM25 + Segment Indexing (Phase 2)

The initial local backend uses TF-IDF + structural heuristics. Phase 2 upgrades to a more sophisticated BM25-based scorer with segment-level indexing for better recall.

### 9.1 Motivation

- TF-IDF treats all terms equally; BM25 adds term frequency saturation and document length normalization
- Line-level scoring misses function/class boundaries; segment-level indexing preserves semantic units
- Caching scored segments avoids re-computation in multi-turn conversations

### 9.2 Architecture

```
Code Input
    |
    v
┌─────────────────┐
│  Segmenter       │  Split into segments: file / function / class / block
└────────┬────────┘
         v
┌─────────────────┐
│  Tokenizer       │  Normalize: lowercase, split camelCase/snake_case, stem
└────────┬────────┘
         v
┌─────────────────┐
│  BM25 Scorer     │  Score segments against query (k1=1.2, b=0.75)
└────────┬────────┘
         v
┌─────────────────┐
│  Rule Weights    │  Boost structural keywords, imports, exports, signatures
└────────┬────────┘
         v
┌─────────────────┐
│  Top-K Selector  │  Keep top segments until token budget met
└────────┬────────┘
         v
┌─────────────────┐
│  Cache Layer     │  LRU in-memory + optional disk persistence
└─────────────────┘
```

### 9.3 Key Interfaces

```go
// Scorer is the pluggable scoring interface.
type Scorer interface {
    Score(query string, segments []Segment) []ScoredSegment
}

// Segment represents a code unit (function, class, block, or line range).
type Segment struct {
    StartLine int
    EndLine   int
    Kind      SegmentKind // Function, Class, Block, Lines
    Name      string      // e.g., "func main", "class Foo"
    Content   string
    Tokens    []string    // pre-tokenized
}

type SegmentKind int
const (
    SegmentFunction SegmentKind = iota
    SegmentClass
    SegmentBlock
    SegmentLines
)

// ScoredSegment pairs a segment with its relevance score.
type ScoredSegment struct {
    Segment Segment
    Score   float64
}

// BM25Scorer implements Scorer using Okapi BM25.
type BM25Scorer struct {
    K1       float64 // term frequency saturation (default 1.2)
    B        float64 // length normalization (default 0.75)
    AvgDL    float64 // average document length
}

// ScoreCache provides LRU caching for scored results.
type ScoreCache interface {
    Get(key string) ([]ScoredSegment, bool)
    Put(key string, segments []ScoredSegment)
}
```

### 9.4 Performance Targets

| Metric | Target |
|--------|--------|
| Token reduction | >= 70% on code-heavy tool outputs |
| Added latency | <= 20ms on 10k LOC files |
| Recall@K | >= 0.85 against full-context baseline |
| Memory footprint | < 50MB for scorer + cache |
| Cache hit rate | > 60% in multi-turn conversations |

### 9.5 Constraints

- Go 1.22+, no ML frameworks, no GPU, no Redis
- Single-node, CPU-only, deterministic
- Pluggable scorer interface (BM25 default, TF-IDF fallback)
- Favor slices over maps for memory locality
- Avoid reflection

### 9.6 Implementation Checklist (Phase 2)

**Core Scoring:**
- [ ] Implement BM25Scorer with configurable k1, b parameters
- [ ] Implement code tokenizer (camelCase/snake_case splitting, lowercasing)
- [ ] Implement pluggable Scorer interface

**Indexing:**
- [ ] Implement Segmenter: split code into function/class/block segments
- [ ] Support Go, Python, JavaScript/TypeScript, Rust, Java segment detection
- [ ] Implement segment-level token pre-computation

**Caching:**
- [ ] Implement LRU in-memory score cache
- [ ] Implement cache key generation (file hash + query hash)
- [ ] Optional disk persistence for cache

**Pruning Pipeline:**
- [ ] Implement Top-K selector with token budget
- [ ] Implement rule-based weight boosting layer
- [ ] Wire BM25 scorer into existing Middleware

**Validation:**
- [ ] Benchmark harness: latency measurement on 1k/5k/10k LOC files
- [ ] Recall@K computation against full-context baseline
- [ ] Token reduction calculation
- [ ] Memory profiling

---

## 10. Scoring Optimization Techniques (Implemented)

The following techniques were applied to bring rule-based + BM25 scoring close to neural pruning quality:

### 10.1 Query Expansion

The `tokenize()` function now uses `codeTokenize()` to split camelCase and snake_case identifiers:
- `authenticateUser` → `authenticate` + `user`
- `get_user_name` → `get` + `user` + `name`
- `parseHTTPResponse` → `parse` + `http` + `response`

This enables cross-identifier matching: a query for "authenticateUser" will match lines containing just "authenticate" or "user".

### 10.2 Name-Match Boosting

`BoostScoresWithQuery()` gives a 1.5x multiplier when a segment's name (e.g., `func authenticate`) contains tokens matching the query. This ensures the most relevant functions are prioritized even when their body content is generic.

### 10.3 Aggressive Token Budget

The BM25 token budget formula was changed from `1.0 - threshold*0.5` (too generous) to `1.0 - threshold`:
- threshold=0.5 → keep 50% of tokens (was 75%)
- threshold=0.3 → keep 70% of tokens (was 85%)

### 10.4 Structural Weight Hierarchy

Boost multipliers by segment type:
- Function: 1.3x
- Class: 1.25x
- Block: 1.1x
- Import/include: additional 1.15x
- Name-match: additional 1.5x

### 10.5 Results After Optimization

| Backend | Token Reduction | Recall@K | Latency (10k LOC) | Allocs/op (10k) |
|---------|----------------|----------|-------------------|-----------------|
| Local   | ~59%           | 1.00     | 15ms              | 200,017         |
| BM25    | ~47%           | 0.51     | 6.8ms             | 827             |

**Trade-off analysis**: Local backend preserves all relevant lines (Recall=1.0) but is slower due to per-line tokenization. BM25 backend is 2x faster with 99.6% fewer allocations, achieving 47% token reduction — approaching the 50%+ range of neural pruning while maintaining sub-10ms latency.

---

## 11. Open Questions

- [ ] Should pruning be applied to all tool-output messages or only those matching specific tool names (e.g., `cat`, `read_file`, `str_replace_editor`)?
- [ ] Should the pruner cache line scores for recently-seen files to avoid re-scoring the same file in a multi-turn conversation?
- [ ] Should there be a per-request opt-out mechanism (e.g., a header `X-Skip-Pruning: true`) for debugging?
- [ ] What is the minimum viable threshold that preserves task performance? The paper uses 0.5 but this may need tuning for ZimaOS Blue's specific workloads.
- [ ] Should BM25 parameters (k1, b) be configurable via config.yaml or hardcoded?
