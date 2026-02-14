# PRD: Provider Pool - Unified LLM Provider Management

**Version**: 0.10.6
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-01-28

---

## 1. Overview

### 1.1 Background

ZimaOS-Blue currently supports multiple LLM providers (OpenAI, Anthropic, Ollama, etc.), but each provider is configured independently. Users often have access to multiple AI services through different channels:

- Direct API subscriptions (OpenAI, Anthropic, Google)
- Local IDE integrations (Antigravity, Cursor, Windsurf)
- Third-party aggregators (OpenRouter, AiHubMix)
- Self-hosted models (Ollama, vLLM)

A Provider Pool system will unify these diverse sources, enabling intelligent routing, load balancing, and cost optimization across all available LLM resources.

### 1.2 Goals

1. Provide a unified interface to manage multiple LLM providers
2. Support reusing credentials from local IDE tools (Antigravity, Cursor, etc.)
3. Enable intelligent model routing based on availability, cost, and performance
4. Support custom providers with OpenAI-compatible APIs
5. Provide usage tracking and quota management per provider
6. Support ACP (Agent Communication Protocol) providers for extended capabilities

### 1.3 Non-Goals

1. Building a new LLM inference engine
2. Model fine-tuning or training
3. Provider billing integration (only tracking, not payment)

---

## 2. User Stories

### 2.1 As a User

- I want to connect my existing Antigravity/Cursor subscription to Blue
- I want to see all available models from all my providers in one place
- I want Blue to automatically choose the best available model for my task
- I want to track my usage across different providers

### 2.2 As an Administrator

- I want to configure multiple API keys for the same provider (load balancing)
- I want to set usage limits per provider to control costs
- I want to prioritize certain providers over others
- I want to add custom OpenAI-compatible endpoints

### 2.3 As a Developer

- I want to add new provider integrations easily
- I want to implement custom routing strategies
- I want to access provider health metrics

---

## 3. Functional Requirements

### 3.1 Provider Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-001 | Support built-in providers: OpenAI, Anthropic, Google Gemini, DeepSeek, Moonshot, Azure OpenAI, Grok (xAI), Qwen (Alibaba Cloud) | P0 |
| FR-002 | Support custom OpenAI-compatible providers | P0 |
| FR-003 | Support ACP (Agent Communication Protocol) providers | P1 |
| FR-004 | Support local IDE provider discovery (Antigravity, Cursor) | P1 |
| FR-005 | Provider enable/disable toggle | P0 |
| FR-006 | Provider health check and status monitoring | P1 |

### 3.2 Model Discovery

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-007 | Auto-fetch available models from each provider | P0 |
| FR-008 | Model enable/disable per provider | P0 |
| FR-009 | Model capability tagging (vision, function calling, etc.) | P1 |
| FR-010 | Model search and filtering | P1 |

### 3.3 Credential Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-011 | Secure API key storage with encryption | P0 |
| FR-012 | Multiple API keys per provider (key pool) | P1 |
| FR-013 | OAuth integration for supported providers (see 3.9) | P1 |
| FR-014 | Credential sharing from local IDE tools (see 3.10) | P1 |

### 3.4 Intelligent Routing

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-015 | Automatic provider selection based on model availability | P0 |
| FR-016 | Fallback to alternative providers on failure | P0 |
| FR-017 | Load balancing across multiple API keys | P1 |
| FR-018 | Cost-based routing optimization | P2 |
| FR-019 | Latency-based routing optimization | P2 |

### 3.5 Usage Tracking

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-020 | Track token usage per provider/model | P0 |
| FR-021 | Track request count and latency | P1 |
| FR-022 | Usage quota limits per provider | P1 |
| FR-023 | Usage statistics dashboard | P1 |

### 3.6 Smart Failover & High Availability

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-024 | Classify API errors into retryable vs non-retryable categories | P0 |
| FR-025 | Auto-failover to alternative provider on retryable errors | P0 |
| FR-026 | Support context size exceeded error detection and failover | P0 |
| FR-027 | Support quota/rate limit exceeded error detection and failover | P0 |
| FR-028 | Support model overloaded error detection and failover | P1 |
| FR-029 | Configurable retry strategy per error type | P1 |
| FR-030 | Error classification for streaming responses | P1 |
| FR-031 | Failover metrics and logging | P1 |

### 3.7 Model Pricing Configuration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-032 | Configure custom pricing per model | P1 |
| FR-033 | Set default pricing for unknown models | P1 |
| FR-034 | Provider-specific pricing overrides | P2 |
| FR-035 | Automatic cost recalculation on pricing update | P1 |

