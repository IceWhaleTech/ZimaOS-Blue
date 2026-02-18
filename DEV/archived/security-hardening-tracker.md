# Security Hardening Tracker

本文档追踪 ZimaOS-Blue 项目的所有安全加固项，包括已实现的、计划中的，以及来自外部项目（如 ClawdBot/OpenClaw）的安全更新。

---

## ClawdBot/OpenClaw 漏洞对照表 (2026年1月)

> 以下是 ClawdBot/OpenClaw 项目在 2026 年 1 月被披露的安全漏洞，以及我们项目的防护状态。

### 严重漏洞 (Critical)

| ClawdBot 漏洞 | 描述 | ZimaOS-Blue 状态 | 防护措施 |
|---------------|------|------------------|----------|
| **未认证远程访问** | 端口 18789 默认无认证暴露，攻击者可在几分钟内完全控制系统 | ✅ **已防护** | 强制 JWT/API Key 认证，无 `auth: "none"` 模式 |
| **API Key 明文暴露** | OpenAI/Anthropic 密钥明文存储，可通过 Shodan 搜索发现 | ✅ **已防护** | AES-256-GCM 加密存储，SHA-256 哈希 |
| **远程代码执行 (RCE)** | URL 参数可覆盖 WebSocket 网关地址，泄露认证令牌 | ⚠️ **需审查** | WebSocket 有连接限制，但需加强令牌验证 |
| **权限提升** | 无目录沙箱或访问控制，完全系统权限 | ✅ **已防护** | 沙箱隔离、cgroup 资源限制、工作目录隔离 |
| **凭证窃取** | OAuth secrets 和 bot tokens 暴露在公开控制面板 | ✅ **已防护** | 加密存储、不暴露敏感信息到前端 |

### 高危漏洞 (High)

| ClawdBot 漏洞 | 描述 | ZimaOS-Blue 状态 | 防护措施 |
|---------------|------|------------------|----------|
| **Prompt 注入** | AI 无法区分合法指令和注入内容，5分钟攻击演示 | ✅ **已防护** | Prompt 注入检测器，威胁检测系统 |
| **CSRF + WebSocket 劫持** | 跨站请求伪造结合 WebSocket 协议滥用 | ⚠️ **需审查** | 有 CORS 配置但当前允许 `*` |
| **控制流劫持** | 假错误消息成功率 45-64% | ⚠️ **需审查** | 有威胁检测，但需加强错误消息验证 |
| **Feed 投毒** | 5 个精心构造的文档可 90% 操纵 AI 响应 | ⚠️ **部分防护** | 有输入验证，但需加强内容过滤 |

### 中危漏洞 (Medium)

| ClawdBot 漏洞 | 描述 | ZimaOS-Blue 状态 | 防护措施 |
|---------------|------|------------------|----------|
| **成本攻击** | 失控代理可产生 $50K/月基础设施成本 | ⚠️ **需实现** | 需要添加 API 消费限制 |
| **账户接管** | 项目重命名时假冒攻击，10秒内抢注句柄 | ✅ **不适用** | 我们不依赖外部社交账户 |
| **隐写术串通** | 代理间可隐蔽交换消息，绕过人工监督 | ⚠️ **需评估** | 需要评估多代理场景 |

### ClawdBot 安全修复同步状态

| ClawdBot 修复 (commit 8cb0fa9, 2026-01-28) | 描述 | 我们的状态 |
|---------------------------------------------|------|------------|
| 移除 `auth: "none"` 模式 | 强制所有实例必须认证 | ✅ **已同步** - 我们从未支持无认证模式 |
| WebSocket 网关令牌验证 | 修复 URL 参数覆盖漏洞 | ⚠️ **需审查** |
| 本地网络访问保护 | 防止 localhost 实例被利用 | ⚠️ **需审查** |
| npm 包重命名 | 从 moltbot 到 openclaw | ✅ **不适用** |

---

## 已实现的安全加固 (Implemented)

### 1. 认证系统 (Authentication)

| 项目 | 状态 | 来源 | 实现位置 | 防护的 ClawdBot 漏洞 |
|------|------|------|----------|---------------------|
| JWT 双令牌系统 (Access + Refresh) | ✅ 已实现 | 自研 | `server/internal/auth/jwt.go` | 未认证远程访问 |
| JWT 令牌黑名单机制 | ✅ 已实现 | 自研 | `server/internal/auth/jwt.go` | 会话劫持 |
| API Key SHA-256 哈希存储 | ✅ 已实现 | 自研 | `server/internal/auth/apikey.go` | API Key 明文暴露 |
| API Key AES-256-GCM 加密 | ✅ 已实现 | 自研 | `server/internal/auth/encryption.go` | API Key 明文暴露 |
| API Key 作用域控制 | ✅ 已实现 | 自研 | `server/internal/auth/apikey.go` | 权限提升 |
| API Key 轮换支持 | ✅ 已实现 | 自研 | `server/internal/auth/apikey.go` | 凭证窃取 |
| 强制认证模式 (无 auth:none) | ✅ 已实现 | ClawdBot | `server/internal/auth/middleware.go` | 未认证远程访问 |

