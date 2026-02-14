# 开发者指南

本指南涵盖参与 ZimaOS Blue 开发所需了解的全部内容。

## 开发环境准备

### 前置要求

- **Go**：1.21 或更高
- **Node.js**：18 或更高
- **pnpm**：8 或更高（前端）
- **Git**：2.30 或更高
- **Docker**：（可选，用于测试）
- **SQLite**：3.x（本地开发）

### 克隆仓库

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

### 后端准备

```bash
cd server

# 安装依赖
go mod download

# 运行测试
go test ./... -v

# 构建
go build -o zimaos-blue ./cmd/server

# 热重载运行（使用 air）
go install github.com/cosmtrek/air@latest
air
```

### 前端准备

```bash
cd web

# 安装依赖
pnpm install

# 启动开发服务
pnpm dev

# 生产构建
pnpm build

# 运行测试
pnpm test
```

### IDE 配置

**VS Code（推荐）**

安装扩展：
- Go
- Vue - Official
- ESLint
- Prettier

设置（`.vscode/settings.json`）：
```json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  },
  "[vue]": {
    "editor.defaultFormatter": "Vue.volar"
  }
}
```

**GoLand/WebStorm**

- 启用 Go modules 集成
- 将 golangci-lint 配置为外部工具
- 为前端启用 ESLint

---

## 项目结构

```
ZimaOS-Blue/
├── server/                 # Go 后端
│   ├── cmd/
│   │   └── server/        # 主入口
│   ├── internal/          # 内部包
│   │   ├── auth/          # 认证
│   │   ├── backup/        # 备份与恢复
│   │   ├── channel/       # 消息通道
│   │   ├── chaos/         # 混沌测试
│   │   ├── config/        # 配置
│   │   ├── database/      # 数据库层
│   │   ├── homeassistant/ # HA 集成
│   │   ├── leakdetect/    # 泄漏检测
│   │   ├── llm/           # LLM 提供商
│   │   ├── plugin/        # 插件系统
│   │   ├── ratelimit/     # 限流
│   │   ├── rbac/          # 基于角色的访问
│   │   ├── server/        # HTTP 服务
│   │   ├── session/       # 会话管理
│   │   ├── setup/         # 设置向导
│   │   └── zimaos/        # ZimaOS 集成
│   ├── go.mod
│   └── go.sum
├── web/                    # Vue.js 前端
│   ├── src/
│   │   ├── api/           # API 客户端
│   │   ├── components/   # Vue 组件
│   │   ├── composables/   # Vue 组合式
│   │   ├── router/       # Vue Router
│   │   ├── stores/       # Pinia 状态
│   │   └── views/        # 页面组件
│   ├── package.json
│   └── vite.config.ts
├── docs/                   # 文档
├── DEV/                    # 开发检查清单
└── i18n/                   # 翻译
```

---

## 代码风格

### Go 代码风格

遵循标准 Go 风格并做少量补充：

**格式化**
```bash
# 格式化代码
gofmt -w .

# 或使用 goimports
goimports -w .
```

**静态检查**
```bash
# 运行检查
golangci-lint run

# 自动修复可修复项
golangci-lint run --fix
```

**命名约定**
- 未导出标识符使用 `camelCase`
- 导出标识符使用 `PascalCase`
- 使用有意义的名称（循环外避免单字母）
- 缩写全大写：`HTTPServer`、`userID`

**错误处理**
```go
// 推荐
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}

// 不推荐
if err != nil {
    return err  // 无上下文
}
```

**注释**
```go
// Package auth 提供认证与授权。
package auth

// User 表示系统用户。
type User struct {
    ID       string
    Username string
}

// CreateUser 使用给定用户名创建用户。
// 若用户名已存在则返回错误。
func CreateUser(username string) (*User, error) {
    // ...
}
```

### TypeScript/Vue 代码风格

**ESLint 配置**
```javascript
// .eslintrc.js
module.exports = {
  extends: [
    'plugin:vue/vue3-recommended',
    '@vue/typescript/recommended',
  ],
  rules: {
    'vue/multi-word-component-names': 'off',
    '@typescript-eslint/explicit-function-return-type': 'off',
  },
}
```

**组件结构**
```vue
<script setup lang="ts">
// 1. 导入
import { ref, computed, onMounted } from 'vue'

// 2. Props 与 Emits
const props = defineProps<{
  title: string
  count?: number
}>()

const emit = defineEmits<{
  update: [value: string]
}>()

// 3. 响应式状态
const isLoading = ref(false)

// 4. 计算属性
const displayTitle = computed(() => props.title.toUpperCase())

// 5. 方法
function handleClick() {
  emit('update', 'new value')
}

// 6. 生命周期
onMounted(() => {
  // ...
})
</script>

<template>
  <!-- 模板 -->
</template>

<style scoped>
/* 样式 */
</style>
```

---

## 测试指南

### Go 测试

