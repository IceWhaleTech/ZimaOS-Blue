# Security

This document describes the security architecture and features of ZimaOS Echo.

## Overview

ZimaOS Echo implements a comprehensive security layer that includes:

- **OIDC Provider**: Built-in OpenID Connect provider for authentication
- **User Management**: Local user accounts with secure password storage
- **Multi-Factor Authentication**: TOTP and WebAuthn/Passkeys support
- **Audit Logging**: Comprehensive security event logging
- **Sandbox Execution**: Isolated execution environment for untrusted code
- **Prompt Injection Defense**: Protection against LLM prompt injection attacks

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Security Layer                                    │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    OIDC Provider                                 │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ Discovery│ │  Token   │ │ UserInfo │ │   Authorization  │   │    │
│  │  │ Endpoint │ │ Endpoint │ │ Endpoint │ │     Endpoint     │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    User Management                               │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │  User    │ │ Password │ │   MFA    │ │     Session      │   │    │
│  │  │  Store   │ │  Argon2  │ │   TOTP   │ │    Management    │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    Audit & Sandbox                               │    │
│  │  ┌──────────────────────┐  ┌────────────────────────────────┐   │    │
│  │  │     Audit Logger     │  │      Sandbox Executor          │   │    │
│  │  │  - Action Logging    │  │  - Process Isolation           │   │    │
│  │  │  - Query Interface   │  │  - Resource Limits             │   │    │
│  │  │  - Retention Policy  │  │  - Syscall Filtering           │   │    │
│  │  └──────────────────────┘  └────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

## Authentication

### Password Authentication

ZimaOS Echo uses **Argon2id** for password hashing, the winner of the Password Hashing Competition and recommended by OWASP.

#### Configuration

```yaml
security:
  password:
    min_length: 12
    require_uppercase: true
    require_lowercase: true
    require_number: true
    require_special: true
    history_count: 5
    expiration_days: 0  # 0 = never expires
    lockout_threshold: 5
    lockout_duration: 15m
```

#### Argon2id Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| Memory | 64 MB | Memory cost |
| Iterations | 3 | Time cost |
| Parallelism | 4 | Parallel threads |
| Salt Length | 16 bytes | Random salt |
| Key Length | 32 bytes | Output hash length |

#### Password Policy

- Minimum 12 characters
- Must contain uppercase, lowercase, number, and special character
- Password history prevents reuse of last 5 passwords
- Account lockout after 5 failed attempts (15 minute duration)

### Multi-Factor Authentication (MFA)

#### TOTP (Time-based One-Time Password)

Compatible with standard authenticator apps (Google Authenticator, Authy, etc.).

```yaml
security:
  mfa:
    enabled: true
    required: false
    issuer: "ZimaOS-Echo"
    recovery_codes_count: 8
```

| Parameter | Value |
|-----------|-------|
| Algorithm | SHA1 |
| Digits | 6 |
| Period | 30 seconds |

#### WebAuthn / Passkeys

Support for hardware security keys and platform authenticators (Touch ID, Windows Hello, etc.).

Features:
- Passwordless authentication
- Discoverable credentials (usernameless login)
- Multiple credentials per user
- Credential management API

### Recovery Codes

- 8 recovery codes generated per user
- Each code is 8 characters (alphanumeric)
- Codes are hashed before storage
- Single-use (each code can only be used once)
- Can be regenerated at any time

## OIDC Provider

ZimaOS Echo includes a built-in OpenID Connect provider for SSO integration.

### Endpoints

| Endpoint | Path | Description |
|----------|------|-------------|
| Discovery | `/.well-known/openid-configuration` | OIDC configuration |
| JWKS | `/.well-known/jwks.json` | JSON Web Key Set |
| Authorization | `/oauth/authorize` | Authorization endpoint |
| Token | `/oauth/token` | Token endpoint |
| UserInfo | `/oauth/userinfo` | User information |
| Revocation | `/oauth/revoke` | Token revocation |
| Introspection | `/oauth/introspect` | Token introspection |

### Configuration

```yaml
security:
  oidc:
    enabled: true
    issuer: "https://your-domain.com"
    signing_key_path: "./keys/oidc.key"
    signing_key_rotation_days: 90
    access_token_ttl: 1h
    refresh_token_ttl: 720h  # 30 days
    authorization_code_ttl: 10m
    clients:
      - client_id: "web-app"
        client_secret: ""  # Public client
        redirect_uris:
          - "http://localhost:3000/callback"
        allowed_scopes:
          - "openid"
          - "profile"
          - "email"
```

