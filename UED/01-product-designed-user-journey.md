# ZimaOS Blue 产品现有用户旅程与信息架构

**版本**: 1.0
**创建日期**: 2026-01-29
**性质**: 现状描述（基于PRD、路由结构、组件架构）

---

## 1. 信息架构

### 1.1 导航结构

```
ZimaOS-Blue
├── 首页 (/)                          - 产品介绍与快速开始
├── 设置向��� (/setup)                 - 初始化配置流程
├── 登录 (/login)                     - 用户认证
│
├── 主要功能区
│   ├── 聊天 (/chat)                  - 核心AI对话界面
│   ├── 语音聊天 (/voice)             - 语音交互
│   ├── 浏览器自动化 (/browser-automation) - 自动化操作
│   └── 自动回复 (/auto-reply)        - 消息自动回复规则
│
├── 渠道集成
│   └── 渠道管理 (/channels)          - WhatsApp/Telegram/Discord等
│
├── 自动化
│   ├── 工作流 (/workflows)           - 可视化工作流编辑
│   ├── Webhook (/webhooks)           - Webhook配置
│   ├── 定时任务 (/cron)              - Cron任务管理
│   └── 自动化中心 (/automation)      - 统一自动化入口
│
├── 扩展生态
│   ├── 插件 (/plugins)              - 插件市场与管理
│   └── 工具 (/tools)                - 工具商店
│
├── 配置与设置
│   ├── 设置 (/settings)             - 系统设置
│   ├── 个人资料 (/profile)          - 用户配置
│   └── 认证提供商 (/admin/auth-providers) - OIDC等认证配置
│
├── 监控与运维
│   ├── 系统监控 (/system)           - 系统状态与资源
│   ├── 审计日志 (/audit)            - 操作审计
│   └── 备份 (/backup)               - 备份管理
│
└── 高级功能
    ├── 安全 (/security)             - 安全设置
    ├── 沙箱 (/sandbox)              - 沙箱配置
    ├── A2UI (/a2ui)                 - Agent-to-UI界面
    ├── 多租户 (/tenants)            - 租户管理
    └── 表单填充器 (/form-filler)    - 智能表单填充（浮动组件）
```

### 1.2 路由定义

```typescript
// 来自 web/src/router/index.ts

public routes (无需认证):
  /           → HomeView
  /setup      → SetupWizardView
  /login      → LoginView
  /auth/callback/:provider → AuthCallbackView

authenticated routes:
  /chat                    → ChatView
  /voice                   → VoiceChatView
  /browser-automation      → BrowserAutomationView
  /auto-reply              → AutoReplyView
  /channels                → ChannelsView
  /workflows               → WorkflowView
  /webhooks                → WebhookView
  /cron                    → CronView
  /automation              → AutomationView
  /plugins                 → PluginsView
  /tools                   → ToolStoreView
  /settings                → SettingsView
  /profile                 → ProfileView
  /system                  → SystemView
  /security                → SecurityView
  /sandbox                 → SandboxView
  /a2ui                    → A2UIView
  /tenants                 → TenantsView
  /tenants/:id             → TenantDetailView
  /form-filler             → FormFillerView

admin only routes:
  /admin/auth-providers    → AuthProvidersView
  /audit                   → AuditView
  /backup                  → BackupView
```

### 1.3 路由守卫逻辑

```
用户访问                    系统检测                    路由行为
─────────────────────────────────────────────────────────────────────────
http://localhost:3000       → 检查 /api/setup/status   → completed?
                                                      │
                           ┌──────────────────────────┘
                           │
                   未完成?           已完成?
                      │                │
                      ▼                ▼
                   跳转 /setup      检查 localStorage token
                                    │
                            有 token?    无 token?
                                │           │
                                ▼           ▼
                            进入首页     跳转 /login
```

**路由守卫实现** (router/index.ts):
```typescript
1. 检查 setup 状态 → 未完成则跳转 /setup
2. 检查认证状态 → 未登录且需要认证则跳转 /login
3. 已登录访问 /login → 跳转 /chat
```

---

## 2. 核心功能模块

### 2.1 模块与路由映射