### 2. 密码安全 (Password Security)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| Argon2id 密码哈希 (OWASP 推荐) | ✅ 已实现 | 自研 | `server/internal/password/argon2.go` | 密码破解 |
| 密码策略验证 | ✅ 已实现 | 自研 | `server/internal/password/policy.go` | 弱密码 |
| 常量时间比较防时序攻击 | ✅ 已实现 | 自研 | `server/internal/password/argon2.go` | 时序攻击 |
| 过期哈希自动重新哈希 | ✅ 已实现 | 自研 | `server/internal/password/argon2.go` | 算法降级 |

### 3. 多因素认证 (MFA)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| TOTP 支持 | ✅ 已实现 | 自研 | `server/internal/mfa/totp.go` | 账户接管 |
| WebAuthn 硬件密钥支持 | ✅ 已实现 | 自研 | `server/internal/mfa/webauthn.go` | 钓鱼攻击 |
| 恢复码机制 | ✅ 已实现 | 自研 | `server/internal/mfa/recovery.go` | 账户锁定 |
| 时钟漂移容忍 (±1 周期) | ✅ 已实现 | 自研 | `server/internal/mfa/totp.go` | 时间同步问题 |

### 4. 速率限制 (Rate Limiting)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| 登录尝试限制 (5次/分钟) | ✅ 已实现 | 自研 | `server/internal/ratelimit/ratelimit.go` | 暴力破解 |
| 密码重置限制 (3次/小时) | ✅ 已实现 | 自研 | `server/internal/ratelimit/ratelimit.go` | 账户枚举 |
| MFA 尝试限制 (5次/分钟) | ✅ 已实现 | 自研 | `server/internal/ratelimit/ratelimit.go` | MFA 绕过 |
| 自动封禁机制 (15分钟) | ✅ 已实现 | 自研 | `server/internal/ratelimit/ratelimit.go` | 持续攻击 |
| Retry-After 响应头 | ✅ 已实现 | 自研 | `server/internal/ratelimit/ratelimit.go` | 客户端重试风暴 |

### 5. 沙箱隔离 (Sandbox Isolation)

| 项目 | 状态 | 来源 | 实现位置 | 防护的 ClawdBot 漏洞 |
|------|------|------|----------|---------------------|
| 命令执行超时控制 | ✅ 已实现 | 自研 | `server/internal/sandbox/executor.go` | 资源耗尽 |
| 环境变量清理 | ✅ 已实现 | ClawdBot | `server/internal/sandbox/executor.go` | 权限提升 |
| 危险环境变量阻止 (LD_PRELOAD等) | ✅ 已实现 | ClawdBot | `server/internal/security/sandbox.go` | 权限提升 |
| 工作目录隔离 | ✅ 已实现 | 自研 | `server/internal/sandbox/executor.go` | 路径遍历 |
| Linux cgroup 资源限制 | ✅ 已实现 | 自研 | `server/internal/sandbox/linux.go` | 资源耗尽 |
| 输出截断保护 | ✅ 已实现 | 自研 | `server/internal/sandbox/executor.go` | 内存耗尽 |

### 6. 威胁检测 (Threat Detection)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| 注入攻击检测 | ✅ 已实现 | 自研 | `server/internal/security/threat_detector.go` | 代码注入 |
| XSS 检测 | ✅ 已实现 | 自研 | `server/internal/security/threat_detector.go` | 跨站脚本 |
| SQL 注入检测 | ✅ 已实现 | 自研 | `server/internal/security/threat_detector.go` | 数据库攻击 |
| 路径遍历检测 | ✅ 已实现 | 自研 | `server/internal/security/threat_detector.go` | 文件访问 |
| 命令注入检测 | ✅ 已实现 | 自研 | `server/internal/security/threat_detector.go` | RCE |
| **Prompt 注入检测** | ✅ 已实现 | ClawdBot | `server/internal/security/threat_detector.go` | Prompt 注入 |
| 暴力破解检测 | ✅ 已实现 | 自研 | `server/internal/security/threat_detector.go` | 暴力破解 |

### 7. 命令安全 (Command Security)

