# ZimaOS Blue 预览模式 Onboarding 设计

**版本**: 1.0
**创建日期**: 2026-01-31
**性质**: 产品设计文档

---

## 1. 设计目标

### 1.1 核心理念

**"零门槛体验，一键转化"**

- 用户无需任何配置即可立即体验产品核心功能
- 降低首次使用门槛，提高产品转化率
- 保持安全性，预览模式有明确的使用限制

### 1.2 关键变更

| 现有流程 | 新流程 |
|----------|--------|
| 强制设置向导 | 直接进入预览模式 |
| 必须创建管理员 | 可选，预览模式无需账户 |
| 需要配置 Provider | 自动提供体验 Key |
| 多步骤引导 | 零配置即用 |

---

## 2. 用户角色定义

### 2.1 访客 (Guest / Preview Mode)

```
权限级别: 最低
认证状态: 未认证
使用限制:
  - Token 上限: 10KB
  - 对话次数: 5 次
  - 功能范围: 仅核心聊天功能
  - 数据持久化: 会话级别，刷新后丢失
```

### 2.2 普通用户 (User)

```
权限级别: 标准
认证状态: 已认证
使用限制:
  - 无 Token 限制（取决于配置的 Provider）
  - 完整功能访问
  - 数据持久化: 完整保存
```

### 2.3 管理员 (Admin)

```
权限级别: 最高
认证状态: 已认证
特殊权限:
  - 创建/管理用户
  - 系统配置
  - Provider 管理
  - 审计日志访问
```

---

## 3. 预览模式详细设计

### 3.1 体验 Key 机制

#### 3.1.1 ZimaOS 体验 Provider

```typescript
interface PreviewProvider {
  id: 'zimaos-preview'
  name: 'ZimaOS Preview'
  type: 'builtin'
  location: 'cloud'
  status: 'active'

  // 体验限制
  limits: {
    maxTokens: 10240        // 10KB token 上限
    maxConversations: 5     // 最多 5 次对话
    expiresAfter: '24h'     // 24小时后过期（可选）
  }

  // 使用统计
  usage: {
    tokensUsed: number
    conversationsUsed: number
  }

  // Logo 显示
  logo: '/assets/providers/zimaos.svg'
  description: 'ZimaOS 体验账户 - 免费试用'
}
```

#### 3.1.2 限制触发逻辑

```
用户发送消息
    ↓
检查预览模式状态
    ↓
├── tokensUsed >= 10KB?
│       ↓ Yes
│   显示限制提示，引导创建账户
│
├── conversationsUsed >= 5?
│       ↓ Yes
│   显示限制提示，引导创建账户
│
└── 正常处理请求
```

#### 3.1.3 限制达到后的处理

```typescript
// 当达到限制时
interface PreviewLimitReached {
  type: 'token_limit' | 'conversation_limit'

  // 自动行为
  actions: {
    // 1. 从 Provider 列表移除体验 Key
    removePreviewProvider: true

    // 2. 显示友好提示
    showUpgradePrompt: true

    // 3. 引导创建账户
    redirectToSetup: false  // 不强制跳转，用户可继续浏览
  }
}
```

### 3.2 UI 变更

#### 3.2.1 右上角入口（预览模式）

```
┌─────────────────────────────────────────────────────────┐
│  [Logo] ZimaOS Blue                    [结束预览模式 ▼] │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │                                                 │   │
│  │              聊天界面                           │   │
│  │                                                 │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘

点击 [结束预览模式] 下拉菜单:
┌──────────────────────────┐
│ 🎉 创建管理员账户        │  ← 主要操作
│ ─────────────────────── │
│ 📊 查看使用情况          │
│ ❓ 了解更多              │
└──────────────────────────┘
```

#### 3.2.2 预览状态指示器