### Supported Features

- **Grant Types**: Authorization Code, Refresh Token
- **PKCE**: Required for public clients (S256 method)
- **Scopes**: openid, profile, email
- **Signing Algorithm**: RS256
- **Key Rotation**: Automatic key rotation support

### Security Considerations

- PKCE is required for all public clients
- Strict redirect URI validation (exact match)
- Short-lived authorization codes (10 minutes)
- Refresh token rotation on use
- State parameter validation

## Rate Limiting

### General Rate Limiting

```yaml
ratelimit:
  rate: 100
  window: 1m
  cleanup_interval: 5m
```

### Authentication Rate Limiting

Separate rate limits for authentication endpoints to prevent brute force attacks:

| Endpoint | Rate | Window |
|----------|------|--------|
| Login | 5 | 1 minute |
| Password Reset | 3 | 1 hour |
| MFA Verification | 5 | 1 minute |

### Account Lockout

- Automatic lockout after 5 failed login attempts
- Lockout duration: 15 minutes
- Manual unlock available via admin API

## Audit Logging

### Overview

All security-relevant events are logged to an audit trail for compliance and forensics.

### Configuration

```yaml
security:
  audit:
    enabled: true
    retention_days: 90
    log_request_body: false
    log_response_body: false
    excluded_paths:
      - "/health"
      - "/metrics"
```

### Logged Events

#### Authentication Events
- Login success/failure
- Logout
- Password change
- MFA setup/disable
- WebAuthn registration/authentication

#### User Management Events
- User create/update/delete
- Role changes
- Account lock/unlock

#### API Access Events
- Sensitive endpoint access
- Configuration changes

#### System Events
- Service start/stop
- Configuration reload

### Audit Log Fields

| Field | Description |
|-------|-------------|
| ID | Unique identifier (UUID) |
| Timestamp | Event time |
| User ID | Acting user (nullable for anonymous) |
| Action | Event type |
| Resource Type | Affected resource type |
| Resource ID | Affected resource ID |
| IP Address | Client IP |
| User Agent | Client user agent |
| Request ID | Correlation ID |
| Status | Success/Failure |
| Details | Additional JSON data |
| Old Value | Previous state (for updates) |
| New Value | New state (for updates) |

### Retention

- Configurable retention period (default: 90 days)
- Automatic cleanup of old entries
- Export to CSV/JSON for archival

## Sandbox Execution

### Overview

The sandbox provides isolated execution for untrusted code with resource limits and syscall filtering.

### Configuration

```yaml
security:
  sandbox:
    enabled: true
    default_timeout: 30s
    max_timeout: 5m
    memory_limit: 256MB
    cpu_limit: 1.0  # 1 CPU core
    process_limit: 10
    network_enabled: false
```

### Platform Support

#### Linux (Primary)

Full isolation using:
- **Namespaces**: PID, NET, MNT, UTS, IPC isolation
- **Seccomp**: Syscall filtering (whitelist approach)
- **Cgroups**: Resource limits (CPU, memory, I/O, processes)

#### Windows

Isolation using:
- **Job Objects**: Process and resource limits
- **Restricted tokens**: Reduced privileges

#### macOS

Isolation using:
- **sandbox-exec**: Apple's sandbox framework
- **Resource limits**: Process limits via setrlimit

### Resource Limits

| Resource | Default | Maximum |
|----------|---------|---------|
| Timeout | 30s | 5m |
| Memory | 256 MB | Configurable |
| CPU | 1.0 core | Configurable |
| Processes | 10 | Configurable |
| Network | Disabled | Optional |

### Seccomp Profile (Linux)

The default seccomp profile allows only essential syscalls:

**Allowed syscalls include:**
- File operations: read, write, open, close, stat, fstat
- Process operations: exit, exit_group, fork, execve
- Memory operations: mmap, munmap, brk, mprotect
- I/O operations: ioctl, fcntl, dup, dup2, pipe

**Blocked by default:**
- Network syscalls (when network disabled)
- Privileged operations (mount, umount, ptrace)
- Module loading (init_module, delete_module)

## API Security

### Prompt Injection Defense

ZimaOS Echo includes comprehensive protection against prompt injection attacks targeting LLM interactions.

#### Overview

