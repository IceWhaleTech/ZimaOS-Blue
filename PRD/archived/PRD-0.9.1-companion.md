# PRD: Echo Companion - Real-time Agent Monitoring

**Status:** Draft
**Author:** ZimaOS Team
**Last Updated:** 2026-01-28
**Target Version:** v0.9.1

---

## Overview

Echo Companion is a real-time monitoring tool that provides developers and administrators with a visual interface to observe AI Agent operations across multiple platforms. Inspired by [Crabwalk](https://github.com/luccast/crabwalk)'s design philosophy, combined with ZimaOS Blue's deep security insights, it delivers a secure, observable, and auditable Agent monitoring solution.

### Core Values

1. **Real-time Observability**: Track Agent sessions, tool calls, and decision chains in real-time
2. **Security Auditing**: Integrate with Echo's security layer for threat detection and anomaly alerts
3. **Cross-platform Monitoring**: Unified monitoring for WhatsApp, Telegram, Discord, Slack, Matrix, Feishu, and more
4. **Debug-friendly**: Help developers quickly identify issues and optimize Agent behavior

## Goals

- [x] Provide real-time Agent activity visualization interface
- [x] Integrate with Echo's existing security layer (Prompt Guard, Audit Log, Sandbox)
- [x] Support multi-platform Agent session monitoring
- [x] Provide threat detection and anomaly behavior alerts
- [x] Support session replay and debug analysis

## User Stories

### Story 1: Real-time Activity Monitoring

**As a** system administrator
**I want to** view all Agent activity status in real-time
**So that** I can detect anomalies promptly and take action

**Acceptance Criteria:**
- [ ] Visualize currently active Agent sessions
- [ ] Real-time updates of session status and tool calls
- [ ] Support filtering by platform, user, and time
- [ ] Display session security scores and threat levels

### Story 2: Security Threat Alerts

**As a** security engineer
**I want to** receive alerts when Prompt Injection or anomalous behavior is detected
**So that** I can respond quickly to security incidents

**Acceptance Criteria:**
- [ ] Integrate Prompt Guard threat detection results
- [ ] Display threat levels in real-time (None/Low/Medium/High/Critical)
- [ ] Support configurable alert thresholds and notification methods
- [ ] Provide threat details and recommended remediation actions

### Story 3: Session Debugging and Replay

**As a** developer
**I want to** replay and analyze historical sessions
**So that** I can debug and optimize Agent behavior

**Acceptance Criteria:**
- [ ] Support session timeline replay
- [ ] Display complete tool call chains and parameters
- [ ] Show LLM input/output and token consumption
- [ ] Support exporting session data for analysis

### Story 4: Operation Chain Visualization

**As a** developer
**I want to** view Agent operation chains graphically
**So that** I can understand the Agent's decision-making process

**Acceptance Criteria:**
- [ ] Use flowcharts to display operation chains
- [ ] Support expanding nodes to view detailed parameters
- [ ] Display execution time and status for each operation
- [ ] Support zoom and drag interactions

## Requirements

### Functional Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| FR-001 | WebSocket real-time data stream | P0 | Establish persistent connection with Echo Gateway |
| FR-002 | Session list and filtering | P0 | Filter by platform, user, time, status |
| FR-003 | Operation chain flowchart visualization | P0 | Use ReactFlow or similar library |
| FR-004 | Threat detection integration | P0 | Integrate Prompt Guard results |
| FR-005 | Audit log integration | P0 | Integrate Audit Log data |
| FR-006 | Real-time alert notifications | P1 | Support WebSocket/Webhook notifications |
| FR-007 | Session replay functionality | P1 | Timeline replay of historical sessions |
| FR-008 | Tool call details display | P1 | Show parameters, return values, duration |
| FR-009 | Sandbox execution monitoring | P1 | Monitor sandbox execution status and resource usage |
| FR-010 | Multi-tenant support | P2 | Support tenant isolation and permission control |
| FR-011 | Data export | P2 | Export in JSON/CSV format |
| FR-012 | Custom dashboards | P2 | Support custom monitoring panels |

### Non-Functional Requirements

| ID | Requirement | Target | Notes |
|----|-------------|--------|-------|
| NFR-001 | WebSocket latency | < 100ms | Real-time requirement |
| NFR-002 | Page load time | < 2s | First contentful paint |
| NFR-003 | Concurrent session support | 1000+ | Number of sessions monitored simultaneously |
| NFR-004 | Data retention period | 7 days | Default retention, configurable |
| NFR-005 | Browser compatibility | Chrome/Firefox/Safari | Latest two versions |
| NFR-006 | Data readability | Plain text files preferred, user-readable | Avoid database storage |

## Technical Design

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Echo Companion Architecture                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        Frontend (Web UI)                             │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌────────────┐  │    │
│  │  │   Dashboard  │ │  Session     │ │   Flow       │ │   Alert    │  │    │
│  │  │   Overview   │ │  Explorer    │ │   Viewer     │ │   Center   │  │    │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └────────────┘  │    │
│  │                                                                      │    │
│  │  Tech Stack: Vue 3 + TypeScript + TailwindCSS + ReactFlow           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    │ WebSocket / REST API                    │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Echo Backend (Go)                                │    │
│  │  ┌──────────────────────────────────────────────────────────────┐   │    │
│  │  │                  Companion Service                            │   │    │
│  │  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐  │   │    │
│  │  │  │  Session   │ │   Event    │ │   Alert    │ │  Replay   │  │   │    │
│  │  │  │  Manager   │ │  Streamer  │ │  Engine    │ │  Service  │  │   │    │
│  │  │  └────────────┘ └────────────┘ └────────────┘ └───────────┘  │   │    │
│  │  └──────────────────────────────────────────────────────────────┘   │    │
│  │                                │                                     │    │
│  │  ┌─────────────────────────────┴────────────────────────────────┐   │    │
│  │  │                  Security Integration                         │   │    │
│  │  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐  │   │    │
│  │  │  │  Prompt    │ │   Audit    │ │  Sandbox   │ │   Auth    │  │   │    │
│  │  │  │  Guard     │ │   Logger   │ │  Monitor   │ │  (OIDC)   │  │   │    │
│  │  │  └────────────┘ └────────────┘ └────────────┘ └───────────┘  │   │    │
│  │  └──────────────────────────────────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     Data Sources                                     │    │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────────────┐  │    │
│  │  │  Channel   │ │   LLM      │ │   Tool     │ │     Audit        │  │    │
│  │  │  Events    │ │  Requests  │ │   Calls    │ │     Logs         │  │    │
│  │  │ (WA/TG/...)│ │            │ │            │ │                  │  │    │
│  │  └────────────┘ └────────────┘ └────────────┘ └──────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Data Storage

Following NFR-006, all data is stored in human-readable plain text files:

```
data/companion/
├── sessions/
│   ├── 2026-01-28/
│   │   ├── session-abc123.jsonl      # One JSON object per line
│   │   └── session-def456.jsonl
│   └── 2026-01-27/
│       └── ...
├── alerts/
│   ├── 2026-01-28.jsonl              # Daily alert log
│   └── 2026-01-27.jsonl
└── stats/
    └── daily-stats.json              # Aggregated statistics
```

**File Format (JSONL - JSON Lines):**
- One JSON object per line for easy parsing and streaming
- Human-readable with standard JSON formatting
- Easy to grep, tail, and process with standard Unix tools
- No database required

### Data Model

#### Session Event

```go
type SessionEvent struct {
    ID          string                 `json:"id"`
    SessionID   string                 `json:"session_id"`
    Timestamp   time.Time              `json:"timestamp"`
    EventType   SessionEventType       `json:"event_type"`
    Platform    string                 `json:"platform"`    // whatsapp, telegram, discord, etc.
    UserID      string                 `json:"user_id"`
    TenantID    string                 `json:"tenant_id,omitempty"`

    // Event-specific data
    Message     *MessageEvent          `json:"message,omitempty"`
    ToolCall    *ToolCallEvent         `json:"tool_call,omitempty"`
    LLMRequest  *LLMRequestEvent       `json:"llm_request,omitempty"`
    Security    *SecurityEvent         `json:"security,omitempty"`

    // Metadata
    Duration    time.Duration          `json:"duration,omitempty"`
    Status      string                 `json:"status"`
    Error       string                 `json:"error,omitempty"`
}

type SessionEventType string

const (
    EventSessionStart    SessionEventType = "session_start"
    EventSessionEnd      SessionEventType = "session_end"
    EventMessageReceived SessionEventType = "message_received"
    EventMessageSent     SessionEventType = "message_sent"
    EventToolCall        SessionEventType = "tool_call"
    EventLLMRequest      SessionEventType = "llm_request"
    EventSecurityThreat  SessionEventType = "security_threat"
    EventSandboxExec     SessionEventType = "sandbox_exec"
)
```

#### Security Event

```go
type SecurityEvent struct {
    ThreatLevel     string   `json:"threat_level"`     // none, low, medium, high, critical
    ThreatScore     int      `json:"threat_score"`
    ThreatTypes     []string `json:"threat_types"`
    DetectedPatterns []string `json:"detected_patterns"`
    Action          string   `json:"action"`           // allowed, blocked, filtered
    FilteredContent string   `json:"filtered_content,omitempty"`
}
```

#### Tool Call Event

```go
type ToolCallEvent struct {
    ToolName    string                 `json:"tool_name"`
    ToolID      string                 `json:"tool_id"`
    Input       map[string]interface{} `json:"input"`
    Output      interface{}            `json:"output,omitempty"`
    Duration    time.Duration          `json:"duration"`
    Status      string                 `json:"status"`
    SandboxUsed bool                   `json:"sandbox_used"`
    Resources   *ResourceUsage         `json:"resources,omitempty"`
}

type ResourceUsage struct {
    CPUTime     time.Duration `json:"cpu_time"`
    MemoryPeak  int64         `json:"memory_peak"`
    IORead      int64         `json:"io_read"`
    IOWrite     int64         `json:"io_write"`
}
```

### API Endpoints

#### WebSocket API

| Endpoint | Description |
|----------|-------------|
| `ws://host/api/v1/companion/stream` | Real-time event stream |
| `ws://host/api/v1/companion/session/:id` | Single session event stream |

#### REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/companion/sessions` | Get session list |
| GET | `/api/v1/companion/sessions/:id` | Get session details |
| GET | `/api/v1/companion/sessions/:id/events` | Get session events |
| GET | `/api/v1/companion/sessions/:id/flow` | Get operation chain data |
| GET | `/api/v1/companion/alerts` | Get alert list |
| PUT | `/api/v1/companion/alerts/:id/ack` | Acknowledge alert |
| GET | `/api/v1/companion/stats` | Get statistics |
| GET | `/api/v1/companion/export` | Export data |

### Security Integration

#### Prompt Guard Integration

```go
// Integrate Companion events in message processing flow
func (s *CompanionService) OnPromptGuardResult(
    sessionID string,
    input string,
    result *promptguard.CheckResult,
) {
    if result.IsThreat {
        s.EmitEvent(SessionEvent{
            SessionID: sessionID,
            EventType: EventSecurityThreat,
            Security: &SecurityEvent{
                ThreatLevel:      result.ThreatLevel.String(),
                ThreatScore:      result.Score,
                ThreatTypes:      result.ThreatTypes,
                DetectedPatterns: result.Patterns,
                Action:           "blocked",
            },
        })

        // Trigger alert
        if result.ThreatLevel >= promptguard.ThreatHigh {
            s.alertEngine.Trigger(Alert{
                Severity: "high",
                Title:    "High-severity prompt injection detected",
                SessionID: sessionID,
                Details:  result,
            })
        }
    }
}
```

#### Audit Log Integration

```go
// Subscribe to audit log events
func (s *CompanionService) SubscribeAuditEvents() {
    s.auditSubscriber.Subscribe(func(entry *audit.Entry) {
        // Convert to Companion event
        event := s.convertAuditEntry(entry)
        s.EmitEvent(event)
    })
}
```

#### Sandbox Monitor Integration

```go
// Monitor sandbox execution
func (s *CompanionService) OnSandboxExecution(
    sessionID string,
    exec *sandbox.Execution,
) {
    s.EmitEvent(SessionEvent{
        SessionID: sessionID,
        EventType: EventSandboxExec,
        ToolCall: &ToolCallEvent{
            ToolName:    exec.Command,
            SandboxUsed: true,
            Duration:    exec.Duration,
            Status:      exec.Status,
            Resources: &ResourceUsage{
                CPUTime:    exec.CPUTime,
                MemoryPeak: exec.MemoryPeak,
            },
        },
    })
}
```

### Frontend Components

#### Dashboard Overview

- Active session count and trend charts
- Platform distribution pie chart
- Threat level distribution
- Recent alerts list
- System health status

#### Session Explorer

- Session list (with filtering and search)
- Session details panel
- Message timeline
- Tool call list

#### Flow Viewer

- ReactFlow operation chain visualization
- Node types: message, tool call, LLM request, security check
- Support expand/collapse node details
- Support zoom and drag

#### Alert Center

- Alert list (sorted by severity)
- Alert details and remediation recommendations
- Alert acknowledgment and mute functionality
- Alert rule configuration

### Configuration

```yaml
companion:
  enabled: true

  # Data storage
  storage:
    base_path: "./data/companion"
    format: "jsonl"  # JSON Lines format

  # WebSocket configuration
  websocket:
    ping_interval: 30s
    write_timeout: 10s
    read_buffer_size: 1024
    write_buffer_size: 1024

  # Data retention
  retention:
    events_days: 7
    sessions_days: 30
    alerts_days: 90

  # Alert configuration
  alerts:
    enabled: true
    threat_threshold: medium  # Minimum threat level to trigger alerts
    channels:
      - type: webhook
        url: "https://your-webhook.com/alerts"
      - type: email
        recipients:
          - "admin@example.com"

  # Security integration
  security:
    prompt_guard_integration: true
    audit_log_integration: true
    sandbox_monitor: true

  # Performance configuration
  performance:
    max_concurrent_sessions: 1000
    event_buffer_size: 10000
    batch_write_interval: 1s
```

## Security Considerations

### Authentication & Authorization

- Use Echo's existing OIDC authentication
- Role-based access control (RBAC)
  - `companion:read` - View sessions and events
  - `companion:admin` - Manage alerts and configuration
- Multi-tenant isolation

### Data Privacy

- Sensitive data masking (API keys, passwords, etc.)
- Configurable data retention policies
- Support data export and deletion (GDPR compliance)

### Threat Detection Enhancement

Companion can provide additional security insights:

1. **Anomaly Behavior Detection**
   - Abnormally high-frequency tool calls
   - Abnormal session duration
   - Abnormal resource usage

2. **Cross-session Correlation Analysis**
   - Identify attack attempts from the same source
   - Detect distributed attack patterns

3. **Real-time Threat Intelligence**
   - Aggregate threat detection results
   - Generate threat trend reports

## Milestones

| Phase | Scope | Target Date |
|-------|-------|-------------|
| Phase 1 | Core architecture and real-time event stream | Week 1-2 |
| Phase 2 | Frontend Dashboard and Session Explorer | Week 3-4 |
| Phase 3 | Flow Viewer and security integration | Week 5-6 |
| Phase 4 | Alert Center and advanced features | Week 7-8 |

## Success Metrics

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Event latency | < 100ms | P99 latency monitoring |
| Threat detection accuracy | > 95% | Manual review sampling |
| User satisfaction | > 4.0/5.0 | User feedback survey |
| System availability | > 99.9% | Monitoring alerts |

## Open Questions

- [ ] Should we support custom alert rules (DSL)?
- [ ] Should we integrate with external SIEM systems?
- [ ] File rotation strategy for large deployments?
- [ ] Should we support mobile access?

## References

- [Crabwalk](https://github.com/luccast/crabwalk) - Design reference
- [ReactFlow](https://reactflow.dev/) - Flowchart visualization library
- [Echo Security Documentation](../docs/guide/security.md) - Security architecture
- [OWASP LLM Top 10](https://owasp.org/www-project-top-10-for-large-language-model-applications/) - LLM security guide
