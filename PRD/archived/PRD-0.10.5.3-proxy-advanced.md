# v0.10.5.3 API Proxy Advanced Features & UI

## Overview

This is the third and final phase of the API Proxy Sidecar implementation, adding model compatibility layer, mock endpoints, configuration hot-reload, and UI integration.

## Goals

1. **Model Compatibility Layer** - Handle feature gaps in different models
2. **Mock Endpoints** - Provide mock responses for development/testing
3. **Configuration Hot-Reload** - Update configuration without restart
4. **Data Masking Interface** - Reserved interface for future implementation
5. **UI Integration** - Proxy dashboard and settings panel

## Non-Goals (This Version)

- Full data masking implementation (future)
- CLI daemon mode (depends on CC CLI support)
- Distributed proxy cluster (future)

## Prerequisites

- v0.10.5.1 Core Proxy Infrastructure (completed)
- v0.10.5.2 Security & Monitoring (completed)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│              API Proxy Advanced Features (v0.10.5.3)            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │   Config     │  │   Model      │  │   Mock               │   │
│  │   Watcher    │  │   Compat     │  │   Endpoints          │   │
│  │              │  │   Layer      │  │                      │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘   │
│         │                 │                      │               │
│  ┌──────┴─────────────────┴──────────────────────┴──────────┐   │
│  │                    Request Pipeline                       │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐ │   │
│  │  │ Masking │→│ Model   │→│ Mock    │→│ Forward         │ │   │
│  │  │(Reserve)│ │ Adapt   │ │ Check   │ │ (v0.10.5.1/2)   │ │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────────────┘ │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                         UI Layer                             ││
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   ││
│  │  │   Dashboard  │  │   Settings   │  │   Alerts Panel   │   ││
│  │  └──────────────┘  └──────────────┘  └──────────────────┘   ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Core Features

### 1. Model Compatibility Layer

