# PRD: Token Economy — Unified Token Savings & Model Routing

**Version**: 0.10.27-2
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-02-15

**References**:
- PRD-0.10.25: CC Cache (two-level response caching)
- PRD-0.10.27: Context Pruner (BM25/IR line-level pruning)
- PRD-0.10.27-1: Unified Pruner (code + non-code)
- Paper: [SWE-Pruner](https://arxiv.org/abs/2601.16746) — 23–54% token reduction
- Model: [ayanami-kitasan/code-pruner](https://huggingface.co/ayanami-kitasan/code-pruner) (Qwen3-0.6B)

---

## 1. Overview

### 1.1 Background

ZimaOS Blue proxies all LLM API requests through its OpenAI-compatible gateway. Token consumption is the dominant cost driver — SWE-Pruner research shows read operations account for **76% of total tokens** in coding agent workflows. Current mitigation is fragmented across three independent systems:

| Layer | Mechanism | Status | Savings |
|-------|-----------|--------|---------|
| CC Cache | Response deduplication (L1 memory + L2 disk) | Shipped (v0.10.25) | 30–60% on repeated queries |
| Context Pruner | Line-level content removal (BM25/IR) | Shipped (v0.10.27) | 20–50% on tool outputs |
| ONNX Neural Pruner | SWE-Pruner model for code-specific pruning | In progress | 23–54% on code (paper) |
| Model Routing | Route simple tasks to cheaper/smaller models | **New** | 50–90% cost per request |

These layers are complementary but currently lack unified configuration, combined metrics, and the critical missing piece: **model routing** — the ability to offload simple tasks (intent classification, file filtering, diff planning, prompt compression) to cheaper or smaller models. This includes both cloud-side downgrades (Opus → Haiku, GPT-4 → GPT-4o-mini) and local model offloading (Ollama/vLLM). The key metric is cost-per-token, not where the model runs.

### 1.2 Goals

| ID | Goal | Target |
|----|------|--------|
| G-001 | Unified "Token Economy" settings page | Single UI for cache + pruner + model routing |
| G-002 | Model routing: offload simple tasks to cheaper models | ≥40% cost reduction on multi-step agent workflows |
| G-003 | Routing decision latency | ≤5ms (header/rule-based), ≤50ms (classifier-based) |
| G-004 | Combined token savings (cache + pruner + routing) | ≥60% total cost reduction vs baseline |
| G-005 | TDD with benchmarks | All components test-first, reproducible benchmarks |
| G-006 | Model tier labeling | Clear visual indicator for model cost tier |

### 1.3 Non-Goals

1. Training custom routing models — use existing instruct models
2. Replacing the provider pool — routing sits above it
3. Automatic model discovery — admin configures available models
4. Modifying agent behavior — transparent proxy-level optimization

### 1.4 Target Users

- **Developers** using ZimaOS Blue as a coding agent proxy who want lower token costs
- **Administrators** who manage LLM API budgets and want visibility into savings
- **Self-hosters** running local models (Ollama, vLLM) alongside cloud APIs

---

## 2. User Stories

### Story 1: Model Routing for Simple Tasks
**As a** developer using a coding agent
**I want** simple sub-tasks (intent classification, file filtering, diff planning) routed to a cheaper model
**So that** I save money by reserving expensive models for tasks that actually need them

**Acceptance Criteria:**
- [ ] Proxy detects task complexity via configurable rules (header, prompt size, tool stage)
- [ ] Simple tasks route to configured cheaper model (e.g., Haiku instead of Opus, GPT-4o-mini instead of GPT-4, or Qwen2.5-7B via Ollama)
- [ ] Complex tasks stay on the original expensive model
- [ ] Routing decision adds ≤5ms latency (rule-based) or ≤50ms (classifier-based)
- [ ] Failed cheaper model calls fall back to the original model automatically

### Story 2: Model Tier Labeling
**As an** administrator
**I want** to see which model handled each request and its cost tier
**So that** I can verify routing decisions and audit cost allocation

**Acceptance Criteria:**
- [ ] Response headers include `X-Model-Tier: premium|standard|economy|free`
- [ ] Response headers include `X-Model-Actual: <model-id>`
- [ ] UI shows tier badge per request in metrics
- [ ] Routing decisions logged with reason

### Story 3: Unified Token Economy Dashboard
**As an** administrator
**I want** a single settings page showing all token-saving mechanisms
**So that** I can configure and monitor cache, pruner, and routing together

**Acceptance Criteria:**
- [ ] Combined savings metric: total tokens saved across all layers
- [ ] Per-layer breakdown: cache hits, pruner compression, routing offloads
- [ ] Cost estimation: approximate USD saved based on model pricing
- [ ] Enable/disable each layer independently

### Story 4: TDD & Benchmarks
**As a** developer contributing to ZimaOS Blue
**I want** all routing logic covered by tests with reproducible benchmarks
**So that** regressions are caught and performance claims are verifiable

**Acceptance Criteria:**
- [ ] Unit tests for every routing rule type (header, size, tool stage, classifier)
- [ ] Integration test: end-to-end request routing with mock providers
- [ ] Benchmark suite: latency, accuracy, cost savings on representative workloads
- [ ] CI gate: routing decision latency ≤5ms (p99) for rule-based routing

---

## 3. Functional Requirements

| ID | Requirement | Priority | Component |
|----|-------------|----------|-----------|
| FR-001 | Rule-based model routing (header, body size, tool stage) | P0 | Proxy |
| FR-002 | Model tier labeling in response headers | P0 | Proxy |
| FR-003 | Fallback: cheaper model failure → retry on original model | P0 | Proxy |
| FR-004 | Routing rules configurable via YAML + runtime API | P0 | Config |
| FR-005 | Unified Token Economy settings page (cache + pruner + routing) | P0 | Frontend |
| FR-006 | Combined savings metrics (tokens, cost estimate) | P1 | Metrics |
| FR-007 | Per-request routing decision logging with reason | P1 | Proxy |
| FR-008 | Classifier-based routing (smaller model classifies intent) | P1 | Proxy |
| FR-009 | ONNX pruner model download with progress UI | P1 | Pruner |
| FR-010 | Cost estimation based on model pricing table | P2 | Metrics |
| FR-011 | Routing analytics: model tier request distribution | P2 | Frontend |

---

## 4. Technical Design

### 4.1 Architecture

```
                         ┌─────────────────────────────────────────┐
                         │           Proxy Pipeline                │
                         │                                         │
  Client Request ──────► │  ① Auth ──► ② Cache Lookup              │
                         │                  │ miss                  │
                         │            ③ Model Router ◄── Rules     │
                         │               /        \                │
                         │         economy      premium            │
                         │             │            │              │
                         │        ④ Pruner MW   ④ Pruner MW       │
                         │             │            │              │
                         │        ⑤ Forward     ⑤ Forward         │
                         │        (Haiku/local) (Opus/GPT-4)      │
                         │             │            │              │
                         │        ⑥ Label Tier  (X-Model-Tier)    │
                         │             │            │              │
                         │        ⑦ Cache Store ◄───┘              │
                         │             │                           │
                         └─────────────┼───────────────────────────┘
                                       ▼
                                  Client Response
                             (+ X-Model-Tier header)
```

### 4.2 Model Router — Routing Dimensions

The router evaluates rules in priority order. First match wins.

| Dimension | Detection Method | Example |
|-----------|-----------------|---------|
| **Header** | `X-Model-Tier: economy` or `X-Task-Type: classify` | Agent SDK sets header for sub-tasks |
| **Request body size** | `len(messages JSON) < threshold` | Small prompts → cheaper model |
| **Tool call stage** | Presence of `tool_choice`, tool name patterns | `file_search`, `list_files` → cheaper model |
| **System prompt tag** | `[SIMPLE]` or `[ORCHESTRATOR]` prefix in system message | Agent marks orchestration steps |
| **Classifier** (P1) | Smaller model classifies intent from first user message | "Is this a simple task?" → yes/no |

### 4.3 Core Interfaces

```go
// server/internal/proxy/model_router.go

// ModelTier indicates the cost tier of a model.
type ModelTier string

const (
    TierPremium  ModelTier = "premium"  // e.g., Opus, GPT-4
    TierStandard ModelTier = "standard" // e.g., Sonnet, GPT-4o
    TierEconomy  ModelTier = "economy"  // e.g., Haiku, GPT-4o-mini
    TierFree     ModelTier = "free"     // e.g., local Ollama models
)

// RoutingRule defines a single routing rule.
type RoutingRule struct {
    Name        string       `yaml:"name"`
    Priority    int          `yaml:"priority"`     // lower = higher priority
    Condition   RouteCondition `yaml:"condition"`
    TargetModel string       `yaml:"target_model"` // model ID to route to
    Tier        ModelTier    `yaml:"tier"`          // premium, standard, economy, free
    Fallback    string       `yaml:"fallback"`     // fallback model on failure
}

// RouteCondition defines when a rule matches.
type RouteCondition struct {
    // Header-based: match if header key=value present
    Header      string `yaml:"header,omitempty"`
    HeaderValue string `yaml:"header_value,omitempty"`

    // Body size: match if serialized messages < max_bytes
    MaxBodyBytes int `yaml:"max_body_bytes,omitempty"`

    // Tool stage: match if tool_choice or tool names match pattern
    ToolPattern string `yaml:"tool_pattern,omitempty"`

    // System prompt tag: match if system message contains tag
    SystemTag string `yaml:"system_tag,omitempty"`
}

// RouteDecision is the output of the router.
type RouteDecision struct {
    Model    string    `json:"model"`
    Tier     ModelTier `json:"tier"`
    Fallback string    `json:"fallback,omitempty"`
    Rule     string    `json:"rule"`     // which rule matched
    Reason   string    `json:"reason"`   // human-readable reason
}
```

### 4.4 Configuration

```yaml
# config.yaml — token_economy section
token_economy:
  # --- CC Cache (existing) ---
  cache:
    enabled: true
    max_size: 1000
    ttl: 3600

  # --- Context Pruner (existing + ONNX) ---
  pruner:
    enabled: true
    backend: "ir"          # "local", "ir", "bm25", "onnx", "remote"
    threshold: 0.5
    model_dir: ""          # auto: {data_dir}/pruner-models/

  # --- Model Routing (new) ---
  routing:
    enabled: false         # disabled by default
    rules:
      - name: "header-economy"
        priority: 10
        condition:
          header: "X-Model-Tier"
          header_value: "economy"
        target_model: "claude-haiku-4-5-20251001"
        tier: "economy"
        fallback: "claude-sonnet-4-20250514"

      - name: "small-prompt"
        priority: 20
        condition:
          max_body_bytes: 4096
        target_model: "claude-haiku-4-5-20251001"
        tier: "economy"
        fallback: "claude-sonnet-4-20250514"

      - name: "tool-filter"
        priority: 30
        condition:
          tool_pattern: "^(list_files|file_search|grep)$"
        target_model: "claude-haiku-4-5-20251001"
        tier: "economy"
        fallback: "claude-sonnet-4-20250514"

      - name: "orchestrator-tag"
        priority: 40
        condition:
          system_tag: "[ORCHESTRATOR]"
        target_model: "gpt-4o-mini"
        tier: "economy"
        fallback: "claude-sonnet-4-20250514"

    # Model tier registry (for labeling models by cost tier)
    model_tiers:
      "claude-opus-*":     "premium"
      "gpt-4":             "premium"
      "claude-sonnet-*":   "standard"
      "gpt-4o":            "standard"
      "claude-haiku-*":    "economy"
      "gpt-4o-mini":       "economy"
      "qwen*":             "free"
      "llama*":            "free"

    # Pricing table for cost estimation (USD per 1M tokens)
    pricing:
      "claude-opus-4-20250514":    { input: 15.0, output: 75.0 }
      "claude-sonnet-4-20250514":  { input: 3.0, output: 15.0 }
      "claude-haiku-4-5-20251001": { input: 0.80, output: 4.0 }
      "gpt-4o":                    { input: 2.5, output: 10.0 }
      "gpt-4o-mini":               { input: 0.15, output: 0.60 }
      "qwen2.5-7b-instruct":      { input: 0.0, output: 0.0 }  # local = free
```

### 4.5 Response Headers

Every proxied response includes:

```
X-Model-Tier: premium|standard|economy|free
X-Model-Actual: claude-haiku-4-5-20251001
X-Model-Requested: claude-sonnet-4-20250514
X-Route-Rule: small-prompt
X-Tokens-Saved-Cache: 0
X-Tokens-Saved-Pruner: 1247
```

### 4.6 Existing Components Integration

| Component | Current Location | Changes |
|-----------|-----------------|---------|
| CC Cache | `server/internal/proxy/cache_handler.go` | Add `X-Tokens-Saved-Cache` header |
| Context Pruner | `server/internal/pruner/` | Add ONNX backend, model download manager |
| Model Router | `server/internal/proxy/model_router.go` | Extend with rule-based routing + tier labeling |
| Provider Pool | `server/internal/proxy/providerpool/` | No changes — routing sits above it |
| Metrics | `server/internal/metrics/` | Add routing stats, combined savings |
| HomeView Card | `web/src/components/metrics/CacheStats.vue` | Rename to TokenEconomyCard, show cost saved as primary metric |
| Card Registry | `web/src/components/dashboard/cardRegistry.ts` | Update `cache-stats` → `token-economy` |

---

## 5. Implementation Phases

### Phase 1: Rule-Based Model Routing (P0) — TDD

**Test first, then implement.**

```
server/internal/proxy/
├── routing_rules.go          # Rule engine
├── routing_rules_test.go     # Unit tests (write FIRST)
├── routing_config.go         # YAML config parsing
└── routing_config_test.go    # Config parsing tests
```

**Step 1.1: Write failing tests**
```go
// routing_rules_test.go
func TestHeaderRule(t *testing.T) {
    rule := RoutingRule{
        Condition: RouteCondition{Header: "X-Model-Tier", HeaderValue: "economy"},
        TargetModel: "claude-haiku-4-5-20251001",
        Origin: OriginLocal,
    }
    req := &RouteRequest{Headers: http.Header{"X-Model-Tier": {"economy"}}}
    decision := rule.Evaluate(req)
    assert.True(t, decision.Matched)
    assert.Equal(t, "claude-haiku-4-5-20251001", decision.Model)
}

func TestBodySizeRule(t *testing.T) { ... }
func TestToolPatternRule(t *testing.T) { ... }
func TestSystemTagRule(t *testing.T) { ... }
func TestPriorityOrdering(t *testing.T) { ... }
func TestFallbackOnLocalFailure(t *testing.T) { ... }
func TestNoMatchPassthrough(t *testing.T) { ... }
```

**Step 1.2: Implement routing engine to pass tests**

**Step 1.3: Integration test with mock providers**
```go
func TestEndToEndRouting(t *testing.T) {
    // Start mock economy server + mock premium server
    // Send request with X-Model-Tier: economy
    // Verify routed to economy model
    // Verify X-Model-Tier: economy in response
}
```

### Phase 2: Tier Labeling & Response Headers (P0)

**Step 2.1: Write failing tests**
```go
func TestTierHeaderInjection(t *testing.T) {
    // Proxy a request, verify X-Model-Tier header in response
}

func TestModelTierRegistry(t *testing.T) {
    // "claude-opus-*" → premium, "claude-haiku-*" → economy
    assert.Equal(t, TierPremium, registry.Resolve("claude-opus-4-20250514"))
    assert.Equal(t, TierEconomy, registry.Resolve("claude-haiku-4-5-20251001"))
}
```

**Step 2.2: Implement header injection in proxy pipeline**

### Phase 3: ONNX Pruner Model Download (P1)

Already implemented in this PR:
- `server/internal/pruner/model_manager.go` — download with progress
- `server/internal/pruner/onnx_backend.go` — ONNX inference (build tag: `onnx`)
- `server/internal/pruner/bpe_tokenizer.go` — Go BPE tokenizer
- Frontend: download button + progress bar in ApiProxySettings.vue

**Remaining: write tests**
```go
func TestModelManagerDownload(t *testing.T) {
    // Mock HTTP server serving model files
    // Verify download progress updates
    // Verify atomic rename (.tmp → final)
    // Verify cancel stops download
}

func TestBPETokenizer(t *testing.T) {
    // Load real vocab.json + merges.txt
    // Verify encoding matches Python tokenizer output
}
```

### Phase 4: Unified Token Economy UI (P1)

#### 4a: HomeView Dashboard Card Redesign

The existing `CacheStats.vue` dashboard card (titled "API 代理缓存" / "Proxy Cache") is replaced with a "Token Economy" card. The card shifts focus from cache internals to **money saved**.

Current card layout (4 sub-cards):
- Cache Entries | Hit Rate | Tokens Saved | Status (enabled/disabled)

New card layout (4 sub-cards):
```
┌──────────────┬──────────────┬──────────────┬──────────────┐
│  $ Saved     │  Tokens      │  Cache       │  Pruner      │
│  ~$3.60      │  Saved 1.2M  │  Hit 42%     │  Compress 38%│
│  this month  │  total       │  156 entries │  ir backend  │
└──────────────┴──────────────┴──────────────┴──────────────┘
```

- Card 1: **Cost Saved** (primary metric) — estimated USD saved = `tokens_saved * price_per_token`. Uses pricing table from config. Green dollar icon.
- Card 2: **Tokens Saved** — combined total from cache hits + pruner compression. Purple chart icon.
- Card 3: **Cache** — hit rate + entry count (condensed from 2 cards into 1).
- Card 4: **Pruner** — compression rate + backend type.

Title changes: `cache.proxyCache` → `tokenEconomy.title` ("Token Economy" / "省流")

File: `web/src/components/metrics/CacheStats.vue` → rename to `TokenEconomyCard.vue`
Registry: `cardRegistry.ts` → update `cache-stats` card to `token-economy`

#### 4b: Settings Page (existing ApiProxySettings.vue)

Consolidate cache, pruner, and routing settings into a single "Token Economy" tab:

```
┌─────────────────────────────────────────────┐
│  Token Economy                              │
│                                             │
│  Total Savings: 1.2M tokens (~$3.60 saved)  │
│  ┌─────────┬──────────┬──────────┐          │
│  │ Cache   │ Pruner   │ Routing  │          │
│  │ 450K    │ 320K     │ 430K     │          │
│  └─────────┴──────────┴──────────┘          │
│                                             │
│  ── CC Cache ──────────────── [ON] ──       │
│  Hit Rate: 42%  Entries: 156/1000           │
│                                             │
│  ── Context Pruner ────────── [ON] ──       │
│  Backend: ir  Compression: 38%              │
│  ONNX Model: [Download] ~1.4 GB            │
│                                             │
│  ── Model Routing ─────────── [OFF] ──      │
│  Rules: 4 configured                        │
│  Premium: 30%  Economy: 50%  Free: 20%      │
│  Economy Models: Haiku, GPT-4o-mini         │
└─────────────────────────────────────────────┘
```

### Phase 5: Classifier-Based Routing (P2)

Optional: use a cheaper model to classify request intent before routing.

```go
// ClassifierRouter calls a cheaper model to classify intent.
type ClassifierRouter struct {
    client   *http.Client
    modelURL string // e.g., http://localhost:11434/v1/chat/completions or cloud API
    model    string // e.g., "claude-haiku-4-5-20251001" or "qwen2.5-7b-instruct"
}

// Classify sends a meta-prompt to the cheaper model:
// "Is this a simple task (file listing, search, classification)
//  or a complex task (code generation, refactoring, reasoning)?
//  Reply: SIMPLE or COMPLEX"
func (c *ClassifierRouter) Classify(ctx context.Context, messages []Message) (ModelTier, error)
```

---

## 6. Test Strategy

### 6.1 Unit Tests (TDD — write first)

| Test File | Coverage |
|-----------|----------|
| `routing_rules_test.go` | All 4 condition types, priority ordering, no-match passthrough |
| `routing_config_test.go` | YAML parsing, validation, defaults |
| `tier_registry_test.go` | Glob pattern matching for model → tier |
| `model_manager_test.go` | Download progress, cancel, atomic rename |
| `bpe_tokenizer_test.go` | Byte-to-unicode, BPE merges, input construction |
| `onnx_backend_test.go` | Score aggregation, line selection (mock session) |

### 6.2 Integration Tests

```go
func TestProxyRoutingEndToEnd(t *testing.T) {
    // 1. Start mock economy server + mock premium server
    // 2. Configure routing rules
    // 3. Send requests matching different rules
    // 4. Verify correct upstream was called
    // 5. Verify response headers (X-Model-Tier, X-Model-Actual)
    // 6. Verify fallback on cheaper model failure
}

func TestTokenEconomyCombined(t *testing.T) {
    // 1. Send identical request twice (cache hit on second)
    // 2. Send large code context (pruner reduces tokens)
    // 3. Send simple task (routed to cheaper model)
    // 4. Verify combined savings metrics
}
```

### 6.3 Benchmarks

```go
// routing_rules_bench_test.go

func BenchmarkRuleEvaluation(b *testing.B) {
    rules := loadTestRules(10) // 10 rules
    req := buildTestRequest()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        evaluateRules(rules, req)
    }
    // Target: ≤5μs per evaluation (≤5ms at p99 with overhead)
}

func BenchmarkOriginResolution(b *testing.B) {
    registry := buildTestRegistry()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        registry.Resolve("claude-sonnet-4-20250514")
    }
    // Target: ≤1μs per resolution
}

func BenchmarkBPETokenize(b *testing.B) {
    tok := loadTestTokenizer()
    code := loadTestCode(1000) // 1000 lines
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        tok.Encode(code)
    }
}
```

**Benchmark Thresholds:**

| Metric | Pass | Good | Excellent |
|--------|------|------|-----------|
| Rule evaluation (10 rules) | ≤50μs | ≤10μs | ≤5μs |
| Origin resolution | ≤5μs | ≤1μs | ≤500ns |
| BPE tokenize (1k lines) | ≤50ms | ≤20ms | ≤10ms |
| ONNX inference (4k tokens) | ≤500ms | ≤200ms | ≤100ms |
| Combined routing decision | ≤5ms | ≤2ms | ≤1ms |

---

## 7. Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Combined token savings | ≥60% on multi-step agent workflows | A/B test: proxy with all layers ON vs OFF |
| Routing decision latency (rule-based) | ≤5ms p99 | Benchmark suite |
| Routing accuracy (classifier) | ≥90% agreement with human labels | Labeled test set of 100 requests |
| Cost reduction | ≥40% USD savings | Compare API bills before/after |
| Test coverage | ≥90% on routing package | `go test -cover` |
| Zero regressions | No existing tests broken | CI pipeline |

---

## 8. Deployment Considerations

1. **Backward Compatibility**: Routing disabled by default. Existing proxy behavior unchanged.
2. **Model Availability**: Routing to local models requires Ollama/vLLM running. Cloud economy models (Haiku, GPT-4o-mini) are always available.
3. **ONNX Runtime**: Build with `-tags onnx` for neural pruner. Default build uses IR backend only.
4. **Model Download**: ~1.4 GB download for ONNX pruner model. Frontend shows progress.
5. **Fallback Safety**: Every routing rule must have a `fallback` model. Cheaper model failure → original model retry is automatic.

---

## 9. Recommended Models by Tier

**Economy tier (for simple tasks):**

| Model | Tier | Cost | Use Case | Provider |
|-------|------|------|----------|----------|
| Claude Haiku 4.5 | economy | $0.80/M in | Intent classification, file filtering, orchestration | Anthropic |
| GPT-4o-mini | economy | $0.15/M in | Diff planning, prompt compression, summarization | OpenAI |
| Qwen2.5-7B-Instruct | free | $0 | Code-specific tasks (self-hosted) | Ollama |
| Llama-3.1-8B-Instruct | free | $0 | General simple tasks (self-hosted) | Ollama |

**Premium tier (for complex tasks):**

| Model | Tier | Cost | Use Case |
|-------|------|------|----------|
| Claude Opus 4 | premium | $15/M in | Large-scale code generation, deep reasoning |
| Claude Sonnet 4 | standard | $3/M in | Multi-file refactoring, complex debugging |
| GPT-4o | standard | $2.5/M in | General complex tasks |
| DeepSeek R1 | standard | $0.55/M in | Mathematical proofs, algorithm design |

---

## Self-Review Checklist

- [ ] All routing rules have unit tests (written before implementation)
- [ ] Benchmark suite covers all latency-sensitive paths
- [ ] ONNX backend isolated behind build tag (`//go:build onnx`)
- [ ] Model download has progress tracking, cancel support, atomic writes
- [ ] Response headers include tier labeling on every proxied request
- [ ] Fallback chain tested: cheaper model failure → original model retry
- [ ] Config validation: reject rules with missing fallback
- [ ] Frontend: unified Token Economy page with all three layers
- [ ] i18n: English and Chinese locale strings
- [ ] No breaking changes to existing proxy behavior when routing is disabled