**单元测试**
```go
func TestUserService_Create(t *testing.T) {
    // Arrange
    svc := NewUserService(mockDB)

    // Act
    user, err := svc.Create("testuser")

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if user.Username != "testuser" {
        t.Errorf("expected username 'testuser', got %s", user.Username)
    }
}
```

**表驱动测试**
```go
func TestValidatePassword(t *testing.T) {
    tests := []struct {
        name     string
        password string
        wantErr  bool
    }{
        {"valid", "SecurePass123!", false},
        {"too_short", "short", true},
        {"no_number", "SecurePassword!", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePassword(tt.password)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**运行测试**
```bash
# 全部测试
go test ./...

# 带覆盖率
go test ./... -cover

# 指定包
go test ./internal/auth/... -v

# 竞态检测
go test ./... -race
```

### 前端测试

**组件测试**
```typescript
import { mount } from '@vue/test-utils'
import ChatInput from '@/components/ChatInput.vue'

describe('ChatInput', () => {
  it('emits send event on enter', async () => {
    const wrapper = mount(ChatInput)

    await wrapper.find('textarea').setValue('Hello')
    await wrapper.find('textarea').trigger('keydown.enter')

    expect(wrapper.emitted('send')).toBeTruthy()
    expect(wrapper.emitted('send')[0]).toEqual(['Hello', []])
  })
})
```

**运行测试**
```bash
# 运行测试
pnpm test

# 带覆盖率
pnpm test:coverage

# 监听模式
pnpm test:watch
```

---

## 贡献指南

### 工作流

1. **Fork** 仓库
2. **创建** 功能分支：`git checkout -b feature/my-feature`
3. **修改** 代码
4. **测试**：`go test ./...` 与 `pnpm test`
5. **提交** 使用约定式提交：`git commit -m "feat: add new feature"`
6. **推送** 到你的 Fork：`git push origin feature/my-feature`
7. **发起** Pull Request

### 提交信息

采用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

类型：
- `feat`：新功能
- `fix`：Bug 修复
- `docs`：文档
- `style`：代码风格（格式化等）
- `refactor`：重构
- `test`：测试
- `chore`：维护

示例：
```
feat(auth): add MFA support
fix(chat): resolve message ordering issue
docs(api): update API reference
refactor(llm): simplify provider interface
```

### Pull Request 规范

1. **标题**：使用约定式提交格式
2. **描述**：说明做了什么、为什么
3. **测试**：新功能需附带测试
4. **文档**：如有需要请更新文档
5. **破坏性变更**：明确说明并文档化

### 代码评审

合并前需通过评审：

- 至少一名维护者批准
- 所有 CI 检查通过
- 无未解决讨论

---

## 架构概览

### 后端架构

```
┌─────────────────────────────────────────────────────────┐
│                      HTTP Server                         │
│  (Echo 框架、中间件、路由)                                  │
├─────────────────────────────────────────────────────────┤
│                      Handlers                            │
│  (请求校验、响应格式化)                                    │
├─────────────────────────────────────────────────────────┤
│                      Services                            │
│  (业务逻辑、编排)                                          │
├─────────────────────────────────────────────────────────┤
│                    Repositories                          │
│  (数据访问、缓存)                                          │
├─────────────────────────────────────────────────────────┤
│                      Database                            │
│  (SQLite，可选 PostgreSQL)                               │
└─────────────────────────────────────────────────────────┘
```

### 前端架构

```
┌─────────────────────────────────────────────────────────┐
│                        Views                             │
│  (页面组件、路由)                                          │
├─────────────────────────────────────────────────────────┤
│                     Components                           │
│  (可复用 UI 组件)                                         │
├─────────────────────────────────────────────────────────┤
│                   Composables                            │
│  (共享逻辑、hooks)                                        │
├─────────────────────────────────────────────────────────┤
│                      Stores                              │
│  (Pinia 状态管理)                                         │
├─────────────────────────────────────────────────────────┤
│                    API Client                            │
│  (HTTP 请求、WebSocket)                                  │
└─────────────────────────────────────────────────────────┘
```

---

## 调试

### 后端调试

**VS Code 启动配置**
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/server/cmd/server",
      "args": ["--config", "config.yaml"]
    }
  ]
}
```

**使用 Delve**
```bash
# 安装 delve
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试
dlv debug ./cmd/server -- --config config.yaml
```

### 前端调试

- 使用 Vue DevTools 浏览器扩展
- 使用浏览器开发者工具
- 在代码中添加 `debugger` 语句

---

## 发布流程

1. 在 `version.go` 中更新版本号
2. 更新 CHANGELOG.md
3. 创建发布分支：`release/v0.5.0`
4. 运行完整测试
5. 在 GitHub 创建带标签的 Release
6. CI 构建并发布产物

---

## 参考资源

- [Go 文档](https://go.dev/doc/)
- [Vue.js 文档](https://vuejs.org/)
- [Echo 框架](https://echo.labstack.com/)
- [Pinia](https://pinia.vuejs.org/)
- [Tailwind CSS](https://tailwindcss.com/)
