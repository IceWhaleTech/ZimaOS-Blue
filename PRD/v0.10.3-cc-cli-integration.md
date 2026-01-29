# PRD: v0.10 Claude Code CLI Integration

**Status:** Draft
**Author:** ZimaOS Team
**Last Updated:** 2026-01-29
**Target Version:** v0.10.3

---

## Overview

This PRD defines the Tauri desktop application packaging strategy for ZimaOS-Echo, with a focus on Claude Code CLI integration. The goal is to provide a seamless desktop experience where users can optionally leverage Claude Code CLI for enhanced functionality while maintaining full usability without it.

### Core Principles

1. **Optional CLI Integration** - Claude Code CLI is optional; core functionality works without it
2. **Guided First-Run Experience** - First-run wizard guides users through CLI setup
3. **Transparent Download Process** - Clear size indication and progress tracking
4. **Multi-Provider Support** - Auto-detect Ollama and other local providers
5. **Environment Awareness** - Read and respect existing environment variables
6. **Separated Configuration** - Claude Code CLI configuration is independent from LLM provider settings

## Goals

- [ ] Provide first-run wizard for Claude Code CLI setup
- [ ] Support running without Claude Code CLI (limited functionality mode)
- [ ] Auto-detect Ollama and other local LLM providers
- [ ] Integrate with cc-switch for provider management
- [ ] Read and utilize environment variables for configuration
- [ ] Collect usage statistics (optional, with user consent)

## User Stories

### Story 1: First-Run Setup Wizard

**As a** new user
**I want to** be guided through the initial setup process
**So that** I can quickly start using the application with optimal configuration

**Acceptance Criteria:**
- [ ] Display welcome screen on first launch
- [ ] Detect existing Claude Code CLI installation
- [ ] Show CLI download size before downloading (~120MB)
- [ ] Display download progress with percentage and speed
- [ ] Allow skipping CLI download (limited mode)
- [ ] Detect and configure Ollama if available

### Story 2: Limited Functionality Mode

**As a** user who doesn't want to download Claude Code CLI
**I want to** use the application with limited features
**So that** I can still benefit from basic functionality

**Acceptance Criteria:**
- [ ] Clear indication of which features require CLI
- [ ] Graceful degradation for CLI-dependent features
- [ ] Persistent reminder with option to download CLI later
- [ ] Full functionality with Ollama or other local providers

### Story 3: Provider Auto-Detection

**As a** user with existing LLM providers
**I want to** have my providers automatically detected
**So that** I can start using them immediately without manual configuration

**Acceptance Criteria:**
- [ ] Auto-detect Ollama on standard ports (11434)
- [ ] Auto-detect Claude Code CLI in PATH
- [ ] Read API keys from environment variables
- [ ] Support cc-switch configuration files

### Story 4: Usage Statistics Collection

**As a** developer
**I want to** collect anonymized usage statistics
**So that** I can improve the application based on real usage patterns

**Acceptance Criteria:**
- [ ] Opt-in statistics collection with clear consent
- [ ] Collect API call counts, token usage, error rates
- [ ] Support both CLI-based and direct API statistics
- [ ] Provide statistics dashboard in settings

## Requirements

### Functional Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| FR-001 | First-run setup wizard | P0 | Guide users through initial configuration |
| FR-002 | CLI download with progress | P0 | Show size, progress, speed |
| FR-003 | Skip CLI option | P0 | Allow running without CLI |
| FR-004 | Limited mode indicators | P0 | Show which features are unavailable |
| FR-005 | Ollama auto-detection | P0 | Detect on localhost:11434 |
| FR-006 | Environment variable reading | P0 | Read ANTHROPIC_API_KEY, etc. |
| FR-007 | cc-switch integration | P1 | Read/write cc-switch config |
| FR-008 | Usage statistics collection | P1 | Opt-in metrics collection |
| FR-009 | Statistics dashboard | P1 | View usage in settings |
| FR-010 | Provider health check | P2 | Verify provider connectivity |
| FR-011 | Multi-provider fallback | P2 | Automatic failover |

### Non-Functional Requirements

