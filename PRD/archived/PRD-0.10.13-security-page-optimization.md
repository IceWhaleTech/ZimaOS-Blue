# PRD-0.10.13: Security Page Optimization

## Overview

This PRD addresses usability and clarity issues in the Security page, focusing on consolidating monitoring features, fixing real-time event tracking, and improving HTTP connection metrics display.

## Goals

1. Consolidate monitoring into the Security page for a unified security dashboard
2. Fix conversation event flow visualization and real-time updates
3. Clarify HTTP connection metrics to reduce user confusion
4. Fix stats display showing no numbers
5. Enhance security scan results with detailed reports and one-click fix capabilities

---

## 1. Monitoring Page Integration

### Current State
- Monitoring exists as a separate page/section
- Security metrics are scattered across different views
- Users need to navigate between pages to get a complete security picture
- **Stats cards show labels but no numbers** (Active Sessions, Total Sessions, Total Events, Total Alerts, Unacknowledged Alerts)

### Proposed Changes

#### 1.1 Move Monitoring into Security Page
- Add a "Monitoring" tab or collapsible section within the Security page
- Consolidate all security-related metrics in one place

#### 1.2 Fix Stats Display
Stats cards currently show:
- Active Sessions (no number)
- Total Sessions (no number)
- Total Events (no number)
- Total Alerts (no number)
- Unacknowledged Alerts (no number)

Root cause: API response field names may not match frontend expectations (snake_case vs camelCase mismatch)

#### 1.3 Clarify Metric Definitions

Current metrics lack clear definitions. Add tooltips/descriptions:

| Metric | Current Issue | Proposed Definition |
|--------|---------------|---------------------|
| Total Requests | Unclear scope | Total API requests processed since server start |
| Blocked Requests | What counts as blocked? | Requests rejected by security rules (rate limit, threat detection, etc.) |
| Active Connections | Confusing with HTTP keep-alive | Currently open WebSocket + SSE connections |
| Threat Events | Too vague | Security incidents detected (injection attempts, suspicious patterns) |

#### 1.3 Add Metric Breakdown
- Show metrics by category (API calls, WebSocket, SSE)
- Add time-based filtering (last hour, 24h, 7d)
- Display trend indicators (up/down arrows with percentages)

---

## 2. Conversation Event Flow Fixes

### Current Issues

1. **One event per conversation**: Only showing single event instead of full conversation flow
2. **Stuck in "active" state**: Events remain active even after completion
3. **Default position obscured**: Flow canvas positioning hides content
4. **Missing information**: Lacks useful context (model, tokens, duration)
5. **Transfer bytes always 0**: Character count not being tracked

### Root Cause Analysis

The event system likely:
- Creates event on message send but never updates on completion
- Doesn't track streaming progress
- Missing byte counting in SSE handler
- **JSON field name mismatch**: Backend uses `snake_case` (e.g., `event_type`, `session_id`, `tool_call`), but frontend TypeScript interfaces expect `camelCase` (e.g., `eventType`, `sessionId`, `toolCall`)

### Proposed Fixes

#### 2.1 Fix JSON Field Name Mismatch (COMPLETED)

Files fixed:
- `web/src/api/companion.ts` - Updated `SessionEvent` interface to use snake_case
- `web/src/components/companion/SessionFlowCanvas.vue` - Updated field references
- `web/src/components/companion/SessionDetail.vue` - Updated field references

#### 2.2 Event Lifecycle Management

```
Event States:
- pending: Message sent, waiting for response
- streaming: Receiving AI response chunks
- completed: Response finished successfully
- error: Request failed
- cancelled: User cancelled streaming
```

#### 2.2 Real-time Event Updates

- Update event status when streaming starts
- Track bytes/tokens during streaming
- Mark completed when SSE sends `[DONE]`
- Update duration on completion

#### 2.3 Event Data Enhancement

Add to each event:
- Provider name
- Model used
- Input/output token counts
- Response duration (ms)
- Transfer size (request + response bytes)

#### 2.4 Flow Canvas Improvements

- Auto-scroll to show latest events
- Expand canvas default viewport
- Add zoom controls
- Show event details on hover/click

#### 2.5 Multi-Event Per Conversation

Each conversation should show:
- All message exchanges as connected nodes
- Branching for regenerated responses
- Clear visual flow from user → assistant → user

---

## 3. HTTP Connection Metrics Clarification

### Current Issues