### 3.8 API Endpoints

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/api/v1/providers` | GET | List all providers | Required |
| `/api/v1/providers` | POST | Add custom provider | Admin |
| `/api/v1/providers/:id` | GET | Get provider details | Required |
| `/api/v1/providers/:id` | PUT | Update provider config | Admin |
| `/api/v1/providers/:id` | DELETE | Remove provider | Admin |
| `/api/v1/providers/:id/models` | GET | List provider models | Required |
| `/api/v1/providers/:id/models/fetch` | POST | Fetch models from provider | Admin |
| `/api/v1/providers/:id/test` | POST | Test provider connection | Admin |
| `/api/v1/providers/usage` | GET | Get usage statistics | Required |
| `/api/v1/models` | GET | List all available models (aggregated) | Required |
| `/api/v1/pricing` | GET | Get pricing configuration | Required |
| `/api/v1/pricing/default` | PUT | Set default pricing for unknown models | Admin |
| `/api/v1/pricing/models` | GET | List custom model pricing | Required |
| `/api/v1/pricing/models/:modelId` | PUT | Set custom pricing for a model | Admin |
| `/api/v1/pricing/models/:modelId` | DELETE | Remove custom pricing (revert to default) | Admin |
| `/api/v1/pricing/recalculate` | POST | Recalculate costs with current pricing | Admin |
| `/api/v1/providers/:id/oauth/authorize` | POST | Initiate OAuth flow for a provider | Admin |
| `/api/v1/providers/:id/oauth/callback` | GET | OAuth callback handler | System |
| `/api/v1/providers/:id/oauth/refresh` | POST | Force refresh OAuth token | Admin |
| `/api/v1/providers/:id/oauth/quota` | GET | Get OAuth provider quota (e.g. Antigravity pools) | Required |
| `/api/v1/ide/configs` | GET | Get all detected IDE Claude Code extension configs | Required |
| `/api/v1/ide/configs/:ide/import` | POST | Import Claude Code extension config from IDE | Admin |

### 3.9 OAuth-Based Provider Authentication

除了传统的 API Key 认证方式，Provider Pool 还需支持 OAuth 方式接入 LLM 服务。部分 IDE 和平台使用 OAuth 作为主要认证方式，其 token 可以被复用来请求 LLM API。

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-036 | Support Google OAuth2 provider (Antigravity) — 使用 Google OAuth access token 请求 `cloudaicompanion.googleapis.com`，支持 Gemini 和 Claude 模型池 | P1 |
| FR-037 | Support GitHub OAuth provider (Copilot) — 使用 GitHub device flow 获取 Copilot token，请求 Copilot API | P2 |
| FR-038 | OAuth token lifecycle management — 自动刷新 access token（使用 refresh token），处理 token 过期和续期 | P1 |
| FR-039 | OAuth quota pool tracking — 对接 OAuth provider 的用量配额 API（如 Antigravity 的 Gemini/Claude 分池配额），在 UI 展示剩余用量 | P1 |
| FR-040 | OAuth provider 作为 proxy endpoint — OAuth 认证的 provider 可以像 API Key provider 一样参与路由、failover 和负载均衡 | P1 |

**已知支持 OAuth 的 IDE/平台：**

| IDE/Platform | OAuth Provider | Token 存储位置 | API 格式 | 可复用性 |
|-------------|---------------|---------------|---------|---------|
| Antigravity (Google) | Google OAuth2 PKCE | System Keychain (SecretStorage) | Custom Google API (`cloudaicompanion.googleapis.com`) | 高 — 有独立配额池，支持 Gemini + Claude |
| GitHub Copilot | GitHub Device Flow | VS Code SecretStorage + `~/.config/gh/hosts.yml` | Custom Copilot API | 中 — 需要二次 token 交换 |
| Cline | OpenAI OAuth (新增) | VS Code SecretStorage | OpenAI API | 中 — 仅 OpenAI 模型 |
| Kiro (AWS) | GitHub/Google/AWS OAuth | 未公开 | 私有 API (无 BYOK) | 低 — 无法复用 |
| Cursor | 自有账户体系 + SAML SSO | 本地加密存储 | 服务端代理 | 低 — 所有请求经 Cursor 服务器 |
| Windsurf | 自有账户体系 + SAML SSO | 未公开 | 私有 Codeium API | 低 — 无法复用 |

**OAuth Provider 请求流程：**

```
1. 用户在 IDE 中完成 OAuth 登录（如 Google 账号登录 Antigravity）
2. IDE Discovery 扫描到 IDE 安装 + OAuth token（从 keychain 或 config 读取）
3. Provider Pool 注册 OAuth provider，存储 refresh token
4. 请求路由到 OAuth provider 时：
   a. 检查 access token 是否过期
   b. 如过期，使用 refresh token 自动续期
   c. 使用 access token 作为 Bearer token 请求 LLM API
   d. 记录用量，更新配额显示
5. 如 OAuth token 完全失效，标记 provider 为 error 状态，触发 failover
```

### 3.10 IDE Config Extraction (Claude Code Extension Settings)

基于 VS Code 的 IDE（Cursor、Windsurf、Antigravity、Kiro、TRAE、Qoder）在 `settings.json` 中可能包含 Claude Code 扩展的配置，这些配置可以被提取并导入到 Provider Pool。

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-041 | 从 VS Code fork IDE 的 `User/settings.json` 中读取 `claudeCode.environmentVariables` 配置 | P1 |
| FR-042 | 提取 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN` 等环境变量，自动创建对应 provider | P1 |
| FR-043 | 提取 `OPENAI_API_KEY`、`GOOGLE_API_KEY` 等其他 provider 的 key 配置 | P1 |
| FR-044 | IDE Discovery 扫描结果中展示检测到的 Claude Code 扩展配置，支持一键导入 | P1 |

**支持提取的 Claude Code 扩展配置字段：**

```json
{
  "claudeCode.environmentVariables": [
    { "name": "ANTHROPIC_BASE_URL", "value": "https://..." },
    { "name": "ANTHROPIC_AUTH_TOKEN", "value": "sk-..." },
    { "name": "OPENAI_API_KEY", "value": "sk-..." },
    { "name": "ANTHROPIC_API_KEY", "value": "sk-ant-..." },
    { "name": "GOOGLE_API_KEY", "value": "AI..." }
  ]
}
```

**配置文件路径（macOS）：**

| IDE | settings.json 路径 |
|-----|-------------------|
| VS Code | `~/Library/Application Support/Code/User/settings.json` |
| Cursor | `~/Library/Application Support/Cursor/User/settings.json` |
| Windsurf | `~/Library/Application Support/Windsurf/User/settings.json` |
| Antigravity | `~/Library/Application Support/Antigravity/User/settings.json` |
| Kiro | `~/Library/Application Support/Kiro/User/settings.json` |
| TRAE | `~/Library/Application Support/Trae/User/settings.json` |
| Qoder | `~/Library/Application Support/Qoder/User/settings.json` |

**提取流程：**

```
1. IDE Discovery 扫描到 IDE 安装后，读取其 User/settings.json
2. 解析 claudeCode.environmentVariables 数组
3. 匹配已知的环境变量名（ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, etc.）
4. 在 IDE Discovery 结果中展示检测到的配置
5. 用户点击"导入"后，自动创建/更新对应的 provider 配置
   - ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN → 创建 Anthropic (Custom) provider
   - OPENAI_API_KEY → 添加到 OpenAI provider
   - GOOGLE_API_KEY → 添加到 Google provider
```

---

## 4. Technical Design