| ID | Requirement | Target | Notes |
|----|-------------|--------|-------|
| NFR-001 | First-run wizard load time | < 1s | Instant startup |
| NFR-002 | Provider detection time | < 3s | All providers |
| NFR-003 | CLI download resume | Yes | Support interrupted downloads |
| NFR-004 | Offline functionality | Basic features | Without network |
| NFR-005 | Memory footprint | < 200MB | Without CLI running |

## Technical Design

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ZimaOS-Echo Tauri Application                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     First-Run Wizard                                 │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌────────────┐  │    │
│  │  │   Welcome    │ │   Provider   │ │     CLI      │ │   Setup    │  │    │
│  │  │    Screen    │→│  Detection   │→│   Download   │→│  Complete  │  │    │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Provider Manager                                 │    │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────────────┐  │    │
│  │  │  Claude    │ │   Ollama   │ │  OpenAI    │ │   Environment    │  │    │
│  │  │  Code CLI  │ │  Detector  │ │  Detector  │ │   Var Reader     │  │    │
│  │  └────────────┘ └────────────┘ └────────────┘ └──────────────────┘  │    │
│  │                                                                      │    │
│  │  ┌────────────────────────────────────────────────────────────────┐ │    │
│  │  │                    cc-switch Integration                        │ │    │
│  │  │  - Read ~/.claude-code-switch/config.json                      │ │    │
│  │  │  - Sync provider configurations                                 │ │    │
│  │  │  - Support provider switching                                   │ │    │
│  │  └────────────────────────────────────────────────────────────────┘ │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Statistics Collector                             │    │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────────────┐  │    │
│  │  │   API      │ │   Token    │ │   Error    │ │   Performance    │  │    │
│  │  │   Calls    │ │   Usage    │ │   Rates    │ │   Metrics        │  │    │
│  │  └────────────┘ └────────────┘ └────────────┘ └──────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### First-Run Wizard Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         First-Run Wizard Flow                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐                                                           │
│  │   Welcome    │                                                           │
│  │   Screen     │                                                           │
│  └──────┬───────┘                                                           │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────┐     ┌──────────────┐                                      │
│  │   Detect     │────▶│   Found      │                                      │
│  │   Providers  │     │   Ollama?    │                                      │
│  └──────────────┘     └──────┬───────┘                                      │
│                              │                                               │
│         ┌────────────────────┼────────────────────┐                         │
│         ▼                    ▼                    ▼                         │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                 │
│  │   Ollama     │     │   CLI in     │     │   No Local   │                 │
│  │   Detected   │     │   PATH?      │     │   Provider   │                 │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                 │
│         │                    │                    │                         │
│         ▼                    ▼                    ▼                         │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                 │
│  │   Configure  │     │   Use        │     │   Download   │                 │
│  │   Ollama     │     │   Existing   │     │   CLI?       │                 │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                 │
│         │                    │                    │                         │
│         │                    │         ┌─────────┴─────────┐                │
│         │                    │         ▼                   ▼                │
│         │                    │  ┌──────────────┐   ┌──────────────┐         │
│         │                    │  │   Download   │   │   Skip       │         │
│         │                    │  │   (~120MB)   │   │   (Limited)  │         │
│         │                    │  └──────┬───────┘   └──────┬───────┘         │
│         │                    │         │                  │                 │
│         └────────────────────┴─────────┴──────────────────┘                 │
│                                        │                                    │
│                                        ▼                                    │
│                               ┌──────────────┐                              │
│                               │   Setup      │                              │
│                               │   Complete   │                              │
│                               └──────────────┘                              │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Provider Detection

#### Ollama Detection

```go
// OllamaDetector detects Ollama installation
type OllamaDetector struct {
    Endpoints []string // Default: ["http://localhost:11434"]
}

func (d *OllamaDetector) Detect() (*OllamaInfo, error) {
    for _, endpoint := range d.Endpoints {
        resp, err := http.Get(endpoint + "/api/tags")
        if err != nil {
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode == 200 {
            var tags OllamaTags
            json.NewDecoder(resp.Body).Decode(&tags)
            return &OllamaInfo{
                Endpoint: endpoint,
                Models:   tags.Models,
                Version:  resp.Header.Get("X-Ollama-Version"),
            }, nil
        }
    }
    return nil, ErrOllamaNotFound
}

type OllamaInfo struct {
    Endpoint string   `json:"endpoint"`
    Models   []string `json:"models"`
    Version  string   `json:"version"`
}
```