1. **Connection count appears high**: HTTP keep-alive connections inflate numbers
2. **Long TTL**: Connections stay "active" too long after last use
3. **User confusion**: High numbers look alarming but are normal

### HTTP Keep-Alive Explanation

HTTP/1.1 and HTTP/2 use persistent connections:
- Browser opens connection, reuses for multiple requests
- Connection stays open for potential future requests
- Default keep-alive timeout varies (browser: 115s, server: configurable)

### Proposed Changes

#### 3.1 Rename Metrics for Clarity

| Current | Proposed | Description |
|---------|----------|-------------|
| Active Connections | Open Sockets | TCP connections currently open |
| - | Active Streams | Connections with ongoing request/response |
| - | Idle Connections | Keep-alive connections waiting for reuse |

#### 3.2 Reduce Keep-Alive Timeout

Current: Unknown (likely default)
Proposed: 30 seconds for idle connections

```go
// server configuration
server.IdleTimeout = 30 * time.Second
server.ReadHeaderTimeout = 10 * time.Second
```

#### 3.3 Connection Breakdown Display

Show connections by type:
- WebSocket (persistent, expected to be long-lived)
- SSE (streaming, active during AI response)
- HTTP (short-lived, should close quickly)

#### 3.4 Add Context to Numbers

Instead of just showing "47 connections", show:
```
Connections: 47
├── WebSocket: 2 (chat sessions)
├── SSE: 1 (streaming response)
└── HTTP Keep-alive: 44 (idle, will close in ~30s)
```

---

## 4. Security Scan Report Enhancement

### Current Issues

1. **Lack of detail**: Scan results show issues but no explanation of what they mean
2. **No remediation guidance**: Users don't know how to fix identified issues
3. **Manual fixes required**: Each issue must be addressed individually

### Proposed Changes

#### 4.1 Detailed Scan Report Format

Transform scan results into a comprehensive report:

```
Security Scan Report
Generated: 2026-02-01 20:30:00

Overall Score: 72/100 (Fair)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔴 Critical Issues (2)
├── [SEC-001] API Key Exposed in Config
│   ├── Location: config.yaml:15
│   ├── Risk: API keys in plaintext can be leaked
│   ├── Impact: Unauthorized access to external services
│   └── Fix: Move to environment variable or secrets manager
│
└── [SEC-002] CORS Allows All Origins
    ├── Location: server/config.go:42
    ├── Risk: Cross-site request forgery attacks
    ├── Impact: Malicious sites can make authenticated requests
    └── Fix: Restrict to specific trusted domains

🟠 Warnings (5)
├── [SEC-003] Weak JWT Expiration (24h)
├── [SEC-004] No Rate Limiting on /api/chat
├── [SEC-005] Debug Mode Enabled
├── [SEC-006] Missing Content-Security-Policy Header
└── [SEC-007] Outdated Dependencies (3 packages)

🟢 Passed Checks (12)
├── HTTPS Enforced
├── SQL Injection Protection
├── XSS Prevention
└── ... (click to expand)
```

#### 4.2 Issue Detail View

Each issue should expand to show:

| Field | Description |
|-------|-------------|
| Issue ID | Unique identifier (e.g., SEC-001) |
| Severity | Critical / High / Medium / Low / Info |
| Category | Authentication, Network, Configuration, Dependencies |
| Location | File path and line number |
| Description | What the issue is |
| Risk | Why this is a security concern |
| Impact | What could happen if exploited |
| Remediation | Step-by-step fix instructions |
| References | Links to OWASP, CVE, documentation |
| Auto-fixable | Yes/No indicator |

#### 4.3 One-Click Fix Feature

For auto-fixable issues, provide a "Fix" button:

**Fixable Issue Types:**
| Issue Type | Auto-Fix Action |
|------------|-----------------|
| Weak JWT expiration | Update config to recommended value (1h) |
| Missing security headers | Add headers to server config |
| Debug mode enabled | Set debug=false in config |
| CORS misconfiguration | Apply restrictive CORS policy |
| Outdated dependencies | Run `go mod tidy` / `npm update` |
| Insecure cookie settings | Enable Secure and HttpOnly flags |

**Fix Flow:**
```
[Scan Results] → [Select Issues] → [Preview Changes] → [Apply Fix] → [Verify]
```

**UI Components:**
1. **Fix Button** - Per-issue fix action
2. **Fix All** - Batch fix all auto-fixable issues
3. **Preview Modal** - Show diff of changes before applying
4. **Rollback** - Undo recent fixes if needed

#### 4.4 Fix Preview Dialog