```go
// internal/proxy/compat.go

// ModelFeatures describes model capabilities
type ModelFeatures struct {
    Model            string `json:"model"`
    ToolCalling      bool   `json:"tool_calling"`
    Vision           bool   `json:"vision"`
    Streaming        bool   `json:"streaming"`
    SystemPrompt     bool   `json:"system_prompt"`
    MaxContextTokens int    `json:"max_context_tokens"`
    MaxOutputTokens  int    `json:"max_output_tokens"`
}

// ModelCompatConfig model compatibility configuration
type ModelCompatConfig struct {
    AutoDetect       bool                      `json:"auto_detect"`
    DetectionCache   time.Duration             `json:"detection_cache"`
    ModelOverrides   map[string]*ModelFeatures `json:"model_overrides"`
    ToolCallFallback string                    `json:"tool_call_fallback"` // error, prompt, skip
}

// ToolCallAdapter adapts tool calls for incompatible models
type ToolCallAdapter struct {
    Mode           string `json:"mode"`           // native, prompt, disabled
    PromptTemplate string `json:"prompt_template"`
    ResponseParser string `json:"response_parser"`
}

// ModelCompatLayer handles model compatibility
type ModelCompatLayer struct {
    config        *ModelCompatConfig
    features      map[string]*ModelFeatures
    detectionTime map[string]time.Time
    mu            sync.RWMutex
}

// NewModelCompatLayer creates a new compatibility layer
func NewModelCompatLayer(config *ModelCompatConfig) *ModelCompatLayer {
    mcl := &ModelCompatLayer{
        config:        config,
        features:      make(map[string]*ModelFeatures),
        detectionTime: make(map[string]time.Time),
    }
    mcl.loadDefaultFeatures()
    return mcl
}

// loadDefaultFeatures loads known model features
func (mcl *ModelCompatLayer) loadDefaultFeatures() {
    defaults := map[string]*ModelFeatures{
        "claude-opus-4-5": {
            Model:            "claude-opus-4-5",
            ToolCalling:      true,
            Vision:           true,
            Streaming:        true,
            SystemPrompt:     true,
            MaxContextTokens: 200000,
            MaxOutputTokens:  32000,
        },
        "claude-sonnet-4-5": {
            Model:            "claude-sonnet-4-5",
            ToolCalling:      true,
            Vision:           true,
            Streaming:        true,
            SystemPrompt:     true,
            MaxContextTokens: 200000,
            MaxOutputTokens:  64000,
        },
        "gpt-4o": {
            Model:            "gpt-4o",
            ToolCalling:      true,
            Vision:           true,
            Streaming:        true,
            SystemPrompt:     true,
            MaxContextTokens: 128000,
            MaxOutputTokens:  16384,
        },
        "llama3": {
            Model:            "llama3",
            ToolCalling:      false,
            Vision:           false,
            Streaming:        true,
            SystemPrompt:     true,
            MaxContextTokens: 8192,
            MaxOutputTokens:  4096,
        },
        "deepseek-chat": {
            Model:            "deepseek-chat",
            ToolCalling:      true,
            Vision:           false,
            Streaming:        true,
            SystemPrompt:     true,
            MaxContextTokens: 64000,
            MaxOutputTokens:  8192,
        },
    }

    for model, features := range defaults {
        mcl.features[model] = features
    }

    // Apply overrides
    for model, override := range mcl.config.ModelOverrides {
        mcl.features[model] = override
    }
}

// GetFeatures returns features for a model
func (mcl *ModelCompatLayer) GetFeatures(model string) *ModelFeatures {
    mcl.mu.RLock()
    defer mcl.mu.RUnlock()

    if features, ok := mcl.features[model]; ok {
        return features
    }

    // Return default features for unknown models
    return &ModelFeatures{
        Model:            model,
        ToolCalling:      false,
        Vision:           false,
        Streaming:        true,
        SystemPrompt:     true,
        MaxContextTokens: 8192,
        MaxOutputTokens:  4096,
    }
}

// AdaptRequest adapts request for model compatibility
func (mcl *ModelCompatLayer) AdaptRequest(model string, req *ChatRequest) (*ChatRequest, error) {
    features := mcl.GetFeatures(model)

    // Handle tool calling
    if len(req.Tools) > 0 && !features.ToolCalling {
        switch mcl.config.ToolCallFallback {
        case "error":
            return nil, ErrToolCallingNotSupported
        case "skip":
            req.Tools = nil
        case "prompt":
            req = mcl.convertToolsToPrompt(req)
        }
    }

    // Handle vision
    if req.HasImages() && !features.Vision {
        return nil, ErrVisionNotSupported
    }

    // Truncate context if needed
    if req.TokenCount() > features.MaxContextTokens {
        req = mcl.truncateContext(req, features.MaxContextTokens)
    }

    return req, nil
}

// convertToolsToPrompt converts tool definitions to prompt
func (mcl *ModelCompatLayer) convertToolsToPrompt(req *ChatRequest) *ChatRequest {
    toolsPrompt := "You have access to the following tools:\n\n"
    for _, tool := range req.Tools {
        toolsPrompt += fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description)
        toolsPrompt += fmt.Sprintf("  Parameters: %s\n\n", tool.Parameters)
    }
    toolsPrompt += "\nTo use a tool, respond with JSON: {\"tool\": \"name\", \"args\": {...}}\n"

    // Prepend to system message
    if req.System != "" {
        req.System = toolsPrompt + "\n" + req.System
    } else {
        req.System = toolsPrompt
    }

    req.Tools = nil
    return req
}

// truncateContext truncates context to fit model limits
func (mcl *ModelCompatLayer) truncateContext(req *ChatRequest, maxTokens int) *ChatRequest {
    // Simple truncation: remove oldest messages
    for req.TokenCount() > maxTokens && len(req.Messages) > 1 {
        req.Messages = req.Messages[1:]
    }
    return req
}
```

### 2. Mock Endpoints