| 项目 | 状态 | 来源 | 实现位置 | 防护的 ClawdBot 漏洞 |
|------|------|------|----------|---------------------|
| 危险命令模式阻止 ($(), &&, ||, ;, |) | ✅ 已实现 | ClawdBot | `server/internal/security/sandbox.go` | RCE |
| 参数验证 (空字节、换行) | ✅ 已实现 | 自研 | `server/internal/security/sandbox.go` | 命令注入 |
| PATH 注入防护 | ✅ 已实现 | 自研 | `server/internal/security/sandbox.go` | 权限提升 |

### 8. WebSocket 安全 (WebSocket Security)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| 连接数限制 (默认 1000) | ✅ 已实现 | 自研 | `server/internal/gateway/gateway.go` | DoS |
| 消息大小限制 (默认 512KB) | ✅ 已实现 | 自研 | `server/internal/gateway/gateway.go` | 内存耗尽 |
| Ping/Pong 心跳 (30秒) | ✅ 已实现 | 自研 | `server/internal/gateway/gateway.go` | 僵尸连接 |
| 读写超时 (10秒) | ✅ 已实现 | 自研 | `server/internal/gateway/gateway.go` | 慢速攻击 |

### 9. RBAC 权限控制 (Role-Based Access Control)

| 项目 | 状态 | 来源 | 实现位置 | 防护的 ClawdBot 漏洞 |
|------|------|------|----------|---------------------|
| 角色定义与权限 | ✅ 已实现 | 自研 | `server/internal/rbac/rbac.go` | 权限提升 |
| 权限继承 | ✅ 已实现 | 自研 | `server/internal/rbac/rbac.go` | 权限混乱 |
| 循环继承检测 | ✅ 已实现 | 自研 | `server/internal/rbac/rbac.go` | 配置错误 |
| 通配符权限匹配 | ✅ 已实现 | 自研 | `server/internal/rbac/rbac.go` | 细粒度控制 |

### 10. 会话管理 (Session Management)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| 会话跟踪 (IP + User-Agent) | ✅ 已实现 | 自研 | `server/internal/security/handler.go` | 会话劫持 |
| 会话过期 | ✅ 已实现 | 自研 | `server/internal/security/handler.go` | 会话固定 |
| 活动跟踪 | ✅ 已实现 | 自研 | `server/internal/security/handler.go` | 异常检测 |

### 11. 加密系统 (Encryption)

| 项目 | 状态 | 来源 | 实现位置 | 防护的 ClawdBot 漏洞 |
|------|------|------|----------|---------------------|
| AES-256-GCM 认证加密 | ✅ 已实现 | 自研 | `server/internal/auth/encryption.go` | 凭证窃取 |
| PBKDF2 密钥派生 (100,000 迭代) | ✅ 已实现 | 自研 | `server/internal/auth/encryption.go` | 密钥破解 |
| 密钥轮换支持 | ✅ 已实现 | 自研 | `server/internal/auth/encryption.go` | 密钥泄露 |
| 安全随机数生成 | ✅ 已实现 | 自研 | `server/internal/auth/encryption.go` | 可预测性 |

### 12. 外部认证 (External Authentication)

| 项目 | 状态 | 来源 | 实现位置 | 防护的攻击类型 |
|------|------|------|----------|---------------|
| OIDC 提供者支持 | ✅ 已实现 | 自研 | `server/internal/extauth/handler.go` | 认证绕过 |
| 账户链接/解绑 | ✅ 已实现 | 自研 | `server/internal/extauth/handler.go` | 账户接管 |
| 令牌交换和刷新 | ✅ 已实现 | 自研 | `server/internal/extauth/handler.go` | 令牌泄露 |

---

## 待实现的安全加固 (Planned)

### 高优先级 (High Priority)

| 项目 | 优先级 | 来源 | 状态 | 防护的漏洞 |
|------|--------|------|------|-----------|
| CORS 源限制 (当前允许 `*`) | 🔴 高 | ClawdBot | ⏳ 待实现 | CSRF |
| WebSocket Origin 检查 | 🔴 高 | ClawdBot | ⏳ 待实现 | WebSocket 劫持 |
| HTTPS/TLS 强制 | 🔴 高 | 自研 | ⏳ 待实现 | 中间人攻击 |
| WebSocket 网关令牌验证加强 | 🔴 高 | ClawdBot | ⏳ 待实现 | RCE |
| API 消费限制 | 🔴 高 | ClawdBot | ⏳ 待实现 | 成本攻击 |

### 中优先级 (Medium Priority)