#### Environment Variable Reading

```go
// EnvironmentReader reads configuration from environment variables
type EnvironmentReader struct{}

func (r *EnvironmentReader) ReadConfig() *EnvConfig {
    return &EnvConfig{
        // Anthropic
        AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),

        // OpenAI
        OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
        OpenAIBaseURL:   os.Getenv("OPENAI_BASE_URL"),

        // Claude Code specific
        ClaudeCodeModel: os.Getenv("CLAUDE_CODE_MODEL"),
        ClaudeCodeMaxTokens: os.Getenv("CLAUDE_CODE_MAX_TOKENS"),

        // Proxy settings
        HTTPProxy:       os.Getenv("HTTP_PROXY"),
        HTTPSProxy:      os.Getenv("HTTPS_PROXY"),
        NoProxy:         os.Getenv("NO_PROXY"),

        // cc-switch
        CCSwitchProfile: os.Getenv("CC_SWITCH_PROFILE"),
    }
}

type EnvConfig struct {
    AnthropicAPIKey     string `json:"anthropic_api_key,omitempty"`
    OpenAIAPIKey        string `json:"openai_api_key,omitempty"`
    OpenAIBaseURL       string `json:"openai_base_url,omitempty"`
    ClaudeCodeModel     string `json:"claude_code_model,omitempty"`
    ClaudeCodeMaxTokens string `json:"claude_code_max_tokens,omitempty"`
    HTTPProxy           string `json:"http_proxy,omitempty"`
    HTTPSProxy          string `json:"https_proxy,omitempty"`
    NoProxy             string `json:"no_proxy,omitempty"`
    CCSwitchProfile     string `json:"cc_switch_profile,omitempty"`
}
```

#### cc-switch Integration

```go
// CCSwitchIntegration integrates with cc-switch configuration
type CCSwitchIntegration struct {
    ConfigPath string // Default: ~/.claude-code-switch/config.json
}

func (c *CCSwitchIntegration) ReadConfig() (*CCSwitchConfig, error) {
    configPath := c.ConfigPath
    if configPath == "" {
        home, _ := os.UserHomeDir()
        configPath = filepath.Join(home, ".claude-code-switch", "config.json")
    }

    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, err
    }

    var config CCSwitchConfig
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    return &config, nil
}

func (c *CCSwitchIntegration) GetActiveProfile() (*Profile, error) {
    config, err := c.ReadConfig()
    if err != nil {
        return nil, err
    }

    for _, profile := range config.Profiles {
        if profile.Name == config.ActiveProfile {
            return &profile, nil
        }
    }

    return nil, ErrProfileNotFound
}

type CCSwitchConfig struct {
    ActiveProfile string    `json:"active_profile"`
    Profiles      []Profile `json:"profiles"`
}

type Profile struct {
    Name     string `json:"name"`
    Provider string `json:"provider"`
    Model    string `json:"model"`
    APIKey   string `json:"api_key,omitempty"`
    BaseURL  string `json:"base_url,omitempty"`
}
```

### CLI Download Manager

```go
// CLIDownloadManager manages Claude Code CLI downloads
type CLIDownloadManager struct {
    BaseURL     string
    CacheDir    string
    OnProgress  func(downloaded, total int64, speed float64)
}

func (m *CLIDownloadManager) GetDownloadInfo() (*DownloadInfo, error) {
    // Fetch latest version info
    resp, err := http.Get(m.BaseURL + "/latest.json")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var info DownloadInfo
    json.NewDecoder(resp.Body).Decode(&info)
    return &info, nil
}

func (m *CLIDownloadManager) Download(ctx context.Context) error {
    info, err := m.GetDownloadInfo()
    if err != nil {
        return err
    }

    // Create download request
    req, _ := http.NewRequestWithContext(ctx, "GET", info.DownloadURL, nil)

    // Support resume
    partialPath := filepath.Join(m.CacheDir, "claude-code.partial")
    if stat, err := os.Stat(partialPath); err == nil {
        req.Header.Set("Range", fmt.Sprintf("bytes=%d-", stat.Size()))
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Track progress
    reader := &ProgressReader{
        Reader:     resp.Body,
        Total:      info.Size,
        OnProgress: m.OnProgress,
    }

    // Write to file
    file, _ := os.OpenFile(partialPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
    defer file.Close()

    _, err = io.Copy(file, reader)
    if err != nil {
        return err
    }

    // Verify checksum
    if err := m.verifyChecksum(partialPath, info.Checksum); err != nil {
        return err
    }

    // Move to final location
    finalPath := filepath.Join(m.CacheDir, "claude-code")
    return os.Rename(partialPath, finalPath)
}

type DownloadInfo struct {
    Version     string `json:"version"`
    Size        int64  `json:"size"`
    SizeHuman   string `json:"size_human"` // e.g., "120 MB"
    DownloadURL string `json:"download_url"`
    Checksum    string `json:"checksum"`
    Platform    string `json:"platform"`
}
```

