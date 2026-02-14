# PRD: Preview Mode & Onboarding Experience

**Version**: 0.10.7
**Author**: ZimaOS-Blue Team
**Status**: Draft
**Created**: 2026-01-31
**UED Reference**: [02-preview-mode-onboarding-design.md](../UED/02-preview-mode-onboarding-design.md)

---

## 1. Overview

### 1.1 Background

ZimaOS-Blue 当前要求用户在首次使用时完成设置向导，包括创建管理员账户和配置 Provider。这种流程虽然完整，但增加了用户的首次使用门槛，可能导致潜在用户流失。

通过引入预览模式（Preview Mode），用户可以零配置立即体验产品核心功能，降低使用门槛，提高产品转化率。

### 1.2 Goals

1. 实现零门槛体验，用户无需任何配置即可立即使用
2. 提供内置体验 Provider，自动注入免费试用额度
3. 实现预览模式到正式模式的平滑转化流程
4. 在空白聊天区域提供预置问题引导，提升用户体验
5. 支持 Typeless 卡片，丰富聊天交互形式
6. 对预览模式下受限功能提供友好提示

### 1.3 Non-Goals

1. 预览模式下的用户系统（无登录、无多用户）
2. 预览模式下的管理功能（用户管理、审计日志等）

### 1.4 Key Clarification

**预览模式功能范围**：预览模式下除了没有用户系统外，其他功能均正常可用，包括：
- 完整的聊天功能（附件、图片、语音等）
- Provider 配置和使用
- 对话历史保存
- 所有 AI 能力

预览模式产生的数据（对话、配置等）在用户创建管理员账户后将自动迁移给管理员。

---

## 2. User Stories

### 2.1 As a New User

- 我希望首次访问时无需任何配置即可开始对话
- 我希望看到预置问题引导，快速了解产品能力
- 我希望在达到使用限制时收到友好提示，而非强制跳转
- 我希望可以随时选择创建账户以解锁完整功能

### 2.2 As a Preview User

- 我希望清楚地知道当前处于预览模式
- 我希望看到剩余使用额度（对话次数、Token 用量）
- 我希望点击受限功能时收到明确提示，而非无响应
- 我希望可以方便地升级到正式账户

### 2.3 As an Administrator

- 我希望从预览模式升级后自动成为管理员
- 我希望升级后可以创建和管理其他用户
- 我希望可以配置自己的 Provider 替代体验 Provider

---

## 3. Functional Requirements

### 3.1 Preview Mode Core

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-001 | 检测系统是否存在管理员账户，无则进入预览模式 | P0 |
| FR-002 | 预览模式下自动注入 ZimaOS Preview Provider | P0 |
| FR-003 | 预览模式限制：10KB Token 上限 | P0 |
| FR-004 | 预览模式限制：5 次对话上限 | P0 |
| FR-005 | 实时显示使用进度（Token/对话次数） | P0 |
| FR-006 | 达到限制时显示友好提示，引导创建账户 | P0 |

### 3.2 Preview Mode UI

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-007 | 右上角显示"结束预览模式"入口 | P0 |
| FR-008 | Provider 选择器显示预览状态和使用进度 | P1 |
| FR-009 | 预览模式下功能按钮显示锁定状态 | P1 |
| FR-010 | 点击受限功能显示解锁提示 | P1 |

### 3.3 Account Upgrade & Data Migration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-011 | 提供简化的管理员创建表单 | P0 |
| FR-012 | 创建账户后自动登录 | P0 |
| FR-013 | 升级后移除 Preview Provider | P0 |
| FR-014 | 升级后引导配置自己的 Provider | P1 |
| FR-015 | 预览模式数据自动迁移给管理员 | P0 |
| FR-016 | 迁移数据包括：对话历史、Provider 配置、系统设置 | P0 |

### 3.4 Preset Questions (空白区域引导)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-017 | 聊天区域为空时显示预置问题卡片 | P1 |
| FR-018 | 预置问题分类：写作、编程、创意、学习、效率 | P1 |
| FR-019 | 随机展示 4 个问题，确保分类多样性 | P1 |
| FR-020 | 支持"换一批"刷新问题 | P1 |
| FR-021 | 点击问题填入输入框并聚焦 | P1 |

