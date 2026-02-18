# OpenAI Integration Tests

本目录包含 OpenAI API 的集成测试，用于验证直接请求和通过 Echo 服务器代理请求是否正常工作。

## 测试类型

### 1. 直接请求测试 (`openai_integration_test.go`)

测试直接调用 OpenAI API，不经过任何代理或服务器。

**测试用例：**
- ✅ 基本聊天补全
- ✅ 流式响应
- ✅ 工具调用（Function Calling）
- ✅ 错误处理
- ✅ 自定义 Base URL（兼容第三方 OpenAI API）
- ✅ 模型列表获取
- ✅ 速率限制测试

### 2. 端到端测试 (`openai_e2e_test.go`)

测试通过 Echo 服务器代理的 OpenAI 请求，模拟 CLI 客户端的实际使用场景。

**测试用例：**
- ✅ 通过服务器的聊天补全
- ✅ 多轮对话上下文保持
- ✅ 流式响应
- ✅ 错误处理
- ✅ 直接请求 vs 代理请求对比
- ✅ 健康检查
- ⚡ 性能基准测试

## 运行测试

### 前置条件

设置 OpenAI API Key 环境变量：

\`\`\`bash
export OPENAI_API_KEY="sk-your-api-key-here"
\`\`\`

### 运行所有集成测试

\`\`\`bash
# 在 server 目录下运行
cd server

# 运行所有 OpenAI 集成测试
go test -v ./internal/llm -run TestOpenAI

# 运行所有端到端测试
go test -v ./internal/server -run TestOpenAI
\`\`\`

### 运行特定测试

\`\`\`bash
# 只运行直接请求测试
go test -v ./internal/llm -run TestOpenAIDirectIntegration

# 只运行通过服务器的测试
go test -v ./internal/server -run TestOpenAIThroughEchoServer

# 只运行流式响应测试
go test -v ./internal/server -run TestOpenAIStreamingThroughServer

# 运行对比测试
go test -v ./internal/server -run TestOpenAIProviderComparison
\`\`\`

### 运行性能基准测试

\`\`\`bash
# 基准测试 - 直接请求
go test -bench=BenchmarkOpenAIDirect ./internal/server -benchtime=5x

# 基准测试 - 通过服务器
go test -bench=BenchmarkOpenAIThroughServer ./internal/server -benchtime=5x

# 对比两种方式的性能
go test -bench=BenchmarkOpenAI ./internal/server -benchtime=5x
\`\`\`

### 使用自定义 Base URL

如果你想测试第三方 OpenAI 兼容 API：

\`\`\`bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_CUSTOM_BASE_URL="https://your-custom-api.com"

go test -v ./internal/llm -run TestOpenAICustomBaseURL
\`\`\`

### 跳过速率限制测试

速率限制测试会发送多个快速请求，可能会触发 API 限制。在 CI 环境中可以跳过：

\`\`\`bash
export SKIP_RATE_LIMIT_TEST=1
go test -v ./internal/llm -run TestOpenAI
\`\`\`

## 测试输出示例

### 成功的测试输出

\`\`\`
=== RUN   TestOpenAIDirectIntegration
=== RUN   TestOpenAIDirectIntegration/DirectRequest_ChatCompletion
    openai_integration_test.go:35: ✓ Direct request successful
    openai_integration_test.go:36:   Model: gpt-4o-mini
    openai_integration_test.go:37:   Response: Hello, World!
    openai_integration_test.go:38:   Tokens: 18 (prompt: 10, completion: 8)
=== RUN   TestOpenAIDirectIntegration/DirectRequest_Streaming
    openai_integration_test.go:67: ✓ Direct streaming request successful
    openai_integration_test.go:68:   Received 12 chunks
--- PASS: TestOpenAIDirectIntegration (3.45s)
    --- PASS: TestOpenAIDirectIntegration/DirectRequest_ChatCompletion (1.23s)
    --- PASS: TestOpenAIDirectIntegration/DirectRequest_Streaming (2.22s)
\`\`\`

### 端到端测试输出

\`\`\`
=== RUN   TestOpenAIThroughEchoServer
=== RUN   TestOpenAIThroughEchoServer/ThroughServer_ChatCompletion
    openai_e2e_test.go:56: ✓ Request through echo server successful
    openai_e2e_test.go:57:   Response: Hello from Echo Server!
=== RUN   TestOpenAIThroughEchoServer/ThroughServer_MultipleMessages
    openai_e2e_test.go:95: ✓ Multi-message conversation successful
    openai_e2e_test.go:96:   Context preserved correctly
--- PASS: TestOpenAIThroughEchoServer (4.12s)
\`\`\`

### 基准测试输出

\`\`\`
BenchmarkOpenAIDirect-8                5    1807890 ns/op
BenchmarkOpenAIThroughServer-8         5    1345678901 ns/op
\`\`\`

## 测试覆盖的场景

### ✅ 直接请求场景
- 基本的聊天补全请求
- 流式响应处理
- 工具调用（Function Calling）
- 错误处理和重试
- 上下文取消
- 自定义 Base URL

### ✅ 通过服务器场景
- HTTP API 请求处理
- SSE（Server-Sent Events）流式响应
- 多轮对话上下文管理
- 错误传播和处理
- 性能对比

### ✅ 边界情况
- 无效的 API Key
- 无效的模型名称
- 网络超时
- 速率限制

## 故障排除

### 测试被跳过

如果看到 `SKIP` 消息，说明没有设置 `OPENAI_API_KEY` 环境变量：

\`\`\`
--- SKIP: TestOpenAIDirectIntegration (0.00s)
    openai_integration_test.go:15: Skipping integration test: OPENAI_API_KEY not set
\`\`\`

**解决方法：** 设置环境变量后重新运行。

### 速率限制错误

如果遇到 `429 Too Many Requests` 错误：

\`\`\`
Request failed: API error: 429 - Rate limit exceeded
\`\`\`

**解决方法：**
1. 增加测试之间的延迟
2. 使用更高配额的 API Key
3. 跳过速率限制测试：`export SKIP_RATE_LIMIT_TEST=1`

### 超时错误

如果测试超时：

\`\`\`
context deadline exceeded
\`\`\`

**解决方法：**
1. 检查网络连接
2. 增加超时时间（在测试代码中修改 `context.WithTimeout`）
3. 使用更快的模型（如 `gpt-4o-mini`）

## CI/CD 集成

### GitHub Actions 示例

\`\`\`yaml
name: OpenAI Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run Integration Tests
        env:
          OPENAI_API_KEY: \${{ secrets.OPENAI_API_KEY }}
          SKIP_RATE_LIMIT_TEST: 1
        run: |
          cd server
          go test -v ./internal/llm -run TestOpenAI
          go test -v ./internal/server -run TestOpenAI
\`\`\`

## 最佳实践

1. **使用测试专用的 API Key**：不要使用生产环境的 API Key
2. **控制测试频率**：避免频繁运行以节省 API 配额
3. **使用便宜的模型**：测试时使用 `gpt-4o-mini` 而不是 `gpt-4`
4. **设置合理的超时**：避免测试挂起
5. **清理测试数据**：每次测试使用唯一的 conversation ID

## 贡献指南

添加新的测试用例时，请遵循以下规范：

1. 测试名称应清晰描述测试内容
2. 使用 `t.Skip()` 处理缺少环境变量的情况
3. 添加详细的日志输出（使用 `t.Logf()`）
4. 包含成功和失败的断言
5. 更新本 README 文档

## 相关文档

- [OpenAI API 文档](https://platform.openai.com/docs/api-reference)
- [Go Testing 包文档](https://pkg.go.dev/testing)
- [Echo 框架文档](https://echo.labstack.com/)