```
┌─────────────────────────────────────────────────────────┐
│  Provider 选择器                                        │
│  ┌─────────────────────────────────────────────────┐   │
│  │ [ZimaOS Logo] ZimaOS Preview                    │   │
│  │ 体验模式 · 剩余 3/5 次对话 · 8.2KB/10KB        │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

#### 3.2.3 限制提示 UI

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │  🎊 体验额度已用完                              │   │
│  │                                                 │   │
│  │  您已使用完预览模式的免费额度。                 │   │
│  │  创建账户后可以：                               │   │
│  │  ✓ 无限制使用                                   │   │
│  │  ✓ 配置自己的 AI Provider                       │   │
│  │  ✓ 保存对话历史                                 │   │
│  │                                                 │   │
│  │  [创建管理员账户]  [稍后再说]                   │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 4. 用户旅程流程

### 4.1 首次访问（预览模式）

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
访问 http://localhost:3000    → 检测系统状态
                            → 无管理员账户 = 预览模式
                            → 自动注入 ZimaOS Preview Provider
                            → 直接进入 /chat
─────────────────────────────────────────────────────────────────────────
开始对话                      → 使用 ZimaOS Preview Provider
                            → 实时显示使用进度
                            → 正常响应
─────────────────────────────────────────────────────────────────────────
达到限制                      → 显示友好提示
                            → 移除 Preview Provider
                            → 引导创建账户（非强制）
─────────────────────────────────────────────────────────────────────────
```

### 4.2 预览模式 → 正式模式转化

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
点击 [结束预览模式]           → 显示下拉菜单
─────────────────────────────────────────────────────────────────────────
选择 [创建管理员账户]         → 显示简化的账户创建表单
                            │
                            │  ┌─────────────────────────┐
                            │  │ 创建管理员账户          │
                            │  │                         │
                            │  │ 用户名: [____________]  │
                            │  │ 密码:   [____________]  │
                            │  │ 确认:   [____________]  │
                            │  │                         │
                            │  │ [创建账户]              │
                            │  └─────────────────────────┘
─────────────────────────────────────────────────────────────────────────
提交表单                      → 创建管理员账户
                            → 自动登录
                            → 移除 Preview Provider（如果还存在）
                            → 右上角变为用户菜单
                            → 引导配置自己的 Provider
─────────────────────────────────────────────────────────────────────────
```

### 4.3 管理员创建新用户

```
用户行为                     系统响应
─────────────────────────────────────────────────────────────────────────
管理员进入 /settings          → 显示用户管理选项
─────────────────────────────────────────────────────────────────────────
点击 [添加用户]               → 显示用户创建表单
                            │
                            │  ┌─────────────────────────┐
                            │  │ 创建新用户              │
                            │  │                         │
                            │  │ 用户名: [____________]  │
                            │  │ 邮箱:   [____________]  │
                            │  │ 密码:   [____________]  │
                            │  │ 角色:   [普通用户 ▼]   │
                            │  │                         │
                            │  │ [创建]                  │
                            │  └─────────────────────────┘
─────────────────────────────────────────────────────────────────────────
```

---

## 5. 路由与认证逻辑变更

### 5.1 新路由守卫逻辑

```
用户访问                    系统检测                    路由行为
─────────────────────────────────────────────────────────────────────────
http://localhost:3000       → 检查是否有管理员账户
                                    │
                           ┌────────┴────────┐
                           │                 │
                      无管理员           有管理员
                           │                 │
                           ▼                 ▼
                      预览模式          检查 token
                      进入 /chat              │
                      注入 Preview       ┌────┴────┐
                      Provider           │         │
                                    有 token   无 token
                                        │         │
                                        ▼         ▼
                                    进入 /chat  跳转 /login
```

### 5.2 路由元信息更新

```typescript
// 新增路由元信息
meta: {
  public: true,           // 无需认证
  previewAllowed: true,   // 预览模式可访问
  hideLayout: true,       // 隐藏导航布局
  requiresAuth: true,     // 需要认证
  requiresAdmin: true,    // 需要管理员
  noPadding: true         // 无内边距
}

// 预览模式可访问的路由
const previewAllowedRoutes = [
  '/chat',        // 核心聊天功能
  '/settings',    // 查看设置（只读部分）
]

// 预览模式不可访问的路由
const previewBlockedRoutes = [
  '/admin/*',     // 管理功能
  '/audit',       // 审计日志
  '/backup',      // 备份管理
  '/tenants',     // 多租户
  // ... 其他高级功能
]
```

### 5.3 认证中间件更新

```go
// 后端中间件逻辑
func AuthMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // 1. 检查是否为预览模式
            if isPreviewMode(c) {
                // 设置预览模式上下文
                c.Set("preview_mode", true)
                c.Set("user_role", "guest")
                return next(c)
            }

            // 2. 正常认证流程
            token := extractToken(c)
            if token == "" {
                return echo.ErrUnauthorized
            }

            // ... 验证 token
            return next(c)
        }
    }
}