| 模块 | 路由入口 | PRD引用 | 组件位置 |
|------|----------|---------|----------|
| **Provider Pool** | /settings | v0.9-provider-pool.md | ProviderSettings.vue |
| **Echo Companion** | 独立页面？ | v0.9.1-companion.md | components/companion/ |
| **Form Filler** | 全局浮动 | v0.9.2-form-filler.md | FormFillerWidget.vue |
| **Voice** | /voice | - | VoiceChatView.vue |
| **Workflow** | /workflows | - | WorkflowView.vue |
| **Channel** | /channels | - | ChannelsView.vue |
| **Cron** | /cron | - | CronView.vue |
| **Browser Automation** | /browser-automation | - | BrowserAutomationView.vue |
| **Auto Reply** | /auto-reply | - | AutoReplyView.vue |
| **Security** | /security | - | SecurityView.vue |
| **Sandbox** | /sandbox | - | SandboxView.vue |
| **A2UI** | /a2ui | - | A2UIView.vue |
| **Plugins** | /plugins | - | PluginsView.vue |
| **Tools** | /tools | - | ToolStoreView.vue |

### 2.2 Provider Pool 支持的提供商

**提供商类型** (来自 v0.9-provider-pool.md):

| 类型 | 提供商 |
|------|--------|
| Builtin | OpenAI, Anthropic, Google Gemini, DeepSeek, Moonshot, Azure OpenAI |
| Custom | OpenAI-compatible endpoints |
| ACP | Agent Communication Protocol providers |
| IDE | Antigravity, Cursor, Windsurf (本地发现) |

---

## 3. 用户旅程流程

### 3.1 首次访问流程

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
访问 http://localhost:3000    → 检测安装状态
                            → 未完成安装，跳转 /setup
─────────────────────────────────────────────────────────────────────────
进入设置向导                  → 显示欢迎页面
                            → 引导创建管理员账户
                            → 选择LLM提供商
                            → 配置首个渠道
                            → 完成设置
─────────────────────────────────────────────────────────────────────────
```

### 3.2 模型接入流程

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
进入 /settings 或 Providers   → 显示提供商列表
                            → 大多显示"未配置"状态
                            → 点击"添加自定义"显示表单
                            → 输入API Key和Base URL
                            → 测试连接
                            → 拉取模型列表
                            → 启用/禁用模型
─────────────────────────────────────────────────────────────────────────
```

### 3.3 渠道接入流程

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
进入 /channels               → 显示支持的平台列表
                            → 点击连接显示配置步骤
                            → 扫描二维码/输入Token
                            → 授权完成
─────────────────────────────────────────────────────────────────────────
```

### 3.4 对话交互流程

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
进入 /chat                    → 显示聊天界面
                            → 输入消息
                            → Agent处理
                            → 返回回复
─────────────────────────────────────────────────────────────────────────
发送图片                      → 接收图片（模型依赖）
                            → 识别并处理
──────────────────────���──────────────────────────────────────────────────
尝试语音                      → 需要前往 /voice 页面
─────────────────────────────────────────────────────────────────────────
```

### 3.5 配置错误场景

```
场景A: API Key无效
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
输入错误的API Key            → "测试连接失败"
                            → 错误码: 401
                            → "请检查您的API Key"
─────────────────────────────────────────────────────────────────────────

场景B: 渠道连接失败
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
Telegram Token无效          → "连接失败"
                            → "请检查Token格式"
─────────────────────────────────────────────────────────────────────────

场景C: 模型响应超时
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
等待响应                     → 加载状态
                            → 超时错误
─────────────────────────────────────────────────────────────────────────
```

---

## 4. 布局与组件结构

### 4.1 默认布局

```vue
<!-- DefaultLayout.vue -->
<div v-if="hideLayout>         <!-- setup/login 等页面 -->
  <RouterView />
  <FormFillerWidget />
</div>

<div v-else>                    <!-- 正常应用布局 -->
  <AppHeader />
  <div class="flex">
    <AppSidebar />
    <main :class="{ 'p-4 sm:p-6': !noPadding }">
      <RouterView />
    </main>
  </div>
  <FormFillerWidget />
</div>
```

**布局控制**:
- `hideLayout`: meta.hideLayout = true (setup, login, auth-callback)
- `noPadding`: meta.noPadding = true (chat页面)

### 4.2 聊天界面结构