### 4.1 Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Provider Pool Architecture                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        Provider Registry                             │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │    │
│  │  │  OpenAI  │ │Anthropic │ │  Gemini  │ │ DeepSeek │ │  Custom  │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │    │
│  │  │ Moonshot │ │  Azure   │ │OpenRouter│ │AiHubMix  │ │   Grok   │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │    │
│  │  ┌──────────┐ ┌──────────┐                                          │    │
│  │  │   Qwen   │ │   ACP    │                                          │    │
│  │  └──────────┘ └──────────┘                                          │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      Provider Pool Manager                           │    │
│  │  ┌────────────────┐ ┌────────────────┐ ┌────────────────────────┐   │    │
│  │  │  Model Router  │ │  Key Manager   │ │   Health Monitor       │   │    │
│  │  └────────────────┘ └────────────────┘ └────────────────────────┘   │    │
│  │  ┌────────────────┐ ┌────────────────┐ ┌────────────────────────┐   │    │
│  │  │ Usage Tracker  │ │ Load Balancer  │ │   Fallback Handler     │   │    │
│  │  └────────────────┘ └────────────────┘ └────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Local IDE Discovery                              │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                 │    │
│  │  │  Antigravity │ │    Cursor    │ │   Windsurf   │                 │    │
│  │  │   (Google)   │ │              │ │              │                 │    │
│  │  └──────────────┘ └──────────────┘ └──────────────┘                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Data Models

```go
// Provider represents an LLM provider configuration
type Provider struct {
    ID          string          `json:"id"`
    Name        string          `json:"name"`
    Type        ProviderType    `json:"type"`        // builtin, custom, acp, ide
    Enabled     bool            `json:"enabled"`
    Status      ProviderStatus  `json:"status"`      // active, inactive, error
    BaseURL     string          `json:"base_url,omitempty"`

    // Authentication
    APIKeys     []APIKey        `json:"api_keys,omitempty"`
    OAuth       *OAuthConfig    `json:"oauth,omitempty"`

    // Configuration
    Priority    int             `json:"priority"`    // Higher = preferred
    RateLimit   *RateLimitConfig `json:"rate_limit,omitempty"`

    // Metadata
    Icon        string          `json:"icon,omitempty"`
    Description string          `json:"description,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

type ProviderType string

const (
    ProviderTypeBuiltin ProviderType = "builtin"
    ProviderTypeCustom  ProviderType = "custom"
    ProviderTypeACP     ProviderType = "acp"
    ProviderTypeIDE     ProviderType = "ide"
)

type ProviderStatus string

const (
    ProviderStatusActive   ProviderStatus = "active"
    ProviderStatusInactive ProviderStatus = "inactive"
    ProviderStatusError    ProviderStatus = "error"
)

