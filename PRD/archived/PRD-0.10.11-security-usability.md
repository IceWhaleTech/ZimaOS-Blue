# PRD v0.10.11 - Security Feature Usability Enhancement

## Version Information
- **Version**: 0.10.11
- **Created**: 2026-02-01
- **Status**: Draft
- **Priority**: High

## Background & Problems

Current security features have the following usability issues:

### 1. Security Scan Issues
- All security check items return static "passed" status without real check logic
- Scan results lack practical meaning and cannot detect real security risks

### 2. Companion Session Information Issues
- Many fields are not populated with real data
- Session information is not real-time
- Demo Mode is mixed with real data, users cannot distinguish
- Current page connection is not reflected

### 3. Connection Monitoring Issues
- HTTP short connections are not tracked and displayed
- WebSocket/WSS connection status is opaque
- Missing real-time connection list data source API

### 4. Data Persistence Issues
- Session data is stored in memory, lost after restart
- Security events retain maximum 1000 entries, no persistence
- Threat statistics are memory-based, no historical data

---

## Goals

1. **Real Security Scanning** - Implement meaningful security check logic
2. **Complete Session Information** - Populate all necessary fields, provide real-time data
3. **Connection Visualization** - Display all connection types (HTTP/WS/WSS)
4. **Data Reliability** - Implement persistence for critical data

---

## Implementation Checklist

### Phase 1: Real Security Scanning

#### 1.1 Authentication Security Checks
- [x] **AUTH-001**: Implement password policy check - Verify system password complexity requirements
- [x] **AUTH-002**: Implement JWT security check - Verify JWT key strength and expiration configuration
- [x] **AUTH-003**: Implement session management check - Verify session timeout and concurrent session limits
- [x] **AUTH-004**: Implement MFA status check - Detect if multi-factor authentication is enabled

#### 1.2 Input Validation Checks
- [x] **INPUT-001**: Implement XSS protection check - Verify input filtering and output encoding configuration
- [x] **INPUT-002**: Implement SQL injection protection check - Verify parameterized query usage
- [x] **INPUT-003**: Implement command injection protection check - Verify command execution input filtering
- [x] **INPUT-004**: Implement path traversal protection check - Verify file path validation logic

#### 1.3 Network Security Checks
- [x] **NET-001**: Implement rate limiting check - Verify API rate limit configuration
- [x] **NET-002**: Implement CORS configuration check - Verify cross-origin policy security
- [x] **NET-003**: Implement TLS/HTTPS check - Verify HTTPS enforcement and certificate configuration
- [x] **NET-004**: Implement IP blocking check - Verify IP blacklist functionality

#### 1.4 AI Security Checks
- [x] **AI-001**: Implement Prompt injection protection check - Verify Prompt Guard is enabled
- [x] **AI-002**: Implement AI output validation check - Verify output filtering configuration
- [x] **AI-003**: Implement model access control check - Verify model whitelist configuration
- [x] **AI-004**: Implement sensitive data filtering check - Verify PII filtering is enabled

#### 1.5 System Security Checks
- [x] **SYS-001**: Implement file permission check - Verify configuration file permissions
- [x] **SYS-002**: Implement debug mode check - Verify production environment has debug disabled
- [x] **SYS-003**: Implement dependency security check - Check for known vulnerable dependencies
- [x] **SYS-004**: Implement error handling check - Verify error messages don't leak sensitive data

#### 1.6 Sandbox Security Checks (New)
- [x] **SANDBOX-001**: Implement sandbox enabled check - Verify code execution is sandboxed
- [x] **SANDBOX-002**: Implement memory limit check - Verify sandbox memory limit configuration
- [x] **SANDBOX-003**: Implement execution timeout check - Verify execution timeout configuration
- [x] **SANDBOX-004**: Implement network isolation check - Verify sandbox network access restrictions

---

### Phase 2: Companion Session Information Enhancement

