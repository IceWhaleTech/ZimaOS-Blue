# 聊天交互流程

本文描述 ZimaOS-Echo 中完整的聊天交互流程，重点说明与 Claude Code CLI 的集成。

## 概述

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              前端 (Vue/Pinia)                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  用户输入 → sendMessage() → SSEClient.connect()                              │
│                                    │                                         │
│                                    ▼                                         │
│  POST /api/v1/conversations/{id}/messages/stream                            │
│  Headers: Accept: text/event-stream                                          │
│  Body: { message, provider, model, temperature, max_tokens }                │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           后端 (chat.go)                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  StreamMessage()                                                             │
│    ├─ 将用户消息写入 SQLite                                                   │
│    ├─ 获取消息历史（50 条）                                                    │
│    ├─ 获取 provider (claude-code)                                            │
│    ├─ 调用 provider.ChatStream(ctx, chatReq)                                │
│    └─ 设置 SSE 头: Content-Type: text/event-stream                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Claude Code Provider (provider.go)                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  ChatStream()                                                                │
│    ├─ buildRunParams() - 构建 CLI 参数                                        │
│    ├─ sandboxedRunner.RunStream(ctx, params)                                │
│    └─ 将 CliStreamChunk 转为 llm.StreamChunk                                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Runner (runner.go)                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│  RunStream()                                                                 │
│    ├─ 构建命令: claude -p --output-format stream-json ...                    │
│    ├─ exec.CommandContext() 启动进程                                          │
│    ├─ cmd.StdoutPipe() 获取输出管道                                           │
│    └─ parser.ParseStream(stdout, "jsonl")                                     │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         OutputParser (parser.go)                              │
├─────────────────────────────────────────────────────────────────────────────┤
│  ParseStream() → parseStreamJSONL()                                          │
│    ├─ bufio.Reader 逐行读取                                                    │
│    ├─ json.Unmarshal 解析每行 JSON                                            │
│    ├─ extractText() - 提取文本内容                                             │
│    └─ 将 CliStreamChunk 发送到 channel                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           SSE 响应格式                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│  data: {"delta":"Hello","done":false,"stream_id":"xxx"}\n\n                 │
│  data: {"delta":" World","done":false,"stream_id":"xxx"}\n\n                │
│  data: {"delta":"!","done":true,"usage":{...},"stream_id":"xxx"}\n\n        │
│  data: [DONE]\n\n                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           前端处理 (sse.ts)                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ReadableStream.read() 读取数据块                                            │
│    ├─ 解析 "data: " 前缀                                                       │
│    ├─ JSON.parse() 解析数据块                                                 │
│    ├─ onMessage(chunk) - 累积 streamingContent                                │
│    └─ 更新 messages 数组中最后一条消息内容                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## CLI 命令

Claude Code CLI 使用以下命令执行：

```bash
claude -p \
  --output-format stream-json \
  --dangerously-skip-permissions \
  --model sonnet \
  --session-id <session-id> \
  --append-system-prompt "<system-prompt>" \
  "<user-prompt>"
```

### 命令参数

| 参数 | 说明 |
|----------|-------------|
| `-p` | 打印模式（非交互） |
| `--output-format stream-json` | 以流式 JSON 格式（JSONL）输出 |
| `--dangerously-skip-permissions` | 跳过权限确认 |
| `--model <model>` | 使用的模型（opus, sonnet, haiku） |
| `--session-id <id>` | 会话 ID，用于保持上下文 |
| `--append-system-prompt <prompt>` | 追加的系统提示 |
| `--resume <session-id>` | 恢复已有会话 |

### 环境变量

| 变量 | 说明 |
|----------|-------------|
| `ANTHROPIC_AUTH_TOKEN` | API 认证令牌 |
| `ANTHROPIC_BASE_URL` | 自定义 API 基础 URL（可选） |

## 关键代码位置

| 组件 | 文件 | 关键函数 |
|-----------|------|--------------|
| 前端 Store | `web/src/stores/chat.ts:139` | `sendMessage()` |
| SSE 客户端 | `web/src/utils/sse.ts:13` | `connect()` |
| 后端 Handler | `server/internal/server/chat.go:356` | `StreamMessage()` |
| Provider | `server/internal/claudecode/provider.go:97` | `ChatStream()` |
| Runner | `server/internal/claudecode/runner.go:224` | `RunStream()` |
| Parser | `server/internal/claudecode/parser.go:132` | `parseStreamJSONL()` |

