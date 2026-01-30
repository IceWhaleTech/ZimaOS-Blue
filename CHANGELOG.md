# Changelog

All notable changes to ZimaOS-Echo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.10.5] - 2026-01-30

### Added

#### API Proxy Sidecar (v0.10.5.1 - Core Infrastructure)
- **Port Management**: Dynamic port allocation with configurable range (19000-19100)
- **Connection Pool**: HTTP/2 connection pooling with keep-alive support
- **Route Selection**: Priority-based, round-robin, and weighted load balancing
- **Failover Mechanism**: Automatic failover with circuit breaker pattern
- **Health Check**: Periodic health checks for all configured providers
- **Proxy Server**: Full request forwarding with SSE streaming support
- **Management API**: Status, health, and provider management endpoints

#### Security & Monitoring (v0.10.5.2)
- **Session Monitoring**: Track active sessions with idle timeout cleanup
- **Prompt Injection Guard**: Detect and block prompt injection attempts
- **Usage Statistics**: Token counting, latency tracking, throughput metrics
- **Authentication**: API key, bearer token, and basic auth support
- **Rate Limiting**: Per-IP rate limiting with burst allowance
- **Request Pipeline**: Middleware chain for auth, guard, and metrics

#### Advanced Features (v0.10.5.3)
- **Model Compatibility Layer**: Adapt requests for different model capabilities
- **Mock Endpoints**: Testing endpoints for development
- **Configuration Hot-Reload**: Reload config without restart
- **Data Masking Interface**: Reserved for future PII masking implementation

#### UI Components
- `ConfigurableDashboard.vue` - Main dashboard with customizable cards
- `FailoverStatus.vue` - Provider failover status display
- `ProviderSettings.vue` - Provider configuration panel
- `SecurityAlerts.vue` - Security alerts display
- `ProxyMetricsPanel.vue` - Proxy metrics visualization
- `SessionMonitor.vue` - Active session monitoring
- `StatsDashboard.vue` - Usage statistics dashboard

#### Performance Optimization (v0.10.5.4)
- **Buffer Pooling**: Reusable byte buffers to reduce GC pressure
- **Response Caching**: TTL-based caching with LRU eviction
- **Connection Metrics**: Track connection reuse rate and latency
- **HTTP/2 Multiplexing**: Optimized connection handling

#### Testing & QA (v0.10.5.5)
- Comprehensive unit tests for all proxy components
- Health check tests
- Streaming response tests
- Security test cases for prompt injection detection
- Authentication bypass tests
- Rate limit bypass tests
- Input validation tests

#### Documentation (v0.10.5.6)
- API Reference (`docs/api-proxy-reference.md`)
- Configuration Guide (`docs/proxy-configuration-guide.md`)
- OpenAPI Specification (`docs/openapi-proxy.yaml`)

### API Endpoints

#### Core Proxy API
- `GET /api/v1/proxy/status` - Get proxy server status
- `GET /api/v1/proxy/health` - Health check endpoint
- `GET /api/v1/proxy/providers` - List all providers

#### Session & Metrics API
- `GET /api/v1/proxy/sessions` - List active sessions
- `GET /api/v1/proxy/sessions/:id` - Get session details
- `GET /api/v1/proxy/metrics` - Get usage metrics
- `GET /api/v1/proxy/metrics/providers` - Get per-provider metrics
- `GET /api/v1/proxy/metrics/latency` - Get latency statistics
- `GET /api/v1/proxy/metrics/timeseries` - Get time-series data

#### Security API
- `GET /api/v1/proxy/guard/stats` - Get guard statistics
- `GET/POST /api/v1/proxy/guard/rules` - Manage guard rules
- `GET /api/v1/proxy/auth/stats` - Get auth statistics
- `GET/POST/DELETE /api/v1/proxy/auth/keys` - Manage API keys

#### Advanced API
- `GET /api/v1/proxy/models` - List model compatibility
- `GET/POST /api/v1/proxy/mock` - Manage mock endpoints
- `POST /api/v1/proxy/config/reload` - Force config reload
- `GET /api/v1/proxy/masking/stats` - Get masking statistics
- `GET/POST/DELETE /api/v1/proxy/masking/rules` - Manage masking rules

#### Failover API
- `GET /api/v1/proxy/failover/metrics` - Get failover metrics
- `GET/PUT /api/v1/proxy/failover/config` - Manage failover config
- `POST /api/v1/proxy/failover/reset` - Reset circuit breakers
- `GET /api/v1/proxy/failover/breakers` - Get circuit breaker status

### Changed
- Updated frontend API client with new proxy endpoints
- Enhanced provider pool management with failover support

### Security
- Prompt injection detection with configurable patterns
- API key authentication for proxy endpoints
- Rate limiting to prevent abuse
- IP allowlist support

### Performance
- Buffer pool reduces memory allocations by >90%
- Connection reuse rate >90% with HTTP/2
- Response caching for repeated requests (optional)

---

## [0.10.4] - Previous Release

See previous release notes for earlier changes.