Prompt injection is a security vulnerability where malicious users attempt to manipulate LLM behavior by injecting instructions into user input. The prompt guard module provides multi-layered defense:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Prompt Injection Defense                              │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Input Detection                               │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │  Role    │ │Delimiter │ │Jailbreak │ │  Data Exfil      │   │    │
│  │  │Injection │ │ Attacks  │ │ Patterns │ │  Detection       │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    Output Filtering                              │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │Sensitive │ │  System  │ │   Code   │ │      PII         │   │    │
│  │  │  Data    │ │  Prompt  │ │Injection │ │    Filter        │   │    │
│  │  │  Filter  │ │   Leak   │ │  Filter  │ │                  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Configuration

```yaml
security:
  prompt_guard:
    enabled: true
    block_on_threat: true
    log_threats: true
    block_threshold: high  # none, low, medium, high, critical
    max_input_length: 100000

    # Detection modules
    detection:
      role_injection: true
      instruction_override: true
      delimiter_attacks: true
      encoding_attacks: true
      jailbreak_patterns: true
      data_exfiltration: true

    # Output filtering
    output_filter:
      sensitive_data: true
      system_prompt_leak: true
      code_injection: true
      pii_filter: false  # Disabled by default for performance
```

#### Threat Levels

| Level | Score | Description |
|-------|-------|-------------|
| None | 0 | No threat detected |
| Low | 10 | Minor suspicious pattern |
| Medium | 25 | Moderate risk pattern |
| High | 50 | Significant threat |
| Critical | 100 | Severe attack attempt |

#### Detection Categories

**Role Injection**
- System/assistant/user role prefix injection
- Instruction override attempts ("ignore previous instructions")
- Role pretending ("pretend you are...")
- Memory wipe attempts ("forget everything")

**Instruction Override**
- System override commands
- Developer/admin mode requests
- Jailbreak commands
- Safety disable attempts

**Delimiter Attacks**
- Markdown code block injection
- XML/HTML tag injection
- JSON structure manipulation
- Comment-based injection

**Encoding Attacks**
- Base64 encoded payloads
- Unicode escape sequences
- URL encoding abuse
- HTML entity encoding

**Jailbreak Patterns**
- DAN (Do Anything Now) patterns
- Hypothetical scenario bypass
- Roleplay-based bypass
- Character/persona manipulation

**Data Exfiltration**
- System prompt leak attempts
- Training data extraction
- Internal information requests
- Credential extraction attempts

#### Output Filtering

The output filter protects against:

**Sensitive Data**
- API keys and secrets
- Private keys
- JWT tokens
- Database connection strings
- Passwords

**System Prompt Leakage**
- Automatic detection of system prompt content in output
- Instruction disclosure prevention
- Rules disclosure prevention

**Code Injection**
- Dangerous shell commands (rm -rf, sudo, etc.)
- SQL injection patterns
- Script tags
- Code execution functions

**PII (Optional)**
- Email addresses
- Phone numbers
- Social Security Numbers
- Credit card numbers
- IP addresses

#### API Usage

```go
// Create a prompt guard
guard := promptguard.NewGuard(nil, nil)

// Check input for injection
result := guard.CheckInput(userInput)
if result.IsThreat {
    log.Warnf("Threat detected: level=%s, score=%d",
        result.ThreatLevel, result.Score)
}

// Filter output
filterResult := guard.FilterOutput(llmResponse)
if filterResult.WasFiltered {
    // Use filtered output
    response = filterResult.FilteredOutput
}

// Process chat with full protection
safeInput, result, err := guard.ProcessChat(userInput)
if err == promptguard.ErrPromptInjectionDetected {
    return errors.New("request blocked due to security concerns")
}
```

#### Middleware Integration

```go
// Add prompt guard middleware to Echo
e.Use(promptguard.Middleware(&promptguard.MiddlewareConfig{
    BlockOnThreat: true,
    LogThreats:    true,
    InputFields:   []string{"content", "message", "prompt"},
    FilterOutput:  true,
}))
```

#### Custom Patterns

Add custom detection patterns:

```go
detector.AddPattern(promptguard.PatternRule{
    Name:        "custom_attack",
    Pattern:     `(?i)my\s+custom\s+pattern`,
    Severity:    promptguard.ThreatHigh,
    Description: "Custom attack pattern",
})
```

Add custom output filters:

```go
filter.AddFilter(promptguard.OutputFilterRule{
    Name:        "custom_filter",
    Pattern:     `sensitive\s+data`,
    Replacement: "[REDACTED]",
    Description: "Custom sensitive data filter",
})
```

#### Best Practices