// APIKey represents a single API key with metadata
type APIKey struct {
    ID          string    `json:"id"`
    Key         string    `json:"-"`              // Never expose in JSON
    KeyHash     string    `json:"key_hash"`       // For identification
    Label       string    `json:"label,omitempty"`
    UsageCount  int64     `json:"usage_count"`
    LastUsed    time.Time `json:"last_used,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
}

// Model represents an available model from a provider
type Model struct {
    ID           string            `json:"id"`
    ProviderID   string            `json:"provider_id"`
    Name         string            `json:"name"`
    DisplayName  string            `json:"display_name"`
    Enabled      bool              `json:"enabled"`

    // Capabilities
    Capabilities ModelCapabilities `json:"capabilities"`

    // Pricing (per 1M tokens)
    InputPrice   float64           `json:"input_price,omitempty"`
    OutputPrice  float64           `json:"output_price,omitempty"`

    // Limits
    ContextWindow int              `json:"context_window,omitempty"`
    MaxOutput     int              `json:"max_output,omitempty"`
}

type ModelCapabilities struct {
    Chat          bool `json:"chat"`
    Completion    bool `json:"completion"`
    Vision        bool `json:"vision"`
    FunctionCall  bool `json:"function_call"`
    Streaming     bool `json:"streaming"`
    Thinking      bool `json:"thinking"`      // Extended thinking mode
}

// UsageRecord tracks usage per provider/model
type UsageRecord struct {
    ID           string    `json:"id"`
    ProviderID   string    `json:"provider_id"`
    ModelID      string    `json:"model_id"`
    Timestamp    time.Time `json:"timestamp"`

    // Token counts
    InputTokens  int64     `json:"input_tokens"`
    OutputTokens int64     `json:"output_tokens"`

    // Request metrics
    RequestCount int64     `json:"request_count"`
    LatencyMs    int64     `json:"latency_ms"`

    // Cost estimation
    EstimatedCost float64  `json:"estimated_cost,omitempty"`
}

// ModelPricing represents custom pricing configuration for a model
type ModelPricing struct {
    ModelID     string    `json:"model_id"`               // Model identifier
    ProviderID  string    `json:"provider_id,omitempty"`  // Optional: specific provider
    InputPrice  float64   `json:"input_price"`            // Price per 1M input tokens (USD)
    OutputPrice float64   `json:"output_price"`           // Price per 1M output tokens (USD)
    CachePrice  float64   `json:"cache_price,omitempty"`  // Price per 1M cache read tokens (USD)
    IsCustom    bool      `json:"is_custom"`              // True if user-defined
    UpdatedAt   time.Time `json:"updated_at"`
}

// PricingConfig holds all pricing configurations
type PricingConfig struct {
    // Default price for unknown models (per 1M tokens)
    DefaultInputPrice  float64 `json:"default_input_price"`
    DefaultOutputPrice float64 `json:"default_output_price"`
    DefaultCachePrice  float64 `json:"default_cache_price"`

    // Custom pricing overrides (model_id -> pricing)
    CustomPricing map[string]*ModelPricing `json:"custom_pricing"`

    // Last update timestamp
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 4.3 IDE Provider Discovery

```go
// IDEProviderDiscovery discovers and connects to local IDE providers
type IDEProviderDiscovery struct {
    providers map[string]*IDEProvider
}

// Supported IDE providers and their config locations
var ideConfigPaths = map[string][]string{
    "antigravity": {
        "~/.config/antigravity/credentials.json",
        "~/Library/Application Support/Antigravity/credentials.json",
    },
    "cursor": {
        "~/.cursor/credentials.json",
        "~/Library/Application Support/Cursor/credentials.json",
    },
    "windsurf": {
        "~/.windsurf/credentials.json",
        "~/Library/Application Support/Windsurf/credentials.json",
    },
}

// DiscoverIDEProviders scans for available IDE providers
func (d *IDEProviderDiscovery) Discover() ([]*Provider, error) {
    var providers []*Provider

    for ideName, paths := range ideConfigPaths {
        for _, path := range paths {
            expandedPath := expandPath(path)
            if _, err := os.Stat(expandedPath); err == nil {
                provider, err := d.loadIDEProvider(ideName, expandedPath)
                if err == nil {
                    providers = append(providers, provider)
                    break
                }
            }
        }
    }

    return providers, nil
}

// ConnectToIDE establishes connection to IDE's API proxy
func (d *IDEProviderDiscovery) ConnectToIDE(ideName string) (*IDEConnection, error) {
    // IDE tools typically expose a local API proxy
    // e.g., Antigravity exposes http://localhost:PORT/v1

    conn := &IDEConnection{
        Name:    ideName,
        BaseURL: d.findIDEProxyURL(ideName),
    }

    // Fetch available models
    models, err := conn.FetchModels()
    if err != nil {
        return nil, err
    }
    conn.Models = models

    return conn, nil
}
```

### 4.3.1 OAuth Provider Integration

```go
// OAuthProvider extends Provider with OAuth-specific token management
type OAuthProvider struct {
    Provider
    TokenManager *OAuthTokenManager
}

// OAuthTokenManager handles OAuth token lifecycle
type OAuthTokenManager struct {
    mu           sync.RWMutex
    accessToken  string
    refreshToken string
    tokenExpiry  time.Time
    tokenURL     string        // Token endpoint for refresh
    clientID     string
    scopes       []string
}

// GetAccessToken returns a valid access token, refreshing if needed
func (tm *OAuthTokenManager) GetAccessToken(ctx context.Context) (string, error) {
    tm.mu.RLock()
    if time.Now().Before(tm.tokenExpiry.Add(-5 * time.Minute)) {
        token := tm.accessToken
        tm.mu.RUnlock()
        return token, nil
    }
    tm.mu.RUnlock()

    // Token expired or about to expire, refresh it
    return tm.refreshAccessToken(ctx)
}

// OAuthProviderType defines known OAuth provider types
type OAuthProviderType string

const (
    OAuthProviderGoogle OAuthProviderType = "google"  // Antigravity
    OAuthProviderGitHub OAuthProviderType = "github"  // Copilot
    OAuthProviderOpenAI OAuthProviderType = "openai"  // Cline OpenAI OAuth
)

// OAuthQuotaPool represents a quota pool for OAuth providers (e.g. Antigravity Gemini/Claude pools)
type OAuthQuotaPool struct {
    Name           string  `json:"name"`            // e.g. "gemini", "claude"
    ModelsIncluded []string `json:"models_included"` // Models in this pool
    RemainingPct   float64 `json:"remaining_pct"`   // 0.0 - 1.0
    ResetTime      string  `json:"reset_time"`      // ISO 8601
}
```

### 4.3.2 IDE Config Extraction (Claude Code Extension)

```go
// IDEClaudeCodeConfig represents Claude Code extension settings found in IDE
type IDEClaudeCodeConfig struct {
    IDEName              string                 `json:"ide_name"`
    IDEType              IDEType                `json:"ide_type"`
    SettingsPath         string                 `json:"settings_path"`
    EnvironmentVariables []ClaudeCodeEnvVar     `json:"environment_variables"`
    SelectedModel        string                 `json:"selected_model,omitempty"`
}

// ClaudeCodeEnvVar represents a single env var from claudeCode.environmentVariables
type ClaudeCodeEnvVar struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

// Known env var names that map to provider configurations
var claudeCodeEnvVarMapping = map[string]struct {
    ProviderID string
    FieldType  string // "api_key", "base_url", "auth_token"
}{
    "ANTHROPIC_API_KEY":    {ProviderID: "anthropic", FieldType: "api_key"},
    "ANTHROPIC_AUTH_TOKEN": {ProviderID: "anthropic", FieldType: "auth_token"},
    "ANTHROPIC_BASE_URL":  {ProviderID: "anthropic", FieldType: "base_url"},
    "OPENAI_API_KEY":      {ProviderID: "openai", FieldType: "api_key"},
    "OPENAI_BASE_URL":     {ProviderID: "openai", FieldType: "base_url"},
    "GOOGLE_API_KEY":      {ProviderID: "google", FieldType: "api_key"},
    "AZURE_OPENAI_API_KEY":     {ProviderID: "azure-openai", FieldType: "api_key"},
    "AZURE_OPENAI_BASE_URL":    {ProviderID: "azure-openai", FieldType: "base_url"},
}

// ExtractClaudeCodeConfig reads Claude Code extension config from IDE settings.json
func ExtractClaudeCodeConfig(settingsPath string) (*IDEClaudeCodeConfig, error) {
    data, err := os.ReadFile(settingsPath)
    if err != nil {
        return nil, err
    }

    var settings map[string]interface{}
    if err := json.Unmarshal(data, &settings); err != nil {
        return nil, err
    }

    // Extract claudeCode.environmentVariables
    envVars, ok := settings["claudeCode.environmentVariables"]
    if !ok {
        return nil, nil // No Claude Code config found
    }

    // Parse env vars array
    config := &IDEClaudeCodeConfig{
        SettingsPath: settingsPath,
    }

    if envList, ok := envVars.([]interface{}); ok {
        for _, item := range envList {
            if envMap, ok := item.(map[string]interface{}); ok {
                config.EnvironmentVariables = append(config.EnvironmentVariables, ClaudeCodeEnvVar{
                    Name:  envMap["name"].(string),
                    Value: envMap["value"].(string),
                })
            }
        }
    }

    // Extract selected model
    if model, ok := settings["claudeCode.selectedModel"].(string); ok {
        config.SelectedModel = model
    }

    return config, nil
}
```

### 4.4 Model Router

```go
// ModelRouter handles intelligent model selection and routing
type ModelRouter struct {
    providers    map[string]*Provider
    models       map[string]*Model
    usageTracker *UsageTracker
    healthCheck  *HealthChecker
}

// Route selects the best provider/model for a request
func (r *ModelRouter) Route(req *RouteRequest) (*RouteResult, error) {
    // 1. Find all providers that have the requested model
    candidates := r.findCandidates(req.ModelID)

    if len(candidates) == 0 {
        return nil, ErrNoAvailableProvider
    }

    // 2. Filter by health status
    healthy := r.filterHealthy(candidates)
    if len(healthy) == 0 {
        // Try unhealthy ones as last resort
        healthy = candidates
    }

    // 3. Sort by routing strategy
    switch req.Strategy {
    case RoutingStrategyPriority:
        sort.Slice(healthy, func(i, j int) bool {
            return healthy[i].Provider.Priority > healthy[j].Provider.Priority
        })
    case RoutingStrategyCost:
        sort.Slice(healthy, func(i, j int) bool {
            return healthy[i].Model.InputPrice < healthy[j].Model.InputPrice
        })
    case RoutingStrategyLatency:
        sort.Slice(healthy, func(i, j int) bool {
            return r.healthCheck.GetLatency(healthy[i].Provider.ID) <
                   r.healthCheck.GetLatency(healthy[j].Provider.ID)
        })
    case RoutingStrategyRoundRobin:
        // Rotate through providers
        healthy = r.roundRobin(healthy)
    }

    // 4. Return best candidate
    return &RouteResult{
        Provider: healthy[0].Provider,
        Model:    healthy[0].Model,
        Fallbacks: healthy[1:],
    }, nil
}

type RoutingStrategy string

const (
    RoutingStrategyPriority   RoutingStrategy = "priority"
    RoutingStrategyCost       RoutingStrategy = "cost"
    RoutingStrategyLatency    RoutingStrategy = "latency"
    RoutingStrategyRoundRobin RoutingStrategy = "round_robin"
)
```

### 4.5 API Error Classification & Smart Failover

```go
// ErrorCategory classifies API errors for failover decisions
type ErrorCategory string

const (
    // Non-retryable errors - do not failover
    ErrorCategoryNonRetryable ErrorCategory = "non_retryable"

    // Retryable with same provider (transient errors)
    ErrorCategoryRetryable ErrorCategory = "retryable"

    // Retryable with different provider (provider-specific limits)
    ErrorCategoryFailover ErrorCategory = "failover"

    // Streaming anomaly - force stop and recover
    ErrorCategoryStreamAnomaly ErrorCategory = "stream_anomaly"
)

// RetryableErrorType defines specific error types that trigger failover
type RetryableErrorType string

const (
    // Context/Token Limits - MUST failover to provider with larger context
    ErrorTypeContextTooLong    RetryableErrorType = "context_too_long"
    ErrorTypeMaxTokensExceeded RetryableErrorType = "max_tokens_exceeded"

    // Rate/Quota Limits - failover to different provider or API key
    ErrorTypeRateLimited       RetryableErrorType = "rate_limited"
    ErrorTypeQuotaExceeded     RetryableErrorType = "quota_exceeded"
    ErrorTypeConcurrencyLimit  RetryableErrorType = "concurrency_limit"

    // Provider Issues - failover to different provider
    ErrorTypeModelOverloaded   RetryableErrorType = "model_overloaded"
    ErrorTypeServiceUnavailable RetryableErrorType = "service_unavailable"
    ErrorTypeTimeout           RetryableErrorType = "timeout"

    // Streaming Anomalies - force stop and attempt recovery
    ErrorTypeRepetitiveOutput  RetryableErrorType = "repetitive_output"
    ErrorTypeInfiniteLoop      RetryableErrorType = "infinite_loop"

    // Non-retryable
    ErrorTypeInvalidRequest    RetryableErrorType = "invalid_request"
    ErrorTypeAuthFailed        RetryableErrorType = "auth_failed"
    ErrorTypeModelNotFound     RetryableErrorType = "model_not_found"
)

// APIErrorClassifier classifies API errors from different providers
type APIErrorClassifier struct {
    patterns map[string][]ErrorPattern
}

// ErrorPattern defines a pattern to match API errors
type ErrorPattern struct {
    Type        RetryableErrorType
    Category    ErrorCategory
    StatusCodes []int              // HTTP status codes to match
    MessagePatterns []string       // Regex patterns for error messages
    ProviderSpecific string        // Optional: only match for specific provider
}

// ClassifyError analyzes response and returns error classification
func (c *APIErrorClassifier) ClassifyError(
    provider string,
    statusCode int,
    responseBody []byte,
) (*ErrorClassification, error) {
    // Parse error response
    var errResp struct {
        Error struct {
            Type    string `json:"type"`
            Message string `json:"message"`
            Code    string `json:"code"`
        } `json:"error"`
    }
    json.Unmarshal(responseBody, &errResp)

    // Match against known patterns
    for _, pattern := range c.patterns[provider] {
        if c.matchPattern(pattern, statusCode, errResp.Error.Message) {
            return &ErrorClassification{
                Type:     pattern.Type,
                Category: pattern.Category,
                Message:  errResp.Error.Message,
                Retryable: pattern.Category != ErrorCategoryNonRetryable,
                ShouldFailover: pattern.Category == ErrorCategoryFailover,
            }, nil
        }
    }

    // Default classification based on status code
    return c.defaultClassification(statusCode, errResp.Error.Message), nil
}

// ErrorClassification result
type ErrorClassification struct {
    Type           RetryableErrorType
    Category       ErrorCategory
    Message        string
    Retryable      bool
    ShouldFailover bool

    // For context errors, suggest minimum context window needed
    SuggestedContextWindow int

    // For rate limits, suggest retry delay
    RetryAfter time.Duration
}

// Known error patterns for major providers
var defaultErrorPatterns = map[string][]ErrorPattern{
    "anthropic": {
        {
            Type:        ErrorTypeContextTooLong,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400},
            MessagePatterns: []string{
                `context size \((\d+) tokens\) exceeds maximum`,
                `Request context size.*exceeds maximum allowed`,
            },
        },
        {
            Type:        ErrorTypeRateLimited,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`rate_limit`, `too many requests`},
        },
        {
            Type:        ErrorTypeQuotaExceeded,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400, 429},
            MessagePatterns: []string{`quota`, `credit`, `billing`},
        },
        {
            Type:        ErrorTypeModelOverloaded,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{529},
            MessagePatterns: []string{`overloaded`},
        },
    },
    "openai": {
        {
            Type:        ErrorTypeContextTooLong,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400},
            MessagePatterns: []string{
                `maximum context length`,
                `tokens.*exceeds.*limit`,
            },
        },
        {
            Type:        ErrorTypeRateLimited,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`rate_limit_exceeded`},
        },
        {
            Type:        ErrorTypeQuotaExceeded,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`insufficient_quota`, `billing`},
        },
    },
    "deepseek": {
        {
            Type:        ErrorTypeContextTooLong,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400},
            MessagePatterns: []string{`context.*too long`, `token.*limit`},
        },
        {
            Type:        ErrorTypeRateLimited,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`rate.*limit`},
        },
    },
    "grok": {
        {
            Type:        ErrorTypeContextTooLong,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400},
            MessagePatterns: []string{`context.*too long`, `maximum context length`, `token.*limit`},
        },
        {
            Type:        ErrorTypeRateLimited,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`rate_limit`, `too many requests`},
        },
        {
            Type:        ErrorTypeQuotaExceeded,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`quota`, `insufficient_quota`},
        },
    },
    "qwen": {
        {
            Type:        ErrorTypeContextTooLong,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400},
            MessagePatterns: []string{`context.*exceeds`, `token.*limit`, `input.*too.*long`},
        },
        {
            Type:        ErrorTypeRateLimited,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{429},
            MessagePatterns: []string{`Throttling`, `rate.*limit`, `QPS.*limit`},
        },
        {
            Type:        ErrorTypeQuotaExceeded,
            Category:    ErrorCategoryFailover,
            StatusCodes: []int{400, 429},
            MessagePatterns: []string{`quota`, `balance.*insufficient`, `Arrearage`},
        },
    },
}
```

### 4.6 Streaming Anomaly Detection

```go
// StreamingAnomalyDetector detects repetitive/looping output in streaming responses
type StreamingAnomalyDetector struct {
    windowSize     int           // Sliding window size for pattern detection
    repeatThreshold int          // Number of repeats to trigger anomaly
    minPatternLen  int           // Minimum pattern length to consider
}