### 3.5 Typeless Cards (聊天卡片支持)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-022 | 支持 info 类型卡片（信息展示） | P1 |
| FR-023 | 支持 action 类型卡片（操作按钮） | P1 |
| FR-024 | 支持 progress 类型卡片（进度展示） | P1 |
| FR-025 | 支持 result 类型卡片（结果展示） | P1 |
| FR-026 | 卡片内容支持：文本、列表、键值对、进度条、代码块 | P1 |
| FR-027 | 卡片支持操作按钮交互 | P2 |

### 3.6 API Endpoints

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/api/v1/system/mode` | GET | 获取系统模式（preview/normal） | Public |
| `/api/v1/preview/status` | GET | 获取预览模式状态和使用情况 | Public |
| `/api/v1/preview/upgrade` | POST | 从预览模式升级（创建管理员） | Public |
| `/api/v1/preset-questions` | GET | 获取预置问题列表 | Public |

---

## 4. Technical Design

### 4.1 Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Preview Mode Architecture                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        System Mode Detection                         │    │
│  │  ┌──────────────────┐              ┌──────────────────┐             │    │
│  │  │  Admin Exists?   │──── No ────▶│  Preview Mode    │             │    │
│  │  │                  │              │  (Guest Access)  │             │    │
│  │  └──────────────────┘              └──────────────────┘             │    │
│  │          │ Yes                              │                        │    │
│  │          ▼                                  ▼                        │    │
│  │  ┌──────────────────┐              ┌──────────────────┐             │    │
│  │  │  Normal Mode     │              │  Inject Preview  │             │    │
│  │  │  (Auth Required) │              │  Provider        │             │    │
│  │  └──────────────────┘              └──────────────────┘             │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        Preview Provider                              │    │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │    │
│  │  │  Usage Tracker   │  │  Limit Checker   │  │  Upgrade Handler │  │    │
│  │  │  - Tokens Used   │  │  - 10KB Token    │  │  - Create Admin  │  │    │
│  │  │  - Conversations │  │  - 5 Convos      │  │  - Auto Login    │  │    │
│  │  └──────────────────┘  └──────────────────┘  └──────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        UI Components                                 │    │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌────────────┐ │    │
│  │  │PreviewBanner │ │PresetQuestions│ │FeatureBlock │ │TypelessCard│ │    │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └────────────┘ │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Data Models

```go
// SystemMode represents the current system mode
type SystemMode struct {
    Mode     string            `json:"mode"`     // "preview" or "normal"
    Features map[string]bool   `json:"features"` // Available features
}

// PresetQuestion represents a preset question for onboarding
type PresetQuestion struct {
    ID         string `json:"id"`
    Icon       string `json:"icon"`
    Title      string `json:"title"`
    FullPrompt string `json:"full_prompt"`
    Category   string `json:"category"` // writing, coding, creative, learning, productivity
}

// TypelessCard represents a dynamic card in chat
type TypelessCard struct {
    ID       string              `json:"id"`
    Type     string              `json:"type"` // info, action, form, progress, result
    Title    string              `json:"title,omitempty"`
    Content  TypelessCardContent `json:"content"`
    Actions  []CardAction        `json:"actions,omitempty"`
    Metadata map[string]any      `json:"metadata,omitempty"`
}

type TypelessCardContent struct {
    Text      string            `json:"text,omitempty"`
    List      []string          `json:"list,omitempty"`
    KeyValues []KeyValue        `json:"key_values,omitempty"`
    Progress  *ProgressInfo     `json:"progress,omitempty"`
    Code      *CodeBlock        `json:"code,omitempty"`
}

type KeyValue struct {
    Key   string `json:"key"`
    Value string `json:"value"`
}

type ProgressInfo struct {
    Current int    `json:"current"`
    Total   int    `json:"total"`
    Label   string `json:"label,omitempty"`
}

type CodeBlock struct {
    Language string `json:"language"`
    Content  string `json:"content"`
}

type CardAction struct {
    ID      string `json:"id"`
    Label   string `json:"label"`
    Type    string `json:"type"` // primary, secondary, danger
    Action  string `json:"action"`
    Payload any    `json:"payload,omitempty"`
}

// UpgradeRequest represents the request to upgrade from preview mode
type UpgradeRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

// UpgradeResponse represents the response after successful upgrade
type UpgradeResponse struct {
    Success      bool   `json:"success"`
    User         User   `json:"user"`
    Token        string `json:"token"`
    RefreshToken string `json:"refresh_token"`
    DataMigrated bool   `json:"data_migrated"` // Whether preview data was migrated
}
```

### 4.3 Preview Mode Detection

```go
// PreviewModeMiddleware checks if system is in preview mode
func PreviewModeMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Check if admin exists
            adminExists := userService.AdminExists()

            if !adminExists {
                // Preview mode
                c.Set("preview_mode", true)
                c.Set("user_role", "guest")

                // Inject preview provider if not exists
                previewStore.EnsurePreviewProvider()
            }

            return next(c)
        }
    }
}

