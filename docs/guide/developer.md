# Developer Guide

This guide covers everything you need to know to contribute to ZimaOS Echo development.

## Development Environment Setup

### Prerequisites

- **Go**: 1.21 or later
- **Node.js**: 18 or later
- **pnpm**: 8 or later (for frontend)
- **Git**: 2.30 or later
- **Docker**: (optional, for testing)
- **SQLite**: 3.x (for local development)

### Clone the Repository

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
```

### Backend Setup

```bash
cd server

# Install dependencies
go mod download

# Run tests
go test ./... -v

# Build
go build -o zimaos-echo ./cmd/server

# Run with hot reload (using air)
go install github.com/cosmtrek/air@latest
air
```

### Frontend Setup

```bash
cd web

# Install dependencies
pnpm install

# Run development server
pnpm dev

# Build for production
pnpm build

# Run tests
pnpm test
```

### IDE Setup

**VS Code (Recommended)**

Install extensions:
- Go
- Vue - Official
- ESLint
- Prettier

Settings (`.vscode/settings.json`):
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

- Enable Go modules integration
- Configure golangci-lint as external tool
- Enable ESLint for frontend

---

## Project Structure

```
ZimaOS-Echo/
├── server/                 # Go backend
│   ├── cmd/
│   │   └── server/        # Main entry point
│   ├── internal/          # Internal packages
│   │   ├── auth/          # Authentication
│   │   ├── backup/        # Backup & restore
│   │   ├── channel/       # Messaging channels
│   │   ├── chaos/         # Chaos testing
│   │   ├── config/        # Configuration
│   │   ├── database/      # Database layer
│   │   ├── homeassistant/ # HA integration
│   │   ├── leakdetect/    # Leak detection
│   │   ├── llm/           # LLM providers
│   │   ├── plugin/        # Plugin system
│   │   ├── ratelimit/     # Rate limiting
│   │   ├── rbac/          # Role-based access
│   │   ├── server/        # HTTP server
│   │   ├── session/       # Session management
│   │   ├── setup/         # Setup wizard
│   │   └── zimaos/        # ZimaOS integration
│   ├── go.mod
│   └── go.sum
├── web/                    # Vue.js frontend
│   ├── src/
│   │   ├── api/           # API clients
│   │   ├── components/    # Vue components
│   │   ├── composables/   # Vue composables
│   │   ├── router/        # Vue Router
│   │   ├── stores/        # Pinia stores
│   │   └── views/         # Page components
│   ├── package.json
│   └── vite.config.ts
├── docs/                   # Documentation
├── DEV/                    # Development checklists
└── i18n/                   # Translations
```

---

## Code Style Guide

### Go Code Style

We follow the standard Go style with some additions:

**Formatting**
```bash
# Format code
gofmt -w .

# Or use goimports
goimports -w .
```

**Linting**
```bash
# Run linter
golangci-lint run

# Fix auto-fixable issues
golangci-lint run --fix
```

**Naming Conventions**
- Use `camelCase` for unexported identifiers
- Use `PascalCase` for exported identifiers
- Use descriptive names (avoid single letters except in loops)
- Acronyms should be all caps: `HTTPServer`, `userID`

**Error Handling**
```go
// Good
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}

// Bad
if err != nil {
    return err  // No context
}
```

**Comments**
```go
// Package auth provides authentication and authorization.
package auth

// User represents a system user.
type User struct {
    ID       string
    Username string
}

// CreateUser creates a new user with the given username.
// It returns an error if the username is already taken.
func CreateUser(username string) (*User, error) {
    // ...
}
```

### TypeScript/Vue Code Style

**ESLint Configuration**
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

**Component Structure**
```vue
<script setup lang="ts">
// 1. Imports
import { ref, computed, onMounted } from 'vue'

// 2. Props & Emits
const props = defineProps<{
  title: string
  count?: number
}>()

const emit = defineEmits<{
  update: [value: string]
}>()

// 3. Reactive state
const isLoading = ref(false)

// 4. Computed properties
const displayTitle = computed(() => props.title.toUpperCase())

// 5. Methods
function handleClick() {
  emit('update', 'new value')
}

// 6. Lifecycle hooks
onMounted(() => {
  // ...
})
</script>

<template>
  <!-- Template -->
</template>

<style scoped>
/* Styles */
</style>
```

---

## Testing Guide

### Go Testing

**Unit Tests**
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

**Table-Driven Tests**
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

**Running Tests**
```bash
# All tests
go test ./...

# With coverage
go test ./... -cover

# Specific package
go test ./internal/auth/... -v

# With race detection
go test ./... -race
```

### Frontend Testing

**Component Tests**
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

**Running Tests**
```bash
# Run tests
pnpm test

# With coverage
pnpm test:coverage

# Watch mode
pnpm test:watch
```

---

## Contributing Guide

### Workflow

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature/my-feature`
3. **Make** your changes
4. **Test** your changes: `go test ./...` and `pnpm test`
5. **Commit** with conventional commits: `git commit -m "feat: add new feature"`
6. **Push** to your fork: `git push origin feature/my-feature`
7. **Open** a Pull Request

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Code style (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

Examples:
```
feat(auth): add MFA support
fix(chat): resolve message ordering issue
docs(api): update API reference
refactor(llm): simplify provider interface
```

### Pull Request Guidelines

1. **Title**: Use conventional commit format
2. **Description**: Explain what and why
3. **Tests**: Include tests for new features
4. **Documentation**: Update docs if needed
5. **Breaking Changes**: Clearly document any breaking changes

### Code Review

All PRs require review before merging:

- At least one approval from maintainers
- All CI checks must pass
- No unresolved conversations

---

## Architecture Overview

### Backend Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      HTTP Server                         │
│  (Echo framework, middleware, routing)                   │
├─────────────────────────────────────────────────────────┤
│                      Handlers                            │
│  (Request validation, response formatting)               │
├─────────────────────────────────────────────────────────┤
│                      Services                            │
│  (Business logic, orchestration)                         │
├─────────────────────────────────────────────────────────┤
│                    Repositories                          │
│  (Data access, caching)                                  │
├─────────────────────────────────────────────────────────┤
│                      Database                            │
│  (SQLite, with optional PostgreSQL)                      │
└─────────────────────────────────────────────────────────┘
```

### Frontend Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        Views                             │
│  (Page components, routing)                              │
├─────────────────────────────────────────────────────────┤
│                     Components                           │
│  (Reusable UI components)                                │
├─────────────────────────────────────────────────────────┤
│                   Composables                            │
│  (Shared logic, hooks)                                   │
├─────────────────────────────────────────────────────────┤
│                      Stores                              │
│  (Pinia state management)                                │
├─────────────────────────────────────────────────────────┤
│                    API Client                            │
│  (HTTP requests, WebSocket)                              │
└─────────────────────────────────────────────────────────┘
```

---

## Debugging

### Backend Debugging

**VS Code Launch Configuration**
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

**Using Delve**
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/server -- --config config.yaml
```

### Frontend Debugging

- Use Vue DevTools browser extension
- Use browser developer tools
- Add `debugger` statements in code

---

## Release Process

1. Update version in `version.go`
2. Update CHANGELOG.md
3. Create release branch: `release/v0.5.0`
4. Run full test suite
5. Create GitHub release with tag
6. CI builds and publishes artifacts

---

## Resources

- [Go Documentation](https://go.dev/doc/)
- [Vue.js Documentation](https://vuejs.org/)
- [Echo Framework](https://echo.labstack.com/)
- [Pinia](https://pinia.vuejs.org/)
- [Tailwind CSS](https://tailwindcss.com/)