func isPreviewMode(c echo.Context) bool {
    // 检查是否存在管理员账户
    adminExists := userService.AdminExists()
    return !adminExists
}
```

---

## 6. API 变更

### 6.1 新增 API 端点

| 端点 | 方法 | 用途 |
|------|------|------|
| /api/v1/system/mode | GET | 获取系统模式（preview/normal） |
| /api/v1/preview/status | GET | 获取预览模式状态和使用情况 |
| /api/v1/preview/upgrade | POST | 从预览模式升级（创建管理员） |

### 6.2 API 响应示例

```json
// GET /api/v1/system/mode
{
  "mode": "preview",
  "features": {
    "chat": true,
    "settings_readonly": true,
    "admin": false,
    "user_management": false
  }
}

// GET /api/v1/preview/status
{
  "active": true,
  "usage": {
    "tokens_used": 8234,
    "tokens_limit": 10240,
    "conversations_used": 3,
    "conversations_limit": 5
  },
  "provider": {
    "id": "zimaos-preview",
    "name": "ZimaOS Preview",
    "status": "active"
  }
}

// POST /api/v1/preview/upgrade
// Request:
{
  "username": "admin",
  "password": "secure_password"
}
// Response:
{
  "success": true,
  "user": {
    "id": "uuid",
    "username": "admin",
    "role": "admin"
  },
  "token": "jwt_token",
  "refresh_token": "refresh_token"
}
```

---

## 7. 数据模型变更

### 7.1 系统配置表

```sql
-- 新增系统配置
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 预览模式相关配置
INSERT INTO system_config (key, value) VALUES
    ('preview_mode_enabled', 'true'),
    ('preview_tokens_limit', '10240'),
    ('preview_conversations_limit', '5');
```

### 7.2 预览使用统计表

```sql
-- 预览模式使用统计（基于 session/IP）
CREATE TABLE IF NOT EXISTS preview_usage (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    ip_address TEXT,
    tokens_used INTEGER DEFAULT 0,
    conversations_used INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expired_at DATETIME
);
```

---

## 8. 前端组件变更

### 8.1 新增组件

```
web/src/components/
├── preview/
│   ├── PreviewBanner.vue        # 预览模式顶部提示条
│   ├── PreviewStatusBadge.vue   # 使用进度徽章
│   ├── PreviewLimitModal.vue    # 限制达到提示弹窗
│   └── PreviewUpgradeForm.vue   # 创建管理员表单
```

### 8.2 PreviewBanner.vue

```vue
<template>
  <div v-if="isPreviewMode" class="preview-banner">
    <div class="flex items-center justify-between px-4 py-2 bg-blue-50 border-b border-blue-100">
      <div class="flex items-center gap-2">
        <span class="text-blue-600">🎉 预览模式</span>
        <PreviewStatusBadge />
      </div>
      <button @click="showUpgradeModal" class="btn-primary-sm">
        结束预览模式
      </button>
    </div>
  </div>
</template>
```

### 8.3 Store 变更

```typescript
// stores/preview.ts
export const usePreviewStore = defineStore('preview', {
  state: () => ({
    isPreviewMode: false,
    usage: {
      tokensUsed: 0,
      tokensLimit: 10240,
      conversationsUsed: 0,
      conversationsLimit: 5
    },
    limitReached: false
  }),

  actions: {
    async fetchStatus() {
      const response = await api.get('/preview/status')
      this.isPreviewMode = response.data.active
      this.usage = response.data.usage
      this.checkLimit()
    },

    checkLimit() {
      this.limitReached =
        this.usage.tokensUsed >= this.usage.tokensLimit ||
        this.usage.conversationsUsed >= this.usage.conversationsLimit
    },

    async upgrade(credentials: { username: string; password: string }) {
      const response = await api.post('/preview/upgrade', credentials)
      // 升级成功后切换到正常模式
      this.isPreviewMode = false
      return response.data
    }
  }
})
```

---

## 9. 删除/简化的功能

### 9.1 移除设置向导

```diff
- web/src/views/SetupWizardView.vue        # 删除
- web/src/components/setup/               # 删除整个目录
-   ├── ProviderDetection.vue
-   ├── BasicSettings.vue
-   └── SecuritySettings.vue
```

### 9.2 路由变更

```typescript
// 移除的路由
// - /setup → 删除