### Limited Mode Feature Matrix

When Claude Code CLI is disabled or not installed, the following features are affected:

| Feature | With CLI Enabled | Without CLI | Notes |
|---------|------------------|-------------|-------|
| **Chat with Claude** | ✅ Full | ✅ Full | Direct API, no CLI needed |
| **Chat with Ollama** | ✅ Full | ✅ Full | Direct API, no CLI needed |
| **Chat with OpenAI** | ✅ Full | ✅ Full | Direct API, no CLI needed |
| **Skills** | ✅ Full | ❌ None | **Requires CLI** |
| **Tool Calling (Native)** | ✅ Full | ⚠️ Limited | Only providers with native support |
| **Tool Calling (Adapter)** | ✅ Full | ❌ None | CLIProxy adapter requires CLI |
| **File Operations** | ✅ Full | ❌ None | **Requires CLI** |
| **Terminal Commands** | ✅ Full | ❌ None | **Requires CLI** |
| **MCP Tools** | ✅ Full | ❌ None | **Requires CLI** |
| **Code Execution** | ✅ Full | ❌ None | **Requires CLI** |
| **Agent Mode** | ✅ Full | ❌ None | **Requires CLI** |
| **Project Context** | ✅ Full | ❌ None | **Requires CLI** |
| **Usage Statistics** | ✅ Full | ✅ Basic | Basic stats still available |
| **Provider Switching** | ✅ Full | ✅ Full | No CLI needed |

### CLI Enable/Disable Toggle

Users can enable or disable Claude Code CLI integration via a toggle in settings:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Claude Code CLI Integration                                                 │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │  [✓] Enable Claude Code CLI (Strongly Recommended)                     │ │
│  │                                                                         │ │
│  │  Claude Code CLI provides advanced features:                           │ │
│  │  • Skills - Pre-built automation workflows                             │ │
│  │  • Tool Calling - Execute tools and functions                          │ │
│  │  • File Operations - Read, write, and edit files                       │ │
│  │  • Terminal Commands - Execute shell commands                          │ │
│  │  • MCP Integration - Model Context Protocol support                    │ │
│  │  • Agent Mode - Autonomous task execution                              │ │
│  │                                                                         │ │
│  │  ⚠️ Disabling CLI will remove these capabilities.                      │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  Status: Installed (v1.0.0)                    [Update] [Uninstall]         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Warning Banner (When CLI Disabled)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  ⚠️ Claude Code CLI is Disabled                                              │
│                                                                              │
│  The following features are unavailable:                                     │
│  • Skills  • Tool Calling  • File Operations  • Terminal  • MCP  • Agent    │
│                                                                              │
│  [Enable CLI (Recommended)]                                    [Dismiss]    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Statistics Collection