// isPreviewMode checks current mode
func isPreviewMode(c echo.Context) bool {
    if val, ok := c.Get("preview_mode").(bool); ok {
        return val
    }
    return !userService.AdminExists()
}
```

### 4.4 Data Migration on Upgrade

```go
// DataMigrationService handles migration of preview data to admin account
type DataMigrationService struct {
    db *sql.DB
}

// MigratePreviewData migrates all preview mode data to the new admin user
func (s *DataMigrationService) MigratePreviewData(adminUserID string) error {
    tx, err := s.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. Migrate conversations - assign to admin user
    _, err = tx.Exec(`
        UPDATE conversations
        SET user_id = ?
        WHERE user_id IS NULL OR user_id = ''
    `, adminUserID)
    if err != nil {
        return fmt.Errorf("migrate conversations: %w", err)
    }

    // 2. Migrate messages - already linked via conversation_id
    // No action needed as messages are linked to conversations

    // 3. Migrate provider configurations - keep as-is
    // Provider configs are system-level, not user-specific

    // 4. Migrate system settings - keep as-is
    // System settings remain unchanged

    // 5. Mark migration as complete
    _, err = tx.Exec(`
        INSERT INTO system_config (key, value, updated_at)
        VALUES ('preview_data_migrated', 'true', CURRENT_TIMESTAMP)
        ON CONFLICT(key) DO UPDATE SET value = 'true', updated_at = CURRENT_TIMESTAMP
    `)
    if err != nil {
        return fmt.Errorf("mark migration complete: %w", err)
    }

    return tx.Commit()
}

// GetMigrationStatus checks if preview data has been migrated
func (s *DataMigrationService) GetMigrationStatus() (bool, error) {
    var value string
    err := s.db.QueryRow(`
        SELECT value FROM system_config WHERE key = 'preview_data_migrated'
    `).Scan(&value)

    if err == sql.ErrNoRows {
        return false, nil
    }
    if err != nil {
        return false, err
    }

    return value == "true", nil
}
```

### 4.5 Upgrade Handler

```go
// UpgradeHandler handles the upgrade from preview mode to normal mode
func (h *PreviewHandler) Upgrade(c echo.Context) error {
    var req UpgradeRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
    }

    // Validate input
    if req.Username == "" || req.Password == "" {
        return echo.NewHTTPError(http.StatusBadRequest, "username and password required")
    }

    // Check if already has admin
    if h.userService.AdminExists() {
        return echo.NewHTTPError(http.StatusConflict, "admin already exists")
    }

    // Create admin user
    user, err := h.userService.CreateAdmin(req.Username, req.Password)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to create admin")
    }

    // Migrate preview data to admin
    dataMigrated := false
    if err := h.migrationService.MigratePreviewData(user.ID); err != nil {
        // Log error but don't fail the upgrade
        log.Printf("Warning: failed to migrate preview data: %v", err)
    } else {
        dataMigrated = true
    }

    // Generate tokens
    token, refreshToken, err := h.authService.GenerateTokens(user)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate tokens")
    }

    return c.JSON(http.StatusOK, UpgradeResponse{
        Success:      true,
        User:         *user,
        Token:        token,
        RefreshToken: refreshToken,
        DataMigrated: dataMigrated,
    })
}
```

### 4.6 API Response Examples

```json
// GET /api/v1/system/mode
{
  "mode": "preview",
  "features": {
    "chat": true,
    "attachment": true,
    "image_upload": true,
    "voice_play": true,
    "voice_input": true,
    "provider_config": true,
    "settings": true,
    "admin": false,
    "user_management": false
  }
}

// GET /api/v1/preview/status
{
  "active": true,
  "message": "预览模式 - 创建账户后数据将自动迁移"
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
  "refresh_token": "refresh_token",
  "data_migrated": true
}

// GET /api/v1/preset-questions
{
  "questions": [
    {
      "id": "email-leave",
      "icon": "💡",
      "title": "帮我写一封邮件给同事请假",
      "full_prompt": "帮我写一封请假邮件，说明因为个人原因需要请假一天",
      "category": "writing"
    },
    {
      "id": "docker",
      "icon": "🔧",
      "title": "解释什么是 Docker 容器",
      "full_prompt": "请用简单的语言解释 Docker 容器是什么，以及它的主要用途",
      "category": "coding"
    }
  ]
}
```

### 4.7 Database Schema

```sql
-- System configuration table
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Track migration status
INSERT INTO system_config (key, value) VALUES
    ('preview_data_migrated', 'false');