1. **Enable all detection modules** for comprehensive protection
2. **Set appropriate block threshold** based on your risk tolerance
3. **Monitor threat logs** to identify attack patterns
4. **Add custom patterns** for application-specific threats
5. **Use output filtering** to prevent data leakage
6. **Set system prompt patterns** to detect prompt leakage
7. **Regularly update patterns** as new attack vectors emerge

### Authentication Methods

1. **API Key**: Header-based authentication
   ```
   Authorization: Bearer <api-key>
   ```

2. **JWT Token**: OAuth 2.0 access tokens
   ```
   Authorization: Bearer <jwt-token>
   ```

3. **Session Cookie**: Browser-based sessions

### CORS Configuration

```yaml
server:
  cors:
    allowed_origins:
      - "https://your-domain.com"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
    allowed_headers:
      - "Authorization"
      - "Content-Type"
    max_age: 86400
```

### Security Headers

The following security headers are set by default:

| Header | Value |
|--------|-------|
| X-Content-Type-Options | nosniff |
| X-Frame-Options | DENY |
| X-XSS-Protection | 1; mode=block |
| Strict-Transport-Security | max-age=31536000; includeSubDomains |
| Content-Security-Policy | default-src 'self' |

## Data Protection

### Encryption at Rest

- Sensitive data (MFA secrets, API keys) encrypted using AES-256-GCM
- Encryption key derived from master secret using HKDF
- Separate keys for different data types

### Encryption in Transit

- TLS 1.2+ required for all connections
- Strong cipher suites only
- Certificate validation enforced

### Secure Configuration

- Secrets loaded from environment variables or secure vault
- Configuration files should have restricted permissions (600)
- Sensitive values masked in logs

## Security Best Practices

### Deployment

1. **Use HTTPS**: Always deploy behind TLS
2. **Firewall**: Restrict access to necessary ports only
3. **Updates**: Keep the system and dependencies updated
4. **Monitoring**: Enable audit logging and alerting

### Configuration

1. **Strong Passwords**: Enforce password policy
2. **MFA**: Enable MFA for all users
3. **API Keys**: Rotate API keys regularly
4. **Least Privilege**: Use minimal required permissions

### Operations

1. **Audit Review**: Regularly review audit logs
2. **Access Review**: Periodically review user access
3. **Incident Response**: Have a plan for security incidents
4. **Backup**: Regular encrypted backups

## Security API Reference

### Authentication Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | User login |
| POST | `/api/v1/auth/logout` | User logout |
| POST | `/api/v1/auth/password` | Change password |
| POST | `/api/v1/auth/password/reset` | Request password reset |
| POST | `/api/v1/auth/password/reset/confirm` | Confirm password reset |

### MFA Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/mfa/setup` | Start MFA setup |
| POST | `/api/v1/auth/mfa/verify` | Verify and enable MFA |
| POST | `/api/v1/auth/mfa/disable` | Disable MFA |
| GET | `/api/v1/auth/mfa/status` | Get MFA status |
| GET | `/api/v1/auth/mfa/recovery` | Get recovery codes |
| POST | `/api/v1/auth/mfa/recovery/regenerate` | Regenerate codes |

### WebAuthn Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/webauthn/register/begin` | Start registration |
| POST | `/api/v1/auth/webauthn/register/finish` | Complete registration |
| POST | `/api/v1/auth/webauthn/login/begin` | Start login |
| POST | `/api/v1/auth/webauthn/login/finish` | Complete login |
| GET | `/api/v1/auth/webauthn/credentials` | List credentials |
| PUT | `/api/v1/auth/webauthn/credentials/:id` | Update credential |
| DELETE | `/api/v1/auth/webauthn/credentials/:id` | Delete credential |

### Audit Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/audit` | List audit logs |
| GET | `/api/v1/audit/:id` | Get audit entry |
| GET | `/api/v1/audit/export` | Export audit logs |

### Sandbox Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/sandbox/execute` | Execute in sandbox |
| GET | `/api/v1/sandbox/status/:id` | Get execution status |
| POST | `/api/v1/sandbox/kill/:id` | Kill execution |

## Vulnerability Reporting

If you discover a security vulnerability, please report it responsibly:

1. **Do not** disclose publicly until fixed
2. Email security concerns to the maintainers
3. Include detailed reproduction steps
4. Allow reasonable time for a fix

## Compliance

ZimaOS Echo security features support compliance with:

- **OWASP Top 10**: Protection against common vulnerabilities
- **GDPR**: Audit logging and data protection
- **SOC 2**: Access controls and monitoring