```
AppHeader
    ├── Sidebar toggle
    └── User menu
AppSidebar
    └── Navigation menu
ChatView
    ├── ConversationList
    ├── ChatMessage (消息列表)
    ├── ChatInput (输入框)
    └── FormFillerWidget (浮动组件)
```

### 4.3 Form Filler

- 全局浮动组件，在所有页面可用
- 输入框聚焦时激活
- 组件位置: `components/formfiller/FormFillerWidget.vue`

### 4.4 视图组件清单

```
views/
├── HomeView.vue
├── SetupWizardView.vue
├── LoginView.vue
├── AuthCallbackView.vue
├── ChatView.vue
├── VoiceChatView.vue
├── SettingsView.vue
├── PluginsView.vue
├── ProfileView.vue
├── SystemView.vue
├── AuthProvidersView.vue (admin)
├── HomeAssistantView.vue
├── BrowserAutomationView.vue
├── AutoReplyView.vue
├── ChannelsView.vue
├── WorkflowView.vue
├── WebhookView.vue
├── CronView.vue
├── AutomationView.vue
├── AuditView.vue (admin)
├── BackupView.vue (admin)
├── ToolStoreView.vue
├── SecurityView.vue
├── SandboxView.vue
├── A2UIView.vue
├── TenantsView.vue
├── TenantDetailView.vue
├── FormFillerView.vue
└── NotFoundView.vue
```

---

## 5. 认证与权限

### 5.1 认证流程

```
/login                      → 输入凭证
    ↓
/auth/callback/:provider    → OAuth回调
    ↓
存储 token (localStorage)
    ↓
跳转 redirect 或 /chat
```

### 5.2 权限级别

| 级别 | 路由示例 | 要求 |
|------|----------|------|
| Public | /, /setup, /login | 无 |
| Authenticated | /chat, /settings, etc. | 有效token |
| Admin | /admin/auth-providers, /audit, /backup | Admin角色 |

### 5.3 路由元信息

```typescript
meta: {
  public: true,         // 无需认证
  hideLayout: true,     // 隐藏导航布局
  requiresAuth: true,   // 需要认证
  requiresAdmin: true,  // 需要管理员
  noPadding: true       // 无内边距
}
```

---

## 6. PRD功能映射

| PRD文档 | 核心功能 | 路由入口 | 组件 |
|---------|----------|----------|------|
| v0.9-provider-pool.md | LLM提供商管理 | /settings | ProviderSettings.vue |
| v0.9.1-companion.md | Agent监控与审计 | (待确认) | components/companion/ |
| v0.9.2-form-filler.md | 表单填充浮动组件 | 全局 | FormFillerWidget.vue |
| v0.9-ota-update.md | OTA更新系统 | /system | (待确认) |
| v0.10.0-claude-code-bundling.md | Claude Code CLI集成 | /settings | ClaudeCodeSettings.vue |
| v0.10-cli-reliability.md | CLI调用可靠性 | 后端 | - |
| v0.10.1-metrics-monitoring.md | 用量统计与监控 | /system | components/metrics/ |
| v0.10.3-cc-cli-integration.md | CC CLI集成 | /settings | ClaudeCodeSettings.vue |
| v0.10.4-tauri-packaging.md | Tauri打包 | 构建配置 | - |
| v1.0.0-rag.md | RAG知识库 | (待实现) | - |
| v1.1.0-mesh-network.md | P2P网络 | (待实现) | - |

---

## 7. API端点

### 7.1 系统相关

| 端点 | 方法 | 用途 |
|------|------|------|
| /api/setup/status | GET | 检查安装状态 |
| /api/health | GET | 健康检查 |

### 7.2 Provider相关

| 端点 | 方法 | 用途 |
|------|------|------|
| /api/v1/providers | GET | 列出所有提供商 |
| /api/v1/providers | POST | 添加自定义提供商 |
| /api/v1/providers/:id | GET | 获取提供商详情 |
| /api/v1/providers/:id | PUT | 更新提供商配置 |
| /api/v1/providers/:id | DELETE | 删除提供商 |
| /api/v1/providers/:id/models | GET | 列出提供商模型 |
| /api/v1/providers/:id/models/fetch | POST | 从提供商拉取模型 |
| /api/v1/providers/:id/test | POST | 测试提供商连接 |
| /api/v1/providers/usage | GET | 获取使用统计 |
| /api/v1/models | GET | 列出所有可用模型 |