## 流式实现

### 后端流式 (chat.go:455-588)

```go
// 设置 SSE 头
c.Response().Header().Set("Content-Type", "text/event-stream")
c.Response().Header().Set("Cache-Control", "no-cache")
c.Response().Header().Set("Connection", "keep-alive")

// 流式循环
for {
    select {
    case <-ctx.Done():
        // 处理取消
    case chunk, ok := <-chunks:
        if !ok {
            // 通道关闭，发送 [DONE]
            c.Response().Write([]byte("data: [DONE]\n\n"))
            c.Response().Flush()
            return nil
        }

        // 写入 SSE 数据
        data := map[string]interface{}{
            "delta":     chunk.Delta,
            "done":      chunk.Done,
            "stream_id": streamID,
        }
        jsonData, _ := json.Marshal(data)
        c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
        c.Response().Flush()
    }
}
```

### 前端流式 (sse.ts:56-91)

```typescript
const reader = response.body?.getReader()
const decoder = new TextDecoder()
let buffer = ''

while (this.isConnected) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
        if (line.startsWith('data: ')) {
            const data = line.slice(6).trim()
            if (data === '[DONE]') {
                options.onComplete?.()
                break
            }
            const chunk: StreamChunk = JSON.parse(data)
            options.onMessage(chunk)
        }
    }
}
```

## 沙箱模式

启用沙箱模式时，CLI 在隔离环境中执行：

```go
// SandboxedRunner 使用沙箱包装 Runner
type SandboxedRunner struct {
    runner         *Runner
    sandboxManager *sandbox.Manager
    config         *ClaudeCodeConfig
    enabled        bool
}

// 在沙箱中执行
func (sr *SandboxedRunner) Run(ctx context.Context, params *RunParams) (*RunResult, error) {
    if !sr.enabled || sr.sandboxManager == nil {
        return sr.runner.Run(ctx, params)
    }
    return sr.runSandboxed(ctx, params)
}
```

### 沙箱配置

```yaml
claudecode:
  sandbox:
    enabled: true
    memory_limit: 536870912  # 512 MB
    cpu_limit: 1.0
    process_limit: 50
    network_enabled: true
    allowed_paths:
      - /data/workspace
    denied_paths:
      - /etc/passwd
      - /etc/shadow
      - /root
      - /home
```

## 取消流程

```
前端: cancelStreaming()
    └─ sseClient.disconnect()
        └─ abortController.abort()
            └─ Fetch 被中止

后端: ctx.Done() 被触发
    ├─ 记录指标（已取消）
    ├─ 保存部分消息 + "[Response interrupted]"
    ├─ 发送取消事件
    └─ 注销流
```

## API 端点

| 方法 | 端点 | 说明 |
|--------|----------|-------------|
| POST | `/api/v1/conversations` | 创建会话 |
| GET | `/api/v1/conversations` | 列出会话 |
| GET | `/api/v1/conversations/{id}` | 获取会话 |
| DELETE | `/api/v1/conversations/{id}` | 删除会话 |
| GET | `/api/v1/conversations/{id}/messages` | 获取消息 |
| POST | `/api/v1/conversations/{id}/messages` | 发送消息（非流式） |
| POST | `/api/v1/conversations/{id}/messages/stream` | 发送消息（流式） |
| POST | `/api/v1/conversations/{id}/messages/cancel` | 取消流 |
| GET | `/api/v1/streams/active` | 列出活跃流 |
| POST | `/api/v1/streams/cancel-all` | 取消所有流 |

## 数据流概览

```
用户输入
    ↓
前端 Store (Pinia)
    ↓
SSE 客户端 (Fetch API)
    ↓
后端 Chat Handler
    ↓
LLM Provider 注册表
    ↓
Claude Code Provider / 其他 Provider
    ↓
CLI Runner / API 调用
    ↓
流式数据块
    ↓
后端：存储消息与指标
    ↓
前端：实时更新 UI
    ↓
显示完整回复
```