#### 2.1 Session Field Population
- [x] **SESSION-001**: Populate `clientIP` field - Extract real client IP from request
- [x] **SESSION-002**: Populate `userAgent` field - Record client User-Agent
- [x] **SESSION-003**: Populate `platform` field - Identify platform based on actual source
- [x] **SESSION-004**: Populate `tokensUsed` field - Real-time cumulative token usage
- [x] **SESSION-005**: Populate `eventCount` field - Real-time event count update
- [x] **SESSION-006**: Populate `threatLevel` field - Calculate based on threat detection results

#### 2.2 Real-time Data Updates
- [x] **REALTIME-001**: Implement session state real-time push - Push state changes via WebSocket
- [x] **REALTIME-002**: Implement event real-time stream - Push new events immediately to frontend
- [x] **REALTIME-003**: Implement statistics real-time update - Real-time token/event count refresh
- [x] **REALTIME-004**: Implement threat level real-time calculation - Update immediately when threat detected

#### 2.3 Demo Mode Separation
- [x] **DEMO-001**: Add clear demo mode indicator - Display "Demo Mode" banner in UI
- [x] **DEMO-002**: Separate demo data storage - Demo data not mixed with real data
- [x] **DEMO-003**: Add demo mode toggle - Explicit enable/disable in settings
- [x] **DEMO-004**: Demo data style differentiation - Use different colors/styles for demo data

#### 2.4 Session Flow Visualization (Canvas)
- [x] **CANVAS-001**: Create SessionFlowCanvas component - Canvas-based flow diagram rendering
- [x] **CANVAS-002**: Implement node rendering - Support message, tool call, LLM request, security check node types
- [x] **CANVAS-003**: Implement connection line rendering - Arrow connections between nodes
- [x] **CANVAS-004**: Implement interaction features - Zoom, drag, node click, hover tooltips
- [x] **CANVAS-005**: Implement view switching - Flow diagram/timeline view toggle
- [x] **CANVAS-006**: Add legend - Color explanation for different node types
- [x] **CANVAS-007**: Add i18n support - English/Chinese translations

---

### Phase 3: Connection Monitoring Enhancement

#### 3.1 HTTP Connection Tracking
- [x] **HTTP-001**: Implement HTTP request tracking middleware - Record all HTTP requests
- [x] **HTTP-002**: Implement active connection list API - `GET /api/v1/connections/active`
- [x] **HTTP-003**: Implement connection details API - `GET /api/v1/connections/:id`
- [x] **HTTP-004**: Implement connection statistics API - `GET /api/v1/connections/stats`

#### 3.2 WebSocket Connection Monitoring
- [x] **WS-001**: Implement WebSocket connection registration - Register to connection pool on connect
- [x] **WS-002**: Implement WebSocket connection unregistration - Remove from pool on disconnect
- [x] **WS-003**: Implement WebSocket connection list - Display all active WS connections
- [x] **WS-004**: Implement WebSocket connection details - Show connection duration, message count, etc.

#### 3.3 Connection Visualization
- [x] **VIS-001**: Implement connection type categorization - HTTP/WS/WSS categorized display
- [x] **VIS-002**: Implement connection timeline - Show connection establish and close times
- [x] **VIS-003**: Implement connection geolocation - Show connection origin based on IP (on-demand query)
- [x] **VIS-004**: Implement connection real-time refresh - Auto-refresh connection list

#### 3.4 Current Page Connection Indication
- [x] **PAGE-001**: Implement page connection indicator - Status bar shows current connection status
- [x] **PAGE-002**: Implement connection type icons - Distinguish HTTP/WS/SSE connections
- [x] **PAGE-003**: Implement connection quality indicator - Show latency and stability
- [x] **PAGE-004**: Implement disconnection/reconnection prompt - Notify user when connection drops

---

### Phase 4: Data Persistence

#### 4.1 Session Data Persistence
- [x] **PERSIST-001**: Design session data table structure - JSONL file storage (Companion)
- [x] **PERSIST-002**: Implement session data write - Persist on session create/update
- [x] **PERSIST-003**: Implement session data read - Load historical sessions on startup
- [x] **PERSIST-004**: Implement session data cleanup - Periodically clean expired sessions

