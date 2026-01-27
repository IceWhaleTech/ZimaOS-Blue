# 安全

本文档描述了 ZimaOS Echo 的安全架构和功能。

## 概述

ZimaOS Echo 实现了全面的安全层，包括：

- **OIDC 提供者**：内置 OpenID Connect 提供者用于身份验证
- **用户管理**：本地用户账户，安全密码存储
- **多因素认证**：支持 TOTP 和 WebAuthn/Passkeys
- **审计日志**：全面的安全事件日志
- **沙箱执行**：用于不受信任代码的隔离执行环境
- **提示词注入防御**：防护 LLM 提示词注入攻击

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           安全层                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    OIDC 提供者                                   │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ 发现端点 │ │ 令牌端点 │ │用户信息  │ │    授权端点      │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    用户管理                                      │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ 用户存储 │ │ 密码     │ │   MFA    │ │    会话管理      │   │    │
│  │  │          │ │ Argon2   │ │   TOTP   │ │                  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    审计与沙箱                                    │    │
│  │  ┌──────────────────────┐  ┌────────────────────────────────┐   │    │
│  │  │     审计日志器       │  │      沙箱执行器                │   │    │
│  │  │  - 操作日志          │  │  - 进程隔离                    │   │    │
│  │  │  - 查询接口          │  │  - 资源限制                    │   │    │
│  │  │  - 保留策略          │  │  - 系统调用过滤                │   │    │
│  │  └──────────────────────┘  └────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

## 身份验证

### 密码认证

ZimaOS Echo 使用 **Argon2id** 进行密码哈希，这是密码哈希竞赛的获胜者，也是 OWASP 推荐的算法。

#### 配置

```yaml
security:
  password:
    min_length: 12
    require_uppercase: true
    require_lowercase: true
    require_number: true
    require_special: true
    history_count: 5
    expiration_days: 0  # 0 = 永不过期
    lockout_threshold: 5
    lockout_duration: 15m
```

#### Argon2id 参数

| 参数 | 值 | 描述 |
|------|-----|------|
| 内存 | 64 MB | 内存成本 |
| 迭代次数 | 3 | 时间成本 |
| 并行度 | 4 | 并行线程数 |
| 盐长度 | 16 字节 | 随机盐 |
| 密钥长度 | 32 字节 | 输出哈希长度 |

#### 密码策略

- 最少 12 个字符
- 必须包含大写字母、小写字母、数字和特殊字符
- 密码历史记录防止重复使用最近 5 个密码
- 5 次失败尝试后账户锁定（锁定时长 15 分钟）

### 多因素认证 (MFA)

#### TOTP（基于时间的一次性密码）

兼容标准身份验证器应用（Google Authenticator、Authy 等）。

```yaml
security:
  mfa:
    enabled: true
    required: false
    issuer: "ZimaOS-Echo"
    recovery_codes_count: 8
```

| 参数 | 值 |
|------|-----|
| 算法 | SHA1 |
| 位数 | 6 |
| 周期 | 30 秒 |

#### WebAuthn / Passkeys

支持硬件安全密钥和平台身份验证器（Touch ID、Windows Hello 等）。

功能：
- 无密码认证
- 可发现凭据（无用户名登录）
- 每个用户多个凭据
- 凭据管理 API

### 恢复码

- 每个用户生成 8 个恢复码
- 每个码 8 个字符（字母数字）
- 存储前进行哈希处理
- 一次性使用（每个码只能使用一次）
- 可随时重新生成

## OIDC 提供者

ZimaOS Echo 包含内置的 OpenID Connect 提供者，用于 SSO 集成。

### 端点

| 端点 | 路径 | 描述 |
|------|------|------|
| 发现 | `/.well-known/openid-configuration` | OIDC 配置 |
| JWKS | `/.well-known/jwks.json` | JSON Web 密钥集 |
| 授权 | `/oauth/authorize` | 授权端点 |
| 令牌 | `/oauth/token` | 令牌端点 |
| 用户信息 | `/oauth/userinfo` | 用户信息 |
| 撤销 | `/oauth/revoke` | 令牌撤销 |
| 内省 | `/oauth/introspect` | 令牌内省 |

### 配置

```yaml
security:
  oidc:
    enabled: true
    issuer: "https://your-domain.com"
    signing_key_path: "./keys/oidc.key"
    signing_key_rotation_days: 90
    access_token_ttl: 1h
    refresh_token_ttl: 720h  # 30 天
    authorization_code_ttl: 10m
    clients:
      - client_id: "web-app"
        client_secret: ""  # 公共客户端
        redirect_uris:
          - "http://localhost:3000/callback"
        allowed_scopes:
          - "openid"
          - "profile"
          - "email"
```