```go
// internal/proxy/mock.go

// MockConfig mock endpoint configuration
type MockConfig struct {
    Enabled   bool            `json:"enabled"`
    Endpoints []*MockEndpoint `json:"endpoints"`
}

// MockEndpoint defines a mock endpoint
type MockEndpoint struct {
    Path       string        `json:"path"`
    Method     string        `json:"method"`
    Response   interface{}   `json:"response"`
    StatusCode int           `json:"status_code"`
    Delay      time.Duration `json:"delay"`
    Enabled    bool          `json:"enabled"`
}

// MockHandler handles mock endpoints
type MockHandler struct {
    config    *MockConfig
    endpoints map[string]*MockEndpoint // path+method -> endpoint
    mu        sync.RWMutex
}

// NewMockHandler creates a new mock handler
func NewMockHandler(config *MockConfig) *MockHandler {
    mh := &MockHandler{
        config:    config,
        endpoints: make(map[string]*MockEndpoint),
    }
    mh.loadDefaultEndpoints()
    mh.indexEndpoints()
    return mh
}

// loadDefaultEndpoints loads built-in mock endpoints
func (mh *MockHandler) loadDefaultEndpoints() {
    defaults := []*MockEndpoint{
        {
            Path:       "/v1/models",
            Method:     "GET",
            StatusCode: 200,
            Response: map[string]interface{}{
                "object": "list",
                "data": []map[string]interface{}{
                    {"id": "claude-opus-4-5", "object": "model"},
                    {"id": "claude-sonnet-4-5", "object": "model"},
                    {"id": "gpt-4o", "object": "model"},
                },
            },
            Enabled: true,
        },
        {
            Path:       "/health",
            Method:     "GET",
            StatusCode: 200,
            Response:   map[string]string{"status": "healthy"},
            Enabled:    true,
        },
        {
            Path:       "/ready",
            Method:     "GET",
            StatusCode: 200,
            Response:   map[string]string{"status": "ready"},
            Enabled:    true,
        },
    }

    mh.config.Endpoints = append(defaults, mh.config.Endpoints...)
}

// indexEndpoints indexes endpoints for fast lookup
func (mh *MockHandler) indexEndpoints() {
    mh.mu.Lock()
    defer mh.mu.Unlock()

    for _, ep := range mh.config.Endpoints {
        if ep.Enabled {
            key := ep.Method + ":" + ep.Path
            mh.endpoints[key] = ep
        }
    }
}

// Handle checks if request matches a mock endpoint
func (mh *MockHandler) Handle(w http.ResponseWriter, r *http.Request) bool {
    if !mh.config.Enabled {
        return false
    }

    mh.mu.RLock()
    key := r.Method + ":" + r.URL.Path
    ep, ok := mh.endpoints[key]
    mh.mu.RUnlock()

    if !ok {
        return false
    }

    // Apply delay
    if ep.Delay > 0 {
        time.Sleep(ep.Delay)
    }

    // Write response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(ep.StatusCode)
    json.NewEncoder(w).Encode(ep.Response)

    return true
}

// AddEndpoint adds a mock endpoint
func (mh *MockHandler) AddEndpoint(ep *MockEndpoint) {
    mh.mu.Lock()
    defer mh.mu.Unlock()

    mh.config.Endpoints = append(mh.config.Endpoints, ep)
    if ep.Enabled {
        key := ep.Method + ":" + ep.Path
        mh.endpoints[key] = ep
    }
}

// RemoveEndpoint removes a mock endpoint
func (mh *MockHandler) RemoveEndpoint(path, method string) {
    mh.mu.Lock()
    defer mh.mu.Unlock()

    key := method + ":" + path
    delete(mh.endpoints, key)

    // Remove from config
    for i, ep := range mh.config.Endpoints {
        if ep.Path == path && ep.Method == method {
            mh.config.Endpoints = append(
                mh.config.Endpoints[:i],
                mh.config.Endpoints[i+1:]...,
            )
            break
        }
    }
}
```

### 3. Configuration Hot-Reload