// NewStreamingAnomalyDetector creates a new detector
func NewStreamingAnomalyDetector() *StreamingAnomalyDetector {
    return &StreamingAnomalyDetector{
        windowSize:      4096,  // 4KB sliding window
        repeatThreshold: 3,     // 3 consecutive repeats
        minPatternLen:   20,    // Minimum 20 chars pattern
    }
}

// StreamBuffer maintains a sliding window of streamed content
type StreamBuffer struct {
    buffer    []byte
    maxSize   int
    detector  *StreamingAnomalyDetector

    // Pattern tracking
    lastChunks []string
    anomalyDetected bool
}

// Write adds content to buffer and checks for anomalies
func (sb *StreamBuffer) Write(chunk []byte) (anomaly *StreamAnomaly, err error) {
    // Add to sliding window
    sb.buffer = append(sb.buffer, chunk...)
    if len(sb.buffer) > sb.maxSize {
        sb.buffer = sb.buffer[len(sb.buffer)-sb.maxSize:]
    }

    // Track recent chunks for repetition detection
    chunkStr := string(chunk)
    sb.lastChunks = append(sb.lastChunks, chunkStr)
    if len(sb.lastChunks) > 10 {
        sb.lastChunks = sb.lastChunks[1:]
    }

    // Check for repetitive patterns
    if anomaly := sb.detectRepetition(); anomaly != nil {
        sb.anomalyDetected = true
        return anomaly, nil
    }

    return nil, nil
}