### 支持的功能

- **授权类型**：授权码、刷新令牌
- **PKCE**：公共客户端必需（S256 方法）
- **作用域**：openid、profile、email
- **签名算法**：RS256
- **密钥轮换**：支持自动密钥轮换

### 安全考虑

- 所有公共客户端必须使用 PKCE
- 严格的重定向 URI 验证（精确匹配）
- 短期授权码（10 分钟）
- 使用时刷新令牌轮换
- 状态参数验证

## 速率限制

### 通用速率限制

```yaml
ratelimit:
  rate: 100
  window: 1m
  cleanup_interval: 5m
```

### 认证速率限制

认证端点有单独的速率限制，以防止暴力攻击：

| 端点 | 速率 | 窗口 |
|------|------|------|
| 登录 | 5 | 1 分钟 |
| 密码重置 | 3 | 1 小时 |
| MFA 验证 | 5 | 1 分钟 |

### 账户锁定

- 5 次登录失败后自动锁定
- 锁定时长：15 分钟
- 可通过管理员 API 手动解锁

## 审计日志

### 概述

所有安全相关事件都记录到审计跟踪中，用于合规和取证。

### 配置

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

### 记录的事件

#### 认证事件
- 登录成功/失败
- 登出
- 密码更改
- MFA 设置/禁用
- WebAuthn 注册/认证

#### 用户管理事件
- 用户创建/更新/删除
- 角色更改
- 账户锁定/解锁

#### API 访问事件
- 敏感端点访问
- 配置更改

#### 系统事件
- 服务启动/停止
- 配置重载

### 审计日志字段

| 字段 | 描述 |
|------|------|
| ID | 唯一标识符 (UUID) |
| 时间戳 | 事件时间 |
| 用户 ID | 操作用户（匿名时可为空） |
| 操作 | 事件类型 |
| 资源类型 | 受影响的资源类型 |
| 资源 ID | 受影响的资源 ID |
| IP 地址 | 客户端 IP |
| 用户代理 | 客户端用户代理 |
| 请求 ID | 关联 ID |
| 状态 | 成功/失败 |
| 详情 | 附加 JSON 数据 |
| 旧值 | 之前的状态（用于更新） |
| 新值 | 新状态（用于更新） |

### 保留

- 可配置的保留期（默认：90 天）
- 自动清理旧条目
- 导出为 CSV/JSON 用于归档

## 沙箱执行

### 概述

沙箱为不受信任的代码提供隔离执行，具有资源限制和系统调用过滤。

### 配置

```yaml
security:
  sandbox:
    enabled: true
    default_timeout: 30s
    max_timeout: 5m
    memory_limit: 256MB
    cpu_limit: 1.0  # 1 个 CPU 核心
    process_limit: 10
    network_enabled: false
```

### 平台支持

#### Linux（主要）

使用以下技术实现完全隔离：
- **命名空间**：PID、NET、MNT、UTS、IPC 隔离
- **Seccomp**：系统调用过滤（白名单方式）
- **Cgroups**：资源限制（CPU、内存、I/O、进程）

#### Windows

使用以下技术实现隔离：
- **作业对象**：进程和资源限制
- **受限令牌**：降低权限

#### macOS

使用以下技术实现隔离：
- **sandbox-exec**：Apple 的沙箱框架
- **资源限制**：通过 setrlimit 进行进程限制

### 资源限制

| 资源 | 默认值 | 最大值 |
|------|--------|--------|
| 超时 | 30 秒 | 5 分钟 |
| 内存 | 256 MB | 可配置 |
| CPU | 1.0 核心 | 可配置 |
| 进程数 | 10 | 可配置 |
| 网络 | 禁用 | 可选 |

### Seccomp 配置文件（Linux）

默认 seccomp 配置文件只允许必要的系统调用：

**允许的系统调用包括：**
- 文件操作：read、write、open、close、stat、fstat
- 进程操作：exit、exit_group、fork、execve
- 内存操作：mmap、munmap、brk、mprotect
- I/O 操作：ioctl、fcntl、dup、dup2、pipe

**默认阻止：**
- 网络系统调用（当网络禁用时）
- 特权操作（mount、umount、ptrace）
- 模块加载（init_module、delete_module）

## API 安全

### 提示词注入防御

ZimaOS Echo 包含全面的提示词注入攻击防护，保护 LLM 交互安全。

#### 概述