```go
// internal/proxy/watcher.go

// ConfigWatcher watches for configuration changes
type ConfigWatcher struct {
    configPath   string
    pollInterval time.Duration
    lastModTime  time.Time
    lastHash     string

    OnChange func(oldConfig, newConfig *ProxyConfig)
    OnError  func(error)

    ctx    context.Context
    cancel context.CancelFunc
}

// NewConfigWatcher creates a new config watcher
func NewConfigWatcher(configPath string, pollInterval time.Duration) *ConfigWatcher {
    ctx, cancel := context.WithCancel(context.Background())
    return &ConfigWatcher{
        configPath:   configPath,
        pollInterval: pollInterval,
        ctx:          ctx,
        cancel:       cancel,
    }
}

// Start starts watching for config changes
func (cw *ConfigWatcher) Start() {
    go cw.watch()
}

// Stop stops the config watcher
func (cw *ConfigWatcher) Stop() {
    cw.cancel()
}

// watch is the main watch loop
func (cw *ConfigWatcher) watch() {
    ticker := time.NewTicker(cw.pollInterval)
    defer ticker.Stop()

    // Get initial state
    cw.updateLastState()

    for {
        select {
        case <-cw.ctx.Done():
            return
        case <-ticker.C:
            cw.checkForChanges()
        }
    }
}

// checkForChanges checks if config file has changed
func (cw *ConfigWatcher) checkForChanges() {
    info, err := os.Stat(cw.configPath)
    if err != nil {
        if cw.OnError != nil {
            cw.OnError(err)
        }
        return
    }

    // Check modification time first (fast check)
    if info.ModTime().Equal(cw.lastModTime) {
        return
    }

    // Read and hash file (accurate check)
    data, err := os.ReadFile(cw.configPath)
    if err != nil {
        if cw.OnError != nil {
            cw.OnError(err)
        }
        return
    }

    hash := cw.hashContent(data)
    if hash == cw.lastHash {
        cw.lastModTime = info.ModTime()
        return
    }

    // Config changed, parse and notify
    newConfig, err := cw.parseConfig(data)
    if err != nil {
        if cw.OnError != nil {
            cw.OnError(fmt.Errorf("failed to parse config: %w", err))
        }
        return
    }

    // Load old config for comparison
    oldData, _ := os.ReadFile(cw.configPath + ".old")
    oldConfig, _ := cw.parseConfig(oldData)

    // Update state
    cw.lastModTime = info.ModTime()
    cw.lastHash = hash

    // Save current as old for next comparison
    os.WriteFile(cw.configPath+".old", data, 0644)

    // Notify
    if cw.OnChange != nil {
        cw.OnChange(oldConfig, newConfig)
    }
}

// updateLastState updates the last known state
func (cw *ConfigWatcher) updateLastState() {
    info, err := os.Stat(cw.configPath)
    if err != nil {
        return
    }

    data, err := os.ReadFile(cw.configPath)
    if err != nil {
        return
    }

    cw.lastModTime = info.ModTime()
    cw.lastHash = cw.hashContent(data)
}

// hashContent returns SHA256 hash of content
func (cw *ConfigWatcher) hashContent(data []byte) string {
    h := sha256.Sum256(data)
    return hex.EncodeToString(h[:])
}

// parseConfig parses config from YAML
func (cw *ConfigWatcher) parseConfig(data []byte) (*ProxyConfig, error) {
    var config ProxyConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    return &config, nil
}

// Hot-reloadable configurations:
// - Provider list and priorities
// - Failover settings
// - Prompt guard rules
// - Authentication settings
// - Rate limit settings
// - Mock endpoints

// NOT hot-reloadable (requires restart):
// - Port binding
// - TLS certificates
// - Core proxy settings
```

### 4. Data Masking Interface (Reserved)

```go
// internal/proxy/masking.go

// MaskingConfig data masking configuration (reserved for future)
type MaskingConfig struct {
    Enabled bool           `json:"enabled"`
    Rules   []*MaskingRule `json:"rules"`
    OnMask  func(original, masked string)
}

// MaskingRule defines what to mask
type MaskingRule struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Pattern     string `json:"pattern"`      // Regex pattern
    Replacement string `json:"replacement"`  // e.g., "[REDACTED]", "***"
    Direction   string `json:"direction"`    // request, response, both
    Enabled     bool   `json:"enabled"`
}

// MaskingCategory reserved masking categories
type MaskingCategory string

const (
    MaskingPII         MaskingCategory = "pii"         // Personal Identifiable Information
    MaskingCredentials MaskingCategory = "credentials" // API keys, passwords, tokens
    MaskingFinancial   MaskingCategory = "financial"   // Credit card numbers, bank accounts
    MaskingCustom      MaskingCategory = "custom"      // User-defined patterns
)

// DataMasker handles data masking (stub implementation)
type DataMasker struct {
    config   *MaskingConfig
    compiled map[string]*regexp.Regexp
    mu       sync.RWMutex
}

// NewDataMasker creates a new data masker
func NewDataMasker(config *MaskingConfig) *DataMasker {
    return &DataMasker{
        config:   config,
        compiled: make(map[string]*regexp.Regexp),
    }
}

// Mask masks sensitive data (stub - returns input unchanged)
func (dm *DataMasker) Mask(content string, direction string) string {
    if !dm.config.Enabled {
        return content
    }

    // TODO: Implement in future version
    // For now, return content unchanged
    return content
}

// AddRule adds a masking rule (stub)
func (dm *DataMasker) AddRule(rule *MaskingRule) error {
    // TODO: Implement in future version
    return nil
}
```