### 7.3 Companion相关

| 端点 | 方法 | 用途 |
|------|------|------|
| ws://.../api/v1/companion/stream | WS | 实时事件流 |
| ws://.../api/v1/companion/session/:id | WS | 单会话事件流 |
| /api/v1/companion/sessions | GET | 获取会话列表 |
| /api/v1/companion/sessions/:id | GET | 获取会话详情 |
| /api/v1/companion/sessions/:id/events | GET | 获取会话事件 |
| /api/v1/companion/sessions/:id/flow | GET | 获取操作链数据 |
| /api/v1/companion/alerts | GET | 获取告警列表 |
| /api/v1/companion/alerts/:id/ack | PUT | 确认告警 |
| /api/v1/companion/stats | GET | 获取统计 |
| /api/v1/companion/export | GET | 导出数据 |

### 7.4 Form Filler相关

| 端点 | 方法 | 用途 |
|------|------|------|
| /api/v1/formfiller/templates | GET | 列出所有填充模板 |
| /api/v1/formfiller/templates | POST | 创建新模板 |
| /api/v1/formfiller/templates/:id | PUT | 更新模板 |
| /api/v1/formfiller/templates/:id | DELETE | 删除模板 |
| /api/v1/formfiller/patterns | GET | 获取字段模式 |
| /api/v1/formfiller/patterns | PUT | 更新字段模式 |
| /api/v1/formfiller/sites/:domain | GET | 获取站点映射 |
| /api/v1/formfiller/detect | POST | 检测HTML中的字段 |

---

## 8. 数据存储结构

### 8.1 Companion 数据存储

```
data/companion/
├── sessions/
│   └── 2026-01-28/
│       └── session-*.jsonl      # 每行一个JSON对象
├── alerts/
│   └── 2026-01-28.jsonl
└── stats/
    └── daily-stats.json
```

### 8.2 Provider 数据存储

```
data/providers/
├── providers.json           # Provider配置
├── models/
│   ├── openai.json
│   └── ...
├── usage/
│   └── 2026-01-28.jsonl
└── health/
    └── status.json
```

### 8.3 Form Filler 数据存储

```
data/formfiller/
├── templates.json
├── patterns.json
├── sites/
│   └── example.com.json
└── corrections.jsonl
```

---

## 9. 配置文件

### 9.1 服务器配置

```yaml
# server/config.yaml
server:
  host: "0.0.0.0"
  port: 80

log:
  level: "debug"
  format: "console"

claude_code_cli:
  enabled: true

companion:
  enabled: true
```

### 9.2 Provider配置

```yaml
provider_pool:
  enabled: true
  routing_strategy: priority
  health_check:
    enabled: true
    interval: 60s
  ide_discovery:
    enabled: true
```

---

## 10. 组件目录结构

```
web/src/
├── components/
│   ├── AppHeader.vue
│   ├── AppSidebar.vue
│   ├── ChatInput.vue
│   ├── ChatMessage.vue
│   ├── ConversationList.vue
│   ├── formfiller/
│   │   └── FormFillerWidget.vue
│   ├── companion/
│   │   ├── SessionList.vue
│   │   ├── FlowViewer.vue
│   │   └── ...
│   ├── metrics/
│   │   ├── MetricsOverview.vue
│   │   ├── TokenUsageChart.vue
│   │   └── ...
│   └── a2ui/
│       └── ...
├── views/
│   └── (26个视图组件)
├── layouts/
│   └── DefaultLayout.vue
└── router/
    └── index.ts
```

---

## 11. 技术栈

### 11.1 前端

| 技术 | 版本/说明 |
|------|-----------|
| 框架 | Vue 3 |
| 语言 | TypeScript |
| 构建 | Vite |
| 样式 | TailwindCSS |
| 路由 | Vue Router |

### 11.2 后端

| 技术 | 版本/说明 |
|------|-----------|
| 语言 | Go 1.24+ |
| 框架 | Echo |
| 数据库 | SQLite (modernc.org/sqlite) |
| 日志 | zap |

---

## 12. 版本信息

| 项目 | 版本 |
|------|------|
| 产品版本 | 0.10.2 |
| Go版本 | 1.24+ |
| 构建时间 | unknown |
| Git提交 | unknown |