提示词注入是一种安全漏洞，恶意用户试图通过在用户输入中注入指令来操纵 LLM 行为。提示词防护模块提供多层防御：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    提示词注入防御                                        │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    输入检测                                      │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │  角色    │ │ 分隔符   │ │ 越狱     │ │  数据泄露        │   │    │
│  │  │  注入    │ │  攻击    │ │  模式    │ │    检测          │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                │                                         │
│  ┌─────────────────────────────┴───────────────────────────────────┐    │
│  │                    输出过滤                                      │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │    │
│  │  │ 敏感数据 │ │  系统    │ │   代码   │ │      PII         │   │    │
│  │  │   过滤   │ │  提示词  │ │  注入    │ │     过滤         │   │    │
│  │  │          │ │   泄露   │ │   过滤   │ │                  │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 配置

```yaml
security:
  prompt_guard:
    enabled: true
    block_on_threat: true
    log_threats: true
    block_threshold: high  # none, low, medium, high, critical
    max_input_length: 100000

    # 检测模块
    detection:
      role_injection: true
      instruction_override: true
      delimiter_attacks: true
      encoding_attacks: true
      jailbreak_patterns: true
      data_exfiltration: true

    # 输出过滤
    output_filter:
      sensitive_data: true
      system_prompt_leak: true
      code_injection: true
      pii_filter: false  # 默认禁用以提高性能
```

#### 威胁级别

| 级别 | 分数 | 描述 |
|------|------|------|
| None | 0 | 未检测到威胁 |
| Low | 10 | 轻微可疑模式 |
| Medium | 25 | 中等风险模式 |
| High | 50 | 重大威胁 |
| Critical | 100 | 严重攻击尝试 |

#### 检测类别

**角色注入**
- System/assistant/user 角色前缀注入
- 指令覆盖尝试（"忽略之前的指令"）
- 角色伪装（"假装你是..."）
- 记忆清除尝试（"忘记所有"）

**指令覆盖**
- 系统覆盖命令
- 开发者/管理员模式请求
- 越狱命令
- 安全禁用尝试

**分隔符攻击**
- Markdown 代码块注入
- XML/HTML 标签注入
- JSON 结构操纵
- 注释注入

**编码攻击**
- Base64 编码载荷
- Unicode 转义序列
- URL 编码滥用
- HTML 实体编码

**越狱模式**
- DAN（Do Anything Now）模式
- 假设场景绕过
- 角色扮演绕过
- 角色/人格操纵

**数据泄露**
- 系统提示词泄露尝试
- 训练数据提取
- 内部信息请求
- 凭据提取尝试

#### 输出过滤

输出过滤器防护：

**敏感数据**
- API 密钥和密钥
- 私钥
- JWT 令牌
- 数据库连接字符串
- 密码

**系统提示词泄露**
- 自动检测输出中的系统提示词内容
- 指令泄露防护
- 规则泄露防护

**代码注入**
- 危险 shell 命令（rm -rf、sudo 等）
- SQL 注入模式
- Script 标签
- 代码执行函数

**PII（可选）**
- 电子邮件地址
- 电话号码
- 社会安全号码
- 信用卡号码
- IP 地址

#### API 使用

```go
// 创建提示词防护
guard := promptguard.NewGuard(nil, nil)

// 检查输入是否有注入
result := guard.CheckInput(userInput)
if result.IsThreat {
    log.Warnf("检测到威胁: level=%s, score=%d",
        result.ThreatLevel, result.Score)
}

// 过滤输出
filterResult := guard.FilterOutput(llmResponse)
if filterResult.WasFiltered {
    // 使用过滤后的输出
    response = filterResult.FilteredOutput
}

// 使用完整保护处理聊天
safeInput, result, err := guard.ProcessChat(userInput)
if err == promptguard.ErrPromptInjectionDetected {
    return errors.New("由于安全问题，请求被阻止")
}
```

#### 中间件集成

```go
// 将提示词防护中间件添加到 Echo
e.Use(promptguard.Middleware(&promptguard.MiddlewareConfig{
    BlockOnThreat: true,
    LogThreats:    true,
    InputFields:   []string{"content", "message", "prompt"},
    FilterOutput:  true,
}))
```

#### 自定义模式

添加自定义检测模式：

```go
detector.AddPattern(promptguard.PatternRule{
    Name:        "custom_attack",
    Pattern:     `(?i)my\s+custom\s+pattern`,
    Severity:    promptguard.ThreatHigh,
    Description: "自定义攻击模式",
})
```

添加自定义输出过滤器：

```go
filter.AddFilter(promptguard.OutputFilterRule{
    Name:        "custom_filter",
    Pattern:     `sensitive\s+data`,
    Replacement: "[已编辑]",
    Description: "自定义敏感数据过滤器",
})
```