// detectRepetition uses sliding window to detect repetitive output
func (sb *StreamBuffer) detectRepetition() *StreamAnomaly {
    if len(sb.buffer) < sb.detector.minPatternLen * sb.detector.repeatThreshold {
        return nil
    }

    // Try different pattern lengths
    for patternLen := sb.detector.minPatternLen; patternLen <= len(sb.buffer)/sb.detector.repeatThreshold; patternLen++ {
        pattern := sb.buffer[len(sb.buffer)-patternLen:]

        // Count consecutive occurrences
        count := 1
        pos := len(sb.buffer) - patternLen*2
        for pos >= 0 {
            if bytes.Equal(sb.buffer[pos:pos+patternLen], pattern) {
                count++
                pos -= patternLen
            } else {
                break
            }
        }

        if count >= sb.detector.repeatThreshold {
            return &StreamAnomaly{
                Type:        ErrorTypeRepetitiveOutput,
                Pattern:     string(pattern),
                RepeatCount: count,
                Message:     fmt.Sprintf("Detected %d consecutive repetitions of %d-char pattern", count, patternLen),
            }
        }
    }

    return nil
}

// StreamAnomaly represents a detected streaming anomaly
type StreamAnomaly struct {
    Type        RetryableErrorType
    Pattern     string
    RepeatCount int
    Message     string
}

// StreamRecoveryStrategy defines how to recover from streaming anomalies
type StreamRecoveryStrategy struct {
    // Force stop the current stream
    ForceStop bool

    // Retry with modifications
    RetryWithTruncation bool   // Truncate context and retry
    TruncateToTokens    int    // Target token count after truncation

    // Failover to different provider
    FailoverToProvider string

    // Add stop sequences to prevent loop
    AddStopSequences []string
}

// GetRecoveryStrategy returns appropriate recovery strategy for anomaly
func GetRecoveryStrategy(anomaly *StreamAnomaly) *StreamRecoveryStrategy {
    switch anomaly.Type {
    case ErrorTypeRepetitiveOutput:
        return &StreamRecoveryStrategy{
            ForceStop:           true,
            RetryWithTruncation: true,
            TruncateToTokens:    0, // Will be calculated based on current context
            AddStopSequences:    []string{anomaly.Pattern[:min(50, len(anomaly.Pattern))]},
        }
    case ErrorTypeInfiniteLoop:
        return &StreamRecoveryStrategy{
            ForceStop:          true,
            FailoverToProvider: "", // Will select next available
        }
    default:
        return &StreamRecoveryStrategy{
            ForceStop: true,
        }
    }
}
```

### 4.7 Enhanced Failover Handler

```go
// EnhancedFailoverHandler extends FailoverHandler with smart error classification
type EnhancedFailoverHandler struct {
    *FailoverHandler
    classifier      *APIErrorClassifier
    anomalyDetector *StreamingAnomalyDetector
    metrics         *FailoverMetrics
}