```go
// StatisticsCollector collects usage statistics
type StatisticsCollector struct {
    Enabled     bool
    StoragePath string
}

type UsageStats struct {
    // API Calls
    TotalCalls      int64            `json:"total_calls"`
    CallsByProvider map[string]int64 `json:"calls_by_provider"`
    CallsByModel    map[string]int64 `json:"calls_by_model"`

    // Token Usage
    InputTokens     int64            `json:"input_tokens"`
    OutputTokens    int64            `json:"output_tokens"`
    CacheHitTokens  int64            `json:"cache_hit_tokens"`

    // Performance
    AvgLatencyMs    float64          `json:"avg_latency_ms"`
    P95LatencyMs    float64          `json:"p95_latency_ms"`

    // Errors
    ErrorCount      int64            `json:"error_count"`
    ErrorsByType    map[string]int64 `json:"errors_by_type"`

    // Cost Estimation
    EstimatedCostUSD float64         `json:"estimated_cost_usd"`

    // Time Range
    PeriodStart     time.Time        `json:"period_start"`
    PeriodEnd       time.Time        `json:"period_end"`
}

func (c *StatisticsCollector) Record(event *APICallEvent) error {
    if !c.Enabled {
        return nil
    }

    // Record to local storage
    return c.appendEvent(event)
}

func (c *StatisticsCollector) GetStats(period string) (*UsageStats, error) {
    // Aggregate statistics for the given period
    // period: "day", "week", "month", "all"
    return c.aggregate(period)
}
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/setup/status` | Get setup wizard status |
| POST | `/api/v1/setup/complete` | Mark setup as complete |
| GET | `/api/v1/providers/detect` | Detect available providers |
| GET | `/api/v1/providers/ollama` | Get Ollama status |
| GET | `/api/v1/cli/status` | Get CLI installation status |
| POST | `/api/v1/cli/download` | Start CLI download |
| GET | `/api/v1/cli/download/progress` | Get download progress |
| DELETE | `/api/v1/cli/download` | Cancel download |
| GET | `/api/v1/env/config` | Get environment configuration |
| GET | `/api/v1/cc-switch/profiles` | Get cc-switch profiles |
| POST | `/api/v1/cc-switch/activate` | Activate a profile |
| GET | `/api/v1/stats` | Get usage statistics |
| GET | `/api/v1/stats/export` | Export statistics |

### UI Components

#### Settings Page Layout

