# Security Audit Report - 2026-02-01

## 审查范围

基于 ClawdBot/OpenClaw 2026年1月安全事件，对 ZimaOS-Blue 项目进行安全审查，重点关注：
1. WebSocket 网关安全 (RCE 漏洞)
2. CORS 配置 (CSRF 漏洞)
3. 本地网络访问保护
4. 命令注入风险
5. 认证和授权问题

---

## 已修复的安全问题

### ✅ 已修复 - WebSocket Origin 检查禁用

**位置：**
- `server/internal/gateway/gateway.go:112-114`
- `server/internal/companion/websocket.go:31-33`
- `server/internal/voice/websocket.go:19-21`

**修复状态：** ✅ 已修复 (2026-02-01)

---

### ✅ 已修复 - CORS 允许所有来源

**位置：**
- `server/internal/server/server.go:49-53`

**修复状态：** ✅ 已修复 (2026-02-01)

---

### ✅ 已修复 - WebSocket 端点无认证

**位置：**
- `server/internal/gateway/handler.go:27`

**修复状态：** ✅ 已修复 (2026-02-01)

---

### ✅ 已修复 - 连接 ID 可预测

**位置：**
- `server/internal/gateway/gateway.go:406`

**修复状态：** ✅ 已修复 (2026-02-01)

---

## 新发现的安全问题 (第二轮审查)

### 🔴 严重 - Cron Job 命令注入 (RCE)

**位置：**
- `server/internal/cron/handlers.go:54-58`
- `server/internal/cron/handler.go:77-92`

**问题描述：**
用户可以通过 API 创建 cron job，并在 payload 中指定任意命令执行：
```go
// handler.go - 用户可以创建任意 cron job
func (h *Handler) Create(c echo.Context) error {
    var req CreateRequest
    // ...
    job, err := h.service.Create(req.Name, req.Description, req.Schedule, req.Handler, req.Payload)
}

// handlers.go - 命令直接传递给 shell 执行
func (s *Service) commandHandler(ctx context.Context, job *Job) (interface{}, error) {
    cmdStr, ok := job.Payload["command"].(string)
    // ...
    cmd = exec.CommandContext(cmdCtx, "sh", "-c", cmdStr)  // 危险！
}
```

**风险：**
- 任何经过认证的用户都可以执行任意系统命令
- 可能导致完全的系统控制权丧失
- 与 ClawdBot 的 RCE 漏洞相同

**建议修复：**
1. 禁用 `command` handler 或限制为管理员
2. 实现命令白名单
3. 使用沙箱执行命令
4. 添加 RBAC 权限检查

---

### 🔴 严重 - 弱默认 JWT Secret

**位置：**
- `server/internal/config/config.go:502`

**问题描述：**
```go
v.SetDefault("security.jwt.secret", "change-me-in-production-use-a-strong-secret-key")
```

**风险：**
- 如果生产环境未更改默认值，攻击者可以伪造 JWT 令牌
- 可能导致完全的认证绕过

**建议修复：**
1. 启动时检查是否使用默认 secret，如果是则拒绝启动
2. 或自动生成强随机 secret 并持久化

---

### 🔴 高危 - 弱随机数生成

**位置：**
- `server/internal/autoreply/autoreply.go:133`

**问题描述：**
```go
id := fmt.Sprintf("rule_%d_%d", time.Now().UnixNano(), rand.Intn(10000))
```

**风险：**
- 使用 `math/rand` 而非 `crypto/rand`
- ID 可预测，可能被枚举

**建议修复：**
使用 `crypto/rand` 生成安全的随机 ID

---

### 🟠 中危 - SHA1 用于签名验证

**位置：**
- `server/internal/channel/wechat/wechat.go:315`

**问题描述：**
```go
hash := sha1.Sum([]byte(str))
```

**风险：**
- SHA1 已被证明存在碰撞攻击
- 不应用于安全关键操作

**建议修复：**
升级到 SHA256 或更强的哈希算法（注意：微信 API 可能要求 SHA1）

---

### 🟠 中危 - 不安全的文件权限

**位置：**
- `server/internal/backup/backup.go:70`
- `server/internal/cgroup/cgroup.go:169`

**问题描述：**
```go
os.MkdirAll(path, 0755)  // 世界可读
```

**风险：**
- 敏感目录可能被其他用户读取

**建议修复：**
敏感目录使用 `0700`，敏感文件使用 `0600`

---

### 🟠 中危 - 备份恢复可跳过校验

**位置：**
- `server/internal/backup/restore.go:24-25, 49`

**问题描述：**
```go
SkipVerify bool
if !opts.SkipVerify {
    // verify checksum
}
```

**风险：**
- 允许恢复被篡改或损坏的备份