## UI Design

### Proxy Dashboard

```
┌─────────────────────────────────────────────────────────────────┐
│  API Proxy Sidecar                                   [Settings] │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐ │
│  │ Status     │  │ Sessions   │  │ Requests   │  │ Blocked    │ │
│  │ ● Running  │  │ Active     │  │ (24h)      │  │ (24h)      │ │
│  │ Port: 9042 │  │    3       │  │   1,523    │  │    5       │ │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘ │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Provider Status                                             ││
│  │  ┌───────────┬────────┬──────────┬─────────┬───────────────┐││
│  │  │ Provider  │ Status │ Requests │ Success │ Circuit       │││
│  │  ├───────────┼────────┼──────────┼─────────┼───────────────┤││
│  │  │ Anthropic │ ●      │ 1,200    │ 98.75%  │ Closed        │││
│  │  │ OpenAI    │ ●      │ 323      │ 94.12%  │ Closed        │││
│  │  │ Ollama    │ ○      │ 0        │ -       │ Closed        │││
│  │  └───────────┴────────┴──────────┴─────────┴───────────────┘││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌──────────────────────────┐  ┌──────────────────────────────┐ │
│  │  Token Usage (24h)       │  │  Security Alerts (24h)       │ │
│  │                          │  │                              │ │
│  │  Input:  2.46M tokens    │  │  ⚠ 5 alerts detected         │ │
│  │  Output: 1.23M tokens    │  │                              │ │
│  │  Cache:  0.58M tokens    │  │  Critical: 2                 │ │
│  │                          │  │  High: 2                     │ │
│  │  Est. Cost: $21.20       │  │  Medium: 1                   │ │
│  │                          │  │                              │ │
│  │  [View Details]          │  │  [View Alerts]               │ │
│  └──────────────────────────┘  └──────────────────────────────┘ │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Latency Metrics                                             ││
│  │                                                              ││
│  │  TTFT (Time to First Token)                                 ││
│  │  ████████████████████████████████  287ms avg (P95: 456ms)   ││
│  │                                                              ││
│  │  Total Response Time                                         ││
│  │  ████████████████████████████████████████████  2.3s avg     ││
│  │                                                              ││
│  │  Proxy Overhead                                              ││
│  │  ██  12ms avg (P95: 25ms)                                   ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Settings Panel

```
┌─────────────────────────────────────────────────────────────────┐
│  Proxy Settings                                          [Save] │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  General                                                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Proxy Enabled    [✓]                                        ││
│  │  Bind Address     [ 127.0.0.1        ]                       ││
│  │  Port Range       [ 9000-9100        ]                       ││
│  │  Load Balancing   [ Priority       ▼ ]                       ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  Providers                                                       │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ Anthropic                                    [✓] [Edit] │││
│  │  │ Priority: 1  |  Endpoint: api.anthropic.com             │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ OpenAI                                       [✓] [Edit] │││
│  │  │ Priority: 2  |  Endpoint: api.openai.com                │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ Ollama                                       [✓] [Edit] │││
│  │  │ Priority: 3  |  Endpoint: localhost:11434               │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │                                           [+ Add Provider]  ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  Security                                                        │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Prompt Guard     [✓]                                        ││
│  │  Guard Mode       [ Block         ▼ ]                        ││
│  │  Authentication   [ ]                                        ││
│  │  Rate Limiting    [ ]                                        ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  Failover                                                        │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Failover Enabled    [✓]                                     ││
│  │  Max Retries         [ 3             ]                       ││
│  │  Retry Delay         [ 1s            ]                       ││
│  │  Circuit Breaker     [✓]                                     ││
│  │  Failure Threshold   [ 5             ]                       ││
│  │  Recovery Timeout    [ 30s           ]                       ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Endpoints

### Configuration

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/config` | Get current configuration |
| PUT | `/api/v1/proxy/config` | Update configuration (hot-reload) |
| POST | `/api/v1/proxy/reload` | Force configuration reload |

### Model Compatibility

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/models` | List known models with features |
| GET | `/api/v1/proxy/models/:name` | Get model features |
| PUT | `/api/v1/proxy/models/:name` | Override model features |