The settings page separates Claude Code CLI configuration from LLM provider configuration.
**Claude Code CLI appears BELOW LLM Providers** (it's an enhancement tool, not a primary provider):

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Settings                                                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  LLM Providers                                                       │    │
│  │  ─────────────────────────────────────────────────────────────────  │    │
│  │                                                                      │    │
│  │  Default Provider: [Anthropic ▼]                                    │    │
│  │                                                                      │    │
│  │  ┌───────────────────────────────────────────────────────────────┐  │    │
│  │  │  Anthropic                                      [Toggle: ON]  │  │    │
│  │  │  Model: claude-3-5-sonnet-20241022                            │  │    │
│  │  │  API Key: ****...****  [Edit]                                 │  │    │
│  │  └───────────────────────────────────────────────────────────────┘  │    │
│  │                                                                      │    │
│  │  ┌───────────────────────────────────────────────────────────────┐  │    │
│  │  │  OpenAI                                         [Toggle: ON]  │  │    │
│  │  │  Model: gpt-4o                                                │  │    │
│  │  │  API Key: ****...****  [Edit]                                 │  │    │
│  │  └───────────────────────────────────────────────────────────────┘  │    │
│  │                                                                      │    │
│  │  ┌───────────────────────────────────────────────────────────────┐  │    │
│  │  │  Ollama                                         [Toggle: ON]  │  │    │
│  │  │  Endpoint: http://localhost:11434  ✓ Connected                │  │    │
│  │  │  Models: llama3.2, codellama, mistral                         │  │    │
│  │  └───────────────────────────────────────────────────────────────┘  │    │
│  │                                                                      │    │
│  │  Fallback Chain: Anthropic → OpenAI → Ollama  [Edit]                │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Claude Code CLI                                    [Toggle: ON]    │    │
│  │  ─────────────────────────────────────────────────────────────────  │    │
│  │                                                                      │    │
│  │  ⚡ Strongly Recommended                                             │    │
│  │                                                                      │    │
│  │  Status: Installed (v1.0.0)                                         │    │
│  │                                                                      │    │
│  │  Enables advanced features:                                          │    │
│  │  • Skills        • Tool Calling    • File Operations                │    │
│  │  • Terminal      • MCP Integration • Agent Mode                     │    │
│  │                                                                      │    │
│  │  [Update]  [Uninstall]  [Advanced Settings ▼]                       │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Statistics & Privacy                                                │    │
│  │  ─────────────────────────────────────────────────────────────────  │    │
│  │                                                                      │    │
│  │  [✓] Collect usage statistics (anonymized)                          │    │
│  │                                                                      │    │
│  │  [View Statistics Dashboard]  [Export Data]                         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Design Principles:**

1. **LLM Providers come first** - These are the primary chat providers users interact with
2. **Claude Code CLI is below LLM Providers** - It's an enhancement tool, not a provider itself
3. **Claude Code CLI is NOT an LLM Provider** - It enables advanced features (Skills, Tool Calling, etc.)
4. **Clear visual separation** - CLI settings and LLM settings are in distinct sections
5. **CLI toggle shows "Strongly Recommended"** - Encourages users to keep it enabled

#### First-Run Wizard

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                         Welcome to ZimaOS Echo                               │
│                                                                              │
│                              [Echo Logo]                                     │
│                                                                              │
│         Your AI-powered assistant for development and automation            │
│                                                                              │
│  ─────────────────────────────────────────────────────────────────────────  │
│                                                                              │
│  Detected Providers:                                                         │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ✓ Ollama                                    localhost:11434        │    │
│  │    Models: llama3.2, codellama, mistral                             │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ○ Claude Code CLI                           Not installed          │    │
│  │    Download size: ~120 MB                                           │    │
│  │    Enables: Full Claude integration, MCP tools, file operations     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ✓ Environment Variables                                            │    │
│  │    ANTHROPIC_API_KEY: ****...****                                   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│                                                                              │
│         [Download Claude Code CLI]        [Skip for now]                    │
│                                                                              │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Download Progress

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                      Downloading Claude Code CLI                             │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                                                                      │    │
│  │  ████████████████████████████░░░░░░░░░░░░░░░░░░░░  62%              │    │
│  │                                                                      │    │
│  │  74.4 MB / 120 MB                              2.3 MB/s             │    │
│  │                                                                      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│                              [Cancel]                                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Limited Mode Banner

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  ⚠ Running in Limited Mode                                                  │
│                                                                              │
│  Some features are unavailable without Claude Code CLI:                     │
│  • Claude model access  • File operations  • Terminal commands  • MCP tools │
│                                                                              │
│  [Download CLI (~120 MB)]                                    [Dismiss]      │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Configuration

Claude Code CLI configuration is **separated from LLM provider settings**. This allows:
- Independent management of CLI features
- Clear distinction between chat capabilities and advanced features
- Easier troubleshooting and configuration

```yaml
# Claude Code CLI Configuration (Separate from LLM)
claude_code_cli:
  # Master switch - strongly recommended to keep enabled
  enabled: true  # Default: true (strongly recommended)

  # Installation settings
  install:
    path: ""  # Empty = auto-detect or ~/.local/share/zimaos-echo/claude-code
    auto_update: false
    verify_checksum: true

  # Download settings
  download:
    base_url: "https://storage.googleapis.com/anthropic-public/claude-code"
    cache_dir: ""  # Empty = ~/.local/share/zimaos-echo/claude-code

  # Feature toggles (only effective when enabled=true)
  features:
    skills: true
    tool_calling: true
    file_operations: true
    terminal_commands: true
    mcp_integration: true
    agent_mode: true
    project_context: true

# LLM Provider Configuration (Separate from CLI)
llm:
  providers:
    anthropic:
      enabled: true
      api_key: ""  # Or from ANTHROPIC_API_KEY env
      model: "claude-3-5-sonnet-20241022"

    openai:
      enabled: true
      api_key: ""  # Or from OPENAI_API_KEY env
      base_url: ""  # Optional custom endpoint
      model: "gpt-4o"

    ollama:
      enabled: true
      endpoints:
        - "http://localhost:11434"
        - "http://127.0.0.1:11434"
      auto_detect: true

    # Other providers...

  # Default provider selection
  default_provider: "anthropic"

  # Fallback chain
  fallback:
    enabled: true
    chain:
      - "anthropic"
      - "openai"
      - "ollama"

# First-run wizard
first_run:
  enabled: true
  show_provider_detection: true
  show_cli_download: true
  allow_skip: true
  recommend_cli: true  # Show "strongly recommended" message

# cc-switch integration
cc_switch:
  enabled: true
  config_path: ""  # Empty = ~/.claude-code-switch/config.json
  sync_profiles: true

# Statistics
statistics:
  enabled: true
  opt_in_required: true
  storage_path: ""  # Empty = ~/.local/share/zimaos-echo/stats
  retention_days: 90

# Tool calling adapters
tool_calling:
  auto_detect: true
  detection_timeout: 5s

  adapters:
    cli_proxy:
      enabled: true  # Only works when claude_code_cli.enabled=true
      prompt_template: "default"

    cc_nexus:
      enabled: true
      schema_mapping: "auto"

  provider_overrides:
    ollama:
      tool_calling: "adapter"
    custom:
      tool_calling: "auto"
```

### Configuration Migration

When upgrading from previous versions, the configuration will be automatically migrated:

1. **LLM settings** remain in `llm` section
2. **CLI-specific settings** are moved to `claude_code_cli` section
3. **Default behavior** keeps CLI enabled for backward compatibility

## Implementation Checklist

### Phase 1: First-Run Wizard (Week 1)

- [ ] Create welcome screen component
- [ ] Implement provider detection service
- [ ] Add Ollama auto-detection
- [ ] Add environment variable reading
- [ ] Create setup completion flow

### Phase 2: CLI Download (Week 2)

- [ ] Implement download manager with progress
- [ ] Add download resume support
- [ ] Add checksum verification
- [ ] Create download UI components
- [ ] Handle download errors gracefully

### Phase 3: Limited Mode (Week 3)

- [ ] Define feature availability matrix
- [ ] Implement feature gating
- [ ] Create limited mode banner
- [ ] Add "download later" reminder
- [ ] Test all features in limited mode

### Phase 4: cc-switch Integration (Week 4)

- [ ] Implement config file reading
- [ ] Add profile switching support
- [ ] Sync with cc-switch changes
- [ ] Create profile management UI

### Phase 5: Statistics (Week 5)

- [ ] Implement statistics collector
- [ ] Create opt-in consent flow
- [ ] Build statistics dashboard
- [ ] Add export functionality

### Phase 6: Tool Calling Detection & API Adapter (Week 6)

- [ ] Implement tool calling capability detection
- [ ] Create API compatibility checker
- [ ] Implement CLIProxy adapter for incompatible APIs
- [ ] Implement ccNexus-style conversion interface
- [ ] Add automatic fallback to adapter when needed

## Tool Calling Detection

### Overview

Not all LLM APIs support tool calling (function calling). This feature automatically detects whether a provider's API supports tool calling and provides adapter interfaces for incompatible APIs.

### Detection Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Tool Calling Detection Flow                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐                                                           │
│  │   Provider   │                                                           │
│  │   Selected   │                                                           │
│  └──────┬───────┘                                                           │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────┐     ┌──────────────┐                                      │
│  │   Check      │────▶│   Native     │                                      │
│  │   Capability │     │   Support?   │                                      │
│  └──────────────┘     └──────┬───────┘                                      │
│                              │                                               │
│         ┌────────────────────┼────────────────────┐                         │
│         ▼                    ▼                    ▼                         │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                 │
│  │   Native     │     │   Partial    │     │   No         │                 │
│  │   Tool Call  │     │   Support    │     │   Support    │                 │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                 │
│         │                    │                    │                         │
│         ▼                    ▼                    ▼                         │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                 │
│  │   Direct     │     │   Use        │     │   Use        │                 │
│  │   API Call   │     │   Adapter    │     │   CLIProxy   │                 │
│  └──────────────┘     └──────────────┘     └──────────────┘                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Provider Capability Matrix

| Provider | Tool Calling | Streaming | Vision | Adapter Required |
|----------|--------------|-----------|--------|------------------|
| Claude (Anthropic) | ✅ Native | ✅ | ✅ | No |
| OpenAI | ✅ Native | ✅ | ✅ | No |
| Ollama | ⚠️ Partial | ✅ | ⚠️ | Optional |
| DeepSeek | ✅ Native | ✅ | ❌ | No |
| Gemini | ✅ Native | ✅ | ✅ | No |
| Local LLMs | ❌ | ✅ | ❌ | Yes |
| Custom OpenAI-Compatible | ⚠️ Varies | ✅ | ⚠️ | Auto-detect |

### API Adapter Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        API Adapter Architecture                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Unified LLM Interface                            │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                 │    │
│  │  │    Chat()    │ │  ToolCall()  │ │   Stream()   │                 │    │
│  │  └──────────────┘ └──────────────┘ └──────────────┘                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Capability Router                                │    │
│  │  - Detect provider capabilities                                      │    │
│  │  - Route to appropriate handler                                      │    │
│  │  - Manage fallback strategies                                        │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│         ┌──────────────────────────┼──────────────────────────┐             │
│         ▼                          ▼                          ▼             │
│  ┌──────────────┐          ┌──────────────┐          ┌──────────────┐       │
│  │   Native     │          │   CLIProxy   │          │   ccNexus    │       │
│  │   Handler    │          │   Adapter    │          │   Adapter    │       │
│  │              │          │              │          │              │       │
│  │ Direct API   │          │ Prompt-based │          │ Tool schema  │       │
│  │ tool calling │          │ tool calling │          │ conversion   │       │
│  └──────────────┘          └──────────────┘          └──────────────┘       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### CLIProxy Adapter

For APIs that don't support native tool calling, CLIProxy converts tool calls to prompt-based interactions:

```go
// CLIProxyAdapter converts tool calls to prompt-based format
type CLIProxyAdapter struct {
    BaseProvider llm.Provider
    ToolRegistry *ToolRegistry
}

// ToolCall converts a tool call request to a prompt
func (a *CLIProxyAdapter) ToolCall(ctx context.Context, req *ToolCallRequest) (*ToolCallResponse, error) {
    // Convert tool definition to prompt format
    prompt := a.buildToolPrompt(req.Tool, req.Input)

    // Call the base provider with the prompt
    resp, err := a.BaseProvider.Chat(ctx, &ChatRequest{
        Messages: []Message{{Role: "user", Content: prompt}},
    })
    if err != nil {
        return nil, err
    }

    // Parse the response to extract tool output
    return a.parseToolResponse(resp.Content)
}
```

### ccNexus Adapter

For providers with partial tool calling support, ccNexus provides schema conversion:

```go
// ccNexusAdapter converts between different tool calling schemas
type ccNexusAdapter struct {
    BaseProvider llm.Provider
    SchemaMapper *SchemaMapper
}

// ConvertToolSchema converts tool schema to provider-specific format
func (a *ccNexusAdapter) ConvertToolSchema(tool *Tool) interface{} {
    switch a.BaseProvider.Name() {
    case "ollama":
        return a.SchemaMapper.ToOllamaFormat(tool)
    case "gemini":
        return a.SchemaMapper.ToGeminiFormat(tool)
    default:
        return a.SchemaMapper.ToOpenAIFormat(tool)
    }
}
```

### Configuration

```yaml
tool_calling:
  # Auto-detection settings
  auto_detect: true
  detection_timeout: 5s

  # Adapter settings
  adapters:
    cli_proxy:
      enabled: true  # Only works when claude_code_cli.enabled=true
      prompt_template: "default"  # or custom template path

    cc_nexus:
      enabled: true
      schema_mapping: "auto"  # auto, strict, loose

  # Provider overrides (skip auto-detection)
  provider_overrides:
    ollama:
      tool_calling: "adapter"  # native, adapter, disabled
    custom:
      tool_calling: "auto"

# Note: When claude_code_cli.enabled=false, cli_proxy adapter is unavailable
# and tool calling falls back to native support only
```

## Security Considerations

1. **API Key Handling** - Never log or expose API keys
2. **Download Verification** - Always verify checksums
3. **Statistics Privacy** - Anonymize all collected data
4. **Environment Variables** - Mask sensitive values in UI
5. **Tool Execution** - Validate tool inputs before execution
6. **Adapter Security** - Sanitize prompts in CLIProxy adapter

## Success Metrics

| Metric | Target |
|--------|--------|
| First-run completion rate | > 80% |
| CLI download success rate | > 95% |
| Provider detection accuracy | > 99% |
| Limited mode user retention | > 60% |

## References

- [v0.10.0 Claude Code CLI Bundling](v0.10.0-claude-code-bundling.md)
- [v0.10.1 Metrics Monitoring](v0.10.1-metrics-monitoring.md)
- [cc-switch Documentation](https://github.com/anthropics/claude-code-switch)
- [Ollama API Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