// 新增的路由
// + /upgrade → PreviewUpgradeView (可选，也可用 Modal)
```

---

## 10. 安全考虑

### 10.1 预览模式限制

| 限制项 | 值 | 说明 |
|--------|-----|------|
| Token 上限 | 10KB | 防止滥用 |
| 对话次数 | 5 次 | 足够体验核心功能 |
| 功能范围 | 仅聊天 | 不暴露敏感功能 |
| 数据持久化 | 会话级 | 刷新后丢失 |
| API 访问 | 只读 | 不能修改系统配置 |

### 10.2 防滥用机制

```typescript
// 基于 IP + 浏览器指纹的限制
interface PreviewSession {
  sessionId: string      // 浏览器 session
  ipAddress: string      // IP 地址
  fingerprint: string    // 浏览器指纹（可选）

  // 防止清除 cookie 后重新获取额度
  // 使用 IP + 指纹组合判断
}
```

---

## 11. 实现优先级

### Phase 1: 核心功能（MVP）

1. ✅ 预览模式检测逻辑
2. ✅ ZimaOS Preview Provider 注入
3. ✅ 使用量统计与限制
4. ✅ 右上角"结束预览模式"入口
5. ✅ 管理员创建表单

### Phase 2: 体验优化

1. ⬜ 预览状态指示器
2. ⬜ 限制达到提示优化
3. ⬜ 使用进度可视化
4. ⬜ 空白区域预置问题引导
5. ⬜ Typeless 卡片支持

### Phase 3: 高级功能

1. ⬜ 防滥用机制
2. ⬜ 预览数据迁移（升级后保留对话）
3. ⬜ 多用户管理界面

### Phase 4: 功能受限提示

1. ⬜ 附件上传受限提示
2. ⬜ 图片上传受限提示
3. ⬜ 语音播放受限提示
4. ⬜ 语音输入受限提示

---

## 12. 测试场景

### 12.1 预览模式测试

| 场景 | 预期结果 |
|------|----------|
| 首次访问，无管理员 | 直接进入 /chat，显示预览模式 |
| 发送消息 | 正常响应，更新使用统计 |
| 达到 Token 限制 | 显示提示，移除 Preview Provider |
| 达到对话限制 | 显示提示，移除 Preview Provider |
| 刷新页面 | 保持预览模式状态 |
| 清除 cookie | 重置使用统计（或基于 IP 限制） |

### 12.2 升级流程测试

| 场景 | 预期结果 |
|------|----------|
| 点击"结束预览模式" | 显示下拉菜单 |
| 创建管理员 | 成功创建，自动登录 |
| 创建后访问 | 正常模式，显示用户菜单 |
| 管理员创建新用户 | 成功创建普通用户 |

---

## 14. 预览模式空白区域引导设计

### 14.1 预置问题卡片

当聊天区域为空时，显示随机的预置问题卡片，引导用户快速体验产品功能。

#### 14.1.1 UI 设计

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│                         [ZimaOS Blue Logo]                              │
│                                                                         │
│                    欢迎体验 ZimaOS Blue                                 │
│                    点击下方问题快速开始                                  │
│                                                                         │
│   ┌─────────────────────┐  ┌─────────────────────┐                     │
│   │ 💡 帮我写一封邮件    │  │ 📝 总结这篇文章     │                     │
│   │    给同事请假        │  │    的主要观点       │                     │
│   └─────────────────────┘  └─────────────────────┘                     │
│                                                                         │
│   ┌─────────────────────┐  ┌─────────────────────┐                     │
│   │ 🔧 解释什么是        │  │ 🎨 帮我设计一个     │                     │
│   │    Docker 容器       │  │    Logo 创意        │                     │
│   └─────────────────────┘  └─────────────────────┘                     │
│                                                                         │
│                         [🔄 换一批]                                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

#### 14.1.2 预置问题分类

```typescript
interface PresetQuestion {
  id: string
  icon: string
  title: string
  fullPrompt: string
  category: 'writing' | 'coding' | 'creative' | 'learning' | 'productivity'
}