#### 4.2 Security Event Persistence
- [x] **EVENT-001**: Design security event table structure - JSONL file storage by date
- [x] **EVENT-002**: Implement async event write - Non-blocking main flow
- [x] **EVENT-003**: Implement event query API - Support time range and type filtering
- [x] **EVENT-004**: Implement event statistics aggregation - Daily/weekly/monthly statistics

#### 4.3 Threat Data Persistence
- [x] **THREAT-001**: Design threat record table structure - BlockedIP JSON storage
- [x] **THREAT-002**: Implement threat record write - Record when threat detected
- [x] **THREAT-003**: Implement threat trend analysis - Historical threat trend charts
- [x] **THREAT-004**: Implement threat report export - Support PDF/CSV export

---

## Technical Approach

### Backend Changes

1. **Security Scanner Refactor** (`server/internal/security/scanner.go`)
   - Change static checks to dynamic checks
   - Add configuration reading and validation logic
   - Implement real security assessment

2. **Connection Manager** (`server/internal/connection/manager.go`)
   - Add connection tracking module
   - Implement connection lifecycle management
   - Provide connection query API

3. **Data Persistence Layer** (`server/internal/storage/`)
   - Add SQLite support
   - Implement Data Access Objects (DAO)
   - Add data migration scripts

### Frontend Changes

1. **Connection Status Component** (`web/src/components/ConnectionStatus.vue`)
   - Status bar connection indicator
   - Connection type icons
   - Real-time status updates

2. **Security Scan Results Optimization** (`web/src/views/SecurityView.vue`)
   - Display real check results
   - Add fix recommendations
   - Support re-scan

3. **Demo Mode UI** (`web/src/components/DemoModeIndicator.vue`)
   - Demo mode indicator
   - Toggle switch
   - Data source explanation

---

## Acceptance Criteria

### Phase 1 Acceptance
- [x] Security scan can detect real configuration issues
- [x] Scan results include specific fix recommendations
- [x] Re-scan shows passed after fixing issues
- [x] Create SecurityScanner standalone module
- [x] Support configurable scan parameters

### Phase 2 Acceptance
- [x] All session information fields have real data
- [x] Data update latency < 1 second (WebSocket real-time push)
- [x] Demo mode has clear indicator (yellow banner)
- [x] Session flow visualization Canvas component available
- [x] Support flow diagram/timeline view switching

### Phase 3 Acceptance
- [x] Can see all active HTTP connections
- [x] Can see all active WebSocket connections
- [x] Current page connection status displayed in real-time

### Phase 4 Acceptance
- [x] Historical data not lost after service restart
- [x] Can query security events within 7 days
- [x] Can export threat reports

---

## Risks & Dependencies

### Risks
1. **Performance Impact** - Connection tracking may affect request latency
   - Mitigation: Use async recording, batch writes
2. **Storage Growth** - Persisted data may grow rapidly
   - Mitigation: Implement data retention policy, periodic cleanup

### Dependencies
1. SQLite or other embedded database
2. Frontend state management optimization

---

## Timeline

| Phase | Content | Priority |
|-------|---------|----------|
| Phase 1 | Real Security Scanning | P0 |
| Phase 2 | Companion Session Enhancement | P0 |
| Phase 3 | Connection Monitoring Enhancement | P1 |
| Phase 4 | Data Persistence | P1 |

---

## Appendix

### Related Files
- Security Scan: [handler.go](server/internal/security/handler.go)
- **Security Scanner**: [scanner.go](server/internal/security/scanner.go) ✅ New
- Threat Detection: [threat_detector.go](server/internal/security/threat_detector.go)
- Companion Session: [types.go](server/internal/companion/types.go)
- WebSocket Stream: [websocket.go](server/internal/companion/websocket.go)
- Connection Monitoring: [session.go](server/internal/proxy/session.go)
- Frontend Security View: [SecurityView.vue](web/src/views/SecurityView.vue)
- Frontend Companion Store: [companion.ts](web/src/stores/companion.ts)
- **Session Flow Canvas**: [SessionFlowCanvas.vue](web/src/components/companion/SessionFlowCanvas.vue) ✅ New
- **Session Detail Component**: [SessionDetail.vue](web/src/components/companion/SessionDetail.vue) ✅ Updated