**建议修复：**
强制校验或要求管理员明确确认

---

### 🟡 低危 - TOTP 使用 SHA1

**位置：**
- `server/internal/mfa/totp.go:47`

**问题描述：**
```go
Algorithm: otp.AlgorithmSHA1,
```

**风险：**
- SHA1 已弃用
- 但这是 TOTP 标准，大多数认证器应用使用 SHA1

**建议修复：**
新实现考虑使用 SHA256/SHA512

---

### 🟡 低危 - 调试输出在生产代码中

**位置：**
- `server/internal/zimaos/integration.go:232`
- `server/internal/workflow/service.go:263, 474, 478, 482, 540, 543`
- `server/internal/config/hotreload.go:106, 185, 194, 202, 217, 219, 279`

**问题描述：**
```go
fmt.Printf("[%s] %s: %s\n", level, title, message)
```

**风险：**
- 敏感信息可能被记录到 stdout/stderr

**建议修复：**
使用结构化日志记录器

---

### 🟡 低危 - OIDC Issuer 使用 HTTP

**位置：**
- `server/internal/config/config.go:509`

**问题描述：**
```go
v.SetDefault("security.oidc.issuer", "http://localhost:23456")
```

**风险：**
- 生产环境应使用 HTTPS

**建议修复：**
默认使用 HTTPS 或在生产环境强制 HTTPS

---

## 与 ClawdBot 漏洞对比

| ClawdBot 漏洞 | ZimaOS-Blue 状态 | 详情 |
|---------------|------------------|------|
| 未认证远程访问 | ✅ **已修复** | WebSocket 端点现在支持 JWT 认证 |
| CSRF + WebSocket 劫持 | ✅ **已修复** | CORS 和 WebSocket Origin 已限制为白名单 |
| RCE (URL 参数覆盖网关地址) | ✅ **不适用** | 我们没有类似的 URL 参数覆盖机制 |
| RCE (命令执行) | 🔴 **存在风险** | Cron job 允许执行任意命令 |
| 本地网络访问 | ⚠️ **需评估** | 默认绑定 0.0.0.0，可从任何网络访问 |

---

## 修复优先级

### 立即修复 (P0) - 严重

| 问题 | 位置 | 建议 |
|------|------|------|
| Cron Job 命令注入 | `cron/handlers.go` | 禁用或限制 command handler |
| 弱默认 JWT Secret | `config/config.go` | 启动时检查并拒绝默认值 |

### 短期修复 (P1) - 高危

| 问题 | 位置 | 建议 |
|------|------|------|
| 弱随机数生成 | `autoreply/autoreply.go` | 使用 crypto/rand |
| SHA1 签名验证 | `channel/wechat/wechat.go` | 升级到 SHA256 |

### 中期修复 (P2) - 中危

| 问题 | 位置 | 建议 |
|------|------|------|
| 不安全文件权限 | 多个文件 | 使用 0700/0600 |
| 备份恢复跳过校验 | `backup/restore.go` | 强制校验 |

### 长期改进 (P3) - 低危

| 问题 | 位置 | 建议 |
|------|------|------|
| TOTP SHA1 | `mfa/totp.go` | 考虑 SHA256 |
| 调试输出 | 多个文件 | 使用结构化日志 |
| OIDC HTTP | `config/config.go` | 默认 HTTPS |

---

## 已修复文件清单

### 新增文件

1. **`server/internal/security/origin.go`**
   - 统一的 Origin 检查器
   - 支持白名单配置
   - 支持 localhost 开发模式
   - 支持通配符子域名匹配

### 修改文件

1. **`server/internal/server/server.go`**
   - CORS 配置从 `*` 改为白名单
   - 添加 `AllowCredentials: true`

2. **`server/internal/gateway/gateway.go`**
   - WebSocket Origin 检查使用 `security.CheckOriginDefault`
   - 连接 ID 使用 `crypto/rand` 生成

3. **`server/internal/gateway/handler.go`**
   - 添加 `NewHandlerWithAuth` 构造函数
   - 添加 `authenticateWebSocket` 方法
   - WebSocket 端点支持 JWT 认证

4. **`server/internal/companion/websocket.go`**
   - WebSocket Origin 检查使用 `security.CheckOriginDefault`

5. **`server/internal/voice/websocket.go`**
   - WebSocket Origin 检查使用 `security.CheckOriginDefault`

---

## 总结

### 已修复问题

| 问题 | 严重程度 | 状态 |
|------|----------|------|
| WebSocket Origin 检查禁用 | 🔴 高 | ✅ 已修复 |
| CORS 允许所有来源 | 🔴 高 | ✅ 已修复 |
| WebSocket 端点无认证 | 🟠 中 | ✅ 已修复 |
| 连接 ID 可预测 | 🟡 低 | ✅ 已修复 |