Before applying any fix, show:
```
┌─────────────────────────────────────────────────┐
│ Preview Fix: SEC-005 Debug Mode Enabled         │
├─────────────────────────────────────────────────┤
│ File: server/config.yaml                        │
│                                                 │
│ - debug: true                                   │
│ + debug: false                                  │
│                                                 │
│ This change will:                               │
│ • Disable verbose error messages                │
│ • Hide stack traces from responses              │
│ • Improve performance slightly                  │
│                                                 │
│ ⚠️ Requires server restart to take effect       │
│                                                 │
│         [Cancel]              [Apply Fix]       │
└─────────────────────────────────────────────────┘
```

#### 4.5 Scan History & Trends

- Store scan results over time
- Show security score trend graph
- Compare current vs previous scan
- Highlight new issues and resolved issues

---

## Implementation Tasks

### Phase 1: Monitoring Integration
- [ ] Create Security page tabs (Overview, Monitoring, Events, Settings)
- [ ] Move monitoring components into Security page
- [ ] Add metric tooltips with definitions
- [ ] Implement time-based filtering

### Phase 2: Event Flow Fixes
- [ ] Implement event state machine (pending → streaming → completed)
- [ ] Add real-time event updates via WebSocket
- [ ] Track and display transfer bytes
- [ ] Fix flow canvas positioning and viewport
- [ ] Support multiple events per conversation

### Phase 3: Connection Metrics
- [ ] Rename connection metrics for clarity
- [ ] Configure appropriate keep-alive timeout
- [ ] Add connection type breakdown
- [ ] Display contextual information with counts

### Phase 4: Security Scan Report
- [ ] Design detailed scan report format with severity levels
- [ ] Add issue detail view with remediation guidance
- [ ] Implement one-click fix for auto-fixable issues
- [ ] Add fix preview dialog with diff view
- [ ] Create "Fix All" batch operation
- [ ] Add rollback capability for recent fixes
- [ ] Implement scan history and trend tracking

---

## Success Metrics

1. **Monitoring Integration**: Single page for all security insights
2. **Event Accuracy**: Events reflect real-time state, bytes tracked correctly
3. **Connection Clarity**: Users understand what connection counts mean
4. **Scan Usability**: Users can understand and fix security issues without external help

---

## Technical Notes

### Files to Modify

**Backend:**
- `server/internal/server/server.go` - HTTP timeout configuration
- `server/internal/security/handler.go` - Connection tracking
- `server/internal/server/chat.go` - Event lifecycle updates

**Frontend:**
- `web/src/views/SecurityView.vue` - Page restructure
- `web/src/components/companion/SessionFlowCanvas.vue` - Event flow fixes
- `web/src/api/security.ts` - New metric endpoints
- `web/src/components/security/ScanReport.vue` - New scan report component
- `web/src/components/security/FixPreviewDialog.vue` - Fix preview modal

### API Changes

New endpoint for detailed connection info:
```
GET /api/security/connections
Response: {
  "total": 47,
  "by_type": {
    "websocket": 2,
    "sse": 1,
    "http_keepalive": 44
  },
  "idle_timeout_seconds": 30
}
```

New endpoints for security scan:
```
GET /api/security/scan
Response: {
  "score": 72,
  "max_score": 100,
  "grade": "fair",
  "issues": [...],
  "passed": [...],
  "scanned_at": "2026-02-01T20:30:00Z"
}

GET /api/security/scan/issues/{id}
Response: {
  "id": "SEC-001",
  "severity": "critical",
  "category": "configuration",
  "title": "API Key Exposed in Config",
  "location": { "file": "config.yaml", "line": 15 },
  "description": "...",
  "risk": "...",
  "impact": "...",
  "remediation": "...",
  "references": ["https://owasp.org/..."],
  "auto_fixable": true
}

POST /api/security/scan/fix
Request: { "issue_ids": ["SEC-001", "SEC-005"] }
Response: {
  "fixed": ["SEC-005"],
  "failed": [{ "id": "SEC-001", "reason": "Requires manual intervention" }],
  "requires_restart": true
}

POST /api/security/scan/fix/preview
Request: { "issue_id": "SEC-005" }
Response: {
  "file": "server/config.yaml",
  "diff": "- debug: true\n+ debug: false",
  "warnings": ["Requires server restart"],
  "reversible": true
}
```

---

## Open Questions

1. Should we add connection limits per client IP?
2. Do we need historical event data persistence?
3. Should monitoring metrics be exportable (Prometheus format)?