#### 最佳实践

1. **启用所有检测模块** 以获得全面保护
2. **设置适当的阻止阈值** 根据您的风险承受能力
3. **监控威胁日志** 以识别攻击模式
4. **添加自定义模式** 针对应用特定威胁
5. **使用输出过滤** 防止数据泄露
6. **设置系统提示词模式** 检测提示词泄露
7. **定期更新模式** 随着新攻击向量的出现

### 认证方法

1. **API 密钥**：基于头部的认证
   ```
   Authorization: Bearer <api-key>
   ```

2. **JWT 令牌**：OAuth 2.0 访问令牌
   ```
   Authorization: Bearer <jwt-token>
   ```

3. **会话 Cookie**：基于浏览器的会话

### CORS 配置

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

### 安全头部

默认设置以下安全头部：

| 头部 | 值 |
|------|-----|
| X-Content-Type-Options | nosniff |
| X-Frame-Options | DENY |
| X-XSS-Protection | 1; mode=block |
| Strict-Transport-Security | max-age=31536000; includeSubDomains |
| Content-Security-Policy | default-src 'self' |

## 数据保护

### 静态加密

- 敏感数据（MFA 密钥、API 密钥）使用 AES-256-GCM 加密
- 加密密钥使用 HKDF 从主密钥派生
- 不同数据类型使用不同密钥

### 传输加密

- 所有连接要求 TLS 1.2+
- 仅使用强密码套件
- 强制证书验证

### 安全配置

- 从环境变量或安全保险库加载密钥
- 配置文件应具有受限权限（600）
- 日志中屏蔽敏感值

## 安全最佳实践

### 部署

1. **使用 HTTPS**：始终在 TLS 后部署
2. **防火墙**：仅限制访问必要端口
3. **更新**：保持系统和依赖项更新
4. **监控**：启用审计日志和告警

### 配置

1. **强密码**：强制执行密码策略
2. **MFA**：为所有用户启用 MFA
3. **API 密钥**：定期轮换 API 密钥
4. **最小权限**：使用最小必需权限

### 运维

1. **审计审查**：定期审查审计日志
2. **访问审查**：定期审查用户访问权限
3. **事件响应**：制定安全事件响应计划
4. **备份**：定期加密备份

## 安全 API 参考

### 认证端点

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/auth/login` | 用户登录 |
| POST | `/api/v1/auth/logout` | 用户登出 |
| POST | `/api/v1/auth/password` | 更改密码 |
| POST | `/api/v1/auth/password/reset` | 请求密码重置 |
| POST | `/api/v1/auth/password/reset/confirm` | 确认密码重置 |

### MFA 端点

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/auth/mfa/setup` | 开始 MFA 设置 |
| POST | `/api/v1/auth/mfa/verify` | 验证并启用 MFA |
| POST | `/api/v1/auth/mfa/disable` | 禁用 MFA |
| GET | `/api/v1/auth/mfa/status` | 获取 MFA 状态 |
| GET | `/api/v1/auth/mfa/recovery` | 获取恢复码 |
| POST | `/api/v1/auth/mfa/recovery/regenerate` | 重新生成恢复码 |

### WebAuthn 端点

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/auth/webauthn/register/begin` | 开始注册 |
| POST | `/api/v1/auth/webauthn/register/finish` | 完成注册 |
| POST | `/api/v1/auth/webauthn/login/begin` | 开始登录 |
| POST | `/api/v1/auth/webauthn/login/finish` | 完成登录 |
| GET | `/api/v1/auth/webauthn/credentials` | 列出凭据 |
| PUT | `/api/v1/auth/webauthn/credentials/:id` | 更新凭据 |
| DELETE | `/api/v1/auth/webauthn/credentials/:id` | 删除凭据 |

### 审计端点

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/audit` | 列出审计日志 |
| GET | `/api/v1/audit/:id` | 获取审计条目 |
| GET | `/api/v1/audit/export` | 导出审计日志 |

### 沙箱端点

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/sandbox/execute` | 在沙箱中执行 |
| GET | `/api/v1/sandbox/status/:id` | 获取执行状态 |
| POST | `/api/v1/sandbox/kill/:id` | 终止执行 |

## 漏洞报告

如果您发现安全漏洞，请负责任地报告：

1. **不要**在修复前公开披露
2. 通过邮件向维护者报告安全问题
3. 包含详细的复现步骤
4. 给予合理的修复时间

## 合规性

ZimaOS Echo 安全功能支持以下合规要求：

- **OWASP Top 10**：防护常见漏洞
- **GDPR**：审计日志和数据保护
- **SOC 2**：访问控制和监控