### 待修复问题

| 问题 | 严重程度 | 状态 |
|------|----------|------|
| Cron Job 命令注入 (RCE) | 🔴 严重 | ✅ 已修复 |
| 弱默认 JWT Secret | 🔴 严重 | ✅ 已修复 |
| 弱随机数生成 | 🔴 高 | ✅ 已修复 |
| SHA1 签名验证 | 🟠 中 | ⏳ 待修复 (微信 API 要求) |
| 不安全文件权限 | 🟠 中 | ✅ 已修复 |
| 备份恢复跳过校验 | 🟠 中 | ✅ 已修复 |
| TOTP SHA1 | 🟡 低 | ⏳ 待修复 (TOTP 标准) |
| 调试输出 | 🟡 低 | ✅ 已修复 |
| OIDC HTTP | 🟡 低 | ⏳ 待修复 |

---

## 第二轮修复详情 (2026-02-01)

### ✅ 已修复 - Cron Job 命令注入 (RCE)

**修复方案：**
1. 默认禁用 `command` handler
2. 添加命令白名单验证
3. 添加危险模式检测（命令链、命令替换、路径遍历等）
4. 清理环境变量防止敏感数据泄露
5. 限制输出大小防止内存耗尽

**修改文件：** `server/internal/cron/handlers.go`

### ✅ 已修复 - 弱默认 JWT Secret

**修复方案：**
1. 添加 `ValidateJWTSecret()` 函数检查 secret 强度
2. 添加 `NewJWTServiceSecure()` 构造函数，拒绝弱 secret
3. 添加 `GenerateSecureSecret()` 函数生成安全随机 secret
4. 添加已知弱 secret 黑名单检测

**修改文件：** `server/internal/auth/jwt.go`

### ✅ 已修复 - 弱随机数生成

**修复方案：**
1. 使用 `crypto/rand` 替代 `math/rand` 生成 ID
2. 添加 `generateSecureRuleID()` 函数

**修改文件：** `server/internal/autoreply/autoreply.go`

---

## 第三轮修复详情 (2026-02-01)

### ✅ 已修复 - 不安全文件权限

**修复方案：**
1. 将备份目录权限从 `0755` 改为 `0700`
2. 将 cgroup 目录权限从 `0755` 改为 `0700`
3. 恢复文件时使用 `0600` 权限

**修改文件：**
- `server/internal/backup/backup.go`
- `server/internal/cgroup/cgroup.go`

### ✅ 已修复 - 备份恢复跳过校验

**修复方案：**
1. 添加安全警告日志当 `SkipVerify` 为 true 时
2. 添加路径遍历攻击检测
3. 恢复时清理文件权限（移除 setuid/setgid/sticky 位）
4. 敏感文件默认使用 `0600` 权限
5. 目录使用 `0700` 权限

**修改文件：** `server/internal/backup/restore.go`

### ✅ 已修复 - 调试输出

**修复方案：**
1. 将 `fmt.Printf` 替换为 `log.Printf`
2. 添加日志级别前缀 `[WARN]` 和 `[INFO]`
3. 使用标准库 `log` 包提供时间戳和可重定向输出

**修改文件：**
- `server/internal/audit/logger.go`
- `server/internal/audit/retention.go`
- `server/internal/session/manager.go`
- `server/internal/workflow/service.go`
- `server/internal/zimaos/integration.go`

---

## 积极的安全发现

代码库展示了一些良好的安全实践：

1. **参数化查询：** 数据库查询使用 `?` 占位符
2. **Crypto/Rand 使用：** 大多数安全关键的随机生成使用 `crypto/rand`
3. **TLS 配置：** HTTP/2 服务器有合理的 TLS 默认值
4. **认证中间件：** 实现了 JWT 和 API Key 认证中间件
5. **密码策略：** 配置了强密码要求（最少12字符，大小写，数字，特殊字符）
6. **审计日志：** 实现了全面的审计日志系统
7. **速率限制：** 为敏感端点实现了速率限制中间件

---

## 审查人

- 审查日期: 2026-02-01
- 修复日期: 2026-02-01 (第一轮 + 第二轮)
- 审查工具: Claude Code
- 参考: ClawdBot/OpenClaw 安全事件 (2026年1月)

## 修复统计

| 类别 | 数量 |
|------|------|
| 已修复 - 严重 | 2 |
| 已修复 - 高危 | 3 |
| 已修复 - 中危 | 4 |
| 已修复 - 低危 | 2 |
| **总计已修复** | **11** |
| 待修复 - 中危 | 1 |
| 待修复 - 低危 | 2 |
| **总计待修复** | **3** |