// ExecuteWithSmartFailover executes request with intelligent failover
func (efh *EnhancedFailoverHandler) ExecuteWithSmartFailover(
    ctx context.Context,
    provider *Provider,
    req *http.Request,
    fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {

    resp, err := fn(provider)

    // Check for errors that need classification
    if err != nil || (resp != nil && resp.StatusCode >= 400) {
        classification := efh.classifyResponse(provider.Config.Name, resp)

        efh.metrics.RecordError(provider.Config.Name, classification)

        switch classification.Category {
        case ErrorCategoryFailover:
            // Find alternative provider based on error type
            altProvider := efh.selectAlternativeProvider(provider, classification)
            if altProvider != nil {
                efh.metrics.RecordFailover(provider.Config.Name, altProvider.Config.Name, classification.Type)
                return fn(altProvider)
            }

        case ErrorCategoryRetryable:
            // Retry with same provider after delay
            if classification.RetryAfter > 0 {
                time.Sleep(classification.RetryAfter)
            }
            return fn(provider)
        }
    }

    return resp, err
}

// selectAlternativeProvider selects best alternative based on error type
func (efh *EnhancedFailoverHandler) selectAlternativeProvider(
    failed *Provider,
    classification *ErrorClassification,
) *Provider {
    providers := efh.router.GetAvailableProviders()

    for _, p := range providers {
        if p.Config.Name == failed.Config.Name {
            continue
        }

        // For context errors, check if provider supports larger context
        if classification.Type == ErrorTypeContextTooLong {
            if p.MaxContextWindow >= classification.SuggestedContextWindow {
                return p
            }
            continue
        }

        // For quota errors, prefer providers with remaining quota
        if classification.Type == ErrorTypeQuotaExceeded {
            if efh.hasRemainingQuota(p) {
                return p
            }
            continue
        }

        // Default: return first available healthy provider
        if efh.isHealthy(p) {
            return p
        }
    }

    return nil
}

// FailoverMetrics tracks failover statistics
type FailoverMetrics struct {
    mu sync.RWMutex

    // Error counts by type
    ErrorsByType map[RetryableErrorType]int64

    // Failover counts
    FailoverCount    int64
    FailoverSuccess  int64
    FailoverFailure  int64

    // Provider-specific stats
    ProviderErrors   map[string]map[RetryableErrorType]int64
    ProviderFailovers map[string]int64
}
```

### 4.8 Configuration

```yaml
provider_pool:
  enabled: true

  # Default routing strategy
  routing_strategy: priority  # priority, cost, latency, round_robin

  # Health check settings
  health_check:
    enabled: true
    interval: 60s
    timeout: 10s

  # Smart Failover settings
  failover:
    enabled: true
    max_retries: 2
    retry_delay: 1s

    # Error classification
    error_classification:
      enabled: true
      # Errors that trigger failover to different provider
      failover_errors:
        - context_too_long
        - quota_exceeded
        - rate_limited
        - model_overloaded
        - service_unavailable
      # Errors that trigger retry with same provider
      retryable_errors:
        - timeout
        - temporary_error

    # Streaming anomaly detection
    streaming_anomaly:
      enabled: true
      window_size: 4096          # Sliding window size in bytes
      repeat_threshold: 3        # Number of repeats to trigger
      min_pattern_length: 20     # Minimum pattern length
      recovery_strategy: truncate_and_retry  # truncate_and_retry, failover, stop

    # Circuit breaker
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      recovery_timeout: 30s

  # IDE discovery
  ide_discovery:
    enabled: true
    scan_interval: 300s
    supported:
      - antigravity
      - cursor
      - windsurf

  # Usage tracking
  usage_tracking:
    enabled: true
    retention_days: 30

  # Built-in providers (can be overridden)
  providers:
    openai:
      enabled: false
      base_url: "https://api.openai.com/v1"

    anthropic:
      enabled: false
      base_url: "https://api.anthropic.com"

    google:
      enabled: false
      base_url: "https://generativelanguage.googleapis.com"

    deepseek:
      enabled: false
      base_url: "https://api.deepseek.com/v1"

    moonshot:
      enabled: false
      base_url: "https://api.moonshot.cn/v1"

    grok:
      enabled: false
      base_url: "https://api.x.ai/v1"

    qwen:
      enabled: false
      base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"

    openrouter:
      enabled: false
      base_url: "https://openrouter.ai/api/v1"
```

### 4.9 Data Storage

Following the project's preference for human-readable files:

```
data/providers/
├── providers.json           # Provider configurations
├── models/
│   ├── openai.json         # Cached models per provider
│   ├── anthropic.json
│   └── ...
├── usage/
│   ├── 2026-01-28.jsonl    # Daily usage logs
│   └── ...
└── health/
    └── status.json          # Current health status
```

---

## 5. UI/UX Design

### 5.1 Provider List

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 🏪 Providers                                                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  [🔍 Search providers...]        [>_ Add Custom ACP Provider] [+ Add Custom] │
│                                                                              │
│  ┌─────────────────────────────┐  ┌─────────────────────────────────────┐   │
│  │                             │  │                                     │   │
│  │  ⚡ Antigravity (Google)  ● │  │  Antigravity (Google)               │   │
│  │  🤖 OpenAI                ○ │  │  [PLUGIN] [Connected]    [→ Logout] │   │
│  │  🅰️ Anthropic             ○ │  │                                     │   │
│  │  ◆ Google Gemini          ○ │  │  👤 orca.zhang@yahoo.com            │   │
│  │  🌐 AiHubMix              ○ │  │     Claude 100%  Gemini 0%          │   │
│  │  🦈 DeepSeek              ○ │  │                                     │   │
│  │  🌙 Moonshot              ○ │  │  ═══════════════════════════════    │   │
│  │  🤖 Grok (xAI)            ○ │  │  📋 API Proxy Endpoint    [Advanced]│   │
│  │  🔷 Qwen (Alibaba)        ○ │  │                                     │   │
│  │  Ⓩ Z.AI Coding Plan       ○ │  │                                     │   │
│  │  ↔️ OpenRouter            ○ │  │                                     │   │
│  │  🔷 Azure OpenAI          ○ │  │  Available Models (38)    [↓ Fetch] │   │
│  │                             │  │  [🔍 Search models...]              │   │
│  │                             │  │                                     │   │
│  │                             │  │  ☑️ Claude Opus 4.5 (Thinking)      │   │
│  │                             │  │     claude-opus-4-5-thinking        │   │
│  │                             │  │  ☑️ Claude Opus 4.5 (High Thinking) │   │
│  │                             │  │     claude-opus-4-5-thinking-high   │   │
│  │                             │  │  ☑️ Claude Opus 4.5 (Low Thinking)  │   │
│  │                             │  │     claude-opus-4-5-thinking-low    │   │
│  │                             │  │                                     │   │
│  └─────────────────────────────┘  └─────────────────────────────────────┘   │
│                                                                              │
│  [All changes saved]                              [Close]        [Save]      │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Custom Provider Configuration

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Add Custom Provider                                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Provider Name:  [________________________]                                  │
│                                                                              │
│  Base URL:       [https://api.example.com/v1____]                           │
│                  OpenAI-compatible API endpoint                              │
│                                                                              │
│  API Key:        [________________________________] 👁️                       │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ Advanced Settings                                              [▼] │    │
│  ├─────────────────────────────────────────────────────────────────────┤    │
│  │                                                                     │    │
│  │  Priority:        [5____] (1-10, higher = preferred)               │    │
│  │                                                                     │    │
│  │  Rate Limit:      [60___] requests per [minute ▼]                  │    │
│  │                                                                     │    │
│  │  Custom Headers:                                                    │    │
│  │  [X-Custom-Header] : [value]                        [+ Add Header] │    │
│  │                                                                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│                                        [Test Connection]  [Cancel]  [Save]   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Usage Statistics

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 📊 Usage Statistics                                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Period: [Last 7 days ▼]                                                    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Total Tokens: 1,234,567                                            │    │
│  │  Total Requests: 456                                                │    │
│  │  Estimated Cost: $12.34                                             │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  By Provider:                                                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Antigravity    ████████████████████████░░░░░░  78%   $9.62         │    │
│  │  OpenAI         ████████░░░░░░░░░░░░░░░░░░░░░░  15%   $1.85         │    │
│  │  DeepSeek       ████░░░░░░░░░░░░░░░░░░░░░░░░░░   7%   $0.87         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  By Model:                                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  claude-opus-4-5-thinking      456,789 tokens    $5.67              │    │
│  │  gpt-4o                        234,567 tokens    $2.34              │    │
│  │  deepseek-chat                 123,456 tokens    $0.12              │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Security Considerations

### 6.1 Credential Security

- API keys encrypted at rest using AES-256-GCM
- Keys never exposed in API responses (only hash for identification)
- Secure memory handling for keys in transit
- Audit logging for credential access

### 6.2 IDE Integration Security

- Only read credentials from known, trusted paths
- Validate IDE process before connecting
- No credential modification in IDE config files
- User consent required before IDE discovery
- OAuth refresh tokens encrypted at rest, same as API keys
- Claude Code extension env vars (from IDE settings.json) treated as sensitive — masked in UI, encrypted in storage

### 6.3 Network Security

- All provider connections use HTTPS
- Certificate validation enforced
- Proxy support for corporate environments
- Request/response logging (with sensitive data redaction)

---

## 7. Testing Strategy

### 7.1 Unit Tests

- Provider configuration parsing
- Model routing logic
- Usage calculation
- Credential encryption/decryption
- **API error classification logic**
- **Streaming anomaly detection algorithm**
- **Failover decision logic**

### 7.2 Integration Tests

- Provider API connectivity
- IDE discovery and connection
- Fallback behavior
- Load balancing distribution
- **Error classification with real API responses**
- **Failover chain execution**
- **Streaming anomaly detection with mock streams**

### 7.3 E2E Tests

- Full request flow through provider pool
- Multi-provider failover scenario
- Usage tracking accuracy
- UI provider management
- **Context size exceeded failover scenario**
- **Rate limit failover scenario**
- **Repetitive output detection and recovery**

---

## 8. Rollout Plan

### Phase 1: Alpha (v0.9-alpha)
- Basic provider management UI
- OpenAI and Anthropic support
- Manual API key configuration

### Phase 2: Beta (v0.9-beta)
- IDE provider discovery
- Model routing and fallback
- Usage tracking

### Phase 3: Stable (v0.9)
- Full provider support
- ACP integration
- Production ready

---

## 9. Metrics & Monitoring

### 9.1 Metrics to Track

- `blue_provider_requests_total`: Requests per provider
- `blue_provider_errors_total`: Errors per provider
- `blue_provider_latency_seconds`: Request latency histogram
- `blue_provider_tokens_total`: Token usage per provider
- `blue_provider_health_status`: Provider health (0/1)
- **`blue_failover_total`**: Total failover events
- **`blue_failover_success_total`**: Successful failovers
- **`blue_error_by_type_total`**: Errors by classification type
- **`blue_streaming_anomaly_total`**: Streaming anomaly detections
- **`blue_context_exceeded_total`**: Context size exceeded errors

### 9.2 Alerts

- Provider health degradation
- High error rate (> 5%)
- Quota approaching limit
- All providers unavailable
- **High failover rate (> 10% of requests)**
- **Streaming anomaly spike**
- **Context exceeded errors increasing**

---

## 10. Open Questions

1. Should we support provider-specific features (e.g., Anthropic's extended thinking)?
2. How to handle model version updates from providers?
3. Should we implement request caching for identical prompts?
4. How to handle rate limiting across multiple API keys?
5. **What is the optimal sliding window size for streaming anomaly detection?**
6. **Should we support session recovery after streaming anomaly (continue from last valid output)?**
7. **How to handle partial streaming responses when failover occurs mid-stream?**
8. **Should we implement automatic context truncation strategies for context-exceeded errors?**
9. **How to handle OAuth token refresh failures — should we prompt user to re-authenticate in IDE?**
10. **Should we support importing Claude Code extension configs from multiple IDEs simultaneously (merge vs override)?**
11. **For Antigravity OAuth, should we call Google's quota API periodically or only on-demand?**
12. **Should GitHub Copilot OAuth be supported given its token exchange complexity?**

---

## 11. References

- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- [Anthropic API Reference](https://docs.anthropic.com/en/api)
- [OpenRouter API](https://openrouter.ai/docs)
- [ACP Specification](https://github.com/anthropics/anthropic-cookbook/tree/main/misc/acp)