```

---

## 5. UI/UX Design

### 5.1 Preview Mode Banner

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  [Logo] ZimaOS Blue                                    [创建账户 ▼]         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  点击 [创建账户] 下拉菜单:                                                  │
│  ┌──────────────────────────┐                                               │
│  │ 🎉 创建管理员账户        │  ← 主要操作                                   │
│  │ ─────────────────────── │                                               │
│  │ ❓ 了解更多              │                                               │
│  └──────────────────────────┘                                               │
│                                                                              │
│  提示文案: "当前为预览模式，创建账户后数据将自动保留"                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Preset Questions (Empty State)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                         [ZimaOS Blue Logo]                                   │
│                                                                              │
│                    欢迎体验 ZimaOS Blue                                      │
│                    点击下方问题快速开始                                       │
│                                                                              │
│   ┌─────────────────────┐  ┌─────────────────────┐                          │
│   │ 💡 帮我写一封邮件    │  │ 📝 总结这篇文章     │                          │
│   │    给同事请假        │  │    的主要观点       │                          │
│   └─────────────────────┘  └─────────────────────┘                          │
│                                                                              │
│   ┌─────────────────────┐  ┌─────────────────────┐                          │
│   │ 🔧 解释什么是        │  │ 🎨 帮我设计一个     │                          │
│   │    Docker 容器       │  │    Logo 创意        │                          │
│   └─────────────────────┘  └─────────────────────┘                          │
│                                                                              │
│                         [🔄 换一批]                                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Upgrade Modal

```
┌─────────────────────────────────────────────────────────────┐
│  ┌─────────────────────────────────────────────────────┐   │
│  │  🎉 创建管理员账户                                  │   │
│  │                                                     │   │
│  │  创建账户后：                                       │   │
│  │  ✓ 您的对话历史将自动保留                           │   │
│  │  ✓ 可以管理用户和系统设置                           │   │
│  │  ✓ 数据安全持久化存储                               │   │
│  │                                                     │   │
│  │  用户名: [____________]                             │   │
│  │  密码:   [____________]                             │   │
│  │  确认:   [____________]                             │   │
│  │                                                     │   │
│  │  [创建账户]  [稍后再说]                             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 5.4 Typeless Card Examples

```
信息卡片:
┌─────────────────────────────────────────────────────────────┐
│  📊 系统状态                                                │
│  ─────────────────────────────────────────────────────────  │
│  CPU 使用率:     45%                                        │
│  内存使用:       2.3GB / 8GB                                │
│  磁盘空间:       120GB / 500GB                              │
│  运行时间:       3 天 12 小时                               │
└─────────────────────────────────────────────────────────────┘

进度卡片:
┌─────────────────────────────────────────────────────────────┐
│  ⏳ 正在处理您的请求                                        │
│  ─────────────────────────────────────────────────────────  │
│  [████████████░░░░░░░░] 60%                                 │
│                                                             │
│  正在分析文档内容...                                        │
└─────────────────────────────────────────────────────────────┘

操作卡片:
┌─────────────────────────────────────────────────────────────┐
│  🔧 检测到可用更新                                          │
│  ─────────────────────────────────────────────────────────  │
│  ZimaOS Blue v0.10.7 已发布                                 │
│                                                             │
│  [立即更新]  [稍后提醒]  [查看详情]                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. Frontend Components

### 6.1 New Components

```
web/src/components/
├── preview/
│   ├── PreviewBanner.vue           # 预览模式顶部提示条
│   └── PreviewUpgradeForm.vue      # 创建管理员表单
├── onboarding/
│   ├── PresetQuestions.vue         # 预置问题卡片组
│   └── PresetQuestionCard.vue      # 单个问题卡片
├── typeless/
│   ├── TypelessCard.vue            # 通用卡片组件
│   ├── CardInfo.vue                # 信息卡片
│   ├── CardProgress.vue            # 进度卡片
│   ├── CardAction.vue              # 操作卡片
│   └── CardResult.vue              # 结果卡片
```

### 6.2 Store Changes

```typescript
// stores/preview.ts
export const usePreviewStore = defineStore('preview', {
  state: () => ({
    isPreviewMode: false
  }),

  actions: {
    async fetchStatus() {
      const response = await api.get('/system/mode')
      this.isPreviewMode = response.data.mode === 'preview'
    },

    async upgrade(credentials: { username: string; password: string }) {
      const response = await api.post('/preview/upgrade', credentials)
      this.isPreviewMode = false
      return response.data
    }
  }
})
```

### 6.3 Route Guard Updates

```typescript
// router/index.ts
router.beforeEach(async (to, from, next) => {
  const previewStore = usePreviewStore()
  const authStore = useAuthStore()

  // Fetch system mode
  await previewStore.fetchStatus()

  // Preview mode - allow access to most routes without auth
  if (previewStore.isPreviewMode) {
    // Block admin-only routes in preview mode
    if (to.meta.requiresAdmin) {
      return next('/chat')
    }
    return next()
  }

  // Normal mode - require auth
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return next('/login')
  }

  next()
})
```

---

## 7. Security Considerations

### 7.1 Preview Mode Scope

| 功能 | 预览模式 | 说明 |
|------|----------|------|
| 聊天功能 | ✅ 完整 | 包括附件、图片、语音 |
| Provider 配置 | ✅ 完整 | 可添加和配置 Provider |
| 对话历史 | ✅ 保存 | 数据持久化，升级后迁移 |
| 用户管理 | ❌ 禁用 | 需要创建管理员后启用 |
| 系统管理 | ❌ 禁用 | 需要创建管理员后启用 |

### 7.2 Route Protection

```typescript
// Preview mode allowed routes (most routes)
const previewAllowedRoutes = [
  '/chat',        // Core chat functionality
  '/settings',    // Settings
  '/providers',   // Provider configuration
]