| 项目 | 优先级 | 来源 | 状态 | 防护的漏洞 |
|------|--------|------|------|-----------|
| 持久化会话存储 | 🟠 中 | 自研 | ⏳ 待实现 | 会话丢失 |
| 请求签名/HMAC 验证 | 🟠 中 | 自研 | ⏳ 待实现 | 请求篡改 |
| IP 白名单/黑名单 | 🟠 中 | ClawdBot | ⏳ 待实现 | 未授权访问 |
| 安全响应头 (CSP, X-Frame-Options) | 🟠 中 | 自研 | ⏳ 待实现 | XSS, 点击劫持 |
| 请求日志和审计追踪 | 🟠 中 | 自研 | ⏳ 待实现 | 取证分析 |

### 低优先级 (Low Priority)

| 项目 | 优先级 | 来源 | 状态 | 防护的漏洞 |
|------|--------|------|------|-----------|
| DDoS 防护机制 | 🟡 低 | 自研 | ⏳ 待实现 | 服务中断 |
| 外部 API 调用证书固定 | 🟡 低 | 自研 | ⏳ 待实现 | 中间人攻击 |
| 安全事件 Webhook 通知 | 🟡 低 | 自研 | ⏳ 待实现 | 响应延迟 |
| Docker 沙箱默认启用 | 🟡 低 | ClawdBot | ⏳ 待实现 | 容器逃逸 |

---

## 安全统计

### 防护覆盖率

| 类别 | ClawdBot 漏洞数 | 已防护 | 需审查 | 覆盖率 |
|------|----------------|--------|--------|--------|
| 严重 (Critical) | 5 | 4 | 1 | 80% |
| 高危 (High) | 4 | 1 | 3 | 25% |
| 中危 (Medium) | 3 | 1 | 2 | 33% |
| **总计** | **12** | **6** | **6** | **50%** |

### 已实现的安全控制

| 类别 | 数量 |
|------|------|
| 认证系统 | 7 |
| 密码安全 | 4 |
| 多因素认证 | 4 |
| 速率限制 | 5 |
| 沙箱隔离 | 6 |
| 威胁检测 | 7 |
| 命令安全 | 3 |
| WebSocket 安全 | 4 |
| RBAC 权限控制 | 4 |
| 会话管理 | 3 |
| 加密系统 | 4 |
| 外部认证 | 3 |
| **总计** | **54** |

---

## 安全配置建议

### 生产环境必须配置

```yaml
# 1. 强制 HTTPS
server:
  tls:
    enabled: true
    cert_file: /path/to/cert.pem
    key_file: /path/to/key.pem

# 2. 限制 CORS 源
cors:
  allowed_origins:
    - "https://your-domain.com"

# 3. 启用 API Key 加密
security:
  api_key_encryption:
    enabled: true
    key_file: /path/to/encryption.key

# 4. 设置 API 消费限制
limits:
  max_monthly_cost: 100  # USD
  max_requests_per_day: 10000

# 5. 启用沙箱
sandbox:
  enabled: true
  docker: true
```

### 网络安全

```bash
# 不要暴露以下端口到公网
# - 80 (API 服务器) - 应通过反向代理访问
# - WebSocket 端口 - 应通过认证网关访问

# 使用 Tailscale 或 VPN 进行内部访问
tailscale up --accept-routes
```

---

## 参考资料

### ClawdBot/OpenClaw 安全事件

- [Over 1,000 AI Agent Servers Exposed](https://beyondmachines.net/event_details/clawdbot-security-issues-over-1000-ai-agent-servers-exposed-to-unauthenticated-access-6-y-a-t-e)
- [AI Hacks AI: RCE in OpenClaw](https://www.cyberkendra.com/2026/01/openclaw-hacked-by-ai.html)
- [OpenClaw Complete Guide 2026](https://www.nxcode.io/resources/news/openclaw-complete-guide-2026)
- [Your Lobster Is Leaking](https://paddo.dev/blog/your-lobster-is-leaking/)
- [Personal AI Agents Security Nightmare - Cisco](https://blogs.cisco.com/ai/personal-ai-agents-like-openclaw-are-a-security-nightmare)
- [Moltbot Security Alert - Bitdefender](https://www.bitdefender.com/en-us/blog/hotforsecurity/moltbot-security-alert-exposed-clawdbot-control-panels-risk-credential-leaks-and-account-takeovers)

### 安全标准

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

---

## 更新日志

| 日期 | 更新内容 | 作者 |
|------|----------|------|
| 2026-02-01 | 初始文档创建，整合 ClawdBot 安全更新 | Claude |
| 2026-02-01 | 添加已实现的安全加固项清单 | Claude |
| 2026-02-01 | 添加待实现的安全加固项 | Claude |
| 2026-02-01 | 添加 ClawdBot 漏洞对照表和防护状态 | Claude |
| 2026-02-01 | 添加安全统计和覆盖率分析 | Claude |