### Mock Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/proxy/mock` | List mock endpoints |
| POST | `/api/v1/proxy/mock` | Add mock endpoint |
| DELETE | `/api/v1/proxy/mock/:path` | Remove mock endpoint |

## Configuration

### config.yaml (v0.10.5.3 additions)

```yaml
proxy:
  # ... v0.10.5.1 & v0.10.5.2 config ...

  # Model compatibility
  model_compat:
    auto_detect: true
    detection_cache: "1h"
    tool_call_fallback: "prompt"  # error, prompt, skip
    model_overrides:
      "llama3":
        tool_calling: false
        vision: false
        streaming: true
        system_prompt: true
        max_context_tokens: 8192
        max_output_tokens: 4096

  # Mock endpoints
  mock:
    enabled: false
    endpoints: []

  # Configuration watching
  config_watch:
    enabled: true
    poll_interval: "10s"

  # Data masking (reserved)
  masking:
    enabled: false
    rules: []
```

## File Structure

```
internal/proxy/
├── ... (v0.10.5.1 & v0.10.5.2 files)
├── compat.go           # Model compatibility layer
├── mock.go             # Mock endpoints
├── watcher.go          # Configuration watcher
└── masking.go          # Data masking (stub)

src/components/proxy/
├── ProxyDashboard.tsx  # Main dashboard
├── ProxySettings.tsx   # Settings panel
├── ProviderStatus.tsx  # Provider status table
├── TokenUsage.tsx      # Token usage display
├── SecurityAlerts.tsx  # Security alerts panel
└── LatencyMetrics.tsx  # Latency visualization
```

## Implementation Checklist

### Phase 1: Model Compatibility

- [ ] Implement `ModelFeatures` struct
- [ ] Implement `ModelCompatLayer`
- [ ] Add default model features
- [ ] Implement tool call adapter
- [ ] Implement context truncation
- [ ] Create model API endpoints
- [ ] Write compatibility tests

### Phase 2: Mock Endpoints

- [ ] Implement `MockEndpoint` struct
- [ ] Implement `MockHandler`
- [ ] Add default mock endpoints
- [ ] Create mock API endpoints
- [ ] Write mock tests

### Phase 3: Configuration Hot-Reload

- [ ] Implement `ConfigWatcher`
- [ ] Add file change detection
- [ ] Implement config parsing
- [ ] Add hot-reload callbacks
- [ ] Create reload API endpoint
- [ ] Write watcher tests

### Phase 4: Data Masking Interface

- [ ] Define `MaskingConfig` struct
- [ ] Define `MaskingRule` struct
- [ ] Create stub `DataMasker`
- [ ] Reserve API endpoints
- [ ] Document future implementation

### Phase 5: UI Integration

- [ ] Create `ProxyDashboard` component
- [ ] Create `ProxySettings` component
- [ ] Create `ProviderStatus` component
- [ ] Create `TokenUsage` component
- [ ] Create `SecurityAlerts` component
- [ ] Create `LatencyMetrics` component
- [ ] Write E2E tests

### Phase 6: Integration & Testing

- [ ] Integrate all v0.10.5.x components
- [ ] Write full integration tests
- [ ] Performance testing
- [ ] Security audit
- [ ] Documentation

## Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Config reload time | < 1s |
| Model detection latency | < 100ms |
| Mock response latency | < 10ms |
| UI render time | < 500ms |

## Success Metrics

| Metric | Target |
|--------|--------|
| Config reload success | 100% |
| Model compatibility accuracy | > 95% |
| UI responsiveness | < 100ms |
| User satisfaction | > 4.5/5 |

## Dependencies

- v0.10.5.1 Core Proxy Infrastructure
- v0.10.5.2 Security & Monitoring
- v0.10.4 Tauri Packaging (for UI)

## Future Enhancements (Post v0.10.5)

1. **Full Data Masking** - Implement PII, credential, financial masking
2. **CLI Daemon Mode** - If CC CLI supports daemon mode
3. **Advanced Auth** - OAuth 2.0, OIDC integration
4. **Distributed Proxy** - Cluster mode for high availability
5. **Request Caching** - Cache common requests
6. **Webhook Alerts** - Send alerts to external systems