// Preview mode blocked routes (admin only)
const previewBlockedRoutes = [
  '/admin/*',     // Admin functions
  '/users',       // User management
  '/audit',       // Audit logs
]
```

---

## 8. Testing Strategy

### 8.1 Unit Tests

- Preview mode detection logic
- Data migration logic
- Preset question randomization
- Typeless card parsing

### 8.2 Integration Tests

- Preview mode API endpoints
- Upgrade flow (preview → admin)
- Data migration verification

### 8.3 E2E Tests

| Scenario | Expected Result |
|----------|-----------------|
| 首次访问，无管理员 | 直接进入 /chat，显示预览模式 |
| 发送消息 | 正常响应，对话保存 |
| 配置 Provider | 正常配置，数据保存 |
| 刷新页面 | 保持预览模式状态，数据保留 |
| 创建管理员 | 成功创建，自动登录，数据迁移 |
| 升级后查看对话 | 预览模式的对话历史保留 |

---

## 9. Implementation Phases

### Phase 1: Core Preview Mode (MVP)

- [ ] 预览模式检测逻辑
- [ ] 右上角"创建账户"入口
- [ ] 管理员创建表单
- [ ] 数据迁移逻辑

### Phase 2: Experience Enhancement

- [ ] 空白区域预置问题引导
- [ ] Typeless 卡片支持

---

## 10. Metrics & Monitoring

### 10.1 Metrics to Track

- `blue_preview_sessions_total`: Total preview sessions
- `blue_preview_upgrades_total`: Successful upgrades to admin
- `blue_preview_data_migrations_total`: Successful data migrations
- `blue_preset_question_clicks_total`: Preset question click count

### 10.2 Conversion Funnel

```
Preview Session Started
        ↓
First Message Sent
        ↓
Multiple Conversations / Provider Configured
        ↓
Upgrade Button Clicked
        ↓
Admin Account Created (Conversion)
        ↓
Data Migrated Successfully
```

---

## 11. Rollout Plan

### Phase 1: Internal Testing
- Deploy to staging environment
- Internal team testing
- Bug fixes and refinements

### Phase 2: Beta Release
- Enable for new installations only
- Monitor conversion metrics
- Gather user feedback

### Phase 3: General Availability
- Enable for all users
- Remove legacy setup wizard
- Full documentation

---

## 12. Open Questions

1. 预置问题是否需要支持自定义/扩展？
2. Typeless 卡片是否需要支持更多内容类型（图表等）？
3. 数据迁移失败时的回滚策略？

---

## 13. References

- [UED Design Document](../UED/02-preview-mode-onboarding-design.md)
- [Provider Pool PRD](./archived/v0.10.6-provider-pool-prd.md)
- [User Journey Document](../UED/01-product-designed-user-journey.md)