const presetQuestions: PresetQuestion[] = [
  // 写作类
  { id: 'email-leave', icon: '💡', title: '帮我写一封邮件给同事请假', fullPrompt: '帮我写一封请假邮件...', category: 'writing' },
  { id: 'summary', icon: '📝', title: '总结这篇文章的主要观点', fullPrompt: '请帮我总结以下内容...', category: 'writing' },

  // 编程类
  { id: 'docker', icon: '🔧', title: '解释什么是 Docker 容器', fullPrompt: '请用简单的语言解释 Docker 容器...', category: 'coding' },
  { id: 'python', icon: '🐍', title: '写一个 Python 脚本', fullPrompt: '帮我写一个 Python 脚本...', category: 'coding' },

  // 创意类
  { id: 'logo', icon: '🎨', title: '帮我设计一个 Logo 创意', fullPrompt: '帮我构思一个 Logo 设计...', category: 'creative' },
  { id: 'story', icon: '📖', title: '写一个短故事', fullPrompt: '帮我写一个有趣的短故事...', category: 'creative' },

  // 学习类
  { id: 'explain', icon: '🎓', title: '用简单的话解释量子计算', fullPrompt: '请用通俗易懂的语言解释量子计算...', category: 'learning' },
  { id: 'compare', icon: '⚖️', title: '比较 React 和 Vue', fullPrompt: '请比较 React 和 Vue 框架...', category: 'learning' },

  // 效率类
  { id: 'schedule', icon: '📅', title: '帮我规划今天的工作', fullPrompt: '帮我规划一下今天的工作安排...', category: 'productivity' },
  { id: 'translate', icon: '🌐', title: '翻译这段文字', fullPrompt: '请帮我翻译以下内容...', category: 'productivity' },
]
```

#### 14.1.3 随机展示逻辑

```typescript
// 每次显示 4 个问题，从不同分类中随机选取
function getRandomQuestions(count: number = 4): PresetQuestion[] {
  const categories = ['writing', 'coding', 'creative', 'learning', 'productivity']
  const selected: PresetQuestion[] = []

  // 确保每个分类至少有一个（如果 count >= 分类数）
  const shuffledCategories = shuffle(categories)

  for (let i = 0; i < Math.min(count, categories.length); i++) {
    const categoryQuestions = presetQuestions.filter(q => q.category === shuffledCategories[i])
    if (categoryQuestions.length > 0) {
      selected.push(categoryQuestions[Math.floor(Math.random() * categoryQuestions.length)])
    }
  }

  // 如果还需要更多，随机补充
  while (selected.length < count) {
    const remaining = presetQuestions.filter(q => !selected.includes(q))
    if (remaining.length === 0) break
    selected.push(remaining[Math.floor(Math.random() * remaining.length)])
  }

  return selected
}
```

#### 14.1.4 交互行为

```typescript
// 点击预置问题
function onPresetQuestionClick(question: PresetQuestion) {
  // 1. 将问题填入输入框
  inputMessage.value = question.fullPrompt

  // 2. 可选：直接发送
  // sendMessage()

  // 3. 聚焦输入框，让用户可以编辑
  inputRef.value?.focus()
}

// 换一批
function refreshQuestions() {
  displayedQuestions.value = getRandomQuestions(4)
}
```

---

## 15. 功能受限提示设计

### 15.1 受限功能列表

预览模式下，以下功能受限并需要友好提示：

| 功能 | 受限原因 | 提示方式 |
|------|----------|----------|
| 附件上传 | 需要存储空间和处理能力 | 点击时弹出提示 |
| 图片上传 | 需要图像处理能力 | 点击时弹出提示 |
| 语音播放 | 需要 TTS 服务 | 点击时弹出提示 |
| 语音输入 | 需要 STT 服务 | 点击时弹出提示 |

### 15.2 统一提示组件

```vue
<!-- PreviewFeatureBlockedToast.vue -->
<template>
  <div class="feature-blocked-toast">
    <div class="toast-content">
      <span class="icon">🔒</span>
      <div class="text">
        <p class="title">{{ title }}</p>
        <p class="description">{{ description }}</p>
      </div>
    </div>
    <button @click="showUpgrade" class="upgrade-btn">
      创建账户解锁
    </button>
  </div>
</template>

<script setup lang="ts">
interface Props {
  feature: 'attachment' | 'image' | 'voice-play' | 'voice-input'
}

const featureMessages = {
  'attachment': {
    title: '附件上传需要创建账户',
    description: '创建账户后可上传文档、代码等文件'
  },
  'image': {
    title: '图片上传需要创建账户',
    description: '创建账户后可上传图片进行分析'
  },
  'voice-play': {
    title: '语音播放需要创建账户',
    description: '创建账户后可使用语音朗读功能'
  },
  'voice-input': {
    title: '语音输入需要创建账户',
    description: '创建账户后可使用语音输入功能'
  }
}
</script>
```

### 15.3 UI 交互设计

#### 15.3.1 按钮状态

```
正常模式:                          预览模式:
┌─────────────────────────┐       ┌─────────────────────────┐
│ [📎] [🖼️] [🎤]  输入... │       │ [📎🔒] [🖼️🔒] [🎤🔒]  输入... │
└─────────────────────────┘       └─────────────────────────┘
                                         ↑
                                   带锁标记，颜色变淡
```

#### 15.3.2 点击受限按钮时的提示

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ 🔒 图片上传需要创建账户                              │   │
│  │                                                     │   │
│  │ 创建账户后可上传图片进行分析                         │   │
│  │                                                     │   │
│  │ [创建账户解锁]                    [稍后再说]        │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 15.4 实现逻辑

```typescript
// 检查功能是否可用
function isFeatureAvailable(feature: string): boolean {
  const previewStore = usePreviewStore()

  if (!previewStore.isPreviewMode) {
    return true // 正常模式，所有功能可用
  }

  // 预览模式下受限的功能
  const restrictedFeatures = ['attachment', 'image', 'voice-play', 'voice-input']
  return !restrictedFeatures.includes(feature)
}

// 处理受限功能点击
function handleRestrictedFeatureClick(feature: string) {
  if (isFeatureAvailable(feature)) {
    // 正常执行功能
    return true
  }

  // 显示受限提示
  showFeatureBlockedToast(feature)
  return false
}
```

---

## 16. Typeless 卡片支持

### 16.1 概述

Typeless 卡片是一种在聊天过程中动态展示的交互式卡片，用于展示结构化信息、操作按钮等。

### 16.2 卡片类型定义

```typescript
interface TypelessCard {
  id: string
  type: 'info' | 'action' | 'form' | 'progress' | 'result'
  title?: string
  content: TypelessCardContent
  actions?: TypelessCardAction[]
  metadata?: Record<string, any>
}

interface TypelessCardContent {
  // 文本内容
  text?: string
  // 列表内容
  list?: string[]
  // 键值对内容
  keyValues?: { key: string; value: string }[]
  // 进度内容
  progress?: { current: number; total: number; label?: string }
  // 代码内容
  code?: { language: string; content: string }
  // 图表内容
  chart?: { type: 'bar' | 'line' | 'pie'; data: any }
}

interface TypelessCardAction {
  id: string
  label: string
  type: 'primary' | 'secondary' | 'danger'
  action: string // 动作标识符
  payload?: any
}
```

### 16.3 卡片渲染示例

#### 16.3.1 信息卡片

```
┌─────────────────────────────────────────────────────────────┐
│  📊 系统状态                                                │
│  ─────────────────────────────────────────────────────────  │
│  CPU 使用率:     45%                                        │
│  内存使用:       2.3GB / 8GB                                │
│  磁盘空间:       120GB / 500GB                              │
│  运行时间:       3 天 12 小时                               │
└─────────────────────────────────────────────────────────────┘
```

#### 16.3.2 操作卡片

```
┌─────────────────────────────────────────────────────────────┐
│  🔧 检测到可用更新                                          │
│  ─────────────────────────────────────────────────────────  │
│  ZimaOS Blue v0.10.6 已发布                                 │
│                                                             │
│  更新内容:                                                  │
│  • 新增 Provider Pool 功能                                  │
│  • 优化聊天响应速度                                         │
│  • 修复若干已知问题                                         │
│                                                             │
│  [立即更新]  [稍后提醒]  [查看详情]                         │
└─────────────────────────────────────────────────────────────┘
```

#### 16.3.3 进度卡片

```
┌─────────────────────────────────────────────────────────────┐
│  ⏳ 正在处理您的请求                                        │
│  ─────────────────────────────────────────────────────────  │
│  [████████████░░░░░░░░] 60%                                 │
│                                                             │
│  正在分析文档内容...                                        │
└─────────────────────────────────────────────────────────────┘
```

### 16.4 卡片组件实现

{% raw %}
```vue
<!-- TypelessCard.vue -->
<template>
  <div class="typeless-card" :class="[`card-${card.type}`]">
    <!-- 标题 -->
    <div v-if="card.title" class="card-header">
      <span class="card-icon">{{ getIcon(card.type) }}</span>
      <span class="card-title">{{ card.title }}</span>
    </div>

    <!-- 内容区域 -->
    <div class="card-content">
      <!-- 文本内容 -->
      <p v-if="card.content.text" class="content-text">
        {{ card.content.text }}
      </p>

      <!-- 列表内容 -->
      <ul v-if="card.content.list" class="content-list">
        <li v-for="item in card.content.list" :key="item">{{ item }}</li>
      </ul>

      <!-- 键值对内容 -->
      <div v-if="card.content.keyValues" class="content-kv">
        <div v-for="kv in card.content.keyValues" :key="kv.key" class="kv-row">
          <span class="kv-key">{{ kv.key }}:</span>
          <span class="kv-value">{{ kv.value }}</span>
        </div>
      </div>

      <!-- 进度条 -->
      <div v-if="card.content.progress" class="content-progress">
        <div class="progress-bar">
          <div
            class="progress-fill"
            :style="{ width: progressPercent + '%' }"
          ></div>
        </div>
        <span class="progress-label">
          {{ card.content.progress.label || `${progressPercent}%` }}
        </span>
      </div>

      <!-- 代码块 -->
      <pre v-if="card.content.code" class="content-code">
        <code :class="`language-${card.content.code.language}`">
          {{ card.content.code.content }}
        </code>
      </pre>
    </div>

    <!-- 操作按钮 -->
    <div v-if="card.actions?.length" class="card-actions">
      <button
        v-for="action in card.actions"
        :key="action.id"
        :class="[`btn-${action.type}`]"
        @click="handleAction(action)"
      >
        {{ action.label }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { TypelessCard } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCard
}>()

const emit = defineEmits<{
  (e: 'action', action: TypelessCardAction): void
}>()

const progressPercent = computed(() => {
  if (!props.card.content.progress) return 0
  const { current, total } = props.card.content.progress
  return Math.round((current / total) * 100)
})

function getIcon(type: string): string {
  const icons: Record<string, string> = {
    info: '📊',
    action: '🔧',
    form: '📝',
    progress: '⏳',
    result: '✅'
  }
  return icons[type] || '📋'
}

function handleAction(action: TypelessCardAction) {
  emit('action', action)
}
</script>
```
{% endraw %}

### 16.5 在聊天消息中集成

```typescript
// 消息类型扩展
interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string

  // 新增: Typeless 卡片
  cards?: TypelessCard[]
}

// 解析消息中的卡片标记
function parseTypelessCards(content: string): {
  text: string
  cards: TypelessCard[]
} {
  // 解析 :::card ... ::: 格式的卡片标记
  const cardRegex = /:::card\s*(\{[\s\S]*?\})\s*:::/g
  const cards: TypelessCard[] = []
  let text = content

  let match
  while ((match = cardRegex.exec(content)) !== null) {
    try {
      const cardData = JSON.parse(match[1])
      cards.push(cardData)
      text = text.replace(match[0], '')
    } catch (e) {
      console.error('Failed to parse card:', e)
    }
  }

  return { text: text.trim(), cards }
}
```

---

## 13. 附录

### 13.1 相关文件

| 文件 | 用途 |
|------|------|
| [01-product-designed-user-journey.md](01-product-designed-user-journey.md) | 现有用户旅程文档 |
| web/src/router/index.ts | 路由配置 |
| web/src/stores/auth.ts | 认证状态管理 |
| server/internal/user/ | 用户管理后端 |
| server/internal/auth/ | 认证中间件 |

### 13.2 设计决策记录

| 决策 | 理由 |
|------|------|
| 10KB Token 限制 | 足够完成 2-3 轮有意义的对话 |
| 5 次对话限制 | 足够体验核心功能，不会过度消耗资源 |
| 不强制跳转 | 用户体验优先，允许继续浏览 |
| 移除设置向导 | 简化流程，降低使用门槛 |
| 第一个用户为管理员 | 简化权限模型，符合单用户场景 |
